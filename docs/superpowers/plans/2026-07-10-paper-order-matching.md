# Paper Order Matching Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让模拟交易挂单仅在有效 A 股时段撮合，自动失效过期委托，并让 Agent 获得干净、明确的实时持仓查询契约。

**Architecture:** 保留现有 `PaperStore`、SQLite 和 30 秒后台循环，在 `paper_matcher.go` 中增加可注入时间的撮合入口。每轮先事务化处理过期委托，再对有效时段内的委托请求 TDX 行情；持仓和 MCP 契约沿用现有接口，只做必要修正。

**Tech Stack:** Go 标准库、`database/sql`、SQLite、现有 TDX 客户端、Go `testing`。

## Global Constraints

- 所有撮合时间固定使用 `Asia/Shanghai` 的 UTC+8 口径。
- 仅排除周六、周日，不新增节假日日历。
- 普通委托撮合窗口为 `09:30:00-11:30:00`、`13:00:00-15:00:00`，边界包含。
- 集合竞价撮合窗口为 `09:20:00-09:25:00`，边界包含。
- 后台撮合周期保持 30 秒。
- 不实现部分成交、盘口排队或逐笔回放。
- 不修改当前工作区中的 OBV 文件。

---

### Task 1: 固定撮合时段与集合竞价委托语义

**Files:**
- Modify: `cmd/server/paper_order.go`
- Modify: `cmd/server/paper_order_test.go`
- Modify: `cmd/server/paper_matcher.go`
- Modify: `cmd/server/paper_matcher_test.go`

**Interfaces:**
- Produces: `paperOrderCanMatch(order PaperOrder, now time.Time) bool`
- Produces: `paperOrderExpired(order PaperOrder, now time.Time) (bool, error)`
- Produces: `(*PaperStore).matchOpenOrdersAt(quote PaperQuoteProvider, now time.Time) error`
- Preserves: `(*PaperStore).MatchOpenOrders(quote PaperQuoteProvider) error`

- [ ] **Step 1: Write failing session-policy tests**

Add table-driven tests to `paper_matcher_test.go` using fixed UTC+8 timestamps:

```go
func paperTestTime(hour, minute, second int) time.Time {
  return time.Date(2026, 7, 10, hour, minute, second, 0, paperShanghaiLocation)
}

func TestPaperOrderCanMatchOnlyInItsSession(t *testing.T) {
  day := PaperOrder{TimeInForce: paperTimeInForceDay}
  auction := PaperOrder{TimeInForce: paperTimeInForceAuctionOnly}
  tests := []struct {
    name  string
    order PaperOrder
    at    time.Time
    want  bool
  }{
    {"day before open", day, paperTestTime(9, 29, 59), false},
    {"day morning", day, paperTestTime(9, 30, 0), true},
    {"day lunch break", day, paperTestTime(12, 0, 0), false},
    {"day afternoon", day, paperTestTime(13, 0, 0), true},
    {"day close boundary", day, paperTestTime(15, 0, 0), true},
    {"day after close", day, paperTestTime(15, 0, 1), false},
    {"auction start", auction, paperTestTime(9, 20, 0), true},
    {"auction end", auction, paperTestTime(9, 25, 0), true},
    {"auction late", auction, paperTestTime(9, 25, 1), false},
  }
  for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
      if got := paperOrderCanMatch(tt.order, tt.at); got != tt.want {
        t.Fatalf("paperOrderCanMatch() = %v, want %v", got, tt.want)
      }
    })
  }
}
```

Add a weekend case with `2026-07-11 10:00:00` and expect `false`.

Add `TestMatchOpenOrdersSkipsOutsideTradingSession`: place a pending limit
order, call `matchOpenOrdersAt` at `12:00:00`, count quote-provider calls, and
assert the count is zero while the order remains `pending`.

- [ ] **Step 2: Run the session test and verify RED**

Run:

```powershell
$env:GOCACHE='E:\project\tdx-api\.tmp\go-build'
& 'C:\Program Files\Go\bin\go.exe' test -vet=off ./cmd/server -run TestPaperOrderCanMatchOnlyInItsSession -count=1
```

Expected: compilation fails because `paperShanghaiLocation` and `paperOrderCanMatch` do not exist.

- [ ] **Step 3: Implement the minimal session policy**

Add to `paper_matcher.go`:

```go
var paperShanghaiLocation = time.FixedZone("Asia/Shanghai", 8*60*60)

func paperOrderCanMatch(order PaperOrder, now time.Time) bool {
  now = now.In(paperShanghaiLocation)
  if now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
    return false
  }
  second := now.Hour()*3600 + now.Minute()*60 + now.Second()
  if order.TimeInForce == paperTimeInForceAuctionOnly {
    return second >= 9*3600+20*60 && second <= 9*3600+25*60
  }
  return second >= 9*3600+30*60 && second <= 11*3600+30*60 ||
    second >= 13*3600 && second <= 15*3600
}
```

- [ ] **Step 4: Verify session tests GREEN**

Run the command from Step 2.

Expected: `ok github.com/injoyai/tdx/cmd/server`.

- [ ] **Step 5: Write failing auction-normalization tests**

Add to `paper_order_test.go`:

```go
func TestNormalizePaperAuctionOrderUsesAuctionOnly(t *testing.T) {
  req := PaperPlaceOrderRequest{
    AccountID: "acct", Code: "600000", Side: paperSideBuy,
    OrderType: paperOrderAuction, Price: 10, Quantity: 100,
  }
  if err := normalizePaperOrderRequest(&req); err != nil {
    t.Fatal(err)
  }
  if req.TimeInForce != paperTimeInForceAuctionOnly {
    t.Fatalf("timeInForce = %q, want auction_only", req.TimeInForce)
  }
}

func TestNormalizePaperOrderRejectsAuctionOnlyLimitOrder(t *testing.T) {
  req := PaperPlaceOrderRequest{
    AccountID: "acct", Code: "600000", Side: paperSideBuy,
    OrderType: paperOrderLimit, TimeInForce: paperTimeInForceAuctionOnly,
    Price: 10, Quantity: 100,
  }
  if err := normalizePaperOrderRequest(&req); err == nil {
    t.Fatal("normalizePaperOrderRequest() error = nil, want error")
  }
}
```

- [ ] **Step 6: Verify auction tests RED**

Run:

```powershell
& 'C:\Program Files\Go\bin\go.exe' test -vet=off ./cmd/server -run 'TestNormalizePaperAuction|TestNormalizePaperOrderRejectsAuctionOnly' -count=1
```

Expected: default auction order remains `day`, and the invalid combination is accepted.

- [ ] **Step 7: Implement auction type/time-in-force pairing**

In `normalizePaperOrderRequest`, replace the generic default with:

```go
if req.TimeInForce == "" {
  if req.OrderType == paperOrderAuction {
    req.TimeInForce = paperTimeInForceAuctionOnly
  } else {
    req.TimeInForce = paperTimeInForceDay
  }
}
if (req.OrderType == paperOrderAuction) !=
  (req.TimeInForce == paperTimeInForceAuctionOnly) {
  return errors.New("auction order type and auction_only time in force must be used together")
}
```

- [ ] **Step 8: Route matching through an injectable clock**

Keep the public method and move its current body to a private method:

```go
func (s *PaperStore) MatchOpenOrders(quote PaperQuoteProvider) error {
  return s.matchOpenOrdersAt(quote, time.Now())
}

func (s *PaperStore) matchOpenOrdersAt(
  quote PaperQuoteProvider,
  now time.Time,
) error {
  // Move the current MatchOpenOrders body here unchanged, then apply the
  // session guard shown below immediately before quote(order.Code).
}
```

Before requesting a quote in the loop, add:

```go
if !paperOrderCanMatch(order, now) {
  continue
}
```

Update the two existing limit matching tests to call `matchOpenOrdersAt` with
`paperTestTime(10, 0, 0)`, so tests never depend on wall-clock time.

- [ ] **Step 9: Run focused tests and commit**

Run:

```powershell
& 'C:\Program Files\Go\bin\go.exe' test -vet=off ./cmd/server -run 'TestPaperOrderCanMatch|TestNormalizePaper|TestMatchPaperLimit' -count=1
```

Expected: PASS.

Commit only Task 1 files:

```powershell
git add cmd/server/paper_types.go cmd/server/paper_order.go cmd/server/paper_order_test.go cmd/server/paper_matcher.go cmd/server/paper_matcher_test.go
git commit -m "fix: enforce paper order trading sessions"
```

---

### Task 2: 自动失效过期委托并释放冻结资源

**Files:**
- Modify: `cmd/server/paper_types.go`
- Modify: `cmd/server/paper_order.go`
- Modify: `cmd/server/paper_matcher.go`
- Modify: `cmd/server/paper_matcher_test.go`

**Interfaces:**
- Consumes: `paperShanghaiLocation`, `paperOrderCanMatch`, `matchOpenOrdersAt`
- Produces: `paperOrderExpired(order PaperOrder, now time.Time) (bool, error)`
- Produces: `(*PaperStore).ExpireOrder(orderID string, now time.Time) error`

- [ ] **Step 1: Write failing expiry-policy tests**

Add table tests for same-day and prior-day expiry:

```go
func TestPaperOrderExpiredAtSessionDeadline(t *testing.T) {
  day := PaperOrder{
    TimeInForce: paperTimeInForceDay,
    CreatedAt: paperTestTime(9, 0, 0).Format(time.RFC3339Nano),
  }
  auction := PaperOrder{
    TimeInForce: paperTimeInForceAuctionOnly,
    CreatedAt: paperTestTime(9, 0, 0).Format(time.RFC3339Nano),
  }
  tests := []struct {
    order PaperOrder
    at    time.Time
    want  bool
  }{
    {day, paperTestTime(15, 0, 0), false},
    {day, paperTestTime(15, 0, 1), true},
    {auction, paperTestTime(9, 25, 0), false},
    {auction, paperTestTime(9, 25, 1), true},
    {day, paperTestTime(9, 0, 0).AddDate(0, 0, 1), true},
  }
  for _, tt := range tests {
    got, err := paperOrderExpired(tt.order, tt.at)
    if err != nil {
      t.Fatal(err)
    }
    if got != tt.want {
      t.Fatalf("paperOrderExpired() = %v, want %v", got, tt.want)
    }
  }
}
```

Also assert malformed `CreatedAt` returns an error.

- [ ] **Step 2: Verify expiry-policy test RED**

Run:

```powershell
& 'C:\Program Files\Go\bin\go.exe' test -vet=off ./cmd/server -run TestPaperOrderExpiredAtSessionDeadline -count=1
```

Expected: compilation fails because `paperOrderExpired` does not exist.

- [ ] **Step 3: Implement minimal expiry policy**

Add `paperOrderExpired` to `paper_matcher.go`. Parse `CreatedAt` with
`time.RFC3339Nano`, compare its Shanghai calendar date with `now`, then use
`09:25:00` for `auction_only` and `15:00:00` for `day`. A prior calendar date
always expires before matching.

```go
func paperOrderExpired(order PaperOrder, now time.Time) (bool, error) {
  createdAt, err := time.Parse(time.RFC3339Nano, order.CreatedAt)
  if err != nil {
    return false, fmt.Errorf("parse order created_at: %w", err)
  }
  createdAt = createdAt.In(paperShanghaiLocation)
  now = now.In(paperShanghaiLocation)
  createdDay := createdAt.Format("2006-01-02")
  currentDay := now.Format("2006-01-02")
  if createdDay != currentDay {
    return createdDay < currentDay, nil
  }
  second := now.Hour()*3600 + now.Minute()*60 + now.Second()
  if order.TimeInForce == paperTimeInForceAuctionOnly {
    return second > 9*3600+25*60, nil
  }
  return second > 15*3600, nil
}
```

- [ ] **Step 4: Verify expiry-policy test GREEN**

Run the command from Step 2.

Expected: PASS.

- [ ] **Step 5: Write failing transactional expiry tests**

Add one buy and one sell test. The buy test places a limit order, invokes
`ExpireOrder`, and asserts:

```go
if expired.Status != paperOrderExpiredStatus {
  t.Fatalf("status = %q, want expired", expired.Status)
}
assertFloatEqual(t, account.FrozenCash, 0)
assertFloatEqual(t, account.AvailableCash, account.InitialCash)
```

The sell test asserts the frozen quantity returns to `sellable_quantity` and
`paper_agent_actions` contains an `expire_order` action.

- [ ] **Step 6: Verify transactional expiry tests RED**

Run:

```powershell
& 'C:\Program Files\Go\bin\go.exe' test -vet=off ./cmd/server -run 'TestExpirePaper' -count=1
```

Expected: compilation fails because the expired status and `ExpireOrder` do not exist.

- [ ] **Step 7: Implement transactional expiry**

Add `paperOrderExpiredStatus = "expired"` to `paper_types.go`.

Rename `releasePaperCancelledOrder` to `releasePaperOrderResources` and use it
from both cancellation and expiration. Implement `ExpireOrder` in
`paper_order.go` using one transaction:

```go
func (s *PaperStore) ExpireOrder(orderID string, now time.Time) error {
  tx, err := s.db.BeginTx(context.Background(), nil)
  if err != nil {
    return err
  }
  defer tx.Rollback()
  order, err := getPaperOrderForUpdate(tx, orderID)
  if err != nil {
    return err
  }
  if order.Status != paperOrderPending {
    return nil
  }
  timestamp := now.In(paperShanghaiLocation).Format(time.RFC3339Nano)
  if err := releasePaperOrderResources(tx, order, timestamp); err != nil {
    return err
  }
  if err := markPaperOrderExpired(tx, order.ID, timestamp); err != nil {
    return err
  }
  if err := insertPaperExpireAction(tx, order, timestamp); err != nil {
    return err
  }
  return tx.Commit()
}
```

`markPaperOrderExpired` updates only rows whose current status is `pending`.
`insertPaperExpireAction` records `action_type="expire_order"`, order ID and
the reason `time_in_force_expired`.

- [ ] **Step 8: Expire before requesting quotes**

In `matchOpenOrdersAt`, order the loop as follows:

```go
expired, err := paperOrderExpired(order, now)
if err != nil {
  return err
}
if expired {
  if err := s.ExpireOrder(order.ID, now); err != nil {
    return err
  }
  continue
}
if !paperOrderCanMatch(order, now) {
  continue
}
q, err := quote(order.Code)
```

Add a quote counter to the expiry integration test and assert it remains zero,
proving expired orders cannot consume行情或成交。

- [ ] **Step 9: Run focused tests and commit**

Run:

```powershell
& 'C:\Program Files\Go\bin\go.exe' test -vet=off ./cmd/server -run 'TestPaperOrderExpired|TestExpirePaper|TestMatchPaper' -count=1
```

Expected: PASS.

Commit only Task 2 files:

```powershell
git add cmd/server/paper_types.go cmd/server/paper_order.go cmd/server/paper_matcher.go cmd/server/paper_matcher_test.go
git commit -m "fix: expire stale paper orders"
```

---

### Task 3: 清理持仓查询并强化 Agent MCP 契约

**Files:**
- Modify: `cmd/server/paper_broker.go`
- Modify: `cmd/server/paper_broker_test.go`
- Modify: `cmd/server/paper_mcp.go`
- Modify: `cmd/server/paper_mcp_test.go`

**Interfaces:**
- Preserves: `(*PaperStore).ListPositions(accountID string) ([]PaperPosition, error)`
- Preserves: MCP tool `tdx_paper_portfolio`
- Produces: Agent-visible instruction to query `positions` and `orders` before trading.

- [ ] **Step 1: Write a failing zero-position filtering test**

Add to `paper_broker_test.go`:

```go
func TestListPaperPositionsOmitsClosedRows(t *testing.T) {
  store := newTestPaperStore(t)
  account, err := store.CreateAccount(PaperCreateAccountRequest{
    Name: "holder",
    InitialPositions: []PaperInitialPosition{
      {Code: "600000", Quantity: 100, CostPrice: 10},
    },
  })
  if err != nil {
    t.Fatal(err)
  }
  if _, err := store.db.Exec(`
    UPDATE paper_positions SET quantity = 0, sellable_quantity = 0
    WHERE account_id = ? AND code = ?
  `, account.ID, "600000"); err != nil {
    t.Fatal(err)
  }
  positions, err := store.ListPositions(account.ID)
  if err != nil {
    t.Fatal(err)
  }
  if len(positions) != 0 {
    t.Fatalf("positions = %+v, want none", positions)
  }
}
```

- [ ] **Step 2: Verify filtering test RED**

Run:

```powershell
& 'C:\Program Files\Go\bin\go.exe' test -vet=off ./cmd/server -run TestListPaperPositionsOmitsClosedRows -count=1
```

Expected: FAIL because `ListPositions` returns the zero-quantity row.

- [ ] **Step 3: Filter zero positions in the existing query**

Change only the SQL predicate in `ListPositions`:

```sql
WHERE account_id = ? AND quantity > 0
```

- [ ] **Step 4: Verify filtering test GREEN**

Run the command from Step 2.

Expected: PASS.

- [ ] **Step 5: Write a failing MCP-description test**

Extend `TestPaperPortfolioMCPSchemaDescriptions`:

```go
if !strings.Contains(tool.Description, "交易决策前") ||
  !strings.Contains(tool.Description, "positions") ||
  !strings.Contains(tool.Description, "orders") {
  t.Fatalf("portfolio description = %q", tool.Description)
}
```

Extend `TestPaperOrderMCPSchemaDescribesEnums` to assert `timeInForce` has no
static default, is required for `place`, and the conditional schema pairs
`auction` with `auction_only` while pairing `market/limit` with `day`.

Extend `TestPaperMCPRulesReturnsContent` to assert the returned text contains
`交易决策前先查询 positions 和 orders` and the structured matching rules
mention 30-second polling and automatic expiration.

- [ ] **Step 6: Verify MCP-description test RED**

Run:

```powershell
& 'C:\Program Files\Go\bin\go.exe' test -vet=off ./cmd/server -run 'TestPaperPortfolioMCPSchemaDescriptions|TestPaperMCPRulesReturnsContent' -count=1
```

Expected: FAIL because the current description does not state the required workflow.

- [ ] **Step 7: Update the existing MCP metadata**

Update `tdx_paper_portfolio` description to:

```text
纸上交易账户查询工具。交易决策前必须使用固定 accountId 查询 positions 和 orders；positions 返回当前持仓、可卖数量和冻结数量。支持 summary/cash/positions/trades/orders/performance/closed_positions/actions。
```

Update `tdx_paper_order` and `tdx_paper_rules` descriptions/rules to state:

- 每 30 秒扫描有效待处理委托。
- `day` 在普通交易时段撮合并于收盘后自动失效。
- `auction` 必须配合 `auction_only`，只在 `09:20:00-09:25:00` 撮合。
- Agent 交易前先查询 `positions` 和 `orders`，服务端仍以 SQLite 校验为准。

Replace `optionalEnumDefault` for `timeInForce` with `optionalEnum`, require
`timeInForce` for MCP `place`, and add conditional schemas equivalent to:

```json
{
  "if": {"properties": {"orderType": {"const": "auction"}}},
  "then": {"properties": {"timeInForce": {"const": "auction_only"}}}
}
```

Add the corresponding `market/limit -> day` condition. Runtime HTTP requests
may still omit the field and use the normalization defaults from Task 1.

- [ ] **Step 8: Run focused tests and commit**

Run:

```powershell
& 'C:\Program Files\Go\bin\go.exe' test -vet=off ./cmd/server -run 'TestListPaperPositions|TestPaperPortfolio|TestPaperMCPRules|TestPaperOrderMCP' -count=1
```

Expected: PASS.

Commit only Task 3 files:

```powershell
git add cmd/server/paper_broker.go cmd/server/paper_broker_test.go cmd/server/paper_mcp.go cmd/server/paper_mcp_test.go
git commit -m "fix: clarify paper portfolio state"
```

---

### Task 4: 全量验证与 MCP 查询契约核验

**Files:**
- Verify only; no production file changes.

**Interfaces:**
- Consumes: all Task 1-3 behavior.
- Produces: test evidence and verified `tdx_paper_portfolio` query capability.

- [ ] **Step 1: Run the complete server test package**

```powershell
$env:GOCACHE='E:\project\tdx-api\.tmp\go-build'
& 'C:\Program Files\Go\bin\go.exe' test -vet=off ./cmd/server -count=1
```

Expected: `ok github.com/injoyai/tdx/cmd/server`.

- [ ] **Step 2: Run repository-wide verification**

```powershell
& 'C:\Program Files\Go\bin\go.exe' test -vet=off ./... -count=1
```

Expected: all packages pass. If an unrelated pre-existing package fails, record
the exact package and error without modifying unrelated files.

- [ ] **Step 3: Verify the MCP contract and current remote query capability**

Run the focused MCP tests from Task 3 to verify the new local metadata. Then
call the currently deployed `tdx_paper_account(action=list)`, select the first
returned active account ID as `accountId`, and call
`tdx_paper_portfolio(view=positions, limit=200)` with that value.

Verify the response includes `quantity`, `sellableQuantity` and
`frozenQuantity`. The remote service will not expose the new `expired` state or
zero-position filtering until the resulting commit is pushed and its Docker
image is redeployed; record this deployment boundary explicitly.

- [ ] **Step 4: Check the final diff scope**

```powershell
git status --short
git diff --check HEAD~3..HEAD
```

Expected: implementation commits touch only the planned paper-trading files;
the pre-existing OBV working-tree changes remain uncommitted and untouched.

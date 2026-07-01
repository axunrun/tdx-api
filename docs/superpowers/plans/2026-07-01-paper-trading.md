# Paper Trading Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a server-side paper brokerage system for A-share/ETF simulated trading, with MCP tools for Agent actions and a read-only WebUI dashboard.

**Architecture:** Keep the current `cmd/server` single-package pattern. Add a small paper trading core with SQLite persistence, reuse existing HTTP handler and MCP patterns, and replace the old static WebUI with a read-only dashboard that only calls paper HTTP APIs.

**Tech Stack:** Go standard library, existing `github.com/glebarez/go-sqlite`, existing TDX client methods, embedded static HTML/CSS/JS, existing JSON response envelope.

---

## File Map

Create:

- `cmd/server/paper_db.go`: database path, migration, connection helpers.
- `cmd/server/paper_types.go`: paper account/order/position/trade/snapshot structs and constants.
- `cmd/server/paper_broker.go`: account, position, cash ledger, trade ledger core.
- `cmd/server/paper_order.go`: order validation, fee calculation, order placement, cancel logic.
- `cmd/server/paper_matcher.go`: background matching, snapshots, closed-position tracking.
- `cmd/server/handlers_paper.go`: WebUI HTTP APIs.
- `cmd/server/paper_mcp.go`: paper MCP tool definitions and dispatch helpers.
- `cmd/server/paper_db_test.go`: migration/path tests.
- `cmd/server/paper_broker_test.go`: account and holding tests.
- `cmd/server/paper_order_test.go`: fees, validation, order tests.
- `cmd/server/paper_matcher_test.go`: matching and snapshot tests.
- `cmd/server/handlers_paper_test.go`: HTTP handler tests.
- `cmd/server/paper_mcp_test.go`: MCP schema/call tests.
- `cmd/server/static/styles.css`: dashboard styles.
- `cmd/server/static/app.js`: dashboard client logic.

Modify:

- `cmd/server/main.go`: register paper routes and start paper background tasks.
- `cmd/server/mcp.go`: include paper MCP tools in `mcpTools()`.
- `cmd/server/handlers_ext.go`: keep `handleWebUI`, but only serve the new embedded dashboard.
- `cmd/server/static/index.html`: replace old WebUI content with read-only dashboard shell.
- `docs/agent_api_design.md`: document paper HTTP APIs and MCP tools.

Do not create:

- ORM layer.
- React/Vite/npm frontend.
- WebSocket server.
- Auth/user tables.

---

## Shared Commands

Use these commands during implementation:

```powershell
$env:GOCACHE = "E:\project\tdx-api\.tmp\go-build"
& "C:\Program Files\Go\bin\go.exe" test -vet=off ./cmd/server
& "C:\Program Files\Go\bin\go.exe" test -vet=off ./...
```

Expected broad verification result:

```text
ok   github.com/injoyai/tdx/cmd/server
```

Some package timings may vary.

---

### Task 1: Paper SQLite Foundation

**Files:**

- Create: `cmd/server/paper_db.go`
- Create: `cmd/server/paper_types.go`
- Create: `cmd/server/paper_db_test.go`

- [ ] **Step 1: Write failing DB path and migration tests**

Create `cmd/server/paper_db_test.go` with tests for:

```go
func TestPaperDBPathUsesDedicatedDefault(t *testing.T) {
	t.Setenv("PAPER_DB_PATH", "")
	got := paperDBPath()
	want := filepath.FromSlash("data/database/tdx-paper.sqlite")
	if got != want {
		t.Fatalf("paperDBPath()=%q want %q", got, want)
	}
}

func TestPaperDBPathUsesEnvOverride(t *testing.T) {
	t.Setenv("PAPER_DB_PATH", filepath.Join(t.TempDir(), "paper.sqlite"))
	if got := paperDBPath(); !strings.HasSuffix(got, "paper.sqlite") {
		t.Fatalf("paperDBPath()=%q", got)
	}
}

func TestInitPaperDBCreatesTables(t *testing.T) {
	path := filepath.Join(t.TempDir(), "paper.sqlite")
	db, err := openPaperDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := initPaperDB(db); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{
		"paper_accounts",
		"paper_positions",
		"paper_orders",
		"paper_trades",
		"paper_cash_ledger",
		"paper_position_ledger",
		"paper_agent_actions",
		"paper_account_snapshots",
		"paper_closed_positions",
		"paper_closed_position_tracking",
	} {
		if !paperTableExists(t, db, table) {
			t.Fatalf("missing table %s", table)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```powershell
$env:GOCACHE = "E:\project\tdx-api\.tmp\go-build"
& "C:\Program Files\Go\bin\go.exe" test -vet=off ./cmd/server -run TestPaperDB
```

Expected: FAIL because `paperDBPath`, `openPaperDB`, and `initPaperDB` are undefined.

- [ ] **Step 3: Add paper types and DB helpers**

Create `cmd/server/paper_types.go` with constants:

```go
const (
	paperAccountActive = "active"
	paperAccountClosed = "closed"

	paperSideBuy  = "buy"
	paperSideSell = "sell"

	paperOrderMarket  = "market"
	paperOrderLimit   = "limit"
	paperOrderAuction = "auction"

	paperOrderPending   = "pending"
	paperOrderFilled    = "filled"
	paperOrderCancelled = "cancelled"
	paperOrderRejected  = "rejected"

	paperTimeInForceDay         = "day"
	paperTimeInForceAuctionOnly = "auction_only"
)
```

Create `cmd/server/paper_db.go` with:

```go
const defaultPaperDBPath = "data/database/tdx-paper.sqlite"

var paperDBWriteMu sync.Mutex

func paperDBPath() string {
	if path := os.Getenv("PAPER_DB_PATH"); path != "" {
		return path
	}
	return filepath.FromSlash(defaultPaperDBPath)
}

func openPaperDB(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if err := initPaperDB(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
```

Add `initPaperDB(db *sql.DB) error` with one `Exec` migration string creating all tables listed in the design. Use `CREATE TABLE IF NOT EXISTS`, `TEXT` IDs, `REAL` money fields, `INTEGER` quantities, and ISO datetime strings.

- [ ] **Step 4: Run DB tests**

Run:

```powershell
& "C:\Program Files\Go\bin\go.exe" test -vet=off ./cmd/server -run "TestPaperDB|TestInitPaperDB"
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add cmd/server/paper_db.go cmd/server/paper_types.go cmd/server/paper_db_test.go
git commit -m "feat: add paper trading database"
```

---

### Task 2: Account and Position Core

**Files:**

- Create: `cmd/server/paper_broker.go`
- Create: `cmd/server/paper_broker_test.go`
- Modify: `cmd/server/paper_types.go`

- [ ] **Step 1: Write failing account creation tests**

Create tests for:

```go
func TestCreatePaperAccountLocksInitialState(t *testing.T) {
	store := newTestPaperStore(t)
	account, err := store.CreateAccount(PaperCreateAccountRequest{
		Name:        "agent-a",
		InitialCash: 100000,
		InitialPositions: []PaperInitialPosition{
			{Code: "300499", Name: "高澜股份", Quantity: 200, CostPrice: 10, BuyDate: "2026-06-01"},
		},
		Note: "initial setup",
	})
	if err != nil {
		t.Fatal(err)
	}
	if account.ID == "" || account.Status != paperAccountActive {
		t.Fatalf("bad account: %+v", account)
	}
	got, err := store.GetAccount(account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.InitialCash != 100000 {
		t.Fatalf("initial cash=%v", got.InitialCash)
	}
	positions, err := store.ListPositions(account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(positions) != 1 || positions[0].SellableQuantity != 200 {
		t.Fatalf("positions=%+v", positions)
	}
}
```

Also test validation:

```go
func TestCreatePaperAccountRejectsInvalidCash(t *testing.T) {
	store := newTestPaperStore(t)
	_, err := store.CreateAccount(PaperCreateAccountRequest{Name: "bad", InitialCash: -1})
	if err == nil {
		t.Fatal("expected error")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```powershell
& "C:\Program Files\Go\bin\go.exe" test -vet=off ./cmd/server -run TestCreatePaperAccount
```

Expected: FAIL because store/types are missing.

- [ ] **Step 3: Implement minimal broker store**

Add to `paper_types.go`:

```go
type PaperCreateAccountRequest struct {
	Name             string
	InitialCash      float64
	InitialPositions []PaperInitialPosition
	Note             string
}

type PaperInitialPosition struct {
	Code      string
	Name      string
	Quantity  int64
	CostPrice float64
	BuyDate   string
}

type PaperAccount struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	InitialCash float64 `json:"initialCash"`
	Status      string  `json:"status"`
	Note        string  `json:"note,omitempty"`
	CreatedAt   string  `json:"createdAt"`
	ClosedAt    string  `json:"closedAt,omitempty"`
}

type PaperPosition struct {
	AccountID        string  `json:"accountId"`
	Code             string  `json:"code"`
	Name             string  `json:"name,omitempty"`
	AssetType        string  `json:"assetType"`
	Quantity         int64   `json:"quantity"`
	SellableQuantity int64   `json:"sellableQuantity"`
	AvgCost          float64 `json:"avgCost"`
	UpdatedAt        string  `json:"updatedAt"`
}
```

Create `paper_broker.go` with:

```go
type PaperStore struct {
	db *sql.DB
}

func NewPaperStore(db *sql.DB) *PaperStore {
	return &PaperStore{db: db}
}

func (s *PaperStore) CreateAccount(req PaperCreateAccountRequest) (PaperAccount, error) {
	if strings.TrimSpace(req.Name) == "" {
		return PaperAccount{}, fmt.Errorf("账户名称不能为空")
	}
	if req.InitialCash < 0 {
		return PaperAccount{}, fmt.Errorf("初始资金不能为负数")
	}
	now := time.Now().Format(time.RFC3339)
	account := PaperAccount{
		ID:          uuid.NewString(),
		Name:        strings.TrimSpace(req.Name),
		InitialCash: req.InitialCash,
		Status:      paperAccountActive,
		Note:        req.Note,
		CreatedAt:   now,
	}
	// Use one transaction. Insert account, cash ledger, initial positions, position ledger, and agent action.
	return account, nil
}
```

Complete the transaction with plain `database/sql`. Do not add an ORM.

- [ ] **Step 4: Run account tests**

```powershell
& "C:\Program Files\Go\bin\go.exe" test -vet=off ./cmd/server -run "TestCreatePaperAccount|TestPaperDB"
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add cmd/server/paper_types.go cmd/server/paper_broker.go cmd/server/paper_broker_test.go
git commit -m "feat: add paper account core"
```

---

### Task 3: Orders, Fees, and Validation

**Files:**

- Create: `cmd/server/paper_order.go`
- Create: `cmd/server/paper_order_test.go`
- Modify: `cmd/server/paper_types.go`

- [ ] **Step 1: Write failing fee and order validation tests**

Test:

```go
func TestPaperFeeForStockSell(t *testing.T) {
	fee := calculatePaperFee(PaperFeeInput{
		AssetType: "stock",
		Side:      paperSideSell,
		Amount:    100000,
	})
	if fee.Commission != 10 {
		t.Fatalf("commission=%v", fee.Commission)
	}
	if fee.StampTax <= 0 {
		t.Fatalf("stamp tax=%v", fee.StampTax)
	}
	if fee.Total() <= fee.Commission {
		t.Fatalf("total fee=%v", fee.Total())
	}
}

func TestPaperFeeForETFSkipsStampTax(t *testing.T) {
	fee := calculatePaperFee(PaperFeeInput{
		AssetType: "etf",
		Side:      paperSideSell,
		Amount:    100000,
	})
	if fee.StampTax != 0 {
		t.Fatalf("ETF stamp tax=%v", fee.StampTax)
	}
}
```

Test order validation:

```go
func TestPlacePaperLimitBuyFreezesCash(t *testing.T) {
	store := newFundedPaperStore(t, 100000)
	order, err := store.PlaceOrder(PaperPlaceOrderRequest{
		AccountID:   testAccountID(t, store),
		Code:        "300499",
		Name:        "高澜股份",
		AssetType:   "stock",
		Side:        paperSideBuy,
		OrderType:   paperOrderLimit,
		Price:       10,
		Quantity:    100,
		TimeInForce: paperTimeInForceDay,
	})
	if err != nil {
		t.Fatal(err)
	}
	if order.Status != paperOrderPending {
		t.Fatalf("status=%s", order.Status)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```powershell
& "C:\Program Files\Go\bin\go.exe" test -vet=off ./cmd/server -run "TestPaperFee|TestPlacePaper"
```

Expected: FAIL because fee and order functions are missing.

- [ ] **Step 3: Implement fixed fee rules and order placement**

Add fixed constants:

```go
const (
	paperCommissionRate = 0.0001
	paperStampTaxRate   = 0.0005
	paperTransferRate   = 0.00001
)
```

Implementation rule:

- Commission = `amount * 0.0001`.
- Stamp tax only on stock sell.
- Transfer fee applies to A-share stock trades.
- ETF has no stamp tax.

`PlaceOrder` must:

- validate active account.
- validate `side`, `orderType`, `timeInForce`.
- reject quantity <= 0.
- reject stock buy quantity not divisible by 100.
- reject limit/auction orders with price <= 0.
- freeze cash for buy orders.
- freeze sellable position for sell orders.
- write `paper_orders`.
- write `paper_agent_actions`.

- [ ] **Step 4: Run order tests**

```powershell
& "C:\Program Files\Go\bin\go.exe" test -vet=off ./cmd/server -run "TestPaperFee|TestPlacePaper|TestCreatePaperAccount"
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add cmd/server/paper_types.go cmd/server/paper_order.go cmd/server/paper_order_test.go
git commit -m "feat: add paper order validation"
```

---

### Task 4: Matching, Trades, Snapshots, and Closed Tracking

**Files:**

- Create: `cmd/server/paper_matcher.go`
- Create: `cmd/server/paper_matcher_test.go`
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Write failing matching tests**

Use an injectable quote function:

```go
type PaperQuoteProvider func(code string) (PaperQuote, error)

func TestMatchPaperLimitBuyFillsWhenPriceIsBelowLimit(t *testing.T) {
	store := newFundedPaperStore(t, 100000)
	accountID := testAccountID(t, store)
	order, err := store.PlaceOrder(PaperPlaceOrderRequest{
		AccountID: accountID,
		Code: "300499", Name: "高澜股份", AssetType: "stock",
		Side: paperSideBuy, OrderType: paperOrderLimit,
		Price: 10, Quantity: 100, TimeInForce: paperTimeInForceDay,
	})
	if err != nil {
		t.Fatal(err)
	}
	err = store.MatchOpenOrders(func(code string) (PaperQuote, error) {
		return PaperQuote{Code: code, Price: 9.9, Name: "高澜股份"}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.GetOrder(order.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != paperOrderFilled {
		t.Fatalf("status=%s", got.Status)
	}
}
```

Also test closed-position tracking:

```go
func TestSellLastPositionCreatesClosedPosition(t *testing.T) {
	store := newStoreWithPosition(t, "300499", 100, 10)
	accountID := testAccountID(t, store)
	order, err := store.PlaceOrder(PaperPlaceOrderRequest{
		AccountID: accountID,
		Code: "300499", Name: "高澜股份", AssetType: "stock",
		Side: paperSideSell, OrderType: paperOrderMarket,
		Quantity: 100, TimeInForce: paperTimeInForceDay,
	})
	if err != nil {
		t.Fatal(err)
	}
	err = store.FillOrder(order.ID, PaperQuote{Code: "300499", Price: 11, Name: "高澜股份"})
	if err != nil {
		t.Fatal(err)
	}
	closed, err := store.ListClosedPositions(accountID, "all")
	if err != nil {
		t.Fatal(err)
	}
	if len(closed) != 1 {
		t.Fatalf("closed=%+v", closed)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```powershell
& "C:\Program Files\Go\bin\go.exe" test -vet=off ./cmd/server -run "TestMatchPaper|TestSellLastPosition"
```

Expected: FAIL because matching functions are missing.

- [ ] **Step 3: Implement minimal matching**

Implement:

```go
func (s *PaperStore) MatchOpenOrders(quote PaperQuoteProvider) error
func (s *PaperStore) FillOrder(orderID string, quote PaperQuote) error
func (s *PaperStore) CreateSnapshot(accountID string, quote PaperQuoteProvider) error
func startPaperBackgroundTasks(store *PaperStore, quote PaperQuoteProvider)
```

Keep first version simple:

- no partial fills.
- market orders fill immediately when matching runs.
- limit buy fills when `quote.Price <= order.Price`.
- limit sell fills when `quote.Price >= order.Price`.
- `auction` follows the same predicate after reference price is available.
- filled orders update cash, positions, trades, ledgers, snapshots, and closed-position tracking.

Add a `// ponytail:` comment above matching explaining no partial fills in v1.

- [ ] **Step 4: Wire startup**

Modify `main.go`:

```go
paperDB, err := openPaperDB(paperDBPath())
if err != nil {
	log.Printf("paper trading db init failed: %v", err)
} else {
	paperStore := NewPaperStore(paperDB)
	startPaperBackgroundTasks(paperStore, quotePaperFromTDX)
}
```

Place this before `ListenAndServe`. Keep server startup alive if paper DB fails; existing APIs should still run.

- [ ] **Step 5: Run matching tests**

```powershell
& "C:\Program Files\Go\bin\go.exe" test -vet=off ./cmd/server -run "TestMatchPaper|TestSellLastPosition|TestCreatePaperAccount"
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add cmd/server/paper_matcher.go cmd/server/paper_matcher_test.go cmd/server/main.go
git commit -m "feat: add paper order matching"
```

---

### Task 5: Paper HTTP APIs

**Files:**

- Create: `cmd/server/handlers_paper.go`
- Create: `cmd/server/handlers_paper_test.go`
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Write failing handler tests**

Test `GET /api/paper/accounts`:

```go
func TestHandlePaperAccountsReturnsAccounts(t *testing.T) {
	store := newFundedPaperStore(t, 100000)
	handler := handlePaperAccountsWithStore(store)
	req := httptest.NewRequest(http.MethodGet, "/api/paper/accounts", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp APIResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Code != 0 {
		t.Fatalf("resp=%+v", resp)
	}
}
```

Test bad account:

```go
func TestHandlePaperAccountRequiresAccountID(t *testing.T) {
	store := newTestPaperStore(t)
	handler := handlePaperAccountWithStore(store)
	req := httptest.NewRequest(http.MethodGet, "/api/paper/account", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```powershell
& "C:\Program Files\Go\bin\go.exe" test -vet=off ./cmd/server -run TestHandlePaper
```

Expected: FAIL because handlers are missing.

- [ ] **Step 3: Implement HTTP handlers**

Create handlers:

```go
func handlePaperDashboard(w http.ResponseWriter, r *http.Request)
func handlePaperAccounts(w http.ResponseWriter, r *http.Request)
func handlePaperAccount(w http.ResponseWriter, r *http.Request)
func handlePaperActivity(w http.ResponseWriter, r *http.Request)
func handlePaperClosedPositions(w http.ResponseWriter, r *http.Request)
```

Also add `WithStore` variants for tests:

```go
func handlePaperAccountsWithStore(store *PaperStore) http.HandlerFunc
```

HTTP rules:

- accept only `GET`.
- use existing `jsonResp` / `jsonErr`.
- no WebUI-side account calculations.
- `dashboard` may return empty arrays when no account exists.

- [ ] **Step 4: Register routes**

Modify `main.go`:

```go
mux.HandleFunc("/api/paper/dashboard", handlePaperDashboard)
mux.HandleFunc("/api/paper/accounts", handlePaperAccounts)
mux.HandleFunc("/api/paper/account", handlePaperAccount)
mux.HandleFunc("/api/paper/activity", handlePaperActivity)
mux.HandleFunc("/api/paper/closed-positions", handlePaperClosedPositions)
```

- [ ] **Step 5: Run handler tests**

```powershell
& "C:\Program Files\Go\bin\go.exe" test -vet=off ./cmd/server -run "TestHandlePaper|TestPaperDB"
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add cmd/server/handlers_paper.go cmd/server/handlers_paper_test.go cmd/server/main.go
git commit -m "feat: add paper dashboard api"
```

---

### Task 6: Paper MCP Tools

**Files:**

- Create: `cmd/server/paper_mcp.go`
- Create: `cmd/server/paper_mcp_test.go`
- Modify: `cmd/server/mcp.go`

- [ ] **Step 1: Write failing MCP schema tests**

```go
func TestPaperMCPToolsAreListed(t *testing.T) {
	seen := map[string]bool{}
	for _, tool := range mcpTools() {
		seen[tool.Name] = true
	}
	for _, name := range []string{
		"tdx_paper_account",
		"tdx_paper_order",
		"tdx_paper_portfolio",
		"tdx_paper_rules",
	} {
		if !seen[name] {
			t.Fatalf("missing tool %s", name)
		}
	}
}

func TestPaperOrderMCPSchemaDescribesEnums(t *testing.T) {
	tool := findMCPToolForTest("tdx_paper_order")
	props := tool.InputSchema["properties"].(map[string]any)
	for _, name := range []string{"action", "side", "orderType", "timeInForce"} {
		prop := props[name].(map[string]any)
		if prop["description"] == "" {
			t.Fatalf("%s missing description", name)
		}
		if len(prop["enum"].([]string)) == 0 {
			t.Fatalf("%s missing enum", name)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```powershell
& "C:\Program Files\Go\bin\go.exe" test -vet=off ./cmd/server -run TestPaperMCP
```

Expected: FAIL because tools are not registered.

- [ ] **Step 3: Add paper MCP tool definitions**

Create:

```go
func paperMCPTools() []mcpTool {
	return []mcpTool{
		newMCPTool("tdx_paper_account", "...", "/api/paper/mcp/account", handlePaperMCPAccount, ...),
		newMCPTool("tdx_paper_order", "...", "/api/paper/mcp/order", handlePaperMCPOrder, ...),
		newMCPTool("tdx_paper_portfolio", "...", "/api/paper/mcp/portfolio", handlePaperMCPPortfolio, ...),
		newMCPTool("tdx_paper_rules", "...", "/api/paper/mcp/rules", handlePaperMCPRules),
	}
}
```

Use clear Chinese descriptions and enums:

- account action: `create`, `list`, `get`, `close`, `recreate`.
- order action: `place`, `cancel`, `list`, `get`.
- side: `buy`, `sell`.
- orderType: `market`, `limit`, `auction`.
- timeInForce: `day`, `auction_only`.
- portfolio view: `summary`, `cash`, `positions`, `trades`, `orders`, `performance`, `closed_positions`, `actions`.

Modify `mcpTools()`:

```go
tools := []mcpTool{existing tools...}
tools = append(tools, paperMCPTools()...)
return tools
```

- [ ] **Step 4: Implement paper MCP HTTP adapters**

Handlers under `/api/paper/mcp/*` may be registered only for MCP internal calls or also exposed as HTTP. They must:

- parse query args from `callAgentHandlerAsMCP`.
- call `PaperStore`.
- write `jsonResp`.
- record `paper_agent_actions` for create/order/cancel/query calls.

- [ ] **Step 5: Run MCP tests**

```powershell
& "C:\Program Files\Go\bin\go.exe" test -vet=off ./cmd/server -run "TestPaperMCP|TestMCP"
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add cmd/server/paper_mcp.go cmd/server/paper_mcp_test.go cmd/server/mcp.go
git commit -m "feat: add paper trading mcp tools"
```

---

### Task 7: Replace WebUI with Read-Only Dashboard

**Files:**

- Modify: `cmd/server/static/index.html`
- Create: `cmd/server/static/styles.css`
- Create: `cmd/server/static/app.js`
- Modify: `cmd/server/handlers_ext.go`

- [ ] **Step 1: Replace HTML shell**

`index.html` should contain:

```html
<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>TDX Agent 模拟交易看板</title>
  <link rel="stylesheet" href="/static/styles.css">
</head>
<body>
  <main class="shell">
    <header class="topbar">
      <section>
        <p class="eyebrow">TDX Agent Paper Broker</p>
        <h1>模拟交易实验监控台</h1>
      </section>
      <section id="marketStatus" class="status-strip"></section>
    </header>
    <section id="accountCards" class="account-grid"></section>
    <section class="layout">
      <article class="panel wide">
        <div class="panel-head"><h2>资产曲线</h2></div>
        <canvas id="equityChart" height="260"></canvas>
      </article>
      <article class="panel"><div class="panel-head"><h2>A股市场概览</h2></div><div id="marketOverview"></div></article>
      <article class="panel wide"><div class="panel-head"><h2>当前持仓</h2></div><div id="positions"></div></article>
      <article class="panel"><div class="panel-head"><h2>委托与成交</h2></div><div id="ordersTrades"></div></article>
      <article class="panel wide"><div class="panel-head"><h2>清仓后表现</h2></div><div id="closedPositions"></div></article>
      <article class="panel wide"><div class="panel-head"><h2>Agent 行为时间线</h2></div><div id="activity"></div></article>
    </section>
  </main>
  <script src="/static/app.js"></script>
</body>
</html>
```

- [ ] **Step 2: Serve embedded static assets**

Modify `handleWebUI` or add a static handler so `/static/styles.css` and `/static/app.js` are served from `staticFiles`.

Keep `/` serving `index.html`.

- [ ] **Step 3: Add CSS**

Implement a dark, dense dashboard with CSS variables:

```css
:root {
  --bg: #0b0f14;
  --panel: #121821;
  --panel-2: #18202b;
  --text: #e6edf3;
  --muted: #8b98a8;
  --red: #ef5350;
  --green: #26a69a;
  --gold: #d6a85c;
  --line: #253041;
}
```

Use 8px or smaller radius. Avoid decorative orbs and gradients.

- [ ] **Step 4: Add JS polling**

`app.js` should:

- fetch `/api/paper/dashboard?range=20d`.
- fetch `/api/paper/activity?limit=80`.
- fetch `/api/paper/closed-positions?range=60d` when an account exists.
- render empty states when no account exists.
- draw equity curve on Canvas.

Use `fetch`, `Intl.NumberFormat`, and plain DOM APIs only.

- [ ] **Step 5: Run server and smoke check**

Run:

```powershell
$env:GOCACHE = "E:\project\tdx-api\.tmp\go-build"
$env:PORT = "18081"
& "C:\Program Files\Go\bin\go.exe" run ./cmd/server
```

Open:

```text
http://127.0.0.1:18081/
```

Expected:

- no old WebUI controls remain.
- dashboard loads.
- empty account state is readable.
- browser console has no missing `/static/*` errors.

- [ ] **Step 6: Commit**

```powershell
git add cmd/server/static/index.html cmd/server/static/styles.css cmd/server/static/app.js cmd/server/handlers_ext.go
git commit -m "feat: replace webui with paper dashboard"
```

---

### Task 8: Docs, End-to-End Verification, and Push Readiness

**Files:**

- Modify: `docs/agent_api_design.md`
- Create: `docs/agent_api_test_outputs/paper_mcp_tools.json`
- Create: `docs/agent_api_test_outputs/paper_dashboard.json`
- Create: `docs/agent_api_test_outputs/paper_account_create.json`

- [ ] **Step 1: Update design docs**

Add a section to `docs/agent_api_design.md`:

```markdown
## 模拟交易 MCP 与 WebUI

模拟交易使用独立数据库 `data/database/tdx-paper.sqlite`，可通过 `PAPER_DB_PATH` 覆盖。
WebUI 通过 `/api/paper/*` HTTP 聚合接口读取，只读展示，不提供人工交易按钮。

| MCP 工具 | 作用 | 关键参数 |
|---|---|---|
| `tdx_paper_account` | 创建、查询、关闭、重建模拟账户 | `action`, `accountId`, `name`, `initialCash`, `initialPositions` |
| `tdx_paper_order` | 提交委托、撤单、查询委托 | `action`, `accountId`, `code`, `side`, `orderType`, `price`, `quantity`, `timeInForce` |
| `tdx_paper_portfolio` | 查询资金、持仓、成交、收益、清仓后表现 | `accountId`, `view`, `from`, `to`, `limit`, `code` |
| `tdx_paper_rules` | 返回当前模拟交易规则 | 无 |
```

- [ ] **Step 2: Run full tests**

```powershell
$env:GOCACHE = "E:\project\tdx-api\.tmp\go-build"
& "C:\Program Files\Go\bin\go.exe" test -vet=off ./...
```

Expected: PASS.

- [ ] **Step 3: Start local server**

```powershell
$env:PORT = "18081"
$env:PAPER_DB_PATH = "E:\project\tdx-api\.tmp\paper-test.sqlite"
& "C:\Program Files\Go\bin\go.exe" run ./cmd/server
```

Expected: server listens on `:18081`.

- [ ] **Step 4: Capture MCP tool list**

POST to `/mcp`:

```json
{"jsonrpc":"2.0","id":1,"method":"tools/list"}
```

Save response to:

```text
docs/agent_api_test_outputs/paper_mcp_tools.json
```

Expected: includes `tdx_paper_account`, `tdx_paper_order`, `tdx_paper_portfolio`, `tdx_paper_rules`.

- [ ] **Step 5: Create a sample account through MCP**

Call `tdx_paper_account` with:

```json
{
  "action": "create",
  "name": "测试账户",
  "initialCash": 100000,
  "initialPositions": [
    {"code": "300499", "quantity": 200, "costPrice": 10, "buyDate": "2026-06-01"}
  ],
  "note": "Codex verification account"
}
```

Save response to:

```text
docs/agent_api_test_outputs/paper_account_create.json
```

- [ ] **Step 6: Capture dashboard JSON**

GET:

```text
http://127.0.0.1:18081/api/paper/dashboard?range=20d
```

Save response to:

```text
docs/agent_api_test_outputs/paper_dashboard.json
```

Expected:

- response `code=0`.
- includes market status.
- includes account cards.
- includes empty or populated curves without frontend-side calculation.

- [ ] **Step 7: Commit**

```powershell
git add docs/agent_api_design.md docs/agent_api_test_outputs/paper_mcp_tools.json docs/agent_api_test_outputs/paper_dashboard.json docs/agent_api_test_outputs/paper_account_create.json
git commit -m "docs: document paper trading tools"
```

---

## Self-Review Checklist

- Spec coverage:
  - Independent SQLite: Task 1.
  - Account lifecycle: Task 2.
  - Orders and fees: Task 3.
  - Matching, snapshots, closed tracking: Task 4.
  - WebUI HTTP APIs: Task 5.
  - MCP tools: Task 6.
  - Read-only WebUI: Task 7.
  - Docs and sample outputs: Task 8.

- Deliberate simplifications:
  - No partial fills in v1.
  - No WebSocket in v1.
  - No frontend build chain.
  - No real brokerage integration.
  - No auth for LAN deployment.

- Required verification:
  - `go test -vet=off ./cmd/server`
  - `go test -vet=off ./...`
  - Local dashboard smoke check.
  - MCP `tools/list` contains paper tools.

package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

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
			t.Fatalf("%s missing from mcpTools()", name)
		}
	}
}

func TestPaperOrderMCPSchemaDescribesImmediateExecution(t *testing.T) {
	tool := findPaperMCPTool(t, "tdx_paper_order")
	properties := tool.InputSchema["properties"].(map[string]any)

	assertMCPEnum(t, properties, "action", "place", "cancel", "list", "get")
	assertMCPEnum(t, properties, "side", "buy", "sell")
	for _, name := range []string{"orderType", "timeInForce"} {
		if _, ok := properties[name]; ok {
			t.Fatalf("legacy matching property %s should not be agent-visible", name)
		}
	}

	quantity := properties["quantity"].(map[string]any)
	if quantity["type"] != "integer" || quantity["minimum"] != 100 ||
		quantity["multipleOf"] != 100 {
		t.Fatalf("quantity schema = %+v", quantity)
	}
	assetType := properties["assetType"].(map[string]any)
	if assetType["default"] != "stock" ||
		!strings.Contains(assetType["description"].(string), "默认 stock") {
		t.Fatalf("assetType schema = %+v", assetType)
	}
	price := properties["price"].(map[string]any)
	if price["type"] != "number" || price["exclusiveMinimum"] != 0 {
		t.Fatalf("price schema = %+v", price)
	}

	required := tool.InputSchema["required"].([]string)
	if !hasString(required, "action") || !hasString(required, "accountId") {
		t.Fatalf("required = %+v, want action and accountId", required)
	}
	conditions := tool.InputSchema["allOf"].([]map[string]any)
	if len(conditions) == 0 {
		t.Fatal("paper order conditional schema missing")
	}
	placeRequired := conditions[0]["then"].(map[string]any)["required"].([]string)
	for _, name := range []string{"code", "side", "price", "quantity"} {
		if !hasString(placeRequired, name) {
			t.Fatalf("place required = %+v, missing %s", placeRequired, name)
		}
	}
	conditionJSON, err := json.Marshal(conditions)
	if err != nil {
		t.Fatal(err)
	}
	conditionText := string(conditionJSON)
	for _, unwanted := range []string{"auction", "auction_only", "timeInForce"} {
		if strings.Contains(conditionText, unwanted) {
			t.Fatalf("legacy matching condition %q remains: %s", unwanted, conditionText)
		}
	}
}

func TestPaperAccountMCPSchemaRequiresConfirmForSideEffects(t *testing.T) {
	tool := findPaperMCPTool(t, "tdx_paper_account")
	properties := tool.InputSchema["properties"].(map[string]any)
	assertMCPEnum(
		t,
		properties,
		"action",
		"create",
		"list",
		"get",
		"set_position",
		"delete",
		"close",
		"recreate",
	)
	confirm := properties["confirm"].(map[string]any)
	if confirm["type"] != "boolean" {
		t.Fatalf("confirm schema = %+v", confirm)
	}
	note := properties["note"].(map[string]any)
	if !strings.Contains(note["description"].(string), "不参与资金和持仓计算") {
		t.Fatalf("note schema = %+v", note)
	}
	conditions := tool.InputSchema["allOf"].([]map[string]any)
	if len(conditions) == 0 {
		t.Fatal("paper account conditional schema missing")
	}
	conditionJSON, err := json.Marshal(conditions)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(conditionJSON), `"delete"`) {
		t.Fatalf("delete confirmation condition missing: %s", conditionJSON)
	}
	if !strings.Contains(string(conditionJSON), `"required":["accountId"]`) {
		t.Fatalf("accountId condition missing: %s", conditionJSON)
	}
	if !strings.Contains(string(conditionJSON), `"required":["accountId","position"]`) {
		t.Fatalf("set_position condition missing: %s", conditionJSON)
	}
	position := properties["position"].(map[string]any)
	positionProperties := position["properties"].(map[string]any)
	quantity := positionProperties["quantity"].(map[string]any)
	if quantity["type"] != "integer" || quantity["minimum"] != 0 {
		t.Fatalf("position quantity schema = %+v", quantity)
	}
	positionJSON, err := json.Marshal(position)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(positionJSON), `"required":["costPrice"]`) {
		t.Fatalf("positive position costPrice condition missing: %s", positionJSON)
	}
}

func TestPaperPortfolioMCPSchemaDescriptions(t *testing.T) {
	tool := findPaperMCPTool(t, "tdx_paper_portfolio")
	properties := tool.InputSchema["properties"].(map[string]any)
	if !strings.Contains(tool.Description, "交易决策前") ||
		!strings.Contains(tool.Description, "positions") ||
		!strings.Contains(tool.Description, "orders") {
		t.Fatalf("portfolio description = %q", tool.Description)
	}

	accountID := properties["accountId"].(map[string]any)
	if !strings.Contains(accountID["description"].(string), "查询账户视图时必填") {
		t.Fatalf("accountId schema = %+v", accountID)
	}
	view := properties["view"].(map[string]any)
	if !strings.Contains(view["description"].(string), "summary/cash/positions") {
		t.Fatalf("view schema = %+v", view)
	}
	code := properties["code"].(map[string]any)
	if !strings.Contains(code["description"].(string), "仅返回该证券相关记录") {
		t.Fatalf("code schema = %+v", code)
	}
	for _, name := range []string{"from", "to"} {
		property := properties[name].(map[string]any)
		if property["pattern"] != `^(\d{4}-\d{2}-\d{2}|\d{8})$` ||
			!strings.Contains(property["description"].(string), "按字符串日期过滤") {
			t.Fatalf("%s schema = %+v", name, property)
		}
	}
}

func TestPaperRulesDescribeImmediateExecution(t *testing.T) {
	result, err := callMCPTool(mustMCPParams(t, "tdx_paper_rules", map[string]any{}))
	if err != nil {
		t.Fatal(err)
	}
	content := result["content"].([]map[string]string)
	if len(content) != 1 || !strings.Contains(content[0]["text"], "指定价格立即成交") ||
		strings.Contains(content[0]["text"], "30 秒") {
		t.Fatalf("content = %+v", content)
	}
	structured := result["structuredContent"].(map[string]any)
	rules := structured["rules"].(map[string]any)
	execution := fmt.Sprint(rules["execution"])
	if !strings.Contains(execution, "price") ||
		!strings.Contains(execution, "positions") ||
		!strings.Contains(execution, "orders") {
		t.Fatalf("execution rules = %s", execution)
	}
}

func TestPaperMCPRulesReturnsContent(t *testing.T) {
	result, err := callMCPTool(mustMCPParams(t, "tdx_paper_rules", map[string]any{}))
	if err != nil {
		t.Fatal(err)
	}

	content := result["content"].([]map[string]string)
	if len(content) != 1 || content[0]["type"] != "text" ||
		!strings.Contains(content[0]["text"], "纸上交易规则") {
		t.Fatalf("content = %+v", content)
	}
	structured := result["structuredContent"].(map[string]any)
	if structured["rules"] == nil {
		t.Fatalf("structuredContent = %+v", structured)
	}
}

func TestPaperMCPAccountCreateWithInitialPositions(t *testing.T) {
	store := newTestPaperStore(t)
	withPaperMCPStore(t, store)

	result, err := callMCPTool(mustMCPParams(t, "tdx_paper_account", map[string]any{
		"action":      "create",
		"name":        "alpha",
		"initialCash": 10000,
		"confirm":     true,
		"initialPositions": []map[string]any{
			{
				"code":      "600000",
				"name":      "浦发银行",
				"quantity":  200,
				"costPrice": 10.5,
			},
		},
	}))
	if err != nil {
		t.Fatal(err)
	}

	structured := result["structuredContent"].(map[string]any)
	account := structured["account"].(PaperAccount)
	if account.Name != "alpha" || account.InitialCash != 10000 {
		t.Fatalf("account = %+v", account)
	}

	positions := structured["positions"].([]PaperPosition)
	if len(positions) != 1 {
		t.Fatalf("len(positions) = %d, want 1", len(positions))
	}
	position := positions[0]
	if position.Code != "600000" || position.Quantity != 200 ||
		position.AvgCost != 10.5 {
		t.Fatalf("position = %+v", position)
	}

	persisted, err := store.ListPositions(account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted) != 1 || persisted[0].Code != "600000" {
		t.Fatalf("persisted positions = %+v", persisted)
	}
}

func TestPaperMCPAccountDeleteRemovesOnlySelectedAccountData(t *testing.T) {
	store := newTestPaperStore(t)
	withPaperMCPStore(t, store)

	deleted, err := store.CreateAccount(PaperCreateAccountRequest{
		Name:        "deleted",
		InitialCash: 10000,
		InitialPositions: []PaperInitialPosition{
			{Code: "600000", Quantity: 100, CostPrice: 10},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	survivor, err := store.CreateAccount(PaperCreateAccountRequest{
		Name:        "survivor",
		InitialCash: 20000,
	})
	if err != nil {
		t.Fatal(err)
	}
	order, err := store.PlaceOrder(PaperPlaceOrderRequest{
		AccountID:   deleted.ID,
		Code:        "600000",
		Side:        paperSideBuy,
		OrderType:   paperOrderLimit,
		TimeInForce: paperTimeInForceDay,
		Price:       9,
		Quantity:    100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.FillOrder(order.ID, PaperQuote{Code: "600000", Price: 9}); err != nil {
		t.Fatal(err)
	}
	seedPaperAccountDeletionRows(t, store.db, deleted.ID)

	if _, err := callMCPTool(mustMCPParams(t, "tdx_paper_account", map[string]any{
		"action":    "delete",
		"accountId": deleted.ID,
	})); err == nil {
		t.Fatal("delete without confirm error = nil, want error")
	}

	result, err := callMCPTool(mustMCPParams(t, "tdx_paper_account", map[string]any{
		"action":    "delete",
		"accountId": deleted.ID,
		"confirm":   true,
	}))
	if err != nil {
		t.Fatal(err)
	}
	structured := result["structuredContent"].(map[string]any)
	if structured["accountId"] != deleted.ID {
		t.Fatalf("structuredContent = %+v", structured)
	}
	if _, err := store.GetAccount(deleted.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetAccount(deleted) error = %v, want sql.ErrNoRows", err)
	}
	if got, err := store.GetAccount(survivor.ID); err != nil || got.Name != "survivor" {
		t.Fatalf("GetAccount(survivor) = %+v, %v", got, err)
	}

	accountDataTables := []string{
		"paper_account_initial_positions",
		"paper_positions",
		"paper_orders",
		"paper_trades",
		"paper_cash_ledger",
		"paper_position_ledger",
		"paper_agent_actions",
		"paper_account_snapshots",
		"paper_closed_positions",
		"paper_closed_position_tracking",
	}
	for _, table := range accountDataTables {
		var count int
		query := "SELECT COUNT(*) FROM " + table + " WHERE account_id = ?"
		if err := store.db.QueryRow(query, deleted.ID).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("%s rows for deleted account = %d, want 0", table, count)
		}
	}
}

func TestPaperMCPPlaceExecutesImmediatelyAtProvidedPrice(t *testing.T) {
	store := newTestPaperStore(t)
	withPaperMCPStore(t, store)
	account, err := store.CreateAccount(PaperCreateAccountRequest{
		Name:        "recorder",
		InitialCash: 20000,
	})
	if err != nil {
		t.Fatal(err)
	}

	buyResult, err := callMCPTool(mustMCPParams(t, "tdx_paper_order", map[string]any{
		"action":    "place",
		"accountId": account.ID,
		"code":      "600000",
		"name":      "浦发银行",
		"side":      "buy",
		"price":     10,
		"quantity":  100,
		"reason":    "记录买入",
		"confirm":   true,
	}))
	if err != nil {
		t.Fatal(err)
	}
	buyData := buyResult["structuredContent"].(map[string]any)
	buyOrder, ok := buyData["order"].(PaperOrder)
	if !ok || buyOrder.Status != paperOrderFilled || buyOrder.FilledQuantity != 100 {
		t.Fatalf("buy order = %+v", buyData["order"])
	}
	buyTrade, ok := buyData["trade"].(PaperTrade)
	if !ok || buyTrade.Price != 10 || buyTrade.Quantity != 100 {
		t.Fatalf("buy trade = %+v", buyData["trade"])
	}
	positions, err := store.ListPositions(account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(positions) != 1 || positions[0].Quantity != 100 ||
		positions[0].SellableQuantity != 100 || positions[0].FrozenQuantity != 0 {
		t.Fatalf("positions after buy = %+v", positions)
	}
	if _, err := store.db.Exec(`
		UPDATE paper_positions SET sellable_quantity = 0 WHERE account_id = ?
	`, account.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.MakeAllPositionsSellable(); err != nil {
		t.Fatal(err)
	}

	sellResult, err := callMCPTool(mustMCPParams(t, "tdx_paper_order", map[string]any{
		"action":    "place",
		"accountId": account.ID,
		"code":      "600000",
		"name":      "浦发银行",
		"side":      "sell",
		"price":     11,
		"quantity":  100,
		"reason":    "记录卖出",
		"confirm":   true,
	}))
	if err != nil {
		t.Fatal(err)
	}
	sellData := sellResult["structuredContent"].(map[string]any)
	sellOrder, ok := sellData["order"].(PaperOrder)
	if !ok || sellOrder.Status != paperOrderFilled {
		t.Fatalf("sell order = %+v", sellData["order"])
	}
	positions, err = store.ListPositions(account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(positions) != 0 {
		t.Fatalf("positions after sell = %+v", positions)
	}
	updated, err := store.GetAccount(account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.AvailableCash <= account.InitialCash || updated.FrozenCash != 0 {
		t.Fatalf("account after round trip = %+v", updated)
	}
	assertPaperRowCount(t, store.db, "paper_trades", 2)
	assertPaperRowCount(t, store.db, "paper_account_snapshots", 2)
}

func TestPaperMCPSetPositionAddsUpdatesAndDeletesWithoutChangingCash(t *testing.T) {
	store := newTestPaperStore(t)
	withPaperMCPStore(t, store)
	account, err := store.CreateAccount(PaperCreateAccountRequest{
		Name:        "position recorder",
		InitialCash: 20000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := callMCPTool(mustMCPParams(t, "tdx_paper_account", map[string]any{
		"action":    "set_position",
		"accountId": account.ID,
		"position": map[string]any{
			"code":     "600000",
			"quantity": 100,
		},
		"confirm": true,
	})); err == nil {
		t.Fatal("set_position without costPrice error = nil, want error")
	}

	assertSetPositionResult(t, account.ID, setPositionExpectation{
		position: map[string]any{
			"code":         "600000",
			"securityName": "浦发银行",
			"quantity":     150,
			"costPrice":    10,
		},
		operation: "added",
		quantity:  150,
		costPrice: 10,
	})
	if _, err := store.db.Exec(`
		UPDATE paper_positions
		SET sellable_quantity = 140, frozen_quantity = 10
		WHERE account_id = ? AND code = ?
	`, account.ID, "600000"); err != nil {
		t.Fatal(err)
	}
	if _, err := callMCPTool(mustMCPParams(t, "tdx_paper_account", map[string]any{
		"action":    "set_position",
		"accountId": account.ID,
		"position": map[string]any{
			"code":      "600000",
			"quantity":  80,
			"costPrice": 12,
		},
		"confirm": true,
	})); err == nil {
		t.Fatal("set_position with frozen quantity error = nil, want error")
	}
	if _, err := store.db.Exec(`
		UPDATE paper_positions
		SET sellable_quantity = quantity, frozen_quantity = 0
		WHERE account_id = ? AND code = ?
	`, account.ID, "600000"); err != nil {
		t.Fatal(err)
	}
	assertSetPositionResult(t, account.ID, setPositionExpectation{
		position: map[string]any{
			"code":         "600000",
			"securityName": "浦发银行",
			"quantity":     80,
			"costPrice":    12,
		},
		operation: "updated",
		quantity:  80,
		costPrice: 12,
	})
	assertSetPositionResult(t, account.ID, setPositionExpectation{
		position: map[string]any{
			"code":     "600000",
			"quantity": 0,
		},
		operation: "deleted",
	})

	positions, err := store.ListPositions(account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(positions) != 0 {
		t.Fatalf("positions = %+v, want none", positions)
	}
	updated, err := store.GetAccount(account.ID)
	if err != nil {
		t.Fatal(err)
	}
	assertFloatEqual(t, updated.AvailableCash, account.AvailableCash)
	assertFloatEqual(t, updated.FrozenCash, account.FrozenCash)
	assertPaperRowCount(t, store.db, "paper_orders", 0)
	assertPaperRowCount(t, store.db, "paper_trades", 0)
	assertPaperRowCount(t, store.db, "paper_position_ledger", 3)
	assertPaperRowCount(t, store.db, "paper_account_snapshots", 3)
	assertPaperRowCount(t, store.db, "paper_agent_actions", 4)
}

type setPositionExpectation struct {
	position  map[string]any
	operation string
	quantity  int64
	costPrice float64
}

func assertSetPositionResult(
	t *testing.T,
	accountID string,
	want setPositionExpectation,
) {
	t.Helper()
	result, err := callMCPTool(mustMCPParams(t, "tdx_paper_account", map[string]any{
		"action":    "set_position",
		"accountId": accountID,
		"position":  want.position,
		"reason":    "人工校正持仓",
		"confirm":   true,
	}))
	if err != nil {
		t.Fatal(err)
	}
	data := result["structuredContent"].(map[string]any)
	if data["operation"] != want.operation {
		t.Fatalf("operation = %v, want %s", data["operation"], want.operation)
	}
	got, ok := data["position"].(PaperPosition)
	if !ok || got.Quantity != want.quantity || got.AvgCost != want.costPrice {
		t.Fatalf("position = %+v", data["position"])
	}
}

func seedPaperAccountDeletionRows(t *testing.T, db *sql.DB, accountID string) {
	t.Helper()
	now := "2026-07-18T15:00:00+08:00"
	if _, err := db.Exec(`
		INSERT INTO paper_account_snapshots (
			id, account_id, trading_day, total_assets, cash_available,
			cash_frozen, market_value, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, "snapshot-delete", accountID, "2026-07-18", 10000, 1000, 0, 9000, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO paper_closed_positions (
			id, account_id, code, quantity, open_amount, close_amount,
			realized_pnl, closed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, "closed-delete", accountID, "600000", 100, 1000, 1100, 100, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO paper_closed_position_tracking (
			id, closed_position_id, account_id, code, trading_day, price, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`, "tracking-delete", "closed-delete", accountID, "600000", "2026-07-18", 11, now); err != nil {
		t.Fatal(err)
	}
}

func TestPaperMCPCancelLimitBuyReleasesCash(t *testing.T) {
	store := newTestPaperStore(t)
	withPaperMCPStore(t, store)
	account, err := store.CreateAccount(PaperCreateAccountRequest{
		Name:        "buyer",
		InitialCash: 20000,
	})
	if err != nil {
		t.Fatal(err)
	}
	order, err := store.PlaceOrder(PaperPlaceOrderRequest{
		AccountID: account.ID,
		Code:      "600000",
		Side:      paperSideBuy,
		OrderType: paperOrderLimit,
		Price:     10,
		Quantity:  100,
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := callMCPTool(mustMCPParams(t, "tdx_paper_order", map[string]any{
		"action":    "cancel",
		"accountId": account.ID,
		"orderId":   order.ID,
		"confirm":   true,
	}))
	if err != nil {
		t.Fatal(err)
	}

	cancelled := result["structuredContent"].(map[string]any)["order"].(PaperOrder)
	if cancelled.Status != paperOrderCancelled || cancelled.CancelledAt == "" {
		t.Fatalf("cancelled order = %+v", cancelled)
	}
	got, err := store.GetAccount(account.ID)
	if err != nil {
		t.Fatal(err)
	}
	assertFloatEqual(t, got.AvailableCash, 20000)
	assertFloatEqual(t, got.FrozenCash, 0)
	assertPaperRowCount(t, store.db, "paper_agent_actions", 3)
}

func TestPaperMCPCancelLimitSellReleasesPosition(t *testing.T) {
	store := newTestPaperStore(t)
	withPaperMCPStore(t, store)
	account, err := store.CreateAccount(PaperCreateAccountRequest{
		Name: "seller",
		InitialPositions: []PaperInitialPosition{
			{Code: "600000", Quantity: 200, CostPrice: 10},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	order, err := store.PlaceOrder(PaperPlaceOrderRequest{
		AccountID: account.ID,
		Code:      "600000",
		Side:      paperSideSell,
		OrderType: paperOrderLimit,
		Price:     11,
		Quantity:  100,
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := callMCPTool(mustMCPParams(t, "tdx_paper_order", map[string]any{
		"action":    "cancel",
		"accountId": account.ID,
		"orderId":   order.ID,
		"confirm":   true,
	})); err != nil {
		t.Fatal(err)
	}

	positions, err := store.ListPositions(account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(positions) != 1 {
		t.Fatalf("len(positions) = %d, want 1", len(positions))
	}
	if positions[0].SellableQuantity != 200 || positions[0].FrozenQuantity != 0 {
		t.Fatalf("position = %+v", positions[0])
	}
}

func TestPaperMCPCancelRejectsFilledOrder(t *testing.T) {
	store := newTestPaperStore(t)
	withPaperMCPStore(t, store)
	account, err := store.CreateAccount(PaperCreateAccountRequest{
		Name:        "buyer",
		InitialCash: 20000,
	})
	if err != nil {
		t.Fatal(err)
	}
	order, err := store.PlaceOrder(PaperPlaceOrderRequest{
		AccountID: account.ID,
		Code:      "600000",
		Side:      paperSideBuy,
		OrderType: paperOrderMarket,
		Quantity:  100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.FillOrder(order.ID, PaperQuote{Code: "600000", Price: 10}); err != nil {
		t.Fatal(err)
	}

	if _, err := callMCPTool(mustMCPParams(t, "tdx_paper_order", map[string]any{
		"action":    "cancel",
		"accountId": account.ID,
		"orderId":   order.ID,
		"confirm":   true,
	})); err == nil {
		t.Fatal("cancel filled order error = nil, want error")
	}

	got, err := store.GetOrder(order.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != paperOrderFilled {
		t.Fatalf("order = %+v", got)
	}
}

func findPaperMCPTool(t *testing.T, name string) mcpTool {
	t.Helper()

	for _, tool := range mcpTools() {
		if tool.Name == name {
			return tool
		}
	}
	t.Fatalf("%s missing", name)
	return mcpTool{}
}

func assertMCPEnum(
	t *testing.T,
	properties map[string]any,
	name string,
	want ...string,
) {
	t.Helper()

	property := properties[name].(map[string]any)
	got := property["enum"].([]string)
	for _, value := range want {
		if !hasString(got, value) {
			t.Fatalf("%s enum = %+v, missing %s", name, got, value)
		}
	}
}

func hasString(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func mustMCPParams(
	t *testing.T,
	name string,
	args map[string]any,
) json.RawMessage {
	t.Helper()

	raw, err := json.Marshal(map[string]any{
		"name":      name,
		"arguments": args,
	})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func withPaperMCPStore(t *testing.T, store *PaperStore) {
	t.Helper()

	old := paperStore
	paperStore = store
	t.Cleanup(func() {
		paperStore = old
	})
}

package main

import (
	"encoding/json"
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

func TestPaperOrderMCPSchemaDescribesEnums(t *testing.T) {
	tool := findPaperMCPTool(t, "tdx_paper_order")
	properties := tool.InputSchema["properties"].(map[string]any)

	assertMCPEnum(t, properties, "action", "place", "cancel", "list", "get")
	assertMCPEnum(t, properties, "side", "buy", "sell")
	assertMCPEnum(t, properties, "orderType", "market", "limit", "auction")
	assertMCPEnum(t, properties, "timeInForce", "day", "auction_only")

	required := tool.InputSchema["required"].([]string)
	if !hasString(required, "action") || !hasString(required, "accountId") {
		t.Fatalf("required = %+v, want action and accountId", required)
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

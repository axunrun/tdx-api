package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlePaperAccountsReturnsAccounts(t *testing.T) {
	store := newTestPaperStore(t)
	account, err := store.CreateAccount(PaperCreateAccountRequest{
		Name:        "alpha",
		InitialCash: 100,
	})
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/paper/accounts", nil)
	handlePaperAccountsWithStore(store)(rec, req)

	data := decodePaperResponse(t, rec, http.StatusOK)
	items := data["items"].([]any)
	if data["count"].(float64) != 1 || len(items) != 1 {
		t.Fatalf("data = %+v", data)
	}
	if items[0].(map[string]any)["id"] != account.ID {
		t.Fatalf("items = %+v, want account %s", items, account.ID)
	}
}

func TestHandlePaperAccountRequiresAccountID(t *testing.T) {
	store := newTestPaperStore(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/paper/account", nil)
	handlePaperAccountWithStore(store)(rec, req)

	resp := decodePaperEnvelope(t, rec, http.StatusBadRequest)
	if resp.Message != "accountId is required" {
		t.Fatalf("message = %q", resp.Message)
	}
}

func TestHandlePaperDashboardReturnsEmptyState(t *testing.T) {
	store := newTestPaperStore(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/paper/dashboard", nil)
	handlePaperDashboardWithStore(store)(rec, req)

	data := decodePaperResponse(t, rec, http.StatusOK)
	marketStatus := data["marketStatus"].(map[string]any)
	if marketStatus["status"] != "unknown" {
		t.Fatalf("marketStatus = %+v", marketStatus)
	}
	if data["selectedAccount"] != nil {
		t.Fatalf("selectedAccount = %+v, want nil", data["selectedAccount"])
	}
	if data["updatedAt"] == "" {
		t.Fatal("updatedAt is empty")
	}
}

func TestHandlePaperDashboardReturnsAllEquityCurves(t *testing.T) {
	store := newTestPaperStore(t)
	for _, name := range []string{"alpha", "beta"} {
		account, err := store.CreateAccount(PaperCreateAccountRequest{
			Name:        name,
			InitialCash: 1000,
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := store.CreateSnapshot(account.ID, fixedPaperQuote(10)); err != nil {
			t.Fatal(err)
		}
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/paper/dashboard", nil)
	handlePaperDashboardWithStore(store)(rec, req)

	data := decodePaperResponse(t, rec, http.StatusOK)
	curves := data["equityCurves"].([]any)
	if len(curves) != 2 {
		t.Fatalf("len(equityCurves) = %d, want 2", len(curves))
	}
	for _, item := range curves {
		points := item.(map[string]any)["points"].([]any)
		if len(points) != 1 {
			t.Fatalf("points = %+v, want one point", points)
		}
	}
}

func TestHandlePaperActivityFiltersAccount(t *testing.T) {
	store := newTestPaperStore(t)
	first, err := store.CreateAccount(PaperCreateAccountRequest{
		Name:        "alpha",
		InitialCash: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateAccount(PaperCreateAccountRequest{
		Name:        "beta",
		InitialCash: 100,
	}); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/paper/activity?accountId="+first.ID,
		nil,
	)
	handlePaperActivityWithStore(store)(rec, req)

	data := decodePaperResponse(t, rec, http.StatusOK)
	items := data["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if items[0].(map[string]any)["accountId"] != first.ID {
		t.Fatalf("items = %+v, want account %s", items, first.ID)
	}
}

func TestFilterPaperEquityPointsUsesDailyRange(t *testing.T) {
	points := []PaperEquityPoint{
		{TradingDay: "2026-01-01", TotalAssets: 1, CreatedAt: "a"},
		{TradingDay: "2026-01-01", TotalAssets: 2, CreatedAt: "b"},
		{TradingDay: "2026-01-02", TotalAssets: 3, CreatedAt: "c"},
		{TradingDay: "2026-01-03", TotalAssets: 4, CreatedAt: "d"},
	}

	got := filterPaperEquityPoints(points, "2")

	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0].TradingDay != "2026-01-02" || got[1].TradingDay != "2026-01-03" {
		t.Fatalf("got = %+v", got)
	}
	all := filterPaperEquityPoints(points, "all")
	if len(all) != 3 || all[0].TotalAssets != 2 {
		t.Fatalf("all = %+v", all)
	}
}

func TestListPaperEquityCurveIncludesTradeSummary(t *testing.T) {
	store := newTestPaperStore(t)
	account, err := store.CreateAccount(PaperCreateAccountRequest{
		Name:        "alpha",
		InitialCash: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSnapshot(account.ID, fixedPaperQuote(10)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`
		INSERT INTO paper_trades (
			id, order_id, account_id, code, side, price, quantity,
			amount, commission, stamp_tax, transfer_fee, traded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "trade_test", "order_test", account.ID, "300499", paperSideBuy,
		10.5, 200, 2100, 0, 0, 0, "2026-01-01T10:00:00+08:00"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`
		UPDATE paper_account_snapshots
		SET trading_day = ?
		WHERE account_id = ?
	`, "2026-01-01", account.ID); err != nil {
		t.Fatal(err)
	}

	points, err := listPaperEquityCurve(store, account.ID, "all")
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 1 {
		t.Fatalf("len(points) = %d, want 1", len(points))
	}
	if points[0].BuyQuantity != 200 || points[0].BuyAmount != 2100 {
		t.Fatalf("point = %+v", points[0])
	}
}

func TestHandlePaperClosedPositionsRequiresAccountID(t *testing.T) {
	store := newTestPaperStore(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/paper/closed-positions", nil)
	handlePaperClosedPositionsWithStore(store)(rec, req)

	resp := decodePaperEnvelope(t, rec, http.StatusBadRequest)
	if resp.Message != "accountId is required" {
		t.Fatalf("message = %q", resp.Message)
	}
}

func decodePaperResponse(
	t *testing.T,
	rec *httptest.ResponseRecorder,
	wantStatus int,
) map[string]any {
	t.Helper()

	resp := decodePaperEnvelope(t, rec, wantStatus)
	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("data = %T, want object", resp.Data)
	}
	return data
}

func decodePaperEnvelope(
	t *testing.T,
	rec *httptest.ResponseRecorder,
	wantStatus int,
) APIResponse {
	t.Helper()

	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, wantStatus, rec.Body)
	}
	var resp APIResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	return resp
}

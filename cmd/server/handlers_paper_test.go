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

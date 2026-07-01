package main

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func newTestPaperStore(t *testing.T) *PaperStore {
	t.Helper()

	db, err := openPaperDB(filepath.Join(t.TempDir(), "paper.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
	})
	return NewPaperStore(db)
}

func TestCreatePaperAccountLocksInitialState(t *testing.T) {
	store := newTestPaperStore(t)

	account, err := store.CreateAccount(PaperCreateAccountRequest{
		Name:        "alpha",
		InitialCash: 10000,
		Note:        "test account",
		InitialPositions: []PaperInitialPosition{
			{Code: "600000", Name: "浦发银行", Quantity: 100, CostPrice: 10.5},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := store.GetAccount(account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "alpha" || got.Status != paperAccountActive {
		t.Fatalf("account = %+v", got)
	}
	if got.InitialCash != 10000 || got.AvailableCash != 10000 || got.FrozenCash != 0 {
		t.Fatalf("cash fields = %+v", got)
	}

	positions, err := store.ListPositions(account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(positions) != 1 {
		t.Fatalf("len(positions) = %d, want 1", len(positions))
	}
	position := positions[0]
	if position.Code != "600000" || position.AssetType != paperAssetStock {
		t.Fatalf("position identity = %+v", position)
	}
	if position.Quantity != 100 || position.SellableQuantity != 100 ||
		position.FrozenQuantity != 0 || position.AvgCost != 10.5 {
		t.Fatalf("position state = %+v", position)
	}

	assertPaperRowCount(t, store.db, "paper_cash_ledger", 1)
	assertPaperRowCount(t, store.db, "paper_account_initial_positions", 1)
	assertPaperRowCount(t, store.db, "paper_position_ledger", 1)
	assertPaperRowCount(t, store.db, "paper_agent_actions", 1)
}

func TestCreatePaperAccountRejectsInvalidCash(t *testing.T) {
	store := newTestPaperStore(t)

	if _, err := store.CreateAccount(PaperCreateAccountRequest{
		Name:        "bad cash",
		InitialCash: -1,
	}); err == nil {
		t.Fatal("CreateAccount() error = nil, want error")
	}

	assertPaperRowCount(t, store.db, "paper_accounts", 0)
}

func TestCreatePaperAccountRejectsInvalidPosition(t *testing.T) {
	store := newTestPaperStore(t)

	tests := []PaperInitialPosition{
		{Quantity: 1, CostPrice: 1},
		{Code: "600000", Quantity: 0, CostPrice: 1},
		{Code: "600000", Quantity: 1, CostPrice: -1},
	}
	for _, position := range tests {
		if _, err := store.CreateAccount(PaperCreateAccountRequest{
			Name:             "bad position",
			InitialPositions: []PaperInitialPosition{position},
		}); err == nil {
			t.Fatalf("CreateAccount(%+v) error = nil, want error", position)
		}
	}

	assertPaperRowCount(t, store.db, "paper_accounts", 0)
}

func TestListPaperAccounts(t *testing.T) {
	store := newTestPaperStore(t)

	first, err := store.CreateAccount(PaperCreateAccountRequest{Name: "first"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.CreateAccount(PaperCreateAccountRequest{Name: "second", InitialCash: 20})
	if err != nil {
		t.Fatal(err)
	}

	accounts, err := store.ListAccounts()
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 2 {
		t.Fatalf("len(accounts) = %d, want 2", len(accounts))
	}
	if accounts[0].ID != first.ID || accounts[1].ID != second.ID {
		t.Fatalf("accounts = %+v", accounts)
	}
}

func assertPaperRowCount(t *testing.T, db *sql.DB, table string, want int) {
	t.Helper()

	var got int
	if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("%s row count = %d, want %d", table, got, want)
	}
}

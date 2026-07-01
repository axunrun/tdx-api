package main

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestPaperDBPathUsesDedicatedDefault(t *testing.T) {
	t.Setenv("PAPER_DB_PATH", "")

	want := filepath.FromSlash(defaultPaperDBPath)
	if got := paperDBPath(); got != want {
		t.Fatalf("paperDBPath() = %q, want %q", got, want)
	}
}

func TestPaperDBPathUsesEnvOverride(t *testing.T) {
	want := filepath.Join(t.TempDir(), "paper.sqlite")
	t.Setenv("PAPER_DB_PATH", want)

	if got := paperDBPath(); got != want {
		t.Fatalf("paperDBPath() = %q, want %q", got, want)
	}
}

func TestInitPaperDBCreatesTables(t *testing.T) {
	db, err := openPaperDB(filepath.Join(t.TempDir(), "paper.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	tables := []string{
		"paper_accounts",
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
	for _, table := range tables {
		if !paperTableExists(t, db, table) {
			t.Fatalf("table %q does not exist", table)
		}
	}
}

func paperTableExists(t *testing.T, db *sql.DB, table string) bool {
	t.Helper()

	var name string
	err := db.QueryRow(`
		SELECT name
		FROM sqlite_master
		WHERE type = 'table' AND name = ?
	`, table).Scan(&name)
	if err == sql.ErrNoRows {
		return false
	}
	if err != nil {
		t.Fatal(err)
	}
	return name == table
}

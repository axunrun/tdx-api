package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"sync"

	_ "github.com/glebarez/go-sqlite"
)

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

func initPaperDB(db *sql.DB) error {
	paperDBWriteMu.Lock()
	defer paperDBWriteMu.Unlock()

	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS paper_accounts (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			status TEXT NOT NULL,
			currency TEXT NOT NULL,
			initial_cash REAL NOT NULL,
			available_cash REAL NOT NULL,
			frozen_cash REAL NOT NULL DEFAULT 0,
			note TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			closed_at TEXT
		);
		CREATE TABLE IF NOT EXISTS paper_account_initial_positions (
			id TEXT PRIMARY KEY,
			account_id TEXT NOT NULL,
			code TEXT NOT NULL,
			name TEXT,
			asset_type TEXT NOT NULL,
			quantity INTEGER NOT NULL,
			cost_price REAL NOT NULL,
			buy_date TEXT,
			created_at TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS paper_positions (
			id TEXT PRIMARY KEY,
			account_id TEXT NOT NULL,
			code TEXT NOT NULL,
			name TEXT,
			asset_type TEXT NOT NULL,
			quantity INTEGER NOT NULL,
			sellable_quantity INTEGER NOT NULL,
			frozen_quantity INTEGER NOT NULL DEFAULT 0,
			avg_cost REAL NOT NULL,
			updated_at TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS paper_orders (
			id TEXT PRIMARY KEY,
			account_id TEXT NOT NULL,
			code TEXT NOT NULL,
			name TEXT,
			asset_type TEXT NOT NULL,
			side TEXT NOT NULL,
			order_type TEXT NOT NULL,
			status TEXT NOT NULL,
			time_in_force TEXT NOT NULL,
			price REAL,
			quantity INTEGER NOT NULL,
			filled_quantity INTEGER NOT NULL,
			reject_reason TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			filled_at TEXT,
			cancelled_at TEXT
		);
		CREATE TABLE IF NOT EXISTS paper_trades (
			id TEXT PRIMARY KEY,
			order_id TEXT NOT NULL,
			account_id TEXT NOT NULL,
			code TEXT NOT NULL,
			side TEXT NOT NULL,
			price REAL NOT NULL,
			quantity INTEGER NOT NULL,
			amount REAL NOT NULL,
			commission REAL NOT NULL DEFAULT 0,
			stamp_tax REAL NOT NULL DEFAULT 0,
			transfer_fee REAL NOT NULL DEFAULT 0,
			traded_at TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS paper_cash_ledger (
			id TEXT PRIMARY KEY,
			account_id TEXT NOT NULL,
			order_id TEXT,
			trade_id TEXT,
			amount REAL NOT NULL,
			balance REAL NOT NULL,
			reason TEXT NOT NULL,
			created_at TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS paper_position_ledger (
			id TEXT PRIMARY KEY,
			account_id TEXT NOT NULL,
			code TEXT NOT NULL,
			order_id TEXT,
			trade_id TEXT,
			quantity_delta INTEGER NOT NULL,
			quantity_after INTEGER NOT NULL,
			reason TEXT NOT NULL,
			created_at TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS paper_agent_actions (
			id TEXT PRIMARY KEY,
			account_id TEXT NOT NULL,
			action_type TEXT NOT NULL,
			request TEXT,
			response TEXT,
			created_at TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS paper_account_snapshots (
			id TEXT PRIMARY KEY,
			account_id TEXT NOT NULL,
			trading_day TEXT NOT NULL,
			total_assets REAL NOT NULL,
			cash_available REAL NOT NULL,
			cash_frozen REAL NOT NULL,
			market_value REAL NOT NULL,
			realized_pnl REAL NOT NULL DEFAULT 0,
			unrealized_pnl REAL NOT NULL DEFAULT 0,
			daily_return REAL NOT NULL DEFAULT 0,
			total_return REAL NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS paper_closed_positions (
			id TEXT PRIMARY KEY,
			account_id TEXT NOT NULL,
			code TEXT NOT NULL,
			name TEXT,
			quantity INTEGER NOT NULL,
			open_amount REAL NOT NULL,
			close_amount REAL NOT NULL,
			realized_pnl REAL NOT NULL,
			opened_at TEXT,
			closed_at TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS paper_closed_position_tracking (
			id TEXT PRIMARY KEY,
			closed_position_id TEXT NOT NULL,
			account_id TEXT NOT NULL,
			code TEXT NOT NULL,
			trading_day TEXT NOT NULL,
			price REAL,
			created_at TEXT NOT NULL
		);
	`)
	return err
}

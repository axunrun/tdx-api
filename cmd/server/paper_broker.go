package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type PaperStore struct {
	db *sql.DB
}

func NewPaperStore(db *sql.DB) *PaperStore {
	return &PaperStore{db: db}
}

func (s *PaperStore) CreateAccount(req PaperCreateAccountRequest) (PaperAccount, error) {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return PaperAccount{}, errors.New("account name is required")
	}
	if req.InitialCash < 0 {
		return PaperAccount{}, errors.New("initial cash cannot be negative")
	}
	for i := range req.InitialPositions {
		position := &req.InitialPositions[i]
		position.Code = strings.TrimSpace(position.Code)
		position.AssetType = strings.TrimSpace(position.AssetType)
		if position.Code == "" {
			return PaperAccount{}, errors.New("initial position code is required")
		}
		if position.Quantity <= 0 {
			return PaperAccount{}, errors.New("initial position quantity must be positive")
		}
		if position.CostPrice < 0 {
			return PaperAccount{}, errors.New("initial position cost price cannot be negative")
		}
		if position.AssetType == "" {
			position.AssetType = paperAssetStock
		}
	}

	now := time.Now().Format(time.RFC3339Nano)
	account := PaperAccount{
		ID:            newPaperID("acct"),
		Name:          req.Name,
		Status:        paperAccountActive,
		Note:          req.Note,
		CreatedAt:     now,
		InitialCash:   req.InitialCash,
		AvailableCash: req.InitialCash,
		FrozenCash:    0,
	}
	requestJSON, _ := json.Marshal(req)

	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return PaperAccount{}, err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
		INSERT INTO paper_accounts (
			id, name, status, currency, initial_cash, available_cash,
			frozen_cash, note, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, account.ID, account.Name, account.Status, "CNY", account.InitialCash,
		account.AvailableCash, account.FrozenCash, nullIfEmpty(account.Note), now, now); err != nil {
		return PaperAccount{}, err
	}
	if _, err := tx.Exec(`
		INSERT INTO paper_cash_ledger (
			id, account_id, amount, balance, reason, created_at
		) VALUES (?, ?, ?, ?, ?, ?)
	`, newPaperID("cash"), account.ID, account.InitialCash, account.AvailableCash,
		"initial_cash", now); err != nil {
		return PaperAccount{}, err
	}
	for _, position := range req.InitialPositions {
		if err := insertInitialPosition(tx, account.ID, position, now); err != nil {
			return PaperAccount{}, err
		}
	}
	if _, err := tx.Exec(`
		INSERT INTO paper_agent_actions (
			id, account_id, action_type, request, created_at
		) VALUES (?, ?, ?, ?, ?)
	`, newPaperID("act"), account.ID, "create_account", string(requestJSON), now); err != nil {
		return PaperAccount{}, err
	}
	if err := tx.Commit(); err != nil {
		return PaperAccount{}, err
	}
	return account, nil
}

func (s *PaperStore) GetAccount(accountID string) (PaperAccount, error) {
	return scanPaperAccount(s.db.QueryRow(`
		SELECT id, name, status, COALESCE(note, ''), created_at,
			COALESCE(closed_at, ''), initial_cash, available_cash, frozen_cash
		FROM paper_accounts
		WHERE id = ?
	`, accountID))
}

func (s *PaperStore) ListAccounts() ([]PaperAccount, error) {
	rows, err := s.db.Query(`
		SELECT id, name, status, COALESCE(note, ''), created_at,
			COALESCE(closed_at, ''), initial_cash, available_cash, frozen_cash
		FROM paper_accounts
		ORDER BY created_at, rowid
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []PaperAccount
	for rows.Next() {
		account, err := scanPaperAccount(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	return accounts, rows.Err()
}

func (s *PaperStore) DeleteAccount(accountID string) error {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return errors.New("account ID is required")
	}

	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var exists int
	if err := tx.QueryRow(
		"SELECT 1 FROM paper_accounts WHERE id = ?",
		accountID,
	).Scan(&exists); err != nil {
		return err
	}

	for _, table := range []string{
		"paper_closed_position_tracking",
		"paper_closed_positions",
		"paper_account_snapshots",
		"paper_agent_actions",
		"paper_position_ledger",
		"paper_cash_ledger",
		"paper_trades",
		"paper_orders",
		"paper_positions",
		"paper_account_initial_positions",
	} {
		if _, err := tx.Exec(
			"DELETE FROM "+table+" WHERE account_id = ?",
			accountID,
		); err != nil {
			return fmt.Errorf("delete %s: %w", table, err)
		}
	}
	if _, err := tx.Exec("DELETE FROM paper_accounts WHERE id = ?", accountID); err != nil {
		return fmt.Errorf("delete paper account: %w", err)
	}
	return tx.Commit()
}

func (s *PaperStore) ListPositions(accountID string) ([]PaperPosition, error) {
	rows, err := s.db.Query(`
		SELECT account_id, code, COALESCE(name, ''), asset_type, quantity,
			sellable_quantity, frozen_quantity, avg_cost, updated_at
		FROM paper_positions
		WHERE account_id = ? AND quantity > 0
		ORDER BY code, id
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var positions []PaperPosition
	for rows.Next() {
		var position PaperPosition
		if err := rows.Scan(&position.AccountID, &position.Code, &position.Name,
			&position.AssetType, &position.Quantity, &position.SellableQuantity,
			&position.FrozenQuantity, &position.AvgCost, &position.UpdatedAt); err != nil {
			return nil, err
		}
		positions = append(positions, position)
	}
	return positions, rows.Err()
}

func (s *PaperStore) MakeAllPositionsSellable() error {
	_, err := s.db.Exec(`
		UPDATE paper_positions
		SET sellable_quantity = MAX(0, quantity - frozen_quantity)
	`)
	return err
}

func (s *PaperStore) RefreshSellablePositions(now time.Time) error {
	tradingDay := now.In(paperShanghaiLocation).Format("2006-01-02")
	_, err := s.db.Exec(`
		UPDATE paper_positions
		SET sellable_quantity = MAX(
			0,
			quantity - frozen_quantity - COALESCE((
				SELECT SUM(ledger.quantity_delta)
				FROM paper_position_ledger AS ledger
				WHERE ledger.account_id = paper_positions.account_id
					AND ledger.code = paper_positions.code
					AND ledger.reason = 'buy_fill'
					AND SUBSTR(ledger.created_at, 1, 10) = ?
			), 0)
		)
	`, tradingDay)
	return err
}

func insertInitialPosition(
	tx *sql.Tx,
	accountID string,
	position PaperInitialPosition,
	now string,
) error {
	if _, err := tx.Exec(`
		INSERT INTO paper_account_initial_positions (
			id, account_id, code, name, asset_type, quantity,
			cost_price, buy_date, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, newPaperID("initpos"), accountID, position.Code, nullIfEmpty(position.Name),
		position.AssetType, position.Quantity, position.CostPrice,
		nullIfEmpty(position.BuyDate), now); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		INSERT INTO paper_positions (
			id, account_id, code, name, asset_type, quantity,
			sellable_quantity, frozen_quantity, avg_cost, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, newPaperID("pos"), accountID, position.Code, nullIfEmpty(position.Name),
		position.AssetType, position.Quantity, position.Quantity, 0,
		position.CostPrice, now); err != nil {
		return err
	}
	_, err := tx.Exec(`
		INSERT INTO paper_position_ledger (
			id, account_id, code, quantity_delta, quantity_after,
			reason, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`, newPaperID("posled"), accountID, position.Code, position.Quantity,
		position.Quantity, "initial_position", now)
	return err
}

type paperAccountScanner interface {
	Scan(dest ...any) error
}

func scanPaperAccount(scanner paperAccountScanner) (PaperAccount, error) {
	var account PaperAccount
	err := scanner.Scan(&account.ID, &account.Name, &account.Status, &account.Note,
		&account.CreatedAt, &account.ClosedAt, &account.InitialCash,
		&account.AvailableCash, &account.FrozenCash)
	return account, err
}

func newPaperID(prefix string) string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
	}
	return prefix + "_" + hex.EncodeToString(b[:])
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

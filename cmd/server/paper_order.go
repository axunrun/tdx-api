package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	paperCommissionRate = 0.0001
	paperStampTaxRate   = 0.0005
	paperTransferRate   = 0.00001
)

func calculatePaperFee(input PaperFeeInput) PaperFee {
	fee := PaperFee{
		Commission: input.Amount * paperCommissionRate,
	}
	if input.AssetType == paperAssetStock {
		fee.TransferFee = input.Amount * paperTransferRate
		if input.Side == paperSideSell {
			fee.StampTax = input.Amount * paperStampTaxRate
		}
	}
	return fee
}

func (s *PaperStore) PlaceOrder(req PaperPlaceOrderRequest) (PaperOrder, error) {
	if err := normalizePaperOrderRequest(&req); err != nil {
		return PaperOrder{}, err
	}

	now := time.Now().Format(time.RFC3339Nano)
	order := PaperOrder{
		ID:             newPaperID("ord"),
		AccountID:      req.AccountID,
		Code:           req.Code,
		Name:           req.Name,
		AssetType:      req.AssetType,
		Side:           req.Side,
		OrderType:      req.OrderType,
		Status:         paperOrderPending,
		TimeInForce:    req.TimeInForce,
		Price:          req.Price,
		Quantity:       req.Quantity,
		FilledQuantity: 0,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	requestJSON, _ := json.Marshal(req)

	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return PaperOrder{}, err
	}
	defer tx.Rollback()

	if err := ensurePaperAccountActive(tx, req.AccountID); err != nil {
		return PaperOrder{}, err
	}
	if req.Side == paperSideBuy {
		if req.OrderType != paperOrderMarket {
			if err := freezePaperOrderCash(tx, order, now); err != nil {
				return PaperOrder{}, err
			}
		}
	} else if err := freezePaperOrderPosition(tx, order, now); err != nil {
		return PaperOrder{}, err
	}
	if err := insertPaperOrder(tx, order); err != nil {
		return PaperOrder{}, err
	}
	responseJSON, _ := json.Marshal(order)
	if _, err := tx.Exec(`
		INSERT INTO paper_agent_actions (
			id, account_id, action_type, request, response, created_at
		) VALUES (?, ?, ?, ?, ?, ?)
	`, newPaperID("act"), order.AccountID, "place_order", string(requestJSON),
		string(responseJSON), now); err != nil {
		return PaperOrder{}, err
	}
	if err := tx.Commit(); err != nil {
		return PaperOrder{}, err
	}
	return order, nil
}

func (s *PaperStore) GetOrder(orderID string) (PaperOrder, error) {
	return scanPaperOrder(s.db.QueryRow(`
		SELECT id, account_id, code, COALESCE(name, ''), asset_type, side,
			order_type, status, time_in_force, COALESCE(reject_reason, ''),
			created_at, updated_at, COALESCE(filled_at, ''),
			COALESCE(cancelled_at, ''), COALESCE(price, 0), quantity,
			filled_quantity
		FROM paper_orders
		WHERE id = ?
	`, orderID))
}

func (s *PaperStore) ListOrders(accountID string) ([]PaperOrder, error) {
	rows, err := s.db.Query(`
		SELECT id, account_id, code, COALESCE(name, ''), asset_type, side,
			order_type, status, time_in_force, COALESCE(reject_reason, ''),
			created_at, updated_at, COALESCE(filled_at, ''),
			COALESCE(cancelled_at, ''), COALESCE(price, 0), quantity,
			filled_quantity
		FROM paper_orders
		WHERE account_id = ?
		ORDER BY created_at, rowid
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []PaperOrder
	for rows.Next() {
		order, err := scanPaperOrder(rows)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	return orders, rows.Err()
}

func normalizePaperOrderRequest(req *PaperPlaceOrderRequest) error {
	req.AccountID = strings.TrimSpace(req.AccountID)
	req.Code = strings.TrimSpace(req.Code)
	req.Name = strings.TrimSpace(req.Name)
	req.AssetType = strings.TrimSpace(req.AssetType)
	req.Side = strings.TrimSpace(req.Side)
	req.OrderType = strings.TrimSpace(req.OrderType)
	req.TimeInForce = strings.TrimSpace(req.TimeInForce)

	if req.AccountID == "" {
		return errors.New("account id is required")
	}
	if req.Code == "" {
		return errors.New("code is required")
	}
	if req.AssetType == "" {
		req.AssetType = paperAssetStock
	}
	if req.AssetType != paperAssetStock && req.AssetType != paperAssetETF {
		return fmt.Errorf("unsupported asset type: %s", req.AssetType)
	}
	if req.Side != paperSideBuy && req.Side != paperSideSell {
		return fmt.Errorf("unsupported side: %s", req.Side)
	}
	if req.OrderType != paperOrderMarket && req.OrderType != paperOrderLimit &&
		req.OrderType != paperOrderAuction {
		return fmt.Errorf("unsupported order type: %s", req.OrderType)
	}
	if req.TimeInForce == "" {
		req.TimeInForce = paperTimeInForceDay
	}
	if req.TimeInForce != paperTimeInForceDay &&
		req.TimeInForce != paperTimeInForceAuctionOnly {
		return fmt.Errorf("unsupported time in force: %s", req.TimeInForce)
	}
	if req.Quantity <= 0 {
		return errors.New("quantity must be positive")
	}
	if req.Quantity%100 != 0 {
		return errors.New("quantity must be a multiple of 100")
	}
	if (req.OrderType == paperOrderLimit || req.OrderType == paperOrderAuction) &&
		req.Price <= 0 {
		return errors.New("price must be positive for limit and auction orders")
	}
	return nil
}

func ensurePaperAccountActive(tx *sql.Tx, accountID string) error {
	var status string
	err := tx.QueryRow(`
		SELECT status
		FROM paper_accounts
		WHERE id = ?
	`, accountID).Scan(&status)
	if err == sql.ErrNoRows {
		return errors.New("account not found")
	}
	if err != nil {
		return err
	}
	if status != paperAccountActive {
		return errors.New("account is not active")
	}
	return nil
}

func freezePaperOrderCash(tx *sql.Tx, order PaperOrder, now string) error {
	amount := order.Price * float64(order.Quantity)
	fee := calculatePaperFee(PaperFeeInput{
		AssetType: order.AssetType,
		Side:      order.Side,
		Amount:    amount,
	})
	frozenCash := amount + fee.Total()
	result, err := tx.Exec(`
		UPDATE paper_accounts
		SET available_cash = available_cash - ?,
			frozen_cash = frozen_cash + ?,
			updated_at = ?
		WHERE id = ? AND available_cash >= ?
	`, frozenCash, frozenCash, now, order.AccountID, frozenCash)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("insufficient available cash")
	}
	return nil
}

func freezePaperOrderPosition(tx *sql.Tx, order PaperOrder, now string) error {
	result, err := tx.Exec(`
		UPDATE paper_positions
		SET sellable_quantity = sellable_quantity - ?,
			frozen_quantity = frozen_quantity + ?,
			updated_at = ?
		WHERE account_id = ? AND code = ? AND asset_type = ?
			AND sellable_quantity >= ?
	`, order.Quantity, order.Quantity, now, order.AccountID, order.Code,
		order.AssetType, order.Quantity)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("insufficient sellable position")
	}
	return nil
}

func insertPaperOrder(tx *sql.Tx, order PaperOrder) error {
	_, err := tx.Exec(`
		INSERT INTO paper_orders (
			id, account_id, code, name, asset_type, side, order_type, status,
			time_in_force, price, quantity, filled_quantity, reject_reason,
			created_at, updated_at, filled_at, cancelled_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, order.ID, order.AccountID, order.Code, nullIfEmpty(order.Name),
		order.AssetType, order.Side, order.OrderType, order.Status,
		order.TimeInForce, order.Price, order.Quantity, order.FilledQuantity,
		nullIfEmpty(order.RejectReason), order.CreatedAt, order.UpdatedAt,
		nullIfEmpty(order.FilledAt), nullIfEmpty(order.CancelledAt))
	return err
}

type paperOrderScanner interface {
	Scan(dest ...any) error
}

func scanPaperOrder(scanner paperOrderScanner) (PaperOrder, error) {
	var order PaperOrder
	err := scanner.Scan(&order.ID, &order.AccountID, &order.Code, &order.Name,
		&order.AssetType, &order.Side, &order.OrderType, &order.Status,
		&order.TimeInForce, &order.RejectReason, &order.CreatedAt,
		&order.UpdatedAt, &order.FilledAt, &order.CancelledAt, &order.Price,
		&order.Quantity, &order.FilledQuantity)
	return order, err
}

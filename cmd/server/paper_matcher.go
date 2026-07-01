package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
)

type PaperQuoteProvider func(code string) (PaperQuote, error)

func (s *PaperStore) MatchOpenOrders(quote PaperQuoteProvider) error {
	if quote == nil {
		return errors.New("quote provider is required")
	}
	rows, err := s.db.Query(`
		SELECT id, account_id, code, COALESCE(name, ''), asset_type, side,
			order_type, status, time_in_force, COALESCE(reject_reason, ''),
			created_at, updated_at, COALESCE(filled_at, ''),
			COALESCE(cancelled_at, ''), COALESCE(price, 0), quantity,
			filled_quantity
		FROM paper_orders
		WHERE status = ?
		ORDER BY created_at, rowid
	`, paperOrderPending)
	if err != nil {
		return err
	}
	defer rows.Close()

	var orders []PaperOrder
	for rows.Next() {
		order, err := scanPaperOrder(rows)
		if err != nil {
			return err
		}
		orders = append(orders, order)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, order := range orders {
		q, err := quote(order.Code)
		if err != nil {
			continue
		}
		if err := s.FillOrder(order.ID, q); err != nil {
			return err
		}
	}
	return nil
}

func (s *PaperStore) FillOrder(orderID string, quote PaperQuote) error {
	if quote.Price <= 0 {
		return errors.New("quote price must be positive")
	}

	now := time.Now().Format(time.RFC3339Nano)
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	order, err := getPaperOrderForUpdate(tx, orderID)
	if err != nil {
		return err
	}
	if order.Status != paperOrderPending {
		return errors.New("order is not pending")
	}
	if !paperOrderMatchesQuote(order, quote) {
		return nil
	}

	// ponytail: no partial fills in v1; split orders later if needed.
	amount := quote.Price * float64(order.Quantity)
	fee := calculatePaperFee(PaperFeeInput{
		AssetType: order.AssetType,
		Side:      order.Side,
		Amount:    amount,
	})
	trade := PaperTrade{
		ID:          newPaperID("trd"),
		OrderID:     order.ID,
		AccountID:   order.AccountID,
		Code:        order.Code,
		Side:        order.Side,
		TradedAt:    now,
		Price:       quote.Price,
		Quantity:    order.Quantity,
		Amount:      amount,
		Commission:  fee.Commission,
		StampTax:    fee.StampTax,
		TransferFee: fee.TransferFee,
	}

	if order.Side == paperSideBuy {
		err = fillPaperBuy(tx, order, trade, fee, now)
	} else {
		err = fillPaperSell(tx, order, trade, fee, quote, now)
	}
	if err != nil {
		return err
	}
	if err := insertPaperTrade(tx, trade); err != nil {
		return err
	}
	if err := markPaperOrderFilled(tx, order, now); err != nil {
		return err
	}
	if err := insertPaperFillAction(tx, order, quote, trade, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *PaperStore) ListTrades(accountID string) ([]PaperTrade, error) {
	rows, err := s.db.Query(`
		SELECT id, order_id, account_id, code, side, traded_at, price,
			quantity, amount, commission, stamp_tax, transfer_fee
		FROM paper_trades
		WHERE account_id = ?
		ORDER BY traded_at, rowid
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trades []PaperTrade
	for rows.Next() {
		var trade PaperTrade
		if err := rows.Scan(&trade.ID, &trade.OrderID, &trade.AccountID,
			&trade.Code, &trade.Side, &trade.TradedAt, &trade.Price,
			&trade.Quantity, &trade.Amount, &trade.Commission, &trade.StampTax,
			&trade.TransferFee); err != nil {
			return nil, err
		}
		trades = append(trades, trade)
	}
	return trades, rows.Err()
}

func (s *PaperStore) ListClosedPositions(
	accountID string,
	rangeName string,
) ([]PaperClosedPosition, error) {
	args := []any{accountID}
	where := "account_id = ?"
	if cutoff, ok := paperClosedPositionCutoff(rangeName); ok {
		where += " AND closed_at >= ?"
		args = append(args, cutoff)
	}
	rows, err := s.db.Query(fmt.Sprintf(`
		SELECT id, account_id, code, COALESCE(name, ''), COALESCE(opened_at, ''),
			closed_at, quantity, open_amount, close_amount, realized_pnl
		FROM paper_closed_positions
		WHERE %s
		ORDER BY closed_at DESC, rowid DESC
	`, where), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var positions []PaperClosedPosition
	for rows.Next() {
		var position PaperClosedPosition
		if err := rows.Scan(&position.ID, &position.AccountID, &position.Code,
			&position.Name, &position.OpenedAt, &position.ClosedAt,
			&position.Quantity, &position.OpenAmount, &position.CloseAmount,
			&position.RealizedPnL); err != nil {
			return nil, err
		}
		positions = append(positions, position)
	}
	return positions, rows.Err()
}

func (s *PaperStore) CreateSnapshot(accountID string, quote PaperQuoteProvider) error {
	account, err := s.GetAccount(accountID)
	if err != nil {
		return err
	}
	positions, err := s.ListPositions(accountID)
	if err != nil {
		return err
	}

	marketValue := 0.0
	unrealizedPnL := 0.0
	for _, position := range positions {
		price := position.AvgCost
		if quote != nil {
			if q, err := quote(position.Code); err == nil && q.Price > 0 {
				price = q.Price
			}
		}
		value := price * float64(position.Quantity)
		marketValue += value
		unrealizedPnL += value - position.AvgCost*float64(position.Quantity)
	}
	realizedPnL, err := sumPaperRealizedPnL(s.db, accountID)
	if err != nil {
		return err
	}
	now := time.Now()
	_, err = s.db.Exec(`
		INSERT INTO paper_account_snapshots (
			id, account_id, trading_day, total_assets, cash_available,
			cash_frozen, market_value, realized_pnl, unrealized_pnl,
			daily_return, total_return, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, newPaperID("snap"), account.ID, now.Format("2006-01-02"),
		account.AvailableCash+account.FrozenCash+marketValue,
		account.AvailableCash, account.FrozenCash, marketValue, realizedPnL,
		unrealizedPnL, 0, 0, now.Format(time.RFC3339Nano))
	return err
}

func startPaperBackgroundTasks(store *PaperStore, quote PaperQuoteProvider) {
	if store == nil || quote == nil {
		return
	}
	go func() {
		matchTicker := time.NewTicker(30 * time.Second)
		snapshotTicker := time.NewTicker(5 * time.Minute)
		defer matchTicker.Stop()
		defer snapshotTicker.Stop()

		for {
			select {
			case <-matchTicker.C:
				if err := store.MatchOpenOrders(quote); err != nil {
					log.Printf("paper match open orders failed: %v", err)
				}
			case <-snapshotTicker.C:
			}
		}
	}()
}

func quotePaperFromTDX(code string) (PaperQuote, error) {
	c := cli()
	if c == nil {
		return PaperQuote{}, errors.New("tdx client is not connected")
	}
	quotes, err := c.GetQuote(code)
	if err != nil {
		return PaperQuote{}, err
	}
	if len(quotes) == 0 || quotes[0] == nil || quotes[0].Kline == nil {
		return PaperQuote{}, errors.New("no quote data")
	}
	return PaperQuote{
		Code:  quotes[0].Code,
		Price: quotes[0].Kline.Close.Float64(),
	}, nil
}

func getPaperOrderForUpdate(tx *sql.Tx, orderID string) (PaperOrder, error) {
	return scanPaperOrder(tx.QueryRow(`
		SELECT id, account_id, code, COALESCE(name, ''), asset_type, side,
			order_type, status, time_in_force, COALESCE(reject_reason, ''),
			created_at, updated_at, COALESCE(filled_at, ''),
			COALESCE(cancelled_at, ''), COALESCE(price, 0), quantity,
			filled_quantity
		FROM paper_orders
		WHERE id = ?
	`, orderID))
}

func paperOrderMatchesQuote(order PaperOrder, quote PaperQuote) bool {
	if order.OrderType == paperOrderMarket {
		return true
	}
	if order.Side == paperSideBuy {
		return quote.Price <= order.Price
	}
	return quote.Price >= order.Price
}

func fillPaperBuy(
	tx *sql.Tx,
	order PaperOrder,
	trade PaperTrade,
	fee PaperFee,
	now string,
) error {
	totalCost := trade.Amount + fee.Total()
	if order.OrderType == paperOrderMarket {
		if err := updatePaperMarketBuyCash(tx, order, totalCost, now); err != nil {
			return err
		}
	} else {
		if err := updatePaperFrozenBuyCash(tx, order, trade, totalCost, now); err != nil {
			return err
		}
	}
	balance, err := getPaperAvailableCash(tx, order.AccountID)
	if err != nil {
		return err
	}
	if err := insertPaperCashLedger(tx, order, trade.ID, -totalCost, balance, now); err != nil {
		return err
	}
	quantityAfter, err := addPaperPosition(tx, order, trade, totalCost, now)
	if err != nil {
		return err
	}
	return insertPaperPositionLedger(tx, order, trade.ID, order.Quantity,
		quantityAfter, "buy_fill", now)
}

func fillPaperSell(
	tx *sql.Tx,
	order PaperOrder,
	trade PaperTrade,
	fee PaperFee,
	quote PaperQuote,
	now string,
) error {
	position, err := getPaperPosition(tx, order)
	if err != nil {
		return err
	}
	if position.Quantity < order.Quantity || position.FrozenQuantity < order.Quantity {
		return errors.New("insufficient frozen position")
	}
	netAmount := trade.Amount - fee.Total()
	quantityAfter := position.Quantity - order.Quantity
	_, err = tx.Exec(`
		UPDATE paper_positions
		SET quantity = quantity - ?,
			frozen_quantity = frozen_quantity - ?,
			updated_at = ?
		WHERE id = ?
	`, order.Quantity, order.Quantity, now, position.ID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`
		UPDATE paper_accounts
		SET available_cash = available_cash + ?,
			updated_at = ?
		WHERE id = ?
	`, netAmount, now, order.AccountID)
	if err != nil {
		return err
	}
	balance, err := getPaperAvailableCash(tx, order.AccountID)
	if err != nil {
		return err
	}
	if err := insertPaperCashLedger(tx, order, trade.ID, netAmount, balance, now); err != nil {
		return err
	}
	if err := insertPaperPositionLedger(tx, order, trade.ID, -order.Quantity,
		quantityAfter, "sell_fill", now); err != nil {
		return err
	}
	if quantityAfter == 0 {
		openAmount := position.AvgCost * float64(order.Quantity)
		return insertPaperClosedPosition(tx, order, quote, position, netAmount,
			netAmount-openAmount, now)
	}
	return nil
}

func updatePaperMarketBuyCash(
	tx *sql.Tx,
	order PaperOrder,
	totalCost float64,
	now string,
) error {
	result, err := tx.Exec(`
		UPDATE paper_accounts
		SET available_cash = available_cash - ?,
			updated_at = ?
		WHERE id = ? AND available_cash >= ?
	`, totalCost, now, order.AccountID, totalCost)
	if err != nil {
		return err
	}
	return requireRowsAffected(result, "insufficient available cash")
}

func updatePaperFrozenBuyCash(
	tx *sql.Tx,
	order PaperOrder,
	trade PaperTrade,
	totalCost float64,
	now string,
) error {
	frozenAmount := order.Price * float64(order.Quantity)
	frozenFee := calculatePaperFee(PaperFeeInput{
		AssetType: order.AssetType,
		Side:      order.Side,
		Amount:    frozenAmount,
	})
	frozenCash := frozenAmount + frozenFee.Total()
	releaseCash := frozenCash - totalCost
	if releaseCash < 0 {
		return errors.New("frozen cash is less than fill cost")
	}
	result, err := tx.Exec(`
		UPDATE paper_accounts
		SET frozen_cash = frozen_cash - ?,
			available_cash = available_cash + ?,
			updated_at = ?
		WHERE id = ? AND frozen_cash >= ?
	`, frozenCash, releaseCash, now, trade.AccountID, frozenCash)
	if err != nil {
		return err
	}
	return requireRowsAffected(result, "insufficient frozen cash")
}

type paperPositionRecord struct {
	ID               string
	Name             string
	Quantity         int64
	SellableQuantity int64
	FrozenQuantity   int64
	AvgCost          float64
	UpdatedAt        string
}

func getPaperPosition(tx *sql.Tx, order PaperOrder) (paperPositionRecord, error) {
	var position paperPositionRecord
	err := tx.QueryRow(`
		SELECT id, COALESCE(name, ''), quantity, sellable_quantity,
			frozen_quantity, avg_cost, updated_at
		FROM paper_positions
		WHERE account_id = ? AND code = ? AND asset_type = ?
		ORDER BY rowid
		LIMIT 1
	`, order.AccountID, order.Code, order.AssetType).Scan(&position.ID,
		&position.Name, &position.Quantity, &position.SellableQuantity,
		&position.FrozenQuantity, &position.AvgCost, &position.UpdatedAt)
	return position, err
}

func addPaperPosition(
	tx *sql.Tx,
	order PaperOrder,
	trade PaperTrade,
	totalCost float64,
	now string,
) (int64, error) {
	sellableDelta := paperBuySellableDelta(order)
	position, err := getPaperPosition(tx, order)
	if err == sql.ErrNoRows {
		_, err = tx.Exec(`
			INSERT INTO paper_positions (
				id, account_id, code, name, asset_type, quantity,
				sellable_quantity, frozen_quantity, avg_cost, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, newPaperID("pos"), order.AccountID, order.Code, nullIfEmpty(order.Name),
			order.AssetType, order.Quantity, sellableDelta, 0,
			totalCost/float64(order.Quantity), now)
		return order.Quantity, err
	}
	if err != nil {
		return 0, err
	}

	quantityAfter := position.Quantity + order.Quantity
	avgCost := (position.AvgCost*float64(position.Quantity) + totalCost) /
		float64(quantityAfter)
	_, err = tx.Exec(`
		UPDATE paper_positions
		SET quantity = ?,
			sellable_quantity = sellable_quantity + ?,
			avg_cost = ?,
			updated_at = ?
		WHERE id = ?
	`, quantityAfter, sellableDelta, avgCost, now, position.ID)
	return quantityAfter, err
}

func paperBuySellableDelta(_ PaperOrder) int64 {
	return 0
}

func insertPaperTrade(tx *sql.Tx, trade PaperTrade) error {
	_, err := tx.Exec(`
		INSERT INTO paper_trades (
			id, order_id, account_id, code, side, price, quantity, amount,
			commission, stamp_tax, transfer_fee, traded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, trade.ID, trade.OrderID, trade.AccountID, trade.Code, trade.Side,
		trade.Price, trade.Quantity, trade.Amount, trade.Commission,
		trade.StampTax, trade.TransferFee, trade.TradedAt)
	return err
}

func markPaperOrderFilled(tx *sql.Tx, order PaperOrder, now string) error {
	_, err := tx.Exec(`
		UPDATE paper_orders
		SET status = ?,
			filled_quantity = ?,
			filled_at = ?,
			updated_at = ?
		WHERE id = ?
	`, paperOrderFilled, order.Quantity, now, now, order.ID)
	return err
}

func insertPaperCashLedger(
	tx *sql.Tx,
	order PaperOrder,
	tradeID string,
	amount float64,
	balance float64,
	now string,
) error {
	_, err := tx.Exec(`
		INSERT INTO paper_cash_ledger (
			id, account_id, order_id, trade_id, amount, balance, reason,
			created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, newPaperID("cash"), order.AccountID, order.ID, tradeID, amount,
		balance, "order_fill", now)
	return err
}

func insertPaperPositionLedger(
	tx *sql.Tx,
	order PaperOrder,
	tradeID string,
	quantityDelta int64,
	quantityAfter int64,
	reason string,
	now string,
) error {
	_, err := tx.Exec(`
		INSERT INTO paper_position_ledger (
			id, account_id, code, order_id, trade_id, quantity_delta,
			quantity_after, reason, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, newPaperID("posled"), order.AccountID, order.Code, order.ID, tradeID,
		quantityDelta, quantityAfter, reason, now)
	return err
}

func insertPaperClosedPosition(
	tx *sql.Tx,
	order PaperOrder,
	quote PaperQuote,
	position paperPositionRecord,
	closeAmount float64,
	realizedPnL float64,
	now string,
) error {
	name := firstNonEmpty(quote.Name, order.Name, position.Name)
	_, err := tx.Exec(`
		INSERT INTO paper_closed_positions (
			id, account_id, code, name, quantity, open_amount, close_amount,
			realized_pnl, opened_at, closed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, newPaperID("closed"), order.AccountID, order.Code, nullIfEmpty(name),
		order.Quantity, position.AvgCost*float64(order.Quantity), closeAmount,
		realizedPnL, nullIfEmpty(position.UpdatedAt), now)
	return err
}

func insertPaperFillAction(
	tx *sql.Tx,
	order PaperOrder,
	quote PaperQuote,
	trade PaperTrade,
	now string,
) error {
	requestJSON, _ := json.Marshal(quote)
	responseJSON, _ := json.Marshal(trade)
	_, err := tx.Exec(`
		INSERT INTO paper_agent_actions (
			id, account_id, action_type, request, response, created_at
		) VALUES (?, ?, ?, ?, ?, ?)
	`, newPaperID("act"), order.AccountID, "fill_order", string(requestJSON),
		string(responseJSON), now)
	return err
}

func getPaperAvailableCash(tx *sql.Tx, accountID string) (float64, error) {
	var balance float64
	err := tx.QueryRow(`
		SELECT available_cash
		FROM paper_accounts
		WHERE id = ?
	`, accountID).Scan(&balance)
	return balance, err
}

func sumPaperRealizedPnL(db *sql.DB, accountID string) (float64, error) {
	var realizedPnL float64
	err := db.QueryRow(`
		SELECT COALESCE(SUM(realized_pnl), 0)
		FROM paper_closed_positions
		WHERE account_id = ?
	`, accountID).Scan(&realizedPnL)
	return realizedPnL, err
}

func paperClosedPositionCutoff(rangeName string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(rangeName)) {
	case "today", "1d":
		return time.Now().Format("2006-01-02"), true
	case "7d":
		return time.Now().AddDate(0, 0, -7).Format(time.RFC3339Nano), true
	case "30d":
		return time.Now().AddDate(0, 0, -30).Format(time.RFC3339Nano), true
	default:
		return "", false
	}
}

func requireRowsAffected(result sql.Result, message string) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New(message)
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type PaperAgentAction struct {
	ID         string `json:"id"`
	AccountID  string `json:"accountId"`
	ActionType string `json:"actionType"`
	Request    string `json:"request,omitempty"`
	Response   string `json:"response,omitempty"`
	CreatedAt  string `json:"createdAt"`
}

type PaperEquityPoint struct {
	TradingDay   string  `json:"tradingDay"`
	TotalAssets  float64 `json:"totalAssets"`
	CreatedAt    string  `json:"createdAt"`
	BuyQuantity  int64   `json:"buyQuantity"`
	BuyAmount    float64 `json:"buyAmount"`
	SellQuantity int64   `json:"sellQuantity"`
	SellAmount   float64 `json:"sellAmount"`
}

type paperTradeSummary struct {
	BuyQuantity  int64
	BuyAmount    float64
	SellQuantity int64
	SellAmount   float64
}

type PaperEquitySeries struct {
	AccountID   string             `json:"accountId"`
	AccountName string             `json:"accountName"`
	Points      []PaperEquityPoint `json:"points"`
}

func handlePaperAccounts(w http.ResponseWriter, r *http.Request) {
	handlePaperAccountsWithStore(paperStore)(w, r)
}

func handlePaperAccount(w http.ResponseWriter, r *http.Request) {
	handlePaperAccountWithStore(paperStore)(w, r)
}

func handlePaperDashboard(w http.ResponseWriter, r *http.Request) {
	handlePaperDashboardWithStore(paperStore)(w, r)
}

func handlePaperActivity(w http.ResponseWriter, r *http.Request) {
	handlePaperActivityWithStore(paperStore)(w, r)
}

func handlePaperClosedPositions(w http.ResponseWriter, r *http.Request) {
	handlePaperClosedPositionsWithStore(paperStore)(w, r)
}

func handlePaperAccountsWithStore(store *PaperStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requirePaperGET(w, r) || !requirePaperStore(w, store) {
			return
		}
		accounts, err := store.ListAccounts()
		if err != nil {
			jsonErr(w, err.Error())
			return
		}
		jsonResp(w, map[string]any{
			"items": emptyPaperAccounts(accounts),
			"count": len(accounts),
		})
	}
}

func handlePaperAccountWithStore(store *PaperStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requirePaperGET(w, r) || !requirePaperStore(w, store) {
			return
		}
		accountID := strings.TrimSpace(r.URL.Query().Get("accountId"))
		if accountID == "" {
			jsonErr(w, "accountId is required")
			return
		}
		account, err := store.GetAccount(accountID)
		if err != nil {
			jsonErr(w, err.Error())
			return
		}
		positions, orders, trades, err := loadPaperAccountActivity(store, account.ID)
		if err != nil {
			jsonErr(w, err.Error())
			return
		}
		jsonResp(w, map[string]any{
			"account":   account,
			"positions": emptyPaperPositions(positions),
			"orders":    emptyPaperOrders(orders),
			"trades":    emptyPaperTrades(trades),
		})
	}
}

func handlePaperDashboardWithStore(store *PaperStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requirePaperGET(w, r) || !requirePaperStore(w, store) {
			return
		}
		accounts, err := store.ListAccounts()
		if err != nil {
			jsonErr(w, err.Error())
			return
		}

		var selectedAccount *PaperAccount
		accountID := strings.TrimSpace(r.URL.Query().Get("accountId"))
		if accountID != "" {
			account, err := store.GetAccount(accountID)
			if err != nil {
				jsonErr(w, err.Error())
				return
			}
			selectedAccount = &account
		} else if len(accounts) > 0 {
			selectedAccount = &accounts[0]
		}
		equityRange := r.URL.Query().Get("range")

		positions := []PaperPosition{}
		orders := []PaperOrder{}
		trades := []PaperTrade{}
		closedPositions := []PaperClosedPosition{}
		equityCurve := []PaperEquityPoint{}
		equityCurves, err := listPaperEquityCurves(store, accounts, equityRange)
		if err != nil {
			jsonErr(w, err.Error())
			return
		}
		if selectedAccount != nil {
			positions, orders, trades, err = loadPaperAccountActivity(store, selectedAccount.ID)
			if err != nil {
				jsonErr(w, err.Error())
				return
			}
			closedPositions, err = store.ListClosedPositions(selectedAccount.ID, "")
			if err != nil {
				jsonErr(w, err.Error())
				return
			}
			equityCurve, err = listPaperEquityCurve(store, selectedAccount.ID, equityRange)
			if err != nil {
				jsonErr(w, err.Error())
				return
			}
		}

		jsonResp(w, map[string]any{
			"marketStatus": map[string]string{
				"status": "unknown",
				"note":   "market data handled by existing agent APIs",
			},
			"updatedAt":       time.Now().Format(time.RFC3339Nano),
			"accounts":        emptyPaperAccounts(accounts),
			"selectedAccount": selectedAccount,
			"positions":       emptyPaperPositions(positions),
			"orders":          emptyPaperOrders(orders),
			"trades":          emptyPaperTrades(trades),
			"closedPositions": emptyPaperClosedPositions(closedPositions),
			"equityCurve":     equityCurve,
			"equityCurves":    equityCurves,
		})
	}
}

func handlePaperActivityWithStore(store *PaperStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requirePaperGET(w, r) || !requirePaperStore(w, store) {
			return
		}
		accountID := strings.TrimSpace(r.URL.Query().Get("accountId"))
		actions, err := listPaperAgentActions(store, accountID, paperActivityLimit(r))
		if err != nil {
			jsonErr(w, err.Error())
			return
		}
		jsonResp(w, map[string]any{
			"items": actions,
			"count": len(actions),
		})
	}
}

func handlePaperClosedPositionsWithStore(store *PaperStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requirePaperGET(w, r) || !requirePaperStore(w, store) {
			return
		}
		accountID := strings.TrimSpace(r.URL.Query().Get("accountId"))
		if accountID == "" {
			jsonErr(w, "accountId is required")
			return
		}
		items, err := store.ListClosedPositions(accountID, r.URL.Query().Get("range"))
		if err != nil {
			jsonErr(w, err.Error())
			return
		}
		jsonResp(w, map[string]any{
			"items": emptyPaperClosedPositions(items),
			"count": len(items),
		})
	}
}

func requirePaperGET(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodGet {
		return true
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusMethodNotAllowed)
	json.NewEncoder(w).Encode(APIResponse{-1, "method not allowed", nil})
	return false
}

func requirePaperStore(w http.ResponseWriter, store *PaperStore) bool {
	if store != nil {
		return true
	}
	jsonErr(w, "paper store is unavailable")
	return false
}

func loadPaperAccountActivity(
	store *PaperStore,
	accountID string,
) ([]PaperPosition, []PaperOrder, []PaperTrade, error) {
	positions, err := store.ListPositions(accountID)
	if err != nil {
		return nil, nil, nil, err
	}
	orders, err := store.ListOrders(accountID)
	if err != nil {
		return nil, nil, nil, err
	}
	trades, err := store.ListTrades(accountID)
	if err != nil {
		return nil, nil, nil, err
	}
	return positions, orders, trades, nil
}

func listPaperAgentActions(
	store *PaperStore,
	accountID string,
	limit int,
) ([]PaperAgentAction, error) {
	query := `
		SELECT id, account_id, action_type, COALESCE(request, ''),
			COALESCE(response, ''), created_at
		FROM paper_agent_actions
	`
	args := []any{}
	if accountID != "" {
		query += ` WHERE account_id = ?`
		args = append(args, accountID)
	}
	query += ` ORDER BY created_at DESC, rowid DESC LIMIT ?`
	args = append(args, limit)

	rows, err := store.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	actions := []PaperAgentAction{}
	for rows.Next() {
		var action PaperAgentAction
		if err := rows.Scan(&action.ID, &action.AccountID, &action.ActionType,
			&action.Request, &action.Response, &action.CreatedAt); err != nil {
			return nil, err
		}
		actions = append(actions, action)
	}
	return actions, rows.Err()
}

func listPaperEquityCurve(
	store *PaperStore,
	accountID string,
	rangeName string,
) ([]PaperEquityPoint, error) {
	rows, err := store.db.Query(`
		SELECT trading_day, total_assets, created_at
		FROM paper_account_snapshots
		WHERE account_id = ?
		ORDER BY trading_day, created_at
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	points := []PaperEquityPoint{}
	for rows.Next() {
		var point PaperEquityPoint
		if err := rows.Scan(&point.TradingDay, &point.TotalAssets,
			&point.CreatedAt); err != nil {
			return nil, err
		}
		points = append(points, point)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	summaries, err := loadPaperTradeSummaries(store, accountID)
	if err != nil {
		return nil, err
	}
	for i := range points {
		summary := summaries[points[i].TradingDay]
		points[i].BuyQuantity = summary.BuyQuantity
		points[i].BuyAmount = summary.BuyAmount
		points[i].SellQuantity = summary.SellQuantity
		points[i].SellAmount = summary.SellAmount
	}
	return filterPaperEquityPoints(points, rangeName), nil
}

func loadPaperTradeSummaries(
	store *PaperStore,
	accountID string,
) (map[string]paperTradeSummary, error) {
	rows, err := store.db.Query(`
		SELECT substr(traded_at, 1, 10), side, SUM(quantity), SUM(amount)
		FROM paper_trades
		WHERE account_id = ?
		GROUP BY substr(traded_at, 1, 10), side
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	summaries := map[string]paperTradeSummary{}
	for rows.Next() {
		var day, side string
		var quantity int64
		var amount float64
		if err := rows.Scan(&day, &side, &quantity, &amount); err != nil {
			return nil, err
		}
		summary := summaries[day]
		if side == paperSideBuy {
			summary.BuyQuantity = quantity
			summary.BuyAmount = amount
		} else if side == paperSideSell {
			summary.SellQuantity = quantity
			summary.SellAmount = amount
		}
		summaries[day] = summary
	}
	return summaries, rows.Err()
}

func listPaperEquityCurves(
	store *PaperStore,
	accounts []PaperAccount,
	rangeName string,
) ([]PaperEquitySeries, error) {
	series := make([]PaperEquitySeries, 0, len(accounts))
	for _, account := range accounts {
		points, err := listPaperEquityCurve(store, account.ID, rangeName)
		if err != nil {
			return nil, err
		}
		series = append(series, PaperEquitySeries{
			AccountID:   account.ID,
			AccountName: account.Name,
			Points:      points,
		})
	}
	return series, nil
}

func filterPaperEquityPoints(
	points []PaperEquityPoint,
	rangeName string,
) []PaperEquityPoint {
	if len(points) == 0 {
		return points
	}
	daily := make([]PaperEquityPoint, 0, len(points))
	for _, point := range points {
		last := len(daily) - 1
		if last >= 0 && daily[last].TradingDay == point.TradingDay {
			daily[last] = point
			continue
		}
		daily = append(daily, point)
	}
	days := paperEquityRangeDays(rangeName)
	if days <= 0 || len(daily) <= days {
		return daily
	}
	return daily[len(daily)-days:]
}

func paperEquityRangeDays(rangeName string) int {
	value := strings.ToLower(strings.TrimSpace(rangeName))
	switch value {
	case "60", "60d":
		return 60
	case "120", "120d":
		return 120
	case "all":
		return 0
	}
	if days, err := strconv.Atoi(strings.TrimSuffix(value, "d")); err == nil && days > 0 {
		return days
	}
	return 20
}

func paperActivityLimit(r *http.Request) int {
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit <= 0 {
		return 50
	}
	if limit > 200 {
		return 200
	}
	return limit
}

func emptyPaperAccounts(items []PaperAccount) []PaperAccount {
	if items == nil {
		return []PaperAccount{}
	}
	return items
}

func emptyPaperPositions(items []PaperPosition) []PaperPosition {
	if items == nil {
		return []PaperPosition{}
	}
	return items
}

func emptyPaperOrders(items []PaperOrder) []PaperOrder {
	if items == nil {
		return []PaperOrder{}
	}
	return items
}

func emptyPaperTrades(items []PaperTrade) []PaperTrade {
	if items == nil {
		return []PaperTrade{}
	}
	return items
}

func emptyPaperClosedPositions(items []PaperClosedPosition) []PaperClosedPosition {
	if items == nil {
		return []PaperClosedPosition{}
	}
	return items
}

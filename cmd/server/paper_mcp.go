package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

func paperMCPTools() []mcpTool {
	account := newMCPTool(
		"tdx_paper_account",
		"纸上交易账户生命周期工具。账户创建后初始资金和初始持仓会锁定，只有用户明确要求时才执行 close 或 recreate；首版 close/recreate 仅保留 schema。",
		"",
		nil,
		requiredEnum("action", "操作：create 创建、list 列表、get 详情、close 关闭、recreate 重建。", "create", "list", "get", "close", "recreate"),
		optionalString("accountId", "账户 ID；get/close/recreate 时需要。"),
		optionalString("name", "账户名称；create 时需要。"),
		optionalNumber("initialCash", "初始现金；create 可选，默认 0。"),
		optionalString("note", "账户备注。"),
	)
	account.InputSchema["properties"].(map[string]any)["initialPositions"] =
		paperInitialPositionsSchema()

	order := newMCPTool(
		"tdx_paper_order",
		"纸上交易委托工具。支持下单、撤单、委托列表和委托详情。",
		"",
		nil,
		requiredEnum("action", "操作：place 下单、cancel 撤单、list 列表、get 详情。", "place", "cancel", "list", "get"),
		requiredString("accountId", "账户 ID。"),
		optionalString("code", "证券代码；place 时需要。"),
		optionalEnum("side", "买卖方向。", "buy", "sell"),
		optionalEnum("orderType", "委托类型。", "market", "limit", "auction"),
		optionalNumber("price", "委托价格；limit/auction 必填。"),
		optionalNumber("quantity", "委托数量；place 时需要，必须为 100 的整数倍。"),
		optionalEnum("timeInForce", "有效期。", "day", "auction_only"),
		optionalString("orderId", "委托 ID；get/cancel 时需要。"),
		optionalString("name", "证券名称。"),
		optionalEnum("assetType", "资产类型。", "stock", "etf"),
	)

	portfolio := newMCPTool(
		"tdx_paper_portfolio",
		"纸上交易账户查询工具。按 view 查询 summary/cash/positions/trades/orders/performance/closed_positions/actions。",
		"",
		nil,
		requiredString("accountId", "账户 ID。"),
		requiredEnum("view", "查询视图。", "summary", "cash", "positions", "trades", "orders", "performance", "closed_positions", "actions"),
		optionalString("from", "起始时间或日期，按字符串时间过滤。"),
		optionalString("to", "结束时间或日期，按字符串时间过滤。"),
		optionalNumber("limit", "最多返回条数，默认 50，最大 200。"),
		optionalString("code", "证券代码过滤。"),
	)

	rules := newMCPTool(
		"tdx_paper_rules",
		"返回纸上交易规则、费用、委托限制和账户生命周期说明。",
		"",
		nil,
	)

	return []mcpTool{account, order, portfolio, rules}
}

func requiredEnum(name, description string, values ...string) mcpToolParam {
	return mcpToolParam{
		Name:        name,
		Type:        "string",
		Description: description,
		Required:    true,
		Enum:        values,
	}
}

func paperInitialPositionsSchema() map[string]any {
	return map[string]any{
		"type":        "array",
		"description": "初始持仓数组；账户创建后锁定，不用于后续追加修改。",
		"items": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"code": map[string]any{
					"type":        "string",
					"description": "证券代码。",
				},
				"name": map[string]any{
					"type":        "string",
					"description": "证券名称。",
				},
				"assetType": map[string]any{
					"type":        "string",
					"description": "资产类型。",
					"enum":        []string{"stock", "etf"},
				},
				"quantity": map[string]any{
					"type":        "number",
					"description": "持仓数量，必须为正数。",
				},
				"costPrice": map[string]any{
					"type":        "number",
					"description": "成本价，不能为负数。",
				},
				"buyDate": map[string]any{
					"type":        "string",
					"description": "买入日期。",
				},
			},
			"required": []string{"code", "quantity"},
		},
	}
}

func callPaperMCPTool(
	name string,
	args map[string]any,
) (map[string]any, bool, error) {
	if args == nil {
		args = map[string]any{}
	}
	switch name {
	case "tdx_paper_account":
		result, err := callPaperAccountMCP(args)
		return result, true, err
	case "tdx_paper_order":
		result, err := callPaperOrderMCP(args)
		return result, true, err
	case "tdx_paper_portfolio":
		result, err := callPaperPortfolioMCP(args)
		return result, true, err
	case "tdx_paper_rules":
		return paperRulesMCPResult(), true, nil
	default:
		return nil, false, nil
	}
}

func callPaperAccountMCP(args map[string]any) (map[string]any, error) {
	store, err := requirePaperMCPStore()
	if err != nil {
		return nil, err
	}
	action := paperStringArg(args, "action")
	switch action {
	case "create":
		var req PaperCreateAccountRequest
		if err := decodePaperMCPArgs(args, &req); err != nil {
			return nil, err
		}
		account, err := store.CreateAccount(req)
		if err != nil {
			return nil, err
		}
		positions, err := store.ListPositions(account.ID)
		if err != nil {
			return nil, err
		}
		return paperMCPResult("账户已创建；初始资金和初始持仓已锁定。", map[string]any{
			"account":   account,
			"positions": emptyPaperPositions(positions),
		}), nil
	case "list":
		accounts, err := store.ListAccounts()
		if err != nil {
			return nil, err
		}
		return paperMCPResult("账户列表已返回。", map[string]any{
			"items": emptyPaperAccounts(accounts),
			"count": len(accounts),
		}), nil
	case "get":
		accountID, err := requirePaperStringArg(args, "accountId")
		if err != nil {
			return nil, err
		}
		account, err := store.GetAccount(accountID)
		if err != nil {
			return nil, err
		}
		positions, orders, trades, err := loadPaperAccountActivity(store, account.ID)
		if err != nil {
			return nil, err
		}
		return paperMCPResult("账户详情已返回。", map[string]any{
			"account":   account,
			"positions": emptyPaperPositions(positions),
			"orders":    emptyPaperOrders(orders),
			"trades":    emptyPaperTrades(trades),
		}), nil
	case "close", "recreate":
		return nil, errors.New("not implemented in first version")
	default:
		return nil, fmt.Errorf("unsupported paper account action: %s", action)
	}
}

func callPaperOrderMCP(args map[string]any) (map[string]any, error) {
	store, err := requirePaperMCPStore()
	if err != nil {
		return nil, err
	}
	action := paperStringArg(args, "action")
	accountID, err := requirePaperStringArg(args, "accountId")
	if err != nil {
		return nil, err
	}
	switch action {
	case "place":
		var req PaperPlaceOrderRequest
		if err := decodePaperMCPArgs(args, &req); err != nil {
			return nil, err
		}
		order, err := store.PlaceOrder(req)
		if err != nil {
			return nil, err
		}
		return paperMCPResult("委托已提交。", map[string]any{"order": order}), nil
	case "list":
		orders, err := store.ListOrders(accountID)
		if err != nil {
			return nil, err
		}
		orders = filterPaperOrders(orders, paperStringArg(args, "code"), "", "")
		orders = limitPaperOrders(orders, paperLimitArg(args))
		return paperMCPResult("委托列表已返回。", map[string]any{
			"items": emptyPaperOrders(orders),
			"count": len(orders),
		}), nil
	case "get":
		orderID, err := requirePaperStringArg(args, "orderId")
		if err != nil {
			return nil, err
		}
		order, err := store.GetOrder(orderID)
		if err != nil {
			return nil, err
		}
		if order.AccountID != accountID {
			return nil, errors.New("order does not belong to account")
		}
		return paperMCPResult("委托详情已返回。", map[string]any{"order": order}), nil
	case "cancel":
		orderID, err := requirePaperStringArg(args, "orderId")
		if err != nil {
			return nil, err
		}
		order, err := store.CancelOrder(accountID, orderID)
		if err != nil {
			return nil, err
		}
		return paperMCPResult("委托已撤销。", map[string]any{"order": order}), nil
	default:
		return nil, fmt.Errorf("unsupported paper order action: %s", action)
	}
}

func callPaperPortfolioMCP(args map[string]any) (map[string]any, error) {
	store, err := requirePaperMCPStore()
	if err != nil {
		return nil, err
	}
	accountID, err := requirePaperStringArg(args, "accountId")
	if err != nil {
		return nil, err
	}
	view, err := requirePaperStringArg(args, "view")
	if err != nil {
		return nil, err
	}
	code := paperStringArg(args, "code")
	from := paperStringArg(args, "from")
	to := paperStringArg(args, "to")
	limit := paperLimitArg(args)

	switch view {
	case "positions":
		positions, err := store.ListPositions(accountID)
		if err != nil {
			return nil, err
		}
		positions = filterPaperPositions(positions, code)
		return paperMCPResult("持仓已返回。", map[string]any{
			"items": emptyPaperPositions(positions),
			"count": len(positions),
		}), nil
	case "trades":
		trades, err := store.ListTrades(accountID)
		if err != nil {
			return nil, err
		}
		trades = filterPaperTrades(trades, code, from, to)
		trades = limitPaperTrades(trades, limit)
		return paperMCPResult("成交已返回。", map[string]any{
			"items": emptyPaperTrades(trades),
			"count": len(trades),
		}), nil
	case "orders":
		orders, err := store.ListOrders(accountID)
		if err != nil {
			return nil, err
		}
		orders = filterPaperOrders(orders, code, from, to)
		orders = limitPaperOrders(orders, limit)
		return paperMCPResult("委托已返回。", map[string]any{
			"items": emptyPaperOrders(orders),
			"count": len(orders),
		}), nil
	case "closed_positions":
		positions, err := store.ListClosedPositions(accountID, "all")
		if err != nil {
			return nil, err
		}
		positions = filterPaperClosedPositions(positions, code, from, to)
		positions = limitPaperClosedPositions(positions, limit)
		return paperMCPResult("已清仓记录已返回。", map[string]any{
			"items": emptyPaperClosedPositions(positions),
			"count": len(positions),
		}), nil
	case "actions":
		actions, err := listPaperAgentActions(store, limit)
		if err != nil {
			return nil, err
		}
		actions = filterPaperActions(actions, accountID, from, to)
		return paperMCPResult("操作记录已返回。", map[string]any{
			"items": actions,
			"count": len(actions),
		}), nil
	case "summary", "cash", "performance":
		return paperPortfolioSummary(store, accountID, view)
	default:
		return nil, fmt.Errorf("unsupported paper portfolio view: %s", view)
	}
}

func paperPortfolioSummary(
	store *PaperStore,
	accountID string,
	view string,
) (map[string]any, error) {
	account, err := store.GetAccount(accountID)
	if err != nil {
		return nil, err
	}
	positions, orders, trades, err := loadPaperAccountActivity(store, accountID)
	if err != nil {
		return nil, err
	}
	closedPositions, err := store.ListClosedPositions(accountID, "all")
	if err != nil {
		return nil, err
	}
	equityCurve, err := listPaperEquityCurve(store, accountID)
	if err != nil {
		return nil, err
	}
	marketValue := paperPositionCostValue(positions)
	data := map[string]any{
		"account":         account,
		"positionCount":   len(positions),
		"orderCount":      len(orders),
		"tradeCount":      len(trades),
		"closedCount":     len(closedPositions),
		"costMarketValue": marketValue,
		"totalAssets":     account.AvailableCash + account.FrozenCash + marketValue,
		"equityCurve":     equityCurve,
	}
	text := "账户汇总已返回。"
	if view == "cash" || view == "performance" {
		data["viewNote"] = view + " 首版使用 summary 兜底。"
		text = "该视图首版使用账户汇总兜底。"
	}
	return paperMCPResult(text, data), nil
}

func paperRulesMCPResult() map[string]any {
	rules := map[string]any{
		"account": []string{
			"账户创建后，initialCash 和 initialPositions 视为建账快照并锁定。",
			"只有用户明确要求时，才允许关闭或重建账户。",
			"首版 MCP 暂不执行 close/recreate，只返回未实现错误。",
		},
		"orders": map[string]any{
			"side":        []string{"buy", "sell"},
			"orderType":   []string{"market", "limit", "auction"},
			"timeInForce": []string{"day", "auction_only"},
			"quantity":    "必须为正数且是 100 的整数倍。",
			"price":       "limit/auction 委托必须提供正价格；market 委托成交时按行情撮合。",
			"cancel":      "仅 pending 委托可撤；撤单会释放对应冻结资金或冻结持仓。",
		},
		"fees": map[string]any{
			"commissionRate": paperCommissionRate,
			"stampTaxRate":   paperStampTaxRate,
			"transferRate":   paperTransferRate,
			"stampTax":       "仅股票卖出收取。",
			"transferFee":    "仅股票收取。",
		},
		"matching": []string{
			"首版不做部分成交；一笔委托要么整笔成交，要么保持 pending。",
			"买入限价在行情价小于等于委托价时成交；卖出限价在行情价大于等于委托价时成交。",
			"买入成交后增加总持仓，但当天不增加可卖持仓，按 A股 T+1 口径处理。",
		},
	}
	text := strings.Join([]string{
		"纸上交易规则：",
		"1. 账户创建后初始资金和初始持仓锁定，close/recreate 必须由用户明确要求。",
		"2. 委托方向支持 buy/sell，类型支持 market/limit/auction。",
		"3. 数量必须为正数且是 100 的整数倍；limit/auction 必须有正价格。",
		"4. 费用包含佣金、股票过户费，股票卖出另收印花税。",
		"5. 买入成交后当天不可卖，服务端按 A股 T+1 可卖口径约束。",
		"6. 首版不支持部分成交；close/recreate 暂未实现。",
	}, "\n")
	return paperMCPResult(text, map[string]any{"rules": rules})
}

func paperMCPResult(text string, data map[string]any) map[string]any {
	return map[string]any{
		"content":           []map[string]string{{"type": "text", "text": text}},
		"structuredContent": data,
	}
}

func requirePaperMCPStore() (*PaperStore, error) {
	if paperStore == nil {
		return nil, errors.New("paper store is unavailable")
	}
	return paperStore, nil
}

func decodePaperMCPArgs(args map[string]any, target any) error {
	b, err := json.Marshal(args)
	if err != nil {
		return fmt.Errorf("paper MCP arguments encode failed: %w", err)
	}
	if err := json.Unmarshal(b, target); err != nil {
		return fmt.Errorf("paper MCP arguments decode failed: %w", err)
	}
	return nil
}

func requirePaperStringArg(args map[string]any, name string) (string, error) {
	value := paperStringArg(args, name)
	if value == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return value, nil
}

func paperStringArg(args map[string]any, name string) string {
	value, ok := args[name]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func paperLimitArg(args map[string]any) int {
	const defaultLimit = 50
	const maxLimit = 200

	value, ok := args["limit"]
	if !ok || value == nil {
		return defaultLimit
	}
	var limit int
	switch v := value.(type) {
	case int:
		limit = v
	case int64:
		limit = int(v)
	case float64:
		limit = int(v)
	case json.Number:
		n, _ := v.Int64()
		limit = int(n)
	case string:
		_, _ = fmt.Sscan(v, &limit)
	}
	if limit <= 0 {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}

func filterPaperPositions(items []PaperPosition, code string) []PaperPosition {
	if code == "" {
		return items
	}
	out := []PaperPosition{}
	for _, item := range items {
		if item.Code == code {
			out = append(out, item)
		}
	}
	return out
}

func filterPaperOrders(
	items []PaperOrder,
	code string,
	from string,
	to string,
) []PaperOrder {
	out := []PaperOrder{}
	for _, item := range items {
		if code != "" && item.Code != code {
			continue
		}
		if !paperTimeInRange(item.CreatedAt, from, to) {
			continue
		}
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].CreatedAt > out[j].CreatedAt
	})
	return out
}

func filterPaperTrades(
	items []PaperTrade,
	code string,
	from string,
	to string,
) []PaperTrade {
	out := []PaperTrade{}
	for _, item := range items {
		if code != "" && item.Code != code {
			continue
		}
		if !paperTimeInRange(item.TradedAt, from, to) {
			continue
		}
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].TradedAt > out[j].TradedAt
	})
	return out
}

func filterPaperClosedPositions(
	items []PaperClosedPosition,
	code string,
	from string,
	to string,
) []PaperClosedPosition {
	out := []PaperClosedPosition{}
	for _, item := range items {
		if code != "" && item.Code != code {
			continue
		}
		if !paperTimeInRange(item.ClosedAt, from, to) {
			continue
		}
		out = append(out, item)
	}
	return out
}

func filterPaperActions(
	items []PaperAgentAction,
	accountID string,
	from string,
	to string,
) []PaperAgentAction {
	out := []PaperAgentAction{}
	for _, item := range items {
		if item.AccountID != accountID {
			continue
		}
		if !paperTimeInRange(item.CreatedAt, from, to) {
			continue
		}
		out = append(out, item)
	}
	return out
}

func paperTimeInRange(value string, from string, to string) bool {
	if from != "" && value < from {
		return false
	}
	if to != "" && value > to {
		return false
	}
	return true
}

func limitPaperOrders(items []PaperOrder, limit int) []PaperOrder {
	if len(items) <= limit {
		return items
	}
	return items[:limit]
}

func limitPaperTrades(items []PaperTrade, limit int) []PaperTrade {
	if len(items) <= limit {
		return items
	}
	return items[:limit]
}

func limitPaperClosedPositions(
	items []PaperClosedPosition,
	limit int,
) []PaperClosedPosition {
	if len(items) <= limit {
		return items
	}
	return items[:limit]
}

func paperPositionCostValue(positions []PaperPosition) float64 {
	total := 0.0
	for _, position := range positions {
		total += position.AvgCost * float64(position.Quantity)
	}
	return total
}

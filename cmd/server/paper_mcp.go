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
		"模拟账户和持仓校正工具。set_position 直接新增、覆盖或删除单只持仓，不改变现金、不生成成交记录；create/set_position/delete/close/recreate 必须由用户明确确认并传 confirm=true。delete 会永久删除指定账户全部数据，但不影响其他账户。",
		"",
		nil,
		requiredEnum("action", "操作：create 创建、list 列表、get 详情、set_position 校正持仓、delete 永久删除、close 关闭、recreate 重建。", "create", "list", "get", "set_position", "delete", "close", "recreate"),
		optionalString("accountId", "账户ID；get/set_position/delete/close/recreate 时需要。"),
		optionalString("name", "账户名称，create 时需要。"),
		optionalNumberSchema("initialCash", "初始现金；create 可选，默认 0。", map[string]any{"minimum": 0}),
		optionalString("note", "账户备注，可记录策略、来源或说明；不参与资金和持仓计算。"),
		optionalString("reason", "持仓校正原因；set_position 时可传，将写入 Agent 行为时间线。"),
		optionalBool("confirm", "create/set_position/delete/close/recreate 等有副作用操作必须为 true。"),
	)
	accountProperties := account.InputSchema["properties"].(map[string]any)
	accountProperties["initialPositions"] = paperInitialPositionsSchema()
	accountProperties["position"] = paperPositionAdjustmentSchema()
	account.InputSchema["allOf"] = []map[string]any{
		{
			"if": map[string]any{
				"properties": map[string]any{
					"action": map[string]any{
						"enum": []string{
							"create",
							"set_position",
							"delete",
							"close",
							"recreate",
						},
					},
				},
				"required": []string{"action"},
			},
			"then": map[string]any{
				"properties": map[string]any{
					"confirm": map[string]any{"const": true},
				},
				"required": []string{"confirm"},
			},
		},
		{
			"if": map[string]any{
				"properties": map[string]any{
					"action": map[string]any{
						"enum": []string{"get", "delete", "close", "recreate"},
					},
				},
				"required": []string{"action"},
			},
			"then": map[string]any{
				"required": []string{"accountId"},
			},
		},
		{
			"if": map[string]any{
				"properties": map[string]any{
					"action": map[string]any{"const": "set_position"},
				},
				"required": []string{"action"},
			},
			"then": map[string]any{
				"required": []string{"accountId", "position"},
			},
		},
	}

	order := newMCPTool(
		"tdx_paper_order",
		"模拟交易记录工具。place 按 Agent 提供的 price 立即成交并自动更新现金、持仓、费用、成交、清仓表现和资产快照，不读取实时行情、不判断交易时段；place/cancel 必须由用户明确确认并传 confirm=true。",
		"",
		nil,
		requiredEnum("action", "操作：place 立即记录成交、cancel 撤销历史遗留待成交委托、list 列表、get 详情。", "place", "cancel", "list", "get"),
		requiredString("accountId", "账户ID，所有订单操作必填。"),
		optionalString("code", "证券代码；place 时需要。"),
		optionalEnum("side", "买卖方向；place 时必填。", "buy", "sell"),
		optionalNumberSchema("price", "本次记录的实际成交价；place 时必填且必须大于 0，服务端不会查询行情替代该价格。", map[string]any{"exclusiveMinimum": 0}),
		optionalInteger("quantity", "委托数量；place 时必填，必须为100的整数倍。", map[string]any{"minimum": 100, "multipleOf": 100}),
		optionalString("orderId", "委托 ID；get/cancel 时需要。"),
		optionalString("name", "证券名称，可选；用于展示和记录。"),
		optionalEnumDefault("assetType", "资产类型，默认 stock；当前支持 stock/etf。", "stock", "stock", "etf"),
		optionalString("reason", "交易理由；place 时可传，将写入 Agent 行为时间线。"),
		optionalBool("confirm", "place/cancel 等有副作用操作必须为 true。"),
	)
	order.InputSchema["allOf"] = []map[string]any{
		{
			"if": map[string]any{
				"properties": map[string]any{
					"action": map[string]any{"const": "place"},
				},
				"required": []string{"action"},
			},
			"then": map[string]any{
				"required": []string{"code", "side", "price", "quantity"},
			},
		},
		{
			"if": map[string]any{
				"properties": map[string]any{
					"action": map[string]any{"enum": []string{"get", "cancel"}},
				},
				"required": []string{"action"},
			},
			"then": map[string]any{
				"required": []string{"orderId"},
			},
		},
		{
			"if": map[string]any{
				"properties": map[string]any{
					"action": map[string]any{"enum": []string{"place", "cancel"}},
				},
				"required": []string{"action"},
			},
			"then": map[string]any{
				"properties": map[string]any{
					"confirm": map[string]any{"const": true},
				},
				"required": []string{"confirm"},
			},
		},
	}

	portfolio := newMCPTool(
		"tdx_paper_portfolio",
		"纸上交易账户查询工具。交易决策前必须使用固定 accountId 查询 positions 和 orders；positions 返回当前持仓、可卖数量和冻结数量。支持 summary/cash/positions/trades/orders/performance/closed_positions/actions。",
		"",
		nil,
		requiredString("accountId", "账户ID，查询账户视图时必填。"),
		requiredEnum("view", "查询视图：summary/cash/positions/trades/orders/performance/closed_positions/actions。", "summary", "cash", "positions", "trades", "orders", "performance", "closed_positions", "actions"),
		optionalDateString("from", "起始日期，YYYY-MM-DD或YYYYMMDD；按字符串日期过滤。"),
		optionalDateString("to", "结束日期，YYYY-MM-DD或YYYYMMDD；按字符串日期过滤。"),
		optionalIntegerDefault("limit", "最多返回条数，默认 50，最大 200。", 50, 1, 200),
		optionalString("code", "可选证券代码过滤，仅返回该证券相关记录。"),
	)

	rules := newMCPTool(
		"tdx_paper_rules",
		"返回模拟交易记录、持仓账务校正、费用和账户生命周期规则。",
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
					"type":        "integer",
					"description": "持仓数量，必须为正整数。",
					"minimum":     1,
				},
				"costPrice": map[string]any{
					"type":        "number",
					"description": "成本价，不能为负数。",
					"minimum":     0,
				},
				"buyDate": map[string]any{
					"type":        "string",
					"description": "买入日期，YYYY-MM-DD或YYYYMMDD。",
					"pattern":     `^(\d{4}-\d{2}-\d{2}|\d{8})$`,
				},
			},
			"required": []string{"code", "quantity"},
		},
	}
}

func paperPositionAdjustmentSchema() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "持仓账务校正。quantity 大于 0 时新增或覆盖；等于 0 时删除。该操作不改变现金，也不生成成交记录。",
		"properties": map[string]any{
			"code": map[string]any{
				"type":        "string",
				"description": "证券代码。",
			},
			"securityName": map[string]any{
				"type":        "string",
				"description": "证券名称，可选；覆盖时不传则保留原名称。",
			},
			"assetType": map[string]any{
				"type":        "string",
				"description": "资产类型，默认 stock；ETF 必须明确传 etf。",
				"enum":        []string{"stock", "etf"},
				"default":     "stock",
			},
			"quantity": map[string]any{
				"type":        "integer",
				"description": "校正后的绝对持仓数量；0 表示删除，允许零股和碎股校正。",
				"minimum":     0,
			},
			"costPrice": map[string]any{
				"type":        "number",
				"description": "校正后的单位成本；quantity 大于 0 时必填。",
				"minimum":     0,
			},
		},
		"required": []string{"code", "quantity"},
		"allOf": []map[string]any{
			{
				"if": map[string]any{
					"properties": map[string]any{
						"quantity": map[string]any{"minimum": 1},
					},
					"required": []string{"quantity"},
				},
				"then": map[string]any{
					"required": []string{"costPrice"},
				},
			},
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
		if err := requirePaperConfirm(args); err != nil {
			return nil, err
		}
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
	case "set_position":
		if err := requirePaperConfirm(args); err != nil {
			return nil, err
		}
		var req PaperSetPositionRequest
		if err := decodePaperMCPArgs(args, &req); err != nil {
			return nil, err
		}
		position, operation, err := store.SetPosition(req)
		if err != nil {
			return nil, err
		}
		return paperMCPResult(
			"持仓账务校正已完成；现金和成交记录未改变。",
			map[string]any{"operation": operation, "position": position},
		), nil
	case "delete":
		if err := requirePaperConfirm(args); err != nil {
			return nil, err
		}
		accountID, err := requirePaperStringArg(args, "accountId")
		if err != nil {
			return nil, err
		}
		if err := store.DeleteAccount(accountID); err != nil {
			return nil, err
		}
		return paperMCPResult(
			"账户及其全部模拟交易数据已永久删除。",
			map[string]any{"accountId": accountID},
		), nil
	case "close", "recreate":
		if err := requirePaperConfirm(args); err != nil {
			return nil, err
		}
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
		if err := requirePaperConfirm(args); err != nil {
			return nil, err
		}
		var req PaperPlaceOrderRequest
		if err := decodePaperMCPArgs(args, &req); err != nil {
			return nil, err
		}
		order, trade, err := store.ExecuteTrade(req)
		if err != nil {
			return nil, err
		}
		return paperMCPResult(
			"交易已按指定价格立即成交，账户数据已更新。",
			map[string]any{"order": order, "trade": trade},
		), nil
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
		if err := requirePaperConfirm(args); err != nil {
			return nil, err
		}
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
		actions, err := listPaperAgentActions(store, accountID, limit)
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
	equityCurve, err := listPaperEquityCurve(store, accountID, "all")
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
			"set_position 只校正当前持仓，不修改 initialPositions、现金或成交记录。",
			"只有用户明确要求时，才允许关闭或重建账户。",
			"首版 MCP 暂不执行 close/recreate，只返回未实现错误。",
		},
		"orders": map[string]any{
			"side":     []string{"buy", "sell"},
			"quantity": "必须为正数且是 100 的整数倍。",
			"price":    "必须提供正成交价；服务端不读取行情替代该价格。",
			"cancel":   "仅用于撤销旧版本遗留的 pending 委托。",
		},
		"fees": map[string]any{
			"commissionRate": paperCommissionRate,
			"stampTaxRate":   paperStampTaxRate,
			"transferRate":   paperTransferRate,
			"stampTax":       "仅股票卖出收取。",
			"transferFee":    "仅股票收取。",
		},
		"execution": []string{
			"place 使用 Agent 提供的 price 立即整笔成交，不判断日期和交易时段。",
			"服务端不轮询 TDX 行情，不支持挂单等待价格触发。",
			"买入后持仓立即可卖；卖出时只校验账户当前可卖数量。",
			"每次成交自动更新现金、positions、orders、trades、费用、清仓表现和资产快照。",
			"Agent 交易决策前必须用固定 accountId 查询 positions 和 orders；服务端以 SQLite 状态为准。",
		},
		"positionAdjustment": []string{
			"set_position 的 quantity 大于 0 时新增或覆盖绝对持仓，小于 0 会被拒绝。",
			"quantity 等于 0 时删除该持仓；ETF 必须传 assetType=etf。",
			"数量大于 0 时必须提供 costPrice；允许碎股校正。",
			"校正会写入持仓流水、Agent 行为和账面资产快照，但不改变现金或生成成交记录。",
			"存在冻结数量时拒绝校正，需先撤销旧版本遗留的 pending 委托。",
		},
	}
	text := strings.Join([]string{
		"纸上交易规则：",
		"1. 账户创建后初始资金和初始持仓锁定，close/recreate 必须由用户明确要求。",
		"2. place 支持 buy/sell，必须提供正数 price，并按指定价格立即成交。",
		"3. 数量必须为正数且是 100 的整数倍。",
		"4. 费用包含佣金、股票过户费，股票卖出另收印花税。",
		"5. 买入后持仓立即可卖，卖出只校验当前可卖数量。",
		"6. 服务端不读取实时行情、不判断交易时段，也不运行定时撮合。",
		"7. Agent 交易决策前先查询 positions 和 orders，服务端以 SQLite 状态为准。",
		"8. place 按整笔成交记录；close/recreate 暂未实现。",
		"9. set_position 用于账务校正，不改变现金或生成成交记录；quantity=0 表示删除持仓。",
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

func requirePaperConfirm(args map[string]any) error {
	value, ok := args["confirm"].(bool)
	if !ok || !value {
		return errors.New("confirm must be true for side-effect paper operations")
	}
	return nil
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

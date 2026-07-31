package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

type PaperPortfolioQuote struct {
	Price      float64 `json:"price"`
	DataDate   string  `json:"dataDate"`
	DataStatus string  `json:"dataStatus"`
}

type PaperPortfolioAccount struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Status        string  `json:"status"`
	InitialCash   float64 `json:"initialCash"`
	AvailableCash float64 `json:"availableCash"`
	FrozenCash    float64 `json:"frozenCash"`
}

type PaperAccountRef struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type PaperPortfolioAssets struct {
	PositionCost        float64  `json:"positionCost"`
	PositionMarketValue *float64 `json:"positionMarketValue"`
	TotalAssets         *float64 `json:"totalAssets"`
	TradingFees         float64  `json:"tradingFees"`
}

type PaperPortfolioTrade struct {
	TradedAt    string  `json:"tradedAt"`
	Side        string  `json:"side"`
	Quantity    int64   `json:"quantity"`
	Price       float64 `json:"price"`
	Amount      float64 `json:"amount"`
	Fee         float64 `json:"fee"`
	Commission  float64 `json:"commission"`
	StampTax    float64 `json:"stampTax"`
	TransferFee float64 `json:"transferFee"`
}

type PaperPortfolioStock struct {
	Code             string                `json:"code"`
	Name             string                `json:"name,omitempty"`
	AssetType        string                `json:"assetType,omitempty"`
	PositionStatus   string                `json:"positionStatus"`
	Quantity         int64                 `json:"quantity"`
	SellableQuantity int64                 `json:"sellableQuantity"`
	FrozenQuantity   int64                 `json:"frozenQuantity"`
	AverageCost      float64               `json:"averageCost"`
	TotalCost        float64               `json:"totalCost"`
	LatestPrice      *float64              `json:"latestPrice"`
	MarketValue      *float64              `json:"marketValue"`
	UnrealizedProfit *float64              `json:"unrealizedProfit"`
	MarketDataDate   string                `json:"marketDataDate,omitempty"`
	MarketDataStatus string                `json:"marketDataStatus,omitempty"`
	Trades           []PaperPortfolioTrade `json:"trades"`
}

type PaperPortfolioFreshness struct {
	QueryTime        string `json:"queryTime"`
	MarketDataDate   string `json:"marketDataDate"`
	MarketDataStatus string `json:"marketDataStatus"`
}

type PaperPortfolioSnapshot struct {
	Account   PaperPortfolioAccount   `json:"account"`
	Assets    PaperPortfolioAssets    `json:"assets"`
	Stocks    []PaperPortfolioStock   `json:"stocks"`
	Freshness PaperPortfolioFreshness `json:"freshness"`
	Warnings  []string                `json:"warnings,omitempty"`
}

var paperPortfolioQuoteLoader = loadPaperPortfolioQuotes

func paperMCPTools() []mcpTool {
	account := newMCPTool(
		"tdx_paper_account",
		"模拟账户生命周期和持仓校正工具。list用于发现账户ID；账户、现金、持仓、成本、总资产和成交历史统一由tdx_paper_portfolio查询。set_position直接新增、覆盖或删除单只持仓，不改变现金、不生成成交记录；create/set_position/delete必须由用户明确确认并传confirm=true。delete会永久删除指定账户全部数据但不影响其他账户；重建时由用户明确要求后依次调用delete和create。",
		"",
		nil,
		requiredEnum("action", "操作：create创建、list列出账户、set_position校正持仓、delete永久删除。账户详情统一调用tdx_paper_portfolio；账户重建使用delete后再create。", "create", "list", "set_position", "delete"),
		optionalString("accountId", "账户ID；set_position/delete时需要。"),
		optionalString("name", "账户名称，create 时需要。"),
		optionalNumberSchema("initialCash", "初始现金；create 可选，默认 0。", map[string]any{"minimum": 0}),
		optionalString("note", "账户备注，可记录策略、来源或说明；不参与资金和持仓计算。"),
		optionalString("reason", "持仓校正原因；set_position 时可传，将写入 Agent 行为时间线。"),
		optionalBool("confirm", "create/set_position/delete 等有副作用操作必须为 true。"),
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
						"const": "delete",
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
		"模拟交易记录工具。place按Agent提供的price立即成交并自动更新现金、持仓、费用、成交、清仓表现和资产快照，不读取实时行情、不判断交易时段；get只按orderId查询单笔遗留委托，成交历史统一由tdx_paper_portfolio按股票归类返回；place/cancel必须由用户明确确认并传confirm=true。",
		"",
		nil,
		requiredEnum("action", "操作：place立即记录成交、cancel撤销历史遗留待成交委托、get查询单笔遗留委托。", "place", "cancel", "get"),
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
		"模拟账户固定快照查询。固定输出账户、现金、持仓成本、当前总资产及成交历史；stocks按股票独立归类，每只股票只出现一条并包含自己的持仓和成交记录，已清仓股票仍可按历史出现。成交记录不返回交易理由。当前持仓调用时使用TDX最新行情估值：交易时段计入实时行情，非交易时间使用最近截止行情并明确日期和状态；任一持仓行情缺失时总资产返回null，不使用成本冒充市值。",
		"",
		nil,
		requiredString("accountId", "账户ID，查询固定账户快照时必填。"),
		optionalDateString("from", "成交历史起始日期，YYYY-MM-DD或YYYYMMDD。"),
		optionalDateString("to", "成交历史结束日期，YYYY-MM-DD或YYYYMMDD。"),
		optionalIntegerDefault("limit", "每只股票最多返回的最近成交数量，默认20，最大200。", 20, 1, 200),
		optionalString("code", "可选股票代码；仅筛选stocks及成交历史，账户级资产汇总仍按完整账户计算。"),
	)
	portfolio.OutputSchema = paperPortfolioOutputSchema()

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

func paperPortfolioOutputSchema() map[string]any {
	trade := map[string]any{
		"type":        "object",
		"description": "单笔成交记录，不包含交易理由。",
		"properties": map[string]any{
			"tradedAt":    map[string]any{"type": "string", "description": "成交时间。"},
			"side":        map[string]any{"type": "string", "enum": []string{"buy", "sell"}},
			"quantity":    map[string]any{"type": "integer"},
			"price":       map[string]any{"type": "number"},
			"amount":      map[string]any{"type": "number"},
			"fee":         map[string]any{"type": "number", "description": "本笔总费用。"},
			"commission":  map[string]any{"type": "number"},
			"stampTax":    map[string]any{"type": "number"},
			"transferFee": map[string]any{"type": "number"},
		},
	}
	stock := map[string]any{
		"type":        "object",
		"description": "一只股票的独立状态；持仓和成交历史按代码归入同一条。",
		"properties": map[string]any{
			"code":           map[string]any{"type": "string"},
			"name":           map[string]any{"type": "string"},
			"assetType":      map[string]any{"type": "string"},
			"positionStatus": map[string]any{"type": "string", "enum": []string{"holding", "closed"}},
			"quantity":       map[string]any{"type": "integer"},
			"sellableQuantity": map[string]any{
				"type":        "integer",
				"description": "当前可卖数量。",
			},
			"frozenQuantity": map[string]any{"type": "integer"},
			"averageCost":    map[string]any{"type": "number"},
			"totalCost":      map[string]any{"type": "number"},
			"latestPrice": map[string]any{
				"type":        []string{"number", "null"},
				"description": "持仓最新价格；行情不可用或已清仓时为null。",
			},
			"marketValue": map[string]any{
				"type":        []string{"number", "null"},
				"description": "当前持仓市值；行情不可用或已清仓时为null。",
			},
			"unrealizedProfit": map[string]any{
				"type":        []string{"number", "null"},
				"description": "当前未实现盈亏；行情不可用或已清仓时为null。",
			},
			"marketDataDate":   map[string]any{"type": "string"},
			"marketDataStatus": map[string]any{"type": "string"},
			"trades": map[string]any{
				"type":        "array",
				"description": "该股票最近成交历史，不包含交易理由。",
				"items":       trade,
			},
		},
	}
	snapshot := map[string]any{
		"type":        "object",
		"description": "固定账户快照；不需要view参数。",
		"properties": map[string]any{
			"account": map[string]any{
				"type":        "object",
				"description": "账户和现金状态。",
				"properties": map[string]any{
					"id":            map[string]any{"type": "string"},
					"name":          map[string]any{"type": "string"},
					"status":        map[string]any{"type": "string"},
					"initialCash":   map[string]any{"type": "number"},
					"availableCash": map[string]any{"type": "number"},
					"frozenCash":    map[string]any{"type": "number"},
				},
			},
			"assets": map[string]any{
				"type":        "object",
				"description": "持仓成本、市值、交易费用和总资产；行情缺失时市值及总资产为null。",
				"properties": map[string]any{
					"positionCost": map[string]any{
						"type":        "number",
						"description": "当前持仓总成本，平均成本已包含买入费用。",
					},
					"positionMarketValue": map[string]any{
						"type":        []string{"number", "null"},
						"description": "当前持仓总市值。",
					},
					"totalAssets": map[string]any{
						"type":        []string{"number", "null"},
						"description": "可用现金+冻结现金+当前持仓总市值。",
					},
					"tradingFees": map[string]any{
						"type":        "number",
						"description": "账户全部成交累计佣金、印花税和过户费。",
					},
				},
			},
			"stocks": map[string]any{
				"type":  "array",
				"items": stock,
			},
			"freshness": map[string]any{
				"type":        "object",
				"description": "查询时间、行情日期和行情状态。",
				"properties": map[string]any{
					"queryTime":        map[string]any{"type": "string"},
					"marketDataDate":   map[string]any{"type": "string"},
					"marketDataStatus": map[string]any{"type": "string"},
				},
			},
			"warnings": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
		},
	}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"content": map[string]any{
				"type":        "array",
				"description": "简短调用结果文本。",
			},
			"structuredContent": snapshot,
		},
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
			"items": paperAccountRefs(accounts),
			"count": len(accounts),
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
	if paperStringArg(args, "view") != "" {
		return nil, errors.New("view has been removed; portfolio now returns a fixed snapshot")
	}
	accountID, err := requirePaperStringArg(args, "accountId")
	if err != nil {
		return nil, err
	}
	code := paperStringArg(args, "code")
	from, err := normalizePaperDateFilter(paperStringArg(args, "from"))
	if err != nil {
		return nil, fmt.Errorf("invalid from: %w", err)
	}
	to, err := normalizePaperDateFilter(paperStringArg(args, "to"))
	if err != nil {
		return nil, fmt.Errorf("invalid to: %w", err)
	}
	if from != "" && to != "" && from > to {
		return nil, errors.New("from must not be later than to")
	}
	limit := paperLimitArg(args)
	snapshot, err := buildPaperPortfolioSnapshot(
		store,
		accountID,
		code,
		from,
		to,
		limit,
		time.Now(),
	)
	if err != nil {
		return nil, err
	}
	return paperMCPResult(
		fmt.Sprintf("账户快照已返回，共%d只相关股票。", len(snapshot.Stocks)),
		snapshot,
	), nil
}

func paperAccountRefs(accounts []PaperAccount) []PaperAccountRef {
	refs := make([]PaperAccountRef, 0, len(accounts))
	for _, account := range accounts {
		refs = append(refs, PaperAccountRef{
			ID:     account.ID,
			Name:   account.Name,
			Status: account.Status,
		})
	}
	return refs
}

func buildPaperPortfolioSnapshot(
	store *PaperStore,
	accountID string,
	code string,
	from string,
	to string,
	limit int,
	now time.Time,
) (PaperPortfolioSnapshot, error) {
	account, err := store.GetAccount(accountID)
	if err != nil {
		return PaperPortfolioSnapshot{}, err
	}
	positions, err := store.ListPositions(accountID)
	if err != nil {
		return PaperPortfolioSnapshot{}, err
	}
	orders, err := store.ListOrders(accountID)
	if err != nil {
		return PaperPortfolioSnapshot{}, err
	}
	allTrades, err := store.ListTrades(accountID)
	if err != nil {
		return PaperPortfolioSnapshot{}, err
	}

	snapshot := PaperPortfolioSnapshot{
		Account: PaperPortfolioAccount{
			ID:            account.ID,
			Name:          account.Name,
			Status:        account.Status,
			InitialCash:   roundPaperPortfolioNumber(account.InitialCash),
			AvailableCash: roundPaperPortfolioNumber(account.AvailableCash),
			FrozenCash:    roundPaperPortfolioNumber(account.FrozenCash),
		},
		Stocks: []PaperPortfolioStock{},
		Freshness: PaperPortfolioFreshness{
			QueryTime: now.Format(time.RFC3339),
		},
	}

	for _, trade := range allTrades {
		snapshot.Assets.TradingFees += paperTradeFee(trade)
	}
	snapshot.Assets.TradingFees = roundPaperPortfolioNumber(snapshot.Assets.TradingFees)

	positionCodes := make([]string, 0, len(positions))
	for i := range positions {
		positions[i].Code = normalizeStockCode(positions[i].Code)
		positionCodes = append(positionCodes, positions[i].Code)
		snapshot.Assets.PositionCost +=
			positions[i].AvgCost * float64(positions[i].Quantity)
	}
	snapshot.Assets.PositionCost = roundPaperPortfolioNumber(
		snapshot.Assets.PositionCost,
	)
	sort.Strings(positionCodes)

	quotes := map[string]PaperPortfolioQuote{}
	if len(positionCodes) > 0 {
		quotes, err = paperPortfolioQuoteLoader(positionCodes, now)
		if err != nil {
			snapshot.Warnings = append(
				snapshot.Warnings,
				"持仓行情获取失败："+err.Error(),
			)
		}
	}
	applyPaperPortfolioAssetValues(&snapshot, positions, quotes)

	stocks := map[string]*PaperPortfolioStock{}
	ensureStock := func(stockCode string) *PaperPortfolioStock {
		stockCode = normalizeStockCode(stockCode)
		stock, ok := stocks[stockCode]
		if !ok {
			stock = &PaperPortfolioStock{
				Code:           stockCode,
				PositionStatus: "closed",
				Trades:         []PaperPortfolioTrade{},
			}
			stocks[stockCode] = stock
		}
		return stock
	}

	filterCode := normalizeStockCode(code)
	for _, position := range positions {
		if filterCode != "" && filterCode != position.Code {
			continue
		}
		stock := ensureStock(position.Code)
		stock.Name = position.Name
		stock.AssetType = position.AssetType
		stock.PositionStatus = "holding"
		stock.Quantity = position.Quantity
		stock.SellableQuantity = position.SellableQuantity
		stock.FrozenQuantity = position.FrozenQuantity
		stock.AverageCost = roundPaperPortfolioNumber(position.AvgCost)
		stock.TotalCost = roundPaperPortfolioNumber(
			position.AvgCost * float64(position.Quantity),
		)
		applyPaperPortfolioStockQuote(stock, quotes[position.Code])
	}

	filteredTrades := filterPaperTrades(allTrades, filterCode, from, to)
	tradeCounts := map[string]int{}
	for _, trade := range filteredTrades {
		stockCode := normalizeStockCode(trade.Code)
		if tradeCounts[stockCode] >= limit {
			continue
		}
		stock := ensureStock(stockCode)
		stock.Trades = append(stock.Trades, paperPortfolioTrade(trade))
		tradeCounts[stockCode]++
	}
	for _, order := range orders {
		stockCode := normalizeStockCode(order.Code)
		stock, ok := stocks[stockCode]
		if !ok {
			continue
		}
		if stock.Name == "" {
			stock.Name = order.Name
		}
		if stock.AssetType == "" {
			stock.AssetType = order.AssetType
		}
	}

	stockCodes := make([]string, 0, len(stocks))
	for stockCode := range stocks {
		stockCodes = append(stockCodes, stockCode)
	}
	sort.Strings(stockCodes)
	for _, stockCode := range stockCodes {
		stock := stocks[stockCode]
		if stock.Name == "" {
			stock.Name = queryStockName(stock.Code)
		}
		snapshot.Stocks = append(snapshot.Stocks, *stock)
	}
	return snapshot, nil
}

func applyPaperPortfolioAssetValues(
	snapshot *PaperPortfolioSnapshot,
	positions []PaperPosition,
	quotes map[string]PaperPortfolioQuote,
) {
	if len(positions) == 0 {
		marketValue := 0.0
		totalAssets := snapshot.Account.AvailableCash + snapshot.Account.FrozenCash
		snapshot.Assets.PositionMarketValue = &marketValue
		snapshot.Assets.TotalAssets = &totalAssets
		snapshot.Freshness.MarketDataStatus = "无当前持仓，无需行情估值"
		return
	}

	marketValue := 0.0
	dates := map[string]struct{}{}
	statuses := map[string]struct{}{}
	missing := make([]string, 0)
	for _, position := range positions {
		quote, ok := quotes[normalizeStockCode(position.Code)]
		if !ok || quote.Price <= 0 {
			missing = append(missing, position.Code)
			continue
		}
		marketValue += quote.Price * float64(position.Quantity)
		if quote.DataDate != "" {
			dates[quote.DataDate] = struct{}{}
		}
		if quote.DataStatus != "" {
			statuses[quote.DataStatus] = struct{}{}
		}
	}
	if len(missing) > 0 {
		snapshot.Freshness.MarketDataStatus =
			"部分持仓行情不可用，总资产不可计算"
		snapshot.Warnings = append(
			snapshot.Warnings,
			"以下持仓缺少有效行情："+strings.Join(missing, "、"),
		)
		return
	}

	marketValue = roundPaperPortfolioNumber(marketValue)
	totalAssets :=
		snapshot.Account.AvailableCash + snapshot.Account.FrozenCash + marketValue
	totalAssets = roundPaperPortfolioNumber(totalAssets)
	snapshot.Assets.PositionMarketValue = &marketValue
	snapshot.Assets.TotalAssets = &totalAssets
	snapshot.Freshness.MarketDataDate = joinPaperPortfolioValues(dates)
	snapshot.Freshness.MarketDataStatus = joinPaperPortfolioValues(statuses)
}

func applyPaperPortfolioStockQuote(
	stock *PaperPortfolioStock,
	quote PaperPortfolioQuote,
) {
	if quote.Price <= 0 {
		stock.MarketDataStatus = "行情不可用"
		return
	}
	price := roundPaperPortfolioNumber(quote.Price)
	marketValue := roundPaperPortfolioNumber(price * float64(stock.Quantity))
	unrealizedProfit := roundPaperPortfolioNumber(marketValue - stock.TotalCost)
	stock.LatestPrice = &price
	stock.MarketValue = &marketValue
	stock.UnrealizedProfit = &unrealizedProfit
	stock.MarketDataDate = quote.DataDate
	stock.MarketDataStatus = quote.DataStatus
}

func loadPaperPortfolioQuotes(
	codes []string,
	now time.Time,
) (map[string]PaperPortfolioQuote, error) {
	c := cli()
	if c == nil {
		return nil, errors.New("TDX客户端未连接")
	}
	quotes, err := c.GetQuote(codes...)
	if err != nil {
		return nil, err
	}
	result := make(map[string]PaperPortfolioQuote, len(quotes))
	for _, quote := range quotes {
		if quote == nil || quote.Kline == nil {
			continue
		}
		result[normalizeStockCode(quote.Code)] = PaperPortfolioQuote{
			Price:      quote.Kline.Close.Float64(),
			DataDate:   quoteKlineDataDate(quote.Kline),
			DataStatus: quoteKlineDataStatus(quote.Kline, now),
		}
	}
	return result, nil
}

func paperPortfolioTrade(trade PaperTrade) PaperPortfolioTrade {
	return PaperPortfolioTrade{
		TradedAt:    trade.TradedAt,
		Side:        trade.Side,
		Quantity:    trade.Quantity,
		Price:       roundPaperPortfolioNumber(trade.Price),
		Amount:      roundPaperPortfolioNumber(trade.Amount),
		Fee:         roundPaperPortfolioNumber(paperTradeFee(trade)),
		Commission:  roundPaperPortfolioNumber(trade.Commission),
		StampTax:    roundPaperPortfolioNumber(trade.StampTax),
		TransferFee: roundPaperPortfolioNumber(trade.TransferFee),
	}
}

func paperTradeFee(trade PaperTrade) float64 {
	return trade.Commission + trade.StampTax + trade.TransferFee
}

func roundPaperPortfolioNumber(value float64) float64 {
	return math.Round(value*10000) / 10000
}

func joinPaperPortfolioValues(values map[string]struct{}) string {
	items := make([]string, 0, len(values))
	for value := range values {
		items = append(items, value)
	}
	sort.Strings(items)
	return strings.Join(items, "；")
}

func normalizePaperDateFilter(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	for _, layout := range []string{"2006-01-02", "20060102"} {
		parsed, err := time.ParseInLocation(layout, value, time.Local)
		if err == nil {
			return parsed.Format(time.DateOnly), nil
		}
	}
	return "", errors.New("date must be YYYY-MM-DD or YYYYMMDD")
}

func paperRulesMCPResult() map[string]any {
	rules := map[string]any{
		"account": []string{
			"账户创建后，initialCash 和 initialPositions 视为建账快照并锁定。",
			"set_position 只校正当前持仓，不修改 initialPositions、现金或成交记录。",
			"只有用户明确要求时，才允许永久删除账户；重建使用 delete 后再 create。",
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
			"Agent交易决策前必须用固定accountId调用tdx_paper_portfolio获取账户、持仓和成交历史；服务端以SQLite状态为准。",
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
		"1. 账户创建后初始资金和初始持仓锁定；删除或重建必须由用户明确要求。",
		"2. place 支持 buy/sell，必须提供正数 price，并按指定价格立即成交。",
		"3. 数量必须为正数且是 100 的整数倍。",
		"4. 费用包含佣金、股票过户费，股票卖出另收印花税。",
		"5. 买入后持仓立即可卖，卖出只校验当前可卖数量。",
		"6. 服务端不读取实时行情、不判断交易时段，也不运行定时撮合。",
		"7. Agent交易决策前调用tdx_paper_portfolio获取固定账户快照，服务端以SQLite状态为准。",
		"8. 账户重建使用 delete 后再 create。",
		"9. set_position 用于账务校正，不改变现金或生成成交记录；quantity=0 表示删除持仓。",
	}, "\n")
	return paperMCPResult(text, map[string]any{"rules": rules})
}

func paperMCPResult(text string, data any) map[string]any {
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
	const defaultLimit = 20
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

func paperTimeInRange(value string, from string, to string) bool {
	date := value
	if len(date) >= len(time.DateOnly) {
		date = date[:len(time.DateOnly)]
	}
	if from != "" && date < from {
		return false
	}
	if to != "" && date > to {
		return false
	}
	return true
}

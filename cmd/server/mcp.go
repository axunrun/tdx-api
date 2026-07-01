package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
)

type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type mcpResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *mcpError       `json:"error,omitempty"`
}

type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type mcpTool struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	InputSchema map[string]any   `json:"inputSchema"`
	Path        string           `json:"-"`
	Handler     http.HandlerFunc `json:"-"`
}

type mcpToolParam struct {
	Name        string
	Type        string
	Description string
	Required    bool
	Enum        []string
	Default     any
}

type mcpToolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

func handleMCP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		jsonResp(w, map[string]any{
			"name":      "tdx-api-mcp",
			"endpoint":  "/mcp",
			"transport": "streamable-http",
		})
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req mcpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeMCPError(w, nil, -32700, "JSON解析失败: "+err.Error())
		return
	}
	if len(req.ID) == 0 && strings.HasPrefix(req.Method, "notifications/") {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	switch req.Method {
	case "initialize":
		writeMCPResult(w, req.ID, map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]any{
				"tools": map[string]any{},
			},
			"serverInfo": map[string]any{
				"name":    "tdx-api",
				"version": "1.0.0",
			},
		})
	case "tools/list":
		writeMCPResult(w, req.ID, map[string]any{"tools": mcpTools()})
	case "tools/call":
		result, err := callMCPTool(req.Params)
		if err != nil {
			writeMCPError(w, req.ID, -32602, err.Error())
			return
		}
		writeMCPResult(w, req.ID, result)
	default:
		writeMCPError(w, req.ID, -32601, "不支持的MCP方法: "+req.Method)
	}
}

func mcpTools() []mcpTool {
	tools := []mcpTool{
		newMCPTool("tdx_asset_search_text", "按名称、代码或拼音搜索A股资产。用于用户只给股票名称或模糊关键词时先确认标准代码；返回候选股票、市场、名称和主要板块。", "/api/agent/assets/search-text", handleAgentAssetsSearchText,
			requiredString("keyword", "股票名称、代码或拼音关键词"),
			optionalNumberDefault("limit", "返回数量，默认20，最大50。", 20),
		),
		newMCPTool("tdx_stock_brief_text", "单只A股快速概览。适合作为个股分析第一步；输出行情、基本面、最新财报、所属板块、估值表现和日/周/月技术指标。", "/api/agent/stock-brief-text", handleAgentStockBriefText,
			requiredString("code", "股票代码，例如300499"),
			optionalString("mkt", "市场覆盖参数，通常留空"),
		),
		newMCPTool("tdx_kline_summary_text", "K线阶段走势摘要。用于中短线和长期走势判断；输出日线、周线、月线的趋势、涨跌、波动和风险提示。", "/api/agent/kline-summary-text", handleAgentKlineSummaryText,
			requiredString("code", "股票代码，例如300499"),
			optionalEnum("level", "输出深度：brief为简版，normal为常规版，deep为深度版。默认normal。", "brief", "normal", "deep"),
			optionalNumberDefault("dayCount", "日线数量，最大500；不传则按level使用默认数量。", nil),
		),
		newMCPTool("tdx_trade_flow_estimate_text", "分档资金流估算。用于观察指定交易日超大单、大单、中单、小单的流入流出；阈值来自近200个交易日逐笔成交金额分位。", "/api/agent/trade-flow-estimate-text", handleAgentTradeFlowEstimateText,
			requiredString("code", "股票代码，例如300499"),
			optionalString("date", "日期，YYYY-MM-DD或YYYYMMDD；不传默认今天"),
		),
		newMCPTool("tdx_f10_summary_text", "低频深度F10摘要。用于深度个股研究；输出经降噪后的财务、股本、股东、机构持股、分红融资、经营分析和行业分析。", "/api/agent/f10-summary-text", handleAgentF10SummaryText,
			requiredString("code", "股票代码，例如300499"),
			optionalString("mkt", "市场覆盖参数，通常留空"),
		),
		newMCPTool("tdx_sector_membership_text", "查询个股所属板块。用于建立个股与概念、地域风格、指数板块的关联；输出完整板块归属。", "/api/agent/sector-membership-text", handleAgentSectorMembershipText,
			requiredString("code", "股票代码，例如300499"),
		),
		newMCPTool("tdx_stock_in_sector_text", "个股在板块内的相对位置。用于判断目标股相对同板块股票是强势、中游还是落后。", "/api/agent/stock-in-sector-text", handleAgentStockInSectorText,
			requiredString("code", "股票代码，例如300499"),
			optionalEnum("sectorType", "板块类型：concept为概念板块，style_region为地域/风格板块，index为指数板块；默认concept。", "concept", "style_region", "index"),
			optionalString("sectorName", "板块名称；留空时默认选择第一个概念板块"),
			optionalEnum("metric", "排序指标：changePct当日涨跌，chg5近5日，chg20近20日，chg60近60日，peTtm市盈率，divYield股息率。默认chg20。", "changePct", "chg5", "chg20", "chg60", "peTtm", "divYield"),
			optionalNumberDefault("limit", "返回成分股数量，默认10，最大50。", 10),
		),
		newMCPTool("tdx_sector_detail_text", "指定板块深度分析。用于热点扫描后继续拆板块；输出板块近20/60日表现、上涨比例、强势股、中游股和弱势股。", "/api/agent/sector-detail-text", handleAgentSectorDetailText,
			optionalString("sectorName", "板块名称；sectorName和indexCode至少传一个"),
			optionalString("indexCode", "板块指数代码；sectorName和indexCode至少传一个"),
			optionalEnum("sectorType", "板块类型：concept概念，style_region地域/风格，index指数；默认concept。", "concept", "style_region", "index"),
			optionalEnum("metric", "排序指标：changePct当日涨跌，chg5近5日，chg20近20日，chg60近60日，peTtm市盈率，divYield股息率。默认chg20。", "changePct", "chg5", "chg20", "chg60", "peTtm", "divYield"),
			optionalNumberDefault("topStocks", "强弱样本数量，默认10，最大30。", 10),
			optionalBoolDefault("excludeNew", "是否排除新股/异常涨幅样本，默认true。", true),
		),
		newMCPTool("tdx_hotspot_scan_text", "板块冷热扫描。用于市场主线、补涨方向和弱势板块识别；默认扫描概念板块，输出强势20、中游20、弱势20及代表股票。", "/api/agent/hotspot-scan-text", handleAgentHotspotScanText,
			optionalEnum("sectorType", "扫描板块类型：concept概念板块，style_region地域/风格板块，index指数板块；默认concept。", "concept", "style_region", "index"),
			optionalEnum("metric", "排序口径：chg5近5日，chg20近20日默认，chg60近60日，changePct当日涨跌，windowReturn指定日期区间收益。中长线优先chg20/chg60；指定历史区间用windowReturn并传startDate/endDate。", "chg5", "chg20", "chg60", "changePct", "windowReturn"),
			optionalString("startDate", "历史窗口开始日期，格式YYYY-MM-DD；仅metric=windowReturn时建议传。"),
			optionalString("endDate", "历史窗口结束日期，格式YYYY-MM-DD；仅metric=windowReturn时建议传。"),
			optionalNumberDefault("window", "兼容参数：窗口交易日数；新调用优先使用startDate/endDate。", nil),
			optionalNumberDefault("offset", "兼容参数：从当前往前偏移交易日数；新调用优先使用startDate/endDate。", nil),
			optionalNumberDefault("limit", "强/中/弱各返回数量，默认20，最大50。", 20),
			optionalNumberDefault("topStocks", "每个板块代表股票数量，默认3，最大10。", 3),
			optionalNumberDefault("minMembers", "最小成分股数量，默认20；低于该样本数的板块会被过滤。", 20),
			optionalBoolDefault("excludeNew", "是否排除新股/异常涨幅样本，默认true。", true),
		),
		newMCPTool("tdx_multi_brief_text", "多股快速概览。用于同时检查关注池或多只对比股票；批量输出每只股票的brief摘要。", "/api/agent/multi-brief-text", handleAgentMultiBriefText,
			requiredString("codes", "逗号分隔股票代码，最多20只，例如300499,603063"),
		),
		newMCPTool("tdx_auction_text", "集合竞价摘要。用于开盘前后判断竞价强弱；默认分析09:20-09:25不可撤单阶段。", "/api/agent/auction-text", handleAgentAuctionText,
			requiredString("code", "股票代码，例如300499"),
			optionalEnum("session", "竞价阶段：open开盘集合竞价默认，close收盘集合竞价，all全量竞价记录。", "open", "close", "all"),
			optionalNumberDefault("limit", "返回记录数量，默认20，最大100。", 20),
		),
		newMCPTool("tdx_market_review_text", "市场级复盘。用于判断A股整体环境；输出主要指数、市场广度、强/中/弱板块和可选关注股联动。", "/api/agent/market-review-text", handleAgentMarketReviewText,
			optionalEnum("session", "复盘视角：auto按查询时间自动判断，current盘中，morning上午收盘，full全天收盘。默认auto。", "auto", "current", "morning", "full"),
			optionalString("codes", "可选关注股，逗号分隔"),
			optionalNumberDefault("top", "强/中/弱板块数量，默认10，最大20。", 10),
		),
		newMCPTool("tdx_intraday_alerts_text", "关注股盘中异动快照。用于交易时段轮询关注池；输出当前行情、短时涨跌、短时放量和异动信号。", "/api/agent/intraday-alerts-text", handleAgentIntradayAlertsText,
			requiredString("codes", "逗号分隔股票代码，最多20只"),
			optionalNumberDefault("windowMinutes", "分时窗口分钟数，默认30，范围5-60。", 30),
		),
		newMCPTool("tdx_global_market_brief_text", "全球外围权重资产简报。用于A股分析前判断外围环境；输出全球指数、亚太市场、商品、汇率、债券和权重股的当日及20/60日表现。", "/api/agent/global-market-brief-text", handleAgentGlobalMarketBriefText),
	}
	return append(tools, paperMCPTools()...)
}

func newMCPTool(name, description, path string, handler http.HandlerFunc, params ...mcpToolParam) mcpTool {
	required := make([]string, 0)
	properties := map[string]any{}
	for _, param := range params {
		property := map[string]any{
			"type":        param.Type,
			"description": param.Description,
		}
		if len(param.Enum) > 0 {
			property["enum"] = param.Enum
		}
		if param.Default != nil {
			property["default"] = param.Default
		}
		properties[param.Name] = property
		if param.Required {
			required = append(required, param.Name)
		}
	}
	return mcpTool{
		Name:        name,
		Description: description,
		Path:        path,
		Handler:     handler,
		InputSchema: map[string]any{
			"type":       "object",
			"properties": properties,
			"required":   required,
		},
	}
}

func requiredString(name, description string) mcpToolParam {
	return mcpToolParam{Name: name, Type: "string", Description: description, Required: true}
}

func optionalString(name, description string) mcpToolParam {
	return mcpToolParam{Name: name, Type: "string", Description: description}
}

func optionalEnum(name, description string, values ...string) mcpToolParam {
	return mcpToolParam{Name: name, Type: "string", Description: description, Enum: values}
}

func optionalNumber(name, description string) mcpToolParam {
	return mcpToolParam{Name: name, Type: "number", Description: description}
}

func optionalNumberDefault(name, description string, defaultValue any) mcpToolParam {
	return mcpToolParam{Name: name, Type: "number", Description: description, Default: defaultValue}
}

func optionalBool(name, description string) mcpToolParam {
	return mcpToolParam{Name: name, Type: "boolean", Description: description}
}

func optionalBoolDefault(name, description string, defaultValue bool) mcpToolParam {
	return mcpToolParam{Name: name, Type: "boolean", Description: description, Default: defaultValue}
}

func callMCPTool(raw json.RawMessage) (map[string]any, error) {
	var params mcpToolCallParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, fmt.Errorf("工具参数解析失败: %w", err)
	}
	if result, ok, err := callPaperMCPTool(params.Name, params.Arguments); ok {
		return result, err
	}
	for _, tool := range mcpTools() {
		if tool.Name == params.Name {
			return callAgentHandlerAsMCP(tool, params.Arguments)
		}
	}
	return nil, fmt.Errorf("未知工具: %s", params.Name)
}

func callAgentHandlerAsMCP(tool mcpTool, args map[string]any) (map[string]any, error) {
	req := httptest.NewRequest(http.MethodGet, tool.Path+"?"+encodeMCPQuery(args), nil)
	rec := httptest.NewRecorder()
	tool.Handler(rec, req)

	var apiResp APIResponse
	if err := json.NewDecoder(bytes.NewReader(rec.Body.Bytes())).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("%s返回不可解析JSON: %w", tool.Name, err)
	}
	if rec.Code >= http.StatusBadRequest || apiResp.Code != 0 {
		return nil, fmt.Errorf("%s调用失败: %s", tool.Name, apiResp.Message)
	}

	text := formatMCPToolText(apiResp.Data)
	return map[string]any{
		"content": []map[string]string{{"type": "text", "text": text}},
		"structuredContent": map[string]any{
			"endpoint": tool.Path,
			"data":     apiResp.Data,
		},
	}, nil
}

func encodeMCPQuery(args map[string]any) string {
	values := url.Values{}
	for key, value := range args {
		if value == nil {
			continue
		}
		switch v := value.(type) {
		case []any:
			for _, item := range v {
				values.Add(key, fmt.Sprint(item))
			}
		default:
			values.Set(key, fmt.Sprint(v))
		}
	}
	return values.Encode()
}

func formatMCPToolText(data any) string {
	if m, ok := data.(map[string]any); ok {
		if content, ok := m["content"].(string); ok && content != "" {
			return content
		}
	}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Sprint(data)
	}
	return string(b)
}

func writeMCPResult(w http.ResponseWriter, id json.RawMessage, result any) {
	writeMCPResponse(w, mcpResponse{JSONRPC: "2.0", ID: id, Result: result})
}

func writeMCPError(w http.ResponseWriter, id json.RawMessage, code int, message string) {
	writeMCPResponse(w, mcpResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &mcpError{Code: code, Message: message},
	})
}

func writeMCPResponse(w http.ResponseWriter, resp mcpResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(resp)
}

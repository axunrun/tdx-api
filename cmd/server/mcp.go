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
	return []mcpTool{
		newMCPTool("tdx_asset_search_text", "搜索A股资产，返回候选股票和主要板块。", "/api/agent/assets/search-text", handleAgentAssetsSearchText,
			requiredString("keyword", "股票名称、代码或拼音关键词"),
			optionalNumber("limit", "返回数量，默认20，最大50"),
		),
		newMCPTool("tdx_stock_brief_text", "单只A股快速概览，适合快速分析入口。", "/api/agent/stock-brief-text", handleAgentStockBriefText,
			requiredString("code", "股票代码，例如300499"),
			optionalString("mkt", "市场覆盖参数，通常留空"),
		),
		newMCPTool("tdx_kline_summary_text", "日/周/月K线聚合摘要。", "/api/agent/kline-summary-text", handleAgentKlineSummaryText,
			requiredString("code", "股票代码，例如300499"),
			optionalString("level", "brief、normal或deep"),
			optionalNumber("dayCount", "日线数量，最大500"),
		),
		newMCPTool("tdx_trade_flow_estimate_text", "按200日历史逐笔阈值估算指定日期分档资金流。", "/api/agent/trade-flow-estimate-text", handleAgentTradeFlowEstimateText,
			requiredString("code", "股票代码，例如300499"),
			optionalString("date", "日期，YYYY-MM-DD或YYYYMMDD；不传默认今天"),
		),
		newMCPTool("tdx_f10_summary_text", "低频深度F10摘要。", "/api/agent/f10-summary-text", handleAgentF10SummaryText,
			requiredString("code", "股票代码，例如300499"),
			optionalString("mkt", "市场覆盖参数，通常留空"),
		),
		newMCPTool("tdx_sector_membership_text", "查询个股所属概念、地域和指数板块。", "/api/agent/sector-membership-text", handleAgentSectorMembershipText,
			requiredString("code", "股票代码，例如300499"),
		),
		newMCPTool("tdx_stock_in_sector_text", "个股在所属板块中的相对位置。", "/api/agent/stock-in-sector-text", handleAgentStockInSectorText,
			requiredString("code", "股票代码，例如300499"),
			optionalString("sectorType", "concept、style_region或index"),
			optionalString("sectorName", "板块名称；留空时默认选择第一个概念板块"),
			optionalString("metric", "changePct、chg5、chg20、chg60、peTtm或divYield"),
			optionalNumber("limit", "返回成分股数量，默认10，最大50"),
		),
		newMCPTool("tdx_sector_detail_text", "指定板块深度分析。", "/api/agent/sector-detail-text", handleAgentSectorDetailText,
			optionalString("sectorName", "板块名称；sectorName和indexCode至少传一个"),
			optionalString("indexCode", "板块指数代码；sectorName和indexCode至少传一个"),
			optionalString("sectorType", "concept、style_region或index，默认concept"),
			optionalString("metric", "changePct、chg5、chg20、chg60、peTtm或divYield"),
			optionalNumber("topStocks", "强弱样本数量，默认10，最大30"),
			optionalBool("excludeNew", "是否排除新股/异常涨幅样本，默认true"),
		),
		newMCPTool("tdx_hotspot_scan_text", "扫描强势、中游和弱势板块。", "/api/agent/hotspot-scan-text", handleAgentHotspotScanText,
			optionalString("sectorType", "concept、style_region或index，默认concept"),
			optionalString("metric", "chg5、chg20、chg60、changePct或windowReturn"),
			optionalString("startDate", "历史窗口开始日期，YYYY-MM-DD"),
			optionalString("endDate", "历史窗口结束日期，YYYY-MM-DD"),
			optionalNumber("window", "兼容参数：窗口交易日数"),
			optionalNumber("offset", "兼容参数：从当前往前偏移交易日数"),
			optionalNumber("limit", "强/中/弱各返回数量，默认20，最大50"),
			optionalNumber("topStocks", "每个板块代表股票数量，默认3，最大10"),
			optionalNumber("minMembers", "最小成分股数量，默认20"),
			optionalBool("excludeNew", "是否排除新股/异常涨幅样本，默认true"),
		),
		newMCPTool("tdx_multi_brief_text", "批量获取多只A股brief。", "/api/agent/multi-brief-text", handleAgentMultiBriefText,
			requiredString("codes", "逗号分隔股票代码，最多20只，例如300499,603063"),
		),
		newMCPTool("tdx_auction_text", "集合竞价摘要。", "/api/agent/auction-text", handleAgentAuctionText,
			requiredString("code", "股票代码，例如300499"),
			optionalString("session", "open、close或all，默认open"),
			optionalNumber("limit", "返回记录数量，默认20，最大100"),
		),
		newMCPTool("tdx_market_review_text", "市场级复盘，含指数、广度、板块和关注股联动。", "/api/agent/market-review-text", handleAgentMarketReviewText,
			optionalString("session", "auto、current、morning或full，默认auto"),
			optionalString("codes", "可选关注股，逗号分隔"),
			optionalNumber("top", "强/中/弱板块数量，默认10，最大20"),
		),
		newMCPTool("tdx_intraday_alerts_text", "关注股盘中异动快照。", "/api/agent/intraday-alerts-text", handleAgentIntradayAlertsText,
			requiredString("codes", "逗号分隔股票代码，最多20只"),
			optionalNumber("windowMinutes", "分时窗口分钟数，默认30，范围5-60"),
		),
		newMCPTool("tdx_global_market_brief_text", "全球外围权重资产简报。", "/api/agent/global-market-brief-text", handleAgentGlobalMarketBriefText),
	}
}

func newMCPTool(name, description, path string, handler http.HandlerFunc, params ...mcpToolParam) mcpTool {
	required := make([]string, 0)
	properties := map[string]any{}
	for _, param := range params {
		properties[param.Name] = map[string]any{
			"type":        param.Type,
			"description": param.Description,
		}
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

func optionalNumber(name, description string) mcpToolParam {
	return mcpToolParam{Name: name, Type: "number", Description: description}
}

func optionalBool(name, description string) mcpToolParam {
	return mcpToolParam{Name: name, Type: "boolean", Description: description}
}

func callMCPTool(raw json.RawMessage) (map[string]any, error) {
	var params mcpToolCallParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, fmt.Errorf("工具参数解析失败: %w", err)
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

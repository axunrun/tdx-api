package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMCPInitializeAndToolsList(t *testing.T) {
	initBody := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`)
	initReq := httptest.NewRequest(http.MethodPost, "/mcp", initBody)
	initRec := httptest.NewRecorder()
	handleMCP(initRec, initReq)
	if initRec.Code != http.StatusOK {
		t.Fatalf("initialize status=%d body=%s", initRec.Code, initRec.Body.String())
	}

	var initResp map[string]any
	if err := json.Unmarshal(initRec.Body.Bytes(), &initResp); err != nil {
		t.Fatal(err)
	}
	if initResp["error"] != nil {
		t.Fatalf("initialize returned error: %v", initResp["error"])
	}

	listBody := bytes.NewBufferString(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	listReq := httptest.NewRequest(http.MethodPost, "/mcp", listBody)
	listRec := httptest.NewRecorder()
	handleMCP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("tools/list status=%d body=%s", listRec.Code, listRec.Body.String())
	}

	var listResp struct {
		Result struct {
			Tools []mcpTool `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listResp); err != nil {
		t.Fatal(err)
	}
	if len(listResp.Result.Tools) == 0 {
		t.Fatal("tools/list returned no tools")
	}
	seen := map[string]bool{}
	for _, tool := range listResp.Result.Tools {
		if tool.Name == "" || tool.Description == "" {
			t.Fatalf("tool missing name or description: %+v", tool)
		}
		if seen[tool.Name] {
			t.Fatalf("duplicate tool name: %s", tool.Name)
		}
		seen[tool.Name] = true
	}
	for _, name := range []string{
		"tdx_stock_brief_text",
		"tdx_global_market_brief_text",
		"tdx_scenario_valuation_text",
		"tdx_implied_expectation_text",
		"tdx_technical_score_text",
	} {
		if !seen[name] {
			t.Fatalf("expected tool %s missing: %+v", name, seen)
		}
	}
}

func TestMCPToolSchemasDescribeHotspotParameters(t *testing.T) {
	var hotspot *mcpTool
	for _, tool := range mcpTools() {
		if tool.Name == "tdx_hotspot_scan_text" {
			tool := tool
			hotspot = &tool
			break
		}
	}
	if hotspot == nil {
		t.Fatal("tdx_hotspot_scan_text missing")
	}
	if hotspot.Description == "" {
		t.Fatal("hotspot tool description missing")
	}
	for _, want := range []string{
		"同周期板块指数",
		"与TdxStat统计日对齐",
		"成分股上涨比例明确使用",
		"固定输出最近完整交易日板块指数单日涨跌",
	} {
		if !strings.Contains(hotspot.Description, want) {
			t.Fatalf("hotspot description missing %q: %s", want, hotspot.Description)
		}
	}

	properties := hotspot.InputSchema["properties"].(map[string]any)
	for _, name := range []string{"metric", "sectorType", "startDate", "endDate", "limit"} {
		property := properties[name].(map[string]any)
		if property["description"] == "" {
			t.Fatalf("%s description missing", name)
		}
	}
	metric := properties["metric"].(map[string]any)
	if len(metric["enum"].([]string)) == 0 {
		t.Fatal("metric enum missing")
	}
	if !hasString(metric["enum"].([]string), "dailyReturn") {
		t.Fatalf("metric enum missing dailyReturn: %+v", metric)
	}
	if !strings.Contains(metric["description"].(string), "不是盘中实时值") {
		t.Fatalf("metric description does not explain changePct timing: %+v", metric)
	}
	for _, want := range []string{"成分股平均", "板块指数", "上涨比例"} {
		if !strings.Contains(metric["description"].(string), want) {
			t.Fatalf("metric description missing %q: %+v", want, metric)
		}
	}
	if metric["default"] != "chg20" {
		t.Fatalf("metric default = %v, want chg20", metric["default"])
	}
	limit := properties["limit"].(map[string]any)
	if limit["type"] != "integer" || limit["default"] != 20 || limit["maximum"] != 50 {
		t.Fatalf("limit schema = %+v", limit)
	}
	startDate := properties["startDate"].(map[string]any)
	if startDate["pattern"] != `^(\d{4}-\d{2}-\d{2}|\d{8})$` {
		t.Fatalf("startDate pattern = %+v", startDate["pattern"])
	}
	if !strings.Contains(startDate["description"].(string), "必须同时提供") {
		t.Fatalf("startDate pair constraint missing: %+v", startDate)
	}
	window := properties["window"].(map[string]any)
	if window["default"] != 20 || window["minimum"] != 1 || window["maximum"] != 250 {
		t.Fatalf("window schema = %+v", window)
	}
	offset := properties["offset"].(map[string]any)
	if offset["default"] != 0 || offset["minimum"] != 0 || offset["maximum"] != 500 {
		t.Fatalf("offset schema = %+v", offset)
	}
	excludeNew := properties["excludeNew"].(map[string]any)
	if !strings.Contains(excludeNew["description"].(string), "N/C") {
		t.Fatalf("excludeNew rules missing: %+v", excludeNew)
	}
}

func TestMCPMarketReviewDescribesDataDatesAndSessionMeaning(t *testing.T) {
	tool := findMCPTool(t, "tdx_market_review_text")
	if !strings.Contains(tool.Description, "当前交易日实时广度") ||
		!strings.Contains(tool.Description, "最近完整交易日盘后广度") {
		t.Fatalf("market review description = %q", tool.Description)
	}

	properties := tool.InputSchema["properties"].(map[string]any)
	session := properties["session"].(map[string]any)
	description := session["description"].(string)
	for _, want := range []string{"09:20-09:25集合竞价", "不会回溯", "数据日期"} {
		if !strings.Contains(description, want) {
			t.Fatalf("session description missing %q: %s", want, description)
		}
	}
}

func TestMCPToolSchemasDescribeAgentReadableConstraints(t *testing.T) {
	brief := findMCPTool(t, "tdx_stock_brief_text")
	briefProperties := brief.InputSchema["properties"].(map[string]any)
	mkt := briefProperties["mkt"].(map[string]any)
	if !hasString(mkt["enum"].([]string), "sh") ||
		!strings.Contains(mkt["description"].(string), "自动识别") {
		t.Fatalf("mkt schema = %+v", mkt)
	}
	if brief.OutputSchema["type"] != "object" {
		t.Fatalf("output schema = %+v", brief.OutputSchema)
	}
	if !strings.Contains(brief.Description, "不等于深度买卖建议") ||
		!strings.Contains(brief.Description, "数据一致性") {
		t.Fatalf("brief description missing calculation boundary: %s", brief.Description)
	}

	kline := findMCPTool(t, "tdx_kline_summary_text")
	klineProperties := kline.InputSchema["properties"].(map[string]any)
	level := klineProperties["level"].(map[string]any)
	if level["default"] != "normal" {
		t.Fatalf("level default = %v, want normal", level["default"])
	}
	dayCount := klineProperties["dayCount"].(map[string]any)
	if dayCount["type"] != "integer" || dayCount["maximum"] != 500 {
		t.Fatalf("dayCount schema = %+v", dayCount)
	}
	if dayCount["default"] != nil ||
		!strings.Contains(dayCount["description"].(string), "brief=60") ||
		!strings.Contains(dayCount["description"].(string), "normal=120") ||
		!strings.Contains(dayCount["description"].(string), "deep=250") ||
		!strings.Contains(dayCount["description"].(string), "近52周") ||
		strings.Contains(dayCount["description"].(string), "递推指标") {
		t.Fatalf("dayCount dynamic default description = %+v", dayCount)
	}

	sectorDetail := findMCPTool(t, "tdx_sector_detail_text")
	if len(sectorDetail.InputSchema["anyOf"].([]map[string]any)) != 2 {
		t.Fatalf("sector detail anyOf = %+v", sectorDetail.InputSchema["anyOf"])
	}
	for _, want := range []string{"最近完整交易日单日涨跌", "近20/60日", "不会把未收盘"} {
		if !strings.Contains(sectorDetail.Description, want) {
			t.Fatalf("sector detail description missing %q: %s", want, sectorDetail.Description)
		}
	}
	sectorDetailProperties := sectorDetail.InputSchema["properties"].(map[string]any)
	sectorName := sectorDetailProperties["sectorName"].(map[string]any)
	if !strings.Contains(sectorName["description"].(string), "精确一致") {
		t.Fatalf("sector detail sectorName must explain exact matching: %+v", sectorName)
	}
	topStocks := sectorDetailProperties["topStocks"].(map[string]any)
	if !strings.Contains(topStocks["description"].(string), "强势、中游、弱势各") {
		t.Fatalf("sector detail topStocks scope unclear: %+v", topStocks)
	}
	sectorExcludeNew := sectorDetailProperties["excludeNew"].(map[string]any)
	for _, want := range []string{"N/C", "单日涨幅超过100%", "排序值超过100%"} {
		if !strings.Contains(sectorExcludeNew["description"].(string), want) {
			t.Fatalf("sector detail excludeNew missing %q: %+v", want, sectorExcludeNew)
		}
	}

	sectorRealtime := findMCPTool(t, "tdx_sector_realtime_text")
	if len(sectorRealtime.InputSchema["anyOf"].([]map[string]any)) != 2 {
		t.Fatalf("sector realtime anyOf = %+v", sectorRealtime.InputSchema["anyOf"])
	}
	for _, want := range []string{
		"09:30-11:30",
		"13:00-15:00",
		"不回退为历史数据",
	} {
		if !strings.Contains(sectorRealtime.Description, want) {
			t.Fatalf("sector realtime description missing %q: %s", want, sectorRealtime.Description)
		}
	}
	realtimeProperties := sectorRealtime.InputSchema["properties"].(map[string]any)
	for _, name := range []string{"sectorName", "indexCode", "sectorType"} {
		property := realtimeProperties[name].(map[string]any)
		if property["description"] == "" {
			t.Fatalf("sector realtime %s description missing", name)
		}
	}

	tradeFlow := findMCPTool(t, "tdx_trade_flow_estimate_text")
	if !strings.Contains(tradeFlow.Description, "近60个交易日") ||
		strings.Contains(tradeFlow.Description, "200个交易日") {
		t.Fatalf("trade flow description has wrong lookback: %s", tradeFlow.Description)
	}
}

func TestMCPKlineSchemaDescribesRawDailyTradingData(t *testing.T) {
	kline := findMCPTool(t, "tdx_kline")
	if !strings.Contains(kline.Description, "逐日收盘价") ||
		!strings.Contains(kline.Description, "tdx_kline_summary_text") {
		t.Fatalf("kline description missing usage boundary: %s", kline.Description)
	}

	properties := kline.InputSchema["properties"].(map[string]any)
	code := properties["code"].(map[string]any)
	if code["pattern"] != `^\d{6}$` {
		t.Fatalf("code schema = %+v", code)
	}
	period := properties["type"].(map[string]any)
	if period["default"] != "day" ||
		!hasString(period["enum"].([]string), "week") ||
		hasString(period["enum"].([]string), "minute1") {
		t.Fatalf("type schema = %+v", period)
	}
	count := properties["count"].(map[string]any)
	if count["default"] != 10 || count["minimum"] != 1 || count["maximum"] != 800 {
		t.Fatalf("count schema = %+v", count)
	}

	outputProperties := kline.OutputSchema["properties"].(map[string]any)
	data := outputProperties["data"].(map[string]any)
	dataProperties := data["properties"].(map[string]any)
	list := dataProperties["list"].(map[string]any)
	item := list["items"].(map[string]any)
	itemProperties := item["properties"].(map[string]any)
	for _, name := range []string{"date", "open", "high", "low", "close", "volume", "amount"} {
		if itemProperties[name] == nil {
			t.Fatalf("kline output schema missing %s", name)
		}
	}
}

func TestMCPToolSchemasDescribeCalculationTools(t *testing.T) {
	scenario := findMCPTool(t, "tdx_scenario_valuation_text")
	scenarioProperties := scenario.InputSchema["properties"].(map[string]any)
	for _, name := range []string{"years", "eps", "bearGrowth", "basePE", "assumptionMode"} {
		if scenarioProperties[name] == nil {
			t.Fatalf("scenario schema missing %s", name)
		}
	}
	for _, name := range []string{"bearPE", "basePE", "bullPE"} {
		property := scenarioProperties[name].(map[string]any)
		if property["exclusiveMinimum"] != 0 ||
			!strings.Contains(property["description"].(string), "必须为正数") {
			t.Fatalf("%s schema = %+v", name, property)
		}
	}
	currentPrice := scenarioProperties["currentPrice"].(map[string]any)
	if currentPrice["exclusiveMinimum"] != 0 ||
		!strings.Contains(currentPrice["description"].(string), "必须为正数") {
		t.Fatalf("scenario currentPrice schema = %+v", currentPrice)
	}
	for _, name := range []string{"bearGrowth", "baseGrowth", "bullGrowth"} {
		property := scenarioProperties[name].(map[string]any)
		if property["minimum"] != -100 || property["maximum"] != 1000 {
			t.Fatalf("%s schema = %+v", name, property)
		}
	}
	years := scenarioProperties["years"].(map[string]any)
	if years["minimum"] != 1 || years["maximum"] != 10 {
		t.Fatalf("scenario years schema = %+v", years)
	}
	if !strings.Contains(scenario.Description, "不输出买卖建议") {
		t.Fatalf("scenario description missing boundary: %s", scenario.Description)
	}

	implied := findMCPTool(t, "tdx_implied_expectation_text")
	impliedProperties := implied.InputSchema["properties"].(map[string]any)
	for _, name := range []string{"years", "eps", "targetPE"} {
		if impliedProperties[name] == nil {
			t.Fatalf("implied schema missing %s", name)
		}
	}
	targetPE := impliedProperties["targetPE"].(map[string]any)
	if targetPE["exclusiveMinimum"] != 0 ||
		!strings.Contains(targetPE["description"].(string), "必须为正数") {
		t.Fatalf("targetPE schema = %+v", targetPE)
	}
	impliedPrice := impliedProperties["currentPrice"].(map[string]any)
	if impliedPrice["exclusiveMinimum"] != 0 {
		t.Fatalf("implied currentPrice schema = %+v", impliedPrice)
	}

	technical := findMCPTool(t, "tdx_technical_score_text")
	for _, want := range []string{"查询时间", "各周期数据日期", "交易时段"} {
		if !strings.Contains(technical.Description, want) {
			t.Fatalf("technical description missing %q: %s", want, technical.Description)
		}
	}
	technicalProperties := technical.InputSchema["properties"].(map[string]any)
	dayCount := technicalProperties["dayCount"].(map[string]any)
	if dayCount["minimum"] != 60 || dayCount["maximum"] != 500 {
		t.Fatalf("technical dayCount schema = %+v", dayCount)
	}
	for _, want := range []string{"250根", "Wilder", "逐根递推", "前后周期"} {
		if !strings.Contains(technical.Description, want) {
			t.Fatalf("technical description missing %q: %s", want, technical.Description)
		}
	}

	kline := findMCPTool(t, "tdx_kline_summary_text")
	for _, want := range []string{"250根", "查询时间", "数据日期", "N个交易日前"} {
		if !strings.Contains(kline.Description, want) {
			t.Fatalf("kline summary description missing %q: %s", want, kline.Description)
		}
	}
	multiBrief := findMCPTool(t, "tdx_multi_brief_text")
	for _, tool := range []mcpTool{
		multiBrief,
		technical,
	} {
		for _, want := range []string{
			"MA", "MACD", "RSI", "BOLL", "KDJ", "BIAS",
			"ATR", "OBV", "量价", "多空比",
		} {
			if !strings.Contains(tool.Description, want) {
				t.Fatalf("%s description missing %q: %s", tool.Name, want, tool.Description)
			}
		}
	}
	brief := findMCPTool(t, "tdx_stock_brief_text")
	for _, want := range []string{
		"不包含技术指标", "5/20/60日", "52周区间", "行情数据日期", "财务及估值",
	} {
		if !strings.Contains(brief.Description, want) {
			t.Fatalf("brief description missing %q: %s", want, brief.Description)
		}
	}
	briefProperties := brief.InputSchema["properties"].(map[string]any)
	if briefProperties["adjust"] != nil {
		t.Fatalf("stock brief should not expose unused adjust: %+v", briefProperties)
	}
	for _, unwanted := range []string{"MACD", "RSI", "KDJ", "ATR", "OBV"} {
		if strings.Contains(kline.Description, unwanted) {
			t.Fatalf("kline description should not contain %q: %s", unwanted, kline.Description)
		}
	}
	for _, tool := range []mcpTool{
		kline,
		multiBrief,
	} {
		properties := tool.InputSchema["properties"].(map[string]any)
		if properties["adjust"] == nil {
			t.Fatalf("%s schema missing adjust: %+v", tool.Name, properties)
		}
		for _, want := range []string{"250根", "数据日期", "交易时段"} {
			if !strings.Contains(tool.Description, want) {
				t.Fatalf("%s description missing %q: %s", tool.Name, want, tool.Description)
			}
		}
	}
	for _, want := range []string{"ATR和OBV", "观察项", "固定0分"} {
		if !strings.Contains(technical.Description, want) {
			t.Fatalf("technical description missing %q: %s", want, technical.Description)
		}
	}
}

func TestMCPDescriptionsAvoidDuplicateAnalysisCalls(t *testing.T) {
	multiBrief := findMCPTool(t, "tdx_multi_brief_text")
	for _, want := range []string{"批量替代路径", "不要再对同一批股票", "重点个股"} {
		if !strings.Contains(multiBrief.Description, want) {
			t.Fatalf("multi brief description missing %q: %s", want, multiBrief.Description)
		}
	}

	sectorDetail := findMCPTool(t, "tdx_sector_detail_text")
	for _, want := range []string{"先用tdx_hotspot_scan_text", "深入指定板块", "无需"} {
		if !strings.Contains(sectorDetail.Description, want) {
			t.Fatalf("sector detail description missing %q: %s", want, sectorDetail.Description)
		}
	}
}

func TestMCPCallUnknownToolReturnsError(t *testing.T) {
	body := bytes.NewBufferString(`{
		"jsonrpc":"2.0",
		"id":3,
		"method":"tools/call",
		"params":{"name":"missing_tool","arguments":{}}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/mcp", body)
	rec := httptest.NewRecorder()
	handleMCP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var resp mcpResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error == nil {
		t.Fatalf("expected error, got %s", rec.Body.String())
	}
}

func TestTextMCPResultContainsLongTextOnlyOnce(t *testing.T) {
	tool := newMCPTool(
		"test_text",
		"test",
		"/api/test-text",
		func(w http.ResponseWriter, _ *http.Request) {
			jsonResp(w, AgentMultiBriefText{
				Format:  "text/plain; charset=utf-8",
				Content: "唯一正文",
			})
		},
	)

	result, err := callAgentHandlerAsMCP(tool, nil)
	if err != nil {
		t.Fatal(err)
	}
	content := result["content"].([]map[string]string)
	if len(content) != 1 || content[0]["text"] != "唯一正文" {
		t.Fatalf("content = %+v", content)
	}
	structured := result["structuredContent"].(map[string]any)
	if len(structured) != 1 || structured["endpoint"] != "/api/test-text" {
		t.Fatalf("structuredContent duplicated text payload: %+v", structured)
	}
	properties := tool.OutputSchema["properties"].(map[string]any)
	if len(properties) != 1 || properties["endpoint"] == nil {
		t.Fatalf("text output schema = %+v", tool.OutputSchema)
	}
}

func findMCPTool(t *testing.T, name string) mcpTool {
	t.Helper()

	for _, tool := range mcpTools() {
		if tool.Name == name {
			return tool
		}
	}
	t.Fatalf("%s missing", name)
	return mcpTool{}
}

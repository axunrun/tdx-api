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

	sectorDetail := findMCPTool(t, "tdx_sector_detail_text")
	if len(sectorDetail.InputSchema["anyOf"].([]map[string]any)) != 2 {
		t.Fatalf("sector detail anyOf = %+v", sectorDetail.InputSchema["anyOf"])
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

	technical := findMCPTool(t, "tdx_technical_score_text")
	technicalProperties := technical.InputSchema["properties"].(map[string]any)
	dayCount := technicalProperties["dayCount"].(map[string]any)
	if dayCount["minimum"] != 60 || dayCount["maximum"] != 500 {
		t.Fatalf("technical dayCount schema = %+v", dayCount)
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

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestMarginTradingHandlerUsesLatestTradingDays(t *testing.T) {
	original := loadMarginTrading
	loadMarginTrading = func(code string, days int) (AgentMarginTrading, error) {
		if code != "300499" || days != 3 {
			t.Fatalf("query = %s/%d, want 300499/3", code, days)
		}
		return AgentMarginTrading{
			Code:              code,
			IsMarginEligible:  true,
			EligibilityStatus: "eligible",
			RequestedDays:     3,
			ActualDays:        3,
			LatestDataDate:    "2026-07-30",
			Records: []MarginTradingRecord{
				{Date: "2026-07-30"},
				{Date: "2026-07-29"},
				{Date: "2026-07-28"},
			},
		}, nil
	}
	defer func() { loadMarginTrading = original }()

	req := httptest.NewRequest(http.MethodGet,
		"/api/agent/margin-trading?code=300499&days=3", nil)
	rec := httptest.NewRecorder()
	handleAgentMarginTrading(rec, req)

	var response APIResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Code != 0 {
		t.Fatalf("response = %s", rec.Body.String())
	}
	data := response.Data.(map[string]any)
	if data["actualDays"] != float64(3) || data["latestDataDate"] != "2026-07-30" {
		t.Fatalf("data = %+v", data)
	}
}

func TestMarginTradingNoRecordsMeansNotEligible(t *testing.T) {
	result := finalizeMarginTradingResult(AgentMarginTrading{
		Code:          "600001",
		RequestedDays: 30,
		Records:       []MarginTradingRecord{},
	})
	if result.IsMarginEligible || result.EligibilityStatus != "not_eligible" {
		t.Fatalf("result = %+v", result)
	}
	text := buildMarginTradingText(result)
	if !strings.Contains(text, "当前不是融资融券标的证券") {
		t.Fatalf("text = %s", text)
	}
	if !strings.Contains(text, "最新已披露交易日：无可用数据") {
		t.Fatalf("empty data date is mislabeled: %s", text)
	}
}

func TestMarginTradingTextExplainsDaysAndDisclosureDate(t *testing.T) {
	result := AgentMarginTrading{
		Code:           "603063",
		Name:           "禾望电气",
		Exchange:       "SSE",
		Source:         "上海证券交易所",
		QueryTime:      "2026-08-01T10:00:00+08:00",
		RequestedDays:  2,
		ActualDays:     2,
		LatestDataDate: "2026-07-30",
		Records: []MarginTradingRecord{
			{Date: "2026-07-30", FinancingBalance: 100, FinancingBuy: 10},
			{Date: "2026-07-29", FinancingBalance: 90, FinancingBuy: 8},
		},
	}
	text := buildMarginTradingText(result)
	for _, want := range []string{
		"最新已披露交易日：2026-07-30",
		"请求2个交易日，实际返回2个交易日",
		"days按交易所实际披露记录计数，不按自然日计数",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("text missing %q:\n%s", want, text)
		}
	}
}

func TestMarginTradingOptionalAmountDistinguishesMissingFromZero(t *testing.T) {
	if got := formatOptionalMarginAmount(nil, 10000); got != "—" {
		t.Fatalf("missing amount = %q", got)
	}
	if got := formatOptionalMarginAmount(marginFloat(0), 10000); got != "0.00" {
		t.Fatalf("zero amount = %q", got)
	}
}

func TestMarginTradingMCPOnlyExposesCodeAndDays(t *testing.T) {
	tool := findMCPTool(t, "tdx_margin_trading_text")
	properties := tool.InputSchema["properties"].(map[string]any)
	if len(properties) != 2 {
		t.Fatalf("properties = %+v", properties)
	}
	if _, ok := properties["code"]; !ok {
		t.Fatal("code parameter missing")
	}
	days := properties["days"].(map[string]any)
	if days["default"] != 30 || days["minimum"] != 1 || days["maximum"] != 120 {
		t.Fatalf("days schema = %+v", days)
	}
	for _, forbidden := range []string{"start", "end", "startDate", "endDate"} {
		if _, ok := properties[forbidden]; ok {
			t.Fatalf("unexpected parameter %s", forbidden)
		}
	}
}

func TestMarginTradingOfficialSourcesLive(t *testing.T) {
	if os.Getenv("TDX_LIVE_TEST") == "" {
		t.Skip("set TDX_LIVE_TEST=1 to call exchange disclosure endpoints")
	}
	for _, test := range []struct {
		exchange string
		code     string
	}{
		{"SSE", "603063"},
		{"SZSE", "300499"},
		{"BSE", "920001"},
	} {
		record, found, err := fetchMarginTradingDay(test.exchange, test.code, "2026-07-30")
		if err != nil {
			t.Fatalf("%s: %v", test.exchange, err)
		}
		if !found || record.Code != test.code || record.Date != "2026-07-30" {
			t.Fatalf("%s: found=%v record=%+v", test.exchange, found, record)
		}
	}

	t.Setenv("AGENT_DB_PATH", t.TempDir()+"/agent.sqlite")
	result, err := loadMarginTradingFromSources("603063", 30)
	if err != nil {
		t.Fatal(err)
	}
	if result.ActualDays != 30 || len(result.Records) != 30 {
		t.Fatalf("result = %+v", result)
	}
	t.Log("\n" + buildMarginTradingText(result))
	for index := 1; index < len(result.Records); index++ {
		if result.Records[index-1].Date <= result.Records[index].Date {
			t.Fatalf("records are not newest first: %+v", result.Records)
		}
	}
}

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/injoyai/tdx"
	"github.com/injoyai/tdx/protocol"
)

func TestBuildScenarioValuationTextUsesDefaultsAndAvoidsAdvice(t *testing.T) {
	brief := AgentStockBrief{
		Code: "603063",
		Name: "禾望电气",
		Quote: &AgentBriefQuote{
			Price: 50,
		},
		Stat: &AgentBriefStat{
			PETTM: 25,
		},
		LatestReport: &AgentBriefLatestReport{
			NetProfitYoY: -10,
		},
	}

	text := buildScenarioValuationText(brief, url.Values{"years": []string{"3"}})

	for _, want := range []string{"情景估值：", "悲观 | -10.00%", "中性 | 0.00%", "乐观 | 8.00%", "不代表买卖建议"} {
		if !strings.Contains(text, want) {
			t.Fatalf("text missing %q:\n%s", want, text)
		}
	}
	for _, banned := range []string{"建议买入", "建议卖出"} {
		if strings.Contains(text, banned) {
			t.Fatalf("text should not contain %q:\n%s", banned, text)
		}
	}
}

func TestBuildScenarioValuationTextDescribesManualInputs(t *testing.T) {
	brief := AgentStockBrief{Code: "603063"}
	query := url.Values{
		"years":        []string{"2"},
		"currentPrice": []string{"100"},
		"eps":          []string{"10"},
		"bearGrowth":   []string{"10"},
		"baseGrowth":   []string{"10"},
		"bullGrowth":   []string{"10"},
		"bearPE":       []string{"15"},
		"basePE":       []string{"15"},
		"bullPE":       []string{"15"},
	}

	text := buildScenarioValuationText(brief, query)

	for _, want := range []string{
		"EPS 10.0000（用户输入）",
		"悲观 | 10.00% | 15.00 | 12.1000 | 181.50元 | +81.50% | +34.72%",
		"关键假设：使用用户输入的增长率和目标PE。",
		"价格/EPS/增长率/目标PE含用户输入项",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("text missing %q:\n%s", want, text)
		}
	}
	for _, banned := range []string{"EPS 默认由价格和 PE_TTM 反推", "未传入 growth/PE"} {
		if strings.Contains(text, banned) {
			t.Fatalf("text should not contain %q:\n%s", banned, text)
		}
	}
}

func TestBuildScenarioValuationTextRejectsNonPositivePE(t *testing.T) {
	text := buildScenarioValuationText(AgentStockBrief{Code: "603063"}, url.Values{
		"currentPrice": []string{"100"},
		"eps":          []string{"10"},
		"bearPE":       []string{"0"},
		"basePE":       []string{"0"},
		"bullPE":       []string{"0"},
	})

	for _, want := range []string{"三情景估值：无法计算", "目标PE必须为正数"} {
		if !strings.Contains(text, want) {
			t.Fatalf("text missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "目标价 0") || strings.Contains(text, "-100.00%") {
		t.Fatalf("text should not continue calculation:\n%s", text)
	}
}

func TestBuildScenarioValuationTextRejectsOutOfRangeYears(t *testing.T) {
	text := buildScenarioValuationText(AgentStockBrief{Code: "603063"}, url.Values{"years": []string{"0"}})

	for _, want := range []string{"三情景估值：无法计算", "years 必须在 1-10 之间"} {
		if !strings.Contains(text, want) {
			t.Fatalf("text missing %q:\n%s", want, text)
		}
	}
}

func TestBuildImpliedExpectationTextCalculatesPressure(t *testing.T) {
	brief := AgentStockBrief{
		Code: "603063",
		Quote: &AgentBriefQuote{
			Price: 50,
		},
		Stat: &AgentBriefStat{
			PETTM: 25,
		},
	}
	query := url.Values{
		"years":    []string{"3"},
		"targetPE": []string{"20"},
	}

	text := buildImpliedExpectationText(brief, query)

	for _, want := range []string{"当前价格隐含未来 EPS", "隐含 EPS 年复合增速", "估值预期压力"} {
		if !strings.Contains(text, want) {
			t.Fatalf("text missing %q:\n%s", want, text)
		}
	}
}

func TestBuildImpliedExpectationTextCalculatesManualInputs(t *testing.T) {
	text := buildImpliedExpectationText(AgentStockBrief{Code: "603063"}, url.Values{
		"years":        []string{"2"},
		"currentPrice": []string{"100"},
		"eps":          []string{"10"},
		"targetPE":     []string{"20"},
	})

	for _, want := range []string{
		"当前价格隐含未来 EPS：5.0000",
		"隐含 EPS 年复合增速：-29.29%",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("text missing %q:\n%s", want, text)
		}
	}
}

func TestBuildImpliedExpectationTextRejectsOutOfRangeYears(t *testing.T) {
	text := buildImpliedExpectationText(AgentStockBrief{Code: "603063"}, url.Values{"years": []string{"0"}})

	for _, want := range []string{"当前价格隐含预期：无法计算", "years 必须在 1-10 之间"} {
		if !strings.Contains(text, want) {
			t.Fatalf("text missing %q:\n%s", want, text)
		}
	}
}

func TestTechnicalScoreLevel(t *testing.T) {
	cases := map[int]string{
		8:  "强多",
		3:  "偏多",
		0:  "中性",
		-3: "偏空",
		-8: "强空",
	}
	for score, want := range cases {
		if got := technicalScoreLevel(score); got != want {
			t.Fatalf("technicalScoreLevel(%d) = %s, want %s", score, got, want)
		}
	}
}

func TestNormalizeTechnicalAdjust(t *testing.T) {
	cases := map[string]string{
		"":     "qfq",
		" qfq": "qfq",
		"none": "none",
	}
	for input, want := range cases {
		got, err := normalizeTechnicalAdjust(input)
		if err != nil {
			t.Fatalf("normalizeTechnicalAdjust(%q) error = %v", input, err)
		}
		if got != want {
			t.Fatalf("normalizeTechnicalAdjust(%q) = %s, want %s", input, got, want)
		}
	}
	if _, err := normalizeTechnicalAdjust("bad"); err == nil {
		t.Fatal("normalizeTechnicalAdjust(bad) expected error")
	}
}

func TestHandleAgentTechnicalScoreTextRejectsBadAdjust(t *testing.T) {
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/agent/technical-score-text?code=603063&adjust=bad",
		nil,
	)
	rec := httptest.NewRecorder()

	handleAgentTechnicalScoreText(rec, req)

	var resp APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Code != -1 || !strings.Contains(resp.Message, "adjust 必须为 qfq 或 none") {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestBuildTechnicalScoreSummaryQFQRequiresGbbq(t *testing.T) {
	oldGbbq := gbbq
	gbbq = nil
	t.Cleanup(func() {
		gbbq = oldGbbq
	})

	_, err := buildTechnicalScoreSummary(&tdx.Client{}, "603063", 60, false, "qfq")
	if err == nil || !strings.Contains(err.Error(), "前复权模块未就绪") {
		t.Fatalf("expected qfq readiness error, got %v", err)
	}
}

func TestTechnicalScoreKlineHeaderDescribesAdjust(t *testing.T) {
	qfq := technicalScoreKlineHeader("qfq", 250, true)
	for _, want := range []string{
		"日线价格指标使用 Gbbq 前复权日K线",
		"成交量沿用K线原始volume",
		"周/月线使用现有TDX周/月K线口径",
	} {
		if !strings.Contains(qfq, want) {
			t.Fatalf("qfq header missing %q: %s", want, qfq)
		}
	}

	none := technicalScoreKlineHeader("none", 250, true)
	for _, want := range []string{
		"日线价格指标使用未复权TDX日K线",
		"成交量使用未复权成交量",
		"周/月线使用现有TDX周/月K线口径",
	} {
		if !strings.Contains(none, want) {
			t.Fatalf("none header missing %q: %s", want, none)
		}
	}
}

func TestScoreTechnicalPeriodCalculatesFormerMissingRows(t *testing.T) {
	klines := testTechnicalScoreKlines()
	rows := scoreTechnicalPeriod(AgentTechnicalPeriod{Name: "日线", Period: "day"}, klines)

	seen := map[string]technicalScoreRow{}
	for _, row := range rows {
		seen[row.Item] = row
	}
	for _, item := range []string{"KDJ", "BIAS", "量价"} {
		row, ok := seen[item]
		if !ok {
			t.Fatalf("%s row missing: %+v", item, rows)
		}
		if row.Value == "-" || strings.Contains(row.Signal, "当前接口未提供") {
			t.Fatalf("%s row not calculated: %+v", item, row)
		}
	}
}

func TestScoreKDJUsesRecursiveValuesAndRealCrossingState(t *testing.T) {
	row := scoreKDJ("日线", testTechnicalScoreKlines())

	if row.Value != "K=72.22 D=60.37 J=95.93" {
		t.Fatalf("KDJ value = %q", row.Value)
	}
	if row.Signal != "K在D上方" || row.Score != 1 {
		t.Fatalf("KDJ signal = %+v", row)
	}
}

func TestKDJSignalRequiresPreviousDayCrossing(t *testing.T) {
	tests := []struct {
		name                 string
		previousK, previousD float64
		currentK, currentD   float64
		want                 string
	}{
		{"golden cross", 40, 45, 50, 46, "K上穿D"},
		{"death cross", 55, 50, 45, 49, "K下穿D"},
		{"above without cross", 55, 50, 60, 52, "K在D上方"},
		{"below without cross", 45, 50, 40, 48, "K在D下方"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := kdjSignal(tt.previousK, tt.previousD, tt.currentK, tt.currentD)
			if got != tt.want {
				t.Fatalf("kdjSignal() = %q, want %q", got, tt.want)
			}
		})
	}
}

func testTechnicalScoreKlines() protocol.Klines {
	out := make(protocol.Klines, 0, 10)
	last := protocol.Yuan(10)
	for i := 0; i < 10; i++ {
		close := protocol.Yuan(float64(10 + i))
		out = append(out, &protocol.Kline{
			Last:   last,
			Open:   protocol.Yuan(float64(10 + i)),
			High:   protocol.Yuan(float64(11 + i)),
			Low:    protocol.Yuan(float64(9 + i)),
			Close:  close,
			Volume: int64(100 + i*10),
		})
		last = close
	}
	out[len(out)-1].Volume = 500
	return out
}

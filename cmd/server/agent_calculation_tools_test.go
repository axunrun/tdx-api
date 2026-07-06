package main

import (
	"net/url"
	"strings"
	"testing"

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

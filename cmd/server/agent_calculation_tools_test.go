package main

import (
	"net/url"
	"strings"
	"testing"
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

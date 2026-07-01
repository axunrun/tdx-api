package main

import (
	"strings"
	"testing"
)

func TestBuildAgentStockBriefTextUsesChineseSummaryAndFiltersDebugFields(t *testing.T) {
	ma20 := 53.01
	rsi6 := 54.0
	macdHist := -1.4
	bollUpper := 60.38
	bollMiddle := 53.01
	bollLower := 45.63
	atr14 := 3.09

	brief := AgentStockBrief{
		Code: "603063",
		Name: "禾望电气",
		Quote: &AgentBriefQuote{
			Code:         "603063",
			Market:       "沪市",
			Price:        50.15,
			LastClose:    48.71,
			Open:         48.81,
			High:         50.95,
			Low:          47.57,
			ChangePct:    2.956,
			AmplitudePct: 6.94,
			TurnoverRate: 4.79,
			Volume:       222077,
			AmountText:   "11.00亿元",
		},
		Finance: &AgentBriefFinance{
			IPODate:               "2017-02-13",
			TotalSharesText:       "4.64亿股",
			FloatSharesText:       "4.64亿股",
			TotalMarketValueText:  "232.70亿元",
			FloatMarketValueText:  "232.70亿元",
			TotalAssetsText:       "91.76亿元",
			NetAssetsText:         "27.64亿元",
			MainRevenueText:       "61.39亿元",
			MainProfitText:        "20.15亿元",
			OperatingProfitText:   "6.12亿元",
			NetProfitText:         "5.11亿元",
			OperatingCashflowText: "8.30亿元",
			Shareholders:          66784,
		},
		LatestReport: &AgentBriefLatestReport{
			ReportDate:                "2026-03-31",
			Basis:                     "按06-23股本",
			NetAssetPerShare:          11.3774,
			OperatingCashflowPerShare: -0.4555,
			WeightedROE:               0.99,
			RevenueText:               "5.74亿元",
			RevenueYoY:                -25.82,
			NetProfitText:             "5112.13万元",
			NetProfitYoY:              -51.48,
		},
		Blocks: []AgentBriefBlock{
			{Type: "concept", TypeName: "概念板块", Name: "光伏"},
			{Type: "concept", TypeName: "概念板块", Name: "智能电网"},
			{Type: "style_region", TypeName: "地域/风格板块", Name: "浙江"},
			{Type: "index", TypeName: "指数板块", Name: "中证1000"},
		},
		Stat: &AgentBriefStat{
			PETTM:     25.31,
			PEStatic:  28.42,
			PB:        4.46,
			DivYield:  1.2,
			ChangePct: 2.95,
			Chg5:      3.15,
			Chg20:     -4.82,
			Chg60:     12.3,
			ChgYTD:    18.2,
		},
		Moneyflow: &AgentBriefMoneyflow{
			Amount:           110020.88,
			AmountPrev:       90000,
			AmountChangePct:  22.25,
			AmountChangeText: "2.00亿元",
			High52W:          66.8,
			Low52W:           28.5,
		},
		Technical: &AgentTechnicalSummary{
			Periods: []AgentTechnicalPeriod{
				{
					Period: "day",
					Name:   "日线",
					Close:  50.15,
					MA: map[string]Metric{
						"ma20": {Available: true, Value: &ma20},
					},
					RSI: map[string]Metric{
						"rsi6": {Available: true, Value: &rsi6},
					},
					MACD: AgentMACD{
						Available: true,
						Hist:      &macdHist,
						Signal:    "MACD柱为负，空头动能占优",
					},
					BOLL: AgentBOLL{
						Available: true,
						Upper:     &bollUpper,
						Middle:    &bollMiddle,
						Lower:     &bollLower,
						Position:  "价格位于布林线中轨下方",
					},
					ATR: AgentATR{
						Available: true,
						ATR14:     &atr14,
					},
				},
			},
		},
	}

	content := buildAgentStockBriefText(brief)

	mustContain := []string{
		"股票：禾望电气（603063）",
		"行情摘要：",
		"振幅 +6.94%",
		"换手率 +4.79%",
		"成交额较昨日增加2.00亿元（+22.25%）",
		"总市值 232.70亿元，流通市值 232.70亿元",
		"总资产 91.76亿元",
		"主营利润 20.15亿元，营业利润 6.12亿元",
		"股东人数 66784",
		"最新财报提示：",
		"报告期 2026-03-31；按06-23股本；每股净资产 11.3774 元",
		"每股经营现金流 -0.4555 元",
		"营业收入 5.74亿元，同比 -25.82%",
		"净利润 5112.13万元，同比 -51.48%",
		"所属板块：",
		"概念板块：光伏、智能电网。",
		"估值与表现：",
		"市净率PB 4.46",
		"阶段涨跌幅：近5日 +3.15%，近20日 -4.82%，近60日 +12.30%，今年以来 +18.20%。",
		"技术指标：",
		"日线：收盘50.15",
		"MA20=53.01",
		"MACD柱为负",
		"RSI6=54.00",
		"布林线：价格位于布林线中轨下方",
		"ATR14=3.09",
	}
	for _, want := range mustContain {
		if !strings.Contains(content, want) {
			t.Fatalf("expected content to contain %q\ncontent:\n%s", want, content)
		}
	}

	mustNotContain := []string{"industry", "time", "insideDish", "outerDisc", "source", "limits"}
	for _, noise := range mustNotContain {
		if strings.Contains(content, noise) {
			t.Fatalf("expected content not to contain %q\ncontent:\n%s", noise, content)
		}
	}
}

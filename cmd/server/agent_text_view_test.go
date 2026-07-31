package main

import (
	"strings"
	"testing"
	"time"

	"github.com/injoyai/tdx/protocol"
)

func TestQuoteKlineDataStatusFollowsUnifiedFreshnessPolicy(t *testing.T) {
	location := time.FixedZone("Asia/Shanghai", 8*60*60)
	today := time.Date(2026, 7, 31, 0, 0, 0, 0, location)
	tests := []struct {
		name   string
		latest time.Time
		now    time.Time
		want   string
	}{
		{"trading", today, time.Date(2026, 7, 31, 10, 30, 0, 0, location), "盘中实时行情"},
		{"break", today, time.Date(2026, 7, 31, 12, 0, 0, 0, location), "午间休市最后行情"},
		{"closed", today, time.Date(2026, 7, 31, 15, 1, 0, 0, location), "当日收盘行情"},
		{
			"previous completed",
			today,
			time.Date(2026, 8, 3, 8, 30, 0, 0, location),
			"最近完整交易日收盘行情",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := quoteKlineDataStatus(&protocol.Kline{Time: tt.latest}, tt.now)
			if !strings.Contains(got, tt.want) {
				t.Fatalf("status = %q, want containing %q", got, tt.want)
			}
		})
	}
}

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
			QueryTime:    "2026-07-31T10:30:00+08:00",
			DataDate:     "2026-07-31",
			DataStatus:   "盘中实时行情，数据随交易更新",
		},
		Finance: &AgentBriefFinance{
			TotalShares:           464000000,
			IPODate:               "2017-02-13",
			TotalSharesText:       "4.64亿股",
			FloatSharesText:       "4.64亿股",
			TotalMarketValue:      23269600000,
			TotalMarketValueText:  "232.70亿元",
			FloatMarketValueText:  "232.70亿元",
			NetProfit:             51121300,
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
			Date:      "20260630",
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
			dayData: agentDayDataContext{
				QueryTime: time.Date(2026, 7, 31, 10, 30, 0, 0, time.Local),
				DataDate:  "2026-07-31",
				Status:    "盘中动态值，当前日K尚未收盘",
				Adjust:    "qfq",
			},
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
			bullBear: technicalScoreRow{
				Period: "日线",
				Item:   "多空比",
				Value:  "买/卖 1.25",
				Signal: "主动买入估算占优",
				Score:  1,
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
		"查询时间：2026-07-31T10:30:00+08:00",
		"行情数据日期：2026-07-31",
		"行情数据状态：盘中实时行情，数据随交易更新",
		"估值摘要：",
		"估值统计日期：2026-06-30",
		"市净率PB 4.46",
		"估值与数据一致性：",
		"市值一致性：当前价 × 总股本 = 232.70亿元，与接口总市值 232.70亿元偏差 0.00%，口径一致。",
		"估值与质量提示：",
		"净利润同比显著弱于营收同比",
	}
	for _, unwanted := range []string{
		"技术指标：", "阶段涨跌幅", "52周价格区间",
	} {
		if strings.Contains(content, unwanted) {
			t.Fatalf("content should not contain %q:\n%s", unwanted, content)
		}
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

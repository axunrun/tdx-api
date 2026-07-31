package main

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseAgentCodeListDeduplicatesCodes(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/agent/multi-brief?codes=603063,000001&code=603063", nil)

	got := parseAgentCodeList(req)

	if strings.Join(got, ",") != "603063,000001" {
		t.Fatalf("codes = %+v", got)
	}
}

func TestBuildAgentMultiBriefTextIsCompact(t *testing.T) {
	summary := AgentMultiBrief{
		Count: 1,
		Items: []AgentMultiBriefItem{
			{
				Code: "603063",
				Name: "禾望电气",
				Brief: AgentStockBrief{
					Code: "603063",
					Name: "禾望电气",
					Technical: &AgentTechnicalSummary{
						dayData: agentDayDataContext{
							QueryTime: time.Date(2026, 7, 31, 10, 30, 0, 0, time.Local),
							DataDate:  "2026-07-31",
							Status:    "盘中动态值，当前日K尚未收盘",
							Adjust:    "qfq",
						},
						Periods: []AgentTechnicalPeriod{{
							Period:   "day",
							Name:     "日线",
							Close:    10.5,
							Return20: 12.3,
							MA: map[string]Metric{
								"ma20": {Available: true, Value: floatPtr(10.1)},
								"ma60": {Available: true, Value: floatPtr(9.8)},
							},
							RSI: map[string]Metric{
								"rsi6": {Available: true, Value: floatPtr(55.2)},
							},
							MACD: AgentMACD{
								Available: true,
								Hist:      floatPtr(0.3),
								Signal:    "MACD柱为正，多头动能占优",
							},
							BOLL: AgentBOLL{
								Available: true,
								Position:  "价格位于布林线中轨上方",
							},
							ATR: AgentATR{
								Available: true,
								ATR14:     floatPtr(0.8),
							},
							OBV: AgentOBV{
								Available: true,
								Signal:    "OBV量能方向偏强",
							},
						}},
						bullBear: technicalScoreRow{
							Period: "日线",
							Item:   "多空比",
							Value:  "买/卖 1.25",
							Signal: "主动买入估算占优",
							Score:  1,
						},
					},
					Quote: &AgentBriefQuote{
						Price:        10.5,
						ChangePct:    2.3,
						AmountText:   "1.20亿元",
						TurnoverRate: 3.4,
						DataDate:     "2026-07-31",
						DataStatus:   "盘中实时行情，数据随交易更新",
					},
					Stat: &AgentBriefStat{Chg20: 12.3},
					Blocks: []AgentBriefBlock{
						{Name: "风电"},
						{Name: "储能"},
					},
				},
			},
		},
	}

	text := buildAgentMultiBriefText(summary)

	for _, want := range []string{
		"多股简讯：共1只",
		"查询时间：2026-07-31T10:30:00",
		"禾望电气（603063）",
		"涨跌幅+2.30%",
		"行情日期2026-07-31，盘中实时行情，数据随交易更新",
		"20日+12.30%",
		"板块：风电、储能",
		"日线技术：数据日期2026-07-31",
		"盘中动态值，当前日K尚未收盘",
		"前复权（qfq）",
		"MA：收盘 10.50",
		"MA20=10.10",
		"MA60=9.80",
		"MACD：MACD柱=0.30",
		"RSI6=55.20",
		"BOLL：价格位于布林线中轨上方",
		"ATR14=0.80",
		"OBV量能方向偏强",
		"KDJ：",
		"BIAS：",
		"量价：",
		"多空比：买/卖 1.25",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("text missing %q: %s", want, text)
		}
	}
}

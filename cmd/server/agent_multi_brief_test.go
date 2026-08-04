package main

import (
	"fmt"
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

func TestMultiBriefRejectsMoreThanTwentyCodes(t *testing.T) {
	codes := make([]string, 21)
	for i := range codes {
		codes[i] = fmt.Sprintf("%06d", i+1)
	}
	req := httptest.NewRequest(
		"GET",
		"/api/agent/multi-brief-text?codes="+strings.Join(codes, ","),
		nil,
	)
	rec := httptest.NewRecorder()

	handleAgentMultiBriefText(rec, req)

	if !strings.Contains(rec.Body.String(), "最多支持20只") {
		t.Fatalf("body = %s", rec.Body.String())
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
								Signal:    "OBV：量能方向偏强；未见背离。（OBV20净变化重复值）",
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
		"口径：前复权（qfq）",
		"【603063 禾望电气】",
		"行情：现价10.50；涨跌幅+2.30%",
		"周期：近20日+12.30%",
		"板块：风电、储能",
		"数据：行情日期2026-07-31",
		"技术日期2026-07-31",
		"盘中动态值，当前日K尚未收盘",
		"MA：收盘 10.50",
		"MA20=10.10",
		"MA60=9.80",
		"MACD：MACD柱=0.30",
		"RSI6=55.20",
		"BOLL：价格位于布林线中轨上方",
		"ATR14=0.80",
		"量能方向偏强；未见背离",
		"KDJ：",
		"BIAS：",
		"量价：",
		"多空比：买/卖 1.25",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("text missing %q: %s", want, text)
		}
	}
	for _, unwanted := range []string{
		"1. 禾望电气", "日线技术：", "；MACD：", "OBV20净变化重复值",
	} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("text should use grouped cards, found %q: %s", unwanted, text)
		}
	}
}

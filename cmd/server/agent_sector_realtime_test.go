package main

import (
	"strings"
	"testing"
	"time"

	"github.com/injoyai/tdx/protocol"
)

func TestBuildAgentSectorRealtimeRejectsNonTradingTime(t *testing.T) {
	now := time.Date(2026, 7, 29, 15, 1, 0, 0, time.Local)
	summary := buildAgentSectorRealtimeAt(
		AgentBriefBlock{Name: "液冷服务", IndexCode: "880685"},
		nil,
		now,
	)

	if summary.Available {
		t.Fatalf("non-trading response must be unavailable: %+v", summary)
	}
	text := buildAgentSectorRealtimeText(summary)
	if !strings.Contains(text, "当前非交易时间，无实时题材板块涨跌幅数据") {
		t.Fatalf("unexpected text: %s", text)
	}
}

func TestBuildAgentSectorRealtimeStopsImmediatelyAfterClose(t *testing.T) {
	now := time.Date(2026, 7, 29, 15, 0, 1, 0, time.Local)
	summary := buildAgentSectorRealtimeAt(
		AgentBriefBlock{Name: "液冷服务", IndexCode: "880685"},
		protocol.Klines{{
			Time:  time.Date(2026, 7, 29, 15, 0, 0, 0, time.Local),
			Last:  100000,
			Close: 103000,
		}},
		now,
	)

	if summary.Available || summary.Session != "closed" {
		t.Fatalf("15:00:01 must be outside realtime session: %+v", summary)
	}
}

func TestBuildAgentSectorRealtimeUsesCurrentTDXIndexKline(t *testing.T) {
	now := time.Date(2026, 7, 29, 10, 30, 15, 0, time.Local)
	klines := protocol.Klines{{
		Time:  time.Date(2026, 7, 29, 10, 30, 0, 0, time.Local),
		Last:  100000,
		Close: 103000,
	}}
	summary := buildAgentSectorRealtimeAt(
		AgentBriefBlock{Name: "液冷服务", IndexCode: "880685"},
		klines,
		now,
	)

	if !summary.Available || summary.ChangePct != 3 ||
		summary.DataDate != "2026-07-29" {
		t.Fatalf("unexpected realtime summary: %+v", summary)
	}
	text := buildAgentSectorRealtimeText(summary)
	for _, want := range []string{
		"查询时间：2026-07-29T10:30:15",
		"交易日期：2026-07-29",
		"实时涨跌幅：+3.00%",
		"TDX板块指数当日日K实时字段",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("text missing %q: %s", want, text)
		}
	}
}

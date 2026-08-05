package main

import (
	"strings"
	"testing"
	"time"

	"github.com/injoyai/tdx/protocol"
)

func TestLatestCompletedMarketIndexKlineSkipsPreopenPlaceholder(t *testing.T) {
	want := &protocol.Kline{Close: 100, Volume: 1}
	got := latestCompletedMarketIndexKline([]*protocol.Kline{
		want,
		{Close: 100},
	})
	if got != want {
		t.Fatalf("latestCompletedMarketIndexKline() = %v, want completed row", got)
	}
}

func TestResolveMarketReviewTypeAuto(t *testing.T) {
	tests := []struct {
		at   time.Time
		want string
	}{
		{at: time.Date(2026, 1, 3, 10, 0, 0, 0, time.Local), want: "non_trading"},
		{at: time.Date(2026, 1, 1, 9, 0, 0, 0, time.Local), want: "preopen"},
		{at: time.Date(2026, 1, 1, 9, 22, 0, 0, time.Local), want: "call_auction"},
		{at: time.Date(2026, 1, 1, 9, 27, 0, 0, time.Local), want: "preopen_after_auction"},
		{at: time.Date(2026, 1, 1, 10, 0, 0, 0, time.Local), want: "current"},
		{at: time.Date(2026, 1, 1, 11, 45, 0, 0, time.Local), want: "morning"},
		{at: time.Date(2026, 1, 1, 14, 0, 0, 0, time.Local), want: "current_with_morning_reference"},
		{at: time.Date(2026, 1, 1, 15, 1, 0, 0, time.Local), want: "full"},
	}

	for _, tt := range tests {
		if got := resolveMarketReviewType("auto", tt.at); got != tt.want {
			t.Fatalf("resolveMarketReviewType(%s) = %s, want %s", tt.at, got, tt.want)
		}
	}

	weekend := time.Date(2026, 1, 3, 15, 30, 0, 0, time.Local)
	if got := resolveMarketReviewType("full", weekend); got != "non_trading" {
		t.Fatalf("explicit session on weekend = %s, want non_trading", got)
	}
}

func TestBuildLatestCompletedMarketBreadthFiltersNonStocksAndOlderDates(t *testing.T) {
	stats := []*protocol.TdxStat{
		{Market: 0, Code: "000001", Date: "20260727", ChangePct: 1},
		{Market: 1, Code: "600000", Date: "20260727", ChangePct: -2},
		{Market: 0, Code: "159001", Date: "20260727", ChangePct: 3},
		{Market: 1, Code: "110001", Date: "20260727", ChangePct: 4},
		{Market: 0, Code: "000002", Date: "20260726", ChangePct: 5},
	}

	got := buildLatestCompletedMarketBreadth(stats)

	if !got.Available || got.Date != "2026-07-27" {
		t.Fatalf("unexpected metadata: %+v", got)
	}
	if got.UniverseTotal != 2 || got.ValidCount != 2 || got.Rising != 1 || got.Falling != 1 {
		t.Fatalf("unexpected breadth: %+v", got)
	}
	if got.DataType != "latest_completed_close" {
		t.Fatalf("dataType = %q, want latest_completed_close", got.DataType)
	}
}

func TestBuildAgentMarketReviewText(t *testing.T) {
	summary := AgentMarketReview{
		GeneratedAt: "2026-07-28T14:00:00+08:00",
		ReviewType:  "full",
		Indexes: []AgentMarketIndex{
			{Name: "上证指数", Date: "2026-07-28", ChangePct: 1.2},
		},
		CurrentBreadth: AgentMarketBreadth{
			Available: true, DataType: "current_snapshot", Date: "2026-07-28",
			AsOf: "2026-07-28T14:00:00+08:00", UniverseTotal: 3, ValidCount: 3,
			Rising: 2, Falling: 1, RisingPct: 66.67, AverageChange: 1.1, MedianChange: 0.8,
		},
		LatestCompletedBreadth: AgentMarketBreadth{
			Available: true, DataType: "latest_completed_close", Date: "2026-07-27",
			UniverseTotal: 3, ValidCount: 3, Rising: 1, Falling: 2,
			RisingPct: 33.33, AverageChange: -0.5, MedianChange: -0.4,
		},
		Hotspots: &AgentMarketHotspots{
			Strong: []AgentHotspotSector{{Name: "光伏", AverageValue: 3}, {Name: "储能", AverageValue: 2}},
			Middle: []AgentHotspotSector{{Name: "银行", AverageValue: 1}, {Name: "保险", AverageValue: 0}},
			Weak:   []AgentHotspotSector{{Name: "白酒", AverageValue: -2}, {Name: "医药", AverageValue: -3}},
		},
		Limits: map[string]int{"hotspotTop": 1},
	}

	text := buildAgentMarketReviewText(summary)

	for _, want := range []string{
		"市场复盘：全天收盘复盘",
		"今日指数（2026-07-28）",
		"当前市场广度（2026-07-28，截至2026-07-28T14:00:00+08:00）",
		"上一交易日盘后广度（2026-07-27）",
		"板块统计基准日：2026-07-27",
		"强势板块：光伏+3.00%",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("text missing %q: %s", want, text)
		}
	}
	if strings.Contains(text, "储能+2.00%") || strings.Contains(text, "保险+0.00%") {
		t.Fatalf("text ignored hotspotTop limit: %s", text)
	}
}

func TestBuildAgentMarketReviewTextMarksCurrentBreadthUnavailable(t *testing.T) {
	text := buildAgentMarketReviewText(AgentMarketReview{
		ReviewType: "preopen",
		CurrentBreadth: AgentMarketBreadth{
			DataType:   "current_snapshot",
			Available:  false,
			SourceNote: "当前交易日尚无有效行情",
		},
		LatestCompletedBreadth: AgentMarketBreadth{
			Available: true, Date: "2026-07-27", UniverseTotal: 2, ValidCount: 2,
		},
	})

	if !strings.Contains(text, "当前市场广度：盘前尚未产生；最近完整盘后广度见下文") {
		t.Fatalf("unexpected text: %s", text)
	}
}

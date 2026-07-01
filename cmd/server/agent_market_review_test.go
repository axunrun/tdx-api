package main

import (
	"strings"
	"testing"
	"time"

	"github.com/injoyai/tdx/protocol"
)

func TestResolveMarketReviewTypeAuto(t *testing.T) {
	tests := []struct {
		at   time.Time
		want string
	}{
		{at: time.Date(2026, 1, 1, 9, 0, 0, 0, time.Local), want: "preopen"},
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
}

func TestBuildMarketBreadth(t *testing.T) {
	stats := []*protocol.TdxStat{
		{Code: "000001", ChangePct: 1},
		{Code: "000002", ChangePct: -2},
		{Code: "000003", ChangePct: 0},
		{Code: "000004", ChangePct: 10},
		{Code: "000005", ChangePct: -10},
	}

	got := buildMarketBreadth(stats)

	if got.Total != 5 || got.Rising != 2 || got.Falling != 2 || got.Flat != 1 {
		t.Fatalf("unexpected breadth: %+v", got)
	}
	if got.LimitUp != 1 || got.LimitDown != 1 {
		t.Fatalf("unexpected limit counts: %+v", got)
	}
}

func TestBuildAgentMarketReviewText(t *testing.T) {
	summary := AgentMarketReview{
		ReviewType: "full",
		Indexes: []AgentMarketIndex{
			{Name: "上证指数", ChangePct: 1.2},
		},
		Breadth: AgentMarketBreadth{
			Rising: 2, Falling: 1, Flat: 0, RisingPct: 66.67, LimitUp: 1, LimitDown: 0,
			AverageChange: 1.1, MedianChange: 0.8,
		},
		Hotspots: &AgentMarketHotspots{
			Strong: []AgentHotspotSector{{Name: "光伏", AverageValue: 3}},
			Middle: []AgentHotspotSector{{Name: "储能", AverageValue: 1}},
			Weak:   []AgentHotspotSector{{Name: "白酒", AverageValue: -2}},
		},
	}

	text := buildAgentMarketReviewText(summary)

	for _, want := range []string{"市场复盘：全天收盘复盘", "主要指数：上证指数+1.20%", "市场广度", "强势板块：光伏+3.00%"} {
		if !strings.Contains(text, want) {
			t.Fatalf("text missing %q: %s", want, text)
		}
	}
}

package main

import (
	"testing"
	"time"
)

func TestResolvePaperMarketSession(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	tests := []struct {
		name   string
		at     time.Time
		status string
		text   string
	}{
		{
			name:   "weekend",
			at:     time.Date(2026, 7, 4, 10, 0, 0, 0, loc),
			status: "closed",
			text:   "非交易日",
		},
		{
			name:   "preopen",
			at:     time.Date(2026, 7, 1, 9, 20, 0, 0, loc),
			status: "preopen",
			text:   "开盘前",
		},
		{
			name:   "morning",
			at:     time.Date(2026, 7, 1, 10, 0, 0, 0, loc),
			status: "trading",
			text:   "上午交易中",
		},
		{
			name:   "break",
			at:     time.Date(2026, 7, 1, 12, 0, 0, 0, loc),
			status: "break",
			text:   "午间休市",
		},
		{
			name:   "afternoon",
			at:     time.Date(2026, 7, 1, 14, 0, 0, 0, loc),
			status: "trading",
			text:   "下午交易中",
		},
		{
			name:   "closed",
			at:     time.Date(2026, 7, 1, 15, 30, 0, 0, loc),
			status: "closed",
			text:   "已收盘",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolvePaperMarketSession(tt.at)
			if got.status != tt.status || got.text != tt.text {
				t.Fatalf("session = %+v, want %s/%s", got, tt.status, tt.text)
			}
		})
	}
}

func TestPaperMarketSnapshotTTL(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	if got := paperMarketSnapshotTTL(
		paperMarketSession{status: "trading"},
		time.Date(2026, 7, 1, 10, 0, 0, 0, loc),
	); got != 30*time.Second {
		t.Fatalf("trading ttl = %s", got)
	}
	if got := paperMarketSnapshotTTL(
		paperMarketSession{status: "closed"},
		time.Date(2026, 7, 1, 16, 0, 0, 0, loc),
	); got != 10*time.Minute {
		t.Fatalf("closed ttl = %s", got)
	}
	if got := paperMarketSnapshotTTL(
		paperMarketSession{status: "break"},
		time.Date(2026, 7, 1, 12, 59, 50, 0, loc),
	); got != 10*time.Second {
		t.Fatalf("boundary ttl = %s", got)
	}
}

func TestApplyPaperMarketFallbackKeepsPreviousBreadth(t *testing.T) {
	previous := PaperMarketSnapshot{
		Breadth: AgentMarketBreadth{Total: 10, Rising: 6, Falling: 3, Flat: 1},
	}
	snapshot := PaperMarketSnapshot{
		Breadth:  AgentMarketBreadth{Total: 0},
		Warnings: []string{"GetTdxStat失败: 超时"},
	}

	got := applyPaperMarketFallback(snapshot, previous, true)

	if got.Breadth.Total != 10 || got.Breadth.Rising != 6 {
		t.Fatalf("breadth = %+v, want previous breadth", got.Breadth)
	}
	if len(got.Warnings) != 2 {
		t.Fatalf("warnings = %+v, want original warning plus fallback note", got.Warnings)
	}
}

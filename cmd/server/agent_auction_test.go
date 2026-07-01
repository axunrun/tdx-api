package main

import (
	"strings"
	"testing"
	"time"

	"github.com/injoyai/tdx/protocol"
)

func TestBuildAgentAuctionResultCalculatesLatestSnapshot(t *testing.T) {
	items := []*protocol.CallAuction{
		{
			Time:      time.Date(2026, 1, 1, 9, 24, 0, 0, time.Local),
			Price:     10000,
			Match:     100,
			Unmatched: 50,
			Flag:      1,
		},
		{
			Time:      time.Date(2026, 1, 1, 9, 25, 0, 0, time.Local),
			Price:     10500,
			Match:     100,
			Unmatched: 200,
			Flag:      1,
		},
	}

	got := buildAgentAuctionResult(items, 10, 1)

	if got.Count != 2 || len(got.Items) != 1 {
		t.Fatalf("unexpected items: %+v", got)
	}
	if got.LatestPrice != 10.5 || got.ChangePct != 5 {
		t.Fatalf("unexpected latest snapshot: %+v", got)
	}
	if !strings.Contains(strings.Join(got.Signals, ","), "高开竞价") {
		t.Fatalf("missing high open signal: %+v", got.Signals)
	}
}

func TestFilterAuctionSessionKeepsOpenAuctionOnly(t *testing.T) {
	items := []*protocol.CallAuction{
		{Time: time.Date(2026, 1, 1, 9, 19, 0, 0, time.Local)},
		{Time: time.Date(2026, 1, 1, 9, 24, 0, 0, time.Local)},
		{Time: time.Date(2026, 1, 1, 14, 59, 0, 0, time.Local)},
	}

	got := filterAuctionSession(items, "open")

	if len(got) != 1 || got[0].Time.Hour() != 9 {
		t.Fatalf("unexpected open auction items: %+v", got)
	}
}

func TestBuildAgentAuctionTextIsCompact(t *testing.T) {
	summary := AgentAuctionSummary{
		Code:    "603063",
		Name:    "禾望电气",
		Session: "open",
		Auction: &AgentAuctionResult{
			Count:             1,
			LatestTime:        "09:25:00",
			LatestPrice:       10.5,
			ChangePct:         5,
			MatchedVolume:     100,
			UnmatchedVolume:   200,
			UnmatchedSideText: "买盘未匹配",
			Signals:           []string{"高开竞价"},
		},
		Context: AgentAuctionContext{Ret5: 1.2, Ret20: -3.4},
	}

	text := buildAgentAuctionText(summary)

	for _, want := range []string{"集合竞价：禾望电气（603063），开盘竞价", "较昨收+5.00%", "信号：高开竞价", "近20日-3.40%"} {
		if !strings.Contains(text, want) {
			t.Fatalf("text missing %q: %s", want, text)
		}
	}
}

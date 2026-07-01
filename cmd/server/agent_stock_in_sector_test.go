package main

import (
	"strings"
	"testing"

	"github.com/injoyai/tdx/protocol"
)

func TestBuildAgentStockInSectorRanksTargetByChangePct(t *testing.T) {
	block := AgentBriefBlock{Type: "concept", TypeName: "概念板块", Name: "风电"}
	members := []stockRow{
		{Code: "603063", Name: "禾望电气", Exchange: "sh"},
		{Code: "000001", Name: "平安银行", Exchange: "sz"},
		{Code: "600000", Name: "浦发银行", Exchange: "sh"},
	}
	stats := []*protocol.TdxStat{
		{Code: "000001", ChangePct: 5, Chg5: 8, PETTM: 6},
		{Code: "603063", ChangePct: 3, Chg5: 4, PETTM: 50},
		{Code: "600000", ChangePct: -1, Chg5: 1, PETTM: 7},
	}

	got := buildAgentStockInSector("603063", block, members, stats, "changePct", 2)

	if got.Target == nil {
		t.Fatal("missing target")
	}
	if got.Target.Rank != 2 || got.Target.Percentile != 66.66666666666666 {
		t.Fatalf("unexpected target rank: %+v", got.Target)
	}
	if len(got.Top) != 2 || got.Top[0].Code != "000001" || got.Top[1].Code != "603063" {
		t.Fatalf("unexpected top list: %+v", got.Top)
	}
}

func TestBuildAgentStockInSectorTextIsCompactChinese(t *testing.T) {
	block := AgentBriefBlock{Type: "concept", TypeName: "概念板块", Name: "风电"}
	summary := buildAgentStockInSector("603063", block, []stockRow{
		{Code: "603063", Name: "禾望电气", Exchange: "sh"},
	}, []*protocol.TdxStat{
		{Code: "603063", ChangePct: 3},
	}, "changePct", 5)

	text := buildAgentStockInSectorText(summary)

	for _, want := range []string{"板块位置：", "风电", "排名 1/1", "禾望电气"} {
		if !strings.Contains(text, want) {
			t.Fatalf("text missing %q: %s", want, text)
		}
	}
	if strings.Contains(text, "{") || strings.Contains(text, `"code"`) {
		t.Fatalf("text should be plain Chinese: %s", text)
	}
}

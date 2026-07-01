package main

import (
	"strings"
	"testing"

	"github.com/injoyai/tdx/protocol"
)

func TestBuildAgentSectorDetailSplitsStrongMiddleWeak(t *testing.T) {
	sector := agentSectorMemberSet{
		Block: AgentBriefBlock{Type: "concept", TypeName: "概念板块", Name: "测试板块"},
		Members: []stockRow{
			{Code: "000001", Name: "一号"},
			{Code: "000002", Name: "二号"},
			{Code: "000003", Name: "三号"},
			{Code: "000004", Name: "四号"},
			{Code: "000005", Name: "五号"},
		},
	}
	stats := []*protocol.TdxStat{
		{Code: "000001", Chg20: 10, ChangePct: 1},
		{Code: "000002", Chg20: 5, ChangePct: 1},
		{Code: "000003", Chg20: 0, ChangePct: 0},
		{Code: "000004", Chg20: -5, ChangePct: -1},
		{Code: "000005", Chg20: -10, ChangePct: -1},
	}

	summary := buildAgentSectorDetail(nilSectorDetailKlineClient{}, sector, stats, "chg20", 2, false)

	if summary.MemberSize != 5 {
		t.Fatalf("member size = %d, want 5", summary.MemberSize)
	}
	if len(summary.TopStocks) != 2 || summary.TopStocks[0].Code != "000001" {
		t.Fatalf("top stocks = %#v", summary.TopStocks)
	}
	if len(summary.MidStocks) != 1 || summary.MidStocks[0].Code != "000003" {
		t.Fatalf("mid stocks = %#v", summary.MidStocks)
	}
	if len(summary.WeakStocks) != 2 || summary.WeakStocks[0].Code != "000005" {
		t.Fatalf("weak stocks = %#v", summary.WeakStocks)
	}

	text := buildAgentSectorDetailText(summary)
	for _, want := range []string{"板块深度", "强势股", "中游股", "弱势股"} {
		if !strings.Contains(text, want) {
			t.Fatalf("text missing %q: %s", want, text)
		}
	}
}

type nilSectorDetailKlineClient struct{}

func (nilSectorDetailKlineClient) GetIndexDayAll(string) (*protocol.KlineResp, error) {
	return nil, nil
}

package main

import (
	"strings"
	"testing"
	"time"

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

func TestBuildAgentSectorDetailAlignsIndexReturnsToCompletedDate(t *testing.T) {
	sector := agentSectorMemberSet{
		Block: AgentBriefBlock{
			Type:      "concept",
			TypeName:  "概念板块",
			Name:      "液冷服务",
			IndexCode: "880685",
		},
		Members: []stockRow{{Code: "000001", Name: "样本股"}},
	}
	stats := []*protocol.TdxStat{{
		Code:      "000001",
		Date:      "20260728",
		ChangePct: 1,
		Chg20:     2,
	}}
	klines := make(protocol.Klines, 62)
	start := time.Date(2026, 5, 29, 0, 0, 0, 0, time.Local)
	for i := range klines {
		klines[i] = &protocol.Kline{Time: start.AddDate(0, 0, i), Close: 100000}
	}
	klines[len(klines)-2].Close = 110000
	klines[len(klines)-1].Close = 121000
	client := fixedSectorDetailKlineClient{list: klines}

	now := time.Date(2026, 7, 29, 14, 59, 0, 0, time.Local)
	summary := buildAgentSectorDetailAt(client, sector, stats, "chg20", 2, false, now)

	if summary.ConstituentDataDate != "2026-07-28" {
		t.Fatalf("constituent date = %s", summary.ConstituentDataDate)
	}
	if summary.Stats.DailyReturn == nil || *summary.Stats.DailyReturn != 10 {
		t.Fatalf("daily return = %+v", summary.Stats.DailyReturn)
	}
	if summary.Stats.DailyBaseDate != "2026-07-27" ||
		summary.Stats.DailyDate != "2026-07-28" {
		t.Fatalf("daily dates = %+v", summary.Stats)
	}
	if summary.Stats.Return20EndDate != "2026-07-28" ||
		summary.Stats.Return60EndDate != "2026-07-28" {
		t.Fatalf("window dates must align to completed day: %+v", summary.Stats)
	}

	text := buildAgentSectorDetailText(summary)
	for _, want := range []string{
		"成分股统计日：2026-07-28",
		"最近完整交易日2026-07-28单日+10.00%",
		"较2026-07-27收盘",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("text missing %q: %s", want, text)
		}
	}
}

type fixedSectorDetailKlineClient struct {
	list protocol.Klines
}

func (c fixedSectorDetailKlineClient) GetIndexDayAll(string) (*protocol.KlineResp, error) {
	return &protocol.KlineResp{List: c.list}, nil
}

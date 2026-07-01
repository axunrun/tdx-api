package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/injoyai/tdx/protocol"
)

func TestBuildAgentHotspotScanRanksSectorsByAverageMetric(t *testing.T) {
	sectors := []agentSectorMemberSet{
		{
			Block: AgentBriefBlock{Type: "concept", TypeName: "概念板块", Name: "风电"},
			Members: []stockRow{
				{Code: "001399", Name: "N惠科"},
				{Code: "603063", Name: "禾望电气"},
				{Code: "000001", Name: "平安银行"},
			},
		},
		{
			Block: AgentBriefBlock{Type: "concept", TypeName: "概念板块", Name: "光伏"},
			Members: []stockRow{
				{Code: "600000", Name: "浦发银行"},
				{Code: "000002", Name: "万科A"},
			},
		},
	}
	stats := []*protocol.TdxStat{
		{Code: "001399", ChangePct: 315.02},
		{Code: "603063", ChangePct: 3, Chg20: 120},
		{Code: "000001", ChangePct: 5},
		{Code: "600000", ChangePct: -1},
		{Code: "000002", ChangePct: 1},
	}

	got := buildAgentHotspotScan(sectors, stats, "changePct", 10, 1, 1, true)

	if len(got.Sectors) != 2 {
		t.Fatalf("sector count = %d", len(got.Sectors))
	}
	if got.Sectors[0].Name != "风电" || got.Sectors[0].AverageValue != 4 {
		t.Fatalf("unexpected first sector: %+v", got.Sectors[0])
	}
	if got.Sectors[0].RisingCount != 2 || got.Sectors[0].RisingPct != 100 {
		t.Fatalf("unexpected rising stats: %+v", got.Sectors[0])
	}
	if got.ExcludedCount != 1 || got.Sectors[0].ExcludedCount != 1 {
		t.Fatalf("unexpected excluded count: %+v", got)
	}
	if len(got.Sectors[0].TopStocks) != 1 || got.Sectors[0].TopStocks[0].Code != "000001" {
		t.Fatalf("unexpected top stocks: %+v", got.Sectors[0].TopStocks)
	}
	if len(got.ColdSectors) != 2 || got.ColdSectors[0].Name != "光伏" {
		t.Fatalf("unexpected cold sectors: %+v", got.ColdSectors)
	}
	if len(got.ColdSectors[0].TopStocks) == 0 || got.ColdSectors[0].TopStocks[0].Code != "000002" {
		t.Fatalf("unexpected cold sector strong stocks: %+v", got.ColdSectors[0].TopStocks)
	}
}

func TestBuildAgentHotspotScanKeepsLongTermMomentumOutliers(t *testing.T) {
	sectors := []agentSectorMemberSet{
		{
			Block: AgentBriefBlock{Type: "concept", TypeName: "概念板块", Name: "CPO概念"},
			Members: []stockRow{
				{Code: "920083", Name: "北交样本"},
				{Code: "300000", Name: "正常样本"},
			},
		},
	}
	stats := []*protocol.TdxStat{
		{Code: "920083", ChangePct: 1, Chg20: 505.7},
		{Code: "300000", ChangePct: 1, Chg20: 20},
	}

	got := buildAgentHotspotScan(sectors, stats, "chg20", 20, 3, 1, true)

	if got.ExcludedCount != 0 || got.Sectors[0].AverageValue != 262.85 {
		t.Fatalf("long-term momentum should not be excluded: %+v", got)
	}
}

func TestBuildAgentHotspotScanExcludesDailyOutliers(t *testing.T) {
	sectors := []agentSectorMemberSet{
		{
			Block: AgentBriefBlock{Type: "concept", TypeName: "概念板块", Name: "CPO概念"},
			Members: []stockRow{
				{Code: "920083", Name: "当日异常"},
				{Code: "300000", Name: "正常样本"},
			},
		},
	}
	stats := []*protocol.TdxStat{
		{Code: "920083", ChangePct: 120, Chg20: 50},
		{Code: "300000", ChangePct: 1, Chg20: 20},
	}

	got := buildAgentHotspotScan(sectors, stats, "chg20", 20, 3, 1, true)

	if got.ExcludedCount != 1 || got.Sectors[0].AverageValue != 20 {
		t.Fatalf("daily outlier should be excluded: %+v", got)
	}
}

func TestHotspotWindowReturnUsesWindowAndOffset(t *testing.T) {
	klines := protocol.Klines{
		{Close: 100000},
		{Close: 110000},
		{Close: 121000},
		{Close: 200000},
	}

	got, ok := hotspotWindowReturn(klines, 2, 1)

	if !ok || got != 21 {
		t.Fatalf("window return = %.2f, ok=%v", got, ok)
	}
}

func TestHotspotDateWindowReturnUsesDateRange(t *testing.T) {
	klines := protocol.Klines{
		{Time: time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local), Close: 100000},
		{Time: time.Date(2026, 1, 2, 0, 0, 0, 0, time.Local), Close: 110000},
		{Time: time.Date(2026, 1, 3, 0, 0, 0, 0, time.Local), Close: 121000},
		{Time: time.Date(2026, 1, 4, 0, 0, 0, 0, time.Local), Close: 200000},
	}

	got, ok := hotspotDateWindowReturn(
		klines,
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local),
		time.Date(2026, 1, 3, 0, 0, 0, 0, time.Local),
	)

	if !ok || got != 21 {
		t.Fatalf("date window return = %.2f, ok=%v", got, ok)
	}
}

func TestBuildAgentHotspotScanReturnsStrongMiddleAndWeakSectors(t *testing.T) {
	sectors := make([]agentSectorMemberSet, 7)
	stats := make([]*protocol.TdxStat, 7)
	for i, value := range []float64{30, 20, 10, 0, -10, -20, -30} {
		code := fmt.Sprintf("%06d", i+1)
		sectors[i] = agentSectorMemberSet{
			Block:   AgentBriefBlock{Type: "concept", TypeName: "概念板块", Name: fmt.Sprintf("板块%d", i+1)},
			Members: []stockRow{{Code: code, Name: fmt.Sprintf("股票%d", i+1)}},
		}
		stats[i] = &protocol.TdxStat{Code: code, Chg20: value}
	}

	got := buildAgentHotspotScan(sectors, stats, "chg20", 2, 1, 1, true)

	if len(got.Sectors) != 2 || got.Sectors[0].Name != "板块1" {
		t.Fatalf("unexpected strong sectors: %+v", got.Sectors)
	}
	if len(got.MiddleSectors) != 2 || got.MiddleSectors[0].Name != "板块4" {
		t.Fatalf("unexpected middle sectors: %+v", got.MiddleSectors)
	}
	if len(got.ColdSectors) != 2 || got.ColdSectors[0].Name != "板块7" {
		t.Fatalf("unexpected weak sectors: %+v", got.ColdSectors)
	}
}

func TestBuildAgentHotspotScanDoesNotDuplicateWeakWhenSampleIsShort(t *testing.T) {
	sectors := make([]agentSectorMemberSet, 2)
	stats := make([]*protocol.TdxStat, 2)
	for i, value := range []float64{10, -10} {
		code := fmt.Sprintf("%06d", i+1)
		sectors[i] = agentSectorMemberSet{
			Block:   AgentBriefBlock{Type: "concept", TypeName: "姒傚康鏉垮潡", Name: fmt.Sprintf("鏉垮潡%d", i+1)},
			Members: []stockRow{{Code: code, Name: fmt.Sprintf("鑲＄エ%d", i+1)}},
		}
		stats[i] = &protocol.TdxStat{Code: code, Chg20: value}
	}

	sectorValues := map[string]float64{
		sectors[0].Block.Name: 10,
		sectors[1].Block.Name: -10,
	}
	got := buildAgentHotspotScanWithValues(
		sectors,
		stats,
		"windowReturn",
		20,
		1,
		1,
		true,
		sectorValues,
		0,
		0,
		"2026-05-25",
		"2026-06-26",
		nil,
	)

	if len(got.Sectors) != 2 || len(got.ColdSectors) != 0 {
		t.Fatalf("short samples should not duplicate weak sectors: %+v", got)
	}
	if len(got.Warnings) == 0 {
		t.Fatalf("short samples should warn: %+v", got)
	}
}

func TestBuildAgentHotspotScanDefaultsToTwentyDayRanking(t *testing.T) {
	sectors := make([]agentSectorMemberSet, 25)
	stats := make([]*protocol.TdxStat, 25)
	for i := range sectors {
		code := fmt.Sprintf("%06d", i+1)
		sectors[i] = agentSectorMemberSet{
			Block:   AgentBriefBlock{Type: "concept", TypeName: "概念板块", Name: fmt.Sprintf("板块%d", i+1)},
			Members: []stockRow{{Code: code, Name: fmt.Sprintf("股票%d", i+1)}},
		}
		stats[i] = &protocol.TdxStat{Code: code, ChangePct: 0, Chg20: float64(i)}
	}

	got := buildAgentHotspotScan(sectors, stats, "", 0, 0, 1, true)

	if got.Metric != "chg20" || got.Limit != 20 || len(got.Sectors) != 20 {
		t.Fatalf("unexpected defaults: %+v", got)
	}
	if got.Sectors[0].Name != "板块25" {
		t.Fatalf("default ranking should use chg20: %+v", got.Sectors[0])
	}
}

func TestBuildAgentHotspotScanTextIsCompactChinese(t *testing.T) {
	summary := AgentHotspotScan{
		Metric:        "changePct",
		ExcludeNew:    true,
		ExcludedCount: 1,
		Sectors: []AgentHotspotSector{
			{
				Name:         "风电",
				TypeName:     "概念板块",
				AverageValue: 4,
				RisingCount:  2,
				MemberCount:  2,
				TopStocks: []AgentStockInSectorItem{
					{Code: "000001", Name: "平安银行", Value: 5},
				},
			},
		},
		ColdSectors: []AgentHotspotSector{
			{
				Name:         "煤炭",
				AverageValue: -6,
				RisingCount:  1,
				MemberCount:  10,
				TopStocks: []AgentStockInSectorItem{
					{Code: "600000", Name: "抗跌样本", Value: -1},
				},
			},
		},
		MiddleSectors: []AgentHotspotSector{
			{
				Name:         "消费电子",
				AverageValue: 1,
				RisingCount:  5,
				MemberCount:  10,
			},
		},
	}

	text := buildAgentHotspotScanText(summary)

	for _, want := range []string{"热点扫描：", "最强板块：", "中游板块：", "最弱板块：", "已排除新股/异常涨幅样本1条", "风电", "平均+4.00%", "平安银行+5.00%", "消费电子", "煤炭", "平均-6.00%", "抗跌股：抗跌样本-1.00%"} {
		if !strings.Contains(text, want) {
			t.Fatalf("text missing %q: %s", want, text)
		}
	}
	if strings.Contains(text, "{") || strings.Contains(text, `"code"`) {
		t.Fatalf("text should be plain Chinese: %s", text)
	}
}

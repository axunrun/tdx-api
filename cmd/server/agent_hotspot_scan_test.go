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

func TestHotspotDateWindowReturnReportsActualTradingDates(t *testing.T) {
	klines := protocol.Klines{
		{Time: time.Date(2026, 1, 2, 0, 0, 0, 0, time.Local), Close: 100000},
		{Time: time.Date(2026, 1, 5, 0, 0, 0, 0, time.Local), Close: 110000},
	}

	got, startDate, endDate, ok := hotspotDateWindowReturnWithDates(
		klines,
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local),
		time.Date(2026, 1, 6, 0, 0, 0, 0, time.Local),
	)

	if !ok || got != 10 || startDate != "2026-01-02" || endDate != "2026-01-05" {
		t.Fatalf(
			"date window return = %.2f, range=%s..%s, ok=%v",
			got,
			startDate,
			endDate,
			ok,
		)
	}
}

func TestMergeHotspotDateRangeRejectsMismatchedRange(t *testing.T) {
	startDate, endDate, mixed := mergeHotspotDateRange(
		"2026-07-27",
		"2026-07-28",
		"2026-07-26",
		"2026-07-28",
	)

	if !mixed || startDate != "2026-07-27" || endDate != "2026-07-28" {
		t.Fatalf("unexpected merged range: %s..%s mixed=%v", startDate, endDate, mixed)
	}
}

func TestBuildAgentHotspotScanUsesOnlyLatestStatDate(t *testing.T) {
	sectors := []agentSectorMemberSet{
		{
			Block: AgentBriefBlock{Type: "concept", TypeName: "概念板块", Name: "风电"},
			Members: []stockRow{
				{Code: "000001", Name: "旧日期样本"},
				{Code: "000002", Name: "最新日期样本"},
			},
		},
	}
	stats := []*protocol.TdxStat{
		{Code: "000001", Date: "20260727", Chg20: 100},
		{Code: "000002", Date: "20260728", Chg20: -10},
	}

	got := buildAgentHotspotScan(sectors, stats, "chg20", 1, 1, 1, false)

	if got.ConstituentDataDate != "2026-07-28" || got.MetricEndDate != "2026-07-28" {
		t.Fatalf("unexpected data dates: %+v", got)
	}
	if len(got.Sectors) != 1 || got.Sectors[0].AverageValue != -10 {
		t.Fatalf("old-date stats must not enter latest snapshot: %+v", got.Sectors)
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
	indexReturn := 3.5
	dailyReturn := 1.25
	summary := AgentHotspotScan{
		GeneratedAt:             "2026-07-29T15:01:00+08:00",
		MetricSource:            "TdxStat成分股聚合",
		MetricEndDate:           "2026-07-28",
		ConstituentDataDate:     "2026-07-28",
		BoardIndexStartDate:     "2026-07-08",
		BoardIndexEndDate:       "2026-07-28",
		BoardIndexDailyBaseDate: "2026-07-28",
		BoardIndexDailyDate:     "2026-07-29",
		Metric:                  "chg20",
		ExcludeNew:              true,
		ExcludedCount:           1,
		Sectors: []AgentHotspotSector{
			{
				Name:                  "风电",
				TypeName:              "概念板块",
				AverageValue:          4,
				BoardIndexReturn:      &indexReturn,
				BoardIndexDailyReturn: &dailyReturn,
				RisingCount:           2,
				MemberCount:           2,
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

	for _, want := range []string{
		"热点扫描：",
		"生成时间：2026-07-29T15:01:00+08:00",
		"指标数据：TdxStat成分股聚合，最近完整交易日2026-07-28",
		"板块指数辅助数据：TDX板块指数日K，实际交易日区间2026-07-08至2026-07-28",
		"板块指数固定单日数据：最近完整交易日2026-07-29（较2026-07-28收盘）",
		"最强板块：",
		"中游板块：",
		"最弱板块：",
		"已排除新股/异常涨幅样本1条",
		"风电",
		"指数：最近完整交易日单日+1.25%；近20日+3.50%",
		"成分：近20日平均+4.00%；上涨2/2",
		"平安银行+5.00%",
		"消费电子",
		"煤炭",
		"成分：近20日平均-6.00%",
		"近20日抗跌股：抗跌样本-1.00%",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("text missing %q: %s", want, text)
		}
	}
	if strings.Contains(text, "{") || strings.Contains(text, `"code"`) {
		t.Fatalf("text should be plain Chinese: %s", text)
	}
}

func TestHotspotMetricWindowMatchesTdxStatPeriod(t *testing.T) {
	tests := []struct {
		metric string
		window int
		ok     bool
	}{
		{metric: "changePct", window: 1, ok: true},
		{metric: "chg5", window: 5, ok: true},
		{metric: "chg20", window: 20, ok: true},
		{metric: "chg60", window: 60, ok: true},
		{metric: "windowReturn", ok: false},
	}
	for _, tt := range tests {
		window, ok := hotspotMetricWindow(tt.metric)
		if window != tt.window || ok != tt.ok {
			t.Fatalf("%s window=%d ok=%v", tt.metric, window, ok)
		}
	}
}

func TestHotspotWindowReturnEndsAtCompletedDataDate(t *testing.T) {
	klines := protocol.Klines{
		{Time: time.Date(2026, 7, 27, 0, 0, 0, 0, time.Local), Close: 100000},
		{Time: time.Date(2026, 7, 28, 0, 0, 0, 0, time.Local), Close: 110000},
		{Time: time.Date(2026, 7, 29, 0, 0, 0, 0, time.Local), Close: 220000},
	}

	value, startDate, endDate, ok := hotspotWindowReturnUntilDateWithDates(
		klines,
		1,
		"2026-07-28",
	)

	if !ok || value != 10 || startDate != "2026-07-27" || endDate != "2026-07-28" {
		t.Fatalf(
			"return=%.2f range=%s..%s ok=%v",
			value,
			startDate,
			endDate,
			ok,
		)
	}
}

func TestHotspotCompletedDailyReturnSwitchesAfterClose(t *testing.T) {
	klines := protocol.Klines{
		{Time: time.Date(2026, 7, 27, 0, 0, 0, 0, time.Local), Close: 100000},
		{Time: time.Date(2026, 7, 28, 0, 0, 0, 0, time.Local), Close: 110000},
		{Time: time.Date(2026, 7, 29, 0, 0, 0, 0, time.Local), Close: 121000},
	}

	beforeClose := time.Date(2026, 7, 29, 14, 59, 0, 0, time.Local)
	value, baseDate, endDate, ok := hotspotCompletedDailyReturn(klines, beforeClose)
	if !ok || value != 10 || baseDate != "2026-07-27" || endDate != "2026-07-28" {
		t.Fatalf(
			"before close return=%.2f range=%s..%s ok=%v",
			value,
			baseDate,
			endDate,
			ok,
		)
	}

	afterClose := time.Date(2026, 7, 29, 15, 1, 0, 0, time.Local)
	value, baseDate, endDate, ok = hotspotCompletedDailyReturn(klines, afterClose)
	if !ok || value != 10 || baseDate != "2026-07-28" || endDate != "2026-07-29" {
		t.Fatalf(
			"after close return=%.2f range=%s..%s ok=%v",
			value,
			baseDate,
			endDate,
			ok,
		)
	}
}

func TestBuildAgentHotspotScanTextDescribesCompletedDailyDates(t *testing.T) {
	summary := AgentHotspotScan{
		GeneratedAt:         "2026-07-29T15:01:00+08:00",
		MetricSource:        "TDX板块指数日K",
		MetricStartDate:     "2026-07-28",
		MetricEndDate:       "2026-07-29",
		ConstituentDataDate: "2026-07-28",
		Metric:              "dailyReturn",
		MinMembers:          20,
		Sectors: []AgentHotspotSector{{
			Name:                    "液冷服务",
			AverageValue:            2.5,
			ConstituentAverageValue: 1.2,
			RisingCount:             100,
			MemberCount:             200,
		}},
	}

	text := buildAgentHotspotScanText(summary)
	for _, want := range []string{
		"最近完整交易日2026-07-29",
		"较2026-07-28收盘",
		"指数：最近完整交易日单日+2.50%",
		"成分：单日平均+1.20%",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("text missing %q: %s", want, text)
		}
	}
}

func TestBuildAgentHotspotScanDailyReturnRanksByIndexAndKeepsConstituentAverage(t *testing.T) {
	sectors := []agentSectorMemberSet{
		{
			Block:   AgentBriefBlock{Name: "板块A", Type: "concept"},
			Members: []stockRow{{Code: "000001", Name: "股票A"}},
		},
		{
			Block:   AgentBriefBlock{Name: "板块B", Type: "concept"},
			Members: []stockRow{{Code: "000002", Name: "股票B"}},
		},
	}
	stats := []*protocol.TdxStat{
		{Code: "000001", Date: "20260728", ChangePct: 8},
		{Code: "000002", Date: "20260728", ChangePct: -3},
	}
	values := map[string]float64{"板块A": -1, "板块B": 2}

	summary := buildAgentHotspotScanWithValues(
		sectors,
		stats,
		"dailyReturn",
		1,
		1,
		1,
		false,
		values,
		0,
		0,
		"",
		"",
		nil,
	)

	if len(summary.Sectors) != 1 || summary.Sectors[0].Name != "板块B" {
		t.Fatalf("daily ranking must use board index return: %+v", summary.Sectors)
	}
	if summary.Sectors[0].AverageValue != 2 ||
		summary.Sectors[0].ConstituentAverageValue != -3 {
		t.Fatalf("daily values were mixed: %+v", summary.Sectors[0])
	}
}

func TestBuildAgentHotspotScanTextDatesBothChangePctIndexReturns(t *testing.T) {
	alignedReturn := -0.5
	latestReturn := 1.25
	summary := AgentHotspotScan{
		Metric:                  "changePct",
		MetricSource:            "TdxStat成分股聚合",
		MetricEndDate:           "2026-07-28",
		ConstituentDataDate:     "2026-07-28",
		BoardIndexStartDate:     "2026-07-27",
		BoardIndexEndDate:       "2026-07-28",
		BoardIndexDailyBaseDate: "2026-07-28",
		BoardIndexDailyDate:     "2026-07-29",
		MinMembers:              20,
		Sectors: []AgentHotspotSector{{
			Name:                  "液冷服务",
			AverageValue:          0.8,
			BoardIndexReturn:      &alignedReturn,
			BoardIndexDailyReturn: &latestReturn,
			MemberCount:           100,
		}},
	}

	text := buildAgentHotspotScanText(summary)
	for _, want := range []string{
		"指数：最近完整交易日单日+1.25%；同统计日单日-0.50%",
		"成分：单日平均+0.80%",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("text missing %q: %s", want, text)
		}
	}
}

func TestApplyHotspotIndexReturnsKeepsOnlyCommonRange(t *testing.T) {
	sectors := []AgentHotspotSector{{Name: "板块A"}, {Name: "板块B"}}
	results := map[string]hotspotIndexReturn{
		"板块A": {
			Value:     3.5,
			StartDate: "2026-07-08",
			EndDate:   "2026-07-28",
		},
		"板块B": {
			Value:     2.5,
			StartDate: "2026-07-09",
			EndDate:   "2026-07-28",
		},
	}
	mismatched := 0

	applyHotspotIndexReturns(
		sectors,
		results,
		"2026-07-08|2026-07-28",
		&mismatched,
	)

	if sectors[0].BoardIndexReturn == nil || *sectors[0].BoardIndexReturn != 3.5 {
		t.Fatalf("common-range result missing: %+v", sectors[0])
	}
	if sectors[1].BoardIndexReturn != nil || mismatched != 1 {
		t.Fatalf("mismatched range should be omitted: %+v mismatched=%d", sectors[1], mismatched)
	}
}

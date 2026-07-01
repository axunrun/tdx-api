package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/injoyai/tdx/protocol"
)

func TestKlineSummaryLevelLimits(t *testing.T) {
	tests := []struct {
		level string
		want  int
	}{
		{"brief", 60},
		{"", 120},
		{"normal", 120},
		{"deep", 250},
	}
	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			limits := resolveAgentKlineLimits(tt.level, 0)
			if limits.Day != tt.want {
				t.Fatalf("Day limit = %d, want %d", limits.Day, tt.want)
			}
			if !limits.WeekAll || !limits.MonthAll {
				t.Fatalf("week/month should use all data: %#v", limits)
			}
		})
	}
}

func TestKlineSummaryDayCountIsCapped(t *testing.T) {
	limits := resolveAgentKlineLimits("deep", 999)
	if limits.Day != 500 {
		t.Fatalf("Day limit = %d, want capped 500", limits.Day)
	}
}

func TestAgentKlinePeriodNameUsesReadableChinese(t *testing.T) {
	tests := []struct {
		period string
		want   string
	}{
		{"day", "日线"},
		{"week", "周线"},
		{"month", "月线"},
	}
	for _, tt := range tests {
		t.Run(tt.period, func(t *testing.T) {
			if got := agentKlinePeriodName(tt.period); got != tt.want {
				t.Fatalf("period name = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildAgentKlinePeriodRawReturnsOriginalKlineItems(t *testing.T) {
	klines := protocol.Klines(testKlineResp(25).List)

	period := buildAgentKlinePeriodRaw("day", klines, 20)

	if period.Period != "day" {
		t.Fatalf("period = %q, want day", period.Period)
	}
	if period.TotalCount != 25 || period.ReturnedCount != 20 || len(period.Items) != 20 {
		t.Fatalf("counts = %#v, items = %d", period, len(period.Items))
	}
	first := period.Items[0]
	if first.Date == "" || first.Open <= 0 || first.High <= 0 ||
		first.Low <= 0 || first.Close <= 0 || first.Volume <= 0 {
		t.Fatalf("raw kline item missing OHLCV: %#v", first)
	}
	if first.Amount <= 0 {
		t.Fatalf("raw kline item missing amount: %#v", first)
	}
}

func TestAgentKlineSummaryJSONUsesRawPeriodsOnly(t *testing.T) {
	klines := protocol.Klines(testKlineResp(25).List)
	summary := AgentKlineSummary{
		Code: "603063",
		Periods: []AgentKlinePeriodRaw{
			buildAgentKlinePeriodRaw("day", klines, 20),
		},
		analysisPeriods: []AgentKlinePeriodSummary{
			buildAgentKlinePeriodSummary("day", "日线", klines, 20),
		},
	}

	encoded := mustJSON(t, summary)

	mustContain := []string{`"items"`, `"open"`, `"high"`, `"low"`, `"close"`, `"volume"`}
	for _, want := range mustContain {
		if !strings.Contains(encoded, want) {
			t.Fatalf("expected JSON to contain %s:\n%s", want, encoded)
		}
	}
	mustNotContain := []string{`"trendStage"`, `"riskLevel"`, `"summary"`, `"movingAverages"`}
	for _, noise := range mustNotContain {
		if strings.Contains(encoded, noise) {
			t.Fatalf("expected raw JSON not to contain %s:\n%s", noise, encoded)
		}
	}
}

func TestBuildAgentKlinePeriodSummaryUsesCompactSignals(t *testing.T) {
	klines := protocol.Klines(testKlineResp(80).List)

	period := buildAgentKlinePeriodSummary("day", "日线", klines, 60)

	if period.Period != "day" || period.Name != "日线" {
		t.Fatalf("period = %#v", period)
	}
	if period.TotalCount != 80 || period.UsedCount != 60 {
		t.Fatalf("counts = %d/%d, want 80/60", period.TotalCount, period.UsedCount)
	}
	if period.StartDate == "" || period.EndDate == "" {
		t.Fatalf("date range missing: %#v", period)
	}
	if period.Close <= 0 || period.High <= 0 || period.Low <= 0 {
		t.Fatalf("price fields missing: %#v", period)
	}
	if len(period.Signals) == 0 {
		t.Fatalf("expected signals, got none")
	}
	if period.StageReturns["ret5"] == 0 || period.StageReturns["ret20"] == 0 {
		t.Fatalf("stage returns missing: %#v", period.StageReturns)
	}
	if period.Volume.Avg5 <= 0 || period.Volume.Avg20 <= 0 || period.Volume.VolumeRatio <= 0 {
		t.Fatalf("volume summary missing: %#v", period.Volume)
	}
	if period.KeyLevels.High20 <= 0 || period.KeyLevels.Low20 <= 0 ||
		period.KeyLevels.DistanceToHigh20Pct == 0 {
		t.Fatalf("key levels missing: %#v", period.KeyLevels)
	}
	if period.MovingAverages.MA5 == nil || period.MovingAverages.MA20 == nil ||
		period.MovingAverages.PriceVsMA20Pct == 0 {
		t.Fatalf("moving average summary missing: %#v", period.MovingAverages)
	}
	if period.TrendStage == "" || period.RiskLevel == "" || period.Summary == "" {
		t.Fatalf("trend stage summary missing: %#v", period)
	}
}

func TestBuildAgentKlinePeriodSummaryAddsSecondBatchKlineDetails(t *testing.T) {
	klines := protocol.Klines{
		testSummaryKline("2026-01-01", 100, 103, 99, 102),
		testSummaryKline("2026-01-02", 102, 105, 101, 104),
		testSummaryKline("2026-01-03", 104, 107, 103, 106),
		testSummaryKline("2026-01-04", 106, 109, 105, 108),
		testSummaryKline("2026-01-05", 108, 111, 107, 110),
		testSummaryKline("2026-01-06", 116, 132, 114, 118),
	}

	period := buildAgentKlinePeriodSummary("day", "日线", klines, 6)

	if period.Candle.Shape == "" || period.Candle.BodyPct <= 0 ||
		period.Candle.UpperShadowPct <= period.Candle.BodyPct {
		t.Fatalf("candle detail missing: %#v", period.Candle)
	}
	if period.Volatility.AtrPct <= 0 || period.Volatility.AvgAmplitude5Pct <= 0 ||
		period.Volatility.Risk == "" {
		t.Fatalf("volatility detail missing: %#v", period.Volatility)
	}
	if period.Streak.Direction != "up" || period.Streak.Count != 5 ||
		period.Streak.ChangePct <= 0 {
		t.Fatalf("streak detail mismatch: %#v", period.Streak)
	}
	if !containsKlineSignal(period.Signals, "跳空高开") ||
		!containsKlineSignal(period.Signals, "长上影线") {
		t.Fatalf("pattern signals missing: %#v", period.Signals)
	}
}

func TestBuildAgentKlineSummaryTextIsChineseAndCompact(t *testing.T) {
	summary := AgentKlineSummary{
		Code: "603063",
		Name: "禾望电气",
		analysisPeriods: []AgentKlinePeriodSummary{
			{
				Period:         "day",
				Name:           "日线",
				TotalCount:     250,
				UsedCount:      120,
				StartDate:      "2026-01-01",
				EndDate:        "2026-06-24",
				Close:          50.15,
				ChangePct:      12.34,
				High:           60.00,
				Low:            40.00,
				MaxDrawdownPct: -8.50,
				VolatilityPct:  6.10,
				Trend:          "上行",
				TrendStage:     "上升趋势中的回调",
				RiskLevel:      "中",
				Position:       "接近区间高位",
				StageReturns: map[string]float64{
					"ret5":  1.20,
					"ret20": -3.40,
					"ret60": 12.34,
				},
				Volume: AgentKlineVolumeSummary{
					Avg5:        120000,
					Avg20:       100000,
					VolumeRatio: 1.20,
					Signal:      "温和放量",
				},
				MovingAverages: AgentKlineMASummary{
					MA5:            floatPtr(51.10),
					MA20:           floatPtr(48.20),
					MA60:           floatPtr(44.30),
					Alignment:      "多头排列",
					PriceVsMA20Pct: 4.05,
				},
				KeyLevels: AgentKlineKeyLevels{
					High20:              60,
					Low20:               42,
					DistanceToHigh20Pct: -16.42,
					DistanceToLow20Pct:  19.40,
				},
				Candle: AgentKlineCandleSummary{
					Shape:          "bullish",
					UpperShadowPct: 2.1,
				},
				Volatility: AgentKlineVolatility{
					AtrPct: 6.1,
				},
				Streak: AgentKlineStreak{
					Direction: "up",
					Count:     3,
					ChangePct: 5.6,
				},
				Summary: "中期趋势保持上行，短线仍需观察量能延续。",
				Signals: []string{"收盘高于MA20", "收盘高于MA60"},
			},
		},
	}

	content := buildAgentKlineSummaryText(summary)

	mustContain := []string{
		"股票：禾望电气（603063）",
		"K线摘要：",
		"日线：样本 120/250",
		"区间 2026-01-01 至 2026-06-24",
		"涨跌幅 +12.34%",
		"最大回撤 -8.50%",
		"趋势 上行",
		"阶段：上升趋势中的回调",
		"风险：中",
		"近5/20/60涨跌：+1.20%/-3.40%/+12.34%",
		"量能：温和放量",
		"均线：多头排列",
		"距20日高点 -16.42%",
		"收盘高于MA20",
	}
	for _, want := range mustContain {
		if !strings.Contains(content, want) {
			t.Fatalf("expected %q in content:\n%s", want, content)
		}
	}
	if strings.Contains(content, `"items"`) || strings.Contains(content, `"open"`) ||
		strings.Contains(content, `"raw"`) {
		t.Fatalf("content should be compact Chinese text:\n%s", content)
	}
}

func floatPtr(value float64) *float64 {
	return &value
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal JSON: %v", err)
	}
	return string(data)
}

func testSummaryKline(date string, open, high, low, close int64) *protocol.Kline {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		panic(err)
	}
	return &protocol.Kline{
		Time:   t,
		Open:   protocol.Price(open * 1000),
		High:   protocol.Price(high * 1000),
		Low:    protocol.Price(low * 1000),
		Close:  protocol.Price(close * 1000),
		Volume: 1000,
		Amount: protocol.Price(close * 1000),
	}
}

func containsKlineSignal(signals []string, code string) bool {
	for _, signal := range signals {
		if strings.Contains(signal, code) {
			return true
		}
	}
	return false
}

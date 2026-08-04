package main

import (
	"encoding/json"
	"math"
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

func TestKlineReturnPctUsesNTradingIntervals(t *testing.T) {
	klines := protocol.Klines{
		{Close: protocol.Yuan(100)},
		{Close: protocol.Yuan(110)},
		{Close: protocol.Yuan(120)},
		{Close: protocol.Yuan(130)},
		{Close: protocol.Yuan(140)},
		{Close: protocol.Yuan(150)},
	}

	if got := klineReturnPct(klines, 5); math.Abs(got-50) > 1e-9 {
		t.Fatalf("5-day return = %.4f, want 50.0000", got)
	}
}

func TestKlineSummaryUsesFullSeriesForRecursiveIndicators(t *testing.T) {
	klines := make(protocol.Klines, 0, 250)
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local)
	for i := 0; i < 250; i++ {
		closePrice := 100 + float64((i*7)%19) - float64(i%5)
		klines = append(klines, &protocol.Kline{
			Time:   start.AddDate(0, 0, i),
			Open:   protocol.Yuan(closePrice - 0.5),
			High:   protocol.Yuan(closePrice + 1 + float64(i%4)),
			Low:    protocol.Yuan(closePrice - 1),
			Close:  protocol.Yuan(closePrice),
			Volume: int64(1000 + i),
		})
	}

	period := buildAgentKlinePeriodSummary("day", "日线", klines, 20)

	if period.TotalCount != 250 || period.UsedCount != 20 {
		t.Fatalf("counts = %d/%d, want 250/20", period.TotalCount, period.UsedCount)
	}
	if period.RSI6 == nil || math.Abs(*period.RSI6-klines.RSIFloat(6)) > 1e-9 {
		t.Fatalf("RSI6 = %v, want full-series %.12f", period.RSI6, klines.RSIFloat(6))
	}
	if math.Abs(period.Volatility.Atr-klines.ATRFloat(14)) > 1e-9 {
		t.Fatalf("ATR14 = %.12f, want full-series %.12f", period.Volatility.Atr, klines.ATRFloat(14))
	}
	technical := buildAgentTechnicalPeriod("day", "日线", klines)
	technicalRSI, ok := metricValue(technical.RSI["rsi6"])
	if !ok || math.Abs(*period.RSI6-technicalRSI) > 1e-9 {
		t.Fatalf("RSI6 differs from technical-score period: %.12f vs %.12f", *period.RSI6, technicalRSI)
	}
	technicalMA20, ok := metricValue(technical.MA["ma20"])
	if !ok || period.MA20 == nil || math.Abs(*period.MA20-technicalMA20) > 1e-9 {
		t.Fatalf("MA20 differs from technical-score period: %v vs %.12f", period.MA20, technicalMA20)
	}
	if technical.ATR.ATR14 == nil ||
		math.Abs(period.Volatility.Atr-*technical.ATR.ATR14) > 1e-9 {
		t.Fatalf(
			"ATR14 differs from technical-score period: %.12f vs %v",
			period.Volatility.Atr,
			technical.ATR.ATR14,
		)
	}
	if period.OBV.Signal != technical.OBV.Signal {
		t.Fatalf("OBV differs from technical-score period: %q vs %q", period.OBV.Signal, technical.OBV.Signal)
	}
}

func TestBuildAgentKlinePeriodSummaryAddsSecondBatchKlineDetails(t *testing.T) {
	klines := protocol.Klines{
		testSummaryKline("2025-12-23", 91, 93, 90, 92),
		testSummaryKline("2025-12-24", 92, 94, 91, 93),
		testSummaryKline("2025-12-25", 93, 95, 92, 94),
		testSummaryKline("2025-12-26", 94, 96, 93, 95),
		testSummaryKline("2025-12-27", 95, 97, 94, 96),
		testSummaryKline("2025-12-28", 96, 98, 95, 97),
		testSummaryKline("2025-12-29", 97, 99, 96, 98),
		testSummaryKline("2025-12-30", 98, 100, 97, 99),
		testSummaryKline("2025-12-31", 99, 101, 98, 100),
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
	technical := buildAgentTechnicalPeriod(
		"day",
		"日线",
		protocol.Klines(testKlineResp(250).List),
	)
	summary := AgentKlineSummary{
		Code: "603063",
		Name: "禾望电气",
		dayData: agentDayDataContext{
			QueryTime: time.Date(2026, 7, 31, 12, 0, 0, 0, time.Local),
			DataDate:  "2026-07-31",
			Status:    "午间休市动态值，当前日K尚未收盘",
			Adjust:    "qfq",
		},
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
				RSI6:           floatPtr(20.46),
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
				Summary:   "中期趋势保持上行，短线仍需观察量能延续。",
				Signals:   []string{"区间最大回撤较大"},
				Technical: technical,
				YearRange: AgentKlineYearRange{
					Available:         true,
					StartDate:         "2025-07-31",
					EndDate:           "2026-07-31",
					High:              66.80,
					Low:               28.50,
					DistanceToHighPct: -24.93,
					DistanceToLowPct:  75.96,
				},
			},
		},
		bullBear: technicalScoreRow{
			Period: "日线",
			Item:   "多空比",
			Value:  "买/卖 1.25",
			Signal: "主动买入估算占优",
			Score:  1,
		},
	}

	content := buildAgentKlineSummaryText(summary)

	mustContain := []string{
		"股票：禾望电气（603063）",
		"K线摘要：",
		"查询时间：2026-07-31T12:00:00",
		"日线数据日期：2026-07-31",
		"日线数据状态：午间休市动态值，当前日K尚未收盘",
		"日线复权口径：前复权（qfq）",
		"日线：样本120/250",
		"区间2026-01-01至2026-06-24",
		"区间涨跌+12.34%",
		"最大回撤-8.50%",
		"趋势上行",
		"阶段上升趋势中的回调",
		"风险中",
		"近5/20/60日+1.20%/-3.40%/+12.34%",
		"近52周区间 28.50-66.80",
		"距20日高/低-16.42%/+19.40%",
		"区间最大回撤较大",
	}
	if strings.Contains(content, "；摘要：") {
		t.Fatalf("text should not repeat narrative summary:\n%s", content)
	}
	for _, unwanted := range []string{
		"技术指标：", "MACD：", "RSI：", "BOLL：", "KDJ：",
		"BIAS：", "ATR：", "OBV：", "量价：", "多空比：",
	} {
		if strings.Contains(content, unwanted) {
			t.Fatalf("content should not contain %q:\n%s", unwanted, content)
		}
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

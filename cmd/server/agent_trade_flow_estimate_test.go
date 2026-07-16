package main

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/injoyai/tdx/protocol"
)

func TestBuildTradeFlowEstimateSplitsLevelsAndMainNet(t *testing.T) {
	trades := protocol.Trades{
		{Time: tickTime("09:30"), Price: priceYuan(11), Volume: 1000, Status: 1},
		{Time: tickTime("09:31"), Price: priceYuan(10), Volume: 500, Status: 0},
		{Time: tickTime("09:32"), Price: priceYuan(10), Volume: 100, Status: 1},
		{Time: tickTime("09:33"), Price: priceYuan(10), Volume: 20, Status: 0},
		{Time: tickTime("09:34"), Price: priceYuan(10), Volume: 1, Status: 2},
	}

	got := buildTradeFlowEstimate("603063", "2026-06-26", trades)

	if got.Summary.TradeCount != 5 || got.Summary.NeutralCount != 1 {
		t.Fatalf("unexpected counts: %+v", got.Summary)
	}
	assertFloat(t, got.Summary.TotalAmount, 1721000)
	assertFloat(t, got.Summary.MainNetInflow, -600000)
	assertFloat(t, got.Summary.NetInflow, -680000)
	assertFloat(t, got.Levels[0].NetAmount, -1100000)
	assertFloat(t, got.Levels[1].NetAmount, 500000)
	assertFloat(t, got.Levels[2].NetAmount, -100000)
	assertFloat(t, got.Levels[3].NetAmount, 20000)
	if got.Direction.Status0 != "inflow" || got.Direction.Status1 != "outflow" {
		t.Fatalf("unexpected direction mapping: %+v", got.Direction)
	}
	if !strings.Contains(strings.Join(got.Warnings, " "), "60日自适应阈值缓存") {
		t.Fatalf("warning has wrong lookback: %+v", got.Warnings)
	}
}

func TestTradeFlowAdaptiveThresholdsUseCumulativeAmountShare(t *testing.T) {
	if tradeFlowLookbackDays != 60 {
		t.Fatalf("trade flow lookback days = %d, want 60", tradeFlowLookbackDays)
	}

	amounts := []float64{100, 90, 80, 70, 60, 50, 40, 30, 20, 10}

	got := buildTradeFlowAdaptiveThresholds("603063", amounts)

	assertFloat(t, got.SuperLargeAmount, 100)
	assertFloat(t, got.LargeAmount, 90)
	assertFloat(t, got.MediumAmount, 70)
	if got.SampleCount != 10 || got.LookbackDays != tradeFlowLookbackDays {
		t.Fatalf("unexpected cache: %+v", got)
	}
	if got.Method != "historical_tick_amount_cumulative_share" {
		t.Fatalf("unexpected method: %s", got.Method)
	}
	if err := validateTradeFlowThresholdCache(got, "603063"); err != nil {
		t.Fatalf("generated cache is invalid: %v", err)
	}
}

func TestValidateTradeFlowThresholdCacheRejectsWrongLookback(t *testing.T) {
	cache := &TradeFlowThresholdCache{
		Code:         "603063",
		LookbackDays: 200,
		SampleCount:  100,
	}

	if err := validateTradeFlowThresholdCache(cache, "603063"); err == nil {
		t.Fatal("expected mismatched lookback cache to be rejected")
	}
}

func TestBuildTradeFlowEstimateTextIsPlainChineseSummary(t *testing.T) {
	estimate := buildTradeFlowEstimate("603063", "2026-06-26", protocol.Trades{
		{Time: tickTime("09:30"), Price: priceYuan(11), Volume: 1000, Status: 1},
		{Time: tickTime("09:31"), Price: priceYuan(10), Volume: 500, Status: 0},
	})

	text := buildTradeFlowEstimateText(estimate)

	if strings.Contains(text, `"code"`) || strings.Contains(text, "{") {
		t.Fatalf("text should be plain summary, got: %s", text)
	}
	for _, want := range []string{"资金流估算：", "分档资金：", "统计口径："} {
		if !strings.Contains(text, want) {
			t.Fatalf("text missing %q: %s", want, text)
		}
	}
	if !strings.Contains(text, "Status=0计为流入，Status=1计为流出") {
		t.Fatalf("text has unexpected direction mapping: %s", text)
	}
	if !strings.Contains(text, "未找到有效的60日自适应阈值缓存") {
		t.Fatalf("fixed threshold explanation missing: %s", text)
	}

	estimate.ThresholdSource = "adaptive"
	adaptiveText := buildTradeFlowEstimateText(estimate)
	if !strings.Contains(adaptiveText, "基于最近60个交易日") {
		t.Fatalf("adaptive threshold lookback missing: %s", adaptiveText)
	}
	for _, want := range []string{"0%~10%", "10%~30%", "30%~55%", "55%~100%"} {
		if !strings.Contains(adaptiveText, want) {
			t.Fatalf("adaptive text missing range %q: %s", want, adaptiveText)
		}
	}
}

func tickTime(hm string) time.Time {
	t, _ := time.Parse("15:04", hm)
	return t
}

func priceYuan(value int) protocol.Price {
	return protocol.Price(value * 1000)
}

func assertFloat(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.0001 {
		t.Fatalf("got %.4f want %.4f", got, want)
	}
}

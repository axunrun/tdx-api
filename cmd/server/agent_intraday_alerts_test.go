package main

import (
	"strings"
	"testing"

	"github.com/injoyai/tdx/protocol"
)

func TestApplyIntradayMinuteSignalsDetectsRiseAndVolume(t *testing.T) {
	minutes := []protocol.PriceNumber{
		{Time: "09:30", Price: 10000, Number: 10},
		{Time: "09:31", Price: 10100, Number: 10},
		{Time: "09:32", Price: 10200, Number: 10},
		{Time: "09:33", Price: 10500, Number: 100},
		{Time: "09:34", Price: 10600, Number: 100},
	}
	item := AgentIntradayAlertItem{Code: "603063", ChangePct: 5.5}

	applyIntradayMinuteSignals(&item, minutes, 2)

	if item.LatestTime != "09:34" || item.LatestPrice != 10.6 {
		t.Fatalf("unexpected latest: %+v", item)
	}
	if item.RecentChangePct <= 2 || item.RecentVolumeRatio <= 2 {
		t.Fatalf("expected recent rise and volume ratio: %+v", item)
	}
	joined := strings.Join(item.Signals, ",")
	for _, want := range []string{"当日强势", "短时拉升", "短时放量"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("signals missing %s: %+v", want, item.Signals)
		}
	}
}

func TestBuildAgentIntradayAlertsText(t *testing.T) {
	summary := AgentIntradayAlerts{
		WindowMinutes: 30,
		Count:         1,
		Items: []AgentIntradayAlertItem{
			{Text: "禾望电气（603063）最新52.00；信号：短时拉升"},
		},
	}

	text := buildAgentIntradayAlertsText(summary)

	for _, want := range []string{"盘中异动提醒：近30分钟窗口，共1只", "短时拉升"} {
		if !strings.Contains(text, want) {
			t.Fatalf("text missing %q: %s", want, text)
		}
	}
}

func TestIntradayMinuteUnavailableWarningUsesTradingDay(t *testing.T) {
	if got := intradayMinuteUnavailableWarning(false); !strings.Contains(got, "非交易日") {
		t.Fatalf("expected non-trading day warning, got %q", got)
	}
	if got := intradayMinuteUnavailableWarning(true); strings.Contains(got, "非交易日") {
		t.Fatalf("unexpected non-trading day warning, got %q", got)
	}
}

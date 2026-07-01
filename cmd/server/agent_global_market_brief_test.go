package main

import (
	"strings"
	"testing"

	"github.com/injoyai/tdx/protocol"
)

func TestBuildAgentGlobalMarketRangeNeedsPriorClose(t *testing.T) {
	bars := make([]protocol.ExKline, 0, 21)
	for i := 0; i < 21; i++ {
		price := float64(100 + i)
		bars = append(bars, protocol.ExKline{Close: price, High: price + 1, Low: price - 1})
	}

	r := buildAgentGlobalMarketRange(bars, 120, 20)

	if !r.Available {
		t.Fatalf("range should be available: %#v", r)
	}
	if r.High != 121 || r.Low != 100 {
		t.Fatalf("range high/low = %.2f/%.2f, want 121/100", r.High, r.Low)
	}
	if r.ReturnPct != 20 {
		t.Fatalf("return = %.2f, want 20", r.ReturnPct)
	}
}

func TestBuildAgentGlobalMarketBriefTextIncludesRangeAndWarnings(t *testing.T) {
	summary := AgentGlobalMarketBrief{
		Groups: []AgentGlobalMarketGroup{
			{
				Key:  "leader",
				Name: "全球权重股",
				Items: []AgentGlobalMarketItem{
					{
						Code:      "SPCX",
						Name:      "SpaceX",
						Price:     153.23,
						ChangePct: 0.15,
						Range20: AgentGlobalMarketRange{
							Available: false,
							Days:      20,
							Reason:    "日K不足21根",
						},
						Range60: AgentGlobalMarketRange{
							Available: false,
							Days:      60,
							Reason:    "日K不足61根",
						},
						Reason: "商业航天观察",
					},
				},
			},
		},
	}

	text := buildAgentGlobalMarketBriefText(summary)

	for _, want := range []string{"外围权重资产概览", "SpaceX", "近20日不可用", "商业航天观察"} {
		if !strings.Contains(text, want) {
			t.Fatalf("text missing %q: %s", want, text)
		}
	}
}

package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/injoyai/tdx/protocol"
)

func TestBuildAgentTechnicalSummaryFromSpecsKeepsSuccessfulPeriods(t *testing.T) {
	periods, warnings, err := buildAgentTechnicalSummaryFromSpecs("603063", []agentTechnicalSpec{
		{
			period: "day",
			name:   "日线",
			count:  250,
			fetch: func(string, uint16) (*protocol.KlineResp, error) {
				return nil, fmt.Errorf("day failed")
			},
		},
		{
			period: "week",
			name:   "周线",
			count:  156,
			fetch: func(string, uint16) (*protocol.KlineResp, error) {
				return testKlineResp(40), nil
			},
		},
	})

	if err != nil {
		t.Fatalf("buildAgentTechnicalSummaryFromSpecs() error = %v", err)
	}
	if len(periods) != 1 {
		t.Fatalf("period count = %d, want 1", len(periods))
	}
	if periods[0].Period != "week" {
		t.Fatalf("period = %q, want week", periods[0].Period)
	}
	if len(warnings) != 1 || warnings[0] == "" {
		t.Fatalf("warnings = %#v, want one warning", warnings)
	}
}

func testKlineResp(count int) *protocol.KlineResp {
	resp := &protocol.KlineResp{List: make([]*protocol.Kline, 0, count)}
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local)
	for i := 0; i < count; i++ {
		price := protocol.Price(1000 + i)
		resp.List = append(resp.List, &protocol.Kline{
			Time:   start.AddDate(0, 0, i),
			Open:   price,
			Close:  price,
			High:   price + 10,
			Low:    price - 10,
			Volume: 1000,
			Amount: price,
		})
	}
	return resp
}

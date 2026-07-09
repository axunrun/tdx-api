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

func TestBuildOBVUsesCloseDirectionAndWindowVolume(t *testing.T) {
	resp := testKlineResp(22)
	ks := protocol.Klines(resp.List)
	for i, item := range ks {
		item.Close = protocol.Price(1000)
		item.Volume = int64(100 + i)
	}
	ks[1].Close = 1010
	ks[2].Close = 990
	ks[3].Close = 990
	for i := 4; i < len(ks); i++ {
		ks[i].Close = protocol.Price(990 + i)
	}

	obv := buildOBV(ks)

	if !obv.Available {
		t.Fatalf("OBV unavailable: %s", obv.Reason)
	}
	if obv.Latest != 2024 {
		t.Fatalf("latest OBV = %d, want 2024", obv.Latest)
	}
	if obv.Trend != "up" {
		t.Fatalf("trend = %q, want up", obv.Trend)
	}
	if obv.Signal == "" {
		t.Fatalf("signal is empty")
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

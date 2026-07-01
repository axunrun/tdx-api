package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseAgentCodeListDeduplicatesCodes(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/agent/multi-brief?codes=603063,000001&code=603063", nil)

	got := parseAgentCodeList(req)

	if strings.Join(got, ",") != "603063,000001" {
		t.Fatalf("codes = %+v", got)
	}
}

func TestBuildAgentMultiBriefTextIsCompact(t *testing.T) {
	summary := AgentMultiBrief{
		Count: 1,
		Items: []AgentMultiBriefItem{
			{
				Code: "603063",
				Name: "禾望电气",
				Brief: AgentStockBrief{
					Code: "603063",
					Name: "禾望电气",
					Quote: &AgentBriefQuote{
						Price:        10.5,
						ChangePct:    2.3,
						AmountText:   "1.20亿元",
						TurnoverRate: 3.4,
					},
					Stat: &AgentBriefStat{Chg20: 12.3},
					Blocks: []AgentBriefBlock{
						{Name: "风电"},
						{Name: "储能"},
					},
				},
			},
		},
	}

	text := buildAgentMultiBriefText(summary)

	for _, want := range []string{"多股简讯：共1只", "禾望电气（603063）", "涨跌幅+2.30%", "20日+12.30%", "板块：风电、储能"} {
		if !strings.Contains(text, want) {
			t.Fatalf("text missing %q: %s", want, text)
		}
	}
}

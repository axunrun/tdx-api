package main

import (
	"math"
	"testing"
)

func TestNormalizeAgentFinanceBalanceValue(t *testing.T) {
	rawNetAssets := 52199415000.0
	got := normalizeAgentFinanceBalanceValue(rawNetAssets)
	want := 5219941500.0
	if got != want {
		t.Fatalf("normalizeAgentFinanceBalanceValue() = %f, want %f", got, want)
	}
}

func TestNormalizeAgentFinanceIncomeValue(t *testing.T) {
	rawRevenue := 5737742000.0
	got := normalizeAgentFinanceBalanceValue(rawRevenue)
	want := 573774200.0
	if got != want {
		t.Fatalf("normalizeAgentFinanceBalanceValue() = %f, want %f", got, want)
	}
}

func TestEnrichAgentBriefPBUsesNormalizedNetAssets(t *testing.T) {
	quote := &AgentBriefQuote{Price: 50.15}
	finance := &AgentBriefFinance{
		TotalShares: 463708515.625,
		NetAssets:   normalizeAgentFinanceBalanceValue(52199415000),
	}
	stat := &AgentBriefStat{}

	enrichAgentBriefPB(quote, finance, stat, nil)

	if math.Abs(stat.PB-4.46) > 0.02 {
		t.Fatalf("PB = %.4f, want about 4.46", stat.PB)
	}
}

func TestEnrichAgentBriefPBUsesLatestReportNetAssetPerShareFirst(t *testing.T) {
	quote := &AgentBriefQuote{Price: 50.15}
	finance := &AgentBriefFinance{
		TotalShares: 463708515.625,
		NetAssets:   normalizeAgentFinanceBalanceValue(52199415000),
	}
	stat := &AgentBriefStat{}
	latestReport := &AgentBriefLatestReport{NetAssetPerShare: 11.3774}

	enrichAgentBriefPB(quote, finance, stat, latestReport)

	if math.Abs(stat.PB-4.41) > 0.02 {
		t.Fatalf("PB = %.4f, want about 4.41", stat.PB)
	}
}

func TestParseF10LatestReportFields(t *testing.T) {
	content := `│●最新主要指标    │   按06-23股本│    2026-03-31│    2025-12-31│
│每股净资产(元)    │           ---│       11.3774│       10.9616│
│每股经营现金流(元)│           ---│       -0.4555│        1.2345│
│加权净资产收益率% │           ---│        0.9900│       10.5000│
【最新提醒】
┌──────────────┬──────────────┬──────────────┐
│财务同比:2026-03-31 营业收入(万元):57377.42 同比增(%):-25.82；净利润(万元):5112.13 同比增(%):-51.48│`

	date, basis := parseF10LatestReportHeader(content)
	if date != "2026-03-31" || basis != "按06-23股本" {
		t.Fatalf("header = %q, %q", date, basis)
	}
	if got, ok := parseF10LatestTableValue(content, "每股净资产"); !ok || got != 11.3774 {
		t.Fatalf("net asset per share = %f, ok=%v", got, ok)
	}
	if got, ok := parseF10LatestTableValue(content, "每股经营现金流"); !ok || got != -0.4555 {
		t.Fatalf("cashflow per share = %f, ok=%v", got, ok)
	}
	if got, ok := parseF10LatestTableValue(content, "加权净资产收益率"); !ok || got != 0.99 {
		t.Fatalf("weighted roe = %f, ok=%v", got, ok)
	}
	revenue, revenueYoY, netProfit, netProfitYoY, ok := parseF10LatestFinancialYoY(content)
	if !ok || revenue != 573774200 || revenueYoY != -25.82 ||
		netProfit != 51121300 || netProfitYoY != -51.48 {
		t.Fatalf("financial yoy = %f %f %f %f ok=%v",
			revenue,
			revenueYoY,
			netProfit,
			netProfitYoY,
			ok,
		)
	}
}

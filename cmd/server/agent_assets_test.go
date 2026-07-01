package main

import (
	"strings"
	"testing"
)

func TestBuildAgentAssetSearchResultsAddsAgentMetadata(t *testing.T) {
	stocks := []stockRow{
		{Code: "603063", Name: "禾望电气", Exchange: "sh"},
	}

	got := buildAgentAssetSearchResults("禾望", stocks)

	if len(got) != 1 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].Code != "603063" || got[0].Name != "禾望电气" {
		t.Fatalf("unexpected asset: %+v", got[0])
	}
	if got[0].Market != "A股" || got[0].AssetType != "stock" {
		t.Fatalf("unexpected metadata: %+v", got[0])
	}
	if got[0].DisplayName != "禾望电气（603063.SH）" {
		t.Fatalf("display name = %q", got[0].DisplayName)
	}
	if got[0].Blocks == nil {
		t.Fatal("search result should use detail granularity")
	}
}

func TestBuildAgentAssetDetailFromStockIncludesBlocks(t *testing.T) {
	stock := stockRow{Code: "603063", Name: "禾望电气", Exchange: "sh"}
	blocks := []AgentBriefBlock{
		{Type: "concept", TypeName: "概念板块", Name: "风电"},
	}

	got := buildAgentAssetDetailFromStock(stock, blocks)

	if got.DisplayName != "禾望电气（603063.SH）" {
		t.Fatalf("display name = %q", got.DisplayName)
	}
	if len(got.Blocks) != 1 || got.Blocks[0].Name != "风电" {
		t.Fatalf("blocks = %+v", got.Blocks)
	}
	if got.AgentUsage == "" {
		t.Fatal("missing agent usage")
	}
}

func TestBuildAgentAssetSearchTextIncludesDetailFields(t *testing.T) {
	text := buildAgentAssetSearchText(AgentAssetSearchResponse{
		Keyword: "高澜",
		Count:   1,
		Items: []AgentAssetDetail{
			{
				Code:        "300499",
				Name:        "高澜股份",
				DisplayName: "高澜股份（300499.SZ）",
				Blocks: []AgentBriefBlock{
					{Name: "液冷服务"},
					{Name: "数据中心"},
				},
			},
		},
	})

	for _, want := range []string{"资产搜索：关键词“高澜”命中1项", "高澜股份（300499.SZ）", "板块：液冷服务、数据中心"} {
		if !strings.Contains(text, want) {
			t.Fatalf("text missing %q: %s", want, text)
		}
	}
}

func TestBuildAgentAssetSearchTextReportsNoResult(t *testing.T) {
	text := buildAgentAssetSearchText(AgentAssetSearchResponse{
		Keyword:  "不存在股票",
		Count:    0,
		Items:    []AgentAssetDetail{},
		Warnings: []string{"查无此股票：未找到与“不存在股票”匹配的A股资产。"},
	})

	if !strings.Contains(text, "查无此股票") || !strings.Contains(text, "不存在股票") {
		t.Fatalf("unexpected no result text: %s", text)
	}
}

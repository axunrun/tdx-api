package main

import (
	"strings"
	"testing"
)

func TestBuildAgentSectorMembershipGroupsBlocks(t *testing.T) {
	blocks := []AgentBriefBlock{
		{Type: "concept", TypeName: "概念板块", Name: "风电"},
		{Type: "concept", TypeName: "概念板块", Name: "光伏"},
		{Type: "index", TypeName: "指数板块", Name: "上证380"},
	}

	got := buildAgentSectorMembership("603063", "禾望电气", blocks)

	if got.Code != "603063" || got.Name != "禾望电气" {
		t.Fatalf("unexpected identity: %+v", got)
	}
	if got.TotalCount != 3 || len(got.Groups) != 2 {
		t.Fatalf("unexpected groups: %+v", got)
	}
	if got.Groups[0].Type != "concept" || got.Groups[0].Count != 2 {
		t.Fatalf("unexpected first group: %+v", got.Groups[0])
	}
}

func TestBuildAgentSectorMembershipTextIsCompactChinese(t *testing.T) {
	summary := buildAgentSectorMembership("603063", "禾望电气", []AgentBriefBlock{
		{Type: "concept", TypeName: "概念板块", Name: "风电"},
		{Type: "concept", TypeName: "概念板块", Name: "光伏"},
		{Type: "style_region", TypeName: "地域/风格板块", Name: "高融资盘"},
	})

	text := buildAgentSectorMembershipText(summary)

	for _, want := range []string{"板块归属：", "概念板块：风电、光伏。", "地域/风格板块：高融资盘。"} {
		if !strings.Contains(text, want) {
			t.Fatalf("text missing %q: %s", want, text)
		}
	}
	if strings.Contains(text, "{") || strings.Contains(text, `"code"`) {
		t.Fatalf("text should be plain Chinese: %s", text)
	}
}

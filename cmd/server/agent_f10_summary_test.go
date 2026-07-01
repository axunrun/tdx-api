package main

import (
	"strings"
	"testing"
)

func TestAgentF10CategoryPlanAvoidsFinanceOverlap(t *testing.T) {
	if shouldIncludeAgentF10Category("最新提示") {
		t.Fatal("f10-summary should not duplicate stock-brief latest report")
	}
	if shouldIncludeAgentF10Category("财务分析") {
		t.Fatal("f10-summary should not duplicate stock-brief finance fields")
	}
	if shouldIncludeAgentF10Category("公司概况") {
		t.Fatal("f10-summary should drop noisy company profile basics")
	}
	if shouldIncludeAgentF10Category("高管治理") {
		t.Fatal("f10-summary should drop governance biographies and trading noise")
	}
	if shouldIncludeAgentF10Category("公司报道") {
		t.Fatal("f10-summary should leave news-like reports to anysearch")
	}
	if !shouldIncludeAgentF10Category("经营分析") || !shouldIncludeAgentF10Category("股东研究") {
		t.Fatal("f10-summary should keep deep company analysis categories")
	}
}

func TestBuildAgentF10SummaryTextUsesSectionsAndSeparators(t *testing.T) {
	summary := AgentF10Summary{
		Code: "603063",
		Name: "禾望电气",
		Sections: []AgentF10Section{
			{
				Name:    "经营分析",
				Usage:   "主营结构、经营评述和业务变化",
				Excerpt: "产品 | 收入 | 毛利率\n新能源电控 | 10.00亿 | 30.00%",
			},
			{
				Name:    "股东研究",
				Usage:   "股东结构、户数和筹码集中度",
				Excerpt: "股东户数 | 66784",
			},
		},
		Excluded: []AgentF10Excluded{
			{Name: "最新提示", Reason: "由 stock-brief 覆盖"},
		},
	}

	text := buildAgentF10SummaryText(summary)

	if strings.Contains(text, `"code"`) || strings.Contains(text, "{") {
		t.Fatalf("text should be plain summary: %s", text)
	}
	for _, want := range []string{"F10深度资料：", "经营分析：", "产品 | 收入 | 毛利率", "已排除："} {
		if !strings.Contains(text, want) {
			t.Fatalf("text missing %q: %s", want, text)
		}
	}
}

func TestCleanAgentF10ExcerptLimitsNoiseAndKeepsSeparators(t *testing.T) {
	raw := strings.Repeat("─", 80) + "\n" +
		"│资产负债率│45.67%│\n\n" +
		strings.Repeat("每股收益增长。", 100)

	got := cleanAgentF10Excerpt("经营分析", raw, 40)

	if strings.Contains(got, strings.Repeat("─", 10)) {
		t.Fatalf("excerpt should remove border noise: %q", got)
	}
	if len([]rune(got)) > 40 {
		t.Fatalf("excerpt too long: %d %q", len([]rune(got)), got)
	}
	if !strings.Contains(got, "资产负债率 | 45.67%") {
		t.Fatalf("excerpt should keep table columns with separators: %q", got)
	}
}

func TestCleanAgentF10ExcerptDropsResearchReportAndInstitutionSurvey(t *testing.T) {
	raw := strings.Join([]string{
		"研报评级☆",
		"★本栏包括【1.投资评级统计】【2.盈利预测统计】【3.盈利预测明细】【4.研报摘要】",
		"【5.机构调研】",
		"【1.投资评级统计】最新评级日期:2025-09-16",
		"1年内 0 3 0 0 0 3",
		"【2.盈利预测统计】 暂无数据",
		"【3.盈利预测明细】 暂无数据",
		"【4.研报摘要】 暂无数据",
		"【5.机构调研】(近6个月)",
		"Q1.长篇调研问答",
	}, "\n")

	got := cleanAgentF10Excerpt("研报评级", raw, 900)

	for _, want := range []string{"【1.投资评级统计】", "【2.盈利预测统计】", "【3.盈利预测明细】"} {
		if !strings.Contains(got, want) {
			t.Fatalf("excerpt missing %q: %s", want, got)
		}
	}
	for _, dropped := range []string{"【4.研报摘要】", "【5.机构调研】", "Q1.长篇调研问答"} {
		if strings.Contains(got, dropped) {
			t.Fatalf("excerpt should drop %q: %s", dropped, got)
		}
	}
}

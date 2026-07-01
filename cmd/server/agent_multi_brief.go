package main

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/injoyai/tdx"
)

type AgentMultiBrief struct {
	Source   string                `json:"source"`
	Count    int                   `json:"count"`
	Items    []AgentMultiBriefItem `json:"items"`
	Warnings []string              `json:"warnings,omitempty"`
	Note     string                `json:"note"`
}

type AgentMultiBriefItem struct {
	Code  string          `json:"code"`
	Name  string          `json:"name,omitempty"`
	Brief AgentStockBrief `json:"brief"`
}

type AgentMultiBriefText struct {
	Format  string `json:"format"`
	Content string `json:"content"`
}

func handleAgentMultiBrief(w http.ResponseWriter, r *http.Request) {
	summary, ok := loadAgentMultiBrief(w, r)
	if !ok {
		return
	}
	jsonResp(w, summary)
}

func handleAgentMultiBriefText(w http.ResponseWriter, r *http.Request) {
	summary, ok := loadAgentMultiBrief(w, r)
	if !ok {
		return
	}
	jsonResp(w, AgentMultiBriefText{
		Format:  "text/plain; charset=utf-8",
		Content: buildAgentMultiBriefText(summary),
	})
}

func loadAgentMultiBrief(w http.ResponseWriter, r *http.Request) (AgentMultiBrief, bool) {
	codes := parseAgentCodeList(r)
	if len(codes) == 0 {
		jsonErr(w, "缺少codes参数")
		return AgentMultiBrief{}, false
	}
	if len(codes) > 20 {
		codes = codes[:20]
	}
	c := cli()
	if c == nil {
		jsonErr(w, "TDX客户端未连接")
		return AgentMultiBrief{}, false
	}
	return buildAgentMultiBrief(c, codes), true
}

func buildAgentMultiBrief(c *tdx.Client, codes []string) AgentMultiBrief {
	items := make([]AgentMultiBriefItem, 0, len(codes))
	warnings := make([]string, 0)
	for _, code := range codes {
		brief, err := buildAgentStockBrief(c, code, "")
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s失败: %v", code, err))
			continue
		}
		items = append(items, AgentMultiBriefItem{
			Code:  code,
			Name:  brief.Name,
			Brief: brief,
		})
	}
	return AgentMultiBrief{
		Source:   "tdx_agent_multi_brief",
		Count:    len(items),
		Items:    items,
		Warnings: warnings,
		Note:     "多股简讯聚合接口；由请求参数传入股票列表，批量返回每只股票的 stock-brief。",
	}
}

func parseAgentCodeList(r *http.Request) []string {
	seen := make(map[string]struct{})
	codes := make([]string, 0)
	for _, raw := range append(r.URL.Query()["code"], r.URL.Query().Get("codes")) {
		for _, part := range strings.FieldsFunc(raw, func(r rune) bool {
			return r == ',' || r == '，' || r == ' ' || r == '\n' || r == '\t'
		}) {
			code := strings.TrimSpace(part)
			if code == "" {
				continue
			}
			if _, exists := seen[code]; exists {
				continue
			}
			seen[code] = struct{}{}
			codes = append(codes, code)
		}
	}
	return codes
}

func buildAgentMultiBriefText(summary AgentMultiBrief) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("多股简讯：共%d只。\n", summary.Count))
	for i, item := range summary.Items {
		b.WriteString(fmt.Sprintf("%d. %s\n", i+1, multiBriefLine(item.Brief)))
	}
	appendWarningsText(&b, summary.Warnings)
	return strings.TrimSpace(b.String())
}

func multiBriefLine(brief AgentStockBrief) string {
	name := brief.Name
	if name == "" {
		name = brief.Code
	}
	parts := []string{fmt.Sprintf("%s（%s）", name, brief.Code)}
	if brief.Quote != nil {
		parts = append(parts, fmt.Sprintf(
			"现价%.2f，涨跌幅%s，成交额%s",
			brief.Quote.Price,
			formatPercentText(brief.Quote.ChangePct),
			brief.Quote.AmountText,
		))
		if brief.Quote.TurnoverRate > 0 {
			parts = append(parts, "换手率"+formatPercentText(brief.Quote.TurnoverRate))
		}
	}
	if brief.Stat != nil {
		parts = append(parts, "20日"+formatPercentText(brief.Stat.Chg20))
	}
	if len(brief.Blocks) > 0 {
		parts = append(parts, "板块："+multiBriefBlockNames(brief.Blocks, 3))
	}
	if len(brief.Warnings) > 0 {
		parts = append(parts, "提示："+strings.Join(brief.Warnings, "；"))
	}
	return strings.Join(parts, "；")
}

func multiBriefBlockNames(blocks []AgentBriefBlock, limit int) string {
	if limit > len(blocks) {
		limit = len(blocks)
	}
	names := make([]string, 0, limit)
	for _, block := range blocks[:limit] {
		names = append(names, block.Name)
	}
	return strings.Join(names, "、")
}

package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"

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
		jsonErr(w, "最多支持20只股票，请分批调用")
		return AgentMultiBrief{}, false
	}
	c := cli()
	if c == nil {
		jsonErr(w, "TDX客户端未连接")
		return AgentMultiBrief{}, false
	}
	adjust, err := normalizeTechnicalAdjust(r.URL.Query().Get("adjust"))
	if err != nil {
		jsonErr(w, err.Error())
		return AgentMultiBrief{}, false
	}
	return buildAgentMultiBrief(c, codes, adjust), true
}

func buildAgentMultiBrief(c *tdx.Client, codes []string, adjust string) AgentMultiBrief {
	items := make([]AgentMultiBriefItem, 0, len(codes))
	warnings := make([]string, 0)
	now := time.Now()
	for _, code := range codes {
		brief, err := buildAgentStockBriefAt(c, code, "", adjust, now, true)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s失败: %v", code, err))
			continue
		}
		if warning := attachAgentBullBear(c, code, brief.Technical); warning != "" {
			brief.Warnings = append(brief.Warnings, warning)
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
	if technical := firstMultiBriefTechnical(summary.Items); technical != nil {
		context := technical.dayData
		if !context.QueryTime.IsZero() {
			b.WriteString(fmt.Sprintf("查询时间：%s\n", context.QueryTime.Format(time.RFC3339)))
		}
		b.WriteString(fmt.Sprintf(
			"口径：%s；日线指标至少250根K线预热；ATR和OBV只作观察，不直接代表方向。\n",
			agentDayAdjustText(context.Adjust),
		))
	}
	for _, item := range summary.Items {
		brief := item.Brief
		name := valueOrDash(brief.Name)
		b.WriteString(fmt.Sprintf("\n【%s %s】\n", brief.Code, name))
		b.WriteString(multiBriefQuoteCardText(brief) + "\n")
		if value, ok := multiBriefReturn20(brief.Technical); ok {
			b.WriteString("周期：近20日" + formatPercentText(value) + "\n")
		}
		if len(brief.Blocks) > 0 {
			b.WriteString("板块：" + multiBriefBlockNames(brief.Blocks, 3) + "\n")
		}
		b.WriteString(multiBriefDataCardText(brief) + "\n")
		for _, line := range multiBriefTechnicalCardLines(brief.Technical) {
			b.WriteString(line + "\n")
		}
		if len(brief.Warnings) > 0 {
			b.WriteString("提示：" + strings.Join(brief.Warnings, "；") + "\n")
		}
	}
	appendWarningsText(&b, summary.Warnings)
	return strings.TrimSpace(b.String())
}

func firstMultiBriefTechnical(items []AgentMultiBriefItem) *AgentTechnicalSummary {
	for _, item := range items {
		if item.Brief.Technical != nil {
			return item.Brief.Technical
		}
	}
	return nil
}

func multiBriefQuoteCardText(brief AgentStockBrief) string {
	if brief.Quote == nil {
		return "行情：不可用"
	}
	priceLabel := "现价"
	if strings.Contains(brief.Quote.DataStatus, "最近完整交易日") {
		priceLabel = "最近收盘价"
	}
	parts := []string{
		fmt.Sprintf("%s%.2f", priceLabel, brief.Quote.Price),
		"涨跌幅" + formatPercentText(brief.Quote.ChangePct),
		"成交额" + brief.Quote.AmountText,
	}
	if brief.Quote.TurnoverRate > 0 {
		parts = append(parts, "换手率"+formatPercentText(brief.Quote.TurnoverRate))
	}
	return "行情：" + strings.Join(parts, "；")
}

func multiBriefDataCardText(brief AgentStockBrief) string {
	parts := make([]string, 0, 4)
	if brief.Quote != nil {
		parts = append(parts,
			"行情日期"+valueOrDash(brief.Quote.DataDate),
			valueOrDash(brief.Quote.DataStatus),
		)
	}
	if brief.Technical != nil {
		context := brief.Technical.dayData
		parts = append(parts,
			"技术日期"+valueOrDash(context.DataDate),
			valueOrDash(context.Status),
		)
	}
	if len(parts) == 0 {
		return "数据：不可用"
	}
	return "数据：" + strings.Join(parts, "；")
}

func multiBriefTechnicalCardLines(summary *AgentTechnicalSummary) []string {
	if summary == nil {
		return []string{"技术指标：不可用"}
	}
	var day *AgentTechnicalPeriod
	for i := range summary.Periods {
		if summary.Periods[i].Period == "day" {
			day = &summary.Periods[i]
			break
		}
	}
	if day == nil {
		return []string{"技术指标：不可用"}
	}
	rows := scoreTechnicalPeriod(*day, nil)
	rows = append(rows, summary.bullBear)
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		value := valueOrDash(row.Value)
		line := row.Item + "：" + value
		signal := strings.TrimSpace(row.Signal)
		if row.Item == "ATR" && day.ATR.Available {
			signal = ""
		}
		if row.Item == "OBV" && day.OBV.Available {
			signal = strings.TrimSpace(strings.TrimPrefix(row.Signal, "OBV："))
			if index := strings.Index(signal, "（"); index >= 0 {
				signal = strings.TrimSpace(signal[:index])
			}
			signal = strings.TrimRight(signal, "。； ")
		}
		if signal != "" && signal != value {
			line += "；" + signal
		}
		lines = append(lines, line)
	}
	return lines
}

func multiBriefReturn20(summary *AgentTechnicalSummary) (float64, bool) {
	if summary == nil {
		return 0, false
	}
	for _, period := range summary.Periods {
		if period.Period == "day" {
			return period.Return20, true
		}
	}
	return 0, false
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

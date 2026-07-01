package main

import (
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/injoyai/tdx"
)

const agentF10SectionExcerptLimit = 900

type AgentF10Summary struct {
	Code     string             `json:"code"`
	Name     string             `json:"name,omitempty"`
	Source   string             `json:"source"`
	Sections []AgentF10Section  `json:"sections"`
	Excluded []AgentF10Excluded `json:"excluded"`
	Limits   map[string]int     `json:"limits"`
	Note     string             `json:"note"`
	Warnings []string           `json:"warnings,omitempty"`
}

type AgentF10Section struct {
	Index   int    `json:"index"`
	Name    string `json:"name"`
	Usage   string `json:"usage"`
	Excerpt string `json:"excerpt"`
	Length  int    `json:"length"`
}

type AgentF10Excluded struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

type AgentF10SummaryText struct {
	Code    string `json:"code"`
	Format  string `json:"format"`
	Content string `json:"content"`
}

func handleAgentF10Summary(w http.ResponseWriter, r *http.Request) {
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if code == "" {
		jsonErr(w, "缺少code")
		return
	}
	c := cli()
	if c == nil {
		jsonErr(w, "TDX客户端未连接")
		return
	}

	summary, err := buildAgentF10Summary(c, code, r.URL.Query().Get("mkt"))
	if err != nil {
		jsonErr(w, err.Error())
		return
	}
	jsonResp(w, summary)
}

func handleAgentF10SummaryText(w http.ResponseWriter, r *http.Request) {
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if code == "" {
		jsonErr(w, "缺少code")
		return
	}
	c := cli()
	if c == nil {
		jsonErr(w, "TDX客户端未连接")
		return
	}

	summary, err := buildAgentF10Summary(c, code, r.URL.Query().Get("mkt"))
	if err != nil {
		jsonErr(w, err.Error())
		return
	}
	jsonResp(w, AgentF10SummaryText{
		Code:    code,
		Format:  "text/plain; charset=utf-8",
		Content: buildAgentF10SummaryText(summary),
	})
}

func buildAgentF10Summary(c *tdx.Client, code, rawMarket string) (AgentF10Summary, error) {
	exchange := exchangeForCode(code, rawMarket)
	categories, err := c.GetCompanyCategory(exchange, code)
	if err != nil {
		return AgentF10Summary{}, err
	}
	summary := AgentF10Summary{
		Code:     code,
		Name:     queryStockName(code),
		Source:   "tdx_agent_f10_summary",
		Sections: make([]AgentF10Section, 0),
		Excluded: make([]AgentF10Excluded, 0),
		Limits: map[string]int{
			"sectionExcerptChars": agentF10SectionExcerptLimit,
		},
		Note: "F10低频深度资料补充接口；不重复stock-brief已覆盖的最新提示和财务字段，保留适合个股深度分析的分类化裁剪摘要。",
	}

	warnings := make([]string, 0)
	for i, category := range categories {
		name := strings.TrimSpace(category.Name)
		if !shouldIncludeAgentF10Category(name) {
			summary.Excluded = append(summary.Excluded, AgentF10Excluded{
				Name:   name,
				Reason: excludedAgentF10Reason(name),
			})
			continue
		}
		content, err := c.GetCompanyContent(exchange, code, category.Filename, category.Start, category.Length)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s失败: %v", name, err))
			continue
		}
		excerpt := cleanAgentF10Excerpt(name, content, agentF10SectionExcerptLimit)
		if excerpt == "" {
			warnings = append(warnings, name+"为空")
			continue
		}
		summary.Sections = append(summary.Sections, AgentF10Section{
			Index:   i,
			Name:    name,
			Usage:   agentF10CategoryUsage(name),
			Excerpt: excerpt,
			Length:  utf8.RuneCountInString(excerpt),
		})
	}
	if len(summary.Sections) == 0 {
		return summary, fmt.Errorf("无可用F10深度资料")
	}
	summary.Warnings = warnings
	return summary, nil
}

func shouldIncludeAgentF10Category(name string) bool {
	switch strings.TrimSpace(name) {
	case "最新提示", "财务分析", "公司概况", "高管治理", "公司报道":
		return false
	default:
		return true
	}
}

func excludedAgentF10Reason(name string) string {
	switch strings.TrimSpace(name) {
	case "最新提示":
		return "由 stock-brief 覆盖最新报告期、财务同比和关键提醒"
	case "财务分析":
		return "由 stock-brief 覆盖结构化财务字段；避免重复输出财务表格"
	case "公司概况":
		return "电话、网址、地址、经营范围等基础资料噪音较高，快速分析价值低"
	case "高管治理":
		return "董监高履历和高管交易明细噪音较高，暂不进入Agent默认上下文"
	case "公司报道":
		return "新闻报道类信息交由anysearch补齐，避免使用滞后F10报道"
	default:
		return "不适合当前F10深度补充接口"
	}
}

func agentF10CategoryUsage(name string) string {
	switch strings.TrimSpace(name) {
	case "公司概况":
		return "公司基础资料、主营、行业和控股关系"
	case "股本结构":
		return "总股本、流通股、限售解禁和股本变化"
	case "股东研究":
		return "股东结构、户数变化和筹码集中度"
	case "机构持股":
		return "机构持仓结构和季度变化"
	case "分红融资":
		return "分红、融资、股息和再融资记录"
	case "高管治理":
		return "董监高、薪酬、持股和治理线索"
	case "资金动向":
		return "大宗交易、龙虎榜、融资融券等资金线索"
	case "资本运作":
		return "并购、定增、重组和投资事项"
	case "热点题材":
		return "题材标签和业务关联逻辑"
	case "公司公告":
		return "近期公告标题、日期和事件类型"
	case "公司报道":
		return "F10内置报道摘要，时效性弱于anysearch"
	case "经营分析":
		return "主营结构、产品地区收入、毛利率和经营评述"
	case "行业分析":
		return "行业地位、竞争格局和景气描述"
	case "研报评级":
		return "评级、目标价和机构观点摘要"
	default:
		return "F10补充资料"
	}
}

func buildAgentF10SummaryText(summary AgentF10Summary) string {
	var b strings.Builder
	if summary.Name != "" {
		b.WriteString(fmt.Sprintf("股票：%s（%s）\n\n", summary.Name, summary.Code))
	} else {
		b.WriteString(fmt.Sprintf("股票代码：%s\n\n", summary.Code))
	}
	b.WriteString("F10深度资料：\n")
	b.WriteString("本接口补充低频深度资料，不重复 stock-brief 的最新提示和财务字段。\n\n")
	for _, section := range summary.Sections {
		if section.Excerpt == "" {
			continue
		}
		b.WriteString(fmt.Sprintf("%s：\n", section.Name))
		if section.Usage != "" {
			b.WriteString(section.Usage)
			b.WriteString("。\n")
		}
		b.WriteString(section.Excerpt)
		b.WriteString("\n\n")
	}
	if len(summary.Excluded) > 0 {
		b.WriteString("已排除：\n")
		for _, item := range summary.Excluded {
			b.WriteString(fmt.Sprintf("- %s：%s。\n", item.Name, item.Reason))
		}
	}
	appendWarningsText(&b, summary.Warnings)
	return strings.TrimSpace(b.String())
}

func cleanAgentF10Excerpt(categoryName, content string, limit int) string {
	lines := make([]string, 0)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || isF10BorderLine(line) {
			continue
		}
		if shouldStopAgentF10Excerpt(categoryName, line) {
			break
		}
		line = cleanAgentF10SectionLine(categoryName, line)
		if line == "" {
			continue
		}
		lines = append(lines, formatF10DataLine(line))
	}
	return trimRunes(strings.Join(lines, "\n"), limit)
}

func shouldStopAgentF10Excerpt(categoryName, line string) bool {
	return strings.TrimSpace(categoryName) == "研报评级" &&
		strings.HasPrefix(line, "【4.研报摘要】")
}

func cleanAgentF10SectionLine(categoryName, line string) string {
	if strings.TrimSpace(categoryName) != "研报评级" {
		return line
	}
	line = strings.ReplaceAll(line, "【4.研报摘要】", "")
	line = strings.ReplaceAll(line, "【5.机构调研】", "")
	return strings.TrimSpace(line)
}

func formatF10DataLine(line string) string {
	if !strings.Contains(line, "│") {
		return strings.Join(strings.Fields(line), " ")
	}
	parts := make([]string, 0)
	for _, part := range strings.Split(line, "│") {
		part = strings.Join(strings.Fields(strings.TrimSpace(part)), " ")
		if part != "" {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, " | ")
}

func isF10BorderLine(line string) bool {
	return strings.Trim(line, "─━┄┅┈┉═┌┐└┘├┤┬┴┼+-= \t\r") == ""
}

func trimRunes(text string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit])
}

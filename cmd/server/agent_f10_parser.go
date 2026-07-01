package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/injoyai/tdx"
)

type AgentBriefLatestReport struct {
	ReportDate                string  `json:"reportDate,omitempty"`
	Basis                     string  `json:"basis,omitempty"`
	NetAssetPerShare          float64 `json:"netAssetPerShare,omitempty"`
	OperatingCashflowPerShare float64 `json:"operatingCashflowPerShare,omitempty"`
	WeightedROE               float64 `json:"weightedRoe,omitempty"`
	Revenue                   float64 `json:"revenue,omitempty"`
	RevenueText               string  `json:"revenueText,omitempty"`
	RevenueYoY                float64 `json:"revenueYoY,omitempty"`
	NetProfit                 float64 `json:"netProfit,omitempty"`
	NetProfitText             string  `json:"netProfitText,omitempty"`
	NetProfitYoY              float64 `json:"netProfitYoY,omitempty"`
	Source                    string  `json:"source"`
	Meaning                   string  `json:"meaning"`
}

func buildAgentBriefLatestReport(c *tdx.Client, code, rawMarket string) (*AgentBriefLatestReport, error) {
	exchange := exchangeForCode(code, rawMarket)
	categories, err := c.GetCompanyCategory(exchange, code)
	if err != nil {
		return nil, err
	}
	if len(categories) == 0 {
		return nil, fmt.Errorf("无F10分类")
	}

	category := categories[0]
	for _, item := range categories {
		if strings.Contains(item.Name, "最新提示") {
			category = item
			break
		}
	}

	content, err := c.GetCompanyContent(exchange, code, category.Filename, category.Start, category.Length)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("F10最新提示为空")
	}
	return parseAgentBriefLatestReport(content)
}

func parseAgentBriefLatestReport(content string) (*AgentBriefLatestReport, error) {
	report := &AgentBriefLatestReport{
		Source:  "f10_cat0_latest_prompt",
		Meaning: "来自F10最新提示，保留最新一期关键财报字段；用于速览中的财报时点、每股净资产和同比变化判断。",
	}
	report.ReportDate, report.Basis = parseF10LatestReportHeader(content)
	report.NetAssetPerShare, _ = parseF10LatestTableValue(content, "每股净资产")
	report.OperatingCashflowPerShare, _ = parseF10LatestTableValue(content, "每股经营现金流")
	report.WeightedROE, _ = parseF10LatestTableValue(content, "加权净资产收益率")

	revenue, revenueYoY, netProfit, netProfitYoY, ok := parseF10LatestFinancialYoY(content)
	if ok {
		report.Revenue = revenue
		report.RevenueText = formatCNYText(revenue)
		report.RevenueYoY = revenueYoY
		report.NetProfit = netProfit
		report.NetProfitText = formatCNYText(netProfit)
		report.NetProfitYoY = netProfitYoY
	}

	if report.ReportDate == "" &&
		report.NetAssetPerShare == 0 &&
		report.Revenue == 0 &&
		report.NetProfit == 0 {
		return nil, fmt.Errorf("未解析到最新财报字段")
	}
	return report, nil
}

func parseF10LatestReportHeader(content string) (string, string) {
	dateRegexp := regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)
	for _, line := range strings.Split(content, "\n") {
		if !strings.Contains(line, "最新主要指标") {
			continue
		}
		basis := ""
		for _, cell := range strings.Split(line, "│") {
			cell = strings.TrimSpace(cell)
			if strings.Contains(cell, "按") && strings.Contains(cell, "股本") {
				basis = cell
			}
			if date := dateRegexp.FindString(cell); date != "" {
				return date, basis
			}
		}
		return "", basis
	}
	return "", ""
}

func parseF10LatestTableValue(content, label string) (float64, bool) {
	for _, line := range strings.Split(content, "\n") {
		if !strings.Contains(line, label) {
			continue
		}
		for _, cell := range strings.Split(line, "│") {
			cell = strings.TrimSpace(cell)
			if cell == "" || strings.Contains(cell, label) || strings.Contains(cell, "---") {
				continue
			}
			if value, ok := parseFirstFloat(cell); ok {
				return value, true
			}
		}
	}
	return 0, false
}

func parseF10LatestFinancialYoY(content string) (float64, float64, float64, float64, bool) {
	re := regexp.MustCompile(
		`财务同比:\d{4}-\d{2}-\d{2}\s+营业收入\(万元\):([-+]?\d+(?:\.\d+)?)\s+同比增\(%\):([-+]?\d+(?:\.\d+)?).*?净利润\(万元\):([-+]?\d+(?:\.\d+)?)\s+同比增\(%\):([-+]?\d+(?:\.\d+)?)`,
	)
	match := re.FindStringSubmatch(content)
	if len(match) != 5 {
		return 0, 0, 0, 0, false
	}
	revenueWan, err1 := strconv.ParseFloat(match[1], 64)
	revenueYoY, err2 := strconv.ParseFloat(match[2], 64)
	netProfitWan, err3 := strconv.ParseFloat(match[3], 64)
	netProfitYoY, err4 := strconv.ParseFloat(match[4], 64)
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		return 0, 0, 0, 0, false
	}
	return revenueWan * 10000, revenueYoY, netProfitWan * 10000, netProfitYoY, true
}

func parseFirstFloat(text string) (float64, bool) {
	re := regexp.MustCompile(`[-+]?\d+(?:\.\d+)?`)
	raw := re.FindString(text)
	if raw == "" {
		return 0, false
	}
	value, err := strconv.ParseFloat(raw, 64)
	return value, err == nil
}

package main

import (
	"fmt"
	"sort"
	"strings"
)

type AgentStockBriefText struct {
	Code    string `json:"code"`
	Format  string `json:"format"`
	Content string `json:"content"`
}

func buildAgentStockBriefText(brief AgentStockBrief) string {
	var b strings.Builder

	if brief.Name != "" {
		b.WriteString(fmt.Sprintf("股票：%s（%s）\n\n", brief.Name, brief.Code))
	} else {
		b.WriteString(fmt.Sprintf("股票代码：%s\n\n", brief.Code))
	}
	appendQuoteText(&b, brief.Quote, brief.Moneyflow)
	appendFinanceText(&b, brief.Finance)
	appendLatestReportText(&b, brief.LatestReport)
	appendBlocksText(&b, brief.Blocks)
	appendStatText(&b, brief.Stat, brief.Moneyflow)
	appendTechnicalText(&b, brief.Technical)
	appendWarningsText(&b, brief.Warnings)

	return strings.TrimSpace(b.String())
}

func appendQuoteText(b *strings.Builder, quote *AgentBriefQuote, moneyflow *AgentBriefMoneyflow) {
	if quote == nil {
		return
	}
	b.WriteString("行情摘要：\n")
	b.WriteString(fmt.Sprintf(
		"%s，当前价格 %.2f 元，涨跌幅 %s。日内区间 %.2f-%.2f 元，振幅 %s，开盘 %.2f 元，昨收 %.2f 元，成交额 %s，成交量 %d 手",
		quote.Market,
		quote.Price,
		formatPercentText(quote.ChangePct),
		quote.Low,
		quote.High,
		formatPercentText(quote.AmplitudePct),
		quote.Open,
		quote.LastClose,
		quote.AmountText,
		quote.Volume,
	))
	if quote.TurnoverRate > 0 {
		b.WriteString(fmt.Sprintf("，换手率 %s", formatPercentText(quote.TurnoverRate)))
	}
	if moneyflow != nil && moneyflow.AmountChangeText != "" {
		b.WriteString(fmt.Sprintf(
			"，成交额较昨日%s（%s）",
			formatSignedCNYText(moneyflow.AmountChangeText, moneyflow.Amount-moneyflow.AmountPrev),
			formatPercentText(moneyflow.AmountChangePct),
		))
	}
	b.WriteString("。\n\n")
}

func appendFinanceText(b *strings.Builder, finance *AgentBriefFinance) {
	if finance == nil {
		return
	}
	b.WriteString("基本面摘要：\n")
	if finance.IPODate != "" {
		b.WriteString(fmt.Sprintf("上市日期：%s。\n", finance.IPODate))
	}
	b.WriteString(fmt.Sprintf(
		"总股本 %s，流通股本 %s，总市值 %s，流通市值 %s，总资产 %s，净资产 %s，主营收入 %s，主营利润 %s，营业利润 %s，净利润 %s，经营现金流 %s，股东人数 %.0f。\n\n",
		valueOrDash(finance.TotalSharesText),
		valueOrDash(finance.FloatSharesText),
		valueOrDash(finance.TotalMarketValueText),
		valueOrDash(finance.FloatMarketValueText),
		valueOrDash(finance.TotalAssetsText),
		valueOrDash(finance.NetAssetsText),
		valueOrDash(finance.MainRevenueText),
		valueOrDash(finance.MainProfitText),
		valueOrDash(finance.OperatingProfitText),
		valueOrDash(finance.NetProfitText),
		valueOrDash(finance.OperatingCashflowText),
		finance.Shareholders,
	))
}

func appendLatestReportText(b *strings.Builder, report *AgentBriefLatestReport) {
	if report == nil {
		return
	}
	parts := make([]string, 0)
	if report.ReportDate != "" {
		parts = append(parts, fmt.Sprintf("报告期 %s", report.ReportDate))
	}
	if report.Basis != "" {
		parts = append(parts, report.Basis)
	}
	if report.NetAssetPerShare != 0 {
		parts = append(parts, fmt.Sprintf("每股净资产 %.4f 元", report.NetAssetPerShare))
	}
	if report.OperatingCashflowPerShare != 0 {
		parts = append(parts, fmt.Sprintf("每股经营现金流 %.4f 元", report.OperatingCashflowPerShare))
	}
	if report.WeightedROE != 0 {
		parts = append(parts, fmt.Sprintf("加权ROE %s", formatPercentText(report.WeightedROE)))
	}
	if report.RevenueText != "" {
		parts = append(parts, fmt.Sprintf("营业收入 %s，同比 %s",
			report.RevenueText,
			formatPercentText(report.RevenueYoY),
		))
	}
	if report.NetProfitText != "" {
		parts = append(parts, fmt.Sprintf("净利润 %s，同比 %s",
			report.NetProfitText,
			formatPercentText(report.NetProfitYoY),
		))
	}
	if len(parts) == 0 {
		return
	}
	b.WriteString("最新财报提示：\n")
	b.WriteString(strings.Join(parts, "；"))
	b.WriteString("。\n\n")
}

func appendBlocksText(b *strings.Builder, blocks []AgentBriefBlock) {
	if len(blocks) == 0 {
		return
	}
	grouped := groupBlockNames(blocks)
	order := []string{"concept", "style_region", "index"}
	names := map[string]string{
		"concept":      "概念板块",
		"style_region": "地域/风格板块",
		"index":        "指数板块",
	}

	b.WriteString("所属板块：\n")
	for _, typ := range order {
		items := grouped[typ]
		if len(items) == 0 {
			continue
		}
		b.WriteString(fmt.Sprintf("%s：%s。\n", names[typ], strings.Join(items, "、")))
	}
	b.WriteString("\n")
}

func appendStatText(b *strings.Builder, stat *AgentBriefStat, moneyflow *AgentBriefMoneyflow) {
	if stat == nil && moneyflow == nil {
		return
	}
	b.WriteString("估值与表现：\n")
	if stat != nil {
		b.WriteString(fmt.Sprintf(
			"PE_TTM %.2f，静态PE %.2f，市净率PB %s，股息率 %s。阶段涨跌幅：近5日 %s，近20日 %s，近60日 %s，今年以来 %s。\n",
			stat.PETTM,
			stat.PEStatic,
			formatOptionalRatioText(stat.PB),
			formatPercentText(stat.DivYield),
			formatPercentText(stat.Chg5),
			formatPercentText(stat.Chg20),
			formatPercentText(stat.Chg60),
			formatPercentText(stat.ChgYTD),
		))
	}
	if moneyflow != nil && (moneyflow.Low52W != 0 || moneyflow.High52W != 0) {
		b.WriteString(fmt.Sprintf("52周价格区间：%.2f-%.2f 元。\n", moneyflow.Low52W, moneyflow.High52W))
	}
	b.WriteString("\n")
}

func appendTechnicalText(b *strings.Builder, summary *AgentTechnicalSummary) {
	if summary == nil || len(summary.Periods) == 0 {
		return
	}
	b.WriteString("技术指标：\n")
	for _, period := range summary.Periods {
		parts := []string{fmt.Sprintf("%s：收盘%.2f", period.Name, period.Close)}
		if ma20, ok := metricValue(period.MA["ma20"]); ok {
			parts = append(parts, fmt.Sprintf("MA20=%.2f", ma20))
		}
		if ma60, ok := metricValue(period.MA["ma60"]); ok {
			parts = append(parts, fmt.Sprintf("MA60=%.2f", ma60))
		}
		if period.MACD.Available {
			if period.MACD.Hist != nil {
				parts = append(parts, fmt.Sprintf("MACD柱=%.2f", *period.MACD.Hist))
			}
			if period.MACD.Signal != "" {
				parts = append(parts, period.MACD.Signal)
			}
		}
		if rsi6, ok := metricValue(period.RSI["rsi6"]); ok {
			parts = append(parts, fmt.Sprintf("RSI6=%.2f", rsi6))
		}
		if period.BOLL.Available && period.BOLL.Position != "" {
			parts = append(parts, "布林线："+period.BOLL.Position)
		}
		if period.ATR.Available && period.ATR.ATR14 != nil {
			parts = append(parts, fmt.Sprintf("ATR14=%.2f", *period.ATR.ATR14))
		}
		b.WriteString(strings.Join(parts, "；"))
		b.WriteString("。\n")
	}
	b.WriteString("\n")
}

func appendWarningsText(b *strings.Builder, warnings []string) {
	if len(warnings) == 0 {
		return
	}
	b.WriteString("数据提示：\n")
	for _, warning := range warnings {
		if strings.TrimSpace(warning) == "" {
			continue
		}
		b.WriteString("- ")
		b.WriteString(warning)
		b.WriteString("\n")
	}
}

func groupBlockNames(blocks []AgentBriefBlock) map[string][]string {
	grouped := make(map[string][]string)
	seen := make(map[string]bool)
	for _, block := range blocks {
		if block.Name == "" {
			continue
		}
		key := block.Type + ":" + block.Name
		if seen[key] {
			continue
		}
		seen[key] = true
		grouped[block.Type] = append(grouped[block.Type], block.Name)
	}
	for typ := range grouped {
		sort.Strings(grouped[typ])
	}
	return grouped
}

func formatPercentText(value float64) string {
	sign := ""
	if value > 0 {
		sign = "+"
	}
	return fmt.Sprintf("%s%.2f%%", sign, value)
}

func formatSignedCNYText(text string, value float64) string {
	if strings.TrimSpace(text) == "" {
		return "-"
	}
	if value > 0 {
		return "增加" + text
	}
	if value < 0 {
		return "减少" + strings.TrimPrefix(text, "-")
	}
	return "持平"
}

func formatOptionalRatioText(value float64) string {
	if value == 0 {
		return "-"
	}
	return fmt.Sprintf("%.2f", value)
}

func valueOrDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

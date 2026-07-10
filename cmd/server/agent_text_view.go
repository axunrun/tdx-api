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
	appendValuationDisciplineText(&b, brief.Quote, brief.Finance, brief.Stat, brief.LatestReport)
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

func appendValuationDisciplineText(
	b *strings.Builder,
	quote *AgentBriefQuote,
	finance *AgentBriefFinance,
	stat *AgentBriefStat,
	report *AgentBriefLatestReport,
) {
	consistency := marketCapConsistencyText(quote, finance)
	signals := valuationQualitySignals(finance, stat, report)
	if consistency == "" && len(signals) == 0 {
		return
	}
	if consistency != "" {
		b.WriteString("估值与数据一致性：\n")
		b.WriteString("- ")
		b.WriteString(consistency)
		b.WriteString("\n")
	}
	b.WriteString("估值与质量提示：\n")
	if len(signals) == 0 {
		b.WriteString("- 未发现显著估值/利润质量异常。\n\n")
		return
	}
	for _, signal := range signals {
		b.WriteString("- ")
		b.WriteString(signal)
		b.WriteString("\n")
	}
	b.WriteString("\n")
}

func marketCapConsistencyText(quote *AgentBriefQuote, finance *AgentBriefFinance) string {
	if quote == nil || finance == nil ||
		quote.Price <= 0 || finance.TotalShares <= 0 || finance.TotalMarketValue <= 0 {
		return ""
	}
	calculated := quote.Price * finance.TotalShares
	deviation := absPct(calculated, finance.TotalMarketValue)
	level := "口径一致"
	switch {
	case deviation > 5:
		level = "明显偏差，需检查价格、股本或单位"
	case deviation > 1:
		level = "小幅偏差，可能存在交易日或股本变动时点差异"
	}
	return fmt.Sprintf(
		"市值一致性：当前价 × 总股本 = %s，与接口总市值 %s偏差 %.2f%%，%s。",
		formatCNYText(calculated),
		valueOrDash(finance.TotalMarketValueText),
		deviation,
		level,
	)
}

func valuationQualitySignals(
	finance *AgentBriefFinance,
	stat *AgentBriefStat,
	report *AgentBriefLatestReport,
) []string {
	signals := make([]string, 0, 4)
	add := func(signal string) {
		if len(signals) < 4 {
			signals = append(signals, signal)
		}
	}
	netProfit := 0.0
	operatingCashflow := 0.0
	if finance != nil {
		netProfit = finance.NetProfit
		operatingCashflow = finance.OperatingCashflow
	}
	netProfitYoY := 0.0
	revenueYoY := 0.0
	roe := 0.0
	if report != nil {
		netProfitYoY = report.NetProfitYoY
		revenueYoY = report.RevenueYoY
		roe = report.WeightedROE
	}
	if stat != nil {
		if stat.PETTM > 40 && netProfitYoY < 0 {
			add("高估值叠加利润下滑，估值压力偏大。")
		}
		if stat.PETTM < 0 || netProfit < 0 {
			add("盈利为负，PE 指标可能失真。")
		}
		if stat.PB > 5 && roe > 0 && roe < 8 {
			add("PB 偏高但 ROE 不足，估值质量不匹配。")
		}
		if stat.PB > 0 && stat.PB < 2 && roe > 15 {
			add("ROE/PB 组合较优，但需验证可持续性。")
		}
	}
	if netProfit > 0 && operatingCashflow < 0 {
		add("盈利为正但经营现金流为负，利润质量需关注。")
	}
	if netProfit > 0 && operatingCashflow/netProfit < 0.5 {
		add("经营现金流覆盖净利润不足。")
	}
	if report != nil && revenueYoY-netProfitYoY > 20 {
		add("净利润同比显著弱于营收同比，需关注毛利率、费用或减值。")
	}
	return signals
}

func appendTechnicalText(b *strings.Builder, summary *AgentTechnicalSummary) {
	if summary == nil || len(summary.Periods) == 0 {
		return
	}
	b.WriteString("技术指标：\n")
	b.WriteString("说明：RSI反映价格动量，OBV反映量价确认；二者不一致时表示价格动量与成交量确认存在背离，不是数据冲突。\n")
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
		if period.OBV.Available && period.OBV.Signal != "" {
			parts = append(parts, period.OBV.Signal)
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

func absPct(calculated, reported float64) float64 {
	if reported == 0 {
		return 0
	}
	deviation := (calculated - reported) / reported * 100
	if deviation < 0 {
		return -deviation
	}
	return deviation
}

package main

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/injoyai/tdx"
	"github.com/injoyai/tdx/protocol"
)

type AgentCalculationText struct {
	Code    string `json:"code"`
	Format  string `json:"format"`
	Content string `json:"content"`
}

type scenarioRow struct {
	Name            string
	GrowthPct       float64
	PE              float64
	TargetEPS       float64
	TargetPrice     float64
	ChangePct       float64
	AnnualReturnPct float64
}

type technicalScoreRow struct {
	Period string
	Item   string
	Value  string
	Signal string
	Score  int
}

func handleAgentScenarioValuationText(w http.ResponseWriter, r *http.Request) {
	brief, err := calculationBriefFromRequest(r)
	if err != nil {
		jsonErr(w, err.Error())
		return
	}
	content := buildScenarioValuationText(brief, r.URL.Query())
	jsonResp(w, AgentCalculationText{
		Code:    brief.Code,
		Format:  "text/plain; charset=utf-8",
		Content: content,
	})
}

func handleAgentImpliedExpectationText(w http.ResponseWriter, r *http.Request) {
	brief, err := calculationBriefFromRequest(r)
	if err != nil {
		jsonErr(w, err.Error())
		return
	}
	content := buildImpliedExpectationText(brief, r.URL.Query())
	jsonResp(w, AgentCalculationText{
		Code:    brief.Code,
		Format:  "text/plain; charset=utf-8",
		Content: content,
	})
}

func handleAgentTechnicalScoreText(w http.ResponseWriter, r *http.Request) {
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
	dayCount := queryInt(r, "dayCount", 250, 60, 500)
	includeWeeklyMonthly := queryBool(r, "includeWeeklyMonthly", true)
	summary, err := buildTechnicalScoreSummary(c, code, dayCount, includeWeeklyMonthly)
	if err != nil {
		jsonErr(w, err.Error())
		return
	}
	jsonResp(w, AgentCalculationText{
		Code:    code,
		Format:  "text/plain; charset=utf-8",
		Content: summary,
	})
}

func calculationBriefFromRequest(r *http.Request) (AgentStockBrief, error) {
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if code == "" {
		return AgentStockBrief{}, fmt.Errorf("缺少code")
	}
	c := cli()
	if c == nil {
		return AgentStockBrief{}, fmt.Errorf("TDX客户端未连接")
	}
	return buildAgentStockBrief(c, code, r.URL.Query().Get("mkt"))
}

func buildScenarioValuationText(brief AgentStockBrief, query queryValues) string {
	price, priceOK := queryFloat(query, "currentPrice")
	if !priceOK && brief.Quote != nil {
		price = brief.Quote.Price
	}
	eps, epsOK := queryFloat(query, "eps")
	epsSource := "用户输入"
	if !epsOK {
		eps, epsOK = epsFromBrief(brief)
		epsSource = "由当前价格 / PE_TTM 反推"
	}
	years := queryIntFromValues(query, "years", 3, 1, 10)
	if price <= 0 || !epsOK || eps <= 0 {
		return calculationHeader(brief) +
			"情景估值：无法计算。\n" +
			"原因：需要有效当前价格、正 EPS 和有效 PE_TTM；服务端不强行解析研报预测 EPS。\n"
	}

	netProfitYoY := 0.0
	if brief.LatestReport != nil {
		netProfitYoY = brief.LatestReport.NetProfitYoY
	}
	bearGrowth, baseGrowth, bullGrowth := defaultGrowths(netProfitYoY)
	bearPE, basePE, bullPE := defaultPEs(brief.Stat)
	bearGrowth = queryFloatDefault(query, "bearGrowth", bearGrowth)
	baseGrowth = queryFloatDefault(query, "baseGrowth", baseGrowth)
	bullGrowth = queryFloatDefault(query, "bullGrowth", bullGrowth)
	bearPE = queryFloatDefault(query, "bearPE", bearPE)
	basePE = queryFloatDefault(query, "basePE", basePE)
	bullPE = queryFloatDefault(query, "bullPE", bullPE)

	rows := []scenarioRow{
		buildScenarioRow("悲观", price, eps, years, bearGrowth, bearPE),
		buildScenarioRow("中性", price, eps, years, baseGrowth, basePE),
		buildScenarioRow("乐观", price, eps, years, bullGrowth, bullPE),
	}

	var b strings.Builder
	b.WriteString(calculationHeader(brief))
	b.WriteString(fmt.Sprintf("当前价格与 EPS 口径：价格 %.2f 元，EPS %.4f（%s），期限 %d 年。\n", price, eps, epsSource, years))
	b.WriteString("情景估值：\n")
	b.WriteString("情景 | 年增速 | 目标PE | 目标EPS | 目标价 | 涨跌幅 | 年化收益\n")
	for _, row := range rows {
		b.WriteString(fmt.Sprintf(
			"%s | %.2f%% | %.2f | %.4f | %.2f元 | %s | %s\n",
			row.Name,
			row.GrowthPct,
			row.PE,
			row.TargetEPS,
			row.TargetPrice,
			formatPercentText(row.ChangePct),
			formatPercentText(row.AnnualReturnPct),
		))
	}
	b.WriteString("安全边际提示：目标价和收益率只反映输入假设，不代表买卖建议。\n")
	b.WriteString("关键假设：未传入 growth/PE 时使用系统保守默认假设，不代表预测。\n")
	b.WriteString("数据口径与限制：价格、PE 和财务同比来自 TDX 单源；EPS 默认由价格和 PE_TTM 反推。\n")
	return strings.TrimSpace(b.String())
}

func buildImpliedExpectationText(brief AgentStockBrief, query queryValues) string {
	price, priceOK := queryFloat(query, "currentPrice")
	if !priceOK && brief.Quote != nil {
		price = brief.Quote.Price
	}
	eps, epsOK := queryFloat(query, "eps")
	epsSource := "用户输入"
	if !epsOK {
		eps, epsOK = epsFromBrief(brief)
		epsSource = "由当前价格 / PE_TTM 反推"
	}
	years := queryIntFromValues(query, "years", 3, 1, 10)
	targetPE := queryFloatDefault(query, "targetPE", defaultTargetPE(brief.Stat))
	if price <= 0 || targetPE <= 0 || !epsOK || eps <= 0 {
		return calculationHeader(brief) +
			"当前价格隐含预期：无法计算。\n" +
			"原因：需要有效当前价格、正 EPS 和正目标 PE；亏损公司不适用该公式。\n"
	}
	requiredFutureEPS := price / targetPE
	impliedCAGR := (math.Pow(requiredFutureEPS/eps, 1/float64(years)) - 1) * 100
	pressure := "低"
	if impliedCAGR >= 15 {
		pressure = "高"
	} else if impliedCAGR >= 5 {
		pressure = "中"
	}

	var b strings.Builder
	b.WriteString(calculationHeader(brief))
	b.WriteString(fmt.Sprintf("当前价格：%.2f 元。\n", price))
	b.WriteString(fmt.Sprintf("当前 EPS：%.4f（%s）。\n", eps, epsSource))
	b.WriteString(fmt.Sprintf("目标 PE 假设：%.2f。\n", targetPE))
	b.WriteString(fmt.Sprintf("当前价格隐含未来 EPS：%.4f。\n", requiredFutureEPS))
	b.WriteString(fmt.Sprintf("隐含 EPS 年复合增速：%s。\n", formatPercentText(impliedCAGR)))
	if brief.LatestReport != nil {
		b.WriteString(fmt.Sprintf("与已有净利润同比对比：最新净利润同比 %s。\n", formatPercentText(brief.LatestReport.NetProfitYoY)))
	}
	b.WriteString(fmt.Sprintf("估值预期压力：%s。\n", pressure))
	b.WriteString("数据口径与限制：这是价格反推公式，不读取报告、不输出买卖建议。\n")
	return strings.TrimSpace(b.String())
}

func buildTechnicalScoreSummary(
	c *tdx.Client,
	code string,
	dayCount int,
	includeWeeklyMonthly bool,
) (string, error) {
	specs := []agentTechnicalSpec{{
		period: "day",
		name:   "日线",
		count:  uint16(dayCount),
		fetch: func(code string, count uint16) (*protocol.KlineResp, error) {
			return fetchDayKlines(c, code, count)
		},
	}}
	if includeWeeklyMonthly {
		specs = append(specs,
			agentTechnicalSpec{"week", "周线", 156, func(code string, count uint16) (*protocol.KlineResp, error) {
				return fetchWeekKlines(c, code, count)
			}},
			agentTechnicalSpec{"month", "月线", 120, func(code string, count uint16) (*protocol.KlineResp, error) {
				return fetchMonthKlines(c, code, count)
			}},
		)
	}
	periods, warnings, err := buildAgentTechnicalSummaryFromSpecs(code, specs)
	if err != nil {
		return "", err
	}

	rows := make([]technicalScoreRow, 0)
	total := 0
	for _, period := range periods {
		periodRows := scoreTechnicalPeriod(period)
		for _, row := range periodRows {
			if period.Period == "day" {
				total += row.Score
			}
			rows = append(rows, row)
		}
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("股票代码：%s\n", code))
	b.WriteString(fmt.Sprintf("K线口径：日线样本 %d，周/月线 %t；价格指标使用现有 TDX K线口径。\n", dayCount, includeWeeklyMonthly))
	b.WriteString(fmt.Sprintf("技术总分：%d（日线，范围约 -9 到 +9）。\n", total))
	b.WriteString(fmt.Sprintf("趋势结论：%s。\n", technicalScoreLevel(total)))
	b.WriteString("评分明细：\n")
	b.WriteString("周期 | 指标 | 当前值 | 信号 | 分数\n")
	for _, row := range rows {
		b.WriteString(fmt.Sprintf("%s | %s | %s | %s | %+d\n", row.Period, row.Item, row.Value, row.Signal, row.Score))
	}
	if len(warnings) > 0 {
		b.WriteString("风险提示：")
		b.WriteString(strings.Join(warnings, "；"))
		b.WriteString("。\n")
	}
	b.WriteString("数据口径与限制：未单独计算 KDJ、BIAS 和主动买卖，缺失项按 0 分处理。\n")
	return strings.TrimSpace(b.String()), nil
}

func scoreTechnicalPeriod(period AgentTechnicalPeriod) []technicalScoreRow {
	rows := []technicalScoreRow{
		scoreMA(period),
		scoreMACD(period),
		scoreRSI(period),
		scoreBOLL(period),
		{Period: period.Name, Item: "KDJ", Value: "-", Signal: "当前接口未提供", Score: 0},
		{Period: period.Name, Item: "BIAS", Value: "-", Signal: "当前接口未提供", Score: 0},
		{Period: period.Name, Item: "量价", Value: "-", Signal: "当前接口未提供", Score: 0},
		{Period: period.Name, Item: "多空比", Value: "-", Signal: "当前接口未提供", Score: 0},
	}
	return rows
}

func scoreMA(period AgentTechnicalPeriod) technicalScoreRow {
	score := 0
	signals := make([]string, 0, 3)
	for _, name := range []string{"ma20", "ma60", "ma120"} {
		value, ok := metricValue(period.MA[name])
		if !ok {
			continue
		}
		if period.Close >= value {
			score++
			signals = append(signals, name+"上方")
		} else {
			score--
			signals = append(signals, name+"下方")
		}
	}
	if len(signals) == 0 {
		return technicalScoreRow{Period: period.Name, Item: "MA", Value: "-", Signal: "均线不足", Score: 0}
	}
	return technicalScoreRow{Period: period.Name, Item: "MA", Value: fmt.Sprintf("收盘 %.2f", period.Close), Signal: strings.Join(signals, "、"), Score: score}
}

func scoreMACD(period AgentTechnicalPeriod) technicalScoreRow {
	if !period.MACD.Available || period.MACD.Hist == nil {
		return technicalScoreRow{Period: period.Name, Item: "MACD", Value: "-", Signal: "MACD不可得", Score: 0}
	}
	score := 0
	if *period.MACD.Hist > 0 {
		score = 2
	} else if *period.MACD.Hist < 0 {
		score = -2
	}
	return technicalScoreRow{Period: period.Name, Item: "MACD", Value: fmt.Sprintf("%.2f", *period.MACD.Hist), Signal: period.MACD.Signal, Score: score}
}

func scoreRSI(period AgentTechnicalPeriod) technicalScoreRow {
	rsi, ok := metricValue(period.RSI["rsi6"])
	if !ok {
		return technicalScoreRow{Period: period.Name, Item: "RSI", Value: "-", Signal: "RSI不可得", Score: 0}
	}
	score := 0
	signal := "中性"
	switch {
	case rsi >= 80:
		score = -2
		signal = "短线过热"
	case rsi >= 60:
		score = 1
		signal = "偏强"
	case rsi <= 20:
		score = -1
		signal = "弱势超卖"
	case rsi <= 40:
		score = -1
		signal = "偏弱"
	}
	return technicalScoreRow{Period: period.Name, Item: "RSI", Value: fmt.Sprintf("%.2f", rsi), Signal: signal, Score: score}
}

func scoreBOLL(period AgentTechnicalPeriod) technicalScoreRow {
	if !period.BOLL.Available {
		return technicalScoreRow{Period: period.Name, Item: "BOLL", Value: "-", Signal: "布林线不可得", Score: 0}
	}
	score := 0
	if strings.Contains(period.BOLL.Position, "上轨") {
		score = 1
	} else if strings.Contains(period.BOLL.Position, "下轨") {
		score = -1
	}
	return technicalScoreRow{Period: period.Name, Item: "BOLL", Value: period.BOLL.Position, Signal: period.BOLL.Position, Score: score}
}

func buildScenarioRow(name string, price, eps float64, years int, growthPct, pe float64) scenarioRow {
	targetEPS := eps * math.Pow(1+growthPct/100, float64(years))
	targetPrice := targetEPS * pe
	changePct := (targetPrice/price - 1) * 100
	annualReturnPct := (math.Pow(targetPrice/price, 1/float64(years)) - 1) * 100
	return scenarioRow{name, growthPct, pe, targetEPS, targetPrice, changePct, annualReturnPct}
}

func epsFromBrief(brief AgentStockBrief) (float64, bool) {
	if brief.Quote == nil || brief.Stat == nil || brief.Quote.Price <= 0 || brief.Stat.PETTM <= 0 {
		return 0, false
	}
	return brief.Quote.Price / brief.Stat.PETTM, true
}

func defaultGrowths(netProfitYoY float64) (float64, float64, float64) {
	switch {
	case netProfitYoY < 0:
		return -10, 0, 8
	case netProfitYoY <= 20:
		return 0, 8, 15
	default:
		return 5, 12, 20
	}
}

func defaultPEs(stat *AgentBriefStat) (float64, float64, float64) {
	if stat == nil || stat.PETTM <= 0 || stat.PETTM > 100 {
		return 15, 25, 35
	}
	return math.Min(stat.PETTM*0.6, 20), math.Min(stat.PETTM*0.8, 30), math.Min(stat.PETTM, 45)
}

func defaultTargetPE(stat *AgentBriefStat) float64 {
	if stat == nil || stat.PETTM <= 0 {
		return 25
	}
	return math.Min(stat.PETTM*0.8, 30)
}

func technicalScoreLevel(score int) string {
	switch {
	case score >= 7:
		return "强多"
	case score >= 3:
		return "偏多"
	case score <= -7:
		return "强空"
	case score <= -3:
		return "偏空"
	default:
		return "中性"
	}
}

func calculationHeader(brief AgentStockBrief) string {
	if brief.Name != "" {
		return fmt.Sprintf("股票：%s（%s）\n", brief.Name, brief.Code)
	}
	return fmt.Sprintf("股票代码：%s\n", brief.Code)
}

type queryValues interface {
	Get(string) string
}

func queryFloat(query queryValues, name string) (float64, bool) {
	raw := strings.TrimSpace(query.Get(name))
	if raw == "" {
		return 0, false
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

func queryFloatDefault(query queryValues, name string, fallback float64) float64 {
	if value, ok := queryFloat(query, name); ok {
		return value
	}
	return fallback
}

func queryInt(r *http.Request, name string, fallback, min, max int) int {
	return queryIntFromValues(r.URL.Query(), name, fallback, min, max)
}

func queryIntFromValues(query queryValues, name string, fallback, min, max int) int {
	value := fallback
	if raw := strings.TrimSpace(query.Get(name)); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			value = parsed
		}
	}
	if value < min {
		return min
	}
	if max > 0 && value > max {
		return max
	}
	return value
}

func queryBool(r *http.Request, name string, fallback bool) bool {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return value
}

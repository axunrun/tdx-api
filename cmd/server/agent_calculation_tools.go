package main

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

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
	adjust, err := normalizeTechnicalAdjust(r.URL.Query().Get("adjust"))
	if err != nil {
		jsonErr(w, err.Error())
		return
	}
	c := cli()
	if c == nil {
		jsonErr(w, "TDX客户端未连接")
		return
	}
	dayCount := queryInt(r, "dayCount", 250, 60, 500)
	includeWeeklyMonthly := queryBool(r, "includeWeeklyMonthly", true)
	summary, err := buildTechnicalScoreSummary(c, code, dayCount, includeWeeklyMonthly, adjust)
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
	epsInput := epsOK
	epsSource := "用户输入"
	if !epsOK {
		eps, epsOK = epsFromBrief(brief)
		epsSource = "由当前价格 / PE_TTM 反推"
	}
	years, yearsOK := queryYears(query)
	if !yearsOK {
		return calculationHeader(brief) +
			"三情景估值：无法计算。\n" +
			"原因：years 必须在 1-10 之间。\n"
	}
	if price <= 0 || !epsOK || eps <= 0 {
		return calculationHeader(brief) +
			"情景估值：无法计算。\n" +
			"原因：需要有效当前价格、正 EPS 和有效 PE_TTM；服务端不强行解析研报预测 EPS。\n"
	}
	if hasNonPositiveInput(query, "bearPE", "basePE", "bullPE") {
		return calculationHeader(brief) +
			"三情景估值：无法计算。\n" +
			"原因：目标PE必须为正数；收到 bearPE/basePE/bullPE 中存在 <= 0 的值。\n"
	}

	netProfitYoY := 0.0
	if brief.LatestReport != nil {
		netProfitYoY = brief.LatestReport.NetProfitYoY
	}
	bearGrowth, baseGrowth, bullGrowth := defaultGrowths(netProfitYoY)
	bearPE, basePE, bullPE := defaultPEs(brief.Stat)
	inputAssumptions := countFloatInputs(query,
		"bearGrowth", "baseGrowth", "bullGrowth",
		"bearPE", "basePE", "bullPE",
	)
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
	b.WriteString(calculationPriceFreshnessText(brief, queryFloatInput(query, "currentPrice")))
	b.WriteString(fmt.Sprintf("当前价格与 EPS 口径：价格 %.2f 元，EPS %.4f（%s），期限 %d 年。\n", price, eps, epsSource, years))
	if !epsInput && brief.Stat != nil && brief.Stat.Date != "" {
		b.WriteString("PE_TTM统计日期：" + formatTdxStatDate(brief.Stat.Date) + "。\n")
	}
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
	b.WriteString("关键假设：")
	b.WriteString(scenarioAssumptionText(inputAssumptions))
	b.WriteString("\n")
	b.WriteString("数据口径与限制：")
	b.WriteString(scenarioDataLimitText(queryFloatInput(query, "currentPrice"), epsInput, inputAssumptions))
	b.WriteString("\n")
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
	years, yearsOK := queryYears(query)
	if !yearsOK {
		return calculationHeader(brief) +
			"当前价格隐含预期：无法计算。\n" +
			"原因：years 必须在 1-10 之间。\n"
	}
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
	b.WriteString(calculationPriceFreshnessText(brief, queryFloatInput(query, "currentPrice")))
	b.WriteString(fmt.Sprintf("当前价格：%.2f 元。\n", price))
	b.WriteString(fmt.Sprintf("当前 EPS：%.4f（%s）。\n", eps, epsSource))
	if !queryFloatInput(query, "eps") && brief.Stat != nil && brief.Stat.Date != "" {
		b.WriteString("PE_TTM统计日期：" + formatTdxStatDate(brief.Stat.Date) + "。\n")
	}
	b.WriteString(fmt.Sprintf("目标 PE 假设：%.2f。\n", targetPE))
	b.WriteString(fmt.Sprintf("当前价格隐含未来 EPS：%.4f。\n", requiredFutureEPS))
	b.WriteString(fmt.Sprintf("隐含 EPS 年复合增速：%s。\n", formatPercentText(impliedCAGR)))
	if brief.LatestReport != nil {
		reportDate := valueOrDash(brief.LatestReport.ReportDate)
		b.WriteString(fmt.Sprintf(
			"与已有净利润同比对比：报告期%s净利润同比%s。\n",
			reportDate,
			formatPercentText(brief.LatestReport.NetProfitYoY),
		))
	}
	b.WriteString(fmt.Sprintf("估值预期压力：%s。\n", pressure))
	b.WriteString("数据口径与限制：这是价格反推公式，不读取报告、不输出买卖建议。\n")
	return strings.TrimSpace(b.String())
}

func calculationPriceFreshnessText(brief AgentStockBrief, manualPrice bool) string {
	if manualPrice {
		return "价格来源：用户输入。\n"
	}
	if brief.Quote == nil {
		return "价格来源：TDX行情；查询时间、行情日期和状态不可用。\n"
	}
	return fmt.Sprintf(
		"价格来源：TDX行情；查询时间：%s；行情日期：%s；行情状态：%s。\n",
		valueOrDash(brief.Quote.QueryTime),
		valueOrDash(brief.Quote.DataDate),
		valueOrDash(brief.Quote.DataStatus),
	)
}

func buildTechnicalScoreSummary(
	c *tdx.Client,
	code string,
	dayCount int,
	includeWeeklyMonthly bool,
	adjust string,
) (string, error) {
	specs := []agentTechnicalSpec{{
		period: "day",
		name:   "日线",
		count:  uint16(indicatorWarmupCount(dayCount)),
		fetch: func(code string, count uint16) (*protocol.KlineResp, error) {
			return fetchTechnicalDayKlines(c, code, count, adjust)
		},
	}}
	if includeWeeklyMonthly {
		specs = append(specs,
			agentTechnicalSpec{"week", "周线", agentIndicatorWarmupBars, func(code string, count uint16) (*protocol.KlineResp, error) {
				return fetchWeekKlines(c, code, count)
			}},
			agentTechnicalSpec{"month", "月线", agentIndicatorWarmupBars, func(code string, count uint16) (*protocol.KlineResp, error) {
				return fetchMonthKlines(c, code, count)
			}},
		)
	}
	periods := make([]technicalScorePeriod, 0, len(specs))
	warnings := make([]string, 0)
	for _, spec := range specs {
		resp, err := spec.fetch(code, spec.count)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%sK线失败: %v", spec.name, err))
			continue
		}
		if resp == nil || len(resp.List) == 0 {
			warnings = append(warnings, fmt.Sprintf("%sK线无数据", spec.name))
			continue
		}
		klines := protocol.Klines(resp.List)
		periods = append(periods, technicalScorePeriod{
			Summary: buildAgentTechnicalPeriod(spec.period, spec.name, klines),
			Klines:  klines,
		})
	}
	if len(periods) == 0 {
		if len(warnings) > 0 {
			return "", fmt.Errorf(strings.Join(warnings, "; "))
		}
		return "", fmt.Errorf("无K线数据")
	}
	dayKline := latestTechnicalScoreDayKline(periods)
	bullBearRow := technicalScoreRow{
		Period: "日线",
		Item:   "多空比",
		Value:  "-",
		Signal: "日线数据日期不可用",
	}
	bullBearWarning := ""
	if dayKline != nil {
		bullBearRow, bullBearWarning = scoreBullBearFromTradesForDate(
			c,
			code,
			dayKline.Time.Format(time.DateOnly),
		)
	}
	if bullBearWarning != "" {
		warnings = append(warnings, bullBearWarning)
	}

	rows := make([]technicalScoreRow, 0)
	total := 0
	for _, period := range periods {
		periodRows := scoreTechnicalPeriod(period.Summary, period.Klines)
		if period.Summary.Period == "day" {
			periodRows = append(periodRows, bullBearRow)
		} else {
			periodRows = append(periodRows, technicalScoreRow{
				Period: period.Summary.Name,
				Item:   "多空比",
				Value:  "-",
				Signal: "仅日线使用逐笔成交估算",
				Score:  0,
			})
		}
		for _, row := range periodRows {
			if period.Summary.Period == "day" {
				total += row.Score
			}
			rows = append(rows, row)
		}
	}
	now := time.Now()
	var b strings.Builder
	b.WriteString(fmt.Sprintf("股票代码：%s\n", code))
	b.WriteString(fmt.Sprintf("查询时间：%s\n", now.Format(time.RFC3339)))
	b.WriteString(technicalScoreKlineHeader(adjust, dayCount, includeWeeklyMonthly))
	for _, period := range periods {
		if period.Summary.Period == "day" {
			continue
		}
		b.WriteString(fmt.Sprintf(
			"%s数据日期：%s\n",
			period.Summary.Name,
			period.Summary.LatestDate,
		))
	}
	if dayKline == nil {
		b.WriteString("日线数据日期：不可用\n")
		b.WriteString("日线数据状态：日线K线不可用，技术总分不具备参考意义。\n")
		b.WriteString("技术总分：不可用（日线K线缺失）。\n")
		b.WriteString("趋势结论：不可用。\n")
	} else {
		b.WriteString(fmt.Sprintf(
			"日线数据日期：%s\n",
			dayKline.Time.Format(time.DateOnly),
		))
		b.WriteString(fmt.Sprintf(
			"日线数据状态：%s。\n",
			technicalScoreDayDataStatus(dayKline.Time, now),
		))
		b.WriteString(fmt.Sprintf("技术总分：%d（日线，范围约 -12 到 +12）。\n", total))
		b.WriteString(fmt.Sprintf("趋势结论：%s。\n", technicalScoreLevel(total)))
	}
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
	b.WriteString("数据口径与限制：递推指标至少使用250根K线预热；RSI使用Wilder平滑；KDJ(9,3,3)逐根递推，金叉/死叉比较前后周期；BIAS/量价由K线公式计算；多空比由TDX逐笔成交估算，仅日线计分，不等同官方资金流。\n")
	return strings.TrimSpace(b.String()), nil
}

func latestTechnicalScoreDayKline(periods []technicalScorePeriod) *protocol.Kline {
	for _, period := range periods {
		if period.Summary.Period != "day" {
			continue
		}
		for i := len(period.Klines) - 1; i >= 0; i-- {
			if period.Klines[i] != nil {
				return period.Klines[i]
			}
		}
	}
	return nil
}

func technicalScoreDayDataStatus(latest time.Time, now time.Time) string {
	latestDate := dateOnly(latest)
	today := dateOnly(now)
	if latestDate.Before(today) {
		if resolveSectorRealtimeSession(now).status == "trading" {
			return "当前交易日K线尚未返回，使用最近完整交易日收盘值"
		}
		return "最近完整交易日收盘值"
	}
	if latestDate.After(today) {
		return "数据日期晚于系统日期，请检查服务器时间"
	}
	switch resolveSectorRealtimeSession(now).status {
	case "trading":
		return "盘中动态值，当前日K尚未收盘"
	case "break":
		return "午间休市动态值，当前日K尚未收盘"
	case "preopen":
		return "盘前动态值，当前日K尚未收盘"
	default:
		return "当日收盘值，日K已完成"
	}
}

func normalizeTechnicalAdjust(raw string) (string, error) {
	adjust := strings.TrimSpace(raw)
	if adjust == "" {
		return "qfq", nil
	}
	if adjust != "qfq" && adjust != "none" {
		return "", fmt.Errorf("adjust 必须为 qfq 或 none")
	}
	return adjust, nil
}

func fetchTechnicalDayKlines(
	c *tdx.Client,
	code string,
	count uint16,
	adjust string,
) (*protocol.KlineResp, error) {
	if adjust == "none" {
		return fetchDayKlines(c, code, count)
	}
	gb := getGbbq()
	if gb == nil {
		return nil, fmt.Errorf("前复权模块未就绪")
	}
	ks, err := gb.QFQKlineDay(code)
	if err != nil {
		return nil, err
	}
	if len(ks) == 0 {
		return nil, fmt.Errorf("前复权K线无数据")
	}
	if int(count) > 0 && len(ks) > int(count) {
		ks = ks[len(ks)-int(count):]
	}
	return &protocol.KlineResp{List: ks}, nil
}

func technicalScoreKlineHeader(adjust string, dayCount int, includeWeeklyMonthly bool) string {
	if adjust == "none" {
		return fmt.Sprintf(
			"K线口径：日线参数 %d，指标预热请求 %d，周/月线 %t；日线价格指标使用未复权TDX日K线；成交量使用未复权成交量；周/月线使用现有TDX周/月K线口径。\n",
			dayCount,
			indicatorWarmupCount(dayCount),
			includeWeeklyMonthly,
		)
	}
	return fmt.Sprintf(
		"K线口径：日线参数 %d，指标预热请求 %d，周/月线 %t；日线价格指标使用 Gbbq 前复权日K线；成交量沿用K线原始volume；周/月线使用现有TDX周/月K线口径。\n",
		dayCount,
		indicatorWarmupCount(dayCount),
		includeWeeklyMonthly,
	)
}

type technicalScorePeriod struct {
	Summary AgentTechnicalPeriod
	Klines  protocol.Klines
}

func scoreTechnicalPeriod(period AgentTechnicalPeriod, klines protocol.Klines) []technicalScoreRow {
	kdj := period.KDJ
	if !kdj.Available && kdj.Reason == "" {
		kdj = buildKDJ(klines)
	}
	bias := period.BIAS
	if !bias.Available && bias.Reason == "" {
		bias = buildBIAS(klines)
	}
	volumePrice := period.VolumePrice
	if !volumePrice.Available && volumePrice.Reason == "" {
		volumePrice = buildVolumePrice(klines)
	}
	rows := []technicalScoreRow{
		scoreMA(period),
		scoreMACD(period),
		scoreRSI(period),
		scoreBOLL(period),
		scoreAgentKDJ(period.Name, kdj),
		scoreAgentBIAS(period.Name, bias),
		scoreATR(period),
		scoreOBV(period),
		scoreAgentVolumePrice(period.Name, volumePrice),
	}
	return rows
}

func scoreMA(period AgentTechnicalPeriod) technicalScoreRow {
	score := 0
	signals := make([]string, 0, 3)
	values := make([]string, 0, 5)
	for _, name := range []string{"ma5", "ma10", "ma20", "ma60", "ma120"} {
		value, ok := metricValue(period.MA[name])
		if ok {
			values = append(values, fmt.Sprintf("%s=%.2f", strings.ToUpper(name), value))
		}
	}
	for _, name := range []string{"ma20", "ma60", "ma120"} {
		value, ok := metricValue(period.MA[name])
		if !ok {
			continue
		}
		if period.Close >= value {
			score++
			signals = append(signals, strings.ToUpper(name)+"上方")
		} else {
			score--
			signals = append(signals, strings.ToUpper(name)+"下方")
		}
	}
	if len(signals) == 0 {
		return technicalScoreRow{Period: period.Name, Item: "MA", Value: "-", Signal: "均线不足", Score: 0}
	}
	return technicalScoreRow{
		Period: period.Name,
		Item:   "MA",
		Value: fmt.Sprintf(
			"收盘 %.2f %s",
			period.Close,
			strings.Join(values, " "),
		),
		Signal: strings.Join(signals, "、"),
		Score:  score,
	}
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
	value := fmt.Sprintf("MACD柱=%.2f", *period.MACD.Hist)
	if period.MACD.DIF != nil && period.MACD.DEA != nil {
		value = fmt.Sprintf(
			"DIF=%.2f DEA=%.2f MACD柱=%.2f",
			*period.MACD.DIF,
			*period.MACD.DEA,
			*period.MACD.Hist,
		)
	}
	return technicalScoreRow{
		Period: period.Name,
		Item:   "MACD",
		Value:  value,
		Signal: period.MACD.Signal,
		Score:  score,
	}
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
	values := []string{fmt.Sprintf("RSI6=%.2f", rsi)}
	for _, name := range []string{"rsi12", "rsi24"} {
		if value, available := metricValue(period.RSI[name]); available {
			values = append(values, fmt.Sprintf("%s=%.2f", strings.ToUpper(name), value))
		}
	}
	return technicalScoreRow{
		Period: period.Name,
		Item:   "RSI",
		Value:  strings.Join(values, " "),
		Signal: signal,
		Score:  score,
	}
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
	value := period.BOLL.Position
	if period.BOLL.Upper != nil && period.BOLL.Middle != nil && period.BOLL.Lower != nil {
		value = fmt.Sprintf(
			"上轨=%.2f 中轨=%.2f 下轨=%.2f",
			*period.BOLL.Upper,
			*period.BOLL.Middle,
			*period.BOLL.Lower,
		)
	}
	return technicalScoreRow{
		Period: period.Name,
		Item:   "BOLL",
		Value:  value,
		Signal: period.BOLL.Position,
		Score:  score,
	}
}

func scoreKDJ(periodName string, klines protocol.Klines) technicalScoreRow {
	return scoreAgentKDJ(periodName, buildKDJ(klines))
}

func scoreAgentKDJ(periodName string, kdj AgentKDJ) technicalScoreRow {
	if !kdj.Available {
		return technicalScoreRow{
			Period: periodName,
			Item:   "KDJ",
			Value:  "-",
			Signal: valueOrDefault(kdj.Reason, "KDJ不可得"),
			Score:  0,
		}
	}
	return technicalScoreRow{
		Period: periodName,
		Item:   "KDJ",
		Value:  fmt.Sprintf("K=%.2f D=%.2f J=%.2f", kdj.K, kdj.D, kdj.J),
		Signal: kdj.Signal,
		Score:  kdj.Score,
	}
}

func kdjSignal(previousK, previousD, currentK, currentD float64) (string, int) {
	switch {
	case previousK <= previousD && currentK > currentD:
		return "K上穿D", 1
	case previousK >= previousD && currentK < currentD:
		return "K下穿D", -1
	case currentK > currentD:
		return "K在D上方", 1
	case currentK < currentD:
		return "K在D下方", -1
	default:
		return "K与D重合", 0
	}
}

func scoreAgentBIAS(periodName string, bias AgentBIAS) technicalScoreRow {
	if !bias.Available {
		return technicalScoreRow{
			Period: periodName,
			Item:   "BIAS",
			Value:  "-",
			Signal: valueOrDefault(bias.Reason, "BIAS不可得"),
			Score:  0,
		}
	}
	return technicalScoreRow{
		Period: periodName,
		Item:   "BIAS",
		Value:  fmt.Sprintf("BIAS5=%.2f%% BIAS10=%.2f%%", bias.BIAS5, bias.BIAS10),
		Signal: bias.Signal,
		Score:  bias.Score,
	}
}

func scoreAgentVolumePrice(
	periodName string,
	volumePrice AgentVolumePrice,
) technicalScoreRow {
	if !volumePrice.Available {
		return technicalScoreRow{
			Period: periodName,
			Item:   "量价",
			Value:  "-",
			Signal: valueOrDefault(volumePrice.Reason, "量价不可得"),
			Score:  0,
		}
	}
	return technicalScoreRow{
		Period: periodName,
		Item:   "量价",
		Value: fmt.Sprintf(
			"量比 %.2f 涨跌幅 %s",
			volumePrice.VolumeRatio,
			formatPercentText(volumePrice.ChangePct),
		),
		Signal: volumePrice.Signal,
		Score:  volumePrice.Score,
	}
}

func scoreATR(period AgentTechnicalPeriod) technicalScoreRow {
	if !period.ATR.Available || period.ATR.ATR14 == nil {
		return technicalScoreRow{
			Period: period.Name,
			Item:   "ATR",
			Value:  "-",
			Signal: valueOrDefault(period.ATR.Reason, "ATR不可得"),
		}
	}
	atrPct := 0.0
	if period.Close > 0 {
		atrPct = *period.ATR.ATR14 / period.Close * 100
	}
	return technicalScoreRow{
		Period: period.Name,
		Item:   "ATR",
		Value: fmt.Sprintf(
			"ATR14=%.2f ATR占价格%s",
			*period.ATR.ATR14,
			formatPercentText(atrPct),
		),
		Signal: "波动观察项，不直接代表方向",
		Score:  0,
	}
}

func scoreOBV(period AgentTechnicalPeriod) technicalScoreRow {
	if !period.OBV.Available {
		return technicalScoreRow{
			Period: period.Name,
			Item:   "OBV",
			Value:  "-",
			Signal: valueOrDefault(period.OBV.Reason, "OBV不可得"),
		}
	}
	return technicalScoreRow{
		Period: period.Name,
		Item:   "OBV",
		Value: fmt.Sprintf(
			"OBV20=%s OBV5=%s",
			formatPercentText(period.OBV.Change20Pct),
			formatPercentText(period.OBV.Change5Pct),
		),
		Signal: period.OBV.Signal,
		Score:  0,
	}
}

func valueOrDefault(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func scoreBullBearFromTradesForDate(
	c *tdx.Client,
	code string,
	date string,
) (technicalScoreRow, string) {
	row := technicalScoreRow{Period: "日线", Item: "多空比", Value: "-", Signal: "逐笔成交不可得", Score: 0}
	trades, err := fetchTradeFlowTrades(c, code, date)
	if err != nil {
		return row, fmt.Sprintf("多空比逐笔成交获取失败: %v", err)
	}
	estimate := buildTradeFlowEstimate(code, date, trades)
	buy := estimate.Summary.TotalBuyAmount
	sell := estimate.Summary.TotalSellAmount
	if buy == 0 && sell == 0 {
		row.Signal = "无主动买卖估算数据"
		return row, ""
	}
	if sell == 0 {
		row.Value = "买/卖 -"
		row.Signal = "主动买入估算占优"
		row.Score = 1
		return row, ""
	}
	ratio := buy / sell
	row.Value = fmt.Sprintf("买/卖 %.2f", ratio)
	row.Signal = "主动买卖均衡"
	switch {
	case ratio > 1.2:
		row.Signal = "主动买入估算占优"
		row.Score = 1
	case ratio < 0.8:
		row.Signal = "主动卖出估算占优"
		row.Score = -1
	}
	return row, ""
}

func attachAgentBullBear(
	c *tdx.Client,
	code string,
	summary *AgentTechnicalSummary,
) string {
	if summary == nil {
		return ""
	}
	date := summary.dayData.DataDate
	if date == "" || date == "不可用" {
		summary.bullBear = technicalScoreRow{
			Period: "日线",
			Item:   "多空比",
			Value:  "-",
			Signal: "日线数据日期不可用",
		}
		return ""
	}
	row, warning := scoreBullBearFromTradesForDate(c, code, date)
	summary.bullBear = row
	return warning
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

func queryFloatInput(query queryValues, name string) bool {
	_, ok := queryFloat(query, name)
	return ok
}

func countFloatInputs(query queryValues, names ...string) int {
	count := 0
	for _, name := range names {
		if queryFloatInput(query, name) {
			count++
		}
	}
	return count
}

func hasNonPositiveInput(query queryValues, names ...string) bool {
	for _, name := range names {
		value, ok := queryFloat(query, name)
		if ok && value <= 0 {
			return true
		}
	}
	return false
}

func scenarioAssumptionText(inputCount int) string {
	switch inputCount {
	case 0:
		return "系统保守默认假设，不代表预测。"
	case 6:
		return "使用用户输入的增长率和目标PE。"
	default:
		return "用户输入 + 系统默认补齐。"
	}
}

func scenarioDataLimitText(priceInput, epsInput bool, inputAssumptions int) string {
	if priceInput || epsInput || inputAssumptions > 0 {
		return "价格/EPS/增长率/目标PE含用户输入项；本工具只做公式计算，不输出买卖建议。"
	}
	return "价格、PE 和财务同比来自 TDX 单源；EPS 由当前价格和 PE_TTM 反推；本工具只做公式计算，不输出买卖建议。"
}

func queryYears(query queryValues) (int, bool) {
	value := 3
	if raw := strings.TrimSpace(query.Get("years")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			return 0, false
		}
		value = parsed
	}
	if value < 1 || value > 10 {
		return 0, false
	}
	return value, true
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

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

const (
	agentKlineBriefDayCount  = 60
	agentKlineNormalDayCount = 120
	agentKlineDeepDayCount   = 250
	agentKlineMaxDayCount    = 500
)

type AgentKlineSummaryText struct {
	Code    string `json:"code"`
	Format  string `json:"format"`
	Content string `json:"content"`
}

type AgentKlineLimits struct {
	Day      int  `json:"day"`
	DayMax   int  `json:"dayMax"`
	WeekAll  bool `json:"weekAll"`
	MonthAll bool `json:"monthAll"`
}

type AgentKlineSummary struct {
	Code            string                `json:"code"`
	Name            string                `json:"name,omitempty"`
	Source          string                `json:"source"`
	Level           string                `json:"level"`
	Periods         []AgentKlinePeriodRaw `json:"periods"`
	Limits          AgentKlineLimits      `json:"limits"`
	Note            string                `json:"note"`
	Warnings        []string              `json:"warnings,omitempty"`
	analysisPeriods []AgentKlinePeriodSummary
	dayData         agentDayDataContext
	bullBear        technicalScoreRow
}

type AgentKlinePeriodRaw struct {
	Period        string           `json:"period"`
	TotalCount    int              `json:"totalCount"`
	ReturnedCount int              `json:"returnedCount"`
	StartDate     string           `json:"startDate"`
	EndDate       string           `json:"endDate"`
	Items         []AgentKlineItem `json:"items"`
}

type AgentKlineItem struct {
	Date   string  `json:"date"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume int64   `json:"volume"`
	Amount float64 `json:"amount"`
}

type AgentKlinePeriodSummary struct {
	Period         string
	Name           string
	TotalCount     int
	UsedCount      int
	StartDate      string
	EndDate        string
	Open           float64
	Close          float64
	High           float64
	Low            float64
	ChangePct      float64
	MaxDrawdownPct float64
	VolatilityPct  float64
	Trend          string
	TrendStage     string
	RiskLevel      string
	Position       string
	RSI6           *float64
	MA20           *float64
	MA60           *float64
	StageReturns   map[string]float64
	Volume         AgentKlineVolumeSummary
	OBV            AgentOBV
	KeyLevels      AgentKlineKeyLevels
	MovingAverages AgentKlineMASummary
	Candle         AgentKlineCandleSummary
	Volatility     AgentKlineVolatility
	Streak         AgentKlineStreak
	Signals        []string
	Summary        string
	Technical      AgentTechnicalPeriod `json:"-"`
	YearRange      AgentKlineYearRange
}

type AgentKlineYearRange struct {
	Available         bool
	StartDate         string
	EndDate           string
	High              float64
	Low               float64
	DistanceToHighPct float64
	DistanceToLowPct  float64
}

type AgentKlineVolumeSummary struct {
	Latest      int64
	Avg5        float64
	Avg20       float64
	VolumeRatio float64
	Signal      string
}

type AgentKlineKeyLevels struct {
	High20              float64
	Low20               float64
	High60              float64
	Low60               float64
	High120             float64
	Low120              float64
	DistanceToHigh20Pct float64
	DistanceToLow20Pct  float64
}

type AgentKlineMASummary struct {
	MA5            *float64
	MA10           *float64
	MA20           *float64
	MA60           *float64
	MA120          *float64
	PriceVsMA20Pct float64
	Alignment      string
}

type AgentKlineCandleSummary struct {
	Shape          string
	BodyPct        float64
	UpperShadowPct float64
	LowerShadowPct float64
	AmplitudePct   float64
}

type AgentKlineVolatility struct {
	Atr               float64
	AtrPct            float64
	AvgAmplitude5Pct  float64
	AvgAmplitude20Pct float64
	Risk              string
}

type AgentKlineStreak struct {
	Direction string
	Count     int
	ChangePct float64
}

func handleAgentKlineSummary(w http.ResponseWriter, r *http.Request) {
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
	summary, err := buildAgentKlineSummary(
		c,
		code,
		r.URL.Query().Get("level"),
		r.URL.Query().Get("dayCount"),
		"none",
	)
	if err != nil {
		jsonErr(w, err.Error())
		return
	}
	jsonResp(w, summary)
}

func handleAgentKlineSummaryText(w http.ResponseWriter, r *http.Request) {
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
	summary, err := buildAgentKlineSummary(
		c,
		code,
		r.URL.Query().Get("level"),
		r.URL.Query().Get("dayCount"),
		r.URL.Query().Get("adjust"),
	)
	if err != nil {
		jsonErr(w, err.Error())
		return
	}
	jsonResp(w, AgentKlineSummaryText{
		Code:    code,
		Format:  "text/plain; charset=utf-8",
		Content: buildAgentKlineSummaryText(summary),
	})
}

func buildAgentKlineSummary(
	c *tdx.Client,
	code string,
	level string,
	rawDayCount string,
	rawAdjust string,
) (AgentKlineSummary, error) {
	adjust, err := normalizeTechnicalAdjust(rawAdjust)
	if err != nil {
		return AgentKlineSummary{}, err
	}
	now := time.Now()
	dayCount := 0
	if rawDayCount != "" {
		dayCount, _ = strconv.Atoi(rawDayCount)
	}
	limits := resolveAgentKlineLimits(level, dayCount)
	normalizedLevel := normalizeAgentKlineLevel(level)
	warnings := make([]string, 0)
	rawPeriods := make([]AgentKlinePeriodRaw, 0, 3)
	analysisPeriods := make([]AgentKlinePeriodSummary, 0, 3)

	appendPeriod := func(period string, klines protocol.Klines, usedLimit int) {
		rawPeriods = append(rawPeriods, buildAgentKlinePeriodRaw(period, klines, usedLimit))
		analysisPeriods = append(analysisPeriods, buildAgentKlinePeriodSummary(
			period,
			agentKlinePeriodName(period),
			klines,
			usedLimit,
		))
	}

	if resp, err := fetchTechnicalDayKlines(
		c,
		code,
		uint16(indicatorWarmupCount(limits.Day)),
		adjust,
	); err != nil {
		warnings = append(warnings, agentKlinePeriodFailMessage("day", err))
	} else if resp != nil && len(resp.List) > 0 {
		appendPeriod("day", protocol.Klines(resp.List), limits.Day)
	}
	if resp, err := c.GetKlineWeekAll(code); err != nil {
		warnings = append(warnings, agentKlinePeriodFailMessage("week", err))
	} else if resp != nil && len(resp.List) > 0 {
		appendPeriod("week", protocol.Klines(resp.List), len(resp.List))
	}
	if resp, err := c.GetKlineMonthAll(code); err != nil {
		warnings = append(warnings, agentKlinePeriodFailMessage("month", err))
	} else if resp != nil && len(resp.List) > 0 {
		appendPeriod("month", protocol.Klines(resp.List), len(resp.List))
	}

	if len(rawPeriods) == 0 {
		if len(warnings) > 0 {
			return AgentKlineSummary{}, fmt.Errorf(strings.Join(warnings, "; "))
		}
		return AgentKlineSummary{}, fmt.Errorf("K线数据不足")
	}
	technicalPeriods := make([]AgentTechnicalPeriod, 0, 1)
	for _, period := range analysisPeriods {
		if period.Period != "day" {
			continue
		}
		technicalPeriods = append(technicalPeriods, AgentTechnicalPeriod{
			Period:     "day",
			LatestDate: period.EndDate,
		})
		break
	}
	return AgentKlineSummary{
		Code:            code,
		Name:            queryStockName(code),
		Source:          "tdx_agent_kline_raw",
		Level:           normalizedLevel,
		Periods:         rawPeriods,
		Limits:          limits,
		Note:            "JSON返回原始K线聚合数据：日线按level/dayCount限量，周线和月线全量返回；Agent分析请使用kline-summary-text。",
		Warnings:        warnings,
		analysisPeriods: analysisPeriods,
		dayData:         buildAgentDayDataContext(technicalPeriods, adjust, now),
	}, nil
}

func buildAgentKlinePeriodRaw(
	period string,
	klines protocol.Klines,
	usedLimit int,
) AgentKlinePeriodRaw {
	total := len(klines)
	used := limitKlines(klines, usedLimit)
	items := make([]AgentKlineItem, 0, len(used))
	for _, item := range used {
		items = append(items, AgentKlineItem{
			Date:   item.Time.Format("2006-01-02"),
			Open:   item.Open.Float64(),
			High:   item.High.Float64(),
			Low:    item.Low.Float64(),
			Close:  item.Close.Float64(),
			Volume: item.Volume,
			Amount: item.Amount.Float64(),
		})
	}
	raw := AgentKlinePeriodRaw{
		Period:        period,
		TotalCount:    total,
		ReturnedCount: len(items),
		Items:         items,
	}
	if len(items) > 0 {
		raw.StartDate = items[0].Date
		raw.EndDate = items[len(items)-1].Date
	}
	return raw
}

func agentKlinePeriodName(period string) string {
	switch period {
	case "day":
		return "日线"
	case "week":
		return "周线"
	case "month":
		return "月线"
	default:
		return period
	}
}

func agentKlinePeriodFailMessage(period string, err error) string {
	if err == nil {
		return agentKlinePeriodName(period) + "失败"
	}
	return agentKlinePeriodName(period) + "失败: " + err.Error()
}

func resolveAgentKlineLimits(level string, dayCount int) AgentKlineLimits {
	if dayCount <= 0 {
		switch normalizeAgentKlineLevel(level) {
		case "brief":
			dayCount = agentKlineBriefDayCount
		case "deep":
			dayCount = agentKlineDeepDayCount
		default:
			dayCount = agentKlineNormalDayCount
		}
	}
	if dayCount > agentKlineMaxDayCount {
		dayCount = agentKlineMaxDayCount
	}
	if dayCount < 20 {
		dayCount = 20
	}
	return AgentKlineLimits{
		Day:      dayCount,
		DayMax:   agentKlineMaxDayCount,
		WeekAll:  true,
		MonthAll: true,
	}
}

func normalizeAgentKlineLevel(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "brief":
		return "brief"
	case "deep":
		return "deep"
	default:
		return "normal"
	}
}

func buildAgentKlinePeriodSummary(
	period string,
	name string,
	klines protocol.Klines,
	usedLimit int,
) AgentKlinePeriodSummary {
	used := limitKlines(klines, usedLimit)
	first := used[0]
	last := used[len(used)-1]
	open := first.Open.Float64()
	close := last.Close.Float64()
	high := first.High.Float64()
	low := first.Low.Float64()
	for _, item := range used {
		high = math.Max(high, item.High.Float64())
		low = math.Min(low, item.Low.Float64())
	}

	changePct := 0.0
	if open > 0 {
		changePct = (close - open) / open * 100
	}
	volatilityPct := 0.0
	if close > 0 {
		volatilityPct = (high - low) / close * 100
	}
	ma20 := optionalKlineMA(used, 20)
	ma60 := optionalKlineMA(used, 60)
	stageReturns := klineStageReturns(used)
	summary := AgentKlinePeriodSummary{
		Period:         period,
		Name:           name,
		TotalCount:     len(klines),
		UsedCount:      len(used),
		StartDate:      first.Time.Format("2006-01-02"),
		EndDate:        last.Time.Format("2006-01-02"),
		Open:           open,
		Close:          close,
		High:           high,
		Low:            low,
		ChangePct:      changePct,
		MaxDrawdownPct: klineMaxDrawdownPct(used),
		VolatilityPct:  volatilityPct,
		Trend:          klinePriceTrend(stageReturns),
		Position:       klineRangePosition(close, high, low),
		RSI6:           optionalKlineRSI(klines, 6),
		MA20:           ma20,
		MA60:           ma60,
		StageReturns:   stageReturns,
		Volume:         klineVolumeSummary(used),
		OBV:            buildOBV(klines),
		KeyLevels:      klineKeyLevels(used, close),
		MovingAverages: buildKlineMASummary(used, close),
		Candle:         klineCandleSummary(last),
		Volatility:     klineVolatilitySummary(klines),
		Streak:         klineStreakSummary(used),
		Technical:      buildAgentTechnicalPeriod(period, name, klines),
	}
	if period == "day" {
		summary.YearRange = klineYearRange(klines, close)
	}
	summary.TrendStage = klineTrendStage(summary)
	summary.RiskLevel = klineRiskLevel(summary)
	summary.Signals = klineSummarySignals(summary, used)
	summary.Summary = klineNarrativeSummary(summary)
	return summary
}

func limitKlines(klines protocol.Klines, usedLimit int) protocol.Klines {
	total := len(klines)
	if usedLimit > 0 && usedLimit < total {
		return klines[total-usedLimit:]
	}
	return klines
}

func optionalKlineMA(klines protocol.Klines, days int) *float64 {
	if len(klines) < days {
		return nil
	}
	sum := 0.0
	for _, item := range klines[len(klines)-days:] {
		sum += item.Close.Float64()
	}
	value := sum / float64(days)
	return &value
}

func optionalKlineRSI(klines protocol.Klines, days int) *float64 {
	if len(klines) <= days {
		return nil
	}
	value := klines.RSIFloat(days)
	return &value
}

func klineStageReturns(klines protocol.Klines) map[string]float64 {
	return map[string]float64{
		"ret5":   klineReturnPct(klines, 5),
		"ret10":  klineReturnPct(klines, 10),
		"ret20":  klineReturnPct(klines, 20),
		"ret60":  klineReturnPct(klines, 60),
		"ret120": klineReturnPct(klines, 120),
	}
}

func klineReturnPct(klines protocol.Klines, days int) float64 {
	if len(klines) < 2 {
		return 0
	}
	start := len(klines) - days - 1
	if start < 0 {
		start = 0
	}
	base := klines[start].Close.Float64()
	latest := klines[len(klines)-1].Close.Float64()
	if base == 0 {
		return 0
	}
	return (latest - base) / base * 100
}

func klineYearRange(klines protocol.Klines, close float64) AgentKlineYearRange {
	used := limitKlines(klines, agentIndicatorWarmupBars)
	if len(used) == 0 {
		return AgentKlineYearRange{}
	}
	high := used[0].High.Float64()
	low := used[0].Low.Float64()
	for _, item := range used[1:] {
		high = math.Max(high, item.High.Float64())
		low = math.Min(low, item.Low.Float64())
	}
	result := AgentKlineYearRange{
		Available: true,
		StartDate: used[0].Time.Format(time.DateOnly),
		EndDate:   used[len(used)-1].Time.Format(time.DateOnly),
		High:      high,
		Low:       low,
	}
	if high > 0 {
		result.DistanceToHighPct = (close/high - 1) * 100
	}
	if low > 0 {
		result.DistanceToLowPct = (close/low - 1) * 100
	}
	return result
}

func klineVolumeSummary(klines protocol.Klines) AgentKlineVolumeSummary {
	if len(klines) == 0 {
		return AgentKlineVolumeSummary{}
	}
	latest := klines[len(klines)-1].Volume
	avg20 := averageKlineVolume(klines, 20)
	ratio := 0.0
	if avg20 > 0 {
		ratio = float64(latest) / avg20
	}
	return AgentKlineVolumeSummary{
		Latest:      latest,
		Avg5:        averageKlineVolume(klines, 5),
		Avg20:       avg20,
		VolumeRatio: ratio,
		Signal:      klineVolumeSignal(ratio),
	}
}

func averageKlineVolume(klines protocol.Klines, days int) float64 {
	if len(klines) == 0 {
		return 0
	}
	start := len(klines) - days
	if start < 0 {
		start = 0
	}
	sum := int64(0)
	for _, item := range klines[start:] {
		sum += item.Volume
	}
	return float64(sum) / float64(len(klines[start:]))
}

func klineVolumeSignal(ratio float64) string {
	switch {
	case ratio >= 2:
		return "显著放量"
	case ratio >= 1.2:
		return "温和放量"
	case ratio > 0 && ratio <= 0.7:
		return "缩量"
	case ratio > 0:
		return "量能平稳"
	default:
		return "量能不足"
	}
}

func klineKeyLevels(klines protocol.Klines, close float64) AgentKlineKeyLevels {
	high20, low20 := klineHighLow(klines, 20)
	high60, low60 := klineHighLow(klines, 60)
	high120, low120 := klineHighLow(klines, 120)
	return AgentKlineKeyLevels{
		High20:              high20,
		Low20:               low20,
		High60:              high60,
		Low60:               low60,
		High120:             high120,
		Low120:              low120,
		DistanceToHigh20Pct: distancePct(close, high20),
		DistanceToLow20Pct:  distancePct(close, low20),
	}
}

func klineHighLow(klines protocol.Klines, days int) (float64, float64) {
	if len(klines) == 0 {
		return 0, 0
	}
	start := len(klines) - days
	if start < 0 {
		start = 0
	}
	high := klines[start].High.Float64()
	low := klines[start].Low.Float64()
	for _, item := range klines[start:] {
		high = math.Max(high, item.High.Float64())
		low = math.Min(low, item.Low.Float64())
	}
	return high, low
}

func distancePct(close, level float64) float64 {
	if level == 0 {
		return 0
	}
	return (close - level) / level * 100
}

func buildKlineMASummary(klines protocol.Klines, close float64) AgentKlineMASummary {
	ma5 := optionalKlineMA(klines, 5)
	ma10 := optionalKlineMA(klines, 10)
	ma20 := optionalKlineMA(klines, 20)
	ma60 := optionalKlineMA(klines, 60)
	return AgentKlineMASummary{
		MA5:            ma5,
		MA10:           ma10,
		MA20:           ma20,
		MA60:           ma60,
		MA120:          optionalKlineMA(klines, 120),
		PriceVsMA20Pct: priceVsMAPct(close, ma20),
		Alignment:      klineMAAlignment(ma5, ma10, ma20, ma60),
	}
}

func priceVsMAPct(close float64, ma *float64) float64 {
	if ma == nil || *ma == 0 {
		return 0
	}
	return (close - *ma) / *ma * 100
}

func klineMAAlignment(ma5, ma10, ma20, ma60 *float64) string {
	if ma5 == nil || ma10 == nil || ma20 == nil || ma60 == nil {
		return "均线数据不足"
	}
	switch {
	case *ma5 >= *ma10 && *ma10 >= *ma20 && *ma20 >= *ma60:
		return "多头排列"
	case *ma5 <= *ma10 && *ma10 <= *ma20 && *ma20 <= *ma60:
		return "空头排列"
	default:
		return "均线交织"
	}
}

func klineCandleSummary(kline *protocol.Kline) AgentKlineCandleSummary {
	if kline == nil || kline.Open == 0 {
		return AgentKlineCandleSummary{}
	}
	open := kline.Open.Float64()
	close := kline.Close.Float64()
	high := kline.High.Float64()
	low := kline.Low.Float64()
	bodyPct := math.Abs(close-open) / open * 100
	upperShadowPct := (high - math.Max(open, close)) / open * 100
	lowerShadowPct := (math.Min(open, close) - low) / open * 100
	return AgentKlineCandleSummary{
		Shape:          klineCandleShape(bodyPct, upperShadowPct, lowerShadowPct, open, close),
		BodyPct:        bodyPct,
		UpperShadowPct: upperShadowPct,
		LowerShadowPct: lowerShadowPct,
		AmplitudePct:   (high - low) / open * 100,
	}
}

func klineCandleShape(
	bodyPct float64,
	upperShadowPct float64,
	lowerShadowPct float64,
	open float64,
	close float64,
) string {
	if upperShadowPct >= math.Max(bodyPct*1.5, 3) {
		return "long_upper_shadow"
	}
	if lowerShadowPct >= math.Max(bodyPct*1.5, 3) {
		return "long_lower_shadow"
	}
	if bodyPct <= 0.5 {
		return "doji"
	}
	if close > open {
		return "bullish"
	}
	if close < open {
		return "bearish"
	}
	return "flat"
}

func klineVolatilitySummary(klines protocol.Klines) AgentKlineVolatility {
	atr := klines.ATRFloat(14)
	close := 0.0
	if len(klines) > 0 {
		close = klines[len(klines)-1].Close.Float64()
	}
	atrPct := 0.0
	if close > 0 {
		atrPct = atr / close * 100
	}
	return AgentKlineVolatility{
		Atr:               atr,
		AtrPct:            atrPct,
		AvgAmplitude5Pct:  averageKlineAmplitudePct(klines, 5),
		AvgAmplitude20Pct: averageKlineAmplitudePct(klines, 20),
		Risk:              klineVolatilityRisk(atrPct),
	}
}

func averageKlineAmplitudePct(klines protocol.Klines, days int) float64 {
	if len(klines) == 0 {
		return 0
	}
	start := len(klines) - days
	if start < 0 {
		start = 0
	}
	sum := 0.0
	count := 0
	for _, item := range klines[start:] {
		close := item.Close.Float64()
		if close <= 0 {
			continue
		}
		sum += (item.High.Float64() - item.Low.Float64()) / close * 100
		count++
	}
	if count == 0 {
		return 0
	}
	return sum / float64(count)
}

func klineVolatilityRisk(atrPct float64) string {
	switch {
	case atrPct >= 8:
		return "高"
	case atrPct >= 4:
		return "中"
	case atrPct > 0:
		return "低"
	default:
		return "未知"
	}
}

func klineStreakSummary(klines protocol.Klines) AgentKlineStreak {
	if len(klines) < 2 {
		return AgentKlineStreak{Direction: "flat"}
	}
	lastDiff := klines[len(klines)-1].Close.Float64() - klines[len(klines)-2].Close.Float64()
	direction := "flat"
	if lastDiff > 0 {
		direction = "up"
	} else if lastDiff < 0 {
		direction = "down"
	}
	if direction == "flat" {
		return AgentKlineStreak{Direction: direction}
	}
	count := 0
	startClose := klines[len(klines)-1].Close.Float64()
	for i := len(klines) - 1; i > 0; i-- {
		diff := klines[i].Close.Float64() - klines[i-1].Close.Float64()
		if (direction == "up" && diff <= 0) || (direction == "down" && diff >= 0) {
			break
		}
		startClose = klines[i-1].Close.Float64()
		count++
	}
	latest := klines[len(klines)-1].Close.Float64()
	changePct := 0.0
	if startClose > 0 {
		changePct = (latest - startClose) / startClose * 100
	}
	return AgentKlineStreak{
		Direction: direction,
		Count:     count,
		ChangePct: changePct,
	}
}

func klineMaxDrawdownPct(klines protocol.Klines) float64 {
	if len(klines) == 0 {
		return 0
	}
	peak := klines[0].Close.Float64()
	maxDrawdown := 0.0
	for _, item := range klines {
		close := item.Close.Float64()
		if close > peak {
			peak = close
		}
		if peak > 0 {
			maxDrawdown = math.Min(maxDrawdown, (close-peak)/peak*100)
		}
	}
	return maxDrawdown
}

func klinePriceTrend(returns map[string]float64) string {
	ret20 := returns["ret20"]
	ret60 := returns["ret60"]
	switch {
	case ret20 >= 3 && ret60 >= 0:
		return "上行"
	case ret20 <= -3 && ret60 <= 0:
		return "下行"
	default:
		return "震荡"
	}
}

func klineRangePosition(close, high, low float64) string {
	if high <= low {
		return "区间位置不明"
	}
	ratio := (close - low) / (high - low)
	switch {
	case ratio >= 0.75:
		return "接近区间高位"
	case ratio <= 0.25:
		return "接近区间低位"
	default:
		return "位于区间中部"
	}
}

func klineSummarySignals(summary AgentKlinePeriodSummary, klines protocol.Klines) []string {
	signals := make([]string, 0)
	if summary.MaxDrawdownPct <= -20 {
		signals = append(signals, "区间最大回撤较大")
	}
	if summary.VolatilityPct >= 40 {
		signals = append(signals, "区间波动较大")
	}
	signals = append(signals, klinePatternSignals(summary, klines)...)
	return signals
}

func klinePatternSignals(summary AgentKlinePeriodSummary, klines protocol.Klines) []string {
	signals := make([]string, 0)
	if len(klines) >= 2 {
		prev := klines[len(klines)-2]
		latest := klines[len(klines)-1]
		if latest.Open.Float64() > prev.High.Float64() {
			signals = append(signals, "跳空高开")
		}
		if latest.Open.Float64() < prev.Low.Float64() {
			signals = append(signals, "跳空低开")
		}
	}
	switch summary.Candle.Shape {
	case "long_upper_shadow":
		signals = append(signals, "长上影线")
	case "long_lower_shadow":
		signals = append(signals, "长下影线")
	}
	if summary.Streak.Count >= 3 {
		signals = append(signals, fmt.Sprintf(
			"连续%s%d日",
			klineStreakDirectionText(summary.Streak.Direction),
			summary.Streak.Count,
		))
	}
	return signals
}

func klineStreakDirectionText(direction string) string {
	switch direction {
	case "up":
		return "上涨"
	case "down":
		return "下跌"
	default:
		return "平盘"
	}
}

func klineTrendStage(summary AgentKlinePeriodSummary) string {
	ret20 := summary.StageReturns["ret20"]
	ret60 := summary.StageReturns["ret60"]
	switch {
	case summary.Trend == "上行" && ret20 < -3:
		return "上升趋势中的回调"
	case summary.Trend == "上行" && ret20 >= 8:
		return "上升趋势加速"
	case summary.Trend == "上行":
		return "上升趋势延续"
	case summary.Trend == "下行" && ret20 > 3:
		return "下降趋势中的反弹"
	case summary.Trend == "下行":
		return "下降趋势延续"
	case ret60 >= 10 && ret20 < 0:
		return "中期上行后的震荡整理"
	case ret60 <= -10 && ret20 > 0:
		return "中期下行后的修复反弹"
	default:
		return "横盘震荡"
	}
}

func klineRiskLevel(summary AgentKlinePeriodSummary) string {
	score := 0
	if summary.MaxDrawdownPct <= -30 {
		score += 2
	} else if summary.MaxDrawdownPct <= -15 {
		score++
	}
	if summary.VolatilityPct >= 80 {
		score += 2
	} else if summary.VolatilityPct >= 40 {
		score++
	}
	switch {
	case score >= 4:
		return "高"
	case score >= 2:
		return "中"
	default:
		return "低"
	}
}

func klineNarrativeSummary(summary AgentKlinePeriodSummary) string {
	return fmt.Sprintf(
		"%s，%s，近20日涨跌幅%s，最大回撤%s",
		summary.TrendStage,
		summary.Position,
		formatPercentText(summary.StageReturns["ret20"]),
		formatPercentText(summary.MaxDrawdownPct),
	)
}

func buildAgentKlineSummaryText(summary AgentKlineSummary) string {
	var b strings.Builder
	if summary.Name != "" {
		b.WriteString(fmt.Sprintf("股票：%s（%s）\n\n", summary.Name, summary.Code))
	} else {
		b.WriteString(fmt.Sprintf("股票代码：%s\n\n", summary.Code))
	}
	b.WriteString("K线摘要：\n")
	appendAgentDayDataContextText(&b, summary.dayData)
	b.WriteString("说明：展示区间由level/dayCount控制；仅输出K线价格结构，不重复技术评分接口的指标。\n")
	for _, period := range summary.analysisPeriods {
		b.WriteString(fmt.Sprintf(
			"%s：样本 %d/%d，区间 %s 至 %s，收盘 %.2f，涨跌幅 %s，最高 %.2f，最低 %.2f，最大回撤 %s，波动区间 %s，趋势 %s，位置 %s",
			period.Name,
			period.UsedCount,
			period.TotalCount,
			period.StartDate,
			period.EndDate,
			period.Close,
			formatPercentText(period.ChangePct),
			period.High,
			period.Low,
			formatPercentText(period.MaxDrawdownPct),
			formatPercentText(period.VolatilityPct),
			period.Trend,
			period.Position,
		))
		if len(period.Signals) > 0 {
			b.WriteString("；信号：" + strings.Join(period.Signals, "、"))
		}
		b.WriteString(fmt.Sprintf(
			"；阶段：%s；风险：%s；近5/20/60涨跌：%s/%s/%s；形态：%s，上影线 %s；连续：%s%d日，区间涨跌 %s；距20日高点 %s，距20日低点 %s",
			period.TrendStage,
			period.RiskLevel,
			formatPercentText(period.StageReturns["ret5"]),
			formatPercentText(period.StageReturns["ret20"]),
			formatPercentText(period.StageReturns["ret60"]),
			klineCandleShapeText(period.Candle.Shape),
			formatPercentText(period.Candle.UpperShadowPct),
			klineStreakDirectionText(period.Streak.Direction),
			period.Streak.Count,
			formatPercentText(period.Streak.ChangePct),
			formatPercentText(period.KeyLevels.DistanceToHigh20Pct),
			formatPercentText(period.KeyLevels.DistanceToLow20Pct),
		))
		if period.YearRange.Available {
			b.WriteString(fmt.Sprintf(
				"；近52周区间 %.2f-%.2f（%s至%s），距高点 %s，距低点 %s",
				period.YearRange.Low,
				period.YearRange.High,
				period.YearRange.StartDate,
				period.YearRange.EndDate,
				formatPercentText(period.YearRange.DistanceToHighPct),
				formatPercentText(period.YearRange.DistanceToLowPct),
			))
		}
		b.WriteString("；摘要：" + period.Summary + "。\n")
	}
	if len(summary.Warnings) > 0 {
		b.WriteString("\n数据提示：\n")
		for _, warning := range summary.Warnings {
			if strings.TrimSpace(warning) == "" {
				continue
			}
			b.WriteString("- ")
			b.WriteString(warning)
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String())
}

func klineCandleShapeText(shape string) string {
	switch shape {
	case "long_upper_shadow":
		return "长上影线"
	case "long_lower_shadow":
		return "长下影线"
	case "doji":
		return "十字星"
	case "bullish":
		return "阳线"
	case "bearish":
		return "阴线"
	default:
		return "平盘"
	}
}

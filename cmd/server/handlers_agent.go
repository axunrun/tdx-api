package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/injoyai/tdx"
	"github.com/injoyai/tdx/protocol"
)

type AgentTechnicalSummary struct {
	Code     string                 `json:"code"`
	Source   string                 `json:"source"`
	Periods  []AgentTechnicalPeriod `json:"periods"`
	Limits   map[string]int         `json:"limits"`
	Note     string                 `json:"note"`
	Warnings []string               `json:"warnings,omitempty"`
	dayData  agentDayDataContext
	bullBear technicalScoreRow
}

type AgentTechnicalPeriod struct {
	Period      string            `json:"period"`
	Name        string            `json:"name"`
	KlineCount  int               `json:"klineCount"`
	LatestDate  string            `json:"latestDate"`
	Close       float64           `json:"close"`
	MA          map[string]Metric `json:"ma"`
	MACD        AgentMACD         `json:"macd"`
	RSI         map[string]Metric `json:"rsi"`
	BOLL        AgentBOLL         `json:"boll"`
	ATR         AgentATR          `json:"atr"`
	OBV         AgentOBV          `json:"obv"`
	Signals     []string          `json:"signals"`
	KDJ         AgentKDJ          `json:"-"`
	BIAS        AgentBIAS         `json:"-"`
	VolumePrice AgentVolumePrice  `json:"-"`
	Return20    float64           `json:"-"`
}

type AgentKDJ struct {
	Available bool
	K         float64
	D         float64
	J         float64
	Signal    string
	Score     int
	Reason    string
}

type AgentBIAS struct {
	Available bool
	BIAS5     float64
	BIAS10    float64
	Signal    string
	Score     int
	Reason    string
}

type AgentVolumePrice struct {
	Available   bool
	VolumeRatio float64
	ChangePct   float64
	Signal      string
	Score       int
	Reason      string
}

type Metric struct {
	Available bool     `json:"available"`
	Value     *float64 `json:"value,omitempty"`
	Reason    string   `json:"reason,omitempty"`
	Text      string   `json:"text,omitempty"`
}

type AgentMACD struct {
	Available bool     `json:"available"`
	DIF       *float64 `json:"dif,omitempty"`
	DEA       *float64 `json:"dea,omitempty"`
	Hist      *float64 `json:"hist,omitempty"`
	Signal    string   `json:"signal,omitempty"`
	Reason    string   `json:"reason,omitempty"`
}

type AgentBOLL struct {
	Available bool     `json:"available"`
	Upper     *float64 `json:"upper,omitempty"`
	Middle    *float64 `json:"middle,omitempty"`
	Lower     *float64 `json:"lower,omitempty"`
	Position  string   `json:"position,omitempty"`
	Reason    string   `json:"reason,omitempty"`
}

type AgentATR struct {
	Available bool     `json:"available"`
	ATR14     *float64 `json:"atr14,omitempty"`
	Usage     string   `json:"usage,omitempty"`
	Reason    string   `json:"reason,omitempty"`
}

type AgentOBV struct {
	Available   bool    `json:"available"`
	Latest      int64   `json:"latest,omitempty"`
	Change5Pct  float64 `json:"change5Pct,omitempty"`
	Change20Pct float64 `json:"change20Pct,omitempty"`
	Trend       string  `json:"trend,omitempty"`
	Divergence  string  `json:"divergence,omitempty"`
	Signal      string  `json:"signal,omitempty"`
	Reason      string  `json:"reason,omitempty"`
}

type agentTechnicalSpec struct {
	period string
	name   string
	count  uint16
	fetch  func(string, uint16) (*protocol.KlineResp, error)
}

const agentIndicatorWarmupBars = 250

type agentDayDataContext struct {
	QueryTime time.Time
	DataDate  string
	Status    string
	Adjust    string
}

func indicatorWarmupCount(requested int) int {
	return max(requested, agentIndicatorWarmupBars)
}

func buildAgentDayDataContext(
	periods []AgentTechnicalPeriod,
	adjust string,
	now time.Time,
) agentDayDataContext {
	context := agentDayDataContext{
		QueryTime: now,
		DataDate:  "不可用",
		Status:    "日线K线不可用",
		Adjust:    adjust,
	}
	for _, period := range periods {
		if period.Period != "day" || period.LatestDate == "" {
			continue
		}
		latest, err := time.ParseInLocation(time.DateOnly, period.LatestDate, now.Location())
		if err != nil {
			context.Status = "日线数据日期无法解析"
			return context
		}
		context.DataDate = period.LatestDate
		context.Status = technicalScoreDayDataStatus(latest, now)
		return context
	}
	return context
}

func agentDayAdjustText(adjust string) string {
	if adjust == "qfq" {
		return "前复权（qfq）"
	}
	return "未复权（none）"
}

type AgentStockBrief struct {
	Code         string                  `json:"code"`
	Name         string                  `json:"name,omitempty"`
	Source       string                  `json:"source"`
	Quote        *AgentBriefQuote        `json:"quote,omitempty"`
	Finance      *AgentBriefFinance      `json:"finance,omitempty"`
	LatestReport *AgentBriefLatestReport `json:"latestReport,omitempty"`
	Blocks       []AgentBriefBlock       `json:"blocks"`
	Stat         *AgentBriefStat         `json:"stat,omitempty"`
	Moneyflow    *AgentBriefMoneyflow    `json:"moneyflow,omitempty"`
	Technical    *AgentTechnicalSummary  `json:"technical,omitempty"`
	Limits       map[string]int          `json:"limits"`
	Note         string                  `json:"note"`
	Warnings     []string                `json:"warnings,omitempty"`
}

type AgentBriefQuote struct {
	Code         string  `json:"code"`
	Market       string  `json:"market"`
	Price        float64 `json:"price"`
	LastClose    float64 `json:"lastClose"`
	Open         float64 `json:"open"`
	High         float64 `json:"high"`
	Low          float64 `json:"low"`
	ChangePct    float64 `json:"changePct"`
	AmplitudePct float64 `json:"amplitudePct"`
	TurnoverRate float64 `json:"turnoverRate,omitempty"`
	Volume       int64   `json:"volume"`
	Amount       float64 `json:"amount"`
	AmountText   string  `json:"amountText"`
	Text         string  `json:"text"`
	QueryTime    string  `json:"queryTime"`
	DataDate     string  `json:"dataDate"`
	DataStatus   string  `json:"dataStatus"`
}

type AgentBriefFinance struct {
	UpdatedDate           string  `json:"updatedDate"`
	IPODate               string  `json:"ipoDate"`
	TotalShares           float64 `json:"totalShares"`
	TotalSharesText       string  `json:"totalSharesText"`
	FloatShares           float64 `json:"floatShares"`
	FloatSharesText       string  `json:"floatSharesText"`
	TotalMarketValue      float64 `json:"totalMarketValue,omitempty"`
	TotalMarketValueText  string  `json:"totalMarketValueText,omitempty"`
	FloatMarketValue      float64 `json:"floatMarketValue,omitempty"`
	FloatMarketValueText  string  `json:"floatMarketValueText,omitempty"`
	TotalAssets           float64 `json:"totalAssets"`
	TotalAssetsText       string  `json:"totalAssetsText"`
	NetAssets             float64 `json:"netAssets"`
	NetAssetsText         string  `json:"netAssetsText"`
	MainRevenue           float64 `json:"mainRevenue"`
	MainRevenueText       string  `json:"mainRevenueText"`
	MainProfit            float64 `json:"mainProfit"`
	MainProfitText        string  `json:"mainProfitText"`
	OperatingProfit       float64 `json:"operatingProfit"`
	OperatingProfitText   string  `json:"operatingProfitText"`
	NetProfit             float64 `json:"netProfit"`
	NetProfitText         string  `json:"netProfitText"`
	OperatingCashflow     float64 `json:"operatingCashflow"`
	OperatingCashflowText string  `json:"operatingCashflowText"`
	Shareholders          float64 `json:"shareholders"`
	Meaning               string  `json:"meaning"`
}

type AgentBriefBlock struct {
	Type        string `json:"type"`
	TypeName    string `json:"typeName"`
	Name        string `json:"name"`
	IndexCode   string `json:"indexCode,omitempty"`
	MemberCount int    `json:"memberCount"`
	Meaning     string `json:"meaning"`
}

type AgentBriefStat struct {
	Date      string  `json:"date"`
	PETTM     float64 `json:"peTtm"`
	PEStatic  float64 `json:"peStatic"`
	PB        float64 `json:"pb,omitempty"`
	DivYield  float64 `json:"divYield"`
	ChangePct float64 `json:"changePct"`
	TrendDays int     `json:"trendDays"`
	Chg5      float64 `json:"chg5"`
	Chg10     float64 `json:"chg10"`
	Chg20     float64 `json:"chg20"`
	Chg60     float64 `json:"chg60"`
	ChgYTD    float64 `json:"chgYtd"`
	Meaning   string  `json:"meaning"`
}

type AgentBriefMoneyflow struct {
	Date             string  `json:"date"`
	BlockIndex       string  `json:"blockIndex,omitempty"`
	Amount           float64 `json:"amount"`
	AmountPrev       float64 `json:"amountPrev"`
	AmountChangePct  float64 `json:"amountChangePct,omitempty"`
	AmountChangeText string  `json:"amountChangeText,omitempty"`
	IPOPrice         float64 `json:"ipoPrice"`
	High52W          float64 `json:"high52w"`
	Low52W           float64 `json:"low52w"`
	AmountMeaning    string  `json:"amountMeaning"`
}

var (
	agentStatCache  *agentTTLCache[[]*protocol.TdxStat]
	agentStat2Cache *agentTTLCache[[]*protocol.TdxStat2]
)

func handleAgentTechnicalSummary(w http.ResponseWriter, r *http.Request) {
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

	summary, err := buildAgentTechnicalSummary(c, code)
	if err != nil {
		jsonErr(w, err.Error())
		return
	}
	jsonResp(w, summary)
}

func handleAgentStockBrief(w http.ResponseWriter, r *http.Request) {
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

	brief, err := buildAgentStockBrief(c, code, r.URL.Query().Get("mkt"))
	if err != nil {
		jsonErr(w, err.Error())
		return
	}
	jsonResp(w, brief)
}

func handleAgentStockBriefText(w http.ResponseWriter, r *http.Request) {
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

	brief, err := buildAgentStockBriefTextData(
		c,
		code,
		r.URL.Query().Get("mkt"),
	)
	if err != nil {
		jsonErr(w, err.Error())
		return
	}
	jsonResp(w, AgentStockBriefText{
		Code:    code,
		Format:  "text/plain; charset=utf-8",
		Content: buildAgentStockBriefText(brief),
	})
}

func buildAgentStockBrief(c *tdx.Client, code, rawMarket string) (AgentStockBrief, error) {
	return buildAgentStockBriefAt(c, code, rawMarket, "none", time.Now(), true)
}

func buildAgentStockBriefTextData(
	c *tdx.Client,
	code string,
	rawMarket string,
) (AgentStockBrief, error) {
	return buildAgentStockBriefAt(c, code, rawMarket, "none", time.Now(), false)
}

func buildAgentStockBriefAt(
	c *tdx.Client,
	code string,
	rawMarket string,
	adjust string,
	now time.Time,
	includeTechnical bool,
) (AgentStockBrief, error) {
	warnings := make([]string, 0)
	brief := AgentStockBrief{
		Code:   code,
		Name:   queryStockName(code),
		Source: "tdx_agent_stock_brief",
		Blocks: make([]AgentBriefBlock, 0),
		Limits: map[string]int{
			"technicalDay":   agentIndicatorWarmupBars,
			"technicalWeek":  agentIndicatorWarmupBars,
			"technicalMonth": agentIndicatorWarmupBars,
		},
		Note: "面向Agent的单股概览聚合接口；板块返回该股完整所属板块摘要；技术指标使用各周期最多250根可用K线预热并只返回最新有效值。RSI和ATR使用Wilder平滑；OBV反映量价确认。",
	}

	if quote, err := buildAgentBriefQuote(c, code, now); err != nil {
		warnings = append(warnings, "GetQuote失败: "+err.Error())
	} else {
		brief.Quote = quote
	}
	if finance, err := buildAgentBriefFinance(c, code, rawMarket); err != nil {
		warnings = append(warnings, "GetFinanceInfo失败: "+err.Error())
	} else {
		brief.Finance = finance
	}
	if latestReport, err := buildAgentBriefLatestReport(c, code, rawMarket); err != nil {
		warnings = append(warnings, "F10最新提示失败: "+err.Error())
	} else {
		brief.LatestReport = latestReport
	}
	blocks, blockWarnings := buildAgentBriefBlocks(c, code, 0)
	brief.Blocks = blocks
	warnings = append(warnings, blockWarnings...)

	if stat, err := buildAgentBriefStat(c, code); err != nil {
		warnings = append(warnings, "GetTdxStat失败: "+err.Error())
	} else {
		brief.Stat = stat
	}
	if moneyflow, err := buildAgentBriefMoneyflow(c, code); err != nil {
		warnings = append(warnings, "GetTdxStat2失败: "+err.Error())
	} else {
		brief.Moneyflow = moneyflow
	}
	if includeTechnical {
		if technical, err := buildAgentTechnicalSummaryAt(c, code, adjust, now); err != nil {
			warnings = append(warnings, "technical-summary失败: "+err.Error())
		} else {
			brief.Technical = &technical
		}
	}
	enrichAgentStockBriefMetrics(&brief)

	if brief.Quote == nil && brief.Finance == nil && brief.LatestReport == nil &&
		brief.Stat == nil {
		return brief, fmt.Errorf(strings.Join(warnings, "; "))
	}
	brief.Warnings = warnings
	return brief, nil
}
func buildAgentTechnicalSummary(c *tdx.Client, code string) (AgentTechnicalSummary, error) {
	return buildAgentTechnicalSummaryAt(c, code, "none", time.Now())
}

func buildAgentTechnicalSummaryWithAdjust(
	c *tdx.Client,
	code string,
	rawAdjust string,
) (AgentTechnicalSummary, error) {
	adjust, err := normalizeTechnicalAdjust(rawAdjust)
	if err != nil {
		return AgentTechnicalSummary{}, err
	}
	return buildAgentTechnicalSummaryAt(c, code, adjust, time.Now())
}

func buildAgentTechnicalSummaryAt(
	c *tdx.Client,
	code string,
	adjust string,
	now time.Time,
) (AgentTechnicalSummary, error) {
	specs := []agentTechnicalSpec{
		{"day", "日线", 250, func(code string, count uint16) (*protocol.KlineResp, error) {
			return fetchTechnicalDayKlines(c, code, count, adjust)
		}},
		{"week", "周线", agentIndicatorWarmupBars, func(code string, count uint16) (*protocol.KlineResp, error) {
			return fetchWeekKlines(c, code, count)
		}},
		{"month", "月线", agentIndicatorWarmupBars, func(code string, count uint16) (*protocol.KlineResp, error) {
			return fetchMonthKlines(c, code, count)
		}},
	}
	periods, warnings, err := buildAgentTechnicalSummaryFromSpecs(code, specs)
	if err != nil {
		return AgentTechnicalSummary{}, err
	}

	return AgentTechnicalSummary{
		Code:    code,
		Source:  "tdx_kline_local_indicators",
		Periods: periods,
		Limits: map[string]int{
			"day":   agentIndicatorWarmupBars,
			"week":  agentIndicatorWarmupBars,
			"month": agentIndicatorWarmupBars,
		},
		Note:     "技术指标由tdx K线在本地计算，各周期最多使用250根可用K线预热，仅返回最后一个有效值；available=false表示K线不足。RSI和ATR使用Wilder平滑；OBV反映量价确认。",
		Warnings: warnings,
		dayData:  buildAgentDayDataContext(periods, adjust, now),
	}, nil
}

func buildAgentTechnicalSummaryFromSpecs(
	code string,
	specs []agentTechnicalSpec,
) ([]AgentTechnicalPeriod, []string, error) {
	periods := make([]AgentTechnicalPeriod, 0, len(specs))
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
		periods = append(periods, buildAgentTechnicalPeriod(
			spec.period,
			spec.name,
			protocol.Klines(resp.List),
		))
	}
	if len(periods) == 0 {
		if len(warnings) > 0 {
			return nil, warnings, fmt.Errorf(strings.Join(warnings, "; "))
		}
		return nil, warnings, fmt.Errorf("无K线数据")
	}
	return periods, warnings, nil
}

func buildAgentBriefQuote(c *tdx.Client, code string, queryTimes ...time.Time) (*AgentBriefQuote, error) {
	now := time.Now()
	if len(queryTimes) > 0 {
		now = queryTimes[0]
	}
	quotes, err := c.GetQuote(code)
	if err != nil {
		return nil, err
	}
	if len(quotes) == 0 || quotes[0] == nil || quotes[0].Kline == nil {
		return nil, fmt.Errorf("无行情数据")
	}
	quote := quotes[0]
	kline := quote.Kline
	lastClose := kline.Last.Float64()
	price := kline.Close.Float64()
	changePct := 0.0
	amplitudePct := 0.0
	if lastClose != 0 {
		changePct = (price - lastClose) / lastClose * 100
		amplitudePct = (kline.High.Float64() - kline.Low.Float64()) / lastClose * 100
	}
	return &AgentBriefQuote{
		Code:         quote.Code,
		Market:       marketName(uint8(quote.Exchange)),
		Price:        price,
		LastClose:    lastClose,
		Open:         kline.Open.Float64(),
		High:         kline.High.Float64(),
		Low:          kline.Low.Float64(),
		ChangePct:    changePct,
		AmplitudePct: amplitudePct,
		Volume:       kline.Volume,
		Amount:       kline.Amount.Float64(),
		AmountText:   formatCNYText(kline.Amount.Float64()),
		Text:         fmt.Sprintf("现价%.2f，涨跌幅%.2f%%，成交额%s", price, changePct, formatCNYText(kline.Amount.Float64())),
		QueryTime:    now.Format(time.RFC3339),
		DataDate:     quoteKlineDataDate(kline, now),
		DataStatus:   quoteKlineDataStatus(kline, now),
	}, nil
}

func quoteKlineDataDate(kline *protocol.Kline, now time.Time) string {
	if kline == nil || kline.Time.IsZero() {
		return "不可用"
	}
	if resolveSectorRealtimeSession(now).text == "非交易日" {
		t := kline.Time
		for i := 0; i < 7; i++ {
			t = t.AddDate(0, 0, -1)
			if resolveSectorRealtimeSession(t).text != "非交易日" {
				return t.Format(time.DateOnly)
			}
		}
	}
	return kline.Time.Format(time.DateOnly)
}

func quoteKlineDataStatus(kline *protocol.Kline, now time.Time) string {
	if kline == nil || kline.Time.IsZero() {
		return "最新可得行情，数据日期不可用"
	}
	latestDate := dateOnly(kline.Time)
	today := dateOnly(now)
	if latestDate.Before(today) {
		return "最近完整交易日收盘行情"
	}
	if latestDate.After(today) {
		return "数据日期晚于系统日期，请检查服务器时间"
	}
	if resolveSectorRealtimeSession(now).text == "非交易日" {
		return "最近完整交易日收盘行情"
	}
	switch resolveSectorRealtimeSession(now).status {
	case "trading":
		return "盘中实时行情，数据随交易更新"
	case "break":
		return "午间休市最后行情，截止上午收盘"
	case "preopen":
		return "盘前最新行情，若含集合竞价则为动态值"
	default:
		return "当日收盘行情"
	}
}
func buildAgentBriefFinance(c *tdx.Client, code, rawMarket string) (*AgentBriefFinance, error) {
	finance, err := c.GetFinanceInfo(exchangeForCode(code, rawMarket), code)
	if err != nil {
		return nil, err
	}
	if finance == nil {
		return nil, fmt.Errorf("无财务数据")
	}
	totalAssets := normalizeAgentFinanceBalanceValue(finance.ZongZiChan)
	netAssets := normalizeAgentFinanceBalanceValue(finance.JingZiChan)
	mainRevenue := normalizeAgentFinanceBalanceValue(finance.ZhuYingShouRu)
	mainProfit := normalizeAgentFinanceBalanceValue(finance.ZhuYingLiRun)
	operatingProfit := normalizeAgentFinanceBalanceValue(finance.YingYeLiRun)
	netProfit := normalizeAgentFinanceBalanceValue(finance.JingLiRun)
	operatingCashflow := normalizeAgentFinanceBalanceValue(finance.JingYingXianJinLiu)
	return &AgentBriefFinance{
		UpdatedDate:           formatTdxDate(finance.UpdatedDate),
		IPODate:               formatTdxDate(finance.IPODate),
		TotalShares:           finance.ZongGuBen,
		TotalSharesText:       formatShareText(finance.ZongGuBen),
		FloatShares:           finance.LiuTongGuBen,
		FloatSharesText:       formatShareText(finance.LiuTongGuBen),
		TotalAssets:           totalAssets,
		TotalAssetsText:       formatCNYText(totalAssets),
		NetAssets:             netAssets,
		NetAssetsText:         formatCNYText(netAssets),
		MainRevenue:           mainRevenue,
		MainRevenueText:       formatCNYText(mainRevenue),
		MainProfit:            mainProfit,
		MainProfitText:        formatCNYText(mainProfit),
		OperatingProfit:       operatingProfit,
		OperatingProfitText:   formatCNYText(operatingProfit),
		NetProfit:             netProfit,
		NetProfitText:         formatCNYText(netProfit),
		OperatingCashflow:     operatingCashflow,
		OperatingCashflowText: formatCNYText(operatingCashflow),
		Shareholders:          finance.GuDongRenShu,
		Meaning:               "股本单位为股，资产、收入、利润和现金流单位为元；用于快速判断公司规模、盈利和现金流质量。",
	}, nil
}

func normalizeAgentFinanceBalanceValue(value float64) float64 {
	// GetFinanceInfo 的金额字段在当前解码口径下比行情软件展示口径大 10 倍。
	// Agent 层先归一化为元，避免 PB、财务规模等派生指标被低估或放大。
	return value / 10
}

func buildAgentBriefBlocks(c *tdx.Client, code string, limitPerType int) ([]AgentBriefBlock, []string) {
	if blocks, err := queryStockBlocks(code); err == nil {
		return applyBlockLimit(blocks, limitPerType), nil
	}

	blocks := make([]AgentBriefBlock, 0)
	warnings := make([]string, 0)
	for _, spec := range blockIndexSpecs {
		items, err := c.GetBlockDataWithIndex(spec.file)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("GetBlockDataWithIndex(%s)失败: %v", spec.file, err))
			continue
		}
		n := 0
		for _, item := range items {
			if item == nil || !blockContainsCode(item.Codes, code) {
				continue
			}
			blocks = append(blocks, AgentBriefBlock{
				Type:        spec.typ,
				TypeName:    spec.typeName,
				Name:        item.Name,
				IndexCode:   item.Index,
				MemberCount: len(item.Codes),
				Meaning:     "该股所属板块摘要，可用于后续抓取板块指数、板块成分股强弱和题材归因。",
			})
			n++
			if limitPerType > 0 && n >= limitPerType {
				break
			}
		}
	}
	return blocks, warnings
}

func applyBlockLimit(blocks []AgentBriefBlock, limitPerType int) []AgentBriefBlock {
	if limitPerType <= 0 {
		return blocks
	}
	countByType := make(map[string]int)
	limited := make([]AgentBriefBlock, 0, len(blocks))
	for _, block := range blocks {
		if countByType[block.Type] >= limitPerType {
			continue
		}
		countByType[block.Type]++
		limited = append(limited, block)
	}
	return limited
}

func buildAgentBriefStat(c *tdx.Client, code string) (*AgentBriefStat, error) {
	stats, err := getCachedAgentStats(c)
	if err != nil {
		return nil, err
	}
	for _, stat := range stats {
		if stat != nil && stat.Code == code {
			return &AgentBriefStat{
				Date:      stat.Date,
				PETTM:     stat.PETTM,
				PEStatic:  stat.PEStatic,
				DivYield:  stat.DivYield,
				ChangePct: stat.ChangePct,
				TrendDays: stat.TrendDays,
				Chg5:      stat.Chg5,
				Chg10:     stat.Chg10,
				Chg20:     stat.Chg20,
				Chg60:     stat.Chg60,
				ChgYTD:    stat.ChgYTD,
				Meaning:   "盘后个股综合统计，适合做估值、阶段涨跌幅和连续涨跌状态的快速判断。",
			}, nil
		}
	}
	return nil, fmt.Errorf("未找到统计数据")
}

func buildAgentBriefMoneyflow(c *tdx.Client, code string) (*AgentBriefMoneyflow, error) {
	stats, err := getCachedAgentStats2(c)
	if err != nil {
		return nil, err
	}
	for _, stat := range stats {
		if stat != nil && stat.Code == code {
			return &AgentBriefMoneyflow{
				Date:          stat.Date,
				BlockIndex:    stat.BlockIndex,
				Amount:        stat.Amount,
				AmountPrev:    stat.AmountPrev,
				IPOPrice:      stat.IPOPrice,
				High52W:       stat.High52W,
				Low52W:        stat.Low52W,
				AmountMeaning: "Amount和AmountPrev单位为万元，用于比较今日/昨日成交活跃度；BlockIndex为通达信板块指数代码。",
			}, nil
		}
	}
	return nil, fmt.Errorf("未找到资金/扩展统计数据")
}

func getCachedAgentStats(c *tdx.Client) ([]*protocol.TdxStat, error) {
	if agentStatCache == nil {
		agentStatCache = newAgentTTLCache(func() ([]*protocol.TdxStat, error) {
			return c.GetTdxStat()
		}, agentStatsTTL())
	}
	return agentStatCache.Get()
}

func getCachedAgentStats2(c *tdx.Client) ([]*protocol.TdxStat2, error) {
	if agentStat2Cache == nil {
		agentStat2Cache = newAgentTTLCache(func() ([]*protocol.TdxStat2, error) {
			return c.GetTdxStat2()
		}, agentStatsTTL())
	}
	return agentStat2Cache.Get()
}

func agentStatsTTL() time.Duration {
	now := time.Now()
	hour, minute, _ := now.Clock()
	minutes := hour*60 + minute
	if minutes >= 9*60+15 && minutes <= 15*60+30 {
		return 30 * time.Second
	}
	return 10 * time.Minute
}

func enrichAgentStockBriefMetrics(brief *AgentStockBrief) {
	if brief == nil || brief.Quote == nil {
		return
	}
	if brief.Finance != nil {
		enrichAgentBriefMarketValue(brief.Quote, brief.Finance)
		enrichAgentBriefTurnover(brief.Code, brief.Quote, brief.Finance)
		enrichAgentBriefPB(brief.Quote, brief.Finance, brief.Stat, brief.LatestReport)
	}
	if brief.Moneyflow != nil {
		enrichAgentBriefAmountChange(brief.Moneyflow)
	}
}

func enrichAgentBriefMarketValue(quote *AgentBriefQuote, finance *AgentBriefFinance) {
	if quote.Price <= 0 || finance == nil {
		return
	}
	if finance.TotalShares > 0 {
		finance.TotalMarketValue = quote.Price * finance.TotalShares
		finance.TotalMarketValueText = formatCNYText(finance.TotalMarketValue)
	}
	if finance.FloatShares > 0 {
		finance.FloatMarketValue = quote.Price * finance.FloatShares
		finance.FloatMarketValueText = formatCNYText(finance.FloatMarketValue)
	}
}

func enrichAgentBriefTurnover(code string, quote *AgentBriefQuote, finance *AgentBriefFinance) {
	if quote == nil || quote.Volume <= 0 {
		return
	}
	volumeShares := quote.Volume * 100
	if g := getGbbq(); g != nil {
		if rate := g.GetTurnover(code, time.Now(), volumeShares); rate > 0 {
			quote.TurnoverRate = rate
			return
		}
	}
	if finance != nil && finance.FloatShares > 0 {
		quote.TurnoverRate = float64(volumeShares) / finance.FloatShares * 100
	}
}

func enrichAgentBriefPB(
	quote *AgentBriefQuote,
	finance *AgentBriefFinance,
	stat *AgentBriefStat,
	latestReport *AgentBriefLatestReport,
) {
	if quote == nil || stat == nil {
		return
	}
	if latestReport != nil && latestReport.NetAssetPerShare > 0 {
		stat.PB = quote.Price / latestReport.NetAssetPerShare
		return
	}
	if finance == nil {
		return
	}
	if finance.TotalShares <= 0 || finance.NetAssets <= 0 {
		return
	}
	bookValuePerShare := finance.NetAssets / finance.TotalShares
	if bookValuePerShare > 0 {
		stat.PB = quote.Price / bookValuePerShare
	}
}

func enrichAgentBriefAmountChange(moneyflow *AgentBriefMoneyflow) {
	if moneyflow == nil || moneyflow.AmountPrev == 0 {
		return
	}
	change := moneyflow.Amount - moneyflow.AmountPrev
	moneyflow.AmountChangePct = change / moneyflow.AmountPrev * 100
	moneyflow.AmountChangeText = formatCNYText(change * 10000)
}

func exchangeForCode(code, rawMarket string) protocol.Exchange {
	if rawMarket != "" {
		return parseExchange(rawMarket)
	}
	switch {
	case strings.HasPrefix(code, "0"), strings.HasPrefix(code, "3"):
		return protocol.ExchangeSZ
	case strings.HasPrefix(code, "4"), strings.HasPrefix(code, "8"), strings.HasPrefix(code, "9"):
		return protocol.ExchangeBJ
	default:
		return protocol.ExchangeSH
	}
}

func marketName(market uint8) string {
	switch market {
	case 0:
		return "深市"
	case 1:
		return "沪市"
	case 2:
		return "北交所"
	default:
		return fmt.Sprintf("未知市场%d", market)
	}
}

func formatTdxDate(date uint32) string {
	if date == 0 {
		return ""
	}
	s := fmt.Sprintf("%08d", date)
	if len(s) != 8 {
		return s
	}
	return fmt.Sprintf("%s-%s-%s", s[:4], s[4:6], s[6:8])
}

func formatCNYText(value float64) string {
	switch {
	case value >= 1e8 || value <= -1e8:
		return fmt.Sprintf("%.2f亿元", value/1e8)
	case value >= 1e4 || value <= -1e4:
		return fmt.Sprintf("%.2f万元", value/1e4)
	default:
		return fmt.Sprintf("%.2f元", value)
	}
}

func formatShareText(value float64) string {
	switch {
	case value >= 1e8 || value <= -1e8:
		return fmt.Sprintf("%.2f亿股", value/1e8)
	case value >= 1e4 || value <= -1e4:
		return fmt.Sprintf("%.2f万股", value/1e4)
	default:
		return fmt.Sprintf("%.0f股", value)
	}
}

func blockContainsCode(codes []string, code string) bool {
	for _, item := range codes {
		if item == code || strings.HasSuffix(item, code) {
			return true
		}
	}
	return false
}

func fetchDayKlines(c *tdx.Client, code string, count uint16) (*protocol.KlineResp, error) {
	return c.GetKlineDay(code, 0, count)
}

func fetchWeekKlines(c *tdx.Client, code string, count uint16) (*protocol.KlineResp, error) {
	return c.GetKlineWeek(code, 0, count)
}

func fetchMonthKlines(c *tdx.Client, code string, count uint16) (*protocol.KlineResp, error) {
	return c.GetKlineMonth(code, 0, count)
}

func buildAgentTechnicalPeriod(period, name string, ks protocol.Klines) AgentTechnicalPeriod {
	latest := ks[len(ks)-1]
	ma := map[string]Metric{
		"ma5":   priceMetric(ks, 5, ks.MA(5), "MA5"),
		"ma10":  priceMetric(ks, 10, ks.MA(10), "MA10"),
		"ma20":  priceMetric(ks, 20, ks.MA(20), "MA20"),
		"ma60":  priceMetric(ks, 60, ks.MA(60), "MA60"),
		"ma120": priceMetric(ks, 120, ks.MA(120), "MA120"),
	}
	macd := buildMACD(ks)
	boll := buildBOLL(ks, latest.Close)
	atr := buildATR(ks)
	obv := buildOBV(ks)
	kdj := buildKDJ(ks)
	bias := buildBIAS(ks)
	volumePrice := buildVolumePrice(ks)

	return AgentTechnicalPeriod{
		Period:     period,
		Name:       name,
		KlineCount: len(ks),
		LatestDate: latest.Time.Format("2006-01-02"),
		Close:      latest.Close.Float64(),
		MA:         ma,
		MACD:       macd,
		RSI: map[string]Metric{
			"rsi6":  rsiMetric(ks, 6, "RSI6"),
			"rsi12": rsiMetric(ks, 12, "RSI12"),
			"rsi24": rsiMetric(ks, 24, "RSI24"),
		},
		BOLL:        boll,
		ATR:         atr,
		OBV:         obv,
		KDJ:         kdj,
		BIAS:        bias,
		VolumePrice: volumePrice,
		Return20:    klineReturnPct(ks, 20),
		Signals:     technicalSignals(latest.Close, ma, macd, boll, obv),
	}
}

func buildKDJ(ks protocol.Klines) AgentKDJ {
	if len(ks) < 9 {
		return AgentKDJ{Reason: "K线不足9根"}
	}
	k, d, j, ok := ks.KDJ(9)
	if !ok {
		return AgentKDJ{Reason: "K线不足9根"}
	}
	signal, score := kdjSignal(k, d, k, d)
	if len(ks) > 1 {
		if previousK, previousD, _, available := ks[:len(ks)-1].KDJ(9); available {
			signal, score = kdjSignal(previousK, previousD, k, d)
		}
	}
	return AgentKDJ{
		Available: true,
		K:         k,
		D:         d,
		J:         j,
		Signal:    signal,
		Score:     score,
	}
}

func buildBIAS(ks protocol.Klines) AgentBIAS {
	if len(ks) < 10 {
		return AgentBIAS{Reason: "K线不足10根"}
	}
	closePrice := ks[len(ks)-1].Close.Float64()
	ma5 := ks.MA(5).Float64()
	ma10 := ks.MA(10).Float64()
	if ma5 == 0 || ma10 == 0 {
		return AgentBIAS{Reason: "均线不足"}
	}
	bias5 := (closePrice/ma5 - 1) * 100
	bias10 := (closePrice/ma10 - 1) * 100
	score := 0
	signal := "正常"
	if bias5 < -5 && bias10 < -8 {
		score = 1
		signal = "负乖离过大"
	} else if bias5 > 5 && bias10 > 8 {
		score = -1
		signal = "正乖离过大"
	}
	return AgentBIAS{
		Available: true,
		BIAS5:     bias5,
		BIAS10:    bias10,
		Signal:    signal,
		Score:     score,
	}
}

func buildVolumePrice(ks protocol.Klines) AgentVolumePrice {
	if len(ks) < 5 {
		return AgentVolumePrice{Reason: "K线不足5根"}
	}
	latest := ks[len(ks)-1]
	avg5 := averageKlineVolume(ks, 5)
	if avg5 <= 0 {
		return AgentVolumePrice{Reason: "量能不足"}
	}
	ratio := float64(latest.Volume) / avg5
	changePct := latest.RiseRate()
	score := 0
	signal := "平量"
	if ratio > 1.5 && changePct > 0 {
		score = 1
		signal = "放量上涨"
	} else if ratio > 1.5 && changePct < 0 {
		score = -1
		signal = "放量下跌"
	} else if ratio < 0.7 {
		signal = "缩量"
	}
	return AgentVolumePrice{
		Available:   true,
		VolumeRatio: ratio,
		ChangePct:   changePct,
		Signal:      signal,
		Score:       score,
	}
}

func priceMetric(ks protocol.Klines, required int, value protocol.Price, label string) Metric {
	if len(ks) < required {
		return unavailableMetric(fmt.Sprintf("K线数量不足%d根", required))
	}
	v := value.Float64()
	return Metric{Available: true, Value: &v, Text: fmt.Sprintf("%s=%.3f", label, v)}
}

func rsiMetric(ks protocol.Klines, days int, label string) Metric {
	if len(ks) <= days {
		return unavailableMetric(fmt.Sprintf("K线数量不足%d根", days+1))
	}
	v := ks.RSIFloat(days)
	return Metric{Available: true, Value: &v, Text: fmt.Sprintf("%s=%.2f", label, v)}
}

func unavailableMetric(reason string) Metric {
	return Metric{Available: false, Reason: reason}
}

func buildMACD(ks protocol.Klines) AgentMACD {
	if len(ks) < 35 {
		return AgentMACD{Available: false, Reason: "K线数量不足35根"}
	}
	dif, dea, hist := ks.MACD()
	difValue := dif.Float64()
	deaValue := dea.Float64()
	histValue := hist.Float64()
	return AgentMACD{
		Available: true,
		DIF:       &difValue,
		DEA:       &deaValue,
		Hist:      &histValue,
		Signal:    macdSignal(hist),
	}
}

func buildBOLL(ks protocol.Klines, close protocol.Price) AgentBOLL {
	if len(ks) < 20 {
		return AgentBOLL{Available: false, Reason: "K线数量不足20根"}
	}
	upper, middle, lower := ks.BOLL(20)
	upperValue := upper.Float64()
	middleValue := middle.Float64()
	lowerValue := lower.Float64()
	return AgentBOLL{
		Available: true,
		Upper:     &upperValue,
		Middle:    &middleValue,
		Lower:     &lowerValue,
		Position:  bollPosition(close, upper, middle, lower),
	}
}

func buildATR(ks protocol.Klines) AgentATR {
	if len(ks) < 15 {
		return AgentATR{Available: false, Reason: "K线数量不足15根"}
	}
	atr14 := ks.ATRFloat(14)
	return AgentATR{
		Available: true,
		ATR14:     &atr14,
		Usage:     "衡量近期波动，不直接代表方向。",
	}
}

func buildOBV(ks protocol.Klines) AgentOBV {
	if len(ks) < 21 {
		return AgentOBV{Available: false, Reason: "K线数量不足21根"}
	}
	values := make([]int64, len(ks))
	for i := 1; i < len(ks); i++ {
		values[i] = values[i-1]
		switch {
		case ks[i].Close > ks[i-1].Close:
			values[i] += ks[i].Volume
		case ks[i].Close < ks[i-1].Close:
			values[i] -= ks[i].Volume
		}
	}
	latest := values[len(values)-1]
	change5 := obvWindowChangePct(values, ks, 5)
	change20 := obvWindowChangePct(values, ks, 20)
	trend := obvTrend(change20)
	divergence := obvDivergence(values, ks, 20)
	return AgentOBV{
		Available:   true,
		Latest:      latest,
		Change5Pct:  change5,
		Change20Pct: change20,
		Trend:       trend,
		Divergence:  divergence,
		Signal:      obvSignal(trend, divergence, change20, change5),
	}
}

func obvWindowChangePct(values []int64, ks protocol.Klines, days int) float64 {
	if len(values) <= days || days <= 0 {
		return 0
	}
	start := len(values) - 1 - days
	delta := float64(values[len(values)-1] - values[start])
	volume := int64(0)
	for _, item := range ks[start+1:] {
		volume += item.Volume
	}
	if volume == 0 {
		return 0
	}
	return delta / float64(volume) * 100
}

func obvTrend(change20 float64) string {
	switch {
	case change20 >= 10:
		return "up"
	case change20 <= -10:
		return "down"
	default:
		return "flat"
	}
}

func obvDivergence(values []int64, ks protocol.Klines, days int) string {
	if len(values) <= days || len(ks) <= days {
		return "none"
	}
	start := len(values) - 1 - days
	latestClose := ks[len(ks)-1].Close
	latestOBV := values[len(values)-1]
	priorHighClose, priorLowClose := ks[start].Close, ks[start].Close
	priorHighOBV, priorLowOBV := values[start], values[start]
	for i := start; i < len(values)-1; i++ {
		if ks[i].Close > priorHighClose {
			priorHighClose = ks[i].Close
		}
		if ks[i].Close < priorLowClose {
			priorLowClose = ks[i].Close
		}
		if values[i] > priorHighOBV {
			priorHighOBV = values[i]
		}
		if values[i] < priorLowOBV {
			priorLowOBV = values[i]
		}
	}
	switch {
	case latestClose >= priorHighClose && latestOBV < priorHighOBV:
		return "bearish"
	case latestClose <= priorLowClose && latestOBV > priorLowOBV:
		return "bullish"
	default:
		return "none"
	}
}

func obvSignal(trend, divergence string, change20, change5 float64) string {
	switch divergence {
	case "bearish":
		return fmt.Sprintf(
			"OBV：近20根K线价格创新高但OBV未同步创新高，存在顶背离迹象。（OBV20净变化占窗口成交量%s，OBV5为%s）",
			formatPercentText(change20),
			formatPercentText(change5),
		)
	case "bullish":
		return fmt.Sprintf(
			"OBV：近20根K线价格创新低但OBV未同步创新低，存在底背离迹象。（OBV20净变化占窗口成交量%s，OBV5为%s）",
			formatPercentText(change20),
			formatPercentText(change5),
		)
	}
	switch trend {
	case "up":
		return fmt.Sprintf(
			"OBV：近20根K线净变化为正，量能方向偏强；未见明显顶背离。（OBV20净变化占窗口成交量%s，OBV5为%s）",
			formatPercentText(change20),
			formatPercentText(change5),
		)
	case "down":
		return fmt.Sprintf(
			"OBV：近20根K线净变化为负，量能方向偏弱；未见明显底背离。（OBV20净变化占窗口成交量%s，OBV5为%s）",
			formatPercentText(change20),
			formatPercentText(change5),
		)
	default:
		return fmt.Sprintf(
			"OBV：近20根K线净变化不明显，量能方向偏震荡；未见明显背离。（OBV20净变化占窗口成交量%s，OBV5为%s）",
			formatPercentText(change20),
			formatPercentText(change5),
		)
	}
}

func macdSignal(hist protocol.Price) string {
	switch {
	case hist > 0:
		return "MACD柱为正，多头动能占优"
	case hist < 0:
		return "MACD柱为负，空头动能占优"
	default:
		return "MACD动能中性"
	}
}

func bollPosition(close, upper, middle, lower protocol.Price) string {
	switch {
	case close >= upper:
		return "价格位于布林线上轨附近或上方"
	case close >= middle:
		return "价格位于布林线中轨上方"
	case close <= lower:
		return "价格位于布林线下轨附近或下方"
	default:
		return "价格位于布林线中轨下方"
	}
}

func technicalSignals(
	close protocol.Price,
	ma map[string]Metric,
	macd AgentMACD,
	boll AgentBOLL,
	obv AgentOBV,
) []string {
	signals := make([]string, 0, 5)
	closeValue := close.Float64()

	if ma20, ok := metricValue(ma["ma20"]); ok {
		if closeValue >= ma20 {
			signals = append(signals, "价格在MA20上方")
		} else {
			signals = append(signals, "价格在MA20下方")
		}
	}
	if ma60, ok := metricValue(ma["ma60"]); ok {
		if closeValue >= ma60 {
			signals = append(signals, "价格在MA60上方")
		} else {
			signals = append(signals, "价格在MA60下方")
		}
	}
	if macd.Available && macd.Signal != "" {
		signals = append(signals, macd.Signal)
	}
	if boll.Available && boll.Position != "" {
		signals = append(signals, boll.Position)
	}
	if obv.Available && obv.Signal != "" {
		signals = append(signals, obv.Signal)
	}
	return signals
}

func metricValue(metric Metric) (float64, bool) {
	if !metric.Available || metric.Value == nil {
		return 0, false
	}
	return *metric.Value, true
}

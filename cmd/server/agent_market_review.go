package main

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/injoyai/tdx"
	"github.com/injoyai/tdx/protocol"
)

type AgentMarketReview struct {
	Source                 string                 `json:"source"`
	Session                string                 `json:"session"`
	ReviewType             string                 `json:"reviewType"`
	GeneratedAt            string                 `json:"generatedAt"`
	Indexes                []AgentMarketIndex     `json:"indexes"`
	CurrentBreadth         AgentMarketBreadth     `json:"currentBreadth"`
	LatestCompletedBreadth AgentMarketBreadth     `json:"latestCompletedBreadth"`
	Hotspots               *AgentMarketHotspots   `json:"hotspots,omitempty"`
	Watchlist              []AgentMarketWatchItem `json:"watchlist,omitempty"`
	Limits                 map[string]int         `json:"limits"`
	Warnings               []string               `json:"warnings,omitempty"`
	Note                   string                 `json:"note"`
}

type AgentMarketIndex struct {
	Code      string  `json:"code"`
	Name      string  `json:"name"`
	Date      string  `json:"date,omitempty"`
	Close     float64 `json:"close,omitempty"`
	LastClose float64 `json:"lastClose,omitempty"`
	ChangePct float64 `json:"changePct,omitempty"`
}

type AgentMarketBreadth struct {
	Available        bool    `json:"available"`
	Complete         bool    `json:"complete"`
	DataType         string  `json:"dataType"`
	Date             string  `json:"date,omitempty"`
	AsOf             string  `json:"asOf,omitempty"`
	Universe         string  `json:"universe"`
	UniverseTotal    int     `json:"universeTotal"`
	ValidCount       int     `json:"validCount"`
	UnavailableCount int     `json:"unavailableCount"`
	Total            int     `json:"total"`
	Rising           int     `json:"rising"`
	Falling          int     `json:"falling"`
	Flat             int     `json:"flat"`
	LimitUp          int     `json:"limitUp"`
	LimitDown        int     `json:"limitDown"`
	RisingPct        float64 `json:"risingPct"`
	AverageChange    float64 `json:"averageChange"`
	MedianChange     float64 `json:"medianChange"`
	Source           string  `json:"source"`
	SourceNote       string  `json:"sourceNote"`
}

type AgentMarketHotspots struct {
	Strong []AgentHotspotSector `json:"strong"`
	Middle []AgentHotspotSector `json:"middle"`
	Weak   []AgentHotspotSector `json:"weak"`
}

type AgentMarketWatchItem struct {
	Code      string  `json:"code"`
	Name      string  `json:"name,omitempty"`
	ChangePct float64 `json:"changePct,omitempty"`
	Chg20     float64 `json:"chg20,omitempty"`
	Text      string  `json:"text,omitempty"`
}

type AgentMarketReviewText struct {
	Format  string `json:"format"`
	Content string `json:"content"`
}

type marketIndexSpec struct {
	code string
	name string
}

var defaultMarketIndexes = []marketIndexSpec{
	{code: "sh000001", name: "上证指数"},
	{code: "sz399001", name: "深证成指"},
	{code: "sz399006", name: "创业板指"},
	{code: "sh000688", name: "科创50"},
	{code: "bj899050", name: "北证50"},
}

func handleAgentMarketReview(w http.ResponseWriter, r *http.Request) {
	summary, ok := loadAgentMarketReview(w, r)
	if !ok {
		return
	}
	jsonResp(w, summary)
}

func handleAgentMarketReviewText(w http.ResponseWriter, r *http.Request) {
	summary, ok := loadAgentMarketReview(w, r)
	if !ok {
		return
	}
	jsonResp(w, AgentMarketReviewText{
		Format:  "text/plain; charset=utf-8",
		Content: buildAgentMarketReviewText(summary),
	})
}

func loadAgentMarketReview(w http.ResponseWriter, r *http.Request) (AgentMarketReview, bool) {
	session := strings.TrimSpace(r.URL.Query().Get("session"))
	if session == "" {
		session = "auto"
	}
	top := parseCount(r.URL.Query().Get("top"), 5)
	if top <= 0 || top > 5 {
		top = 5
	}
	codes := parseAgentCodeList(r)
	if len(codes) > 20 {
		codes = codes[:20]
	}
	c := cli()
	if c == nil {
		jsonErr(w, "TDX客户端未连接")
		return AgentMarketReview{}, false
	}
	stats, err := getCachedAgentStats(c)
	if err != nil {
		jsonErr(w, "GetTdxStat失败: "+err.Error())
		return AgentMarketReview{}, false
	}
	now := time.Now()
	return buildAgentMarketReview(c, stats, codes, session, top, now), true
}

func buildAgentMarketReview(
	c *tdx.Client,
	stats []*protocol.TdxStat,
	codes []string,
	session string,
	top int,
	now time.Time,
) AgentMarketReview {
	reviewType := resolveMarketReviewType(session, now)
	warnings := make([]string, 0)
	indexes, indexWarnings := buildMarketIndexes(c)
	warnings = append(warnings, indexWarnings...)
	currentBreadth, breadthWarnings := loadCurrentMarketBreadth(c, indexes, now)
	warnings = append(warnings, breadthWarnings...)
	completedBreadth := buildLatestCompletedMarketBreadth(stats)
	hotspots, hotspotWarnings := buildMarketHotspots(stats, top)
	warnings = append(warnings, hotspotWarnings...)
	return AgentMarketReview{
		Source:                 "tdx_agent_market_review",
		Session:                session,
		ReviewType:             reviewType,
		GeneratedAt:            now.Format(time.RFC3339),
		Indexes:                indexes,
		CurrentBreadth:         currentBreadth,
		LatestCompletedBreadth: completedBreadth,
		Hotspots:               hotspots,
		Watchlist:              buildMarketWatchItems(stats, codes),
		Limits: map[string]int{
			"hotspotTop": top,
			"watchlist":  20,
		},
		Warnings: warnings,
		Note:     "当前广度与最近完整盘后广度分开输出；所有数据均携带实际日期和时点。",
	}
}

func resolveMarketReviewType(session string, now time.Time) string {
	if now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
		return "non_trading"
	}
	if session != "auto" && session != "" {
		return session
	}
	minute := now.Hour()*60 + now.Minute()
	switch {
	case minute < 9*60+20:
		return "preopen"
	case minute < 9*60+25:
		return "call_auction"
	case minute < 9*60+30:
		return "preopen_after_auction"
	case minute < 11*60+30:
		return "current"
	case minute < 13*60:
		return "morning"
	case minute < 15*60:
		return "current_with_morning_reference"
	default:
		return "full"
	}
}

func buildMarketIndexes(c *tdx.Client) ([]AgentMarketIndex, []string) {
	items := make([]AgentMarketIndex, 0, len(defaultMarketIndexes))
	warnings := make([]string, 0)
	for _, spec := range defaultMarketIndexes {
		resp, err := c.GetIndexDay(spec.code, 0, 2)
		if err != nil || resp == nil || len(resp.List) == 0 {
			warnings = append(warnings, spec.name+"指数K线获取失败")
			continue
		}
		kline := latestCompletedMarketIndexKline(resp.List)
		if kline == nil {
			warnings = append(warnings, spec.name+"指数无有效K线")
			continue
		}
		lastClose := kline.Last.Float64()
		changePct := 0.0
		if lastClose > 0 {
			changePct = (kline.Close.Float64() - lastClose) / lastClose * 100
		}
		items = append(items, AgentMarketIndex{
			Code:      spec.code,
			Name:      spec.name,
			Date:      kline.Time.Format("2006-01-02"),
			Close:     kline.Close.Float64(),
			LastClose: lastClose,
			ChangePct: changePct,
		})
	}
	return items, warnings
}

func latestCompletedMarketIndexKline(klines []*protocol.Kline) *protocol.Kline {
	for i := len(klines) - 1; i >= 0; i-- {
		if klines[i] != nil && klines[i].Close.Float64() > 0 && klines[i].Volume > 0 {
			return klines[i]
		}
	}
	return nil
}

func medianFloat64(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	cp := append([]float64(nil), values...)
	sort.Float64s(cp)
	mid := len(cp) / 2
	if len(cp)%2 == 1 {
		return cp[mid]
	}
	return (cp[mid-1] + cp[mid]) / 2
}

func buildMarketHotspots(stats []*protocol.TdxStat, top int) (*AgentMarketHotspots, []string) {
	sectors, err := querySectorMemberSets("concept")
	if err != nil {
		return nil, []string{"热点板块获取失败: " + err.Error()}
	}
	summary := buildAgentHotspotScan(sectors, stats, "chg20", top, 3, 20, true)
	return &AgentMarketHotspots{
		Strong: summary.Sectors,
		Middle: summary.MiddleSectors,
		Weak:   summary.ColdSectors,
	}, summary.Warnings
}

func buildMarketWatchItems(stats []*protocol.TdxStat, codes []string) []AgentMarketWatchItem {
	if len(codes) == 0 {
		return nil
	}
	byCode := make(map[string]*protocol.TdxStat, len(stats))
	for _, stat := range stats {
		if stat != nil {
			byCode[stat.Code] = stat
		}
	}
	items := make([]AgentMarketWatchItem, 0, len(codes))
	for _, code := range codes {
		item := AgentMarketWatchItem{Code: code, Name: queryStockName(code)}
		if stat := byCode[code]; stat != nil {
			item.ChangePct = stat.ChangePct
			item.Chg20 = stat.Chg20
			item.Text = fmt.Sprintf(
				"%s%s，盘后统计(%s)单日%s，20日%s",
				item.Name,
				code,
				formatTdxStatDate(stat.Date),
				formatPercentText(item.ChangePct),
				formatPercentText(item.Chg20),
			)
		} else {
			item.Text = item.Name + code + "暂无TdxStat快照"
		}
		items = append(items, item)
	}
	return items
}

func buildAgentMarketReviewText(summary AgentMarketReview) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("市场复盘：%s。\n", marketReviewTypeText(summary.ReviewType)))
	if len(summary.Indexes) > 0 {
		parts := make([]string, 0, len(summary.Indexes))
		for _, index := range summary.Indexes {
			parts = append(parts, fmt.Sprintf("%s%s", index.Name, formatPercentText(index.ChangePct)))
		}
		indexDate := latestMarketIndexDate(summary.Indexes)
		label := "最近指数"
		if indexDate != "" && strings.HasPrefix(summary.GeneratedAt, indexDate) {
			label = "今日指数"
		}
		b.WriteString(fmt.Sprintf("%s（%s）：%s。\n", label, indexDate, strings.Join(parts, "，")))
	}
	appendCurrentMarketBreadthText(&b, summary.CurrentBreadth, summary.ReviewType)
	appendCompletedMarketBreadthText(
		&b,
		summary.LatestCompletedBreadth,
		summary.CurrentBreadth.Date,
	)
	if summary.Hotspots != nil {
		top := summary.Limits["hotspotTop"]
		if top <= 0 || top > 5 {
			top = 5
		}
		if summary.LatestCompletedBreadth.Date != "" {
			b.WriteString(
				"板块统计基准日：" +
					summary.LatestCompletedBreadth.Date +
					"（20日区间指标）。\n",
			)
		}
		b.WriteString("强势板块：" + marketSectorNames(summary.Hotspots.Strong, top) + "。\n")
		b.WriteString("中游板块：" + marketSectorNames(summary.Hotspots.Middle, top) + "。\n")
		b.WriteString("弱势板块：" + marketSectorNames(summary.Hotspots.Weak, top) + "。\n")
	}
	if len(summary.Watchlist) > 0 {
		parts := make([]string, 0, len(summary.Watchlist))
		for _, item := range summary.Watchlist {
			parts = append(parts, item.Text)
		}
		b.WriteString("关注股联动：" + strings.Join(parts, "；") + "。\n")
	}
	b.WriteString("结论提示：该接口提供市场环境快照；个股细节请接续调用 stock-brief。\n")
	appendWarningsText(&b, summary.Warnings)
	return strings.TrimSpace(b.String())
}

func marketReviewTypeText(reviewType string) string {
	switch reviewType {
	case "non_trading":
		return "非交易日市场背景"
	case "preopen":
		return "开盘前市场背景"
	case "call_auction":
		return "09:20-09:25开盘集合竞价"
	case "preopen_after_auction":
		return "集合竞价结束、等待连续竞价"
	case "morning":
		return "上午收盘复盘"
	case "current_with_morning_reference":
		return "午后盘中状态"
	case "full":
		return "全天收盘复盘"
	default:
		return "盘中当前状态"
	}
}

func appendCurrentMarketBreadthText(
	b *strings.Builder,
	breadth AgentMarketBreadth,
	reviewType string,
) {
	if !breadth.Available {
		if reviewType == "preopen" || reviewType == "call_auction" ||
			reviewType == "preopen_after_auction" {
			b.WriteString("当前市场广度：盘前尚未产生；最近完整盘后广度见下文。\n")
			return
		}
		b.WriteString(fmt.Sprintf("当前市场广度：不可用（%s）。\n", breadth.SourceNote))
		return
	}
	b.WriteString(fmt.Sprintf(
		"当前市场广度（%s，截至%s）：有效%d/%d只，上涨%d家，下跌%d家，平盘%d家，"+
			"上涨占比%s；涨停约%d家，跌停约%d家；平均涨跌%s，中位数%s。\n",
		breadth.Date,
		breadth.AsOf,
		breadth.ValidCount,
		breadth.UniverseTotal,
		breadth.Rising,
		breadth.Falling,
		breadth.Flat,
		formatPercentText(breadth.RisingPct),
		breadth.LimitUp,
		breadth.LimitDown,
		formatPercentText(breadth.AverageChange),
		formatPercentText(breadth.MedianChange),
	))
}

func appendCompletedMarketBreadthText(
	b *strings.Builder,
	breadth AgentMarketBreadth,
	currentDate string,
) {
	if !breadth.Available {
		b.WriteString("最近完整盘后广度：不可用。\n")
		return
	}
	label := "最近完整盘后广度"
	if currentDate == "" || breadth.Date < currentDate {
		label = "上一交易日盘后广度"
	}
	b.WriteString(fmt.Sprintf(
		"%s（%s）：A股%d只，上涨%d家，下跌%d家，平盘%d家，上涨占比%s；"+
			"平均涨跌%s，中位数%s。\n",
		label,
		breadth.Date,
		breadth.ValidCount,
		breadth.Rising,
		breadth.Falling,
		breadth.Flat,
		formatPercentText(breadth.RisingPct),
		formatPercentText(breadth.AverageChange),
		formatPercentText(breadth.MedianChange),
	))
}

func marketSectorNames(sectors []AgentHotspotSector, limit int) string {
	if len(sectors) == 0 {
		return "暂无"
	}
	if limit > len(sectors) {
		limit = len(sectors)
	}
	parts := make([]string, 0, limit)
	for _, sector := range sectors[:limit] {
		parts = append(parts, fmt.Sprintf("%s%s", sector.Name, formatPercentText(sector.AverageValue)))
	}
	return strings.Join(parts, "、")
}

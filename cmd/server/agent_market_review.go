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
	Source      string                 `json:"source"`
	Session     string                 `json:"session"`
	ReviewType  string                 `json:"reviewType"`
	GeneratedAt string                 `json:"generatedAt"`
	Indexes     []AgentMarketIndex     `json:"indexes"`
	Breadth     AgentMarketBreadth     `json:"breadth"`
	Hotspots    *AgentMarketHotspots   `json:"hotspots,omitempty"`
	Watchlist   []AgentMarketWatchItem `json:"watchlist,omitempty"`
	Limits      map[string]int         `json:"limits"`
	Warnings    []string               `json:"warnings,omitempty"`
	Note        string                 `json:"note"`
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
	Total         int     `json:"total"`
	Rising        int     `json:"rising"`
	Falling       int     `json:"falling"`
	Flat          int     `json:"flat"`
	LimitUp       int     `json:"limitUp"`
	LimitDown     int     `json:"limitDown"`
	RisingPct     float64 `json:"risingPct"`
	AverageChange float64 `json:"averageChange"`
	MedianChange  float64 `json:"medianChange"`
	Source        string  `json:"source"`
	SourceNote    string  `json:"sourceNote"`
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
	top := parseCount(r.URL.Query().Get("top"), 10)
	if top <= 0 || top > 20 {
		top = 10
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
	hotspots, hotspotWarnings := buildMarketHotspots(stats, top)
	warnings = append(warnings, hotspotWarnings...)
	return AgentMarketReview{
		Source:      "tdx_agent_market_review",
		Session:     session,
		ReviewType:  reviewType,
		GeneratedAt: now.Format(time.RFC3339),
		Indexes:     indexes,
		Breadth:     buildMarketBreadth(stats),
		Hotspots:    hotspots,
		Watchlist:   buildMarketWatchItems(stats, codes),
		Limits: map[string]int{
			"hotspotTop": top,
			"watchlist":  20,
		},
		Warnings: warnings,
		Note:     "市场级复盘接口；市场广度为查询时点快照，上午历史广度需后续增加定时缓存后才能精确回放。",
	}
}

func resolveMarketReviewType(session string, now time.Time) string {
	if session != "auto" && session != "" {
		return session
	}
	minute := now.Hour()*60 + now.Minute()
	switch {
	case minute < 9*60+30:
		return "preopen"
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
		kline := resp.List[len(resp.List)-1]
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

func buildMarketBreadth(stats []*protocol.TdxStat) AgentMarketBreadth {
	values := make([]float64, 0, len(stats))
	breadth := AgentMarketBreadth{
		Source:     "GetTdxStat",
		SourceNote: "市场广度使用TdxStat查询时点快照；涨停/跌停按±9.9%近似统计，不区分ST、北交所和20cm品种。",
	}
	sum := 0.0
	for _, stat := range stats {
		if stat == nil || stat.Code == "" {
			continue
		}
		change := stat.ChangePct
		breadth.Total++
		sum += change
		values = append(values, change)
		switch {
		case change > 0:
			breadth.Rising++
		case change < 0:
			breadth.Falling++
		default:
			breadth.Flat++
		}
		if change >= 9.9 {
			breadth.LimitUp++
		}
		if change <= -9.9 {
			breadth.LimitDown++
		}
	}
	if breadth.Total > 0 {
		breadth.RisingPct = float64(breadth.Rising) / float64(breadth.Total) * 100
		breadth.AverageChange = sum / float64(breadth.Total)
		breadth.MedianChange = medianFloat64(values)
	}
	return breadth
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
				"%s%s，当日%s，20日%s",
				item.Name,
				code,
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
		b.WriteString("主要指数：" + strings.Join(parts, "，") + "。\n")
	}
	b.WriteString(fmt.Sprintf(
		"市场广度：上涨%d家，下跌%d家，平盘%d家，上涨占比%s；涨停约%d家，跌停约%d家；平均涨跌%s，中位数%s。\n",
		summary.Breadth.Rising,
		summary.Breadth.Falling,
		summary.Breadth.Flat,
		formatPercentText(summary.Breadth.RisingPct),
		summary.Breadth.LimitUp,
		summary.Breadth.LimitDown,
		formatPercentText(summary.Breadth.AverageChange),
		formatPercentText(summary.Breadth.MedianChange),
	))
	if summary.Hotspots != nil {
		b.WriteString("强势板块：" + marketSectorNames(summary.Hotspots.Strong, 5) + "。\n")
		b.WriteString("中游板块：" + marketSectorNames(summary.Hotspots.Middle, 5) + "。\n")
		b.WriteString("弱势板块：" + marketSectorNames(summary.Hotspots.Weak, 5) + "。\n")
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
	case "preopen":
		return "开盘前市场背景"
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

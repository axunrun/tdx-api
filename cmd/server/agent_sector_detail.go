package main

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/injoyai/tdx/protocol"
)

type AgentSectorDetail struct {
	Source              string                   `json:"source"`
	GeneratedAt         string                   `json:"generatedAt"`
	ConstituentDataDate string                   `json:"constituentDataDate,omitempty"`
	Sector              AgentBriefBlock          `json:"sector"`
	Metric              string                   `json:"metric"`
	MemberSize          int                      `json:"memberSize"`
	Stats               AgentSectorDetailStats   `json:"stats"`
	ExcludeNew          bool                     `json:"excludeNew"`
	ExcludedCount       int                      `json:"excludedCount,omitempty"`
	TopStocks           []AgentStockInSectorItem `json:"topStocks"`
	MidStocks           []AgentStockInSectorItem `json:"midStocks"`
	WeakStocks          []AgentStockInSectorItem `json:"weakStocks"`
	Warnings            []string                 `json:"warnings,omitempty"`
	Note                string                   `json:"note"`
}

type AgentSectorDetailStats struct {
	RisingCount       int      `json:"risingCount"`
	FallingCount      int      `json:"fallingCount"`
	RisingPct         float64  `json:"risingPct"`
	AverageValue      float64  `json:"averageValue"`
	DailyReturn       *float64 `json:"dailyReturn,omitempty"`
	DailyBaseDate     string   `json:"dailyBaseDate,omitempty"`
	DailyDate         string   `json:"dailyDate,omitempty"`
	Return20          float64  `json:"return20,omitempty"`
	Return20StartDate string   `json:"return20StartDate,omitempty"`
	Return20EndDate   string   `json:"return20EndDate,omitempty"`
	Return60          float64  `json:"return60,omitempty"`
	Return60StartDate string   `json:"return60StartDate,omitempty"`
	Return60EndDate   string   `json:"return60EndDate,omitempty"`
	IndexKlineOK      bool     `json:"indexKlineOk"`
}

type AgentSectorDetailText struct {
	Format  string `json:"format"`
	Content string `json:"content"`
}

func handleAgentSectorDetail(w http.ResponseWriter, r *http.Request) {
	summary, ok := loadAgentSectorDetail(w, r)
	if !ok {
		return
	}
	jsonResp(w, summary)
}

func handleAgentSectorDetailText(w http.ResponseWriter, r *http.Request) {
	summary, ok := loadAgentSectorDetail(w, r)
	if !ok {
		return
	}
	jsonResp(w, AgentSectorDetailText{
		Format:  "text/plain; charset=utf-8",
		Content: buildAgentSectorDetailText(summary),
	})
}

func loadAgentSectorDetail(w http.ResponseWriter, r *http.Request) (AgentSectorDetail, bool) {
	sectorName := strings.TrimSpace(r.URL.Query().Get("sectorName"))
	indexCode := strings.TrimSpace(r.URL.Query().Get("indexCode"))
	if sectorName == "" && indexCode == "" {
		jsonErr(w, "缺少sectorName或indexCode")
		return AgentSectorDetail{}, false
	}
	sectorType := strings.TrimSpace(r.URL.Query().Get("sectorType"))
	if sectorType == "" {
		sectorType = "concept"
	}
	metric := strings.TrimSpace(r.URL.Query().Get("metric"))
	if metric == "" {
		metric = "chg20"
	}
	limit := parseCount(r.URL.Query().Get("topStocks"), 10)
	if limit <= 0 || limit > 30 {
		limit = 10
	}
	excludeNew := parseHotspotExcludeNew(r.URL.Query().Get("excludeNew"))

	sector, err := findAgentSectorMemberSet(sectorType, sectorName, indexCode)
	if err != nil {
		jsonErr(w, err.Error())
		return AgentSectorDetail{}, false
	}
	c := cli()
	if c == nil {
		jsonErr(w, "TDX客户端未连接")
		return AgentSectorDetail{}, false
	}
	stats, err := getCachedAgentStats(c)
	if err != nil {
		jsonErr(w, "GetTdxStat失败: "+err.Error())
		return AgentSectorDetail{}, false
	}
	return buildAgentSectorDetail(c, sector, stats, metric, limit, excludeNew), true
}

func findAgentSectorMemberSet(
	sectorType string,
	sectorName string,
	indexCode string,
) (agentSectorMemberSet, error) {
	sectors, err := querySectorMemberSets(sectorType)
	if err != nil {
		return agentSectorMemberSet{}, err
	}
	for _, sector := range sectors {
		if sectorName != "" && sector.Block.Name != sectorName {
			continue
		}
		if indexCode != "" && sector.Block.IndexCode != indexCode {
			continue
		}
		return sector, nil
	}
	if indexCode != "" {
		return agentSectorMemberSet{}, fmt.Errorf("未找到板块指数代码%s", indexCode)
	}
	return agentSectorMemberSet{}, fmt.Errorf("未找到板块%s", sectorName)
}

func buildAgentSectorDetail(
	c interface {
		GetIndexDayAll(string) (*protocol.KlineResp, error)
	},
	sector agentSectorMemberSet,
	stats []*protocol.TdxStat,
	metric string,
	limit int,
	excludeNew bool,
) AgentSectorDetail {
	return buildAgentSectorDetailAt(c, sector, stats, metric, limit, excludeNew, time.Now())
}

func buildAgentSectorDetailAt(
	c interface {
		GetIndexDayAll(string) (*protocol.KlineResp, error)
	},
	sector agentSectorMemberSet,
	stats []*protocol.TdxStat,
	metric string,
	limit int,
	excludeNew bool,
	now time.Time,
) AgentSectorDetail {
	latestStatDate := latestHotspotStatDate(stats)
	statByCode := make(map[string]*protocol.TdxStat, len(stats))
	for _, stat := range stats {
		if stat != nil && (latestStatDate == "" || stat.Date == latestStatDate) {
			statByCode[stat.Code] = stat
		}
	}
	stocks, rising, falling, excluded := sectorDetailStockItems(sector, statByCode, metric, excludeNew)
	sort.Slice(stocks, func(i, j int) bool {
		return stocks[i].Value > stocks[j].Value
	})
	for i := range stocks {
		stocks[i].Rank = i + 1
		stocks[i].Percentile = float64(len(stocks)-i) / float64(len(stocks)) * 100
	}
	top := limitStockInSectorItems(stocks, limit)
	weak := append([]AgentStockInSectorItem(nil), stocks...)
	sort.Slice(weak, func(i, j int) bool {
		return weak[i].Value < weak[j].Value
	})
	weak = limitStockInSectorItems(weak, limit)
	mid := middleSectorStocks(top, weak, sector, statByCode, metric, limit, excludeNew)
	average := 0.0
	for _, item := range stocks {
		average += item.Value
	}
	if len(stocks) > 0 {
		average /= float64(len(stocks))
	}
	statsSummary := AgentSectorDetailStats{
		RisingCount:  rising,
		FallingCount: falling,
		RisingPct:    percentOf(float64(rising), float64(len(stocks))),
		AverageValue: average,
	}
	warnings := []string(nil)
	if sector.Block.IndexCode != "" {
		klines, ok := loadSectorDetailIndexKlines(c, sector)
		if ok {
			statsSummary.IndexKlineOK = true
			dailyReturn, baseDate, dailyDate, dailyOK :=
				hotspotCompletedDailyReturn(klines, now)
			if dailyOK {
				statsSummary.DailyReturn = &dailyReturn
				statsSummary.DailyBaseDate = baseDate
				statsSummary.DailyDate = dailyDate
				statsSummary.Return20,
					statsSummary.Return20StartDate,
					statsSummary.Return20EndDate,
					_ = hotspotWindowReturnUntilDateWithDates(klines, 20, dailyDate)
				statsSummary.Return60,
					statsSummary.Return60StartDate,
					statsSummary.Return60EndDate,
					_ = hotspotWindowReturnUntilDateWithDates(klines, 60, dailyDate)
			} else {
				warnings = append(warnings, "板块指数K线不足，无法计算最近完整交易日涨跌幅")
			}
		} else {
			warnings = append(warnings, "板块指数K线不可用，已仅返回成分股统计")
		}
	}
	return AgentSectorDetail{
		Source:              "tdx_agent_sector_detail",
		GeneratedAt:         now.Format(time.RFC3339),
		ConstituentDataDate: formatTdxStatDate(latestStatDate),
		Sector:              sector.Block,
		Metric:              metric,
		MemberSize:          len(stocks),
		Stats:               statsSummary,
		ExcludeNew:          excludeNew,
		ExcludedCount:       excluded,
		TopStocks:           top,
		MidStocks:           mid,
		WeakStocks:          weak,
		Warnings:            warnings,
		Note:                "板块深度接口；成分股使用最新TdxStat完整统计日，板块指数单日及20/60日收益均截至最近完整交易日。",
	}
}

func sectorDetailStockItems(
	sector agentSectorMemberSet,
	statByCode map[string]*protocol.TdxStat,
	metric string,
	excludeNew bool,
) ([]AgentStockInSectorItem, int, int, int) {
	items := make([]AgentStockInSectorItem, 0, len(sector.Members))
	rising := 0
	falling := 0
	excluded := 0
	for _, member := range sector.Members {
		stat := statByCode[member.Code]
		if stat == nil {
			continue
		}
		item := stockInSectorItem(stat, member.Name, metric)
		if excludeNew && isSectorDetailExcludedStock(item, metric) {
			excluded++
			continue
		}
		if item.ChangePct > 0 {
			rising++
		}
		if item.ChangePct < 0 {
			falling++
		}
		items = append(items, item)
	}
	return items, rising, falling, excluded
}

func loadSectorDetailIndexKlines(
	c interface {
		GetIndexDayAll(string) (*protocol.KlineResp, error)
	},
	sector agentSectorMemberSet,
) (protocol.Klines, bool) {
	if sector.Block.IndexCode == "" {
		return nil, false
	}
	resp, err := c.GetIndexDayAll("sh" + sector.Block.IndexCode)
	if err != nil || resp == nil {
		return nil, false
	}
	return cleanHotspotKlines(resp.List), true
}

func middleSectorStocks(
	top []AgentStockInSectorItem,
	bottom []AgentStockInSectorItem,
	sector agentSectorMemberSet,
	statByCode map[string]*protocol.TdxStat,
	metric string,
	limit int,
	excludeNew bool,
) []AgentStockInSectorItem {
	skip := map[string]bool{}
	for _, item := range top {
		skip[item.Code] = true
	}
	for _, item := range bottom {
		skip[item.Code] = true
	}
	items := make([]AgentStockInSectorItem, 0, len(sector.Members))
	for _, member := range sector.Members {
		if skip[member.Code] {
			continue
		}
		stat := statByCode[member.Code]
		if stat == nil {
			continue
		}
		item := stockInSectorItem(stat, member.Name, metric)
		if excludeNew && isSectorDetailExcludedStock(item, metric) {
			continue
		}
		items = append(items, item)
	}
	if len(items) == 0 {
		return nil
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Value > items[j].Value
	})
	for i := range items {
		items[i].Rank = i + 1
		items[i].Percentile = float64(len(items)-i) / float64(len(items)) * 100
	}
	start := (len(items) - limit + 1) / 2
	if start < 0 {
		start = 0
	}
	if start+limit > len(items) {
		limit = len(items) - start
	}
	items = items[start : start+limit]
	return limitStockInSectorItems(items, limit)
}

func isSectorDetailExcludedStock(item AgentStockInSectorItem, metric string) bool {
	if isHotspotExcludedStock(item) {
		return true
	}
	switch metric {
	case "chg5", "chg20", "chg60", "changePct":
		return item.Value > 100
	default:
		return false
	}
}

func buildAgentSectorDetailText(summary AgentSectorDetail) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf(
		"板块深度：%s（%s，%s）\n",
		summary.Sector.Name,
		summary.Sector.TypeName,
		valueOrDash(summary.Sector.IndexCode),
	))
	if summary.GeneratedAt != "" {
		b.WriteString("生成时间：" + summary.GeneratedAt + "\n")
	}
	if summary.ConstituentDataDate != "" {
		b.WriteString("成分股统计日：" + summary.ConstituentDataDate + "\n")
	}
	b.WriteString(fmt.Sprintf(
		"样本%d只，排序指标%s；平均%s，上涨%d/%d（%s）。\n",
		summary.MemberSize,
		stockInSectorMetricText(summary.Metric),
		formatPercentText(summary.Stats.AverageValue),
		summary.Stats.RisingCount,
		summary.MemberSize,
		formatPercentText(summary.Stats.RisingPct),
	))
	if summary.ExcludeNew && summary.ExcludedCount > 0 {
		b.WriteString(fmt.Sprintf("已过滤新股/异常涨幅样本%d条。\n", summary.ExcludedCount))
	}
	if summary.Stats.IndexKlineOK {
		if summary.Stats.DailyReturn != nil {
			b.WriteString(fmt.Sprintf(
				"板块指数：最近完整交易日%s单日%s（较%s收盘）。\n",
				summary.Stats.DailyDate,
				formatPercentText(*summary.Stats.DailyReturn),
				summary.Stats.DailyBaseDate,
			))
		}
		if summary.Stats.Return20EndDate != "" {
			b.WriteString(fmt.Sprintf(
				"板块指数近20日：%s（实际交易日%s至%s）。\n",
				formatPercentText(summary.Stats.Return20),
				summary.Stats.Return20StartDate,
				summary.Stats.Return20EndDate,
			))
		}
		if summary.Stats.Return60EndDate != "" {
			b.WriteString(fmt.Sprintf(
				"板块指数近60日：%s（实际交易日%s至%s）。\n",
				formatPercentText(summary.Stats.Return60),
				summary.Stats.Return60StartDate,
				summary.Stats.Return60EndDate,
			))
		}
	}
	writeSectorDetailStocks(&b, "强势股", summary.TopStocks)
	writeSectorDetailStocks(&b, "中游股", summary.MidStocks)
	writeSectorDetailStocks(&b, "弱势股", summary.WeakStocks)
	appendWarningsText(&b, summary.Warnings)
	return strings.TrimSpace(b.String())
}

func writeSectorDetailStocks(
	b *strings.Builder,
	label string,
	items []AgentStockInSectorItem,
) {
	if len(items) == 0 {
		return
	}
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, fmt.Sprintf(
			"%s%s",
			stockInSectorItemName(item),
			formatPercentText(item.Value),
		))
	}
	b.WriteString(label + "：" + strings.Join(parts, "、") + "\n")
}

package main

import (
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/injoyai/tdx"
	"github.com/injoyai/tdx/protocol"
)

type AgentHotspotScan struct {
	Source        string               `json:"source"`
	SectorType    string               `json:"sectorType"`
	Metric        string               `json:"metric"`
	Window        int                  `json:"window,omitempty"`
	Offset        int                  `json:"offset,omitempty"`
	StartDate     string               `json:"startDate,omitempty"`
	EndDate       string               `json:"endDate,omitempty"`
	Limit         int                  `json:"limit"`
	MinMembers    int                  `json:"minMembers"`
	ExcludeNew    bool                 `json:"excludeNew"`
	ExcludedCount int                  `json:"excludedCount"`
	Sectors       []AgentHotspotSector `json:"sectors"`
	MiddleSectors []AgentHotspotSector `json:"middleSectors"`
	ColdSectors   []AgentHotspotSector `json:"coldSectors"`
	Note          string               `json:"note"`
	Warnings      []string             `json:"warnings,omitempty"`
}

type AgentHotspotSector struct {
	Type          string                   `json:"type"`
	TypeName      string                   `json:"typeName"`
	Name          string                   `json:"name"`
	IndexCode     string                   `json:"indexCode,omitempty"`
	MemberCount   int                      `json:"memberCount"`
	RisingCount   int                      `json:"risingCount"`
	FallingCount  int                      `json:"fallingCount"`
	RisingPct     float64                  `json:"risingPct"`
	AverageValue  float64                  `json:"averageValue"`
	ExcludedCount int                      `json:"excludedCount,omitempty"`
	TopStocks     []AgentStockInSectorItem `json:"topStocks"`
	BottomStocks  []AgentStockInSectorItem `json:"bottomStocks,omitempty"`
}

type AgentHotspotScanText struct {
	Format  string `json:"format"`
	Content string `json:"content"`
}

type agentSectorMemberSet struct {
	Block   AgentBriefBlock
	Members []stockRow
}

func handleAgentHotspotScan(w http.ResponseWriter, r *http.Request) {
	summary, ok := loadAgentHotspotScan(w, r)
	if !ok {
		return
	}
	jsonResp(w, summary)
}

func handleAgentHotspotScanText(w http.ResponseWriter, r *http.Request) {
	summary, ok := loadAgentHotspotScan(w, r)
	if !ok {
		return
	}
	jsonResp(w, AgentHotspotScanText{
		Format:  "text/plain; charset=utf-8",
		Content: buildAgentHotspotScanText(summary),
	})
}

func loadAgentHotspotScan(w http.ResponseWriter, r *http.Request) (AgentHotspotScan, bool) {
	sectorType := strings.TrimSpace(r.URL.Query().Get("sectorType"))
	if sectorType == "" {
		sectorType = "concept"
	}
	metric := strings.TrimSpace(r.URL.Query().Get("metric"))
	if metric == "" {
		metric = "chg20"
	}
	limit := parseCount(r.URL.Query().Get("limit"), 20)
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	topStocks := parseCount(r.URL.Query().Get("topStocks"), 3)
	if topStocks <= 0 || topStocks > 10 {
		topStocks = 3
	}
	minMembers := parseCount(r.URL.Query().Get("minMembers"), 20)
	if minMembers <= 0 {
		minMembers = 20
	}
	excludeNew := parseHotspotExcludeNew(r.URL.Query().Get("excludeNew"))
	window := parseCount(r.URL.Query().Get("window"), 20)
	if window <= 0 || window > 250 {
		window = 20
	}
	offset := parseCount(r.URL.Query().Get("offset"), 0)
	if offset < 0 || offset > 500 {
		offset = 0
	}
	startDateText := firstNonEmptyQuery(r, "startDate", "start")
	endDateText := firstNonEmptyQuery(r, "endDate", "end")
	startDate, hasStartDate, ok := parseHotspotDateParam(startDateText)
	if !ok {
		jsonErr(w, "startDate格式应为YYYY-MM-DD或YYYYMMDD")
		return AgentHotspotScan{}, false
	}
	endDate, hasEndDate, ok := parseHotspotDateParam(endDateText)
	if !ok {
		jsonErr(w, "endDate格式应为YYYY-MM-DD或YYYYMMDD")
		return AgentHotspotScan{}, false
	}
	if hasStartDate != hasEndDate {
		jsonErr(w, "startDate和endDate需要同时提供")
		return AgentHotspotScan{}, false
	}
	if hasStartDate && endDate.Before(startDate) {
		jsonErr(w, "endDate不能早于startDate")
		return AgentHotspotScan{}, false
	}

	sectors, err := querySectorMemberSets(sectorType)
	if err != nil {
		jsonErr(w, err.Error())
		return AgentHotspotScan{}, false
	}
	c := cli()
	if c == nil {
		jsonErr(w, "TDX客户端未连接")
		return AgentHotspotScan{}, false
	}
	stats, err := getCachedAgentStats(c)
	if err != nil {
		jsonErr(w, "GetTdxStat失败: "+err.Error())
		return AgentHotspotScan{}, false
	}
	sectorValues := map[string]float64(nil)
	warnings := []string(nil)
	if metric == "windowReturn" {
		if hasStartDate {
			sectorValues, warnings = loadHotspotDateWindowReturns(c, sectors, startDate, endDate)
			window = 0
			offset = 0
		} else {
			sectorValues, warnings = loadHotspotWindowReturns(c, sectors, window, offset)
		}
	}
	return buildAgentHotspotScanWithValues(
		sectors,
		stats,
		metric,
		limit,
		topStocks,
		minMembers,
		excludeNew,
		sectorValues,
		window,
		offset,
		formatHotspotDateParam(startDate, hasStartDate),
		formatHotspotDateParam(endDate, hasEndDate),
		warnings,
	), true
}

func buildAgentHotspotScan(
	sectors []agentSectorMemberSet,
	stats []*protocol.TdxStat,
	metric string,
	limit int,
	topStocks int,
	minMembers int,
	excludeNew bool,
) AgentHotspotScan {
	return buildAgentHotspotScanWithValues(
		sectors,
		stats,
		metric,
		limit,
		topStocks,
		minMembers,
		excludeNew,
		nil,
		0,
		0,
		"",
		"",
		nil,
	)
}

func buildAgentHotspotScanWithValues(
	sectors []agentSectorMemberSet,
	stats []*protocol.TdxStat,
	metric string,
	limit int,
	topStocks int,
	minMembers int,
	excludeNew bool,
	sectorValues map[string]float64,
	window int,
	offset int,
	startDate string,
	endDate string,
	warnings []string,
) AgentHotspotScan {
	if metric == "" {
		metric = "chg20"
	}
	if limit <= 0 {
		limit = 20
	}
	if topStocks <= 0 {
		topStocks = 3
	}
	statByCode := make(map[string]*protocol.TdxStat, len(stats))
	for _, stat := range stats {
		if stat != nil {
			statByCode[stat.Code] = stat
		}
	}

	items := make([]AgentHotspotSector, 0, len(sectors))
	excludedCount := 0
	for _, sector := range sectors {
		item, ok := buildAgentHotspotSector(
			sector,
			statByCode,
			metric,
			topStocks,
			minMembers,
			excludeNew,
		)
		if ok {
			if metric == "windowReturn" {
				value, exists := sectorValues[sector.Block.Name]
				if !exists {
					continue
				}
				item.AverageValue = value
			}
			excludedCount += item.ExcludedCount
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].AverageValue > items[j].AverageValue
	})
	actualLimit := limit
	if actualLimit > len(items) {
		actualLimit = len(items)
	}
	warnings = appendHotspotSampleWarning(warnings, len(items), limit)
	coldSectors := append([]AgentHotspotSector(nil), items...)
	sort.Slice(coldSectors, func(i, j int) bool {
		return coldSectors[i].AverageValue < coldSectors[j].AverageValue
	})
	if metric == "windowReturn" {
		coldSectors = excludeHotspotSectors(coldSectors, items[:actualLimit])
	}
	coldLimit := limit
	if coldLimit > len(coldSectors) {
		coldLimit = len(coldSectors)
	}
	middleSectors := middleHotspotSectors(items, limit)
	return AgentHotspotScan{
		Source:        "tdx_agent_hotspot_scan",
		SectorType:    sectorTypeForSummary(sectors),
		Metric:        metric,
		Window:        window,
		Offset:        offset,
		StartDate:     startDate,
		EndDate:       endDate,
		Limit:         limit,
		MinMembers:    minMembers,
		ExcludeNew:    excludeNew,
		ExcludedCount: excludedCount,
		Sectors:       append([]AgentHotspotSector(nil), items[:actualLimit]...),
		MiddleSectors: middleSectors,
		ColdSectors:   append([]AgentHotspotSector(nil), coldSectors[:coldLimit]...),
		Note:          "热点扫描基于TdxStat统计字段和SQLite板块成分计算；windowReturn使用板块指数日K计算窗口收益，同时返回同期最强、中游和最弱板块。",
		Warnings:      warnings,
	}
}

func buildAgentHotspotSector(
	sector agentSectorMemberSet,
	statByCode map[string]*protocol.TdxStat,
	metric string,
	topStocks int,
	minMembers int,
	excludeNew bool,
) (AgentHotspotSector, bool) {
	stocks := make([]AgentStockInSectorItem, 0, len(sector.Members))
	sum := 0.0
	rising := 0
	falling := 0
	excluded := 0
	for _, member := range sector.Members {
		stat := statByCode[member.Code]
		if stat == nil {
			continue
		}
		item := stockInSectorItem(stat, member.Name, metric)
		if excludeNew && isHotspotExcludedStock(item) {
			excluded++
			continue
		}
		stocks = append(stocks, item)
		sum += item.Value
		if item.ChangePct > 0 {
			rising++
		}
		if item.ChangePct < 0 {
			falling++
		}
	}
	if len(stocks) < minMembers {
		return AgentHotspotSector{}, false
	}
	sort.Slice(stocks, func(i, j int) bool {
		return stocks[i].Value > stocks[j].Value
	})
	for i := range stocks {
		stocks[i].Rank = i + 1
		stocks[i].Percentile = float64(len(stocks)-i) / float64(len(stocks)) * 100
	}
	top := limitStockInSectorItems(stocks, topStocks)
	bottom := append([]AgentStockInSectorItem(nil), stocks...)
	sort.Slice(bottom, func(i, j int) bool {
		return bottom[i].Value < bottom[j].Value
	})
	return AgentHotspotSector{
		Type:          sector.Block.Type,
		TypeName:      sector.Block.TypeName,
		Name:          sector.Block.Name,
		IndexCode:     sector.Block.IndexCode,
		MemberCount:   len(stocks),
		RisingCount:   rising,
		FallingCount:  falling,
		RisingPct:     float64(rising) / float64(len(stocks)) * 100,
		AverageValue:  sum / float64(len(stocks)),
		ExcludedCount: excluded,
		TopStocks:     top,
		BottomStocks:  limitStockInSectorItems(bottom, topStocks),
	}, true
}

func buildAgentHotspotScanText(summary AgentHotspotScan) string {
	var b strings.Builder
	b.WriteString("热点扫描：\n")
	b.WriteString(fmt.Sprintf(
		"板块类型%s，排序指标%s，最少成分%d只。",
		sectorTypeName(summary.SectorType),
		hotspotMetricText(summary),
		summary.MinMembers,
	))
	if summary.ExcludeNew {
		b.WriteString(fmt.Sprintf(" 已排除新股/异常涨幅样本%d条。", summary.ExcludedCount))
	}
	b.WriteString("\n\n最强板块：\n")
	for i, sector := range summary.Sectors {
		writeHotspotSectorLine(&b, i+1, sector, "代表股")
	}
	if len(summary.MiddleSectors) > 0 {
		b.WriteString("\n中游板块：\n")
		for i, sector := range summary.MiddleSectors {
			writeHotspotSectorLine(&b, i+1, sector, "代表股")
		}
	}
	if len(summary.ColdSectors) > 0 {
		b.WriteString("\n最弱板块：\n")
		for i, sector := range summary.ColdSectors {
			writeHotspotSectorLine(&b, i+1, sector, "抗跌股")
		}
	}
	b.WriteString("\n用途：该接口用于发现热点板块；单股在板块中的位置请使用stock-in-sector。")
	if len(summary.Warnings) > 0 {
		b.WriteString("\n\n注意：\n")
		for _, warning := range summary.Warnings {
			b.WriteString("- " + warning + "\n")
		}
	}
	return strings.TrimSpace(b.String())
}

func writeHotspotSectorLine(
	b *strings.Builder,
	rank int,
	sector AgentHotspotSector,
	label string,
) {
	b.WriteString(fmt.Sprintf(
		"%d. %s：平均%s，上涨%d/%d（%s）。",
		rank,
		sector.Name,
		formatPercentText(sector.AverageValue),
		sector.RisingCount,
		sector.MemberCount,
		formatPercentText(sector.RisingPct),
	))
	if label == "" {
		label = "代表股"
	}
	stocks := sector.TopStocks
	if len(stocks) > 0 {
		parts := make([]string, 0, len(stocks))
		for _, stock := range stocks {
			parts = append(parts, fmt.Sprintf(
				"%s%s",
				stockInSectorItemName(stock),
				formatPercentText(stock.Value),
			))
		}
		b.WriteString(" " + label + "：" + strings.Join(parts, "、") + "。")
	}
	b.WriteString("\n")
}

func loadHotspotWindowReturns(
	c *tdx.Client,
	sectors []agentSectorMemberSet,
	window int,
	offset int,
) (map[string]float64, []string) {
	values := make(map[string]float64, len(sectors))
	failed := 0
	for _, sector := range sectors {
		klines, ok := loadHotspotIndexKlines(c, sector)
		if !ok {
			failed++
			continue
		}
		value, ok := hotspotWindowReturn(klines, window, offset)
		if !ok {
			failed++
			continue
		}
		values[sector.Block.Name] = value
	}
	if failed == 0 {
		return values, nil
	}
	return values, []string{fmt.Sprintf("windowReturn有%d个板块未能计算窗口收益", failed)}
}

func loadHotspotDateWindowReturns(
	c *tdx.Client,
	sectors []agentSectorMemberSet,
	startDate time.Time,
	endDate time.Time,
) (map[string]float64, []string) {
	values := make(map[string]float64, len(sectors))
	failed := 0
	for _, sector := range sectors {
		klines, ok := loadHotspotIndexKlines(c, sector)
		if !ok {
			failed++
			continue
		}
		value, ok := hotspotDateWindowReturn(klines, startDate, endDate)
		if !ok {
			failed++
			continue
		}
		values[sector.Block.Name] = value
	}
	if failed == 0 {
		return values, nil
	}
	return values, []string{fmt.Sprintf("windowReturn有%d个板块未能按日期区间计算收益", failed)}
}

func loadHotspotIndexKlines(c *tdx.Client, sector agentSectorMemberSet) (protocol.Klines, bool) {
	if sector.Block.IndexCode == "" {
		return nil, false
	}
	resp, err := c.GetIndexDayAll("sh" + sector.Block.IndexCode)
	if err != nil || resp == nil {
		return nil, false
	}
	return cleanHotspotKlines(resp.List), true
}

func cleanHotspotKlines(klines protocol.Klines) protocol.Klines {
	cleaned := make(protocol.Klines, 0, len(klines))
	for _, kline := range klines {
		if kline == nil {
			continue
		}
		close := kline.Close.Float64()
		if close < 100 || close > 10000 {
			continue
		}
		year := kline.Time.Year()
		if year < 1990 || year > 2100 {
			continue
		}
		cleaned = append(cleaned, kline)
	}
	sort.Slice(cleaned, func(i, j int) bool {
		return cleaned[i].Time.Before(cleaned[j].Time)
	})
	if len(cleaned) < 2 {
		return cleaned
	}
	stable := protocol.Klines{cleaned[len(cleaned)-1]}
	for i := len(cleaned) - 2; i >= 0; i-- {
		nextClose := stable[len(stable)-1].Close.Float64()
		close := cleaned[i].Close.Float64()
		if math.Abs((nextClose-close)/close*100) > 50 {
			continue
		}
		stable = append(stable, cleaned[i])
	}
	for i, j := 0, len(stable)-1; i < j; i, j = i+1, j-1 {
		stable[i], stable[j] = stable[j], stable[i]
	}
	return stable
}

func hotspotWindowReturn(klines protocol.Klines, window int, offset int) (float64, bool) {
	if window <= 0 || offset < 0 || len(klines) <= offset+window {
		return 0, false
	}
	end := len(klines) - 1 - offset
	start := end - window
	base := klines[start].Close.Float64()
	latest := klines[end].Close.Float64()
	if base == 0 {
		return 0, false
	}
	return (latest - base) / base * 100, true
}

func hotspotDateWindowReturn(
	klines protocol.Klines,
	startDate time.Time,
	endDate time.Time,
) (float64, bool) {
	var base *protocol.Kline
	var latest *protocol.Kline
	for _, kline := range klines {
		if kline == nil {
			continue
		}
		date := dateOnly(kline.Time)
		if base == nil && !date.Before(startDate) {
			base = kline
		}
		if !date.After(endDate) {
			latest = kline
		}
	}
	if base == nil || latest == nil || latest.Time.Before(base.Time) {
		return 0, false
	}
	baseClose := base.Close.Float64()
	latestClose := latest.Close.Float64()
	if baseClose == 0 {
		return 0, false
	}
	return (latestClose - baseClose) / baseClose * 100, true
}

func hotspotMetricText(summary AgentHotspotScan) string {
	if summary.Metric == "windowReturn" && summary.StartDate != "" && summary.EndDate != "" {
		return fmt.Sprintf("窗口收益%s至%s", summary.StartDate, summary.EndDate)
	}
	if summary.Metric == "windowReturn" {
		return fmt.Sprintf("窗口收益%d日，偏移%d日", summary.Window, summary.Offset)
	}
	return stockInSectorMetricText(summary.Metric)
}

func middleHotspotSectors(items []AgentHotspotSector, limit int) []AgentHotspotSector {
	if limit <= 0 || len(items) == 0 {
		return nil
	}
	startRange := limit
	endRange := len(items) - limit
	if startRange >= endRange {
		return nil
	}
	candidates := items[startRange:endRange]
	if limit > len(candidates) {
		limit = len(candidates)
	}
	start := (len(candidates) - limit + 1) / 2
	return append([]AgentHotspotSector(nil), candidates[start:start+limit]...)
}

func appendHotspotSampleWarning(warnings []string, itemCount int, limit int) []string {
	if limit <= 0 || itemCount >= limit*3 {
		return warnings
	}
	return append(warnings, fmt.Sprintf(
		"可排序板块仅%d个，不足以返回最强/中游/最弱各%d个",
		itemCount,
		limit,
	))
}

func excludeHotspotSectors(
	items []AgentHotspotSector,
	excluded []AgentHotspotSector,
) []AgentHotspotSector {
	if len(items) == 0 || len(excluded) == 0 {
		return items
	}
	names := make(map[string]struct{}, len(excluded))
	for _, item := range excluded {
		names[item.Name] = struct{}{}
	}
	filtered := make([]AgentHotspotSector, 0, len(items))
	for _, item := range items {
		if _, exists := names[item.Name]; exists {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func sectorTypeForSummary(sectors []agentSectorMemberSet) string {
	if len(sectors) == 0 {
		return ""
	}
	return sectors[0].Block.Type
}

func parseHotspotExcludeNew(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

func firstNonEmptyQuery(r *http.Request, names ...string) string {
	for _, name := range names {
		value := strings.TrimSpace(r.URL.Query().Get(name))
		if value != "" {
			return value
		}
	}
	return ""
}

func parseHotspotDateParam(raw string) (time.Time, bool, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false, true
	}
	for _, layout := range []string{"2006-01-02", "20060102"} {
		value, err := time.ParseInLocation(layout, raw, time.Local)
		if err == nil {
			return dateOnly(value), true, true
		}
	}
	return time.Time{}, false, false
}

func formatHotspotDateParam(value time.Time, exists bool) string {
	if !exists {
		return ""
	}
	return value.Format("2006-01-02")
}

func dateOnly(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, value.Location())
}

func isHotspotExcludedStock(item AgentStockInSectorItem) bool {
	if item.ChangePct > 100 {
		return true
	}
	name := strings.TrimSpace(item.Name)
	return strings.HasPrefix(name, "N") || strings.HasPrefix(name, "C")
}

func sectorTypeName(sectorType string) string {
	switch sectorType {
	case "concept":
		return "概念板块"
	case "style_region":
		return "地域/风格板块"
	case "index":
		return "指数板块"
	default:
		return sectorType
	}
}

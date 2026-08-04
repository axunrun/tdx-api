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
	Source                  string               `json:"source"`
	GeneratedAt             string               `json:"generatedAt"`
	MetricSource            string               `json:"metricSource"`
	MetricStartDate         string               `json:"metricStartDate,omitempty"`
	MetricEndDate           string               `json:"metricEndDate,omitempty"`
	ConstituentDataDate     string               `json:"constituentDataDate,omitempty"`
	BoardIndexStartDate     string               `json:"boardIndexStartDate,omitempty"`
	BoardIndexEndDate       string               `json:"boardIndexEndDate,omitempty"`
	BoardIndexDailyBaseDate string               `json:"boardIndexDailyBaseDate,omitempty"`
	BoardIndexDailyDate     string               `json:"boardIndexDailyDate,omitempty"`
	SectorType              string               `json:"sectorType"`
	Metric                  string               `json:"metric"`
	Window                  int                  `json:"window,omitempty"`
	Offset                  int                  `json:"offset,omitempty"`
	StartDate               string               `json:"startDate,omitempty"`
	EndDate                 string               `json:"endDate,omitempty"`
	Limit                   int                  `json:"limit"`
	MinMembers              int                  `json:"minMembers"`
	ExcludeNew              bool                 `json:"excludeNew"`
	ExcludedCount           int                  `json:"excludedCount"`
	Sectors                 []AgentHotspotSector `json:"sectors"`
	MiddleSectors           []AgentHotspotSector `json:"middleSectors"`
	ColdSectors             []AgentHotspotSector `json:"coldSectors"`
	Note                    string               `json:"note"`
	Warnings                []string             `json:"warnings,omitempty"`
}

type AgentHotspotSector struct {
	Type                    string                   `json:"type"`
	TypeName                string                   `json:"typeName"`
	Name                    string                   `json:"name"`
	IndexCode               string                   `json:"indexCode,omitempty"`
	MemberCount             int                      `json:"memberCount"`
	RisingCount             int                      `json:"risingCount"`
	FallingCount            int                      `json:"fallingCount"`
	RisingPct               float64                  `json:"risingPct"`
	AverageValue            float64                  `json:"averageValue"`
	ConstituentAverageValue float64                  `json:"constituentAverageValue"`
	BoardIndexReturn        *float64                 `json:"boardIndexReturn,omitempty"`
	BoardIndexDailyReturn   *float64                 `json:"boardIndexDailyReturn,omitempty"`
	ExcludedCount           int                      `json:"excludedCount,omitempty"`
	TopStocks               []AgentStockInSectorItem `json:"topStocks"`
	BottomStocks            []AgentStockInSectorItem `json:"bottomStocks,omitempty"`
}

type AgentHotspotScanText struct {
	Format  string `json:"format"`
	Content string `json:"content"`
}

type agentSectorMemberSet struct {
	Block   AgentBriefBlock
	Members []stockRow
}

type hotspotIndexReturn struct {
	Value     float64
	StartDate string
	EndDate   string
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
	now := time.Now()
	sectorValues := map[string]float64(nil)
	warnings := []string(nil)
	metricStartDate := ""
	metricEndDate := ""
	switch metric {
	case "windowReturn":
		if hasStartDate {
			sectorValues, metricStartDate, metricEndDate, warnings =
				loadHotspotDateWindowReturns(c, sectors, startDate, endDate)
			window = 0
			offset = 0
		} else {
			sectorValues, metricStartDate, metricEndDate, warnings =
				loadHotspotWindowReturns(c, sectors, window, offset)
		}
	case "dailyReturn":
		sectorValues, metricStartDate, metricEndDate, warnings =
			loadHotspotCompletedDailyReturns(c, sectors, now)
	}
	summary := buildAgentHotspotScanWithValues(
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
	)
	summary.GeneratedAt = now.Format(time.RFC3339)
	if metric == "windowReturn" || metric == "dailyReturn" {
		summary.MetricStartDate = metricStartDate
		summary.MetricEndDate = metricEndDate
	} else if metricWindow, ok := hotspotMetricWindow(metric); ok {
		startDate, endDate, dailyBaseDate, dailyDate, indexWarnings :=
			loadSelectedHotspotIndexReturns(
				c,
				&summary,
				metricWindow,
				summary.ConstituentDataDate,
				now,
			)
		summary.BoardIndexStartDate = startDate
		summary.BoardIndexEndDate = endDate
		summary.BoardIndexDailyBaseDate = dailyBaseDate
		summary.BoardIndexDailyDate = dailyDate
		summary.Warnings = append(summary.Warnings, indexWarnings...)
	}
	return summary, true
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
	latestStatDate := latestHotspotStatDate(stats)
	statByCode := make(map[string]*protocol.TdxStat, len(stats))
	for _, stat := range stats {
		if stat != nil && (latestStatDate == "" || stat.Date == latestStatDate) {
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
			if metric == "windowReturn" || metric == "dailyReturn" {
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
	if metric == "windowReturn" || metric == "dailyReturn" {
		coldSectors = excludeHotspotSectors(coldSectors, items[:actualLimit])
	}
	coldLimit := limit
	if coldLimit > len(coldSectors) {
		coldLimit = len(coldSectors)
	}
	middleSectors := middleHotspotSectors(items, limit)
	note := "榜单按TdxStat成分股统计指标排序；入榜板块补充同周期TDX板块指数日K收益。"
	if metric == "dailyReturn" {
		note = "榜单按TDX板块指数最近完整交易日单日涨跌排序；成分股统计日期单独标注。"
	} else if metric == "windowReturn" {
		note = "榜单按TDX板块指数指定交易日区间收益排序；成分股统计日期单独标注。"
	}
	note += " 上涨比例使用输出所标TdxStat完整统计日的单日涨跌。"
	return AgentHotspotScan{
		Source:              "tdx_agent_hotspot_scan",
		MetricSource:        hotspotMetricSource(metric),
		MetricEndDate:       hotspotStatMetricEndDate(metric, latestStatDate),
		ConstituentDataDate: formatTdxStatDate(latestStatDate),
		SectorType:          sectorTypeForSummary(sectors),
		Metric:              metric,
		Window:              window,
		Offset:              offset,
		StartDate:           startDate,
		EndDate:             endDate,
		Limit:               limit,
		MinMembers:          minMembers,
		ExcludeNew:          excludeNew,
		ExcludedCount:       excludedCount,
		Sectors:             append([]AgentHotspotSector(nil), items[:actualLimit]...),
		MiddleSectors:       middleSectors,
		ColdSectors:         append([]AgentHotspotSector(nil), coldSectors[:coldLimit]...),
		Note:                note,
		Warnings:            warnings,
	}
}

func latestHotspotStatDate(stats []*protocol.TdxStat) string {
	latest := ""
	for _, stat := range stats {
		if stat != nil && stat.Date > latest {
			latest = stat.Date
		}
	}
	return latest
}

func hotspotMetricSource(metric string) string {
	if metric == "windowReturn" || metric == "dailyReturn" {
		return "TDX板块指数日K"
	}
	return "TdxStat成分股聚合"
}

func hotspotStatMetricEndDate(metric string, statDate string) string {
	if metric == "windowReturn" || metric == "dailyReturn" {
		return ""
	}
	return formatTdxStatDate(statDate)
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
	average := sum / float64(len(stocks))
	return AgentHotspotSector{
		Type:                    sector.Block.Type,
		TypeName:                sector.Block.TypeName,
		Name:                    sector.Block.Name,
		IndexCode:               sector.Block.IndexCode,
		MemberCount:             len(stocks),
		RisingCount:             rising,
		FallingCount:            falling,
		RisingPct:               float64(rising) / float64(len(stocks)) * 100,
		AverageValue:            average,
		ConstituentAverageValue: average,
		ExcludedCount:           excluded,
		TopStocks:               top,
		BottomStocks:            limitStockInSectorItems(bottom, topStocks),
	}, true
}

func buildAgentHotspotScanText(summary AgentHotspotScan) string {
	var b strings.Builder
	b.WriteString("热点扫描：\n")
	if summary.GeneratedAt != "" {
		b.WriteString("生成时间：" + summary.GeneratedAt + "\n")
	}
	writeHotspotDataTime(&b, summary)
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
		writeHotspotSectorLine(&b, i+1, sector, summary, "强势股")
	}
	if len(summary.MiddleSectors) > 0 {
		b.WriteString("\n中游板块：\n")
		for i, sector := range summary.MiddleSectors {
			writeHotspotSectorLine(&b, i+1, sector, summary, "强势股")
		}
	}
	if len(summary.ColdSectors) > 0 {
		b.WriteString("\n最弱板块：\n")
		for i, sector := range summary.ColdSectors {
			writeHotspotSectorLine(&b, i+1, sector, summary, "抗跌股")
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

func writeHotspotDataTime(b *strings.Builder, summary AgentHotspotScan) {
	if summary.Metric == "windowReturn" || summary.Metric == "dailyReturn" {
		if summary.MetricStartDate != "" && summary.MetricEndDate != "" {
			if summary.Metric == "dailyReturn" {
				b.WriteString(fmt.Sprintf(
					"指标数据：%s，最近完整交易日%s（较%s收盘）。\n",
					summary.MetricSource,
					summary.MetricEndDate,
					summary.MetricStartDate,
				))
			} else {
				b.WriteString(fmt.Sprintf(
					"指标数据：%s，实际交易日区间%s至%s。\n",
					summary.MetricSource,
					summary.MetricStartDate,
					summary.MetricEndDate,
				))
			}
		}
		if summary.ConstituentDataDate != "" {
			b.WriteString(fmt.Sprintf(
				"成分股辅助数据：TdxStat最近完整统计日%s；上涨比例和代表股采用该日数据。\n",
				summary.ConstituentDataDate,
			))
		}
		return
	}
	if summary.MetricEndDate != "" {
		b.WriteString(fmt.Sprintf(
			"指标数据：%s，最近完整交易日%s。\n",
			summary.MetricSource,
			summary.MetricEndDate,
		))
	}
	if summary.BoardIndexStartDate != "" && summary.BoardIndexEndDate != "" {
		b.WriteString(fmt.Sprintf(
			"板块指数辅助数据：TDX板块指数日K，实际交易日区间%s至%s。\n",
			summary.BoardIndexStartDate,
			summary.BoardIndexEndDate,
		))
	}
	if summary.BoardIndexDailyBaseDate != "" && summary.BoardIndexDailyDate != "" {
		b.WriteString(fmt.Sprintf(
			"板块指数固定单日数据：最近完整交易日%s（较%s收盘）。\n",
			summary.BoardIndexDailyDate,
			summary.BoardIndexDailyBaseDate,
		))
	}
}

func writeHotspotSectorLine(
	b *strings.Builder,
	rank int,
	sector AgentHotspotSector,
	summary AgentHotspotScan,
	label string,
) {
	period := hotspotMetricPeriodText(summary.Metric)
	b.WriteString(fmt.Sprintf("%d. %s\n", rank, sector.Name))
	b.WriteString("   指数：")
	switch summary.Metric {
	case "windowReturn":
		b.WriteString(fmt.Sprintf(
			"指定区间%s。",
			formatPercentText(sector.AverageValue),
		))
		period = "最近完整交易日"
	case "dailyReturn":
		b.WriteString(fmt.Sprintf(
			"最近完整交易日单日%s。",
			formatPercentText(sector.AverageValue),
		))
		period = "单日"
	default:
		indexParts := make([]string, 0, 2)
		if sector.BoardIndexDailyReturn != nil {
			indexParts = append(indexParts, "最近完整交易日单日"+
				formatPercentText(*sector.BoardIndexDailyReturn))
		}
		if sector.BoardIndexReturn != nil {
			indexLabel := period
			if summary.Metric == "changePct" {
				indexLabel = "同统计日单日"
			}
			indexParts = append(indexParts, indexLabel+formatPercentText(*sector.BoardIndexReturn))
		}
		if len(indexParts) == 0 {
			indexParts = append(indexParts, "不可用")
		}
		b.WriteString(strings.Join(indexParts, "；") + "。")
	}
	b.WriteString("\n   成分：")
	constituentAverage := sector.AverageValue
	if summary.Metric == "dailyReturn" || summary.Metric == "windowReturn" {
		constituentAverage = sector.ConstituentAverageValue
	}
	b.WriteString(fmt.Sprintf(
		"%s平均%s；上涨%d/%d（%s）。",
		period,
		formatPercentText(constituentAverage),
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
		b.WriteString(" " + period + label + "：" + strings.Join(parts, "、") + "。")
	}
	b.WriteString("\n")
}

func hotspotMetricPeriodText(metric string) string {
	switch metric {
	case "chg5":
		return "近5日"
	case "chg20":
		return "近20日"
	case "chg60":
		return "近60日"
	default:
		return "单日"
	}
}

func hotspotMetricWindow(metric string) (int, bool) {
	switch metric {
	case "changePct":
		return 1, true
	case "chg5":
		return 5, true
	case "chg20":
		return 20, true
	case "chg60":
		return 60, true
	default:
		return 0, false
	}
}

func loadSelectedHotspotIndexReturns(
	c *tdx.Client,
	summary *AgentHotspotScan,
	window int,
	endDate string,
	now time.Time,
) (string, string, string, string, []string) {
	selected := selectedHotspotSectors(*summary)
	results := make(map[string]hotspotIndexReturn, len(selected))
	dailyResults := make(map[string]hotspotIndexReturn, len(selected))
	rangeCounts := make(map[string]int)
	dailyRangeCounts := make(map[string]int)
	bestRange := ""
	bestDailyRange := ""
	failed := 0
	dailyFailed := 0
	for _, sector := range selected {
		klines, ok := loadHotspotIndexKlines(c, agentSectorMemberSet{
			Block: AgentBriefBlock{Name: sector.Name, IndexCode: sector.IndexCode},
		})
		if !ok {
			failed++
			dailyFailed++
			continue
		}
		dailyValue, dailyBaseDate, dailyDate, dailyOK :=
			hotspotCompletedDailyReturn(klines, now)
		if dailyOK {
			dailyResult := hotspotIndexReturn{
				Value:     dailyValue,
				StartDate: dailyBaseDate,
				EndDate:   dailyDate,
			}
			dailyResults[sector.Name] = dailyResult
			dailyRangeKey := dailyBaseDate + "|" + dailyDate
			dailyRangeCounts[dailyRangeKey]++
			if bestDailyRange == "" ||
				dailyRangeCounts[dailyRangeKey] > dailyRangeCounts[bestDailyRange] {
				bestDailyRange = dailyRangeKey
			}
		} else {
			dailyFailed++
		}
		value, startDate, actualEndDate, ok := hotspotWindowReturnUntilDateWithDates(
			klines,
			window,
			endDate,
		)
		if !ok {
			failed++
			continue
		}
		result := hotspotIndexReturn{
			Value:     value,
			StartDate: startDate,
			EndDate:   actualEndDate,
		}
		results[sector.Name] = result
		rangeKey := startDate + "|" + actualEndDate
		rangeCounts[rangeKey]++
		if bestRange == "" || rangeCounts[rangeKey] > rangeCounts[bestRange] {
			bestRange = rangeKey
		}
	}

	mismatched := 0
	if bestRange != "" {
		applyHotspotIndexReturns(summary.Sectors, results, bestRange, &mismatched)
		applyHotspotIndexReturns(summary.MiddleSectors, results, bestRange, &mismatched)
		applyHotspotIndexReturns(summary.ColdSectors, results, bestRange, &mismatched)
	}
	dailyMismatched := 0
	if bestDailyRange != "" {
		applyHotspotDailyReturns(
			summary.Sectors,
			dailyResults,
			bestDailyRange,
			&dailyMismatched,
		)
		applyHotspotDailyReturns(
			summary.MiddleSectors,
			dailyResults,
			bestDailyRange,
			&dailyMismatched,
		)
		applyHotspotDailyReturns(
			summary.ColdSectors,
			dailyResults,
			bestDailyRange,
			&dailyMismatched,
		)
	}

	warnings := []string(nil)
	if bestRange == "" {
		warnings = append(warnings, "入榜板块均未能计算同周期板块指数收益")
	}
	if failed > 0 {
		warnings = append(warnings, fmt.Sprintf(
			"有%d个入榜板块未能计算同周期板块指数收益",
			failed,
		))
	}
	if mismatched > 0 {
		warnings = append(warnings, fmt.Sprintf(
			"有%d个入榜板块因指数实际交易日区间不一致未展示指数收益",
			mismatched,
		))
	}
	if dailyFailed > 0 {
		warnings = append(warnings, fmt.Sprintf(
			"有%d个入榜板块未能计算最近完整交易日指数涨跌幅",
			dailyFailed,
		))
	}
	if dailyMismatched > 0 {
		warnings = append(warnings, fmt.Sprintf(
			"有%d个入榜板块因最近完整交易日不一致未展示单日指数涨跌",
			dailyMismatched,
		))
	}
	startDate, actualEndDate := splitHotspotRange(bestRange)
	dailyBaseDate, dailyDate := splitHotspotRange(bestDailyRange)
	return startDate, actualEndDate, dailyBaseDate, dailyDate, warnings
}

func selectedHotspotSectors(summary AgentHotspotScan) []AgentHotspotSector {
	selected := make([]AgentHotspotSector, 0, len(summary.Sectors)+
		len(summary.MiddleSectors)+len(summary.ColdSectors))
	seen := make(map[string]struct{})
	for _, sectors := range [][]AgentHotspotSector{
		summary.Sectors,
		summary.MiddleSectors,
		summary.ColdSectors,
	} {
		for _, sector := range sectors {
			if _, exists := seen[sector.Name]; exists {
				continue
			}
			seen[sector.Name] = struct{}{}
			selected = append(selected, sector)
		}
	}
	return selected
}

func applyHotspotIndexReturns(
	sectors []AgentHotspotSector,
	results map[string]hotspotIndexReturn,
	expectedRange string,
	mismatched *int,
) {
	for i := range sectors {
		result, exists := results[sectors[i].Name]
		if !exists {
			continue
		}
		if result.StartDate+"|"+result.EndDate != expectedRange {
			*mismatched++
			continue
		}
		value := result.Value
		sectors[i].BoardIndexReturn = &value
	}
}

func applyHotspotDailyReturns(
	sectors []AgentHotspotSector,
	results map[string]hotspotIndexReturn,
	expectedRange string,
	mismatched *int,
) {
	for i := range sectors {
		result, exists := results[sectors[i].Name]
		if !exists {
			continue
		}
		if result.StartDate+"|"+result.EndDate != expectedRange {
			*mismatched++
			continue
		}
		value := result.Value
		sectors[i].BoardIndexDailyReturn = &value
	}
}

func splitHotspotRange(value string) (string, string) {
	parts := strings.SplitN(value, "|", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

func loadHotspotWindowReturns(
	c *tdx.Client,
	sectors []agentSectorMemberSet,
	window int,
	offset int,
) (map[string]float64, string, string, []string) {
	values := make(map[string]float64, len(sectors))
	failed := 0
	commonStart := ""
	commonEnd := ""
	mixedRange := false
	for _, sector := range sectors {
		klines, ok := loadHotspotIndexKlines(c, sector)
		if !ok {
			failed++
			continue
		}
		value, startDate, endDate, ok := hotspotWindowReturnWithDates(klines, window, offset)
		if !ok {
			failed++
			continue
		}
		nextStart, nextEnd, nextMixed := mergeHotspotDateRange(
			commonStart,
			commonEnd,
			startDate,
			endDate,
		)
		if nextMixed {
			mixedRange = true
			continue
		}
		commonStart, commonEnd = nextStart, nextEnd
		values[sector.Block.Name] = value
	}
	warnings := hotspotWindowWarnings(failed, mixedRange, false)
	return values, commonStart, commonEnd, warnings
}

func loadHotspotCompletedDailyReturns(
	c *tdx.Client,
	sectors []agentSectorMemberSet,
	now time.Time,
) (map[string]float64, string, string, []string) {
	results := make(map[string]hotspotIndexReturn, len(sectors))
	rangeCounts := make(map[string]int)
	bestRange := ""
	failed := 0
	for _, sector := range sectors {
		klines, ok := loadHotspotRecentIndexKlines(c, sector, 3)
		if !ok {
			failed++
			continue
		}
		value, baseDate, endDate, ok := hotspotCompletedDailyReturn(klines, now)
		if !ok {
			failed++
			continue
		}
		result := hotspotIndexReturn{Value: value, StartDate: baseDate, EndDate: endDate}
		results[sector.Block.Name] = result
		rangeKey := baseDate + "|" + endDate
		rangeCounts[rangeKey]++
		if bestRange == "" || rangeCounts[rangeKey] > rangeCounts[bestRange] {
			bestRange = rangeKey
		}
	}

	values := make(map[string]float64, rangeCounts[bestRange])
	mismatched := 0
	for name, result := range results {
		if result.StartDate+"|"+result.EndDate != bestRange {
			mismatched++
			continue
		}
		values[name] = result.Value
	}
	warnings := []string(nil)
	if failed > 0 {
		warnings = append(warnings, fmt.Sprintf(
			"有%d个板块未能计算最近完整交易日指数涨跌幅",
			failed,
		))
	}
	if mismatched > 0 {
		warnings = append(warnings, fmt.Sprintf(
			"有%d个板块因最近完整交易日不一致未参与排序",
			mismatched,
		))
	}
	if bestRange == "" {
		return values, "", "", warnings
	}
	parts := strings.SplitN(bestRange, "|", 2)
	return values, parts[0], parts[1], warnings
}

func loadHotspotDateWindowReturns(
	c *tdx.Client,
	sectors []agentSectorMemberSet,
	startDate time.Time,
	endDate time.Time,
) (map[string]float64, string, string, []string) {
	values := make(map[string]float64, len(sectors))
	failed := 0
	commonStart := ""
	commonEnd := ""
	mixedRange := false
	for _, sector := range sectors {
		klines, ok := loadHotspotIndexKlines(c, sector)
		if !ok {
			failed++
			continue
		}
		value, actualStart, actualEnd, ok := hotspotDateWindowReturnWithDates(
			klines,
			startDate,
			endDate,
		)
		if !ok {
			failed++
			continue
		}
		nextStart, nextEnd, nextMixed := mergeHotspotDateRange(
			commonStart,
			commonEnd,
			actualStart,
			actualEnd,
		)
		if nextMixed {
			mixedRange = true
			continue
		}
		commonStart, commonEnd = nextStart, nextEnd
		values[sector.Block.Name] = value
	}
	warnings := hotspotWindowWarnings(failed, mixedRange, true)
	return values, commonStart, commonEnd, warnings
}

func mergeHotspotDateRange(
	commonStart string,
	commonEnd string,
	startDate string,
	endDate string,
) (string, string, bool) {
	if commonStart == "" && commonEnd == "" {
		return startDate, endDate, false
	}
	return commonStart, commonEnd, commonStart != startDate || commonEnd != endDate
}

func hotspotWindowWarnings(failed int, mixedRange bool, explicitDates bool) []string {
	warnings := []string(nil)
	if failed > 0 {
		message := "windowReturn有%d个板块未能计算窗口收益"
		if explicitDates {
			message = "windowReturn有%d个板块未能按日期区间计算收益"
		}
		warnings = append(warnings, fmt.Sprintf(message, failed))
	}
	if mixedRange {
		warnings = append(warnings, "已排除实际交易日区间与排行榜统一区间不一致的板块")
	}
	return warnings
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

func loadHotspotRecentIndexKlines(
	c *tdx.Client,
	sector agentSectorMemberSet,
	count uint16,
) (protocol.Klines, bool) {
	if sector.Block.IndexCode == "" {
		return nil, false
	}
	resp, err := c.GetIndexDay("sh"+sector.Block.IndexCode, 0, count)
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
	value, _, _, ok := hotspotWindowReturnWithDates(klines, window, offset)
	return value, ok
}

func hotspotWindowReturnWithDates(
	klines protocol.Klines,
	window int,
	offset int,
) (float64, string, string, bool) {
	if window <= 0 || offset < 0 || len(klines) <= offset+window {
		return 0, "", "", false
	}
	end := len(klines) - 1 - offset
	start := end - window
	base := klines[start].Close.Float64()
	latest := klines[end].Close.Float64()
	if base == 0 {
		return 0, "", "", false
	}
	return (latest - base) / base * 100,
		klines[start].Time.Format(time.DateOnly),
		klines[end].Time.Format(time.DateOnly),
		true
}

func hotspotWindowReturnUntilDateWithDates(
	klines protocol.Klines,
	window int,
	endDate string,
) (float64, string, string, bool) {
	if endDate == "" {
		return hotspotWindowReturnWithDates(klines, window, 0)
	}
	target, err := time.ParseInLocation(time.DateOnly, endDate, time.Local)
	if err != nil {
		return 0, "", "", false
	}
	for i := len(klines) - 1; i >= 0; i-- {
		if klines[i] == nil || dateOnly(klines[i].Time).After(target) {
			continue
		}
		return hotspotWindowReturnWithDates(klines[:i+1], window, 0)
	}
	return 0, "", "", false
}

func hotspotCompletedDailyReturn(
	klines protocol.Klines,
	now time.Time,
) (float64, string, string, bool) {
	today := dateOnly(now)
	closeTime := time.Date(
		now.Year(),
		now.Month(),
		now.Day(),
		15,
		0,
		0,
		0,
		now.Location(),
	)
	allowToday := now.Weekday() == time.Saturday ||
		now.Weekday() == time.Sunday ||
		now.After(closeTime)
	for i := len(klines) - 1; i > 0; i-- {
		if klines[i] == nil {
			continue
		}
		endDate := dateOnly(klines[i].Time)
		if endDate.After(today) || (endDate.Equal(today) && !allowToday) {
			continue
		}
		for j := i - 1; j >= 0; j-- {
			if klines[j] == nil || !klines[j].Time.Before(klines[i].Time) {
				continue
			}
			base := klines[j].Close.Float64()
			latest := klines[i].Close.Float64()
			if base == 0 {
				return 0, "", "", false
			}
			return (latest - base) / base * 100,
				klines[j].Time.Format(time.DateOnly),
				klines[i].Time.Format(time.DateOnly),
				true
		}
	}
	return 0, "", "", false
}

func hotspotDateWindowReturn(
	klines protocol.Klines,
	startDate time.Time,
	endDate time.Time,
) (float64, bool) {
	value, _, _, ok := hotspotDateWindowReturnWithDates(klines, startDate, endDate)
	return value, ok
}

func hotspotDateWindowReturnWithDates(
	klines protocol.Klines,
	startDate time.Time,
	endDate time.Time,
) (float64, string, string, bool) {
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
		return 0, "", "", false
	}
	baseClose := base.Close.Float64()
	latestClose := latest.Close.Float64()
	if baseClose == 0 {
		return 0, "", "", false
	}
	return (latestClose - baseClose) / baseClose * 100,
		base.Time.Format(time.DateOnly),
		latest.Time.Format(time.DateOnly),
		true
}

func hotspotMetricText(summary AgentHotspotScan) string {
	if summary.Metric == "windowReturn" && summary.StartDate != "" && summary.EndDate != "" {
		return fmt.Sprintf("板块指数窗口收益（请求%s至%s）", summary.StartDate, summary.EndDate)
	}
	if summary.Metric == "windowReturn" {
		return fmt.Sprintf("板块指数窗口收益%d日，偏移%d日", summary.Window, summary.Offset)
	}
	if summary.Metric == "dailyReturn" {
		return "板块指数最近完整交易日单日涨跌幅"
	}
	if summary.Metric == "changePct" {
		return "最近完整交易日单日涨跌幅"
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

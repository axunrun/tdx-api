package main

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/injoyai/tdx/protocol"
)

type AgentStockInSector struct {
	Code       string                   `json:"code"`
	Name       string                   `json:"name,omitempty"`
	Source     string                   `json:"source"`
	Metric     string                   `json:"metric"`
	Sector     AgentBriefBlock          `json:"sector"`
	MemberSize int                      `json:"memberSize"`
	Target     *AgentStockInSectorItem  `json:"target,omitempty"`
	Top        []AgentStockInSectorItem `json:"top"`
	Bottom     []AgentStockInSectorItem `json:"bottom"`
	Note       string                   `json:"note"`
	Warnings   []string                 `json:"warnings,omitempty"`
}

type AgentStockInSectorItem struct {
	Code       string  `json:"code"`
	Name       string  `json:"name,omitempty"`
	Rank       int     `json:"rank"`
	Percentile float64 `json:"percentile"`
	Value      float64 `json:"value"`
	ChangePct  float64 `json:"changePct"`
	Chg5       float64 `json:"chg5"`
	Chg20      float64 `json:"chg20"`
	Chg60      float64 `json:"chg60"`
	PETTM      float64 `json:"peTtm"`
	PEStatic   float64 `json:"peStatic"`
	DivYield   float64 `json:"divYield"`
}

type AgentStockInSectorText struct {
	Code    string `json:"code"`
	Format  string `json:"format"`
	Content string `json:"content"`
}

func handleAgentStockInSector(w http.ResponseWriter, r *http.Request) {
	summary, ok := loadAgentStockInSector(w, r)
	if !ok {
		return
	}
	jsonResp(w, summary)
}

func handleAgentStockInSectorText(w http.ResponseWriter, r *http.Request) {
	summary, ok := loadAgentStockInSector(w, r)
	if !ok {
		return
	}
	jsonResp(w, AgentStockInSectorText{
		Code:    summary.Code,
		Format:  "text/plain; charset=utf-8",
		Content: buildAgentStockInSectorText(summary),
	})
}

func loadAgentStockInSector(w http.ResponseWriter, r *http.Request) (AgentStockInSector, bool) {
	code := normalizeStockCode(r.URL.Query().Get("code"))
	if code == "" {
		jsonErr(w, "缺少code")
		return AgentStockInSector{}, false
	}
	block, err := chooseStockSectorBlock(
		code,
		r.URL.Query().Get("sectorType"),
		r.URL.Query().Get("sectorName"),
	)
	if err != nil {
		jsonErr(w, err.Error())
		return AgentStockInSector{}, false
	}
	members, err := querySectorMemberStocks(block.Type, block.Name)
	if err != nil {
		jsonErr(w, err.Error())
		return AgentStockInSector{}, false
	}
	c := cli()
	if c == nil {
		jsonErr(w, "TDX客户端未连接")
		return AgentStockInSector{}, false
	}
	stats, err := getCachedAgentStats(c)
	if err != nil {
		jsonErr(w, "GetTdxStat失败: "+err.Error())
		return AgentStockInSector{}, false
	}
	limit := parseCount(r.URL.Query().Get("limit"), 10)
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	metric := r.URL.Query().Get("metric")
	return buildAgentStockInSector(code, block, members, stats, metric, limit), true
}

func chooseStockSectorBlock(code, sectorType, sectorName string) (AgentBriefBlock, error) {
	blocks, err := queryStockBlocks(code)
	if err != nil {
		return AgentBriefBlock{}, err
	}
	for _, block := range blocks {
		if sectorName != "" && block.Name != sectorName {
			continue
		}
		if sectorType != "" && block.Type != sectorType {
			continue
		}
		return block, nil
	}
	for _, block := range blocks {
		if block.Type == "concept" {
			return block, nil
		}
	}
	if len(blocks) > 0 {
		return blocks[0], nil
	}
	return AgentBriefBlock{}, fmt.Errorf("未找到板块归属")
}

func buildAgentStockInSector(
	code string,
	block AgentBriefBlock,
	members []stockRow,
	stats []*protocol.TdxStat,
	metric string,
	limit int,
) AgentStockInSector {
	if metric == "" {
		metric = "changePct"
	}
	if limit <= 0 {
		limit = 10
	}
	memberNames := make(map[string]string, len(members))
	memberSet := make(map[string]bool, len(members))
	for _, member := range members {
		memberSet[member.Code] = true
		memberNames[member.Code] = member.Name
	}

	items := make([]AgentStockInSectorItem, 0, len(members))
	for _, stat := range stats {
		if stat == nil || !memberSet[stat.Code] {
			continue
		}
		items = append(items, stockInSectorItem(stat, memberNames[stat.Code], metric))
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Value > items[j].Value
	})
	for i := range items {
		items[i].Rank = i + 1
		items[i].Percentile = float64(len(items)-i) / float64(len(items)) * 100
	}

	var target *AgentStockInSectorItem
	for i := range items {
		if items[i].Code == code {
			item := items[i]
			target = &item
			break
		}
	}

	top := limitStockInSectorItems(items, limit)
	reversed := append([]AgentStockInSectorItem(nil), items...)
	sort.Slice(reversed, func(i, j int) bool {
		return reversed[i].Value < reversed[j].Value
	})

	return AgentStockInSector{
		Code:       code,
		Name:       memberNames[code],
		Source:     "tdx_agent_stock_in_sector",
		Metric:     metric,
		Sector:     block,
		MemberSize: len(items),
		Target:     target,
		Top:        top,
		Bottom:     limitStockInSectorItems(reversed, limit),
		Note:       "板块内位置接口，当前按TdxStat统计字段排序；不拉取全量实时行情，适合判断个股在所属板块中的相对强弱。",
	}
}

func stockInSectorItem(
	stat *protocol.TdxStat,
	name string,
	metric string,
) AgentStockInSectorItem {
	item := AgentStockInSectorItem{
		Code:      stat.Code,
		Name:      name,
		ChangePct: stat.ChangePct,
		Chg5:      stat.Chg5,
		Chg20:     stat.Chg20,
		Chg60:     stat.Chg60,
		PETTM:     stat.PETTM,
		PEStatic:  stat.PEStatic,
		DivYield:  stat.DivYield,
	}
	item.Value = stockInSectorMetricValue(item, metric)
	return item
}

func stockInSectorMetricValue(item AgentStockInSectorItem, metric string) float64 {
	switch metric {
	case "chg5":
		return item.Chg5
	case "chg20":
		return item.Chg20
	case "chg60":
		return item.Chg60
	case "peTtm":
		return item.PETTM
	case "divYield":
		return item.DivYield
	default:
		return item.ChangePct
	}
}

func limitStockInSectorItems(
	items []AgentStockInSectorItem,
	limit int,
) []AgentStockInSectorItem {
	if limit > len(items) {
		limit = len(items)
	}
	return append([]AgentStockInSectorItem(nil), items[:limit]...)
}

func buildAgentStockInSectorText(summary AgentStockInSector) string {
	var b strings.Builder
	if summary.Name != "" {
		b.WriteString(fmt.Sprintf("股票：%s（%s）\n\n", summary.Name, summary.Code))
	} else {
		b.WriteString(fmt.Sprintf("股票代码：%s\n\n", summary.Code))
	}
	b.WriteString("板块位置：\n")
	b.WriteString(fmt.Sprintf(
		"所属%s：%s，样本%d只，排序指标%s。\n",
		summary.Sector.TypeName,
		summary.Sector.Name,
		summary.MemberSize,
		stockInSectorMetricText(summary.Metric),
	))
	if summary.Target != nil {
		b.WriteString(fmt.Sprintf(
			"%s排名 %d/%d，指标值%s，板块百分位%.1f%%。\n",
			valueOrDash(summary.Target.Name),
			summary.Target.Rank,
			summary.MemberSize,
			formatPercentText(summary.Target.Value),
			summary.Target.Percentile,
		))
	}
	if len(summary.Top) > 0 {
		b.WriteString("板块前列：")
		parts := make([]string, 0, len(summary.Top))
		for _, item := range summary.Top {
			parts = append(parts, fmt.Sprintf("%s%s", stockInSectorItemName(item), formatPercentText(item.Value)))
		}
		b.WriteString(strings.Join(parts, "、"))
		b.WriteString("。\n")
	}
	b.WriteString("用途：该接口只比较板块内相对位置；完整板块成分和强弱扫描留给sector-detail或hotspot-scan。")
	return strings.TrimSpace(b.String())
}

func stockInSectorMetricText(metric string) string {
	switch metric {
	case "chg5":
		return "近5日涨跌幅"
	case "chg20":
		return "近20日涨跌幅"
	case "chg60":
		return "近60日涨跌幅"
	case "peTtm":
		return "PE_TTM"
	case "divYield":
		return "股息率"
	default:
		return "当日涨跌幅"
	}
}

func stockInSectorItemName(item AgentStockInSectorItem) string {
	if item.Name != "" {
		return item.Name
	}
	return item.Code
}

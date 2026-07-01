package main

import (
	"fmt"
	"net/http"
	"strings"
)

type AgentAssetSearchResponse struct {
	Source   string             `json:"source"`
	Keyword  string             `json:"keyword"`
	Count    int                `json:"count"`
	Limit    int                `json:"limit"`
	Items    []AgentAssetDetail `json:"items"`
	Warnings []string           `json:"warnings,omitempty"`
}

type AgentAssetSearchText struct {
	Keyword string `json:"keyword"`
	Format  string `json:"format"`
	Content string `json:"content"`
}

type AgentAssetDetail struct {
	Code        string            `json:"code"`
	Name        string            `json:"name"`
	Exchange    string            `json:"exchange"`
	Market      string            `json:"market"`
	AssetType   string            `json:"assetType"`
	DisplayName string            `json:"displayName"`
	Blocks      []AgentBriefBlock `json:"blocks"`
	AgentUsage  string            `json:"agentUsage"`
	Warnings    []string          `json:"warnings,omitempty"`
}

func handleAgentAssetsSearch(w http.ResponseWriter, r *http.Request) {
	summary, ok := loadAgentAssetsSearch(w, r)
	if !ok {
		return
	}
	jsonResp(w, summary)
}

func handleAgentAssetsSearchText(w http.ResponseWriter, r *http.Request) {
	summary, ok := loadAgentAssetsSearch(w, r)
	if !ok {
		return
	}
	jsonResp(w, AgentAssetSearchText{
		Keyword: summary.Keyword,
		Format:  "text/plain; charset=utf-8",
		Content: buildAgentAssetSearchText(summary),
	})
}

func loadAgentAssetsSearch(w http.ResponseWriter, r *http.Request) (AgentAssetSearchResponse, bool) {
	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
	if keyword == "" {
		keyword = strings.TrimSpace(r.URL.Query().Get("q"))
	}
	if keyword == "" {
		jsonErr(w, "缺少keyword")
		return AgentAssetSearchResponse{}, false
	}
	limit := parseCount(r.URL.Query().Get("limit"), 20)
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	stocks, err := searchStocks(keyword, limit)
	if err != nil {
		jsonErr(w, err.Error())
		return AgentAssetSearchResponse{}, false
	}
	items := buildAgentAssetSearchResults(keyword, stocks)
	warnings := make([]string, 0)
	if len(items) == 0 {
		warnings = append(warnings, fmt.Sprintf("查无此股票：未找到与“%s”匹配的A股资产。", keyword))
	}
	return AgentAssetSearchResponse{
		Source:   "tdx_agent_assets",
		Keyword:  keyword,
		Count:    len(items),
		Limit:    limit,
		Items:    items,
		Warnings: warnings,
	}, true
}

func handleAgentAssetsDetail(w http.ResponseWriter, r *http.Request) {
	code := normalizeStockCode(r.URL.Query().Get("code"))
	if code == "" {
		jsonErr(w, "缺少code")
		return
	}

	stock, err := findStockRow(code)
	if err != nil {
		jsonErr(w, err.Error())
		return
	}
	blocks, blockErr := queryStockBlocks(code)
	detail := buildAgentAssetDetailFromStock(stock, blocks)
	if blockErr != nil {
		detail.Warnings = append(detail.Warnings, "板块归属查询失败: "+blockErr.Error())
	}
	jsonResp(w, detail)
}

func buildAgentAssetSearchResults(keyword string, stocks []stockRow) []AgentAssetDetail {
	items := make([]AgentAssetDetail, 0, len(stocks))
	for _, stock := range stocks {
		blocks, err := queryStockBlocks(stock.Code)
		detail := buildAgentAssetDetailFromStock(stock, blocks)
		detail.AgentUsage = "搜索候选项已包含资产详情；如候选唯一，可直接使用code调用后续分析接口。"
		if err != nil {
			detail.Warnings = append(detail.Warnings, "板块归属查询失败: "+err.Error())
		}
		items = append(items, detail)
	}
	return items
}

func buildAgentAssetSearchText(summary AgentAssetSearchResponse) string {
	var b strings.Builder
	if summary.Count == 0 {
		b.WriteString(fmt.Sprintf("资产搜索：查无此股票，未找到与“%s”匹配的A股资产。", summary.Keyword))
		appendWarningsText(&b, summary.Warnings)
		return strings.TrimSpace(b.String())
	}
	b.WriteString(fmt.Sprintf("资产搜索：关键词“%s”命中%d项。\n", summary.Keyword, summary.Count))
	for i, item := range summary.Items {
		parts := []string{fmt.Sprintf("%d. %s", i+1, item.DisplayName)}
		if len(item.Blocks) > 0 {
			parts = append(parts, "板块："+assetSearchBlockNames(item.Blocks, 5))
		}
		if len(item.Warnings) > 0 {
			parts = append(parts, "提示："+strings.Join(item.Warnings, "；"))
		}
		b.WriteString(strings.Join(parts, "；") + "\n")
	}
	appendWarningsText(&b, summary.Warnings)
	return strings.TrimSpace(b.String())
}

func assetSearchBlockNames(blocks []AgentBriefBlock, limit int) string {
	if limit > len(blocks) {
		limit = len(blocks)
	}
	names := make([]string, 0, limit)
	for _, block := range blocks[:limit] {
		names = append(names, block.Name)
	}
	return strings.Join(names, "、")
}

func buildAgentAssetDetailFromStock(stock stockRow, blocks []AgentBriefBlock) AgentAssetDetail {
	if blocks == nil {
		blocks = []AgentBriefBlock{}
	}
	return AgentAssetDetail{
		Code:        stock.Code,
		Name:        stock.Name,
		Exchange:    strings.ToUpper(stock.Exchange),
		Market:      agentMarketName(stock.Exchange),
		AssetType:   "stock",
		DisplayName: agentAssetDisplayName(stock),
		Blocks:      blocks,
		AgentUsage:  "A股资产详情；可作为stock-brief、kline-summary、trade-flow-estimate、f10-summary等接口的入口参数说明。",
	}
}

func findStockRow(code string) (stockRow, error) {
	stocksCacheMu.RLock()
	cache := stocksCache
	stocksCacheMu.RUnlock()
	for _, stock := range cache {
		if stock.Code == code {
			return stock, nil
		}
	}
	if _, err := loadStocksCacheFromDB(); err != nil {
		return stockRow{}, fmt.Errorf("股票名称库为空，请先刷新股票数据")
	}
	stocksCacheMu.RLock()
	cache = stocksCache
	stocksCacheMu.RUnlock()
	for _, stock := range cache {
		if stock.Code == code {
			return stock, nil
		}
	}
	return stockRow{}, fmt.Errorf("未找到资产: %s", code)
}

func agentAssetDisplayName(stock stockRow) string {
	return fmt.Sprintf("%s（%s.%s）", stock.Name, stock.Code, strings.ToUpper(stock.Exchange))
}

func agentMarketName(exchange string) string {
	switch strings.ToLower(exchange) {
	case "sh", "sz", "bj":
		return "A股"
	default:
		return "未知"
	}
}

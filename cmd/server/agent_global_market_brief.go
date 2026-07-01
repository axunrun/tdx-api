package main

import (
	"fmt"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/injoyai/tdx"
	"github.com/injoyai/tdx/protocol"
)

const agentExDailyKlineCategory = 9

var agentExClientMu sync.Mutex

type AgentGlobalMarketBrief struct {
	Source      string                   `json:"source"`
	GeneratedAt string                   `json:"generatedAt"`
	Items       []AgentGlobalMarketItem  `json:"items"`
	Groups      []AgentGlobalMarketGroup `json:"groups"`
	Warnings    []string                 `json:"warnings,omitempty"`
	Limits      map[string]int           `json:"limits"`
	Note        string                   `json:"note"`
}

type AgentGlobalMarketGroup struct {
	Key   string                  `json:"key"`
	Name  string                  `json:"name"`
	Items []AgentGlobalMarketItem `json:"items"`
}

type AgentGlobalMarketItem struct {
	Group      string                 `json:"group"`
	GroupName  string                 `json:"groupName"`
	Code       string                 `json:"code"`
	Name       string                 `json:"name"`
	Market     uint8                  `json:"market"`
	MarketName string                 `json:"marketName"`
	AssetType  string                 `json:"assetType"`
	Reason     string                 `json:"reason"`
	Price      float64                `json:"price,omitempty"`
	PreClose   float64                `json:"preClose,omitempty"`
	ChangePct  float64                `json:"changePct,omitempty"`
	Range20    AgentGlobalMarketRange `json:"range20"`
	Range60    AgentGlobalMarketRange `json:"range60"`
	Warnings   []string               `json:"warnings,omitempty"`
}

type AgentGlobalMarketRange struct {
	Available   bool    `json:"available"`
	Days        int     `json:"days"`
	ReturnPct   float64 `json:"returnPct,omitempty"`
	High        float64 `json:"high,omitempty"`
	Low         float64 `json:"low,omitempty"`
	PositionPct float64 `json:"positionPct,omitempty"`
	Reason      string  `json:"reason,omitempty"`
}

type AgentGlobalMarketBriefText struct {
	Format  string `json:"format"`
	Content string `json:"content"`
}

type agentGlobalMarketSeed struct {
	Group      string
	GroupName  string
	Code       string
	Name       string
	Market     uint8
	MarketName string
	AssetType  string
	Reason     string
}

var agentGlobalMarketSeeds = []agentGlobalMarketSeed{
	{"risk", "全球风险偏好", "SPY", "标普500ETF", 74, "美国股票", "index_etf", "全球权益风险偏好核心代理"},
	{"risk", "全球风险偏好", "A_NDX", "纳斯达克100", 12, "国际指数", "index", "科技成长股风险偏好核心指标"},
	{"risk", "全球风险偏好", "DIA", "道琼斯工业ETF", 74, "美国股票", "index_etf", "美国传统蓝筹和工业周期代理"},
	{"risk", "全球风险偏好", "IWM", "罗素2000ETF", 74, "美国股票", "index_etf", "美国中小盘风险偏好代理"},
	{"risk", "全球风险偏好", "CNY0", "富时A50期指连续", 12, "国际指数", "index_future", "外资视角下的A股权重股预期"},
	{"risk", "全球风险偏好", "FEZ", "欧洲STOXX50ETF", 74, "美国股票", "index_etf", "欧洲蓝筹权益风险偏好代理"},

	{"apac", "亚太核心市场", "HSI", "恒生指数", 27, "香港指数", "index", "中国资产离岸定价"},
	{"apac", "亚太核心市场", "HZ5017", "恒生科技指数", 27, "香港指数", "index", "港股科技成长风险偏好"},
	{"apac", "亚太核心市场", "NK0Y", "日经225期指连续", 12, "国际指数", "index", "日本股市和亚太风险偏好"},
	{"apac", "亚太核心市场", "EWJ", "日本ETF", 74, "美国股票", "index_etf", "日本宽基市场代理"},
	{"apac", "亚太核心市场", "513800", "日本东证指数ETF", 33, "基金", "index_etf", "TOPIX/东证指数代理"},
	{"apac", "亚太核心市场", "D_NQKR", "纳斯达克韩国指数", 12, "国际指数", "index", "TDX可用韩国市场指数代理，不等同KOSPI本体"},
	{"apac", "亚太核心市场", "EWY", "韩国ETF", 74, "美国股票", "index_etf", "韩国市场和半导体链风险偏好代理"},
	{"apac", "亚太核心市场", "MTWL8", "MSCI台湾指数主连", 23, "全球期指", "index_future", "TDX可用台湾市场指数代理，不等同台湾加权本体"},
	{"apac", "亚太核心市场", "EWT", "台湾ETF", 74, "美国股票", "index_etf", "台湾市场和半导体链风险偏好代理"},
	{"apac", "亚太核心市场", "INDY", "Nifty 50指数ETF", 74, "美国股票", "index_etf", "印度市场风险偏好代理"},

	{"commodity", "大宗商品", "CL00Y", "WTI原油连续", 17, "NYMEX", "commodity", "能源价格、通胀和地缘风险"},
	{"commodity", "大宗商品", "BZ00Y", "布伦特原油连续", 17, "NYMEX", "commodity", "国际油价主基准"},
	{"commodity", "大宗商品", "NG00Y", "NYMEX天然气连续", 17, "NYMEX", "commodity", "能源价格波动"},
	{"commodity", "大宗商品", "GC00Y", "COMEX黄金连续", 16, "COMEX", "commodity", "避险、美元和实际利率"},
	{"commodity", "大宗商品", "SI00Y", "COMEX白银连续", 16, "COMEX", "commodity", "贵金属与工业属性"},
	{"commodity", "大宗商品", "HG00Y", "COMEX铜连续", 16, "COMEX", "commodity", "全球制造业景气度"},
	{"commodity", "大宗商品", "ZS00Y", "CBOT大豆连续", 18, "CBOT", "commodity", "农产品通胀与饲料链"},
	{"commodity", "大宗商品", "T002", "商品指数-农产品", 42, "商品指数", "commodity_index", "农产品板块整体表现"},
	{"commodity", "大宗商品", "T003", "商品指数-工业品", 42, "商品指数", "commodity_index", "工业品整体表现"},
	{"commodity", "大宗商品", "T012", "商品指数-贵金属", 42, "商品指数", "commodity_index", "贵金属整体表现"},

	{"fx", "汇率", "USDCNH", "美元兑离岸人民币", 10, "基本汇率", "fx", "人民币外部压力和外资风险偏好"},
	{"fx", "汇率", "USDJPY", "美元兑日元", 10, "基本汇率", "fx", "套息交易和亚太流动性"},
	{"fx", "汇率", "EURUSD", "欧元兑美元", 10, "基本汇率", "fx", "美元强弱辅助判断"},
	{"fx", "汇率", "UUP", "美元指数ETF", 74, "美国股票", "fx_etf", "美元指数可交易代理，观察美元强弱"},

	{"bond", "利率与债券", "SHY", "1-3年美债ETF", 74, "美国股票", "bond_etf", "美国短端利率压力代理，价格与收益率大体反向"},
	{"bond", "利率与债券", "IEF", "7-10年美债ETF", 74, "美国股票", "bond_etf", "美国中长端利率压力代理，价格与收益率大体反向"},
	{"bond", "利率与债券", "TLT", "20年期以上美债ETF", 74, "美国股票", "bond_etf", "美国长端利率和久期资产压力代理，价格与收益率大体反向"},
	{"bond", "利率与债券", "CBON", "中国债券ETF", 74, "美国股票", "bond_etf", "海外市场中的中国债券代理"},

	{"leader", "全球权重股", "NVDA", "NVIDIA", 74, "美国股票", "stock", "AI、半导体和全球成长股风向标"},
	{"leader", "全球权重股", "MSFT", "微软", 74, "美国股票", "stock", "AI软件、云计算和纳指权重"},
	{"leader", "全球权重股", "AAPL", "苹果", 74, "美国股票", "stock", "消费电子和纳指权重"},
	{"leader", "全球权重股", "BRK.B", "伯克希尔哈撒韦B", 74, "美国股票", "stock", "美国多行业蓝筹和价值股风险偏好"},
	{"leader", "全球权重股", "JPM", "摩根大通", 74, "美国股票", "stock", "美国金融体系和利率周期权重股"},
	{"leader", "全球权重股", "XOM", "埃克森美孚", 74, "美国股票", "stock", "能源行业权重股和油价链条代理"},
	{"leader", "全球权重股", "LLY", "礼来", 74, "美国股票", "stock", "美国医药创新和防御成长权重"},
	{"leader", "全球权重股", "UNH", "联合健康", 74, "美国股票", "stock", "美国医疗服务和防御权重"},
	{"leader", "全球权重股", "WMT", "沃尔玛", 74, "美国股票", "stock", "美国消费防御和居民消费代理"},
	{"leader", "全球权重股", "V", "Visa", 74, "美国股票", "stock", "支付网络和消费交易活跃度代理"},
	{"leader", "全球权重股", "PG", "宝洁", 74, "美国股票", "stock", "日常消费品防御权重"},
	{"leader", "全球权重股", "HD", "家得宝", 74, "美国股票", "stock", "美国地产后周期和家居消费代理"},
	{"leader", "全球权重股", "TM", "丰田汽车", 74, "美国股票", "stock", "日本制造业、汽车链和日元敏感权重股"},
	{"leader", "全球权重股", "SONY", "索尼", 74, "美国股票", "stock", "日本科技消费电子和娱乐内容权重股"},
	{"leader", "全球权重股", "TSM", "台积电", 74, "美国股票", "stock", "全球半导体核心资产"},
	{"leader", "全球权重股", "SPCX", "SpaceX", 74, "美国股票", "stock", "商业航天和高估值风险偏好观察"},
	{"leader", "全球权重股", "00700", "腾讯控股", 31, "港股", "stock", "港股科技核心权重"},
	{"leader", "全球权重股", "09988", "阿里巴巴", 31, "港股", "stock", "中概和港股科技核心权重"},
	{"leader", "全球权重股", "03690", "美团", 31, "港股", "stock", "中国消费与互联网情绪"},
}

func handleAgentGlobalMarketBrief(w http.ResponseWriter, r *http.Request) {
	summary := buildAgentGlobalMarketBrief()
	jsonResp(w, summary)
}

func handleAgentGlobalMarketBriefText(w http.ResponseWriter, r *http.Request) {
	summary := buildAgentGlobalMarketBrief()
	jsonResp(w, AgentGlobalMarketBriefText{
		Format:  "text/plain; charset=utf-8",
		Content: buildAgentGlobalMarketBriefText(summary),
	})
}

func buildAgentGlobalMarketBrief() AgentGlobalMarketBrief {
	warnings := make([]string, 0)
	ex, err := ensureAgentExClient()
	if err != nil {
		warnings = append(warnings, "扩展行情连接失败: "+err.Error())
	}
	items := make([]AgentGlobalMarketItem, 0, len(agentGlobalMarketSeeds))
	for _, seed := range agentGlobalMarketSeeds {
		items = append(items, buildAgentGlobalMarketItem(ex, seed))
	}
	return AgentGlobalMarketBrief{
		Source:      "tdx_agent_global_market_brief",
		GeneratedAt: time.Now().Format(time.RFC3339),
		Items:       items,
		Groups:      groupAgentGlobalMarketItems(items),
		Warnings:    warnings,
		Limits:      map[string]int{"dailyBars": 61, "range20": 20, "range60": 60},
		Note:        "外围权重资产聚合接口；每次调用实时读取TDX扩展行情，不使用数据库；20/60日区间基于扩展日K线本地计算。",
	}
}

func ensureAgentExClient() (*tdx.Client, error) {
	agentExClientMu.Lock()
	defer agentExClientMu.Unlock()
	if exClient != nil {
		return exClient, nil
	}
	c, err := tdx.DialExHqDefault()
	if err != nil {
		return nil, err
	}
	exClient = c
	return exClient, nil
}

func buildAgentGlobalMarketItem(ex *tdx.Client, seed agentGlobalMarketSeed) AgentGlobalMarketItem {
	item := AgentGlobalMarketItem{
		Group:      seed.Group,
		GroupName:  seed.GroupName,
		Code:       seed.Code,
		Name:       seed.Name,
		Market:     seed.Market,
		MarketName: seed.MarketName,
		AssetType:  seed.AssetType,
		Reason:     seed.Reason,
	}
	if ex == nil {
		item.Warnings = append(item.Warnings, "扩展行情未连接")
		return item
	}
	if quote, err := ex.ExQuote(seed.Market, seed.Code); err == nil && quote != nil {
		item.Price = quote.Price
		item.PreClose = quote.PreClose
		if quote.PreClose > 0 && quote.Price > 0 {
			item.ChangePct = (quote.Price - quote.PreClose) / quote.PreClose * 100
		}
	} else {
		item.Warnings = append(item.Warnings, "报价获取失败")
	}
	bars, err := ex.ExBars(agentExDailyKlineCategory, seed.Market, seed.Code, 0, 61)
	if err != nil {
		item.Warnings = append(item.Warnings, "日K获取失败")
		return item
	}
	if item.Price <= 0 && len(bars) > 0 {
		item.Price = bars[len(bars)-1].Close
	}
	item.Range20 = buildAgentGlobalMarketRange(bars, item.Price, 20)
	item.Range60 = buildAgentGlobalMarketRange(bars, item.Price, 60)
	return item
}

func buildAgentGlobalMarketRange(
	bars []protocol.ExKline,
	price float64,
	days int,
) AgentGlobalMarketRange {
	if len(bars) <= days || price <= 0 {
		return AgentGlobalMarketRange{
			Available: false,
			Days:      days,
			Reason:    fmt.Sprintf("日K不足%d根", days+1),
		}
	}
	window := bars[len(bars)-days:]
	high := -math.MaxFloat64
	low := math.MaxFloat64
	for _, bar := range window {
		if bar.High > high {
			high = bar.High
		}
		if bar.Low > 0 && bar.Low < low {
			low = bar.Low
		}
	}
	base := bars[len(bars)-days-1].Close
	if base <= 0 || high <= 0 || low == math.MaxFloat64 {
		return AgentGlobalMarketRange{
			Available: false,
			Days:      days,
			Reason:    "日K价格异常",
		}
	}
	position := 0.0
	if high > low {
		position = (price - low) / (high - low) * 100
	}
	return AgentGlobalMarketRange{
		Available:   true,
		Days:        days,
		ReturnPct:   (price - base) / base * 100,
		High:        high,
		Low:         low,
		PositionPct: position,
	}
}

func groupAgentGlobalMarketItems(items []AgentGlobalMarketItem) []AgentGlobalMarketGroup {
	order := []struct {
		key  string
		name string
	}{
		{"risk", "全球风险偏好"},
		{"apac", "亚太核心市场"},
		{"commodity", "大宗商品"},
		{"fx", "汇率"},
		{"bond", "利率与债券"},
		{"leader", "全球权重股"},
	}
	groups := make([]AgentGlobalMarketGroup, 0, len(order))
	for _, spec := range order {
		group := AgentGlobalMarketGroup{Key: spec.key, Name: spec.name}
		for _, item := range items {
			if item.Group == spec.key {
				group.Items = append(group.Items, item)
			}
		}
		if len(group.Items) > 0 {
			groups = append(groups, group)
		}
	}
	return groups
}

func buildAgentGlobalMarketBriefText(summary AgentGlobalMarketBrief) string {
	var b strings.Builder
	b.WriteString("外围权重资产概览\n")
	for _, group := range summary.Groups {
		b.WriteString("\n")
		b.WriteString(group.Name)
		b.WriteString("：\n")
		for _, item := range group.Items {
			b.WriteString("- ")
			b.WriteString(formatAgentGlobalMarketItemText(item))
			b.WriteString("\n")
		}
	}
	appendWarningsText(&b, summary.Warnings)
	return strings.TrimSpace(b.String())
}

func formatAgentGlobalMarketItemText(item AgentGlobalMarketItem) string {
	parts := []string{
		fmt.Sprintf("%s（%s）现价%.2f，当日%s", item.Name, item.Code, item.Price, formatPercentText(item.ChangePct)),
		formatAgentGlobalMarketRangeText("近20日", item.Range20),
		formatAgentGlobalMarketRangeText("近60日", item.Range60),
		item.Reason,
	}
	if len(item.Warnings) > 0 {
		parts = append(parts, "提示: "+strings.Join(item.Warnings, "；"))
	}
	return strings.Join(parts, "；")
}

func formatAgentGlobalMarketRangeText(label string, r AgentGlobalMarketRange) string {
	if !r.Available {
		return label + "不可用（" + r.Reason + "）"
	}
	return fmt.Sprintf(
		"%s%s，区间%.2f-%.2f，位置%s",
		label,
		formatPercentText(r.ReturnPct),
		r.Low,
		r.High,
		formatAgentGlobalMarketPositionText(r.PositionPct),
	)
}

func formatAgentGlobalMarketPositionText(value float64) string {
	return fmt.Sprintf("%.2f%%", value)
}

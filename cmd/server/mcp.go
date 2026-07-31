package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
)

type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type mcpResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *mcpError       `json:"error,omitempty"`
}

type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type mcpTool struct {
	Name         string           `json:"name"`
	Description  string           `json:"description"`
	InputSchema  map[string]any   `json:"inputSchema"`
	OutputSchema map[string]any   `json:"outputSchema,omitempty"`
	Path         string           `json:"-"`
	Handler      http.HandlerFunc `json:"-"`
}

type mcpToolParam struct {
	Name        string
	Type        string
	Description string
	Required    bool
	Enum        []string
	Default     any
	Schema      map[string]any
}

type mcpToolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

func handleMCP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		jsonResp(w, map[string]any{
			"name":      "tdx-api-mcp",
			"endpoint":  "/mcp",
			"transport": "streamable-http",
		})
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req mcpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeMCPError(w, nil, -32700, "JSON解析失败: "+err.Error())
		return
	}
	if len(req.ID) == 0 && strings.HasPrefix(req.Method, "notifications/") {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	switch req.Method {
	case "initialize":
		writeMCPResult(w, req.ID, map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]any{
				"tools": map[string]any{},
			},
			"serverInfo": map[string]any{
				"name":    "tdx-api",
				"version": "1.0.0",
			},
		})
	case "tools/list":
		writeMCPResult(w, req.ID, map[string]any{"tools": mcpTools()})
	case "tools/call":
		result, err := callMCPTool(req.Params)
		if err != nil {
			writeMCPError(w, req.ID, -32602, err.Error())
			return
		}
		writeMCPResult(w, req.ID, result)
	default:
		writeMCPError(w, req.ID, -32601, "不支持的MCP方法: "+req.Method)
	}
}

func mcpTools() []mcpTool {
	klineTool := newMCPTool(
		"tdx_kline",
		"原始K线行情结构化数据。用于获取逐日或逐周期的开盘价、最高价、最低价、收盘价、成交量和成交额；逐日收盘价序列读取type=day结果的list[].date与list[].close。需要低Token趋势判断时优先调用tdx_kline_summary_text。",
		"/api/kline",
		handleKline,
		mcpToolParam{
			Name:        "code",
			Type:        "string",
			Description: "必填，6位A股股票代码，例如300499。",
			Required:    true,
			Schema:      map[string]any{"pattern": `^\d{6}$`},
		},
		optionalEnumDefault(
			"type",
			"K线周期：day日线（默认）、week周线、month月线、quarter季线、year年线。返回最近count个对应周期；逐日收盘价必须使用day。",
			"day",
			"day", "week", "month", "quarter", "year",
		),
		optionalIntegerDefault(
			"count",
			"返回最近K线数量，默认10，范围1-800。数量越大返回内容和Token占用越高。",
			10,
			1,
			800,
		),
	)
	klineTool.OutputSchema = klineToolOutputSchema()

	tools := []mcpTool{
		newMCPTool("tdx_asset_search_text", "按名称、代码或拼音搜索A股资产。用于用户只给股票名称或模糊关键词时先确认标准代码；返回候选股票、市场、名称和主要板块。", "/api/agent/assets/search-text", handleAgentAssetsSearchText,
			requiredString("keyword", "股票名称、代码或拼音关键词"),
			optionalIntegerDefault("limit", "返回数量，默认20，最大50。", 20, 1, 50),
		),
		klineTool,
		newMCPTool("tdx_stock_brief_text", "单只A股基本快照，不等于深度买卖建议。输出查询时点行情、基本面、最新财报、所属板块、估值和数据一致性；不包含技术指标、5/20/60日涨跌及52周区间。交易时段行情按实时快照，午休使用上午最后行情，盘后使用当日收盘，盘前和非交易日使用最近完整交易日，并明确查询时间、行情数据日期和状态；财务及估值保留各自报告或统计日期。技术判断另调用tdx_technical_score_text，价格历史结构另调用tdx_kline_summary_text。", "/api/agent/stock-brief-text", handleAgentStockBriefText,
			requiredString("code", "股票代码，例如300499"),
			optionalMarket("mkt"),
		),
		newMCPTool("tdx_kline_summary_text", "K线价格结构摘要，不重复technical-score的技术指标。输出日/周/月区间涨跌、最高最低、最大回撤、区间波动、价格位置、K线形态、连续涨跌和关键价位，日线另含基于最近250根日K计算的近52周区间。近N日收益按当前收盘与N个交易日前收盘计算；交易时段纳入未完成日K，午休使用上午最后日K，盘后使用当日收盘，盘前和非交易日使用最近完整交易日，并明确查询时间、数据日期和状态。", "/api/agent/kline-summary-text", handleAgentKlineSummaryText,
			requiredString("code", "股票代码，例如300499"),
			optionalEnumDefault("level", "输出深度：brief为简版，normal为常规版，deep为深度版。默认normal。", "normal", "brief", "normal", "deep"),
			optionalInteger("dayCount", "日线展示与阶段统计数量，最大500；不传时按level使用brief=60、normal=120、deep=250。近52周区间始终独立使用最近250根日K计算。", map[string]any{"minimum": 1, "maximum": 500}),
			optionalEnumDefault("adjust", "日线价格复权口径：qfq前复权默认，none未复权；周/月线使用现有TDX口径。", "qfq", "qfq", "none"),
		),
		newMCPTool("tdx_trade_flow_estimate_text", "分档资金流估算。用于观察指定交易日超大单、大单、中单、小单的流入流出；优先使用近60个交易日逐笔成交金额计算的自适应阈值，缓存缺失或口径不匹配时明确回退固定金额阈值。", "/api/agent/trade-flow-estimate-text", handleAgentTradeFlowEstimateText,
			requiredString("code", "股票代码，例如300499"),
			optionalDateString("date", "交易日期，YYYY-MM-DD或YYYYMMDD；不传默认今天"),
		),
		newMCPTool("tdx_margin_trading_text", "个股融资融券逐日明细。直连股票所属交易所的官方披露数据，按最新已披露记录向前返回指定数量的交易日；不把周末、节假日或尚未披露的日期计入days。交易所接口可联通但回溯后无该证券明细时，判定该证券当前不是融资融券标的。用于观察融资余额、融资买入、融资偿还、融券卖出、融券余量及交易所可提供的两融余额变化。", "/api/agent/margin-trading-text", handleAgentMarginTradingText,
			requiredString("code", "必填，6位A股股票或A股ETF代码，例如300499、603063"),
			optionalIntegerDefault("days", "返回最近已披露的交易日记录数量，默认30，范围1-120。days=30表示从交易所最新一条可用记录向前取30个实际交易日，不是最近30个自然日。", 30, 1, 120),
		),
		newMCPTool("tdx_f10_summary_text", "低频深度F10摘要。用于深度个股研究和风险线索复核；输出经降噪后的股本、股东、机构持股、分红融资、经营分析、行业分析和F10风险线索，不重复stock-brief估值字段。", "/api/agent/f10-summary-text", handleAgentF10SummaryText,
			requiredString("code", "股票代码，例如300499"),
			optionalMarket("mkt"),
		),
		newMCPTool("tdx_sector_membership_text", "查询个股完整板块归属，用于不需要行情和基本面的纯板块映射流程。tdx_stock_brief_text已包含所属板块；已调用brief时通常无需重复调用本工具。", "/api/agent/sector-membership-text", handleAgentSectorMembershipText,
			requiredString("code", "股票代码，例如300499"),
		),
		newMCPTool("tdx_stock_in_sector_text", "个股在板块内的相对位置。用于判断目标股相对同板块股票是强势、中游还是落后。", "/api/agent/stock-in-sector-text", handleAgentStockInSectorText,
			requiredString("code", "股票代码，例如300499"),
			optionalEnumDefault("sectorType", "板块类型：concept为概念板块，style_region为地域/风格板块，index为指数板块；默认concept。", "concept", "concept", "style_region", "index"),
			optionalString("sectorName", "板块名称；留空时默认选择第一个概念板块"),
			optionalEnumDefault("metric", "成分股排序指标：changePct为输出所标TdxStat完整统计日的单日涨跌（非盘中实时），chg5近5日，chg20近20日，chg60近60日，peTtm市盈率，divYield股息率。默认chg20。", "chg20", "changePct", "chg5", "chg20", "chg60", "peTtm", "divYield"),
			optionalIntegerDefault("limit", "返回成分股数量，默认10，最大50。", 10, 1, 50),
		),
		newMCPTool("tdx_sector_detail_text", "指定板块深度分析。通常先用tdx_hotspot_scan_text发现候选板块，再用本工具深入指定板块；调用后无需重复解读热点扫描中该板块的简略数据。sectorName和indexCode至少传一个；输出板块指数最近完整交易日单日涨跌及日期、截至该日的近20/60日表现，并另列所标TdxStat完整统计日的成分股上涨比例、强势股、中游股和弱势股。板块指数完整日与成分股统计日分别标注，盘中不会把未收盘数据冒充完整日数据。", "/api/agent/sector-detail-text", handleAgentSectorDetailText,
			optionalString("sectorName", "TDX板块名称，必须与TDX返回名称精确一致，例如液冷服务；sectorName和indexCode至少传一个。名称不确定时优先传indexCode。"),
			optionalString("indexCode", "TDX板块指数代码，例如880685；sectorName和indexCode至少传一个。"),
			optionalEnumDefault("sectorType", "板块类型：concept概念，style_region地域/风格，index指数；默认concept。", "concept", "concept", "style_region", "index"),
			optionalEnumDefault("metric", "成分股排序指标：changePct为输出所标TdxStat完整统计日的单日涨跌（非盘中实时），chg5近5日，chg20近20日，chg60近60日，peTtm市盈率，divYield股息率。默认chg20。", "chg20", "changePct", "chg5", "chg20", "chg60", "peTtm", "divYield"),
			optionalIntegerDefault("topStocks", "强势、中游、弱势各最多返回的成分股数量，默认各10只，最大各30只。", 10, 1, 30),
			optionalBoolDefault("excludeNew", "是否过滤异常样本，默认true：排除名称以N/C开头、TdxStat单日涨幅超过100%的股票；metric为changePct/chg5/chg20/chg60时还排除对应排序值超过100%的股票。", true),
		),
		newMCPTool("tdx_sector_realtime_text", "指定题材板块盘中实时涨跌。仅在工作日09:30-11:30、13:00-15:00连续交易时段返回TDX板块指数实时涨跌幅、交易日期和查询时点；盘前、午休、收盘后及周末明确返回非交易时间且不回退为历史数据。sectorName和indexCode至少传一个。", "/api/agent/sector-realtime-text", handleAgentSectorRealtimeText,
			optionalString("sectorName", "TDX板块名称，例如液冷服务；sectorName和indexCode至少传一个。名称必须与TDX板块名称一致。"),
			optionalString("indexCode", "TDX板块指数代码，例如880685；sectorName和indexCode至少传一个。"),
			optionalEnumDefault("sectorType", "板块类型：concept概念，style_region地域/风格，index指数；默认concept。", "concept", "concept", "style_region", "index"),
		),
		newMCPTool("tdx_hotspot_scan_text", "板块冷热扫描。dailyReturn按TDX板块指数最近完整交易日单日涨跌排序；chg5/chg20/chg60/changePct按TdxStat成分股平均值排序。上述标准口径的每个入榜板块都固定输出最近完整交易日板块指数单日涨跌及日期，20/60日等口径同时保留与TdxStat统计日对齐的同周期板块指数收益；成分股上涨比例明确使用所标TdxStat完整统计日。windowReturn按指定指数区间排序。", "/api/agent/hotspot-scan-text", handleAgentHotspotScanText,
			optionalEnumDefault("sectorType", "扫描板块类型：concept概念板块，style_region地域/风格板块，index指数板块；默认concept。", "concept", "concept", "style_region", "index"),
			optionalEnumDefault("metric", "排序口径：dailyReturn按TDX板块指数最近完整交易日单日涨跌幅排序；changePct按最新TdxStat完整统计日的成分股平均单日涨跌排序，不是盘中实时值；chg5/chg20/chg60按成分股平均区间涨跌排序；windowReturn按TDX板块指数日K区间收益排序。dailyReturn及所有TdxStat标准口径都固定输出最近完整交易日板块指数单日涨跌及日期；chg5/chg20/chg60另输出同周期指数收益；上涨比例使用所标TdxStat完整统计日。默认chg20。", "chg20", "dailyReturn", "chg5", "chg20", "chg60", "changePct", "windowReturn"),
			optionalDateString("startDate", "windowReturn请求窗口开始日期，YYYY-MM-DD或YYYYMMDD；startDate和endDate必须同时提供，且优先于window/offset。输出另行标明实际采用的首个交易日。"),
			optionalDateString("endDate", "windowReturn请求窗口结束日期，YYYY-MM-DD或YYYYMMDD；startDate和endDate必须同时提供，且endDate不得早于startDate。输出另行标明实际采用的末个交易日。"),
			optionalIntegerDefault("window", "仅用于未传startDate/endDate的windowReturn：区间交易日数，默认20，范围1-250。", 20, 1, 250),
			optionalIntegerDefault("offset", "仅用于未传startDate/endDate的windowReturn：从最新板块指数交易日向前偏移的交易日数，默认0，范围0-500。", 0, 0, 500),
			optionalIntegerDefault("limit", "强势/中游/弱势各返回数量，默认20，最大50；可排序板块不足时实际数量会减少并输出warning。", 20, 1, 50),
			optionalIntegerDefault("topStocks", "每个板块返回的股票数量，默认3，最大10；chg5/chg20/chg60/changePct按所选成分股指标返回；dailyReturn和windowReturn的代表股使用输出所标TdxStat完整统计日单日涨跌。", 3, 1, 10),
			optionalIntegerDefault("minMembers", "板块最少有效成分股数量，默认20；按输出所标TdxStat完整统计日匹配并执行excludeNew过滤后，样本不足的板块不参与排序。", 20, 1, 0),
			optionalBoolDefault("excludeNew", "是否排除名称以N/C开头的新股及TdxStat完整统计日涨幅超过100%的异常样本，默认true。", true),
		),
		newMCPTool("tdx_multi_brief_text", "多股快速概览，是多股筛选的批量替代路径。批量输出行情、20日表现、板块及日线MA、MACD、RSI、BOLL、KDJ、BIAS、ATR、OBV、量价和逐笔成交估算的多空比；共同指标与technical-score使用同一计算方法，至少250根预热。调用后不要再对同一批股票逐只调用tdx_stock_brief_text和tdx_technical_score_text；仅对筛选出的重点个股进入单股深度组合。交易时段纳入实时行情和未完成日K，午休使用上午最后行情，盘后使用当日收盘，盘前和非交易日使用最近完整交易日，并为每只股票明确数据日期及状态。ATR和OBV只作观察，不直接代表方向。", "/api/agent/multi-brief-text", handleAgentMultiBriefText,
			requiredString("codes", "逗号分隔股票代码，最多20只，例如300499,603063"),
			optionalEnumDefault("adjust", "日线价格复权口径：qfq前复权默认，none未复权。", "qfq", "qfq", "none"),
		),
		newMCPTool("tdx_auction_text", "集合竞价摘要。用于开盘前后判断竞价强弱；默认分析09:20-09:25不可撤单阶段。", "/api/agent/auction-text", handleAgentAuctionText,
			requiredString("code", "股票代码，例如300499"),
			optionalEnumDefault("session", "竞价阶段：open开盘集合竞价默认，close收盘集合竞价，all全量竞价记录。", "open", "open", "close", "all"),
			optionalIntegerDefault("limit", "返回记录数量，默认20，最大100。", 20, 1, 100),
		),
		newMCPTool("tdx_market_review_text", "市场级复盘。用于判断A股整体环境；严格分开输出当前交易日实时广度与最近完整交易日盘后广度，两者均标注日期、时点、A股有效样本数和数据源，不会用昨日数据冒充今日。只提供少量强/中/弱板块环境摘要；需要完整板块排序、周期比较和代表股时再调用tdx_hotspot_scan_text。", "/api/agent/market-review-text", handleAgentMarketReviewText,
			optionalEnumDefault("session", "复盘视角：auto按系统时间识别非交易日、盘前、09:20-09:25集合竞价、竞价结束、盘中、午休和收盘；current/morning/full仅调整文本视角，不会回溯或改写数据日期。默认auto。", "auto", "auto", "current", "morning", "full"),
			optionalString("codes", "可选关注股，逗号分隔"),
			optionalIntegerDefault("top", "强/中/弱板块数量，默认10，最大20。", 10, 1, 20),
		),
		newMCPTool("tdx_intraday_alerts_text", "关注股盘中异动快照。用于交易时段轮询关注池；输出当前行情、短时涨跌、短时放量和异动信号。", "/api/agent/intraday-alerts-text", handleAgentIntradayAlertsText,
			requiredString("codes", "逗号分隔股票代码，最多20只"),
			optionalIntegerDefault("windowMinutes", "分时窗口分钟数，默认30，范围5-60。", 30, 5, 60),
		),
		newMCPTool("tdx_global_market_brief_text", "全球外围权重资产简报。用于A股分析前判断外围环境；输出全球指数、亚太市场、商品、汇率、债券和权重股的当日及20/60日表现。", "/api/agent/global-market-brief-text", handleAgentGlobalMarketBriefText),
		newMCPTool("tdx_scenario_valuation_text", "三情景估值计算。复用stock-brief的价格、PE和财务同比；只计算目标EPS、目标价、涨跌幅和年化收益，不输出买卖建议。", "/api/agent/scenario-valuation-text", handleAgentScenarioValuationText,
			requiredString("code", "股票代码，例如300499"),
			optionalMarket("mkt"),
			optionalIntegerDefault("years", "估值期限，默认3年，范围1-10。", 3, 1, 10),
			optionalNumberSchema("currentPrice", "可选当前价格；必须为正数；不传则使用行情现价。", positiveNumberSchema()),
			optionalNumber("eps", "可选EPS；可为负，但 EPS<=0 时 PE 情景估值会返回不适用说明。"),
			optionalNumberSchema("bearGrowth", "悲观年增速，单位%；-100表示归零，1000为防呆上限。", growthPctSchema()),
			optionalNumberSchema("baseGrowth", "中性年增速，单位%；-100表示归零，1000为防呆上限。", growthPctSchema()),
			optionalNumberSchema("bullGrowth", "乐观年增速，单位%；-100表示归零，1000为防呆上限。", growthPctSchema()),
			optionalNumberSchema("bearPE", "悲观情景目标PE，必须为正数。", positiveNumberSchema()),
			optionalNumberSchema("basePE", "中性情景目标PE，必须为正数。", positiveNumberSchema()),
			optionalNumberSchema("bullPE", "乐观情景目标PE，必须为正数。", positiveNumberSchema()),
			optionalEnumDefault("assumptionMode", "假设来源说明，当前仅影响调用语义：conservative默认、analyst_forecast、manual。", "conservative", "conservative", "analyst_forecast", "manual"),
			optionalEnumDefault("level", "输出深度：brief、normal、deep；当前保持同一计算口径。", "normal", "brief", "normal", "deep"),
		),
		newMCPTool("tdx_implied_expectation_text", "当前价格隐含预期计算。用当前价、EPS和目标PE反推未来EPS与隐含年复合增速；只做公式计算，不输出买卖建议。", "/api/agent/implied-expectation-text", handleAgentImpliedExpectationText,
			requiredString("code", "股票代码，例如300499"),
			optionalMarket("mkt"),
			optionalIntegerDefault("years", "估值期限，默认3年，范围1-10。", 3, 1, 10),
			optionalNumberSchema("currentPrice", "可选当前价格；必须为正数；不传则使用行情现价。", positiveNumberSchema()),
			optionalNumber("eps", "可选EPS；可为负，但 EPS<=0 时隐含预期公式不适用。"),
			optionalNumberSchema("targetPE", "目标PE，必须为正数；不传则用当前PE_TTM的保守折扣或默认25。", positiveNumberSchema()),
			optionalEnumDefault("level", "输出深度：brief、normal、deep；当前保持同一计算口径。", "normal", "brief", "normal", "deep"),
		),
		newMCPTool("tdx_technical_score_text", "统一技术评分。输出日/周/月MA、MACD、RSI、BOLL、KDJ、BIAS、ATR、OBV、量价，日线另含逐笔成交估算的主动买卖金额多空比；该多空比不是分档资金流，不替代tdx_trade_flow_estimate_text。ATR和OBV为观察项、固定0分，不改变技术总分。输出查询时间及各周期数据日期；交易时段按未完成日K动态计算，午休使用上午最后日K，盘后使用当日收盘，盘前和非交易日使用最近完整交易日。递推指标至少使用250根可用K线预热；RSI使用Wilder平滑，KDJ(9,3,3)逐根递推并通过前后周期确认交叉。", "/api/agent/technical-score-text", handleAgentTechnicalScoreText,
			requiredString("code", "股票代码，例如300499"),
			optionalIntegerDefault("dayCount", "日线参数，默认250，范围60-500；小于250时仍请求至少250根用于递推指标预热。", 250, 60, 500),
			optionalEnumDefault("adjust", "日线价格复权口径：qfq前复权默认，none未复权；周/月线暂使用现有TDX口径。", "qfq", "qfq", "none"),
			optionalBoolDefault("includeWeeklyMonthly", "是否包含周线、月线评分，默认true。", true),
			optionalEnumDefault("level", "输出深度：brief、normal、deep；当前保持同一评分口径。", "normal", "brief", "normal", "deep"),
		),
	}
	for i := range tools {
		if tools[i].Name == "tdx_sector_detail_text" ||
			tools[i].Name == "tdx_sector_realtime_text" {
			tools[i].InputSchema["anyOf"] = []map[string]any{
				{"required": []string{"sectorName"}},
				{"required": []string{"indexCode"}},
			}
		}
	}
	tools = append(tools, candidatePoolMCPTools()...)
	return append(tools, paperMCPTools()...)
}

func newMCPTool(name, description, path string, handler http.HandlerFunc, params ...mcpToolParam) mcpTool {
	required := make([]string, 0)
	properties := map[string]any{}
	for _, param := range params {
		property := map[string]any{
			"type":        param.Type,
			"description": param.Description,
		}
		if len(param.Enum) > 0 {
			property["enum"] = param.Enum
		}
		if param.Default != nil {
			property["default"] = param.Default
		}
		for key, value := range param.Schema {
			property[key] = value
		}
		properties[param.Name] = property
		if param.Required {
			required = append(required, param.Name)
		}
	}
	tool := mcpTool{
		Name:         name,
		Description:  description,
		OutputSchema: textToolOutputSchema(),
		Path:         path,
		Handler:      handler,
		InputSchema: map[string]any{
			"type":       "object",
			"properties": properties,
			"required":   required,
		},
	}
	if !strings.HasSuffix(name, "_text") {
		tool.OutputSchema = jsonToolOutputSchema()
	}
	return tool
}

func requiredString(name, description string) mcpToolParam {
	return mcpToolParam{Name: name, Type: "string", Description: description, Required: true}
}

func optionalString(name, description string) mcpToolParam {
	return mcpToolParam{Name: name, Type: "string", Description: description}
}

func optionalEnum(name, description string, values ...string) mcpToolParam {
	return mcpToolParam{Name: name, Type: "string", Description: description, Enum: values}
}

func optionalEnumDefault(
	name string,
	description string,
	defaultValue string,
	values ...string,
) mcpToolParam {
	return mcpToolParam{
		Name:        name,
		Type:        "string",
		Description: description,
		Enum:        values,
		Default:     defaultValue,
	}
}

func optionalNumber(name, description string) mcpToolParam {
	return mcpToolParam{Name: name, Type: "number", Description: description}
}

func optionalNumberSchema(name, description string, schema map[string]any) mcpToolParam {
	return mcpToolParam{Name: name, Type: "number", Description: description, Schema: schema}
}

func optionalNumberDefault(name, description string, defaultValue any) mcpToolParam {
	return mcpToolParam{Name: name, Type: "number", Description: description, Default: defaultValue}
}

func positiveNumberSchema() map[string]any {
	return map[string]any{"exclusiveMinimum": 0}
}

func growthPctSchema() map[string]any {
	return map[string]any{"minimum": -100, "maximum": 1000}
}

func optionalIntegerDefault(
	name string,
	description string,
	defaultValue any,
	minimum int,
	maximum int,
) mcpToolParam {
	schema := map[string]any{"minimum": minimum}
	if maximum != 0 {
		schema["maximum"] = maximum
	}
	return mcpToolParam{
		Name:        name,
		Type:        "integer",
		Description: description,
		Default:     defaultValue,
		Schema:      schema,
	}
}

func optionalInteger(name, description string, schema map[string]any) mcpToolParam {
	return mcpToolParam{Name: name, Type: "integer", Description: description, Schema: schema}
}

func optionalDateString(name, description string) mcpToolParam {
	return mcpToolParam{
		Name:        name,
		Type:        "string",
		Description: description,
		Schema: map[string]any{
			"pattern": `^(\d{4}-\d{2}-\d{2}|\d{8})$`,
		},
	}
}

func optionalMarket(name string) mcpToolParam {
	return mcpToolParam{
		Name:        name,
		Type:        "string",
		Description: "A股市场覆盖参数：可传 sh、sz、bj；留空或不传时按股票代码自动识别。仅支持A股，不支持港股或美股扩展行情。",
		Enum:        []string{"", "sh", "sz", "bj"},
	}
}

func optionalBool(name, description string) mcpToolParam {
	return mcpToolParam{Name: name, Type: "boolean", Description: description}
}

func optionalBoolDefault(name, description string, defaultValue bool) mcpToolParam {
	return mcpToolParam{Name: name, Type: "boolean", Description: description, Default: defaultValue}
}

func textToolOutputSchema() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "工具调用结果。content[0].text 为给 Agent 阅读的纯文本或 Markdown；structuredContent.text 为同一文本，endpoint/data 保留原始结构化结果。",
		"properties": map[string]any{
			"text": map[string]any{
				"type":        "string",
				"description": "给 Agent 阅读的纯文本或 Markdown 摘要。",
			},
			"endpoint": map[string]any{
				"type":        "string",
				"description": "内部 HTTP 接口路径。",
			},
			"data": map[string]any{
				"type":        "object",
				"description": "原始结构化响应数据；具体字段随工具不同而变化。",
			},
		},
		"required": []string{"text", "endpoint", "data"},
	}
}

func jsonToolOutputSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"description":          "工具调用结果。content[0].text 为简短文本说明；structuredContent 为机器可读 JSON 数据。调用失败时返回 MCP error.message。",
		"additionalProperties": true,
	}
}

func klineToolOutputSchema() map[string]any {
	numberField := func(description string) map[string]any {
		return map[string]any{"type": "number", "description": description}
	}
	return map[string]any{
		"type":        "object",
		"description": "原始K线MCP结果。精确数据位于data.list；text是同一数据的JSON文本表示。",
		"properties": map[string]any{
			"endpoint": map[string]any{
				"type":        "string",
				"description": "内部HTTP接口路径，固定为/api/kline。",
			},
			"text": map[string]any{
				"type":        "string",
				"description": "原始K线JSON的文本表示；程序化读取优先使用data。",
			},
			"data": map[string]any{
				"type":        "object",
				"description": "K线结构化响应。",
				"properties": map[string]any{
					"code":  map[string]any{"type": "string", "description": "股票代码。"},
					"type":  map[string]any{"type": "string", "description": "实际K线周期。"},
					"count": map[string]any{"type": "integer", "description": "实际返回条数。"},
					"list": map[string]any{
						"type":        "array",
						"description": "K线记录。逐日收盘价取date和close字段。",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"date":   map[string]any{"type": "string", "description": "K线日期，YYYY-MM-DD。"},
								"open":   numberField("开盘价。"),
								"high":   numberField("最高价。"),
								"low":    numberField("最低价。"),
								"close":  numberField("收盘价；type=day时即逐日收盘价。"),
								"volume": map[string]any{"type": "integer", "description": "成交量。"},
								"amount": numberField("成交额。"),
							},
							"required": []string{"date", "open", "high", "low", "close", "volume"},
						},
					},
				},
				"required": []string{"code", "type", "count", "list"},
			},
		},
		"required": []string{"endpoint", "text", "data"},
	}
}

func callMCPTool(raw json.RawMessage) (map[string]any, error) {
	var params mcpToolCallParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, fmt.Errorf("工具参数解析失败: %w", err)
	}
	if result, ok, err := callCandidatePoolMCPTool(params.Name, params.Arguments); ok {
		return result, err
	}
	if result, ok, err := callPaperMCPTool(params.Name, params.Arguments); ok {
		return result, err
	}
	for _, tool := range mcpTools() {
		if tool.Name == params.Name {
			return callAgentHandlerAsMCP(tool, params.Arguments)
		}
	}
	return nil, fmt.Errorf("未知工具: %s", params.Name)
}

func callAgentHandlerAsMCP(tool mcpTool, args map[string]any) (map[string]any, error) {
	req := httptest.NewRequest(http.MethodGet, tool.Path+"?"+encodeMCPQuery(args), nil)
	rec := httptest.NewRecorder()
	tool.Handler(rec, req)

	var apiResp APIResponse
	if err := json.NewDecoder(bytes.NewReader(rec.Body.Bytes())).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("%s返回不可解析JSON: %w", tool.Name, err)
	}
	if rec.Code >= http.StatusBadRequest || apiResp.Code != 0 {
		return nil, fmt.Errorf("%s调用失败: %s", tool.Name, apiResp.Message)
	}

	text := formatMCPToolText(apiResp.Data)
	return map[string]any{
		"content": []map[string]string{{"type": "text", "text": text}},
		"structuredContent": map[string]any{
			"endpoint": tool.Path,
			"text":     text,
			"data":     apiResp.Data,
		},
	}, nil
}

func encodeMCPQuery(args map[string]any) string {
	values := url.Values{}
	for key, value := range args {
		if value == nil {
			continue
		}
		switch v := value.(type) {
		case []any:
			for _, item := range v {
				values.Add(key, fmt.Sprint(item))
			}
		default:
			values.Set(key, fmt.Sprint(v))
		}
	}
	return values.Encode()
}

func formatMCPToolText(data any) string {
	if m, ok := data.(map[string]any); ok {
		if content, ok := m["content"].(string); ok && content != "" {
			return content
		}
	}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Sprint(data)
	}
	return string(b)
}

func writeMCPResult(w http.ResponseWriter, id json.RawMessage, result any) {
	writeMCPResponse(w, mcpResponse{JSONRPC: "2.0", ID: id, Result: result})
}

func writeMCPError(w http.ResponseWriter, id json.RawMessage, code int, message string) {
	writeMCPResponse(w, mcpResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &mcpError{Code: code, Message: message},
	})
}

func writeMCPResponse(w http.ResponseWriter, resp mcpResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(resp)
}

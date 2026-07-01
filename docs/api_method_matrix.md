# TDX API 方法暴露矩阵

本文档用于梳理 `github.com/injoyai/tdx` fork 项目中的底层数据能力，判断哪些方法适合暴露为 HTTP API 供 Agent 和 WebUI 调用。

评估维度：

- 方法：Go 层可调用方法。
- 已暴露 API：当前 `cmd/server` 是否已有对应 REST API。
- 用途：该方法提供的数据能力。
- 分析价值：对股市/个股深度分析的价值，分为高、中、低。
- 建议：保留、建议暴露、内部使用、隐藏/管理接口等。
- 备注：成本、风险或更好的组合方式。

## 当前已暴露 API 概览

| API | 对应方法 | 说明 |
|---|---|---|
| `/api/quote` | `GetQuote` | 实时行情 |
| `/api/kline` | `GetKline*` | 个股分页 K 线 |
| `/api/kline/all` | `GetKline*All` | 个股全量 K 线 |
| `/api/kline/qfq` | `Gbbq.QFQKlineDay` | 前复权日 K |
| `/api/kline/hfq` | `Gbbq.HFQKlineDay` | 后复权日 K |
| `/api/minute` | `GetHistoryMinute` | 分时/历史分时 |
| `/api/trade` | `GetMinuteTrade` | 当日分笔成交 |
| `/api/call-auction` | `GetCallAuction` | 集合竞价 |
| `/api/gbbq` | `GetGbbq` | 原始股本变迁 |
| `/api/adjust-factors` | `Gbbq.GetFactors` | 复权因子 |
| `/api/gbbq/adjust` | `Gbbq.QFQ/HFQ` + K 线 | 自定义周期复权 K 线 |
| `/api/gbbq/all` | `GetGbbqAll` | 全市场股本变迁 |
| `/api/finance` | `GetFinanceInfo` | 财务摘要 |
| `/api/f10` | `GetCompanyCategory/GetCompanyContent` | F10 分类与正文 |
| `/api/stat` | `GetTdxStat` | 个股综合统计 |
| `/api/moneyflow` | `GetTdxStat2` | 成交额、52 周高低、板块归属 |
| `/api/blocks` | `GetBlockDataWithIndex` | 板块成分 |
| `/api/hy` | `GetTdxHy` | 行业归属 |
| `/api/codes` | `GetStockCodeAll` | 股票代码池 |
| `/api/codes/etf` | `GetETFCodeAll` | ETF 代码池 |
| `/api/codes/index` | `GetIndexCodeAll` | 指数代码池 |
| `/api/index/kline` | `GetIndex*` | 指数分页 K 线 |
| `/api/index/all` | `GetIndex*All` | 指数全量 K 线 |
| `/api/search` | `GetZHBFiles/GetStockCodeAll` | 股票搜索，旧搜索逻辑 |
| `/api/history-trade` | `GetHistoryMinuteTradeDay` | 单日历史分笔 |
| `/api/stocks/search` | 本地股票库 | SQLite 股票检索 |
| `/api/stocks/refresh` | `refreshStocks` | 更新本地股票库 |

## Client 核心方法

| 方法 | 已暴露 API | 用途 | 分析价值 | 建议 | 备注 |
|---|---:|---|---|---|---|
| `GetCount(exchange)` | 否 | 获取指定市场证券数量 | 低 | 内部使用 | 可作为分页拉取代码前的辅助信息。 |
| `GetCode(exchange, start)` | 否 | 分页获取证券代码 | 中 | 内部使用 | 建议由代码池接口封装，不直接暴露。 |
| `GetCodeAll(exchange)` | 间接 | 获取指定市场全部代码 | 中 | 内部使用 | 是 `GetStockCodeAll` 等方法的底层能力。 |
| `GetStockCodeAll()` | 是 `/api/codes` | 获取 A 股股票代码池 | 高 | 保留 | Agent 分析 universe 的基础数据。 |
| `GetETFCodeAll()` | 是 `/api/codes/etf` | 获取 ETF 代码池 | 高 | 保留 | ETF 分析需要。 |
| `GetIndexCodeAll()` | 是 `/api/codes/index` | 获取指数代码池 | 高 | 保留 | 指数和市场环境分析需要。 |
| `GetQuote(codes...)` | 是 `/api/quote` | 实时行情、开高低、成交量额、五档盘口 | 高 | 保留并增强 | 当前 API 只返回基础行情，建议补充买卖五档、内外盘、现量。 |
| `GetCallAuction(code)` | 是 `/api/call-auction` | 集合竞价数据 | 中高 | 保留 | 可用于开盘强弱、竞价异常分析。 |
| `GetMinute(code)` | 否 | 当日分时图 | 高 | 建议暴露 | 当前 `/api/minute` 默认调用 `GetHistoryMinute(today)`，可显式支持实时分时。 |
| `GetHistoryMinute(date, code)` | 是 `/api/minute?date=` | 历史分时图 | 高 | 保留 | 日内走势分析重要数据。 |
| `GetTrade(code, start, count)` | 否 | 分笔成交分页 | 中 | 内部使用 | 更适合被全量分笔接口封装。 |
| `GetMinuteTrade(code, start, count)` | 是 `/api/trade` | 当日分笔分页 | 高 | 保留 | 当前固定取 2000 条，建议支持 `start/count` 参数。 |
| `GetTradeAll(code)` | 否 | 当日全量分笔别名 | 高 | 建议暴露 | 可与 `GetMinuteTradeAll` 合并为 `/api/trade/all`。 |
| `GetMinuteTradeAll(code)` | 否 | 当日全量分笔成交 | 高 | 建议暴露 | 适合成交结构、主动买卖、日内回放。 |
| `GetHistoryTrade(date, code, start, count)` | 否 | 历史分笔分页 | 高 | 内部使用 | 更适合由单日全量接口封装。 |
| `GetHistoryMinuteTrade(date, code, start, count)` | 否 | 历史分钟分笔分页 | 中 | 内部使用 | 分页能力，不建议直接暴露给 Agent。 |
| `GetHistoryTradeFull(code, workday)` | 否 | 跨交易日历史分笔全量 | 高 | 后台/限流 | 数据量很大，不建议普通 API 直接开放。 |
| `GetHistoryTradeBefore(code, workday, before)` | 否 | 指定日期前历史分笔 | 高 | 后台/限流 | 适合离线数据构建。 |
| `GetHistoryTradeDay(date, code)` | 否 | 单日历史成交全量 | 高 | 建议暴露 | 可与 `/api/history-trade` 对齐或补充。 |
| `GetHistoryMinuteTradeDay(date, code)` | 是 `/api/history-trade` | 单日历史分笔全量 | 高 | 保留 | 当前已有 API。 |

## 个股 K 线方法

| 方法 | 已暴露 API | 用途 | 分析价值 | 建议 | 备注 |
|---|---:|---|---|---|---|
| `GetKline(type, code, start, count)` | 间接 | 通用个股 K 线分页 | 高 | 内部统一入口 | 已由 `/api/kline` 封装。 |
| `GetKlineAll(type, code)` | 间接 | 通用个股全量 K 线 | 高 | 内部统一入口 | 已由 `/api/kline/all` 封装。 |
| `GetKlineUntil(type, code, fn)` | 否 | 条件停止拉取 K 线 | 高 | 内部/后台 | 对增量同步有价值，不适合直接 HTTP 暴露。 |
| `GetKlineMinute(code, start, count)` | 是 | 1 分钟 K 线 | 高 | 保留 | `/api/kline?type=minute1`。 |
| `GetKlineMinuteAll(code)` | 是 | 全量 1 分钟 K 线 | 高 | 保留并限流 | 数据量较大。 |
| `GetKlineMinuteUntil(code, fn)` | 否 | 条件拉取 1 分钟 K | 中 | 内部使用 | 增量同步用。 |
| `GetKlineMinute241Until(code, fn)` | 否 | 241 分钟修正拉取 | 中 | 内部使用 | 特定时间修正逻辑。 |
| `GetKline5Minute(code, start, count)` | 是 | 5 分钟 K 线 | 高 | 保留 | 技术分析常用。 |
| `GetKline5MinuteAll(code)` | 是 | 全量 5 分钟 K 线 | 高 | 保留并限流 | 数据量较大。 |
| `GetKline5MinuteUntil(code, fn)` | 否 | 条件拉取 5 分钟 K | 中 | 内部使用 | 增量同步用。 |
| `GetKline15Minute(code, start, count)` | 是 | 15 分钟 K 线 | 高 | 保留 | 技术分析常用。 |
| `GetKline15MinuteAll(code)` | 是 | 全量 15 分钟 K 线 | 高 | 保留并限流 | 数据量较大。 |
| `GetKline15MinuteUntil(code, fn)` | 否 | 条件拉取 15 分钟 K | 中 | 内部使用 | 增量同步用。 |
| `GetKline30Minute(code, start, count)` | 是 | 30 分钟 K 线 | 高 | 保留 | 技术分析常用。 |
| `GetKline30MinuteAll(code)` | 是 | 全量 30 分钟 K 线 | 高 | 保留并限流 | 数据量较大。 |
| `GetKline30MinuteUntil(code, fn)` | 否 | 条件拉取 30 分钟 K | 中 | 内部使用 | 增量同步用。 |
| `GetKline60Minute(code, start, count)` | 是 | 60 分钟 K 线 | 高 | 保留 | 技术分析常用。 |
| `GetKlineHour(code, start, count)` | 否 | 60 分钟 K 线别名 | 低 | 不单独暴露 | 避免 API 重复。 |
| `GetKline60MinuteAll(code)` | 是 | 全量 60 分钟 K 线 | 高 | 保留并限流 | 数据量较大。 |
| `GetKlineHourAll(code)` | 否 | 全量 60 分钟 K 线别名 | 低 | 不单独暴露 | 避免 API 重复。 |
| `GetKline60MinuteUntil(code, fn)` | 否 | 条件拉取 60 分钟 K | 中 | 内部使用 | 增量同步用。 |
| `GetKlineHourUntil(code, fn)` | 否 | 条件拉取 60 分钟 K 别名 | 低 | 不单独暴露 | 避免 API 重复。 |
| `GetKlineDay(code, start, count)` | 是 | 日 K 线 | 高 | 保留 | 个股分析核心数据。 |
| `GetKlineDayAll(code)` | 是 | 全量日 K 线 | 高 | 保留 | 回测、长期趋势分析核心数据。 |
| `GetKlineDayUntil(code, fn)` | 否 | 条件拉取日 K | 高 | 内部/后台 | 增量同步和数据缓存有价值。 |
| `GetKlineWeek(code, start, count)` | 是 | 周 K 线 | 高 | 保留 | 中期趋势分析。 |
| `GetKlineWeekAll(code)` | 是 | 全量周 K 线 | 高 | 保留 | 中长期分析。 |
| `GetKlineWeekUntil(code, fn)` | 否 | 条件拉取周 K | 中 | 内部使用 | 增量同步用。 |
| `GetKlineMonth(code, start, count)` | 是 | 月 K 线 | 高 | 保留 | 长周期分析。 |
| `GetKlineMonthAll(code)` | 是 | 全量月 K 线 | 高 | 保留 | 长周期分析。 |
| `GetKlineMonthUntil(code, fn)` | 否 | 条件拉取月 K | 中 | 内部使用 | 增量同步用。 |
| `GetKlineQuarter(code, start, count)` | 是 | 季 K 线 | 中高 | 保留 | 长周期分析。 |
| `GetKlineQuarterAll(code)` | 是 | 全量季 K 线 | 中高 | 保留 | 长周期分析。 |
| `GetKlineQuarterUntil(code, fn)` | 否 | 条件拉取季 K | 中 | 内部使用 | 增量同步用。 |
| `GetKlineYear(code, start, count)` | 是 | 年 K 线 | 中 | 保留 | 长周期概览。 |
| `GetKlineYearAll(code)` | 是 | 全量年 K 线 | 中 | 保留 | 长周期概览。 |
| `GetKlineYearUntil(code, fn)` | 否 | 条件拉取年 K | 中 | 内部使用 | 增量同步用。 |

## 指数 K 线方法

| 方法 | 已暴露 API | 用途 | 分析价值 | 建议 | 备注 |
|---|---:|---|---|---|---|
| `GetIndex(type, code, start, count)` | 是 `/api/index/kline` | 通用指数 K 线分页 | 高 | 保留 | 大盘和行业指数分析核心数据。 |
| `GetIndexAll(type, code)` | 是 `/api/index/all` | 通用指数全量 K 线 | 高 | 保留 | 长期市场周期分析。 |
| `GetIndexUntil(type, code, fn)` | 否 | 条件拉取指数 K 线 | 中 | 内部/后台 | 增量同步用。 |
| `GetIndexMinute(code, start, count)` | 是 | 指数 1 分钟 K | 高 | 保留 | 盘中大盘分析。 |
| `GetIndex5Minute(code, start, count)` | 是 | 指数 5 分钟 K | 高 | 保留 | 盘中大盘分析。 |
| `GetIndex15Minute(code, start, count)` | 是 | 指数 15 分钟 K | 高 | 保留 | 趋势分析。 |
| `GetIndex30Minute(code, start, count)` | 是 | 指数 30 分钟 K | 高 | 保留 | 趋势分析。 |
| `GetIndex60Minute(code, start, count)` | 是 | 指数 60 分钟 K | 高 | 保留 | 趋势分析。 |
| `GetIndexDay(code, start, count)` | 是 | 指数日 K | 高 | 保留 | 市场环境分析核心数据。 |
| `GetIndexDayUntil(code, fn)` | 否 | 条件拉取指数日 K | 中 | 内部/后台 | 增量同步用。 |
| `GetIndexDayAll(code)` | 是 | 指数全量日 K | 高 | 保留 | 市场周期分析。 |
| `GetIndexWeekAll(code)` | 是 | 指数全量周 K | 高 | 保留 | 中期市场分析。 |
| `GetIndexMonthAll(code)` | 是 | 指数全量月 K | 高 | 保留 | 长期市场分析。 |
| `GetIndexQuarterAll(code)` | 是 | 指数全量季 K | 中 | 保留 | 长期市场分析。 |
| `GetIndexYearAll(code)` | 是 | 指数全量年 K | 中 | 保留 | 长期市场概览。 |

## 基本面、板块与配置方法

| 方法 | 已暴露 API | 用途 | 分析价值 | 建议 | 备注 |
|---|---:|---|---|---|---|
| `GetGbbq(code)` | 是 `/api/gbbq` | 原始股本变迁/除权除息事件 | 中 | 高级/调试接口 | Agent 更适合使用复权 K 线或复权因子。 |
| `GetGbbqAll()` | 是 `/api/gbbq/all` | 全市场股本变迁 | 高但重 | 后台/管理接口 | 不建议作为公开普通 API。 |
| `GetCompanyCategory(exchange, code)` | 是 `/api/f10` | F10 分类列表 | 中高 | 保留 | 与正文接口配合使用。 |
| `GetCompanyContent(exchange, code, filename, start, length)` | 是 `/api/f10?cat=` | F10 正文 | 中高 | 保留 | 可用于文本分析、公告和股东资料抽取。 |
| `GetFinanceInfo(exchange, code)` | 是 `/api/finance` | 财务摘要 | 高 | 保留 | 基本面分析核心接口。 |
| `GetBlockFileRaw(file)` | 否 | 原始板块文件字节 | 低 | 不暴露 | 内部解析用。 |
| `GetBlockData(file)` | 否 | 板块成分，不含指数 id | 中 | 内部使用 | 建议统一使用 `GetBlockDataWithIndex`。 |
| `GetReportFile(file)` | 否 | 原始报表文件下载 | 低 | 不暴露 | 维护/调试用途。 |
| `GetZHBFiles()` | 间接 | 下载并解压 `zhb.zip` | 中 | 内部使用 | 多个统计、板块、搜索接口依赖它。 |
| `GetTdxZs()` | 否 | 板块名称与指数代码映射 | 高 | 建议暴露或合并 | 可增强板块分析，建议并入 `/api/blocks` 或新增 `/api/block-indexes`。 |
| `GetTdxBk()` | 否 | 板块简称与全称映射 | 中 | 内部使用 | 对展示有帮助，但不适合单独暴露。 |
| `GetBlockDataWithIndex(file)` | 是 `/api/blocks` | 板块成分 + 指数代码 | 高 | 保留 | 主题/行业/地域分析核心数据。 |
| `GetTdxStat()` | 是 `/api/stat` | PE、股息率、区间涨跌幅 | 高 | 保留 | 全市场横截面分析核心数据。 |
| `GetTdxStat2()` | 是 `/api/moneyflow` | 成交额、52 周高低、板块归属 | 高 | 保留 | 资金、位置、板块归属分析。 |
| `GetXgsg()` | 否 | 新股申购数据 | 中高 | 建议暴露 | 新股事件分析、市场供给分析可用。 |
| `GetTdxHy()` | 是 `/api/hy` | 通达信/申万行业归属 | 高 | 保留 | 行业聚合和横截面分析必需。 |

## 扩展行情 ExHq 方法

当前扩展行情方法尚未暴露为 HTTP API。是否暴露取决于项目是否计划覆盖港股、期货、外盘等非 A 股数据。

| 方法 | 已暴露 API | 用途 | 分析价值 | 建议 | 备注 |
|---|---:|---|---|---|---|
| `ExMarkets()` | 否 | 扩展行情市场列表 | 中 | 条件暴露 | 如果支持港股/期货，建议作为元数据接口。 |
| `ExCount()` | 否 | 扩展行情品种数量 | 低 | 内部使用 | 单独分析价值有限。 |
| `ExInstruments(start, count)` | 否 | 扩展行情品种列表 | 高 | 建议暴露 | 非 A 股 universe 基础。 |
| `ExQuote(market, code)` | 否 | 扩展市场实时行情 | 高 | 建议暴露 | 港股/期货/外盘实时分析核心接口。 |
| `ExQuoteList(market, category, start, count)` | 否 | 扩展行情列表 | 中高 | 建议暴露 | 市场扫描用途。 |
| `ExBars(category, market, code, start, count)` | 否 | 扩展 K 线 | 高 | 建议暴露 | 非 A 股技术分析核心接口。 |
| `ExMinute(market, code)` | 否 | 扩展分时 | 中高 | 建议暴露 | 盘中分析。 |
| `ExHistMinute(market, code, date)` | 否 | 扩展历史分时 | 中高 | 建议暴露 | 日内历史分析。 |
| `ExTrade(market, code, start, count)` | 否 | 扩展分笔 | 中高 | 建议暴露 | 成交结构分析。 |
| `ExHistTrade(market, code, date, start, count)` | 否 | 扩展历史分笔 | 中高 | 建议暴露 | 历史成交结构分析。 |
| `ExBarsRange(market, code, date, date2)` | 否 | 扩展区间 K 线 | 高 | 建议暴露 | 区间拉取比分页更适合 Agent。 |

## Gbbq 服务方法

| 方法 | 已暴露 API | 用途 | 分析价值 | 建议 | 备注 |
|---|---:|---|---|---|---|
| `Gbbq.All()` | 间接 | 缓存内全量股本变迁 | 中 | 后台/内部 | 可用于管理或缓存检查。 |
| `Gbbq.GetEquity(code, time)` | 否 | 指定日期股本 | 高 | 建议暴露 | 可支持换手率、市值、股本变化分析。 |
| `Gbbq.GetXRXDs(code)` | 否 | 除权除息事件列表 | 中 | 高级接口 | 对解释复权结果有价值。 |
| `Gbbq.GetXRXDMap(code)` | 否 | 除权除息事件 Map | 低 | 内部使用 | 不适合直接暴露。 |
| `Gbbq.GetFactors(code, klines)` | 是 `/api/adjust-factors` | 复权因子 | 高 | 保留为高级接口 | Agent 可用，但普通用户更适合复权 K 线。 |
| `Gbbq.QFQ(code, klines)` | 间接 | 对已有 K 线前复权 | 高 | 内部组合 | 已由复权 K 线接口使用。 |
| `Gbbq.HFQ(code, klines)` | 间接 | 对已有 K 线后复权 | 高 | 内部组合 | 已由复权 K 线接口使用。 |
| `Gbbq.QFQKlineDay(code)` | 是 `/api/kline/qfq` | 前复权日 K | 高 | 保留 | 推荐作为长期个股价格分析默认接口。 |
| `Gbbq.HFQKlineDay(code)` | 是 `/api/kline/hfq` | 后复权日 K | 高 | 保留 | 总收益和历史复盘可用。 |
| `Gbbq.GetTurnover(code, time, volume)` | 否 | 换手率计算 | 高 | 建议暴露 | 可与行情/K 线量能组合为分析指标。 |
| `Gbbq.Update()` | 否 | 更新复权缓存 | 低 | 管理接口 | 不建议公开。 |

## 代码池方法

| 方法 | 已暴露 API | 用途 | 分析价值 | 建议 | 备注 |
|---|---:|---|---|---|---|
| `Codes.Update()` | 否 | 更新代码数据库 | 低 | 管理接口 | 不建议公开。 |
| `CodesBase.Iter()` | 否 | 遍历代码库 | 中 | 内部使用 | 可作为导出接口底层能力。 |
| `CodesBase.Get(code)` | 否 | 查询单个代码信息 | 高 | 建议暴露 | 建议并入 `/api/stock/profile`。 |
| `CodesBase.GetName(code)` | 否 | 查询代码名称 | 高 | 建议暴露 | 建议并入搜索或 profile。 |
| `CodesBase.GetStocks(limit)` | 否 | 股票对象列表 | 高 | 建议暴露 | 比单纯代码列表更适合 Agent。 |
| `CodesBase.GetStockCodes(limit)` | 间接 | 股票代码列表 | 高 | 已有 API | 当前 `/api/codes` 已覆盖代码，不含名称等字段。 |
| `CodesBase.GetETFs(limit)` | 否 | ETF 对象列表 | 高 | 建议暴露 | 建议补充 ETF 名称和交易所。 |
| `CodesBase.GetETFCodes(limit)` | 间接 | ETF 代码列表 | 高 | 已有 API | 当前 `/api/codes/etf` 已覆盖代码。 |
| `CodesBase.GetIndexes(limit)` | 否 | 指数对象列表 | 高 | 建议暴露 | 建议补充指数名称和交易所。 |
| `CodesBase.GetIndexCodes(limit)` | 间接 | 指数代码列表 | 高 | 已有 API | 当前 `/api/codes/index` 已覆盖代码。 |

## 交易日方法

| 方法 | 已暴露 API | 用途 | 分析价值 | 建议 | 备注 |
|---|---:|---|---|---|---|
| `Workday.Update()` | 否 | 更新交易日缓存 | 低 | 管理接口 | 不建议公开。 |
| `Workday.Is(time)` | 否 | 判断指定日期是否交易日 | 中高 | 建议暴露 | 对 Agent 制定查询日期、回测窗口有用。 |
| `Workday.TodayIs()` | 否 | 判断今天是否交易日 | 中高 | 建议暴露 | 盘中任务和自动分析有用。 |
| `Workday.RangeYear(year, fn)` | 否 | 遍历某年交易日 | 中 | 内部使用 | 可封装成交易日列表接口。 |
| `Workday.Range(start, end, fn)` | 否 | 遍历区间交易日 | 中 | 内部使用 | 可封装成交易日列表接口。 |
| `Workday.IterYear(year)` | 否 | 某年交易日迭代器 | 中 | 内部使用 | Go 内部能力。 |
| `Workday.Iter(start, end)` | 否 | 区间交易日迭代器 | 中 | 内部使用 | Go 内部能力。 |

## 建议优先新增 API

| 建议 API | 底层方法 | 说明 | 优先级 |
|---|---|---|---|
| `/api/trade/all` | `GetMinuteTradeAll` | 当日全量分笔 | 高 |
| `/api/history-trade/day` | `GetHistoryTradeDay` 或 `GetHistoryMinuteTradeDay` | 单日历史成交 | 高 |
| `/api/minute/live` | `GetMinute` | 当日实时分时 | 高 |
| `/api/xgsg` | `GetXgsg` | 新股申购 | 中高 |
| `/api/block-indexes` | `GetTdxZs` | 板块指数映射 | 中高 |
| `/api/stock/profile` | `CodesBase.Get`、`GetFinanceInfo`、`GetTdxHy`、`GetTdxStat2` | 单股画像聚合接口 | 高 |
| `/api/stock/turnover` | `Gbbq.GetTurnover`、行情/K 线成交量 | 换手率 | 高 |
| `/api/workday/is` | `Workday.Is` | 判断交易日 | 中高 |
| `/api/workday/today` | `Workday.TodayIs` | 今日是否交易日 | 中高 |
| `/api/ex/markets` | `ExMarkets` | 扩展行情市场列表 | 中 |
| `/api/ex/instruments` | `ExInstruments` | 扩展行情品种列表 | 中高 |
| `/api/ex/quote` | `ExQuote` | 扩展行情报价 | 高 |
| `/api/ex/kline` | `ExBars`、`ExBarsRange` | 扩展行情 K 线 | 高 |
| `/api/ex/minute` | `ExMinute`、`ExHistMinute` | 扩展分时 | 中高 |
| `/api/ex/trade` | `ExTrade`、`ExHistTrade` | 扩展分笔 | 中高 |

## 建议隐藏或改为管理接口

| 当前 API/方法 | 建议 | 原因 |
|---|---|---|
| `/api/gbbq/all` / `GetGbbqAll` | 改为管理/后台接口 | 全市场拉取成本高，不适合普通调用。 |
| `/api/stocks/refresh` / `refreshStocks` | 改为管理接口 | 维护动作，不是分析数据接口。 |
| `/api/search` | 合并到 `/api/stocks/search` | 搜索逻辑重复，且全局缓存需要并发保护。 |
| `GetBlockFileRaw` | 不暴露 | 原始文件字节，对 Agent 分析价值低。 |
| `GetReportFile` | 不暴露 | 原始报表下载，适合内部解析。 |
| `GetZHBFiles` | 不直接暴露 | 原始配置包，建议通过结构化接口输出。 |
| `Codes.Update` | 管理接口 | 维护动作。 |
| `Gbbq.Update` | 管理接口 | 维护动作。 |
| `Workday.Update` | 管理接口 | 维护动作。 |

## 源码优化建议

1. 修复 `go test ./...` 失败。
   - 当前失败点：`client.go:347`。
   - 原因：`logs.Err("[%s] 代码列表获取失败: %v (跳过)", ex, err)` 被 `go vet` 判断为格式化风险。
   - 建议：改为 `logs.Err(fmt.Sprintf(...))` 或使用日志库支持的格式化方法。

2. 统一 API 元数据。
   - 当前服务端路由、WebUI `API_CATALOG`、页面 endpoint 数量分散维护。
   - 建议建立统一 API 描述表，由后端和前端共享或生成。

3. 合并搜索能力。
   - `/api/search` 与 `/api/stocks/search` 功能重复。
   - 建议保留一个主搜索接口，并修复 `stockNameMap` 的并发风险。

4. 给重接口加保护。
   - 全量 K 线、全市场复权、股票库刷新等接口应增加限流、缓存或管理开关。

5. 建立分析型聚合接口。
   - Agent 调用更适合一次获取结构化画像，而不是连续调用多个底层接口。
   - 推荐优先实现 `/api/stock/profile` 与 `/api/stock/analysis-data`。

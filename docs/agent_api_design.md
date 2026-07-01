# Hermes Agent API 开发框架

本文档是 `tdx-api` 面向 Hermes 金融证券分析 Agent 的长期开发基准。目标不是把
`github.com/injoyai/tdx` 的全部方法直接透传给 Agent，而是把底层数据整理成低噪音、低上下文、
可组合、可维护的分析接口。

当前核心市场为中国 A 股。港股、美股、日韩、商品、汇率、全球指数等扩展市场只作为 A 股分析的
补充数据，用于判断主要资产变化对 A 股的宏观扩散效应。新闻、政策、国际时事、行业事件等文字资讯
由 `anysearch` 等搜索工具补齐，`tdx-api` 只负责结构化市场数据。

## 设计原则

1. Agent 优先调用 `/api/agent/*` 聚合接口，原子接口主要用于 WebUI、调试和补查。
2. 聚合接口默认返回摘要和关键字段，不默认返回大数组、全市场明细或 F10 原文。
3. 同一能力可以同时存在“子接口”和“聚合字段”，例如 `technical-summary` 可独立调用，也已并入
   `stock-brief`。
4. 底层 TDX 方法尽量保持上游兼容；清洗、中文化、限量、派生指标计算放在 Agent 聚合层。
5. SQLite 负责维护 A 股字典和板块归属，TDX 负责行情与基础数据。
6. WebUI 是辅助工具，不决定 Agent API 的边界。

## SQLite 数据库约定

Agent 相关 SQLite 默认统一使用一个文件：

- 默认路径：`data/database/tdx-agent.sqlite`
- 全局覆盖环境变量：`AGENT_DB_PATH`
- 兼容覆盖环境变量：`STOCKS_DB_PATH`、`BLOCKS_DB_PATH`

后续新增接口只要需要 SQLite，默认都应写入该文件，并用清晰表名区分职责：

| 表名 | 职责 |
|---|---|
| `stocks` | A 股代码、名称、交易所、拼音检索字典 |
| `block_memberships` | A 股代码到概念/地域风格/指数板块的归属 |
| `block_index_meta` | 板块索引库刷新时间等元数据 |

命名约定：业务表使用复数名词；关联表使用 `*_relations` 或 `*_memberships`；字段使用
snake_case，主键和索引含义必须能从字段名直接看出。

## API 分层

| 层级 | 作用 | 调用方 |
|---|---|---|
| Agent 聚合 API | 面向分析场景，返回精简结构化数据或中文文本 | Hermes Agent 主流程 |
| Agent 子模块 API | 可独立调用，也可被聚合 API 复用 | Hermes Agent 二次追问 |
| 原子数据 API | 暴露单项底层能力 | WebUI、调试、补查 |
| 管理/后台 API | 刷新 SQLite、复权、资产池、缓存 | 管理任务，不建议 Agent 常规调用 |

## MCP 部署入口

局域网部署时，MCP 与现有 HTTP API 共用同一个 Go 服务：

- HTTP API：`http://<服务器IP>:8080/api/agent/...`
- MCP endpoint：`http://<服务器IP>:8080/mcp`

MCP 当前只暴露面向 Agent 的文本聚合工具，工具名使用 `tdx_*_text` 风格，例如
`tdx_stock_brief_text`、`tdx_kline_summary_text`、`tdx_market_review_text`。
JSON 调试和人工核查仍使用原 `/api/agent/*` HTTP 接口，避免 MCP 输出塞入过多原始数据。

## 当前进度

| API | 定位 | 状态 | 说明 |
|---|---|---|---|
| `/api/agent/stock-brief` | 个股简讯 JSON | 已完成 | 聚合行情、财务、F10 cat0、板块、估值、资金统计、技术指标 |
| `/api/agent/stock-brief-text` | 个股简讯文本 | 已完成 | 面向 Agent 的中文低噪音输出 |
| `/api/agent/technical-summary` | 技术指标摘要 | 已完成 | 日/周/月 MA、MACD、RSI、BOLL、ATR；也已并入 `stock-brief` |
| SQLite 股票名称库 | A 股代码-名称字典 | 已完成 | 容器启动时刷新，避免只返回股票代码 |
| SQLite 板块索引库 | 个股所属板块查询 | 已完成 | 容器启动时刷新，更新而非追加 |
| `/api/agent/kline-summary` | K 线聚合 JSON | 已完成 | 日线按 level/dayCount 限量返回原始 K 线聚合数据；周线、月线全量返回 |
| `/api/agent/kline-summary-text` | K 线形态与阶段走势文本 | 已完成 | 面向 Agent 的中文低噪音输出，包含趋势阶段和风险摘要 |
| `/api/agent/trade-flow-estimate` | 单日逐笔资金流估算 JSON | 已完成 | 支持 `date=YYYY-MM-DD`，优先按 200 日逐笔成交额自适应阈值估算分档资金流 |
| `/api/agent/trade-flow-estimate-text` | 单日逐笔资金流估算文本 | 已完成 | 面向 Agent 的中文低噪音输出，明确非外部 APP 官方口径 |
| `/api/agent/f10-summary` | F10 深度资料 JSON | 已完成 | 覆盖股本、股东、机构、分红、资金、资本、题材、公告、经营、行业、研报等低频资料 |
| `/api/agent/f10-summary-text` | F10 深度资料文本 | 已完成 | 面向 Agent 的深度基本面补充输出 |
| `/api/agent/assets/search` | A 股资产搜索 JSON | 已完成 | 模糊解析代码、名称、拼音输入；候选项返回与 `assets/detail` 同颗粒度详情 |
| `/api/agent/assets/search-text` | A 股资产搜索文本 | 已完成 | 面向 Agent 的中文资产解析结果；查无结果时明确提示“查无此股票” |
| `/api/agent/assets/detail` | A 股资产详情 JSON | 已完成 | 明确代码查询单只资产详情，返回标准代码、名称、市场、类型和板块归属 |
| `/api/agent/sector-membership` | 个股板块归属 JSON | 已完成 | 返回个股完整板块归属，并按概念/地域风格/指数分组 |
| `/api/agent/sector-membership-text` | 个股板块归属文本 | 已完成 | 面向 Agent 的中文低噪音板块归属摘要 |
| `/api/agent/stock-in-sector` | 个股板块内位置 JSON | 已完成 | 在指定或默认所属板块内按涨跌/估值等指标排序 |
| `/api/agent/stock-in-sector-text` | 个股板块内位置文本 | 已完成 | 面向 Agent 的中文低噪音相对强弱摘要 |
| `/api/agent/sector-detail` | 指定板块深度分析 JSON | 已完成 | 根据板块名称或指数代码返回板块阶段表现、成分股强弱、中游股和弱势股 |
| `/api/agent/sector-detail-text` | 指定板块深度分析文本 | 已完成 | 面向 Agent 的中文低噪音板块拆解摘要 |
| `/api/agent/hotspot-scan` | 热点扫描 JSON | 已完成 | 按概念/地域/指数板块扫描近20日或指定历史窗口涨跌，默认返回最强20、中游20、最弱20并排除新股异常涨幅 |
| `/api/agent/hotspot-scan-text` | 热点扫描文本 | 已完成 | 面向 Agent 的中文低噪音强弱与中游板块摘要 |
| `/api/agent/multi-brief` | 多股简讯 JSON | 已完成 | 请求参数传入股票列表，批量复用 `stock-brief`；不作为分时监控接口 |
| `/api/agent/multi-brief-text` | 多股简讯文本 | 已完成 | 面向 Agent 的中文多股 brief 列表摘要 |
| `/api/agent/auction` | 集合竞价分析 JSON | 已完成 | 默认分析 09:20-09:25 开盘不可撤单竞价，返回末笔、相对昨收、未匹配方向和近 5/20 个交易日日K走势背景 |
| `/api/agent/auction-text` | 集合竞价分析文本 | 已完成 | 面向 Agent 的中文低噪音竞价摘要 |
| `/api/agent/market-review` | 市场级复盘 JSON | 已完成 | 按查询时间自动识别开盘前、盘中、上午收盘、午后盘中或全天收盘视角，聚合上证、深成、创业板、科创50、北证50、市场广度、热点板块和可选关注股 |
| `/api/agent/market-review-text` | 市场级复盘文本 | 已完成 | 面向 Agent 的中文低噪音市场环境摘要 |
| `/api/agent/intraday-alerts` | 盘中异动提醒 JSON | 已完成 | 按调用时点检查多只关注股的当日强弱、近段涨跌和近段放量；分时不可用时降级为实时行情快照 |
| `/api/agent/intraday-alerts-text` | 盘中异动提醒文本 | 已完成 | 面向 Agent 的中文低噪音异动摘要，不做后台轮询和推送 |
| `/api/agent/global-market-brief` | 全球外围权重资产 JSON | 已完成 | 内置精选白名单，不使用 SQLite；覆盖风险偏好、亚太核心市场、商品、汇率、利率债券和全球权重股；补入欧洲STOXX50ETF、恒生科技指数、韩国/台湾可用指数代理、印度、日本宽基和东证代理；实时读取 TDX 扩展行情并计算 20/60 日涨跌幅、区间高低点和区间位置 |
| `/api/agent/global-market-brief-text` | 全球外围权重资产文本 | 已完成 | 面向 Agent 的中文低噪音外围市场环境摘要 |

## 已实现聚合接口参数

| API | 参数 | 说明 |
|---|---|---|
| `/api/agent/stock-brief` | `code` 必填；`mkt` 可选 | `code` 为股票代码；`mkt` 用于覆盖市场，通常可省略 |
| `/api/agent/stock-brief-text` | `code` 必填；`mkt` 可选 | 与 JSON 版一致，返回中文低噪音文本 |
| `/api/agent/technical-summary` | `code` 必填 | 返回日/周/月技术指标摘要 |
| `/api/agent/kline-summary` | `code` 必填；`level` 可选；`dayCount` 可选 | `level=brief|normal|deep`；`dayCount` 覆盖日线数量，最大 500 |
| `/api/agent/kline-summary-text` | `code` 必填；`level` 可选；`dayCount` 可选 | 与 JSON 版一致，但只返回中文清洗摘要 |
| `/api/agent/trade-flow-estimate` | `code` 必填；`date` 可选 | `date=YYYY-MM-DD` 或 `YYYYMMDD`；不传默认今天 |
| `/api/agent/trade-flow-estimate-text` | `code` 必填；`date` 可选 | 与 JSON 版一致，返回中文资金流估算摘要 |
| `/api/agent/f10-summary` | `code` 必填；`mkt` 可选 | 返回低频深度 F10 分类裁剪摘要 |
| `/api/agent/f10-summary-text` | `code` 必填；`mkt` 可选 | 与 JSON 版一致，返回中文低噪音文本 |
| `/api/agent/assets/search` | `keyword` 必填，也支持 `q`；`limit` 可选 | 搜索 A 股资产，默认最多 20 条，最大 50 条；每个候选项包含 `assets/detail` 同颗粒度字段；查无结果返回 `count=0` 和空 `items`，不作为接口错误 |
| `/api/agent/assets/search-text` | 同 JSON 版 | 返回中文资产搜索结果；唯一命中时可直接读取代码和主要板块，查无结果时返回“查无此股票” |
| `/api/agent/assets/detail` | `code` 必填 | 明确代码查询单只 A 股资产标准名称、市场属性和板块归属 |
| `/api/agent/sector-membership` | `code` 必填 | 查询个股完整板块归属，返回原始列表和分组结果 |
| `/api/agent/sector-membership-text` | `code` 必填 | 与 JSON 版一致，返回中文板块归属摘要 |
| `/api/agent/stock-in-sector` | `code` 必填；`sectorType` 可选；`sectorName` 可选；`metric` 可选；`limit` 可选 | 默认选第一个概念板块；`metric=changePct|chg5|chg20|chg60|peTtm|divYield`；`limit` 默认 10 最大 50 |
| `/api/agent/stock-in-sector-text` | 同 JSON 版 | 返回中文相对强弱摘要 |
| `/api/agent/sector-detail` | `sectorName` 或 `indexCode` 必填；`sectorType` 可选；`metric` 可选；`topStocks` 可选；`excludeNew` 可选 | `sectorType=concept|style_region|index`，默认 `concept`；`metric=changePct|chg5|chg20|chg60|peTtm|divYield`，默认 `chg20`；`topStocks` 默认 10 最大 30；`excludeNew` 默认 true，过滤新股/异常涨幅样本；返回板块指数近20/60日收益、成分股上涨比例、强势股、中游股和弱势股 |
| `/api/agent/sector-detail-text` | 同 JSON 版 | 返回中文板块深度摘要，适合在 `hotspot-scan` 找到板块后继续拆解 |
| `/api/agent/hotspot-scan` | `sectorType` 可选；`metric` 可选；`startDate` 可选；`endDate` 可选；`window` 可选；`offset` 可选；`limit` 可选；`topStocks` 可选；`minMembers` 可选；`excludeNew` 可选 | `sectorType=concept|style_region|index`，默认 `concept`；`metric=chg20` 默认，适合中长线热点；`metric=chg5` 短期热点；`metric=chg60` 中期主线；`metric=changePct` 当日异动；`metric=windowReturn` 使用板块指数日 K 线计算历史窗口收益，推荐传 `startDate=YYYY-MM-DD&endDate=YYYY-MM-DD`；`window/offset` 保留兼容；另支持 `peTtm|divYield`；`limit` 默认 20 最大 50，控制最强/最弱数量，中游最多返回 `limit` 个且不与最强/最弱重叠；每个板块含 `topStocks` 和 `bottomStocks`；`topStocks` 默认 3 最大 10；`minMembers` 默认 20；`excludeNew` 默认 true；`windowReturn` 通过 `GetIndexDayAll` 获取板块指数 K 线，避免普通 K 线解码导致日期错位 |
| `/api/agent/hotspot-scan-text` | 同 JSON 版 | 返回中文强弱与中游板块扫描摘要 |
| `/api/agent/multi-brief` | `codes` 或 `code` 必填 | `codes=603063,000001` 或多次传 `code=603063&code=000001`；最多 20 只；JSON 返回每只股票的 `stock-brief` 聚合结果 |
| `/api/agent/multi-brief-text` | 同 JSON 版 | 返回中文多股简讯摘要，每只股票一行，包含价格、涨跌幅、成交额、换手率、20日表现和主要板块 |
| `/api/agent/auction` | `code` 必填；`session` 可选；`limit` 可选 | `session=open|close|all`，默认 `open`；`open` 过滤 09:20-09:25 开盘不可撤单竞价，`close` 过滤 14:57-15:00，`all` 返回全天竞价记录；`limit` 默认 20 最大 100 |
| `/api/agent/auction-text` | 同 JSON 版 | 返回中文竞价摘要，包含末笔价格、较昨收涨跌、匹配量、未匹配方向、近 5/20 个交易日日K累计涨跌背景和信号 |
| `/api/agent/market-review` | `session` 可选；`codes` 可选；`top` 可选 | `session=auto|current|morning|full`，默认 `auto`；`auto` 按查询时间判断视角；`codes` 传关注股列表；`top` 默认 10 最大 20，控制强/中/弱板块数量 |
| `/api/agent/market-review-text` | 同 JSON 版 | 返回中文市场环境摘要，包含上证、深成、创业板、科创50、北证50、市场广度、强/中/弱板块和可选关注股联动 |
| `/api/agent/intraday-alerts` | `codes` 或 `code` 必填；`windowMinutes` 可选 | `codes=603063,000001` 或多次传 `code=603063&code=000001`；最多 20 只；`windowMinutes` 默认 30，允许 5-60；JSON 返回交易日判断、每只股票的实时行情、分时近段涨跌、近段成交量变化和异动信号 |
| `/api/agent/intraday-alerts-text` | 同 JSON 版 | 返回中文盘中异动摘要；若非交易日导致分时为空，会明确提示“非交易日无可用分时数据” |
| `/api/agent/global-market-brief` | 无参数 | 返回外围权重资产池；分组包括 `risk`、`apac`、`commodity`、`fx`、`bond`、`leader`；每项包含 `price`、`changePct`、`range20`、`range60`，其中区间字段包含涨跌幅、区间最高、区间最低和当前区间位置；TDX 未验证到 Sensex30、俄罗斯RTS 本体稳定直连代码，暂不硬塞无效项 |
| `/api/agent/global-market-brief-text` | 无参数 | 返回中文摘要；保留每项现价、当日涨跌、20/60 日涨跌幅、区间高低点和观察意义 |

## 已确认的 stock-brief 边界

`stock-brief` 是“个股简讯”，不再继续扩张为深度分析大包。

保留内容：

- 股票名称和代码。
- 实时行情摘要：价格、涨跌幅、振幅、成交额、成交量、换手率、成交额较昨日变化。
- 基本面摘要：总股本、流通股本、总市值、流通市值、总资产、净资产、收入、利润、经营现金流、股东人数。
- F10 cat0 最新财报提示：报告期、每股净资产、每股经营现金流、加权 ROE、营收同比、净利润同比。
- 所属板块摘要：概念、地域/风格、指数板块。
- 估值与表现：PE、PB、股息率、5/20/60 日和年初至今涨跌幅、52 周区间。
- 技术指标摘要：来自 `technical-summary` 的日/周/月指标。

明确不放入：

- 原始日 K 列表。
- F10 原文。
- 全量分时、全量分笔。
- 行业/板块内排名。
- 外围市场关联资产。
- 新闻、政策、公告全文分析。

这些内容应通过独立子模块或专门场景 API 获取。

## 推荐 Agent API 清单

### 一、个股分析模块

| API | 用途 | 优先级 | 组合方法/数据源 | 备注 |
|---|---|---:|---|---|
| `/api/agent/stock-brief` | 个股简讯 | 已完成 | `GetQuote`、`GetFinanceInfo`、F10 cat0、SQLite 板块、`GetTdxStat`、`GetTdxStat2`、`technical-summary` | 主入口之一 |
| `/api/agent/stock-brief-text` | 个股简讯文本 | 已完成 | 同上 | 给 Agent 低上下文阅读 |
| `/api/agent/technical-summary` | 技术指标摘要 | 已完成 | 日/周/月 K 线本地计算 | 可独立调用，也并入 brief |
| `/api/agent/kline-summary` | K 线摘要 | 已完成 | `GetKlineDay`、`GetKlineWeekAll`、`GetKlineMonthAll` | 输出阶段涨跌、量能、关键价位、均线结构、趋势阶段、风险等级，不返回原始数组 |
| `/api/agent/kline-summary-text` | K 线摘要文本 | 已完成 | 同上 | 给 Agent 低上下文阅读 |
| `/api/agent/trade-flow-estimate` | 单日资金流估算 | 已完成 | `GetMinuteTradeAll`、`GetHistoryMinuteTradeDay` | 支持指定日期；按超大/大/中/小单分档估算 |
| `/api/agent/trade-flow-estimate-text` | 单日资金流估算文本 | 已完成 | 同上 | 给 Agent 低上下文阅读 |
| `/api/agent/f10-summary` | F10深度资料 JSON | 已完成 | F10 cat3-cat6、cat8-cat11、cat13-cat15 | 低频深度资料，不重复 `stock-brief` 的最新提示和财务字段 |
| `/api/agent/f10-summary-text` | F10深度资料文本 | 已完成 | 同上 | 给 Agent 深度分析补充阅读 |
| `/api/agent/assets/search` | A 股资产搜索 | 已完成 | SQLite 股票名称库、板块索引库 | 解析用户输入的代码、名称或拼音，并返回完整详情候选 |
| `/api/agent/assets/search-text` | A 股资产搜索文本 | 已完成 | 同上 | 给 Agent 低上下文阅读 |
| `/api/agent/assets/detail` | A 股资产详情 | 已完成 | SQLite 股票名称库、板块索引库 | 明确代码查询单只资产详情 |
| `/api/agent/sector-membership` | 查询个股完整所属板块 | 已完成 | SQLite 板块索引 | 只做归属识别，不做板块强弱和成分股对比 |
| `/api/agent/sector-membership-text` | 个股板块归属文本 | 已完成 | 同上 | 给 Agent 低上下文阅读 |
| `/api/agent/stock-in-sector` | 个股在板块/行业中的位置 | 已完成 | SQLite 板块、`GetTdxStat` | 对比同板块涨跌、阶段表现、估值等统计字段 |
| `/api/agent/stock-in-sector-text` | 个股板块位置文本 | 已完成 | 同上 | 给 Agent 低上下文阅读 |
| `/api/agent/sector-detail` | 指定板块深度分析 | 已完成 | SQLite 板块、`GetTdxStat`、板块指数日 K | 在热点扫描后按板块继续拆解成分股强弱和板块指数阶段表现 |
| `/api/agent/sector-detail-text` | 指定板块深度分析文本 | 已完成 | 同上 | 给 Agent 低上下文阅读 |
| `/api/agent/hotspot-scan` | 热点扫描与候选股 JSON | 已完成 | SQLite 板块、`GetTdxStat`、板块指数日 K | 默认按近20日涨跌幅统计板块强弱；`windowReturn` 支持指定历史窗口，返回最强20、中游20、最弱20，排除新股/异常涨幅样本 |
| `/api/agent/hotspot-scan-text` | 热点扫描与候选股文本 | 已完成 | 同上 | 给 Agent 低上下文阅读 |

### 二、板块与热点模块

| API | 用途 | 优先级 | 组合方法/数据源 | 备注 |
|---|---|---:|---|---|
| `/api/agent/sector-membership` | 查询个股完整所属板块 | 已完成 | SQLite 板块索引 | 只做归属识别，不做板块强弱和成分股对比 |
| `/api/agent/sector-detail` | 某板块深度分析 | 已完成 | SQLite 板块、`GetTdxStat`、板块指数日 K | 成分股强弱、板块指数近20/60日表现、中游股和弱势股 |
| `/api/agent/hotspot-scan` | 热点扫描与候选股 | 已完成 | `GetTdxStat`、SQLite 板块、板块指数日 K | 默认近20日排序，也支持 `metric=windowReturn&window=&offset=` 指定历史窗口，输出最强20、中游20和最弱20个板块及代表股票 |
| `/api/agent/hotspot-scan-text` | 热点扫描与候选股文本 | 已完成 | 同上 | 给 Agent 低上下文阅读 |

### 三、交易日监控模块

| API | 用途 | 优先级 | 组合方法/数据源 | 备注 |
|---|---|---:|---|---|
| `/api/agent/multi-brief` | 多股简讯批量查询 | 已完成 | `stock-brief` | 已由 watchlist 更名；只做批量 brief，不承担分时监控 |
| `/api/agent/auction` | 9:25 集合竞价分析 | 已完成 | `GetCallAuction`、`GetQuote`、近 5/20 日 K 线 | 默认聚焦开盘竞价，支持 session 切换收盘竞价或全量记录 |
| `/api/agent/market-review` | 市场级复盘/上下文 | 已完成 | 上证、深成、创业板、科创50、北证50、市场广度、热点板块、可选关注股 | 合并原 `/api/agent/noon-review` 和 `/api/agent/close-review`，按查询时间自动判断输出视角 |
| `/api/agent/intraday-alerts` | 盘中异动提醒 | 已完成 | `GetQuote`、`GetMinute` | 调用一次计算当前状态；可由调用方轮询，服务端不做后台推送 |

### 四、外围市场与资产池模块

外围市场不按个股或板块建立映射字典。A 股分析主要看全球主要资产变化带来的宏观扩散效应，
统一由 `global-market-brief` 提供。

| API | 用途 | 优先级 | 数据源 | 备注 |
|---|---|---:|---|---|
| `/api/agent/global-market-brief` | 全球市场简报 | 已完成 | 内置外围权重资产白名单、`ExQuote`、`ExBars` | 不使用 SQLite；覆盖风险偏好、亚太核心市场、商品、汇率、利率债券和全球权重股；已补入欧洲STOXX50ETF、恒生科技、韩国/台湾可用指数代理、印度、日本宽基、东证代理、丰田、索尼；Sensex30、俄罗斯RTS、日本国债暂无已验证稳定 TDX 行情，暂不硬塞无效项 |

### 五、资产池与字典模块

| API | 用途 | 优先级 | 数据源 | 备注 |
|---|---|---:|---|---|
| `/api/agent/assets/search` | 搜索 A 股资产 | 已完成 | SQLite 股票名称库、板块索引库 | 模糊输入返回完整详情候选 |
| `/api/agent/assets/search-text` | 搜索 A 股资产文本 | 已完成 | SQLite 股票名称库、板块索引库 | 中文输出搜索命中或查无结果 |
| `/api/agent/assets/detail` | 查询 A 股资产详情 | 已完成 | SQLite 股票名称库、板块索引库 | 明确代码查询单只资产详情 |

## 原子 API 保留策略

底层原子接口继续保留，但不是 Agent 主入口。

| API | 底层方法 | 用途 |
|---|---|---|
| `/api/quote` | `GetQuote` | 单股票行情 |
| `/api/kline` | `GetKlineDay` 等 | K 线调试 |
| `/api/minute` | `GetMinute`、`GetHistoryMinute` | 分时 |
| `/api/trade` | `GetMinuteTrade` | 分笔 |
| `/api/finance` | `GetFinanceInfo` | 财务原始字段 |
| `/api/f10` | `GetCompanyCategory`、`GetCompanyContent` | F10 原文调试 |
| `/api/stat` | `GetTdxStat` | 全市场统计 |
| `/api/moneyflow` | `GetTdxStat2` | 扩展统计 |
| `/api/blocks` | `GetBlockDataWithIndex` | 板块原始数据 |
| `/api/ex/quote` | `ExQuote` | 扩展市场行情 |
| `/api/ex/kline` | `ExBars`、`ExBarsRange` | 扩展市场 K 线 |
| `/api/ex/markets` | `ExMarkets` | 扩展市场列表 |

后续可补充的原子接口：

- `/api/minute/live`
- `/api/trade/all`
- `/api/history-trade/day`
- `/api/xgsg`
- `/api/block-indexes`
- `/api/stock/equity`
- `/api/stock/turnover`
- `/api/workday/is`
- `/api/workday/today`
- `/api/ex/instruments`
- `/api/ex/minute`
- `/api/ex/trade`

## 数据量限制

| 数据类型 | 默认数量 | 最大数量 | 聚合 API 策略 |
|---|---:|---:|---|
| 日 K | brief=60, normal=120, deep=250 | 500 | `kline-summary` 默认只输出摘要，可用 `dayCount` 覆盖但封顶 |
| 周 K | 全量取数 | 摘要输出 | `kline-summary` 使用 `GetKlineWeekAll`，不返回原始数组 |
| 月 K | 全量取数 | 摘要输出 | `kline-summary` 使用 `GetKlineMonthAll`，不返回原始数组 |
| 分钟 K | 摘要优先 | 800 | 盘中接口默认只返回分段摘要 |
| 分时 | 摘要优先 | 241 点 | 默认不返回完整数组 |
| 分笔 | 200 | 2000 | 默认返回成交结构摘要 |
| 全市场统计 | Top 100 | 500 | 服务端先排序筛选 |
| 板块成分 | 分页 | 500 | 默认不展开全部详情 |
| F10 正文 | 结构化提取 | 限制字符数 | Agent 聚合接口不返回原文 |
| 扩展市场 K 线 | 60 | 250 | 默认摘要化 |

不建议 Agent 直接调用：

- 全量日 K。
- 跨多日全量分笔。
- `/api/gbbq/all`。
- 原始 `zhb.zip`。
- 原始 report file。
- 全市场未筛选统计列表。

## level 参数

聚合 API 建议统一支持 `level=brief|normal|deep`。

| level | 用途 | 返回规模 |
|---|---|---|
| `brief` | 周期性监控、快速判断 | 只返回摘要字段 |
| `normal` | 常规分析 | 返回裁剪后的结构化数据和摘要 |
| `deep` | 深度分析 | 返回更多周期和关联数据，但仍受最大限制 |

## 返回结构规范

Agent 聚合 API 建议保持统一外壳：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "source": "tdx_agent",
    "updatedAt": "2026-06-25T15:00:00+08:00",
    "level": "normal",
    "limits": {},
    "warnings": [],
    "payload": {}
  }
}
```

当前已有接口暂不强制迁移外壳，避免破坏已完成调用；后续新增接口优先遵守该结构。

## 实施优先级

### 第一阶段：个股分析闭环

目标：Hermes Agent 可以稳定完成 A 股单票分析。

1. 已完成 `/api/agent/stock-brief`
2. 已完成 `/api/agent/stock-brief-text`
3. 已完成 `/api/agent/technical-summary`
4. 已完成 `/api/agent/kline-summary`
5. 已完成 `/api/agent/kline-summary-text`
6. 已完成 `/api/agent/trade-flow-estimate`
7. 已完成 `/api/agent/f10-summary`
8. 已完成 `/api/agent/assets/search`
9. 已完成 `/api/agent/assets/search-text`
10. 已完成 `/api/agent/assets/detail`

### 第二阶段：板块、热点和交易日监控

目标：支持热点扫描、持仓监控、盘中复盘。

1. 已完成 `/api/agent/sector-membership`
2. 已完成 `/api/agent/stock-in-sector`
3. 已完成 `/api/agent/hotspot-scan`
4. 已完成 `/api/agent/multi-brief`
5. 已完成 `/api/agent/auction`
6. 已完成 `/api/agent/market-review`
7. 原 `/api/agent/noon-review`、`/api/agent/close-review` 合并进 `market-review`
8. 已完成 `/api/agent/intraday-alerts`

### 第三阶段：外围市场与非交易日分析

目标：用全球主要资产支撑外围影响分析。

1. 已完成 `/api/agent/global-market-brief`
2. 扩展市场原子 API 仅作为调试保留，不纳入 Agent 主链路

## 近期建议

当前 Agent 主链路已经形成闭环。后续新增接口只有在提供新数据口径时再做；单纯编排已有接口的能力交给 Agent 调用链完成。

## 2026-06-27 逐笔资金流估算接口

`/api/agent/trade-flow-estimate` 与 `/api/agent/trade-flow-estimate-text` 已完成单日通用查询：

- 参数：`code` 必填；`date` 可选，格式 `YYYY-MM-DD` 或 `YYYYMMDD`，不传默认今天。
- 当 `date` 为今天时，使用 `GetMinuteTradeAll`；历史日期使用 `GetHistoryMinuteTradeDay`。
- 分档默认口径：
  - 超大单：成交额 >= 100 万元，或成交量 >= 50 万股。
  - 大单：成交额 >= 20 万元，或成交量 >= 10 万股。
  - 中单：成交额 >= 4 万元，或成交量 >= 2 万股。
  - 小单：其余成交。
- 各档净流入 = 主动买入金额 - 主动卖出金额。
- 主力净流入 = 超大单净流入 + 大单净流入。
- 中性成交计入总成交额，不计入买入额、卖出额和净流入。
- 返回中保留 `source=tdx_tick_estimate`、`isOfficial=false` 与阈值配置，避免误认为外部 APP 官方口径。

### 自适应分档阈值

`/api/admin/trade-flow-thresholds/refresh?code=603063` 已支持按关注股刷新自适应阈值：

- 股票范围由 `code` 指定，不做全市场刷新。
- 回看交易日硬编码为 200 天。
- 每个交易日调用一次历史逐笔接口，调用间隔 3 秒。
- 统计只使用 Go 程序排序和分位计算，不使用 LLM。
- 阈值缓存路径：`data/trade_flow_thresholds/{code}.json`。
- 分位口径：按历史逐笔成交额从大到小排序，以累计成交额占比分档。
  - 超大单：累计成交额位于 0%~10% 的成交。
  - 大单：累计成交额位于 10%~30% 的成交。
  - 中单：累计成交额位于 30%~55% 的成交。
  - 小单：累计成交额位于 55%~100% 的成交。
  - 阈值含义：超大单、大单、中单分别取上述区间下边界金额，低于中单阈值为小单。
- `trade-flow-estimate` 默认优先读取自适应阈值；缺失时回退固定阈值并返回 warning。

603063 本次刷新结果：

- 样本数：828,640 笔。
- 超大单阈值：5,470,455 元。
- 大单阈值：1,961,388 元。
- 中单阈值：751,941 元。

本次人工检查输出：

- `docs/agent_trade_flow_estimate_603063_raw_output.json`
- `docs/agent_trade_flow_estimate_603063_text_output.md`
- `docs/agent_trade_flow_thresholds_603063_refresh_output.json`

## 2026-06-27 F10 深度资料聚合接口

`/api/agent/f10-summary` 与 `/api/agent/f10-summary-text` 已完成：

- 参数：`code` 必填；`mkt` 可选。
- 递进边界：
  - 短期/快速分析优先使用 `stock-brief`、`kline-summary-text`、`trade-flow-estimate-text`。
  - 深度基本面分析再追加 `f10-summary-text`。
- 为避免重叠，`f10-summary` 明确排除：
  - `最新提示`：由 `stock-brief` 覆盖最新报告期、财务同比和关键提醒。
  - `财务分析`：由 `stock-brief` 覆盖结构化财务字段；后续如需表格级财务历史，再独立增强。
  - `公司概况`：电话、网址、地址、经营范围等基础资料噪音较高，默认排除。
  - `高管治理`：董监高履历和高管交易明细噪音较高，默认排除。
  - `公司报道`：新闻报道类信息交由 `anysearch` 补齐。
- 当前保留分类：
  - 股本结构、股东研究、机构持股、分红融资、资金动向、资本运作、热点题材、公司公告、经营分析、行业分析、研报评级。
- 降噪口径：
  - 去除 F10 表格边框线和装饰线。
  - 保留 ` | ` 列分隔符，避免表格字段混在一起。
  - `研报评级` 分类只保留投资评级统计、盈利预测统计和盈利预测明细；默认去掉 `研报摘要` 和 `机构调研`，避免长篇问答挤占 Agent 上下文。
  - 每个分类默认裁剪 900 字符。
  - 不输出原始 F10 全文；原文仍可通过 `/api/f10?cat=N` 调试查看。

本次人工检查输出：

- `docs/agent_f10_summary_603063_raw_output.json`
- `docs/agent_f10_summary_603063_text_output.md`
## 2026-06-25 K 线摘要第二批增强

`/api/agent/kline-summary` 和 `/api/agent/kline-summary-text` 已在第一批
阶段涨跌、量能、关键价位、均线结构、趋势阶段、风险等级基础上，继续追加
低噪声 K 线颗粒度字段：

- `candle`：最后一根 K 线的实体、上影线、下影线、振幅和形态。
- `volatility`：ATR、ATR 占比、5/20 根平均振幅和波动风险。
- `streak`：最近连续上涨、下跌或平盘方向、持续根数和区间涨跌幅。
- `signals`：继续保留均线与风险信号，并追加 `gap_up`、`gap_down`、
  `long_upper_shadow`、`long_lower_shadow`、连续涨跌等关键形态信号。

边界保持不变：日线按 `level/dayCount` 限量，周线和月线全量取数后只输出
摘要，不返回原始 K 线数组。
## 2026-06-25 K 线聚合接口边界修订

`/api/agent/kline-summary` 与 `/api/agent/kline-summary-text` 的职责重新对齐到
已完成聚合接口的约定：

- `/api/agent/kline-summary`：返回原始 K 线聚合数据，不作为 Agent 直接分析输入。
  - 日线支持 `level=brief|normal|deep`，分别默认返回 60/120/250 根。
  - 日线支持 `dayCount` 覆盖，最大封顶 500 根。
  - 周线、月线使用全量 K 线。
  - 每个周期返回 `items[]`，包含 `date/open/high/low/close/volume/amount`。
- `/api/agent/kline-summary-text`：返回面向 Agent 的中文清洗结果。
  - 不暴露原始 K 线数组。
  - 保留阶段涨跌、量能、关键价位、均线结构、K 线形态、ATR、连续涨跌、趋势阶段、风险提示等低噪声摘要。
  - 文本输出参考 `stock-brief-text` 的风格，用中文表达，避免英文 raw 字段和调试字段。

本次人工检查输出：

- `docs/agent_kline_summary_603063_raw_output.json`
- `docs/agent_kline_summary_603063_text_output.md`

## 2026-06-28 热点扫描参数补充

`/api/agent/hotspot-scan` 和 `/api/agent/hotspot-scan-text`：

- 默认：`sectorType=concept&metric=chg20&limit=20&topStocks=3&minMembers=20&excludeNew=true`。
- `metric=chg20|chg60|chg5|changePct|peTtm|divYield` 使用 `GetTdxStat` 当前统计字段排序，默认可返回最强、中游、最弱各 `limit` 个板块。
- `metric=windowReturn` 使用板块指数日 K 线计算历史区间收益；推荐传 `startDate=YYYY-MM-DD&endDate=YYYY-MM-DD`，也兼容 `YYYYMMDD`。
- `window`、`offset` 仍保留为兼容参数；若同时传 `startDate/endDate`，以日期区间为准，`window/offset` 不参与计算。
- `windowReturn` 依赖板块指数 K 线覆盖率；若可计算板块数不足 `limit * 3`，中游板块可能少于 `limit`，接口会在 `warnings` 中说明失败数量。若必须稳定返回每组 20，需要后续改为“板块成分股历史 K 线聚合”口径，而不是当前板块指数 K 线口径。
- 2026-06-28 修正：`windowReturn` 已改用 `GetIndexDayAll` 读取板块指数 K 线，避免 `GetKlineDayAll` 按普通股票 K 线解码导致日期错位。
- 本次人工检查输出：
  - `docs/agent_hotspot_scan_concept_window_return_2026-05-25_2026-06-26_raw_output.json`
  - `docs/agent_hotspot_scan_concept_window_return_2026-05-25_2026-06-26_text_output.md`

## 2026-06-28 多股简讯聚合接口

`/api/agent/multi-brief` 和 `/api/agent/multi-brief-text` 已由 watchlist 更名完成：

- 参数：`codes` 或 `code` 必填。
- `codes` 支持逗号、中文逗号、空格、换行分隔。
- 也支持多次传 `code=603063&code=000001`。
- 最大 20 只，重复代码自动去重。
- JSON 版复用 `stock-brief`，每只股票返回完整简讯数据，便于调试和后续编排。
- text 版降噪为每股一行：名称、现价、涨跌幅、成交额、换手率、20 日表现和主要板块。
- 第一版不维护服务端持久化关注池；关注列表由调用方传入。

本次人工检查输出：

- `docs/agent_multi_brief_603063_000001_raw_output.json`
- `docs/agent_multi_brief_603063_000001_text_output.md`

备注：该接口等价于多股 `stock-brief`，不足以支撑分时监控；分时观察使用 `intraday-alerts`。

## 2026-06-28 集合竞价分析接口

`/api/agent/auction` 和 `/api/agent/auction-text` 已完成第一版：

- 参数：`code` 必填；`session` 可选；`limit` 可选。
- `session=open|close|all`，默认 `open`。
- `open` 过滤 09:20-09:25 开盘不可撤单竞价。
- `close` 过滤 14:57-15:00 收盘竞价。
- `all` 返回全天竞价记录，用于调试。
- `limit` 默认 20，最大 100。
- 输出包含：竞价末笔时间、价格、较昨收涨跌幅、匹配量、未匹配量、未匹配方向、近 5/20 个交易日日K累计涨跌背景和竞价信号。
- 注意：`GetCallAuction` 会返回全日竞价相关记录，Agent 聚合层默认过滤为开盘竞价，避免把 15:00 附近记录误当 9:25 竞价。

本次人工检查输出：

- `docs/agent_auction_603063_raw_output.json`
- `docs/agent_auction_603063_text_output.md`

## 2026-06-28 市场级复盘接口

`/api/agent/market-review` 和 `/api/agent/market-review-text` 已完成第一版：

- 原计划的 `/api/agent/noon-review` 和 `/api/agent/close-review` 不再单独实现，统一合并为 `market-review`。
- 参数：`session` 可选，默认 `auto`。
- `session=auto` 按查询时间判断：
  - 09:30 前：`preopen` 开盘前市场背景。
  - 09:30-11:30：`current` 盘中当前状态。
  - 11:30-13:00：`morning` 上午收盘复盘。
  - 13:00-15:00：`current_with_morning_reference` 午后盘中状态。
  - 15:00 后：`full` 全天收盘复盘。
- 也支持手动传 `session=current|morning|full`。
- `codes` 可选，用于关注股联动摘要。
- `top` 可选，默认 10，最大 20，控制强/中/弱板块数量。
- 输出包含：
  - 主要指数：上证指数、深证成指、创业板指、科创50。
  - 市场广度：上涨/下跌/平盘、涨停/跌停近似数、平均涨跌、中位数涨跌。
  - 热点板块：复用热点扫描逻辑，返回强势、中游、弱势板块。
  - 关注股联动：可选 `codes` 下的个股当日和 20 日表现。
- 当前边界：
  - 市场广度来自 `GetTdxStat` 查询时点快照。
  - 涨停/跌停按 `±9.9%` 近似统计，不区分 ST、北交所和 20cm 品种。
  - 上午历史市场广度尚未缓存，15:00 后无法精确回放 11:30 宽度快照；后续如需要，应增加 11:30 定时缓存。
  - 第一版不输出全量分时数组。

本次人工检查输出：

- `docs/agent_market_review_auto_603063_000001_raw_output.json`
- `docs/agent_market_review_auto_603063_000001_text_output.md`

## 2026-06-28 盘中异动提醒接口

`/api/agent/intraday-alerts` 和 `/api/agent/intraday-alerts-text` 已完成第一版：

- 参数：`codes` 或 `code` 必填；`windowMinutes` 可选。
- `codes` 支持逗号、中文逗号、空格、换行分隔，也支持多次传 `code=603063&code=000001`。
- 最多 20 只股票；`windowMinutes` 默认 30，允许 5-60。
- JSON 版返回每只股票的当前价格、当日涨跌幅、分时近段涨跌、近段成交量变化、异动信号和提示。
- text 版返回面向 Agent 的中文摘要，只保留当前行情和有效异动结论。
- 信号口径：
  - 当日涨跌幅 `>= 5%` 标记为“当日强势”。
  - 当日涨跌幅 `<= -5%` 标记为“当日弱势”。
  - 近段涨跌幅 `>= 2%` 标记为“短时拉升”。
  - 近段涨跌幅 `<= -2%` 标记为“短时回落”。
  - 近段成交量相对前一等长窗口 `>= 2` 标记为“短时放量”。
- 当前边界：
  - 该接口是一次性查询接口，不在服务端维护关注列表、不做后台轮询、不推送。
  - 接口会先做交易日判断：周末直接视为非交易日；工作日优先用上证指数日 K 最新日期确认，判断失败时保守按工作日处理。
  - `GetMinute` 在非交易日、部分时段或部分服务器上可能返回空分时；此时接口退化为实时行情快照，并在 `warnings` 中明确提示。
  - 退化时 text 版不展示“开盘以来、近段涨跌、近段量比”等无效 0 值，避免给 Agent 造成误读。

本次人工检查输出：

- `docs/agent_intraday_alerts_603063_000001_raw_output.json`
- `docs/agent_intraday_alerts_603063_000001_text_output.md`

## 2026-06-29 指定板块深度分析接口

`/api/agent/sector-detail` 和 `/api/agent/sector-detail-text` 已完成第一版：

- 参数：`sectorName` 或 `indexCode` 必填；`sectorType` 可选；`metric` 可选；`topStocks` 可选；`excludeNew` 可选。
- `sectorType=concept|style_region|index`，默认 `concept`。
- `metric=changePct|chg5|chg20|chg60|peTtm|divYield`，默认 `chg20`。
- `topStocks` 默认 10，最大 30；同时控制强势股、中游股和弱势股返回数量。
- `excludeNew` 默认 true，过滤新股/异常涨幅样本；如需调试全量样本可传 `excludeNew=false`。
- JSON 返回板块基本信息、样本数量、上涨/下跌家数、上涨比例、排序指标均值、板块指数近 20/60 日收益、强势股、中游股和弱势股。
- text 版返回中文低噪音摘要，适合在 `hotspot-scan` 找到候选板块后继续拆解。
- 当前不拉取每只成分股实时行情；成分股强弱来自 `GetTdxStat`，板块指数阶段收益来自 `GetIndexDayAll`。
  - 映射覆盖度取决于 SQLite 资产池和关系表，当前仍是默认种子级别。

本次人工检查输出：

- `docs/agent_sector-detail_raw_output.json`
- `docs/agent_sector-detail-text_output.md`
## 模拟交易与只读 WebUI

模拟交易功能使用独立 SQLite，不与 Agent 行情分析库混用：

- 默认路径：`data/database/tdx-paper.sqlite`
- Docker 推荐挂载：`/app/data/database/tdx-paper.sqlite`
- 覆盖环境变量：`PAPER_DB_PATH`

服务端关系：

```text
Agent -> MCP 工具 -> Paper Broker Service -> tdx-paper.sqlite
WebUI -> /api/paper/* HTTP API -> Paper Broker Service -> tdx-paper.sqlite
```

WebUI 只读展示，不提供人工下单按钮，不直接读取 SQLite，也不直接调用 TDX 原始方法。

### Paper MCP 工具

| MCP 工具 | 作用 | 关键参数 |
|---|---|---|
| `tdx_paper_account` | 创建、列出、查询模拟账户 | `action=create|list|get|close|recreate`；`name`；`initialCash`；`initialPositions`；`accountId` |
| `tdx_paper_order` | 提交委托、撤单、查询委托 | `action=place|cancel|list|get`；`accountId`；`code`；`side=buy|sell`；`orderType=market|limit|auction`；`price`；`quantity`；`timeInForce=day|auction_only`；`orderId`；`reason` 交易理由 |
| `tdx_paper_portfolio` | 查询资金、持仓、成交、收益和清仓表现 | `accountId`；`view=summary|cash|positions|trades|orders|performance|closed_positions|actions`；`from`；`to`；`limit`；`code` |
| `tdx_paper_rules` | 返回模拟交易规则说明 | 无必填参数 |

当前第一版已实现账户创建/查询、下单、撤单、订单查询、持仓查询、成交查询、清仓查询和规则说明。
账户 `close/recreate` 保留 schema，后续在需要销户重建审计流程时再打开。

### Paper HTTP API

| HTTP API | 作用 | 参数 |
|---|---|---|
| `/api/paper/dashboard` | WebUI 看板总览 | `accountId` 可选；`range=today|20d|60d|all` |
| `/api/paper/accounts` | 账户列表 | 无 |
| `/api/paper/account` | 单账户详情 | `accountId` 必填；`range` 可选 |
| `/api/paper/activity` | Agent 行为与账户变动事件 | `accountId` 可选；`type` 可选；`limit` 默认 50 最大 200 |
| `/api/paper/closed-positions` | 清仓后表现 | `accountId` 必填；`range=20d|60d|all` |

### 第一版交易口径

- 支持 A股股票与 A股 ETF。
- 现金买入，持仓卖出，不支持真实券商接入、融资融券或做空。
- 委托方向：`buy`、`sell`。
- 委托类型：`market`、`limit`、`auction`。
- 数量必须为 100 的整数倍。
- 限价买入提交时冻结资金，限价卖出提交时冻结可卖持仓。
- 市价买入不预冻结资金，撮合时按服务端实时行情校验现金并成交。
- 买入成交后当天不增加可卖持仓，服务端按 A股 T+1 可卖口径约束。
- 第一版不做部分成交；一笔委托要么整笔成交，要么保持 pending。
- 撤单会释放冻结资金或冻结持仓。
- 费用固定：佣金万 1，全佣；股票卖出收印花税；股票收过户费；ETF 不收印花税。

### 验证输出

以下文件用于人工核查模拟交易接口输出：

- `docs/agent_api_test_outputs/paper_mcp_tools.json`
- `docs/agent_api_test_outputs/paper_account_create.json`
- `docs/agent_api_test_outputs/paper_rules.json`
- `docs/agent_api_test_outputs/paper_dashboard.json`

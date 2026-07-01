# Agent Text 聚合接口表

本文档只列面向 Agent 直接阅读的 `_text` 聚合接口。JSON 调试接口、原子接口和已取消接口不列入主链路。

## Text 接口清单

| 接口原名 | 接口中文名 | 包含方法 | 参数说明 | 输出内容简述 |
|---|---|---|---|---|
| `/api/agent/assets/search-text` | A 股资产搜索文本 | SQLite 股票名称库、板块索引库 | `keyword` 必填，也支持 `q`；`limit` 可选，默认 20，最大 50 | 根据代码、名称或拼音搜索 A 股；命中时输出标准代码、名称和主要板块；查无结果时明确提示“查无此股票”。 |
| `/api/agent/stock-brief-text` | 个股简讯文本 | `GetQuote`、`GetFinanceInfo`、F10 最新提示、SQLite 板块、`GetTdxStat`、`GetTdxStat2`、本地技术指标 | `code` 必填；`mkt` 可选 | 单股快速入口，输出行情、成交额、换手、财务规模、最新财报提示、估值、阶段表现、板块和日/周/月技术摘要。 |
| `/api/agent/kline-summary-text` | K 线走势摘要文本 | `GetKlineDay`、`GetKlineWeekAll`、`GetKlineMonthAll`、本地 K 线指标计算 | `code` 必填；`level=brief\|normal\|deep` 可选；`dayCount` 可选，最大 500 | 输出日/周/月走势、阶段涨跌、量能、关键价位、均线结构、K 线形态、ATR、连续涨跌、趋势阶段和风险提示。 |
| `/api/agent/trade-flow-estimate-text` | 单日资金流估算文本 | `GetMinuteTradeAll`、`GetHistoryMinuteTradeDay`、本地逐笔分档统计、自适应阈值缓存 | `code` 必填；`date` 可选，支持 `YYYY-MM-DD` 或 `YYYYMMDD`，不传默认今天 | 按超大/大/中/小单估算买入、卖出和净额，说明阈值口径，并明确不是外部 APP 官方资金流。 |
| `/api/agent/f10-summary-text` | F10 深度资料文本 | `GetCompanyCategory`、`GetCompanyContent`、F10 分类裁剪清洗 | `code` 必填；`mkt` 可选 | 输出股本、股东、机构、分红融资、资金动向、资本运作、热点题材、公告、经营、行业、研报评级等低频资料；不重复 brief 财务字段；研报评级去掉研报摘要和机构调研长文。 |
| `/api/agent/sector-membership-text` | 个股板块归属文本 | SQLite `block_memberships` | `code` 必填 | 输出该股所属概念、地域/风格、指数板块，用于后续选择板块分析入口。 |
| `/api/agent/stock-in-sector-text` | 个股板块内位置文本 | SQLite 板块、`GetTdxStat` | `code` 必填；`sectorType`、`sectorName`、`metric`、`limit` 可选 | 输出个股在指定或默认所属板块中的相对排名、阶段表现和同板块比较。 |
| `/api/agent/sector-detail-text` | 指定板块深度分析文本 | SQLite 板块、`GetTdxStat`、`GetIndexDayAll` | `sectorName` 或 `indexCode` 必填；`sectorType` 可选；`metric` 可选；`topStocks` 可选；`excludeNew` 可选 | 输出板块样本数、上涨比例、板块指数近 20/60 日表现、强势股、中游股和弱势股。 |
| `/api/agent/hotspot-scan-text` | 热点扫描文本 | SQLite 板块、`GetTdxStat`、`GetIndexDayAll` | `sectorType`、`metric`、`startDate`、`endDate`、`window`、`offset`、`limit`、`topStocks`、`minMembers`、`excludeNew` 可选 | 输出强势、中游、弱势板块及代表股票；支持近 5/20/60 日或指定历史窗口。 |
| `/api/agent/multi-brief-text` | 多股简讯文本 | 批量复用 `stock-brief` | `codes` 或多个 `code` 必填；最多 20 只 | 一次输出多只股票的简短行情、成交、换手、20 日表现和主要板块；适合快速扫关注池。 |
| `/api/agent/auction-text` | 集合竞价分析文本 | `GetCallAuction`、`GetQuote`、近 5/20 日日 K | `code` 必填；`session=open\|close\|all` 可选；`limit` 可选 | 默认输出 09:20-09:25 开盘不可撤单竞价摘要，包括末笔价格、较昨收涨跌、匹配量、未匹配方向和竞价信号。 |
| `/api/agent/intraday-alerts-text` | 盘中异动提醒文本 | `GetQuote`、`GetMinute`、交易日判断、本地分时窗口计算 | `codes` 或多个 `code` 必填；`windowMinutes` 可选，默认 30，范围 5-60 | 输出多只股票当前行情、短时涨跌、短时放量和异动信号；非交易日或分时不可用时明确降级为行情快照。 |
| `/api/agent/market-review-text` | 市场级复盘文本 | 指数行情、`GetTdxStat`、热点扫描、可选关注股联动 | `session=auto\|current\|morning\|full` 可选；`codes` 可选；`top` 可选，默认 10，最大 20 | 按查询时间输出盘前、盘中、午间或收盘视角，包含上证、深成、创业板、科创50、北证50、市场广度、强/中/弱板块和关注股联动。 |
| `/api/agent/global-market-brief-text` | 全球外围市场简报文本 | 内置全球主要资产白名单、`ExQuote`、`ExBars`、本地 20/60 日区间计算 | 无参数 | 输出全球风险偏好、亚太核心市场、商品、汇率、利率债券和全球权重股的当日、20 日、60 日表现及区间位置；含欧洲STOXX50ETF、恒生科技、韩国/台湾可用指数代理。 |

## 参数解释

### 通用参数

| 参数 | 适用接口 | 含义 | 可选值/格式 | 默认值 | 使用建议 |
|---|---|---|---|---|---|
| `code` | 单股接口 | A 股 6 位股票代码 | 如 `300499`、`603063` | 无 | 用户已经给出明确代码时直接使用。 |
| `codes` | 多股接口 | 多个 A 股代码 | 逗号、中文逗号、空格、换行分隔，如 `300499,603063` | 无 | 用于多股简讯、盘中异动、市场复盘关注股联动。 |
| `mkt` | `stock-brief-text`、`f10-summary-text` | 手动指定市场 | 通常可省略；代码无法判断市场时再传 | 自动识别 | 日常 A 股分析不建议传，避免误设。 |
| `limit` | 搜索、板块、热点等接口 | 控制返回条数 | 正整数，各接口上限不同 | 由接口决定 | Agent 默认少取，人工要求扩展时再调大。 |
| `metric` | 板块/排序类接口 | 排序指标 | `changePct`、`chg5`、`chg20`、`chg60`、`peTtm`、`divYield`、`windowReturn` | 多数为 `chg20` | 中短线看 `chg20`，中期主线看 `chg60`，当日异动看 `changePct`。 |
| `sectorType` | 板块接口 | 板块类别 | `concept`、`style_region`、`index` | 多数为 `concept` | 做题材和行业分析时优先 `concept`。 |

### 资产搜索

| 参数 | 接口 | 含义 | 可选值/格式 | 默认值 | 使用建议 |
|---|---|---|---|---|---|
| `keyword` | `assets/search-text` | 用户输入的股票代码、名称、简称或拼音 | 任意字符串，如 `高澜`、`300499`、`博盈特焊` | 无 | 推荐作为用户自然语言输入后的第一步解析。 |
| `q` | `assets/search-text` | `keyword` 的别名 | 任意字符串 | 无 | 兼容简写；有 `keyword` 时优先用 `keyword`。 |
| `limit` | `assets/search-text` | 最大候选数量 | 1-50 | 20 | 多候选时让 Agent 或用户确认，不要盲选。 |

### K 线摘要

| 参数 | 接口 | 含义 | 可选值/格式 | 默认值 | 使用建议 |
|---|---|---|---|---|---|
| `level` | `kline-summary-text` | 日线取样深度 | `brief`、`normal`、`deep` | `normal` | 快速分析用 `normal`；深度复盘再用 `deep`。 |
| `dayCount` | `kline-summary-text` | 覆盖日线数量 | 正整数，最大 500 | 跟随 `level` | 只有用户明确要求特定周期时使用。 |

### 资金流估算

| 参数 | 接口 | 含义 | 可选值/格式 | 默认值 | 使用建议 |
|---|---|---|---|---|---|
| `date` | `trade-flow-estimate-text` | 需要估算的交易日期 | `YYYY-MM-DD` 或 `YYYYMMDD` | 今天 | 非交易日或盘后复盘建议指定日期，避免 Agent 误读当前日期。 |

### 个股板块位置

| 参数 | 接口 | 含义 | 可选值/格式 | 默认值 | 使用建议 |
|---|---|---|---|---|---|
| `sectorName` | `stock-in-sector-text`、`sector-detail-text` | 指定板块名称 | 如 `液冷服务`、`光伏` | `stock-in-sector-text` 默认选该股第一个概念板块 | 用户关注某个题材时显式传入，避免默认板块不符合分析目标。 |
| `indexCode` | `sector-detail-text` | 指定板块指数代码 | 如 `880685` | 无 | 板块名可能重复时使用指数代码更稳定。 |
| `topStocks` | `sector-detail-text`、`hotspot-scan-text` | 每个板块展示代表股票数量 | 正整数，`sector-detail` 最大 30，`hotspot-scan` 最大 10 | 多数为 3 或 10 | 人工检查板块时可调大；Agent 默认不要过大。 |
| `excludeNew` | `sector-detail-text`、`hotspot-scan-text` | 是否排除新股/异常涨幅样本 | `true`、`false` | `true` | 默认保持 `true`，避免新股异常涨幅污染板块强弱判断。 |

### 热点扫描窗口

| 参数 | 接口 | 含义 | 可选值/格式 | 默认值 | 使用建议 |
|---|---|---|---|---|---|
| `startDate` | `hotspot-scan-text` | 历史窗口起始日期 | `YYYY-MM-DD` 或 `YYYYMMDD` | 无 | 与 `endDate` 同时使用，适合回看某段行情。 |
| `endDate` | `hotspot-scan-text` | 历史窗口结束日期 | `YYYY-MM-DD` 或 `YYYYMMDD` | 无 | 与 `startDate` 同时使用时，优先于 `window/offset`。 |
| `window` | `hotspot-scan-text` | 历史窗口长度 | 交易日数量 | 无 | 兼容旧用法；新调用优先用日期区间。 |
| `offset` | `hotspot-scan-text` | 从当前往前偏移的交易日数量 | 交易日数量 | 0 | 兼容旧用法；新调用优先用日期区间。 |
| `minMembers` | `hotspot-scan-text` | 板块最小成分股数量 | 正整数 | 20 | 过滤过小板块，避免样本太少导致排序失真。 |

### 竞价、盘中和市场复盘

| 参数 | 接口 | 含义 | 可选值/格式 | 默认值 | 使用建议 |
|---|---|---|---|---|---|
| `session` | `auction-text` | 竞价阶段 | `open`、`close`、`all` | `open` | 真实开盘集合竞价分析用 `open`，调试全天竞价记录用 `all`。 |
| `session` | `market-review-text` | 市场复盘视角 | `auto`、`current`、`morning`、`full` | `auto` | 常规使用 `auto`，由查询时间决定输出盘中/午间/收盘视角。 |
| `windowMinutes` | `intraday-alerts-text` | 分时近段窗口 | 5-60 分钟 | 30 | 观察短时异动用 15 或 30；过短容易噪音高。 |
| `top` | `market-review-text` | 市场复盘展示板块数量 | 1-20 | 10 | 人工复盘可调到 20；Agent 快速判断保持默认。 |

## 场景调用组合与顺序

### 1. 用户输入股票名称或不确定代码

1. `/api/agent/assets/search-text?keyword=...`
2. 若唯一命中，取 `code` 继续后续分析；若多项命中，由 Agent 让用户确认。

### 2. 单股快速分析

1. `/api/agent/stock-brief-text?code=...`
2. `/api/agent/kline-summary-text?code=...&level=normal`
3. 必要时追加 `anysearch`：补最新新闻、政策、公告全文、行业事件。

### 3. 单股深度分析

1. `/api/agent/stock-brief-text?code=...`
2. `/api/agent/kline-summary-text?code=...&level=normal`
3. `/api/agent/trade-flow-estimate-text?code=...`
4. `/api/agent/f10-summary-text?code=...`
5. `/api/agent/sector-membership-text?code=...`
6. `/api/agent/stock-in-sector-text?code=...`
7. `/api/agent/global-market-brief-text`
8. `anysearch`：补公司公告全文、监管问询、行业新闻、政策变化、海外事件。

### 4. 板块机会分析

1. `/api/agent/hotspot-scan-text?sectorType=concept&metric=chg20&limit=20`
2. 选定板块后调用 `/api/agent/sector-detail-text?sectorName=...&sectorType=concept&metric=chg20`
3. 对板块内候选股调用 `/api/agent/stock-brief-text?code=...`
4. `anysearch`：补板块催化、产业政策、订单/价格变化、行业事件。

### 5. 盘中观察

1. `/api/agent/market-review-text?session=auto&codes=...`
2. `/api/agent/intraday-alerts-text?codes=...&windowMinutes=30`
3. 重点个股追加 `/api/agent/stock-brief-text?code=...`
4. 交易开盘前后可追加 `/api/agent/auction-text?code=...`
5. `anysearch`：补盘中突发新闻、政策传闻、公告异动原因。

### 6. 多股关注池快速扫盘

1. `/api/agent/multi-brief-text?codes=...`
2. 对异常个股调用 `/api/agent/intraday-alerts-text?codes=...`
3. 对重点个股再进入单股快速或深度分析链。

### 7. 宏观外围影响分析

1. `/api/agent/global-market-brief-text`
2. `/api/agent/market-review-text?session=auto`
3. `anysearch`：补美联储、汇率、商品、地缘风险、海外科技权重股新闻。

## 完善与精简建议

1. 保持当前接口数量，不再新增只做编排的 `stock-deep`、`sector-ranking`、`market-baseline`。
2. `technical-summary` 不需要新增 text 版；技术文字已经由 `stock-brief-text` 和 `kline-summary-text` 覆盖。
3. `assets/detail` 不需要 text 版；`assets/search-text` 已可承担用户输入解析场景，明确代码可直接进入分析链。
4. `global-market-brief-text` 当前信息量偏大，后续如人工检查觉得过长，可以增加 `level=brief`，只输出每组最关键资产。
5. `f10-summary-text` 已去掉研报摘要和机构调研长文；如仍偏长，下一步优先压缩“行业分析”的排名表，而不是继续砍经营、股东、资金等核心资料。

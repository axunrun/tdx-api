# Agent Text 聚合接口表

本文档只列面向 Agent 直接阅读的 `_text` 聚合接口。JSON 调试接口、原子接口和已取消接口不列入主链路。

## Text 接口清单

| 接口原名 | 接口中文名 | 包含方法 | 参数说明 | 输出内容简述 |
|---|---|---|---|---|
| `/api/agent/assets/search-text` | A 股资产搜索文本 | SQLite 股票名称库、板块索引库 | `keyword` 必填，也支持 `q`；`limit` 可选，默认 20，最大 50 | 根据代码、名称或拼音搜索 A 股；命中时输出标准代码、名称和主要板块；查无结果时明确提示“查无此股票”。 |
| `/api/agent/stock-brief-text` | 个股基本快照文本 | `GetQuote`、`GetFinanceInfo`、F10 最新提示、SQLite 板块、估值统计 | `code` 必填；`mkt` 可选 | 输出行情、成交额、换手、财务规模、最新财报提示、估值和板块；不包含技术指标、周期涨跌及52周区间。 |
| `/api/agent/technical-score-text` | 统一技术评分文本 | 前复权日K、周/月K、本地统一指标计算、日线逐笔主动买卖估算 | `code` 必填；`dayCount=60..500`、`adjust=qfq\|none`、`includeWeeklyMonthly=true\|false`可选；无`level`参数 | 输出MA、MACD、RSI、BOLL、KDJ、BIAS、ATR、OBV、量价和日线多空比；明确查询时间、周期日期和盘中状态。 |
| `/api/agent/kline-summary-text` | K 线价格结构文本 | `GetKlineDay`、`GetKlineWeekAll`、`GetKlineMonthAll`、本地价格结构计算 | `code` 必填；`level=brief\|normal\|deep`、`dayCount`、`adjust=qfq\|none` 可选 | 按周期分行输出区间涨跌、最高最低、回撤、波动、位置、K线形态、连续涨跌、关键价位和52周区间；不重复技术指标和叙述摘要。 |
| `/api/agent/trade-flow-estimate-text` | 单日资金流估算文本 | `GetMinuteTradeAll`、`GetHistoryMinuteTradeDay`、本地逐笔分档统计、自适应阈值缓存 | `code` 必填；`date` 可选，支持 `YYYY-MM-DD` 或 `YYYYMMDD`，不传默认今天 | 按超大/大/中/小单估算买入、卖出和净额，说明阈值口径，并明确不是外部 APP 官方资金流。 |
| `/api/agent/margin-trading-text` | 个股融资融券文本 | 沪深北交易所官方披露、本地缓存与区间计算 | `code` 必填；`days` 默认30；`mode=summary\|full` 默认summary | summary输出最新值、区间变化和高低点；full输出逐日明细；days按实际披露交易日计数。 |
| `/api/agent/f10-summary-text` | F10 深度资料文本 | `GetCompanyCategory`、`GetCompanyContent`、F10 分类裁剪清洗 | `code` 必填；`mkt`、`sections` 可选 | 默认输出全部保留分类；sections可精确选择分类；文本不重复分类用途和排除清单，研报评级去掉研报摘要和机构调研长文。 |
| `/api/agent/sector-membership-text` | 个股板块归属文本 | SQLite `block_memberships` | `code` 必填 | 输出该股所属概念、地域/风格、指数板块，用于后续选择板块分析入口。 |
| `/api/agent/stock-in-sector-text` | 个股板块内位置文本 | SQLite 板块、`GetTdxStat` | `code` 必填；`sectorType`、`sectorName`、`metric`、`limit` 可选；`metric` 默认 `chg20` | 仅使用最近完整TdxStat统计日，输出查询时间、统计日期、个股排名、百分位和同板块比较；不是盘中实时排名。 |
| `/api/agent/sector-detail-text` | 指定板块深度分析文本 | SQLite 板块、`GetTdxStat`、`GetIndexDayAll` | `sectorName` 或 `indexCode` 必填；`sectorName` 必须与 TDX 名称精确一致；`sectorType`、`metric`、`topStocks`、`excludeNew` 可选 | 输出板块指数最近完整交易日单日涨跌及日期、截至该日的近 20/60 日表现，并另列成分股统计日期、上涨比例和强中弱股票。 |
| `/api/agent/sector-realtime-text` | 指定板块盘中实时涨跌文本 | SQLite 板块、`GetIndexDay` 当日日 K 实时字段 | `sectorName` 或 `indexCode` 必填；`sectorType` 可选 | 仅工作日 09:30-11:30、13:00-15:00 返回实时板块指数涨跌、交易日期和查询时点；其他时段明确返回无实时数据，不回退历史值。 |
| `/api/agent/hotspot-scan-text` | 热点扫描文本 | SQLite 板块、`GetTdxStat`、`GetIndexDayAll` | `sectorType`、`metric`、`startDate`、`endDate`、`window`、`offset`、`limit`、`topStocks`、`minMembers`、`excludeNew` 可选 | 日期口径统一写在头部；每个板块以指数/成分两行输出单日与周期指数收益、成分平均、上涨比例和代表股。 |
| `/api/agent/multi-brief-text` | 多股简讯文本 | 批量复用 `stock-brief` | `codes` 或多个 `code` 必填；最多 20 只 | 一次输出多只股票的简短行情、成交、换手、20 日表现和主要板块；适合快速扫关注池。 |
| `/api/agent/auction-text` | 集合竞价分析文本 | `GetCallAuction`、`GetQuote`、近 5/20 日日 K | `code` 必填；`session=open\|close\|all` 可选；`limit` 可选 | 输出查询时间、竞价行情日期和状态、末笔价格、较昨收涨跌、TDX原始数量口径的匹配/未匹配量，以及标明截止日期的日K背景。 |
| `/api/agent/intraday-alerts-text` | 盘中异动提醒文本 | `GetQuote`、`GetMinute`、交易日判断、本地分时窗口计算 | `codes` 或多个 `code` 必填；`windowMinutes` 可选，默认 30，范围 5-60 | 输出查询时间；异动股逐只展开，无明显异动股合并一行，重复告警归并；非交易日或分时不可用时明确降级。 |
| `/api/agent/market-review-text` | 市场级复盘文本 | 指数行情、严格A股实时行情、`GetTdxStat`最近完整盘后统计、热点扫描、可选关注股联动 | `session=auto\|current\|morning\|full` 可选；`codes` 可选；`top` 可选，默认 5，范围 1-5 | 按查询时间识别非交易日、盘前、09:20-09:25集合竞价、竞价结束、盘中、午休或收盘；同时分列当前实时广度和最近完整盘后广度，明确日期、时点、股票范围及有效样本数。 |
| `/api/agent/global-market-brief-text` | 全球外围市场简报文本 | 内置全球主要资产白名单、`ExQuote`、`ExBars`、本地 20/60 日区间计算 | 无参数 | 统一输出生成时间；输出各资产当日、20日、60日表现和区间位置，不重复静态入选理由。 |
| `/api/agent/scenario-valuation-text` | 三情景估值计算文本 | `stock-brief`行情、PE和财务同比、本地公式 | `code` 必填；`years`、`currentPrice`、`eps`、三组增长率和目标PE可选 | 输出价格来源及时效、PE统计日期、三情景目标EPS、目标价、涨跌幅和年化收益；不支持无效的 `level`、`assumptionMode`。 |
| `/api/agent/implied-expectation-text` | 当前价格隐含预期文本 | `stock-brief`行情、PE、最新财报、本地公式 | `code` 必填；`years`、`currentPrice`、`eps`、`targetPE`可选 | 输出价格来源及时效、PE统计日期、净利润报告期、隐含未来EPS和复合增速；不支持无效的 `level`。 |

## 参数解释

### 通用参数

| 参数 | 适用接口 | 含义 | 可选值/格式 | 默认值 | 使用建议 |
|---|---|---|---|---|---|
| `code` | 单股接口 | A 股 6 位股票代码 | 如 `300499`、`603063` | 无 | 用户已经给出明确代码时直接使用。 |
| `codes` | 多股接口 | 多个 A 股代码 | 逗号、中文逗号、空格、换行分隔，如 `300499,603063` | 无 | 用于多股简讯、盘中异动、市场复盘关注股联动。 |
| `mkt` | `stock-brief-text`、`f10-summary-text` | 手动指定市场 | 通常可省略；代码无法判断市场时再传 | 自动识别 | 日常 A 股分析不建议传，避免误设。 |
| `limit` | 搜索、板块、热点等接口 | 控制返回条数 | 正整数，各接口上限不同 | 由接口决定 | Agent 默认少取，人工要求扩展时再调大。 |
| `metric` | 板块/排序类接口 | 排序指标 | `dailyReturn`、`changePct`、`chg5`、`chg20`、`chg60`、`peTtm`、`divYield`、`windowReturn` | 多数为 `chg20` | 热点扫描中 `dailyReturn` 是板块指数最近完整交易日单日涨跌；`changePct` 是 TdxStat 成分股平均单日涨跌；历史板块指数区间排序使用 `windowReturn`。 |
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

### 技术评分

| 参数 | 接口 | 含义 | 可选值/格式 | 默认值 | 使用建议 |
|---|---|---|---|---|---|
| `dayCount` | `technical-score-text` | 日线请求数量和展示口径 | 60-500 | 250 | 递推指标始终至少使用250根可用K线预热；通常保持默认。 |
| `adjust` | `technical-score-text` | 日线复权方式 | `qfq`、`none` | `qfq` | 常规分析使用前复权；核对原始价格时使用none。 |
| `includeWeeklyMonthly` | `technical-score-text` | 是否同时输出周线、月线指标 | `true`、`false` | `true` | 高频盘中判断可设false降低上下文；深度分析保持true。 |

### 资金流估算

| 参数 | 接口 | 含义 | 可选值/格式 | 默认值 | 使用建议 |
|---|---|---|---|---|---|
| `date` | `trade-flow-estimate-text` | 需要估算的交易日期 | `YYYY-MM-DD` 或 `YYYYMMDD` | 今天 | 非交易日或盘后复盘建议指定日期，避免 Agent 误读当前日期。 |
| `days` | `margin-trading-text` | 从最新披露记录向前取实际交易日数量 | 1-120 | 30 | 深度核查可增加；不按自然日计数。 |
| `mode` | `margin-trading-text` | 融资融券文本颗粒度 | `summary`、`full` | `summary` | Agent常规分析用summary；只有逐日复核时用full。 |

### F10 分类

| 参数 | 接口 | 含义 | 可选值/格式 | 默认值 | 使用建议 |
|---|---|---|---|---|---|
| `sections` | `f10-summary-text` | 精确选择F10分类 | 逗号分隔：股本结构、股东研究、机构持股、分红融资、资金动向、资本运作、热点题材、公司公告、经营分析、行业分析、研报评级 | 全部保留分类 | 已知分析目标时只取所需分类，降低上下文。 |

### 个股板块位置

| 参数 | 接口 | 含义 | 可选值/格式 | 默认值 | 使用建议 |
|---|---|---|---|---|---|
| `sectorName` | `stock-in-sector-text`、`sector-detail-text`、`sector-realtime-text` | 指定板块名称 | 如 `液冷服务`、`光伏` | `stock-in-sector-text` 默认选该股第一个概念板块 | `sector-detail` 和 `sector-realtime` 要求与 TDX 板块名精确一致；名称不确定时使用 `indexCode`。 |
| `indexCode` | `sector-detail-text`、`sector-realtime-text` | 指定板块指数代码 | 如 `880685` | 无 | 板块名可能重复时使用指数代码更稳定。 |
| `topStocks` | `sector-detail-text`、`hotspot-scan-text` | 每个板块展示股票数量 | 正整数，`sector-detail` 最大 30，`hotspot-scan` 最大 10 | 多数为 3 或 10 | `sector-detail` 表示强势、中游、弱势各最多返回该数量；热点标准周期按所选 `metric` 返回强势/抗跌股。 |
| `excludeNew` | `sector-detail-text`、`hotspot-scan-text` | 是否排除新股/异常涨幅样本 | `true`、`false` | `true` | 两者排除 N/C 新股及单日涨幅超过100%的样本；`sector-detail` 的涨跌类 metric 还排除对应排序值超过100%的样本。 |

### 热点扫描窗口

| 参数 | 接口 | 含义 | 可选值/格式 | 默认值 | 使用建议 |
|---|---|---|---|---|---|
| `startDate` | `hotspot-scan-text` | `windowReturn` 请求窗口起始日期 | `YYYY-MM-DD` 或 `YYYYMMDD` | 无 | 必须与 `endDate` 同时提供并优先于 `window/offset`；输出标明实际首个交易日。 |
| `endDate` | `hotspot-scan-text` | `windowReturn` 请求窗口结束日期 | `YYYY-MM-DD` 或 `YYYYMMDD` | 无 | 必须与 `startDate` 同时提供且不得更早；输出标明实际末个交易日。 |
| `window` | `hotspot-scan-text` | 未传日期时的指数窗口长度 | 1-250个交易日 | 20 | 仅用于 `windowReturn`。 |
| `offset` | `hotspot-scan-text` | 从最新板块指数交易日向前偏移 | 0-500个交易日 | 0 | 仅用于未传日期的 `windowReturn`。 |
| `minMembers` | `hotspot-scan-text` | 板块最小有效成分股数量 | 正整数 | 20 | 按最近完整交易日TdxStat匹配并执行异常过滤后，样本不足的板块不参与排序。 |

### 竞价、盘中和市场复盘

| 参数 | 接口 | 含义 | 可选值/格式 | 默认值 | 使用建议 |
|---|---|---|---|---|---|
| `session` | `auction-text` | 竞价阶段 | `open`、`close`、`all` | `open` | 真实开盘集合竞价分析用 `open`，调试全天竞价记录用 `all`。 |
| `session` | `market-review-text` | 市场复盘视角 | `auto`、`current`、`morning`、`full` | `auto` | 常规使用 `auto`，由系统时间识别非交易日、盘前、集合竞价、盘中、午休和收盘；其他值只改变文字视角，不会回溯或改写数据日期。 |
| `windowMinutes` | `intraday-alerts-text` | 分时近段窗口 | 5-60 分钟 | 30 | 观察短时异动用 15 或 30；过短容易噪音高。 |
| `top` | `market-review-text` | 强势、中游、弱势板块各自展示数量 | 1-5 | 5 | 仅用于少量市场环境摘要；完整排名使用 `hotspot-scan-text`。 |

## 场景调用组合与顺序

### 1. 用户输入股票名称或不确定代码

1. `/api/agent/assets/search-text?keyword=...`
2. 若唯一命中，取 `code` 继续后续分析；若多项命中，由 Agent 让用户确认。

### 2. 单股快速分析

1. `/api/agent/stock-brief-text?code=...`
2. `/api/agent/technical-score-text?code=...&includeWeeklyMonthly=false`
3. `/api/agent/kline-summary-text?code=...&level=normal`
4. 必要时追加 `anysearch`：补最新新闻、政策、公告全文、行业事件。

### 3. 单股深度分析

1. `/api/agent/stock-brief-text?code=...`
2. `/api/agent/technical-score-text?code=...&includeWeeklyMonthly=true`
3. `/api/agent/kline-summary-text?code=...&level=normal`
4. `/api/agent/trade-flow-estimate-text?code=...`
5. `/api/agent/margin-trading-text?code=...&days=30&mode=summary`
6. `/api/agent/f10-summary-text?code=...`，已知目标时用`sections`缩小分类。
7. `/api/agent/stock-in-sector-text?code=...`，直接使用brief已返回的板块，不重复调用membership。
8. `/api/agent/global-market-brief-text`
9. `anysearch`：补公司公告全文、监管问询、行业新闻、政策变化、海外事件。

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
2. `technical-summary` 不需要新增 text 版；Agent技术指标统一使用 `technical-score-text`，`stock-brief-text`和`kline-summary-text`均不重复技术指标。
3. `assets/detail` 不需要 text 版；`assets/search-text` 已可承担用户输入解析场景，明确代码可直接进入分析链。
4. `global-market-brief-text` 当前信息量偏大；暂不改变默认完整输出，后续只有在调用频率明显增加时再考虑按资产组筛选。
5. `f10-summary-text` 已去掉研报摘要和机构调研长文；如仍偏长，下一步优先压缩“行业分析”的排名表，而不是继续砍经营、股东、资金等核心资料。

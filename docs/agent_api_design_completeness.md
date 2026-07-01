# Agent API Design 完整性检查

检查日期：2026-06-30
测试股票：300499（高澜股份）

## 结论

当前 Agent API 架构已经形成闭环，可以支撑 A 股分析主流程。

已覆盖能力：

- 资产识别：`assets/search`、`assets/detail`
- 单股快速分析：`stock-brief`、`stock-brief-text`
- 技术与周期走势：`technical-summary`、`kline-summary`、`kline-summary-text`
- 单日资金流估算：`trade-flow-estimate`、`trade-flow-estimate-text`
- 深度基本面：`f10-summary`、`f10-summary-text`
- 板块归属与相对位置：`sector-membership`、`stock-in-sector`
- 板块深度拆解：`sector-detail`
- 全市场热点：`hotspot-scan`
- 多股快速浏览：`multi-brief`
- 竞价与盘中：`auction`、`intraday-alerts`
- 市场复盘：`market-review`
- 外围宏观扩散效应：`global-market-brief`

## 已收口的取消项

以下接口不再作为 Agent 主链路开发目标：

- `stock-deep`：只是已有接口编排，交给 Agent 自行组合调用。
- `sector-ranking`：由 `hotspot-scan` 覆盖。
- `market-baseline`：由 `market-review`、`global-market-brief` 和 Agent 时间判断覆盖。
- `asset-quote`：对精选外围资产没有比 `global-market-brief` 更深的信息。
- `universe/list`、`universe/assets`：当前不维护外围资产池数据库。
- `related-assets`、`sector-related-assets`：取消个股/板块到外围资产的低置信度映射，外围影响统一看全球主要资产。

## 本次测试输出

输出文件使用接口路径规范化命名：`/api/agent/stock-brief-text` -> `api_agent_stock-brief-text.md`。
涉及个股的接口统一使用 `code=300499`。
`sector-detail` 使用 300499 所属概念板块 `液冷服务`。

## 需要人工重点检查

- `stock-brief-text` 是否适合作为快速分析入口。
- `kline-summary-text` 是否信息密度合适。
- `trade-flow-estimate-text` 是否清楚表达“估算口径”。
- `f10-summary-text` 是否仍有 F10 噪音。
- `sector-detail-text` 中强势/中游/弱势股是否便于做板块判断。
- `global-market-brief-text` 是否过长；如过长，可后续只保留每组关键资产摘要。

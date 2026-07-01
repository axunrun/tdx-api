# Agent 聚合 API 调试输出

生成时间：2026-06-24

## 1. 当前实现状态

之前讨论的目标架构是：

```text
底层 tdx 方法
  -> 内部服务层整理、限量、补中文、补代码含义
    -> Agent 聚合 API
```

当前代码里已经开始实现这个方向，但还没有完整实现全部聚合接口。

已实现的 Agent 聚合接口只有：

```text
GET /api/agent/technical-summary?code=603063
```

这个接口已经符合上述三层思路：

```text
底层 tdx 方法：
  GetKlineDay
  GetKlineWeek
  GetKlineMonth
  protocol.Klines.MA
  protocol.Klines.MACD
  protocol.Klines.RSI
  protocol.Klines.BOLL
  protocol.Klines.ATR

内部服务层：
  限制日线 250 根、周线 156 根、月线 120 根
  只返回最后一个有效技术指标值
  不返回原始 K 线数组
  增加中文周期名和中文信号
  增加 available/reason，避免数据不足时把 0 误判为真实指标

Agent 聚合 API：
  /api/agent/technical-summary
```

尚未实现的 Agent 聚合接口：

```text
/api/agent/stock-brief
/api/agent/stock-deep
/api/agent/company-profile
/api/agent/hotspot-scan
/api/agent/watchlist
/api/agent/auction
/api/agent/noon-review
/api/agent/close-review
/api/agent/related-assets
/api/agent/sector-related-assets
/api/agent/global-market-brief
/api/agent/market-baseline
```

这些目前还是设计清单，没有实际路由和 handler。

## 2. 已确认接口：technical-summary

### 2.1 调用方式

```http
GET /api/agent/technical-summary?code=603063
```

本地完整地址：

```text
http://127.0.0.1:8080/api/agent/technical-summary?code=603063
```

参数：

| 参数 | 必填 | 示例 | 说明 |
|---|---|---|---|
| code | 是 | 603063 | A 股 6 位股票代码 |

### 2.2 接口定位

这个接口是个股技术指标摘要接口。

它不是 K 线接口，也不是单独的 MACD 接口，而是一次性聚合：

- 日线技术指标
- 周线技术指标
- 月线技术指标

每个周期返回：

- MA
- MACD
- RSI
- BOLL
- ATR
- 中文 signals

## 3. technical-summary 内部方法组成

| 输出模块 | 底层方法 | 数据限制 | 说明 |
|---|---|---:|---|
| 日线 K 线 | GetKlineDay(code, 0, 250) | 250 | 用于计算日线技术指标 |
| 周线 K 线 | GetKlineWeek(code, 0, 156) | 156 | 用于计算周线技术指标 |
| 月线 K 线 | GetKlineMonth(code, 0, 120) | 120 | 用于计算月线技术指标 |
| MA | Klines.MA(n) | n=5/10/20/60/120 | 均线位置 |
| MACD | Klines.MACD() | 至少 35 根 K 线 | 趋势动能 |
| RSI | Klines.RSI(n) | n=6/12/24 | 超买超卖 |
| BOLL | Klines.BOLL(20) | 至少 20 根 K 线 | 布林线位置 |
| ATR | Klines.ATR(14) | 至少 15 根 K 线 | 波动率 |

## 4. 返回结构说明

### 4.1 顶层结构

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "code": "603063",
    "source": "tdx_kline_local_indicators",
    "periods": [],
    "limits": {
      "day": 250,
      "week": 156,
      "month": 120
    },
    "note": "技术指标由tdx K线在本地计算，仅返回日线、周线、月线最后一个有效指标值；available=false表示该周期K线数量不足。"
  }
}
```

### 4.2 periods 结构

`periods` 固定包含日线、周线、月线三个周期，正常情况下类似：

```json
[
  {
    "period": "day",
    "name": "日线",
    "klineCount": 250,
    "latestDate": "2026-06-24",
    "close": 50.15,
    "ma": {},
    "macd": {},
    "rsi": {},
    "boll": {},
    "atr": {},
    "signals": []
  },
  {
    "period": "week",
    "name": "周线"
  },
  {
    "period": "month",
    "name": "月线"
  }
]
```

## 5. 指标字段说明

### 5.1 MA

```json
"ma": {
  "ma5": {
    "available": true,
    "value": 50.826,
    "text": "MA5=50.826"
  },
  "ma120": {
    "available": false,
    "reason": "K线数量不足120根"
  }
}
```

说明：

- `available=true` 表示该指标可用。
- `value` 是指标值。
- `text` 是给 Agent 直接阅读的简短中文/符号说明。
- `available=false` 表示数据不足。
- `reason` 说明不可用原因。

### 5.2 MACD

```json
"macd": {
  "available": true,
  "dif": -0.57,
  "dea": 0.13,
  "hist": -1.4,
  "signal": "MACD柱为负，空头动能占优"
}
```

如果 K 线数量不足：

```json
"macd": {
  "available": false,
  "reason": "K线数量不足35根"
}
```

### 5.3 RSI

```json
"rsi": {
  "rsi6": {
    "available": true,
    "value": 54,
    "text": "RSI6=54"
  },
  "rsi12": {
    "available": true,
    "value": 39,
    "text": "RSI12=39"
  },
  "rsi24": {
    "available": true,
    "value": 37,
    "text": "RSI24=37"
  }
}
```

### 5.4 BOLL

```json
"boll": {
  "available": true,
  "upper": 60.386,
  "middle": 53.01,
  "lower": 45.634,
  "position": "价格位于布林线中轨下方"
}
```

如果 K 线数量不足：

```json
"boll": {
  "available": false,
  "reason": "K线数量不足20根"
}
```

### 5.5 ATR

```json
"atr": {
  "available": true,
  "atr14": 3.09,
  "usage": "衡量近期波动，不直接代表方向。"
}
```

如果 K 线数量不足：

```json
"atr": {
  "available": false,
  "reason": "K线数量不足15根"
}
```

## 6. 603063 调试样例摘要

最近一次实际调用 `603063` 的结果显示：

### 日线

```text
收盘价：50.15
MA20：53.010
MA60：47.145
MACD：DIF=-0.57, DEA=0.13, HIST=-1.4
RSI6：54
RSI12：39
RSI24：37
BOLL：上轨 60.386，中轨 53.01，下轨 45.634
ATR14：3.09
信号：
- 价格在MA20下方
- 价格在MA60上方
- MACD柱为负，空头动能占优
- 价格位于布林线中轨下方
```

### 周线

```text
收盘价：50.15
MA20：41.811
MA60：35.597
MACD：DIF=5.513, DEA=4.705, HIST=1.616
RSI6：28
RSI12：70
RSI24：63
BOLL：上轨 62.849，中轨 41.811，下轨 20.773
ATR14：7.24
信号：
- 价格在MA20上方
- 价格在MA60上方
- MACD柱为正，多头动能占优
- 价格位于布林线中轨上方
```

### 月线

```text
收盘价：50.15
K线数量：108
MA20：33.555
MA60：29.090
MA120：不可用，原因：K线数量不足120根
MACD：DIF=5.333, DEA=3.253, HIST=4.16
RSI6：74
RSI12：66
RSI24：67
BOLL：上轨 49.951，中轨 33.555，下轨 17.159
ATR14：7.955
信号：
- 价格在MA20上方
- 价格在MA60上方
- MACD柱为正，多头动能占优
- 价格位于布林线上轨附近或上方
```

## 7. 讨论过但尚未实现的聚合接口清单

下面这些是下一阶段要做的 Agent 聚合 API。

### 7.1 stock-brief

```text
GET /api/agent/stock-brief?code=603063
```

用途：个股快速画像。

建议组合：

- GetQuote
- GetFinanceInfo
- GetTdxHy
- GetTdxStat
- GetTdxStat2
- technical-summary

### 7.2 stock-deep

```text
GET /api/agent/stock-deep?code=603063&level=normal
```

用途：个股深度分析数据包。

建议组合：

- GetQuote
- technical-summary
- GetFinanceInfo
- GetCompanyCategory
- GetCompanyContent
- GetTdxHy
- GetBlockDataWithIndex
- GetTdxStat
- GetTdxStat2
- GetGbbq
- related-assets

### 7.3 company-profile

```text
GET /api/agent/company-profile?code=603063
```

用途：公司资料和基本面摘要。

建议组合：

- GetFinanceInfo
- GetCompanyCategory
- GetCompanyContent

### 7.4 hotspot-scan

```text
GET /api/agent/hotspot-scan?limit=100
```

用途：热点板块扫描和候选股筛选。

建议组合：

- GetTdxStat
- GetTdxStat2
- GetBlockDataWithIndex
- GetTdxHy
- 指数 K 线

### 7.5 watchlist

```text
GET /api/agent/watchlist?codes=603063,600519&level=brief
```

用途：交易日关注股监控。

建议组合：

- GetQuote
- GetMinute
- GetMinuteTrade
- technical-summary
- GetTdxStat2
- GetBlockDataWithIndex

### 7.6 auction

```text
GET /api/agent/auction?codes=603063,600519
```

用途：9:25 集合竞价分析。

建议组合：

- GetCallAuction
- GetQuote
- technical-summary
- GetBlockDataWithIndex

### 7.7 noon-review

```text
GET /api/agent/noon-review?codes=603063,600519
```

用途：上午盘收盘分析。

建议组合：

- GetQuote
- GetMinute
- GetMinuteTrade
- GetTdxStat
- GetTdxStat2
- GetBlockDataWithIndex
- technical-summary

### 7.8 close-review

```text
GET /api/agent/close-review?codes=603063,600519
```

用途：下午收盘分析。

建议组合：

- GetQuote
- GetMinute
- GetMinuteTradeAll
- GetTdxStat
- GetTdxStat2
- GetBlockDataWithIndex
- technical-summary

### 7.9 related-assets

```text
GET /api/agent/related-assets?code=603063
```

用途：查找个股相关港股、美股、指数、商品、汇率资产。

建议组合：

- SQLite 精选资产池
- ExQuote
- ExBars

### 7.10 market-baseline

```text
GET /api/agent/market-baseline?level=normal
```

用途：非交易日或开盘前的市场基准数据。

建议组合：

- A 股主要指数 K 线
- GetTdxStat
- GetTdxStat2
- GetBlockDataWithIndex
- ExQuote
- ExBars

## 8. 当前结论

目前已经实现的是：

```text
底层 tdx K 线方法
  -> 服务端限量、计算、补中文、补 available/reason
    -> /api/agent/technical-summary
```

还没有实现的是：

```text
行情 + 技术 + 财务 + F10 + 行业板块 + 外围资产
  -> 完整个股深度分析 Agent 聚合 API
```

下一步最合理的是实现：

```text
/api/agent/stock-brief
```

原因：

- 它比 `stock-deep` 小。
- 可以复用已经完成的 `technical-summary`。
- 能先验证“行情 + 技术 + 财务 + 行业”的聚合输出可读性。


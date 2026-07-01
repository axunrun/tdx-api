# Agent Stock Brief API 调试输出

- 调用时间：2026-06-24 18:50:07 +08:00
- 请求地址：`http://127.0.0.1:18080/api/agent/stock-brief?code=603063`
- 股票代码：`603063`
- 接口：`/api/agent/stock-brief`
- 说明：以下为接口原始 JSON 响应，未做字段删减，便于检查 Agent 可读性与上下文体积。

```json
{
    "code":  0,
    "message":  "success",
    "data":  {
                 "code":  "603063",
                 "source":  "tdx_agent_stock_brief",
                 "quote":  {
                               "code":  "603063",
                               "market":  "沪市",
                               "time":  "14995186",
                               "price":  50.15,
                               "lastClose":  48.71,
                               "open":  48.81,
                               "high":  50.95,
                               "low":  47.57,
                               "changePct":  2.956271812769447,
                               "volume":  222077,
                               "amount":  1100208768,
                               "insideDish":  97887,
                               "outerDisc":  124190,
                               "text":  "现价50.15，涨跌幅2.96%，成交额1100208768.00"
                           },
                 "finance":  {
                                 "updatedDate":  "2026-06-13",
                                 "ipoDate":  "2017-07-28",
                                 "totalShares":  463708515.625,
                                 "floatShares":  463708515.625,
                                 "totalAssets":  91759670000,
                                 "netAssets":  52199415000,
                                 "mainRevenue":  5737741250,
                                 "mainProfit":  3404650000,
                                 "operatingProfit":  511960976.5625,
                                 "netProfit":  511212968.75,
                                 "operatingCashflow":  -2089906093.75,
                                 "shareholders":  66784,
                                 "meaning":  "股本单位为股，资产、收入、利润和现金流单位为元；用于快速判断公司规模、盈利和现金流质量。"
                             },
                 "industry":  {
                                  "tdxHy":  "T0706",
                                  "swHy":  "X300502",
                                  "meaning":  "通达信行业和申万行业代码，用于后续关联行业指数、同行公司和板块景气度。"
                              },
                 "blocks":  [
                                {
                                    "type":  "concept",
                                    "typeName":  "概念板块",
                                    "name":  "粤港澳",
                                    "indexCode":  "880919",
                                    "memberCount":  361,
                                    "meaning":  "该股所属板块摘要，可用于后续抓取板块指数、板块成分股强弱和题材归因。"
                                },
                                {
                                    "type":  "concept",
                                    "typeName":  "概念板块",
                                    "name":  "光伏",
                                    "indexCode":  "880544",
                                    "memberCount":  400,
                                    "meaning":  "该股所属板块摘要，可用于后续抓取板块指数、板块成分股强弱和题材归因。"
                                },
                                {
                                    "type":  "concept",
                                    "typeName":  "概念板块",
                                    "name":  "风电",
                                    "indexCode":  "880582",
                                    "memberCount":  400,
                                    "meaning":  "该股所属板块摘要，可用于后续抓取板块指数、板块成分股强弱和题材归因。"
                                },
                                {
                                    "type":  "concept",
                                    "typeName":  "概念板块",
                                    "name":  "氢能源",
                                    "indexCode":  "880705",
                                    "memberCount":  400,
                                    "meaning":  "该股所属板块摘要，可用于后续抓取板块指数、板块成分股强弱和题材归因。"
                                },
                                {
                                    "type":  "concept",
                                    "typeName":  "概念板块",
                                    "name":  "智能电网",
                                    "indexCode":  "880520",
                                    "memberCount":  276,
                                    "meaning":  "该股所属板块摘要，可用于后续抓取板块指数、板块成分股强弱和题材归因。"
                                },
                                {
                                    "type":  "style_region",
                                    "typeName":  "风格/地域板块",
                                    "name":  "拟减持",
                                    "indexCode":  "880815",
                                    "memberCount":  400,
                                    "meaning":  "该股所属板块摘要，可用于后续抓取板块指数、板块成分股强弱和题材归因。"
                                },
                                {
                                    "type":  "style_region",
                                    "typeName":  "风格/地域板块",
                                    "name":  "高融资盘",
                                    "indexCode":  "880779",
                                    "memberCount":  80,
                                    "meaning":  "该股所属板块摘要，可用于后续抓取板块指数、板块成分股强弱和题材归因。"
                                },
                                {
                                    "type":  "style_region",
                                    "typeName":  "风格/地域板块",
                                    "name":  "保险新进",
                                    "indexCode":  "880782",
                                    "memberCount":  140,
                                    "meaning":  "该股所属板块摘要，可用于后续抓取板块指数、板块成分股强弱和题材归因。"
                                },
                                {
                                    "type":  "index",
                                    "typeName":  "指数板块",
                                    "name":  "上证380",
                                    "memberCount":  380,
                                    "meaning":  "该股所属板块摘要，可用于后续抓取板块指数、板块成分股强弱和题材归因。"
                                }
                            ],
                 "stat":  {
                              "date":  "20260623",
                              "peTtm":  47.37,
                              "peStatic":  42.5353,
                              "divYield":  0.26,
                              "changePct":  -2.97,
                              "trendDays":  -2,
                              "chg5":  -1.28,
                              "chg10":  -8.73,
                              "chg20":  -18.68,
                              "chg60":  49.88,
                              "chgYtd":  47.87,
                              "meaning":  "盘后个股综合统计，适合做估值、阶段涨跌幅和连续涨跌状态的快速判断。"
                          },
                 "moneyflow":  {
                                   "date":  "20260623",
                                   "blockIndex":  "880582",
                                   "amount":  99736.04,
                                   "amountPrev":  143218.09,
                                   "ipoPrice":  13.36,
                                   "high52w":  67,
                                   "low52w":  27.85,
                                   "amountMeaning":  "Amount和AmountPrev单位为万元，用于比较今日/昨日成交活跃度；BlockIndex为通达信板块指数代码。"
                               },
                 "technical":  {
                                   "code":  "603063",
                                   "source":  "tdx_kline_local_indicators",
                                   "periods":  [
                                                   {
                                                       "period":  "day",
                                                       "name":  "日线",
                                                       "klineCount":  250,
                                                       "latestDate":  "2026-06-24",
                                                       "close":  50.15,
                                                       "ma":  {
                                                                  "ma10":  {
                                                                               "available":  true,
                                                                               "value":  50.29,
                                                                               "text":  "MA10=50.290"
                                                                           },
                                                                  "ma120":  {
                                                                                "available":  true,
                                                                                "value":  39.414,
                                                                                "text":  "MA120=39.414"
                                                                            },
                                                                  "ma20":  {
                                                                               "available":  true,
                                                                               "value":  53.01,
                                                                               "text":  "MA20=53.010"
                                                                           },
                                                                  "ma5":  {
                                                                              "available":  true,
                                                                              "value":  50.826,
                                                                              "text":  "MA5=50.826"
                                                                          },
                                                                  "ma60":  {
                                                                               "available":  true,
                                                                               "value":  47.145,
                                                                               "text":  "MA60=47.145"
                                                                           }
                                                              },
                                                       "macd":  {
                                                                    "available":  true,
                                                                    "dif":  -0.57,
                                                                    "dea":  0.13,
                                                                    "hist":  -1.4,
                                                                    "signal":  "MACD柱为负，空头动能占优"
                                                                },
                                                       "rsi":  {
                                                                   "rsi12":  {
                                                                                 "available":  true,
                                                                                 "value":  39,
                                                                                 "text":  "RSI12=39"
                                                                             },
                                                                   "rsi24":  {
                                                                                 "available":  true,
                                                                                 "value":  37,
                                                                                 "text":  "RSI24=37"
                                                                             },
                                                                   "rsi6":  {
                                                                                "available":  true,
                                                                                "value":  54,
                                                                                "text":  "RSI6=54"
                                                                            }
                                                               },
                                                       "boll":  {
                                                                    "available":  true,
                                                                    "upper":  60.386,
                                                                    "middle":  53.01,
                                                                    "lower":  45.634,
                                                                    "position":  "价格位于布林线中轨下方"
                                                                },
                                                       "atr":  {
                                                                   "available":  true,
                                                                   "atr14":  3.09,
                                                                   "usage":  "衡量近期波动，不直接代表方向。"
                                                               },
                                                       "signals":  [
                                                                       "价格在MA20下方",
                                                                       "价格在MA60上方",
                                                                       "MACD柱为负，空头动能占优",
                                                                       "价格位于布林线中轨下方"
                                                                   ]
                                                   },
                                                   {
                                                       "period":  "week",
                                                       "name":  "周线",
                                                       "klineCount":  156,
                                                       "latestDate":  "2026-06-24",
                                                       "close":  50.15,
                                                       "ma":  {
                                                                  "ma10":  {
                                                                               "available":  true,
                                                                               "value":  51.268,
                                                                               "text":  "MA10=51.268"
                                                                           },
                                                                  "ma120":  {
                                                                                "available":  true,
                                                                                "value":  28.337,
                                                                                "text":  "MA120=28.337"
                                                                            },
                                                                  "ma20":  {
                                                                               "available":  true,
                                                                               "value":  41.811,
                                                                               "text":  "MA20=41.811"
                                                                           },
                                                                  "ma5":  {
                                                                              "available":  true,
                                                                              "value":  51.942,
                                                                              "text":  "MA5=51.942"
                                                                          },
                                                                  "ma60":  {
                                                                               "available":  true,
                                                                               "value":  35.597,
                                                                               "text":  "MA60=35.597"
                                                                           }
                                                              },
                                                       "macd":  {
                                                                    "available":  true,
                                                                    "dif":  5.513,
                                                                    "dea":  4.705,
                                                                    "hist":  1.616,
                                                                    "signal":  "MACD柱为正，多头动能占优"
                                                                },
                                                       "rsi":  {
                                                                   "rsi12":  {
                                                                                 "available":  true,
                                                                                 "value":  70,
                                                                                 "text":  "RSI12=70"
                                                                             },
                                                                   "rsi24":  {
                                                                                 "available":  true,
                                                                                 "value":  63,
                                                                                 "text":  "RSI24=63"
                                                                             },
                                                                   "rsi6":  {
                                                                                "available":  true,
                                                                                "value":  28,
                                                                                "text":  "RSI6=28"
                                                                            }
                                                               },
                                                       "boll":  {
                                                                    "available":  true,
                                                                    "upper":  62.849,
                                                                    "middle":  41.811,
                                                                    "lower":  20.773,
                                                                    "position":  "价格位于布林线中轨上方"
                                                                },
                                                       "atr":  {
                                                                   "available":  true,
                                                                   "atr14":  7.24,
                                                                   "usage":  "衡量近期波动，不直接代表方向。"
                                                               },
                                                       "signals":  [
                                                                       "价格在MA20上方",
                                                                       "价格在MA60上方",
                                                                       "MACD柱为正，多头动能占优",
                                                                       "价格位于布林线中轨上方"
                                                                   ]
                                                   },
                                                   {
                                                       "period":  "month",
                                                       "name":  "月线",
                                                       "klineCount":  108,
                                                       "latestDate":  "2026-06-24",
                                                       "close":  50.15,
                                                       "ma":  {
                                                                  "ma10":  {
                                                                               "available":  true,
                                                                               "value":  36.972,
                                                                               "text":  "MA10=36.972"
                                                                           },
                                                                  "ma120":  {
                                                                                "available":  false,
                                                                                "reason":  "K线数量不足120根"
                                                                            },
                                                                  "ma20":  {
                                                                               "available":  true,
                                                                               "value":  33.555,
                                                                               "text":  "MA20=33.555"
                                                                           },
                                                                  "ma5":  {
                                                                              "available":  true,
                                                                              "value":  42.218,
                                                                              "text":  "MA5=42.218"
                                                                          },
                                                                  "ma60":  {
                                                                               "available":  true,
                                                                               "value":  29.09,
                                                                               "text":  "MA60=29.090"
                                                                           }
                                                              },
                                                       "macd":  {
                                                                    "available":  true,
                                                                    "dif":  5.333,
                                                                    "dea":  3.253,
                                                                    "hist":  4.16,
                                                                    "signal":  "MACD柱为正，多头动能占优"
                                                                },
                                                       "rsi":  {
                                                                   "rsi12":  {
                                                                                 "available":  true,
                                                                                 "value":  66,
                                                                                 "text":  "RSI12=66"
                                                                             },
                                                                   "rsi24":  {
                                                                                 "available":  true,
                                                                                 "value":  67,
                                                                                 "text":  "RSI24=67"
                                                                             },
                                                                   "rsi6":  {
                                                                                "available":  true,
                                                                                "value":  74,
                                                                                "text":  "RSI6=74"
                                                                            }
                                                               },
                                                       "boll":  {
                                                                    "available":  true,
                                                                    "upper":  49.951,
                                                                    "middle":  33.555,
                                                                    "lower":  17.159,
                                                                    "position":  "价格位于布林线上轨附近或上方"
                                                                },
                                                       "atr":  {
                                                                   "available":  true,
                                                                   "atr14":  7.955,
                                                                   "usage":  "衡量近期波动，不直接代表方向。"
                                                               },
                                                       "signals":  [
                                                                       "价格在MA20上方",
                                                                       "价格在MA60上方",
                                                                       "MACD柱为正，多头动能占优",
                                                                       "价格位于布林线上轨附近或上方"
                                                                   ]
                                                   }
                                               ],
                                   "limits":  {
                                                  "day":  250,
                                                  "month":  120,
                                                  "week":  156
                                              },
                                   "note":  "技术指标由tdx K线在本地计算，仅返回日线、周线、月线最后一个有效指标值；available=false表示该周期K线数量不足。"
                               },
                 "limits":  {
                                "blocksPerType":  20,
                                "technicalDay":  250,
                                "technicalMonth":  120,
                                "technicalWeek":  156
                            },
                 "note":  "面向Agent的单股概览聚合接口；板块仅返回命中的板块摘要，不返回全部成分股；技术指标只返回各周期最新有效值。"
             }
}
```
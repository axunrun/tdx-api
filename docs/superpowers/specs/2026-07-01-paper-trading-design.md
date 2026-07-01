# 模拟交易与只读看板设计

日期: 2026-07-01

## 目标

为 `tdx-api` 新增一套模拟券商能力，让外部 Agent 可以通过 MCP 完成模拟 A股/ETF 交易。
服务端负责账户、订单、成交、费用、持仓、收益、快照和日志计算；Agent 只负责交易决策。

旧 WebUI 将被新的只读交易实验看板替换。WebUI 只展示状态，不提供人工交易操作。

## 范围

第一版实现“完整模拟交易闭环 + 服务端必要交易约束”。

包含:

- 独立模拟交易 SQLite 数据库。
- MCP 账户、下单、撤单、查询工具。
- 服务端订单撮合与账户计算。
- Agent 行为和账户变动审计。
- 只读 WebUI 看板。
- WebUI 专用 HTTP 聚合接口。

不包含:

- 真实券商接入。
- Agent 策略模块。
- WebUI 人工买卖按钮。
- 登录权限。
- WebSocket。
- 独立前端构建链。
- 完整交易所级别撮合队列。

## 总体架构

```text
Agent -> MCP 工具 -> 服务端 -> SQLite
WebUI -> HTTP API -> 服务端 -> SQLite
后台任务 -> 服务端行情方法 -> SQLite
```

核心模块:

| 模块 | 作用 |
|---|---|
| Paper Broker Core | 账户、资金、持仓、订单、成交、费用、收益计算 |
| Paper MCP Tools | 给 Agent 使用的模拟交易工具 |
| Paper Matcher | 后台撮合市价单、限价单和集合竞价单 |
| Paper HTTP API | 给 WebUI 使用的聚合查询接口 |
| Paper Readonly WebUI | 展示交易实验状态，不提供操作 |

WebUI 数据原则:

- WebUI 不直接读取 SQLite。
- WebUI 不直接调用 TDX 原始接口。
- WebUI 只调用服务端 paper HTTP 聚合接口。
- 服务端内部复用现有 tdx-api 行情、指数、K线、板块、global-market、brief 等方法。
- 前端只负责展示、筛选和图表渲染。

## 数据库

独立数据库:

```text
data/database/tdx-paper.sqlite
```

Docker 内路径:

```text
/app/data/database/tdx-paper.sqlite
```

环境变量:

```text
PAPER_DB_PATH
```

表结构:

| 表名 | 作用 |
|---|---|
| paper_accounts | 模拟账户主表 |
| paper_account_initial_positions | 创建账户时声明的初始持仓，锁定留痕 |
| paper_positions | 当前持仓 |
| paper_orders | 委托订单 |
| paper_trades | 成交记录 |
| paper_cash_ledger | 资金流水 |
| paper_position_ledger | 持仓变动流水 |
| paper_agent_actions | Agent 行为记录 |
| paper_account_snapshots | 账户资产快照 |
| paper_closed_positions | 清仓股票记录 |
| paper_closed_position_tracking | 清仓后股票继续涨跌跟踪 |

账户创建后锁定初始状态。需要重建时关闭旧账户并新建账户，保留历史。

## MCP 工具

第一版新增 4 个 MCP 工具，尽量通过参数区分功能。

| 工具 | 作用 |
|---|---|
| `tdx-paper-account` | 账户生命周期 |
| `tdx-paper-order` | 下单、撤单、查委托 |
| `tdx-paper-portfolio` | 查资金、持仓、成交、收益、清仓后表现 |
| `tdx-paper-rules` | 查询模拟交易规则说明 |

### `tdx-paper-account`

`action`:

| action | 作用 |
|---|---|
| `create` | 创建模拟账户 |
| `list` | 列出账户 |
| `get` | 获取账户摘要 |
| `close` | 销户归档 |
| `recreate` | 用户明确要求时，关闭旧账户并新建账户 |

参数:

| 参数 | 说明 |
|---|---|
| `action` | 必填 |
| `accountId` | 查询、关闭、重建时必填 |
| `name` | 创建账户名称 |
| `initialCash` | 初始现金 |
| `initialPositions` | 初始持仓数组 |
| `note` | 用户说明 |

`initialPositions` 每项:

| 参数 | 说明 |
|---|---|
| `code` | 股票或 ETF 代码 |
| `quantity` | 持仓数量 |
| `costPrice` | 成本价，可空；为空时用创建时行情价 |
| `buyDate` | 买入日期，可空；为空视为历史持仓，立即可卖 |

### `tdx-paper-order`

`action`:

| action | 作用 |
|---|---|
| `place` | 提交买卖委托 |
| `cancel` | 撤单 |
| `list` | 查询委托 |
| `get` | 查询单笔委托 |

参数:

| 参数 | 说明 |
|---|---|
| `accountId` | 必填 |
| `action` | 必填 |
| `code` | 下单时必填 |
| `side` | `buy` / `sell` |
| `orderType` | `market` / `limit` / `auction` |
| `price` | 限价单、集合竞价单需要 |
| `quantity` | 委托数量 |
| `timeInForce` | `day` / `auction_only` |
| `orderId` | 撤单、查询单笔委托时必填 |

实际成交价由服务端行情决定，Agent 不能指定实际成交价。

### `tdx-paper-portfolio`

`view`:

| view | 输出 |
|---|---|
| `summary` | 账户总览 |
| `cash` | 资金与流水 |
| `positions` | 当前持仓 |
| `trades` | 成交记录 |
| `orders` | 委托记录 |
| `performance` | 收益曲线、回撤、胜率等 |
| `closed_positions` | 清仓股票后续表现 |
| `actions` | Agent 行为记录 |

参数:

| 参数 | 说明 |
|---|---|
| `accountId` | 必填 |
| `view` | 必填 |
| `from` | 起始日期，可选 |
| `to` | 结束日期，可选 |
| `limit` | 返回条数 |
| `code` | 按证券代码过滤，可选 |

### `tdx-paper-rules`

返回当前模拟交易规则:

- 支持 A股股票和 A股 ETF。
- 现金买入，持仓卖出。
- 不支持真实交易、融资融券、做空。
- 费用规则。
- T+1 规则。
- 集合竞价规则。
- 市价单、限价单、撤单规则。
- WebUI 只读。

## 订单撮合

订单类型:

| 类型 | 处理方式 |
|---|---|
| `market` | 获取当前实时行情，按当前价立即成交；行情不可用则拒单 |
| `limit` | 进入挂单队列，后台定时撮合 |
| `auction` | 集合竞价单，9:25 按参考开盘价撮合 |

市价买入:

- 获取当前价。
- 校验现金。
- 按服务端行情价成交。
- 扣除成交金额和费用。
- 增加持仓。
- 股票当日买入默认不可卖。

市价卖出:

- 获取当前价。
- 校验可卖数量。
- 按服务端行情价成交。
- 扣除费用。
- 增加现金。
- 减少持仓。
- 清仓时写入清仓跟踪表。

限价买入:

- 提交时冻结最大所需资金。
- 当前价小于等于买入限价时成交。
- 成交后释放多冻结资金。
- 当日未成交，收盘后自动撤单并释放冻结资金。

限价卖出:

- 提交时冻结可卖持仓。
- 当前价大于等于卖出限价时成交。
- 当日未成交，收盘后自动撤单并释放冻结持仓。

集合竞价:

| 时间 | 规则 |
|---|---|
| 09:15-09:20 | 可提交，可撤单 |
| 09:20-09:25 | 可提交，不可撤单 |
| 09:25 | 用 TDX 开盘价或集合竞价参考价撮合 |
| 09:25 后未成交 | `day` 进入连续竞价队列；`auction_only` 自动撤单 |

成交判断:

- 买单委托价大于等于参考价时成交。
- 卖单委托价小于等于参考价时成交。
- 市价集合竞价单按参考价成交。
- 参考价不可用时不成交，并按 `timeInForce` 处理。

## 交易约束与费用

| 规则 | 第一版处理 |
|---|---|
| A股股票 T+1 | 当日买入不可卖 |
| ETF | 支持；费用和 T+0/T+1 规则按证券类型处理 |
| 买入数量 | 股票按 100 股整数倍 |
| 卖出数量 | 不超过可卖数量 |
| 停牌/无行情 | 拒单 |
| 现金不足 | 拒单 |
| 持仓不足 | 拒单 |
| 撤单 | 已成交不可撤；09:20-09:25 集合竞价单不可撤 |

费用固定写死:

- 佣金: 万 1，全佣。
- 印花税: 按当前国家标准，卖出股票收取。
- 过户费: 按当前 A股标准。
- ETF: 不收印花税。

实现前需要再次核对当前官方费用标准。

## HTTP API

WebUI 只调用 HTTP API。

| 接口 | 作用 |
|---|---|
| `GET /api/paper/dashboard` | 看板总览 |
| `GET /api/paper/accounts` | 账户列表 |
| `GET /api/paper/account` | 单账户详情 |
| `GET /api/paper/activity` | Agent 行为、委托、成交事件 |
| `GET /api/paper/closed-positions` | 清仓后表现 |

### `/api/paper/dashboard`

参数:

| 参数 | 说明 |
|---|---|
| `accountId` | 可选；为空返回默认账户或全部账户摘要 |
| `range` | `today` / `20d` / `60d` / `all` |

返回:

- 市场状态。
- 指数概览。
- 市场广度。
- 热点、中游、弱势板块。
- 当前账户卡片。
- 多账户收益曲线。
- 当前持仓摘要。
- 未完成订单摘要。
- 最近成交摘要。
- 最近 Agent 行为摘要。

### `/api/paper/accounts`

返回所有账户状态、初始资金、当前总资产、累计收益、今日收益和最大回撤。

### `/api/paper/account`

参数:

| 参数 | 说明 |
|---|---|
| `accountId` | 必填 |
| `range` | `today` / `20d` / `60d` / `all` |

返回账户资金、当前持仓、收益曲线、订单、成交、资金流水摘要和持仓流水摘要。

### `/api/paper/activity`

参数:

| 参数 | 说明 |
|---|---|
| `accountId` | 可选 |
| `type` | `agent` / `order` / `trade` / `reject` / `snapshot` |
| `limit` | 默认 100 |

### `/api/paper/closed-positions`

参数:

| 参数 | 说明 |
|---|---|
| `accountId` | 必填 |
| `range` | `20d` / `60d` / `all` |

## 后台任务

| 任务 | 频率 | 作用 |
|---|---:|---|
| 挂单撮合 | 30 秒 | 检查未完成订单，满足条件则成交 |
| 账户快照 | 5 分钟 + 成交后 | 生成资产曲线数据 |
| 清仓后跟踪 | 5 分钟 | 更新已清仓股票后续表现 |
| 收盘清理 | 收盘后 | 撤销当日未成交订单，释放冻结资金/持仓 |

任务失败只记录错误，不中断主服务。非交易时段不做高频行情刷新。

## WebUI

新 WebUI 是只读交易实验监控台。

页面结构:

```text
顶部状态栏
├─ 当前市场状态
├─ 更新时间
├─ 当前账户选择
└─ 全局收益概览

主区域
├─ A股市场概览
├─ 账户资产曲线
├─ 多账户收益对比
├─ 当前持仓
├─ 委托与成交
├─ 清仓后表现
└─ Agent 行为时间线
```

前端文件:

```text
cmd/server/static/index.html
cmd/server/static/styles.css
cmd/server/static/app.js
```

视觉方向:

- 深色专业监控台。
- 高密度信息，不做传统券商软件式拥挤表格。
- 图表使用原生 Canvas/SVG。
- 不引入 React、Vite、ECharts。
- 不做装饰性渐变球或营销型 Hero。

## 验收标准

- Agent 可以通过 MCP 创建模拟账户。
- Agent 可以通过 MCP 买入、卖出、撤单。
- 服务端自动计算现金、冻结资金、持仓、成本、费用、盈亏。
- 服务端记录每一次 Agent 行为和账户变化。
- 服务端生成账户资产曲线。
- 清仓股票继续跟踪后续涨跌。
- WebUI 只读展示账户运行状态。
- WebUI 不直接读 SQLite。
- WebUI 不直接调用 TDX 原始接口。
- Docker 部署后数据可持久化。
- 现有 agent API 和 MCP 工具不被破坏。

## 实现顺序

1. 替换旧 WebUI。
2. 新增 paper SQLite 层。
3. 新增 paper broker service。
4. 新增下单、撤单和撮合。
5. 新增 MCP 工具。
6. 新增 WebUI HTTP API。
7. 重做只读 WebUI。
8. 补充测试与样例输出。

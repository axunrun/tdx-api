#!/usr/bin/env python3
"""
A股技术指标采集模块

数据源（已验证联通）：
  - 腾讯行情 qt.gtimg.cn → 实时报价
  - 腾讯K线 ifzq.gtimg.cn → 日K线（主）
  - BaoStock → 日K线+估值（备选）

依赖：标准库 + baostock（pip已安装）
"""

import urllib.request
import json
import statistics
from typing import Optional, Tuple


# ──────────────────────────────────────────────
# 市场前缀检测
# ──────────────────────────────────────────────

def _detect_market_prefix(code: str) -> str:
    '''根据股票代码自动判断市场前缀
    返回: sh | sz | bj | hk | us_'''
    code = code.strip()
    for pfx in ['sh', 'sz', 'bj', 'hk', 'us_']:
        if code.startswith(pfx):
            return pfx
    if code.isdigit():
        if code.startswith(('920',)):          # 北交所 920xxx
            return 'bj'
        elif code.startswith(('4', '8')):      # 北交所 4xxxxx, 8xxxxx
            return 'bj'
        elif code.startswith(('6', '9')):      # 600 上证 / 900 上海B股
            return 'sh'
        elif code.startswith(('0', '3', '2')): # 000/002/300 深证
            return 'sz'
        elif len(code) == 5:                   # 5位纯数字 → 港股
            return 'hk'
        else:
            return 'sh'
    else:
        return 'us_'


def _full_code(code: str, prefix: str = None) -> str:
    '''将裸代码转为带前缀的全格式'''
    code = code.strip()
    if prefix is None:
        prefix = _detect_market_prefix(code)
    for pfx in ['sh', 'sz', 'bj', 'hk', 'us_']:
        if code.startswith(pfx):
            return code
    return f'{prefix}{code}'


# ──────────────────────────────────────────────
# 数据采集层
# ──────────────────────────────────────────────

def _fetch_tencent_quote(code: str) -> Optional[dict]:
    """从腾讯行情获取实时报价（88字段），自动适配市场前缀"""
    full = _full_code(code)
    url = f"http://qt.gtimg.cn/q={full}"
    req = urllib.request.Request(url, headers={"User-Agent": "Mozilla/5.0"})
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            text = resp.read().decode("gbk")
    except Exception:
        return None

    parts = text.split("~")
    if len(parts) < 47:
        return None

    def safe_float(s, default=0.0):
        try:
            return float(s)
        except (ValueError, TypeError):
            return default

    market = _detect_market_prefix(code)

    if market == 'hk':
        # 港股字段略有不同
        return {
            "name": parts[1] if len(parts) > 1 else "",
            "current_price": safe_float(parts[3]),
            "prev_close": safe_float(parts[4]),
            "open_price": safe_float(parts[5]),
            "volume_hand": int(safe_float(parts[6])),
            "outer_disc": 0,
            "inner_disc": 0,
            "change_pct": safe_float(parts[32]),
            "high": safe_float(parts[33]),
            "low": safe_float(parts[34]),
            "turnover_rate": 0.0,
            "pe_ttm": safe_float(parts[39]),
            "amplitude": safe_float(parts[43]),
            "circ_mcap": safe_float(parts[44]),
            "total_mcap": safe_float(parts[45]),
            "pb": 0.0,
        }

    return {
        "name": parts[1] if len(parts) > 1 else "",
        "current_price": safe_float(parts[3]),
        "prev_close": safe_float(parts[4]),
        "open_price": safe_float(parts[5]),
        "volume_hand": int(safe_float(parts[6])),  # 手
        "outer_disc": int(safe_float(parts[7])),    # 外盘（主动买）
        "inner_disc": int(safe_float(parts[8])),    # 内盘（主动卖）
        "change_pct": safe_float(parts[32]),
        "high": safe_float(parts[33]),
        "low": safe_float(parts[34]),
        "turnover_rate": safe_float(parts[38]),      # %
        "pe_ttm": safe_float(parts[39]),
        "amplitude": safe_float(parts[43]),          # %
        "circ_mcap": safe_float(parts[44]),          # 亿
        "total_mcap": safe_float(parts[45]),         # 亿
        "pb": safe_float(parts[46]),
    }


def _fetch_tencent_kline(code: str, days: int = 120) -> Optional[list]:
    """从腾讯K线获取前复权日K线，自动适配市场前缀"""
    full = _full_code(code)
    market = _detect_market_prefix(code)
    if market == 'us_':
        return None  # 美股不支持腾讯K线
    url = f"http://web.ifzq.gtimg.cn/appstock/app/fqkline/get?param={full},day,,,{days},qfq"
    req = urllib.request.Request(url, headers={"User-Agent": "Mozilla/5.0"})
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            data = json.loads(resp.read().decode("utf-8"))
    except Exception:
        return None

    kline = data.get("data", {}).get(full, {})
    rows = kline.get("qfqday") or kline.get("day")
    if not rows or len(rows) < 1:
        return None

    result = []
    for r in rows:
        try:
            row = {
                "date": r[0],
                "open": float(r[1]),
                "close": float(r[2]),
                "high": float(r[3]),
                "low": float(r[4]),
                "volume": float(r[5]),
            }
            result.append(row)
        except (IndexError, ValueError):
            continue
    return result if len(result) >= 1 else None


def _fetch_baostock_kline(code: str, days: int = 120) -> Optional[list]:
    """从BaoStock获取K线+估值作为备选（仅A股 sh/sz，不支持北交所）"""
    market = _detect_market_prefix(code)
    if market not in ('sh', 'sz'):
        return None
    import baostock as bs
    bs.login()
    try:
        rs = bs.query_history_k_data_plus(
            f"{market}.{code.lstrip('sh').lstrip('sz')}",
            "date,open,high,low,close,volume,peTTM,pbMRQ",
            frequency="d", adjustflag="3"
        )
        rows = []
        while rs.error_code == "0" and rs.next():
            row = rs.get_row_data()
            if row[0] != "":
                rows.append({
                    "date": row[0],
                    "open": float(row[1]),
                    "high": float(row[2]),
                    "low": float(row[3]),
                    "close": float(row[4]),
                    "volume": float(row[5]),
                    "pe_ttm": float(row[6]) if row[6] else 0.0,
                    "pb": float(row[7]) if row[7] else 0.0,
                })
        if len(rows) > days:
            rows = rows[-days:]
    finally:
        bs.logout()

    return rows if len(rows) >= 5 else None


def _fetch_kline_with_fallback(code: str) -> Optional[list]:
    """获取K线，腾讯主→BaoStock备选"""
    kline = _fetch_tencent_kline(code)
    if kline:
        return kline
    bs_data = _fetch_baostock_kline(code)
    if bs_data:
        return bs_data
    return None


# ──────────────────────────────────────────────
# 衍生指标计算层
# ──────────────────────────────────────────────

def calc_ma(closes: list, n: int) -> Optional[float]:
    if len(closes) < n:
        return None
    return sum(closes[-n:]) / n


def calc_macd(closes: list):
    """返回 (dif, dea, macd_bar, signal_str)"""
    if len(closes) < 26:
        return None, None, None, "数据不足"

    def ema(data, n):
        result = [data[0]]
        k = 2 / (n + 1)
        for i in range(1, len(data)):
            result.append(data[i] * k + result[-1] * (1 - k))
        return result

    ema_fast = ema(closes, 12)
    ema_slow = ema(closes, 26)
    dif_vals = [f - s for f, s in zip(ema_fast[-len(ema_slow):], ema_slow)]
    dea_vals = ema(dif_vals, 9)

    dif = round(dif_vals[-1], 2)
    dea = round(dea_vals[-1], 2)
    bar = round(2 * (dif - dea), 2)

    # 信号判断
    above_zero = dif > 0
    cross_up = dif_vals[-1] > dea_vals[-1] and (len(dif_vals) < 2 or dif_vals[-2] <= dea_vals[-2])
    cross_down = dif_vals[-1] < dea_vals[-1] and (len(dif_vals) < 2 or dif_vals[-2] >= dea_vals[-2])

    if cross_up:
        signal = "金叉零轴上" if above_zero else "金叉零轴下"
    elif cross_down:
        signal = "死叉零轴上" if above_zero else "死叉零轴下"
    else:
        signal = f"{'零轴上' if above_zero else '零轴下'}粘合"

    return dif, dea, bar, signal


def calc_rsi(closes: list, n: int) -> Optional[float]:
    if len(closes) < n + 1:
        return None
    gains = sum(max(closes[i] - closes[i - 1], 0) for i in range(-n, 0))
    losses = sum(max(closes[i - 1] - closes[i], 0) for i in range(-n, 0))
    if losses == 0:
        return 100.0 if gains > 0 else 50.0
    rs = gains / losses
    return round(100 - 100 / (1 + rs), 2)


def calc_boll(closes: list, n: int = 20, k: float = 2.0):
    """返回 (上轨, 中轨, 下轨)"""
    if len(closes) < n:
        return None, None, None
    segment = closes[-n:]
    mid = sum(segment) / n
    std_dev = statistics.stdev(segment)
    upper = round(mid + k * std_dev, 2)
    lower = round(mid - k * std_dev, 2)
    return upper, round(mid, 2), lower


def calc_kdj(highs: list, lows: list, closes: list, n: int = 9):
    """返回 (k, d, j, signal_str)"""
    if len(highs) < n or len(lows) < n or len(closes) < n:
        return None, None, None, "数据不足"

    h9 = max(highs[-n:])
    l9 = min(lows[-n:])
    rsv = (closes[-1] - l9) / (h9 - l9) * 100 if h9 != l9 else 50
    k = round(2 / 3 * 50 + 1 / 3 * rsv, 2)
    d = round(2 / 3 * 50 + 1 / 3 * k, 2)
    j = round(3 * k - 2 * d, 2)

    # 简化信号：K在D上方/下方
    signal = "K上穿D" if k > d else "K下穿D"
    return k, d, j, signal


def calc_bias(close: float, ma_n: Optional[float]) -> Optional[float]:
    if ma_n is None or ma_n == 0:
        return None
    return round((close / ma_n - 1) * 100, 2)


# ──────────────────────────────────────────────
# 评分层
# ──────────────────────────────────────────────

def _score_ma(closes: list) -> tuple:
    ma5 = calc_ma(closes, 5)
    ma10 = calc_ma(closes, 10)
    ma20 = calc_ma(closes, 20)
    ma60 = calc_ma(closes, 60)

    signal = "数据不足"
    score = 0

    if all(v is not None for v in [ma5, ma10, ma20, ma60]):
        if ma5 > ma10 > ma20 > ma60:
            signal = "多头排列"
            score = 3
        elif ma5 < ma10 < ma20 < ma60:
            signal = "空头排列"
            score = -3
        else:
            signal = "交叉粘合"
            score = 0

    return {
        "ma5": round(ma5, 2) if ma5 else None,
        "ma10": round(ma10, 2) if ma10 else None,
        "ma20": round(ma20, 2) if ma20 else None,
        "ma60": round(ma60, 2) if ma60 else None,
        "signal": signal,
        "score": score,
    }, score


def _score_macd(closes: list) -> tuple:
    dif, dea, bar, signal = calc_macd(closes)
    if dif is None:
        return {"signal": "数据不足", "score": 0}, 0

    above_zero = dif > 0
    is_golden = "金叉" in signal  # 金叉=+2
    is_death = "死叉" in signal   # 死叉=-2

    if is_golden and above_zero:
        score = 2
    elif is_golden and not above_zero:
        score = 1
    elif is_death and not above_zero:
        score = -2
    elif is_death and above_zero:
        score = -1
    else:
        score = 0

    return {
        "dif": dif,
        "dea": dea,
        "bar": bar,
        "signal": signal,
        "score": score,
    }, score


def _score_rsi(closes: list) -> tuple:
    rsi6 = calc_rsi(closes, 6)
    rsi12 = calc_rsi(closes, 12)
    rsi24 = calc_rsi(closes, 24)

    score = 0
    if rsi6 is not None:
        # 超卖回升 → +2；超买回落 → -2
        # 简化：看最新值
        if rsi6 < 20:
            signal = "超卖"
            score = 2
        elif rsi6 > 80:
            signal = "超买"
            score = -2
        else:
            signal = "正常"
            score = 0
    else:
        signal = "数据不足"

    return {
        "rsi6": rsi6, "rsi12": rsi12, "rsi24": rsi24,
        "signal": signal, "score": score,
    }, score


def _score_boll(closes: list, kline: list) -> tuple:
    upper, mid, lower = calc_boll(closes)
    if upper is None:
        return {"signal": "数据不足", "score": 0}, 0

    current = closes[-1]
    if current <= lower:
        signal = "下轨支撑"
        score = 1
    elif current >= upper:
        signal = "上轨压力"
        score = -1
    else:
        signal = "中轨附近"
        score = 0

    return {
        "upper": upper, "middle": mid, "lower": lower,
        "signal": signal, "score": score,
    }, score


def _score_kdj(highs: list, lows: list, closes: list) -> tuple:
    k, d, j, signal = calc_kdj(highs, lows, closes)
    if k is None:
        return {"signal": "数据不足", "score": 0}, 0

    score = 1 if k > d else -1

    return {
        "k": k, "d": d, "j": j,
        "signal": signal, "score": score,
    }, score


def _score_bias(closes: list) -> tuple:
    if len(closes) < 10:
        return {"signal": "数据不足", "score": 0}, 0

    ma5 = calc_ma(closes, 5)
    ma10 = calc_ma(closes, 10)
    b5 = calc_bias(closes[-1], ma5)
    b10 = calc_bias(closes[-1], ma10)

    score = 0
    # 负乖离过大（超跌）→ +1；正乖离过大（超涨）→ -1
    if b5 is not None and b5 is not None:
        if b5 < -5 and b10 < -8:
            signal = "负乖离过大(超跌)"
            score = 1
        elif b5 > 5 and b10 > 8:
            signal = "正乖离过大(超涨)"
            score = -1
        else:
            signal = "正常"
            score = 0
    else:
        signal = "数据不足"

    return {
        "bias5": b5, "bias10": b10,
        "signal": signal, "score": score,
    }, score


def _score_volume(quote: dict, kline: list) -> tuple:
    """量能评分"""
    if not quote or not kline:
        return {"signal": "数据不足", "score": 0}, 0

    vol_today = quote.get("volume_hand", 0)
    vols = [k["volume"] for k in kline[-5:]]
    vol_avg_5 = sum(vols) / len(vols) if vols else 1

    ratio = vol_today / vol_avg_5 if vol_avg_5 > 0 else 1.0
    change_pct = quote.get("change_pct", 0)

    if ratio > 1.5 and change_pct > 0:
        signal = "放量上涨"
        score = 1
    elif ratio > 1.5 and change_pct < 0:
        signal = "放量下跌"
        score = -1
    elif ratio < 0.7:
        signal = "缩量"
        score = 0
    else:
        signal = "平量"
        score = 0

    return {
        "volume_today": vol_today,
        "volume_avg_5": round(vol_avg_5, 0),
        "volume_ratio": round(ratio, 2),
        "signal": signal,
        "score": score,
    }, score


def _score_bull_bear(quote: dict) -> tuple:
    """多空量能比评分"""
    outer = quote.get("outer_disc", 0)
    inner = quote.get("inner_disc", 0)
    if inner == 0:
        return {"ratio": 0, "signal": "无内盘数据", "score": 0}, 0

    ratio = round(outer / inner, 2)

    if ratio > 1.2:
        signal = "多头强势"
        score = 1
    elif ratio < 0.8:
        signal = "空头强势"
        score = -1
    else:
        signal = "均衡"
        score = 0

    return {"ratio": ratio, "signal": signal, "score": score}, score


# ──────────────────────────────────────────────
# 主入口
# ──────────────────────────────────────────────

def get_technical_indicators(code: str, name: str = "") -> dict:
    """
    获取个股全部技术指标

    参数：
        code: 股票代码，如 "601869" / "hk00700" / "bj920394"
        name: 股票名称（可选），为空时自动查询

    返回：
        完整结构化dict，含14项指标和评分
    """
    result = {
        "stock_code": code,
        "stock_name": name or code,
        "date": "",
        "data_source": "",
        "quote": {},
        "indicators": {},
        "total_score": 0,
        "score_range": "±12",
        "unavailable_indicators": ["资金流向", "筹码分布"],
        "errors": [],
    }

    market = _detect_market_prefix(code)

    # 美股暂不支持（腾讯API无美股行情）
    if market == 'us_':
        result["errors"].append("暂不支持美股行情（腾讯API无美股数据）")
        return result

    # 1. 获取实时报价
    quote = _fetch_tencent_quote(code)
    if quote and quote.get("name"):
        result["stock_name"] = quote["name"]

    if not quote:
        result["errors"].append("腾讯行情获取失败，尝试BaoStock备选")
        # 尝试从BaoStock获取PE/PB
        bs_data = _fetch_baostock_kline(code, 1)
        if bs_data and len(bs_data) > 0:
            last = bs_data[-1]
            quote = {
                "current_price": last["close"],
                "volume_hand": int(last["volume"]),
                "turnover_rate": 0.0,
                "pe_ttm": last.get("pe_ttm", 0),
                "pb": last.get("pb", 0),
                "amplitude": 0.0,
                "total_mcap": 0.0,
                "circ_mcap": 0.0,
                "change_pct": 0.0,
                "outer_disc": 0,
                "inner_disc": 0,
            }
        else:
            result["errors"].append("所有行情源均不可用")
            return result

    result["quote"] = {
        "current_price": quote["current_price"],
        "change_pct": quote["change_pct"],
        "turnover_rate": quote["turnover_rate"],
        "amplitude": quote["amplitude"],
        "pe_ttm": quote["pe_ttm"],
        "pb": quote["pb"],
        "total_mcap": quote["total_mcap"],
        "circ_mcap": quote["circ_mcap"],
    }

    # 2. 获取日K线
    kline = _fetch_kline_with_fallback(code)
    if not kline:
        result["errors"].append("所有K线源均不可用，无法计算衍生指标")
        return result

    result["date"] = kline[-1]["date"]
    # 判断数据源：K线≥5行来自腾讯主源，否则来自备选
    if len(kline) >= 5:
        result["data_source"] = "腾讯行情 + 腾讯K线"
    elif any(k.get("pe_ttm") for k in kline):
        result["data_source"] = "腾讯行情 + BaoStock(备选)"
    else:
        result["data_source"] = "腾讯行情 + 腾讯K线(仅当日)"

    # 提取序列
    closes = [k["close"] for k in kline]
    highs = [k["high"] for k in kline]
    lows = [k["low"] for k in kline]

    # 3. 计算各指标
    indicators = {}
    total_score = 0

    ind, s = _score_ma(closes)
    indicators["ma"] = ind
    total_score += s

    ind, s = _score_macd(closes)
    indicators["macd"] = ind
    total_score += s

    ind, s = _score_rsi(closes)
    indicators["rsi"] = ind
    total_score += s

    ind, s = _score_boll(closes, kline)
    indicators["boll"] = ind
    total_score += s

    ind, s = _score_kdj(highs, lows, closes)
    indicators["kdj"] = ind
    total_score += s

    ind, s = _score_bias(closes)
    indicators["bias"] = ind
    total_score += s

    ind, s = _score_volume(quote, kline)
    indicators["volume"] = ind
    total_score += s

    ind, s = _score_bull_bear(quote)
    indicators["bull_bear"] = ind
    total_score += s

    result["indicators"] = indicators
    result["total_score"] = total_score
    return result


def _fmt(v) -> str:
    """格式化显示值，None→'—'"""
    if v is None:
        return "—"
    if isinstance(v, float):
        return f"{v:.2f}"
    return str(v)


def format_report(result: dict) -> str:
    """将技术指标结果格式化为Markdown报告"""
    s = result["stock_name"]
    c = result["stock_code"]

    if result.get("error"):
        return f"**{s}（{c}）**\n\n❌ {result['error']}"

    lines = [
        f"### 1️⃣ 个股技术面",
        f"",
        f"**{s}（{c}）** | **数据日期**：{result['date']} | **数据源**：{result['data_source']}",
        f"",
        f"| 指标 | 当前值 | 信号 | 评分 |",
        f"|------|--------|:----:|:----:|",
    ]

    q = result.get("quote", {})
    if q:
        cur = q.get('current_price', '—')
        chg = q.get('change_pct', 0) or 0
        lines.append(f"| 最新价 | {cur} | {chg:+.2f}% | — |")

        tr = q.get('turnover_rate', '—')
        amp = q.get('amplitude', '—')
        lines.append(f"| 换手率 | {tr}% | 振幅 {amp}% | — |")

        pe = q.get('pe_ttm', '—')
        pb = q.get('pb', '—')
        lines.append(f"| PE(TTM) | {pe} | PB {pb} | — |")

        tm = q.get('total_mcap', '—')
        cm = q.get('circ_mcap', '—')
        lines.append(f"| 总市值 | {tm}亿 | 流通 {cm}亿 | — |")

    ind = result.get("indicators", {})

    v = ind.get("volume", {})
    vol = v.get('volume_today', 0)
    lines.append(f"| 成交量 | {vol:,.0f}手 | {v.get('signal', '—')} | {v.get('score', 0):+d} |")

    m = ind.get("ma", {})
    vals = " / ".join(_fmt(m.get(k)) for k in ["ma5","ma10","ma20","ma60"])
    lines.append(f"| MA排列 | {vals} | {m.get('signal', '—')} | {m.get('score', 0):+d} |")

    mac = ind.get("macd", {})
    d1 = _fmt(mac.get('dif'))
    d2 = _fmt(mac.get('dea'))
    d3 = _fmt(mac.get('bar'))
    lines.append(f"| MACD | DIF:{d1} DEA:{d2} 柱:{d3} | {mac.get('signal','—')} | {mac.get('score',0):+d} |")

    r = ind.get("rsi", {})
    rsi_vals = f"{_fmt(r.get('rsi6'))}/{_fmt(r.get('rsi12'))}/{_fmt(r.get('rsi24'))}"
    lines.append(f"| RSI | {rsi_vals} | {r.get('signal','—')} | {r.get('score',0):+d} |")

    b = ind.get("boll", {})
    boll_vals = f"上{b.get('upper','—')} 中{b.get('middle','—')} 下{b.get('lower','—')}"
    lines.append(f"| BOLL | {boll_vals} | {b.get('signal','—')} | {b.get('score',0):+d} |")

    kd = ind.get("kdj", {})
    kdj_vals = f"K:{_fmt(kd.get('k'))} D:{_fmt(kd.get('d'))} J:{_fmt(kd.get('j'))}"
    lines.append(f"| KDJ | {kdj_vals} | {kd.get('signal','—')} | {kd.get('score',0):+d} |")

    bi = ind.get("bias", {})
    bias_vals = f"{_fmt(bi.get('bias5'))}/{_fmt(bi.get('bias10'))}"
    lines.append(f"| BIAS | {bias_vals} | {bi.get('signal','—')} | {bi.get('score',0):+d} |")

    bb = ind.get("bull_bear", {})
    lines.append(f"| 多空量能比 | {bb.get('ratio','—')} | {bb.get('signal','—')} | {bb.get('score',0):+d} |")

    lines.append(f"| **总分** | | | **{result['total_score']:+d}/{result['score_range']}** |")
    lines.append("")
    lines.append(f"> ❌ 不可用指标：{'、'.join(result['unavailable_indicators'])}")
    if result["errors"]:
        lines.append(f"> ⚠️ 告警：{'；'.join(result['errors'])}")

    return "\n".join(lines)


# ──────────────────────────────────────────────
# 命令行入口
# ──────────────────────────────────────────────

if __name__ == "__main__":
    import sys
    codes = sys.argv[1:] if len(sys.argv) > 1 else ["601869"]
    for code in codes:
        print(f"\n{'='*60}")
        print(f"获取 {code} 技术指标...")
        result = get_technical_indicators(code)
        print(format_report(result))
        print(f"\n完整数据:")
        import json
        # 精简输出（忽略函数引用）
        print(json.dumps(result, ensure_ascii=False, default=str, indent=2)[:1000])

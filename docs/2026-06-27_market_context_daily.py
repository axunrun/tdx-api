#!/usr/bin/env python3
"""
每日市场概况采集 · 盘前/盘后更新

输出: /workspace/agent/data/YYYY-MM-DD_市场概况.json
所有 Agent 分析时先读此文件，避免重复抓取大盘数据。

数据源: 腾讯行情(主) + TickFlow(备选) + anysearch(新闻)
"""

import sys
import json
import urllib.request
from datetime import datetime, date
from pathlib import Path

OUTPUT_DIR = Path("/workspace/agent/data")

# ══════════════════════════════════════════════════════════════
# 配置
# ══════════════════════════════════════════════════════════════
INDICES = {
    "sh000001": "上证指数",
    "sz399001": "深证成指",
    "sz399006": "创业板指",
    "sh000688": "科创50",
    "sh000016": "上证50",
    "sh000300": "沪深300",
}

TICKFLOW_TOKEN = "tk_5cf41651c32d45b3b8a28787d13e3434"

# ══════════════════════════════════════════════════════════════
# 工具
# ══════════════════════════════════════════════════════════════
def _req(url: str, timeout: int = 10):
    try:
        r = urllib.request.Request(url, headers={"User-Agent": "Mozilla/5.0"})
        resp = urllib.request.urlopen(r, timeout=timeout)
        return resp.read().decode("utf-8", errors="ignore")
    except Exception:
        return None

def _req_gbk(url: str, timeout: int = 10):
    try:
        r = urllib.request.Request(url, headers={"User-Agent": "Mozilla/5.0"})
        resp = urllib.request.urlopen(r, timeout=timeout)
        return resp.read().decode("gbk", errors="ignore")
    except Exception:
        return None

# ══════════════════════════════════════════════════════════════
# D1: 主要指数（腾讯行情）
# ══════════════════════════════════════════════════════════════
def fetch_indices_tencent() -> list:
    """腾讯批量获取指数行情"""
    codes = ",".join(INDICES.keys())
    raw = _req_gbk(f"http://qt.gtimg.cn/q={codes}", timeout=10)
    if not raw:
        return []
    results = []
    for line in raw.strip().split("\n"):
        if "=" not in line:
            continue
        fields = line.split("~")
        code_key = fields[0].split("_")[-1] if "_" in fields[0] else ""
        name = INDICES.get(code_key, fields[1] if len(fields) > 1 else "")
        results.append({
            "code": code_key,
            "name": name,
            "price": float(fields[3]) if len(fields) > 3 and fields[3] else None,
            "change_pct": f"{fields[32]}%" if len(fields) > 32 else None,
            "open": float(fields[5]) if len(fields) > 5 and fields[5] else None,
            "high": float(fields[33]) if len(fields) > 33 and fields[33] else None,
            "low": float(fields[34]) if len(fields) > 34 and fields[34] else None,
            "volume": int(fields[6]) if len(fields) > 6 and fields[6] else 0,
            "amount": float(fields[37]) if len(fields) > 37 and fields[37] else None,
        })
    return results

# ══════════════════════════════════════════════════════════════
# D2: TickFlow 备选（腾讯断连时）
# ══════════════════════════════════════════════════════════════
def fetch_indices_tickflow() -> list:
    """TickFlow备选获取指数（仅支持部分指数）"""
    results = []
    index_symbols = [
        ("000001.SH", "上证指数"), ("399001.SZ", "深证成指"),
        ("399006.SZ", "创业板指"), ("000688.SH", "科创50"),
        ("000016.SH", "上证50"), ("000300.SH", "沪深300"),
    ]
    for sym, name in index_symbols:
        url = f"https://api.tickflow.org/v1/klines?symbol={sym}&count=1&api_key={TICKFLOW_TOKEN}"
        raw = _req(url, timeout=10)
        if not raw:
            continue
        try:
            d = json.loads(raw).get("data", {})
            ts = d.get("timestamp", [])
            if not ts:
                continue
            i = -1  # 最新一条
            results.append({
                "code": sym.split(".")[0],
                "name": name,
                "price": d["close"][i] if d.get("close") else None,
                "open": d["open"][i] if d.get("open") else None,
                "high": d["high"][i] if d.get("high") else None,
                "low": d["low"][i] if d.get("low") else None,
                "change_pct": "N/A(非实时)",
                "source": "TickFlow",
            })
        except (json.JSONDecodeError, KeyError, IndexError):
            continue
    return results

# ══════════════════════════════════════════════════════════════
# D3: 持仓股当日速览
# ══════════════════════════════════════════════════════════════
def fetch_portfolio_quotes() -> list:
    """获取持仓股实时报价"""
    codes = ["sh603063", "sh600933"]
    raw = _req_gbk(f"http://qt.gtimg.cn/q={','.join(codes)}", timeout=10)
    if not raw:
        return []
    results = []
    for line in raw.strip().split("\n"):
        if "=" not in line:
            continue
        fields = line.split("~")
        if len(fields) < 4:
            continue
        results.append({
            "name": fields[1],
            "code": fields[2],
            "price": float(fields[3]) if fields[3] else None,
            "change_pct": f"{fields[32]}%" if len(fields) > 32 else None,
            "volume": int(fields[6]) if fields[6] else 0,
            "turnover_rate": f"{fields[38]}%" if len(fields) > 38 else None,
        })
    return results

# ══════════════════════════════════════════════════════════════
# 主流程
# ══════════════════════════════════════════════════════════════
def collect():
    today = date.today().isoformat()
    now = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    is_weekend = date.today().weekday() >= 5

    print(f"采集市场概况: {today}", file=sys.stderr)

    # 指数（腾讯主 → TickFlow备选）
    indices = fetch_indices_tencent()
    source = "腾讯行情"
    if not indices:
        print(" 腾讯不可用, 切换TickFlow...", file=sys.stderr)
        indices = fetch_indices_tickflow()
        source = "TickFlow"

    # 持仓股
    portfolio = fetch_portfolio_quotes()

    # 构建输出
    output = {
        "date": today,
        "collect_time": now,
        "is_weekend": is_weekend,
        "data_source": source,
        "indices": indices,
        "portfolio": portfolio,
        "summary": _generate_summary(indices, portfolio),
    }

    # 写入文件
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)
    path = OUTPUT_DIR / f"{today}_市场概况.json"
    with open(path, "w", encoding="utf-8") as f:
        json.dump(output, f, ensure_ascii=False, indent=2)

    print(f"已写入: {path}", file=sys.stderr)
    print(json.dumps({"status": "ok", "path": str(path), "data_source": source}, ensure_ascii=False))

def _generate_summary(indices: list, portfolio: list) -> str:
    """生成一句话市场摘要"""
    if not indices:
        return "市场数据未获取到"
    sh = next((i for i in indices if "上证" in i.get("name", "")), None)
    cy = next((i for i in indices if "创业板" in i.get("name", "")), None)
    parts = []
    if sh and sh.get("change_pct"):
        parts.append(f"上证{sh['change_pct']}")
    if cy and cy.get("change_pct"):
        parts.append(f"创业板{cy['change_pct']}")
    if portfolio:
        names = [p.get("name", "") for p in portfolio if p.get("change_pct")]
        if names:
            chgs = [f"{p['name']}{p['change_pct']}" for p in portfolio if p.get("change_pct")]
            parts.append("持仓: " + " ".join(chgs))
    return " | ".join(parts) if parts else "无数据"

if __name__ == "__main__":
    collect()

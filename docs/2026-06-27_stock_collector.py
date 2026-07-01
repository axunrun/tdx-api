#!/usr/bin/env python3
"""
股市数据采集器 · 多源聚合
输出格式化数据，供 cron 任务分析使用

用法:
  python3 stock_collector.py [时段]

时段: open | midday | close | holiday
"""

import sys
import json
import re
import urllib.request
import urllib.error
from datetime import datetime, timedelta

# ============================================================
# 配置
# ============================================================
PORTFOLIO_PATH = "/workspace/knowledge/portfolio.yaml"
TUSHARE_TOKEN = "6e90d4534d1459c364488d0c1ca898d24246077204242c1adc667933"

# 新浪财经代码映射
SINA_CODES = {
    "sh000001": "sh000001",   # 上证指数
    "sz399001": "sz399001",   # 深证成指
    "hkHSI": "hkHSI",         # 恒生指数
    "us_.DJI": "us_.DJI",     # 道琼斯
}

# ============================================================
# 新浪财经直调（零依赖）
# ============================================================
def sina_quote(codes):
    """获取新浪财经实时行情"""
    if isinstance(codes, str):
        codes = [codes]
    code_str = ",".join(codes)
    url = f"https://hq.sinajs.cn/list={code_str}"
    req = urllib.request.Request(url, headers={"Referer": "https://finance.sina.com.cn"})
    try:
        resp = urllib.request.urlopen(req, timeout=10)
        text = resp.read().decode("gbk")
        results = {}
        for line in text.strip().split("\n"):
            if not line:
                continue
            match = re.search(r'var hq_str_(\w+)="(.*)"', line)
            if not match:
                continue
            code = match.group(1)
            fields = match.group(2).split(",")
            if code.startswith("sh") or code.startswith("sz"):
                results[code] = {
                    "name": fields[0],
                    "open": fields[1],
                    "prev_close": fields[2],
                    "current": fields[3],
                    "high": fields[4],
                    "low": fields[5],
                    "volume": fields[8],
                    "amount": fields[9],
                    "date": fields[30],
                    "time": fields[31],
                }
            elif code.startswith("hk"):
                results[code] = {
                    "name": fields[1],
                    "current": fields[2],
                    "high": fields[3],
                    "low": fields[4],
                    "open": fields[5],
                    "volume": fields[7],
                    "time": fields[9],
                }
            elif code.startswith("us"):
                results[code] = {
                    "name": fields[0],
                    "current": fields[1],
                    "prev_close": fields[2],
                    "open": fields[3],
                    "high": fields[5],
                    "low": fields[6],
                    "time": fields[17],
                }
        return results
    except Exception as e:
        return {"error": str(e)}


# ============================================================
# AKShare 数据
# ============================================================
def akshare_data(code, market="A股"):
    """使用 AKShare 获取详细数据"""
    try:
        import akshare as ak
        
        if market == "A股":
            # 获取个股实时行情
            df = ak.stock_zh_a_spot_em()
            stock = df[df["代码"] == code]
            if not stock.empty:
                s = stock.iloc[0]
                return {
                    "name": s.get("名称", ""),
                    "current": float(s.get("最新价", 0)),
                    "change_pct": float(s.get("涨跌幅", 0)),
                    "volume": float(s.get("成交量", 0)),
                    "amount": float(s.get("成交额", 0)),
                    "turnover": float(s.get("换手率", 0)),
                    "pe": float(s.get("市盈率-动态", 0)),
                    "total_mv": float(s.get("总市值", 0)),
                    "circulating_mv": float(s.get("流通市值", 0)),
                }
            return {"error": "未找到该股票"}
        else:
            return {"error": f"暂不支持 {market} 的 AKShare 查询"}
    except ImportError:
        return {"error": "akshare 未安装"}
    except Exception as e:
        return {"error": str(e)}


def akshare_concept():
    """获取概念板块涨跌幅"""
    try:
        import akshare as ak
        df = ak.stock_board_concept_name_em()
        top = df.nsmallest(5, "涨跌幅")  # 涨幅前5
        bottom = df.nlargest(5, "涨跌幅")  # 跌幅前5
        return {
            "top_gainers": [
                {"name": r["板块名称"], "change": float(r["涨跌幅"])}
                for _, r in top.iterrows()
            ],
            "top_losers": [
                {"name": r["板块名称"], "change": float(r["涨跌幅"])}
                for _, r in bottom.iterrows()
            ],
        }
    except Exception as e:
        return {"error": str(e)}


def akshare_moneyflow(code):
    """个股资金流向"""
    try:
        import akshare as ak
        df = ak.stock_individual_fund_flow(stock=code, market="sh")
        if not df.empty:
            latest = df.iloc[-1]
            return {
                "date": str(latest.get("日期", "")),
                "net_inflow": float(latest.get("主力净流入-净额", 0)),
                "net_inflow_pct": float(latest.get("主力净流入-净占比", 0)),
                "super_large_inflow": float(latest.get("超大单净流入-净额", 0)),
                "large_inflow": float(latest.get("大单净流入-净额", 0)),
            }
        return {"error": "无资金流向数据"}
    except Exception as e:
        return {"error": str(e)}


# ============================================================
# Tushare 数据
# ============================================================
def tushare_data(code, fields="basic"):
    """使用 Tushare Pro 获取数据"""
    try:
        import tushare as ts
        ts.set_token(TUSHARE_TOKEN)
        pro = ts.pro_api()
        
        if fields == "daily":
            df = pro.daily(ts_code=code, start_date=(datetime.now() - timedelta(days=5)).strftime("%Y%m%d"))
            if not df.empty:
                d = df.iloc[0]
                return {
                    "date": d.get("trade_date", ""),
                    "open": float(d.get("open", 0)),
                    "high": float(d.get("high", 0)),
                    "low": float(d.get("low", 0)),
                    "close": float(d.get("close", 0)),
                    "change_pct": float(d.get("pct_chg", 0)),
                    "volume": float(d.get("vol", 0)),
                    "amount": float(d.get("amount", 0)),
                }
        
        if fields == "concept":
            df = pro.concept(ts_code=code)
            if not df.empty:
                return [{"name": r["concept_name"], "id": r["concept_id"]} for _, r in df.iterrows()]
        
        if fields == "holder":
            df = pro.top10_floatholders(ts_code=code)
            if not df.empty:
                return [
                    {"name": r["holder_name"], "ratio": float(r.get("hold_ratio", 0))}
                    for _, r in df.head(5).iterrows()
                ]
        
        return {"error": f"无 {fields} 数据或接口需更高积分"}
    except ImportError:
        return {"error": "tushare 未安装"}
    except Exception as e:
        return {"error": str(e)}


# ============================================================
# 新闻采集
# ============================================================
def fetch_financial_news():
    """采集财经新闻"""
    news = []
    sources = [
        ("财联社", "https://www.cls.cn/telegraph"),
    ]
    for name, url in sources:
        try:
            req = urllib.request.Request(url, headers={"User-Agent": "Mozilla/5.0"})
            resp = urllib.request.urlopen(req, timeout=10)
            text = resp.read().decode("utf-8", errors="ignore")
            news.append({"source": name, "status": "fetched", "length": len(text)})
        except Exception as e:
            news.append({"source": name, "status": "error", "error": str(e)})
    return news


# ============================================================
# 主入口
# ============================================================
def main():
    session = sys.argv[1] if len(sys.argv) > 1 else "open"

    output = {
        "session": session,
        "timestamp": datetime.now().strftime("%Y-%m-%d %H:%M:%S"),
        "is_trading_day": datetime.now().weekday() < 5,
    }

    # 1. 大盘实时行情
    output["benchmarks"] = sina_quote(["sh000001", "sz399001"])

    # 2. 持仓股行情（从 portfolio.yaml 读，但这里先硬编码demo）
    # 实际运行时由 cron prompt 传入持仓代码
    output["message"] = "请通过 cron prompt 传入持仓代码列表进行查询"

    # 3. 概念板块
    output["concepts"] = akshare_concept()

    # 4. 新闻
    output["news"] = fetch_financial_news()

    # 输出 JSON
    print(json.dumps(output, ensure_ascii=False, indent=2, default=str))


if __name__ == "__main__":
    main()

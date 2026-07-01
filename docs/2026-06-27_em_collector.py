#!/usr/bin/env python3
"""
东方财富专线采集器 · em_collector.py

作用: 直接从东方财富 API 获取补充数据（无 AKShare 依赖）
维度: D2 基本面 F10 + D3 股东结构 + D4 概念/行业板块 + D8 公告 + 全市场行情 + 资金流向

用法:
  python3 em_collector.py 603063          → 单股全量采集
  python3 em_collector.py 603063 --spot   → 全市场行情 + 概念板块排名

输出: JSON 到 stdout
"""

import sys
import json
import re
import urllib.request
import time
from datetime import datetime, date
from typing import Optional


# ══════════════════════════════════════════════════════════════
# HTTP 工具
# ══════════════════════════════════════════════════════════════

def _req(url: str, timeout: int = 15) -> Optional[str]:
    try:
        r = urllib.request.Request(url, headers={'User-Agent': 'Mozilla/5.0'})
        resp = urllib.request.urlopen(r, timeout=timeout)
        return resp.read().decode('utf-8', errors='ignore')
    except Exception:
        return None

def _req_gbk(url: str, timeout: int = 15) -> Optional[str]:
    try:
        r = urllib.request.Request(url, headers={'User-Agent': 'Mozilla/5.0'})
        resp = urllib.request.urlopen(r, timeout=timeout)
        return resp.read().decode('gbk', errors='ignore')
    except Exception:
        return None


# ══════════════════════════════════════════════════════════════
# 市场前缀检测
# ══════════════════════════════════════════════════════════════

def _detect_prefix(code: str) -> str:
    code = code.strip()
    if code.startswith(('sh', 'sz', 'bj', 'hk', 'us_')):
        return code[:2] if len(code) > 2 and code[2] == '_' else code[:2]
    c = code[:3]
    if c.startswith('6'):
        return 'sh'
    if c.startswith(('0', '3')):
        return 'sz'
    if c.startswith(('4', '8', '920')):
        return 'bj'
    return 'sh'

def _strip_prefix(code: str) -> str:
    for p in ['sh', 'sz', 'bj', 'hk', 'us_']:
        if code.startswith(p):
            return code[len(p):]
    return code

def _market_flag(code: str) -> int:
    p = _detect_prefix(code)
    return 1 if p in ('sh', 'bj') else 0

def _em_secid(code: str) -> str:
    p = _detect_prefix(code)
    raw = _strip_prefix(code)
    m = 1 if p in ('sh', 'bj') else 0
    return f"{m}.{raw}"


# ══════════════════════════════════════════════════════════════
# D2: 东方财富 datacenter — F10 基础数据
# ══════════════════════════════════════════════════════════════

def fetch_em_f10(code: str, market: str = 'SH') -> dict:
    """东方财富 datacenter F10 公司概况（EPS/ROE/营收/净利/同比）"""
    secucode = f'{_strip_prefix(code)}.{market}'
    url = ('https://datacenter.eastmoney.com/securities/api/data/v1/get'
           f'?reportName=RPT_LICO_FN_CPD&columns=ALL'
           f'&filter=(SECUCODE=%22{secucode}%22)&pageNumber=1&pageSize=1'
           f'&sortTypes=-1&sortColumns=NOTICE_DATE')
    raw = _req(url, timeout=15)
    if not raw:
        return {'error': '东方F10无响应'}
    try:
        data = json.loads(raw)
        if not data.get('result') or not data['result'].get('data'):
            return {'error': '东方F10无数据'}
        d = data['result']['data'][0]
        return {
            'industry': d.get('PUBLISHNAME', ''),
            'trade_market': d.get('TRADE_MARKET', ''),
            'basic_eps': d.get('BASIC_EPS'),
            'weight_roe': d.get('WEIGHTAVG_ROE'),
            'total_revenue': d.get('TOTAL_OPERATE_INCOME'),
            'net_profit': d.get('PARENT_NETPROFIT'),
            'revenue_yoy': d.get('YSTZ'),
            'profit_yoy': d.get('SJLTZ'),
            'bps': d.get('BPS'),
            'report_date': str(d.get('REPORTDATE', '')),
            'notice_date': str(d.get('NOTICE_DATE', '')),
        }
    except (json.JSONDecodeError, KeyError) as e:
        return {'error': f'东方F10解析失败: {e}'}


# ══════════════════════════════════════════════════════════════
# D3: 东方财富 datacenter — 前十大股东
# ══════════════════════════════════════════════════════════════

def _classify_holder(name: str) -> str:
    if '社保' in name or '基本养老' in name:
        return '社保基金'
    if '保险' in name:
        return '保险'
    if '香港中央结算' in name:
        return '北向资金'
    if '基金' in name or 'ETF' in name or '证券投资' in name:
        return '公募基金'
    if '有限合伙' in name or '投资' in name:
        return '投资机构'
    if name in ['郑大鹏', '肖安波', '盛小军', '石玉庆', '周党生']:
        return '个人/高管'
    return '其他'

def fetch_em_holders(code: str, market: str = 'SH') -> list:
    """前十大流通股东（含社保/机构识别）"""
    secucode = f'{_strip_prefix(code)}.{market}'
    url = ('https://datacenter.eastmoney.com/securities/api/data/v1/get'
           f'?reportName=RPT_F10_EH_HOLDERS&columns=ALL'
           f'&filter=(SECUCODE=%22{secucode}%22)&pageNumber=1&pageSize=20'
           f'&sortTypes=-1&sortColumns=END_DATE')
    raw = _req(url, timeout=15)
    if not raw:
        return []
    try:
        data = json.loads(raw)
        if not data.get('result') or not data['result'].get('data'):
            return []
        holders = []
        for item in data['result']['data']:
            holders.append({
                'end_date': str(item.get('END_DATE', ''))[:10],
                'holder_name': item.get('HOLDER_NAME', ''),
                'hold_num': item.get('HOLD_NUM'),
                'hold_ratio': item.get('HOLD_RATIO'),
                'hold_change': item.get('HOLD_CHANGE'),
                'holder_type': _classify_holder(item.get('HOLDER_NAME', '')),
            })
        return holders
    except json.JSONDecodeError:
        return []


# ══════════════════════════════════════════════════════════════
# D4: 东方财富 push2 — 概念/行业板块排名
# ══════════════════════════════════════════════════════════════

def fetch_em_concept_board(top_n: int = 30) -> list:
    """概念板块涨幅排名（替代 AKShare stock_board_concept_name_em）"""
    url = "https://79.push2.eastmoney.com/api/qt/clist/get"
    params = (
        "?pn=1&pz={}&po=1&np=1"
        "&ut=bd1d9ddb04089700cf9c27f6f7426281"
        "&fltt=2&invt=2&fid=f3"
        "&fs=m:90+t:3+f:!50"
        "&fields=f2,f3,f4,f8,f12,f14,f15,f16,f17,f18,f20,f21,f24,f25,f22,f33,f104,f105,f136"
    ).format(top_n)
    raw = _req(url + params, timeout=15)
    if not raw:
        return []
    try:
        data = json.loads(raw)
        items = data.get('data', {}).get('diff', [])
        results = []
        for item in items:
            results.append({
                'code': item.get('f12'),
                'name': item.get('f14'),
                'change_pct': item.get('f3'),
                'price': item.get('f2'),
                'total_mv': item.get('f20'),
                'turnover_rate': item.get('f8'),
                'up_count': item.get('f104'),
                'down_count': item.get('f105'),
                'lead_stock': item.get('f136'),
            })
        return results
    except (json.JSONDecodeError, KeyError):
        return []


def fetch_em_industry_board(top_n: int = 30) -> list:
    """行业板块涨幅排名（替代 AKShare stock_board_industry_name_em）"""
    url = "https://79.push2.eastmoney.com/api/qt/clist/get"
    params = (
        "?pn=1&pz={}&po=1&np=1"
        "&ut=bd1d9ddb04089700cf9c27f6f7426281"
        "&fltt=2&invt=2&fid=f3"
        "&fs=m:90+t:2+f:!50"
        "&fields=f2,f3,f4,f8,f12,f14,f15,f16,f17,f18,f20,f21,f24,f25,f22,f33,f104,f105,f136"
    ).format(top_n)
    raw = _req(url + params, timeout=15)
    if not raw:
        return []
    try:
        data = json.loads(raw)
        items = data.get('data', {}).get('diff', [])
        results = []
        for item in items:
            results.append({
                'code': item.get('f12'),
                'name': item.get('f14'),
                'change_pct': item.get('f3'),
                'price': item.get('f2'),
                'total_mv': item.get('f20'),
                'turnover_rate': item.get('f8'),
            })
        return results
    except (json.JSONDecodeError, KeyError):
        return []


# ══════════════════════════════════════════════════════════════
# D8: 东方财富 — 近期公告
# ══════════════════════════════════════════════════════════════

def fetch_em_announcements(code: str, top_n: int = 15) -> list:
    """获取近期公告，标注重大合同/订单"""
    raw_code = _strip_prefix(code)
    url = ('https://np-anotice-stock.eastmoney.com/api/security/ann'
           f'?sr=-1&page_size={top_n}&page_index=1&ann_type=A'
           f'&stock_list={raw_code}&f_node=0&s_node=0')
    raw = _req(url, timeout=15)
    if not raw:
        return []
    try:
        data = json.loads(raw)
        items = data.get('data', {}).get('list', [])
        results = []
        for item in items:
            title = item.get('title', '')
            date_str = str(item.get('notice_date', ''))[:10]
            keywords = ['合同', '中标', '订单', '协议', '战略合作', '担保', '增发',
                        '股权激励', '回购', '分红', '业绩', '预增', '投资']
            matched = [kw for kw in keywords if kw in title]
            results.append({
                'date': date_str,
                'title': title,
                'keywords': matched,
                'is_important': '合同' in title or '中标' in title or '订单' in title,
            })
        return results
    except (json.JSONDecodeError, KeyError):
        return []


# ══════════════════════════════════════════════════════════════
# 全市场行情（东方财富 push2）
# ══════════════════════════════════════════════════════════════

def fetch_em_spot_market(top_n: int = 30, sort_by: str = "f3") -> list:
    """全市场 A 股实时行情（替代 AKShare stock_zh_a_spot_em）
    
    sort_by: f3=涨跌幅, f62=主力净流入, f20=总市值, f12=代码
    返回 top_n 条按 sort_by 降序排列
    """
    url = "https://82.push2.eastmoney.com/api/qt/clist/get"
    params = (
        "?pn=1&pz={}&po=1&np=1"
        "&ut=bd1d9ddb04089700cf9c27f6f7426281"
        "&fltt=2&invt=2&fid={}"
        "&fs=m:0+t:6+f:!2,m:0+t:80+f:!2,m:1+t:2+f:!2,m:1+t:23+f:!2"
        "&fields=f2,f3,f4,f8,f12,f14,f15,f16,f17,f18,f20,f21,f24,f25,f62,f115,f152"
    ).format(top_n, sort_by)
    raw = _req(url + params, timeout=15)
    if not raw:
        return []
    try:
        data = json.loads(raw)
        items = data.get('data', {}).get('diff', [])
        results = []
        for item in items:
            results.append({
                'code': item.get('f12'),
                'name': item.get('f14'),
                'price': item.get('f2'),
                'change_pct': item.get('f3'),
                'change_amount': item.get('f4'),
                'volume': item.get('f15'),
                'amount': item.get('f16'),
                'turnover_rate': item.get('f8'),
                'pe': item.get('f17'),
                'pb': item.get('f152'),
                'total_mv': item.get('f20'),
                'circ_mv': item.get('f21'),
                'amplitude': item.get('f18'),
                'main_force_net': item.get('f62'),
            })
        return results
    except (json.JSONDecodeError, KeyError):
        return []


# ══════════════════════════════════════════════════════════════
# 个股资金流向（东方财富 push2his）
# ══════════════════════════════════════════════════════════════

def fetch_em_moneyflow(code: str, days: int = 5) -> list:
    """个股逐日资金流向（替代 AKShare stock_individual_fund_flow）
    
    返回包含主力/超大单/大单/中单/小单净流入
    
    ⚠️ 注意: push2his.eastmoney.com 在部分服务器 IP 被反爬封锁,
    如返回空列表说明当前网络环境不支持直调, 可考虑:
    - 通过 tdx-api 扩展此功能
    - 更换网络环境/代理
    """
    secid = _em_secid(code)
    url = "https://push2his.eastmoney.com/api/qt/stock/fflow/daykline/get"
    params = (
        f"?secid={secid}&klt=101"
        "&fields1=f1,f2,f3,f7"
        "&fields2=f51,f52,f53,f54,f55,f56,f57"
        f"&lmt={days}"
    )
    raw = _req(url + params, timeout=15)
    if not raw:
        return []
    try:
        data = json.loads(raw)
        klines = data.get('data', {}).get('klines', [])
        if not klines:
            return []
        results = []
        for line in klines:
            p = line.split(',')
            if len(p) < 7:
                continue
            results.append({
                'date': p[0],
                'main_net': float(p[1]) if p[1] else 0,
                'small_net': float(p[2]) if p[2] else 0,
                'mid_net': float(p[3]) if p[3] else 0,
                'big_net': float(p[4]) if p[4] else 0,
                'super_big_net': float(p[5]) if p[5] else 0,
                'main_pct': float(p[6]) if p[6] else 0,
            })
        return results
    except (json.JSONDecodeError, KeyError, ValueError):
        return []


# ══════════════════════════════════════════════════════════════
# 单股补充采集
# ══════════════════════════════════════════════════════════════

def collect_stock_supplement(code: str) -> dict:
    """单股东方财富补充数据（不含行情，行情走 tdx-api）"""
    raw_code = _strip_prefix(code)
    market = 'SH' if _detect_prefix(code) in ('sh', 'bj') else 'SZ'

    result = {
        'stock_code': raw_code,
        'collect_time': datetime.now().strftime('%Y-%m-%d %H:%M:%S'),
        'data_sources': [],
    }

    # D2: F10 公司概况
    print(f'  [1/4] 采集F10公司概况...', file=sys.stderr)
    result['f10'] = fetch_em_f10(raw_code, market)
    if 'error' not in result['f10']:
        result['data_sources'].append('东方财富datacenter(F10)')

    # D3: 前十大股东
    print(f'  [2/4] 采集前十大股东...', file=sys.stderr)
    result['holders'] = fetch_em_holders(raw_code, market)
    if result['holders']:
        result['data_sources'].append('东方财富datacenter(股东)')
        type_count = {}
        for h in result['holders']:
            t = h.get('holder_type', '其他')
            type_count[t] = type_count.get(t, 0) + 1
        result['holder_summary'] = type_count

    # D8: 公告
    print(f'  [3/4] 采集近期公告...', file=sys.stderr)
    result['announcements'] = fetch_em_announcements(raw_code)
    if result['announcements']:
        result['data_sources'].append('东方财富公告')
        important = [a for a in result['announcements'] if a.get('is_important')]
        result['important_contracts'] = important

    # 资金流向
    print(f'  [4/4] 采集资金流向...', file=sys.stderr)
    result['moneyflow'] = fetch_em_moneyflow(raw_code)
    if result['moneyflow']:
        result['data_sources'].append('东方财富资金流向')

    result['data_source_count'] = len(set(result['data_sources']))
    return result


# ══════════════════════════════════════════════════════════════
# 主入口
# ══════════════════════════════════════════════════════════════

if __name__ == '__main__':
    if len(sys.argv) < 2:
        print('用法: python3 em_collector.py <股票代码> [--spot]', file=sys.stderr)
        sys.exit(1)

    code = sys.argv[1]
    include_spot = '--spot' in sys.argv

    print(f'em_collector · 东方财富专线采集', file=sys.stderr)
    print(f'股票: {code}', file=sys.stderr)

    output = {}

    # 全市场/板块数据（需要 --spot 标志）
    if include_spot:
        print(f'  [选项] 采集概念板块排名...', file=sys.stderr)
        output['concept_board'] = fetch_em_concept_board()
        print(f'  [选项] 采集行业板块排名...', file=sys.stderr)
        output['industry_board'] = fetch_em_industry_board()
        print(f'  [选项] 采集全市场行情...', file=sys.stderr)
        output['spot_market'] = fetch_em_spot_market()

    # 单股补充数据
    output['stock'] = collect_stock_supplement(code)

    print(json.dumps(output, ensure_ascii=False, indent=2, default=str))
    print(f'✅ 采集完成', file=sys.stderr)

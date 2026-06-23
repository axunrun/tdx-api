package main

import (
	"net/http"
	"strconv"
	"strings"
	"time"
	"github.com/injoyai/tdx"
	"github.com/injoyai/tdx/protocol"
)

// ====== 行情 ======

func handleQuote(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" { jsonErr(w, "缺少code"); return }
	c := cli()
	if c == nil { jsonErr(w, "未连接"); return }
	q, err := c.GetQuote(code)
	if err != nil || len(q) == 0 { jsonErr(w, "无数据"); return }
	qt := q[0]; k := qt.Kline
	jsonResp(w, map[string]interface{}{
		"code": qt.Code,
		"price": k.Close.Float64(), "lastClose": k.Last.Float64(),
		"open": k.Open.Float64(), "high": k.High.Float64(), "low": k.Low.Float64(),
		"volume": k.Volume, "amount": k.Amount.Float64(),
		"time": qt.ServerTime,
	})
}

func handleKline(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	typ := r.URL.Query().Get("type")
	if code == "" { jsonErr(w, "缺少code"); return }
	cnt := parseCount(r.URL.Query().Get("count"), 10)
	c := cli()
	if c == nil { jsonErr(w, "未连接"); return }
	list, err := fetchKline(c, code, typ, cnt, false)
	if err != nil { jsonErr(w, err.Error()); return }
	jsonResp(w, map[string]interface{}{"code": code, "type": typ, "count": len(list), "list": list})
}

func handleKlineAll(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	typ := r.URL.Query().Get("type")
	if code == "" { jsonErr(w, "缺少code"); return }
	c := cli()
	if c == nil { jsonErr(w, "未连接"); return }
	list, err := fetchKlineAll(c, code, typ)
	if err != nil { jsonErr(w, err.Error()); return }
	jsonResp(w, map[string]interface{}{"code": code, "type": typ, "count": len(list), "list": list})
}

func handleQfqKline(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" { jsonErr(w, "缺少code"); return }
	gb := getGbbq()
	if gb == nil { jsonErr(w, "复权模块未就绪"); return }
	ks, err := gb.QFQKlineDay(code)
	if err != nil || len(ks) == 0 { jsonErr(w, "无数据"); return }
	list := toKlineList(ks)
	cnt := parseCount(r.URL.Query().Get("count"), len(list))
	if cnt > 0 && cnt < len(list) { list = list[len(list)-cnt:] }
	jsonResp(w, map[string]interface{}{"code": code, "type": "qfq_day", "count": len(list), "list": list})
}

func handleHfqKline(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" { jsonErr(w, "缺少code"); return }
	gb := getGbbq()
	if gb == nil { jsonErr(w, "复权模块未就绪"); return }
	ks, err := gb.HFQKlineDay(code)
	if err != nil || len(ks) == 0 { jsonErr(w, "无数据"); return }
	list := toKlineList(ks)
	cnt := parseCount(r.URL.Query().Get("count"), len(list))
	if cnt > 0 && cnt < len(list) { list = list[len(list)-cnt:] }
	jsonResp(w, map[string]interface{}{"code": code, "type": "hfq_day", "count": len(list), "list": list})
}

func handleMinute(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" { jsonErr(w, "缺少code"); return }
	date := r.URL.Query().Get("date")
	if date == "" {
		// 默认今天
		date = time.Now().Format("20060102")
	}
	c := cli()
	if c == nil { jsonErr(w, "未连接"); return }
	resp, err := c.GetHistoryMinute(date, code)
	if err != nil || resp == nil { jsonErr(w, "无数据"); return }
	type MinuteItem struct {
		Time   string  `json:"time"`
		Price  float64 `json:"price"`
		Number int     `json:"number"`
	}
	list := make([]MinuteItem, 0)
	for _, m := range resp.List {
		list = append(list, MinuteItem{m.Time, m.Price.Float64(), m.Number})
	}
	jsonResp(w, map[string]interface{}{"code": code, "date": date, "count": len(list), "list": list})
}

func handleTrade(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" { jsonErr(w, "缺少code"); return }
	c := cli()
	if c == nil { jsonErr(w, "未连接"); return }
	resp, err := c.GetMinuteTrade(code, 0, 2000)
	if err != nil || resp == nil { jsonErr(w, "无数据"); return }
	type TradeItem struct {
		Time   string  `json:"time"`
		Price  float64 `json:"price"`
		Volume int     `json:"volume"`
		Status int     `json:"status"`
	}
	list := make([]TradeItem, 0)
	for _, t := range resp.List {
		list = append(list, TradeItem{t.Time.Format("15:04:05"), t.Price.Float64(), t.Volume, t.Status})
	}
	jsonResp(w, map[string]interface{}{"code": code, "count": len(list), "list": list})
}

func handleCallAuction(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" { jsonErr(w, "缺少code"); return }
	c := cli()
	if c == nil { jsonErr(w, "未连接"); return }
	resp, err := c.GetCallAuction(code)
	if err != nil || resp == nil { jsonErr(w, "无数据"); return }
	type CallItem struct {
		Time       string  `json:"time"`
		Price      float64 `json:"price"`
		Match      int64   `json:"match"`
		Unmatched  int64   `json:"unmatched"`
		Flag       int8    `json:"flag"`
	}
	list := make([]CallItem, 0)
	for _, a := range resp.List {
		list = append(list, CallItem{a.Time.Format("15:04:05"), a.Price.Float64(), a.Match, a.Unmatched, a.Flag})
	}
	jsonResp(w, map[string]interface{}{"code": code, "count": len(list), "list": list})
}

// ====== 复权系统 ======

func handleGbbq(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" { jsonErr(w, "缺少code"); return }
	c := cli()
	if c == nil { jsonErr(w, "未连接"); return }
	resp, err := c.GetGbbq(code)
	if err != nil || resp == nil { jsonErr(w, "无数据"); return }
	type GbbqItem struct {
		Date     string  `json:"date"`
		Category int     `json:"category"`
		C1       float64 `json:"c1"`
		C2       float64 `json:"c2"`
		C3       float64 `json:"c3"`
		C4       float64 `json:"c4"`
	}
	list := make([]GbbqItem, 0)
	for _, g := range resp.List {
		list = append(list, GbbqItem{g.Time.Format("2006-01-02"), g.Category, g.C1, g.C2, g.C3, g.C4})
	}
	jsonResp(w, map[string]interface{}{"code": code, "count": len(list), "list": list})
}

func handleFactors(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" { jsonErr(w, "缺少code"); return }
	gb := getGbbq()
	if gb == nil { jsonErr(w, "复权模块未就绪"); return }
	c := cli()
	if c == nil { jsonErr(w, "未连接"); return }
	resp, err := c.GetKlineDayAll(code)
	if err != nil || resp == nil { jsonErr(w, "获取K线失败"); return }
	fs := gb.GetFactors(code, resp.List)
	type FactorItem struct {
		Date   string  `json:"date"`
		QFQMul float64 `json:"qfq_mul"`
		QFQAdd float64 `json:"qfq_add"`
		HFQMul float64 `json:"hfq_mul"`
		HFQAdd float64 `json:"hfq_add"`
	}
	list := make([]FactorItem, 0)
	for _, f := range fs {
		list = append(list, FactorItem{f.Time.Format("2006-01-02"), f.QFQMul, f.QFQAdd, f.HFQMul, f.HFQAdd})
	}
	jsonResp(w, map[string]interface{}{"code": code, "count": len(list), "list": list})
}

// handleGbbqAll 全市场复权因子
func handleGbbqAll(w http.ResponseWriter, r *http.Request) {
	c := cli()
	if c == nil { jsonErr(w, "未连接"); return }
	m, err := c.GetGbbqAll()
	if err != nil { jsonErr(w, err.Error()); return }
	codes := make([]string, 0, len(m))
	for code := range m {
		codes = append(codes, code)
	}
	jsonResp(w, map[string]interface{}{"count": len(codes), "codes": codes})
}

// ====== 基本面 ======

func handleFinance(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" { jsonErr(w, "缺少code"); return }
	mkt := parseExchange(r.URL.Query().Get("mkt"))
	c := cli()
	if c == nil { jsonErr(w, "未连接"); return }
	f, err := c.GetFinanceInfo(mkt, code)
	if err != nil || f == nil { jsonErr(w, "无数据"); return }
	jsonResp(w, map[string]interface{}{
		"code": code, "updatedDate": f.UpdatedDate, "ipoDate": f.IPODate,
		"liuTongGuBen": f.LiuTongGuBen, "zongGuBen": f.ZongGuBen,
		"guoJiaGu": f.GuoJiaGu, "zongZiChan": f.ZongZiChan,
		"liuDongZiChan": f.LiuDongZiChan, "guDingZiChan": f.GuDingZiChan,
		"wuXingZiChan": f.WuXingZiChan, "guDongRenShu": f.GuDongRenShu,
		"liuDongFuZhai": f.LiuDongFuZhai, "changQiFuZhai": f.ChangQiFuZhai,
		"ziBenGongJiJin": f.ZiBenGongJiJin, "jingZiChan": f.JingZiChan,
		"zhuYingShouRu": f.ZhuYingShouRu, "zhuYingLiRun": f.ZhuYingLiRun,
		"yingShouZhangKuan": f.YingShouZhangKuan, "yingYeLiRun": f.YingYeLiRun,
		"touZiShouYi": f.TouZiShouYi, "jingYingXianJinLiu": f.JingYingXianJinLiu,
		"zongXianJinLiu": f.ZongXianJinLiu, "cunHuo": f.CunHuo,
		"liRunZongHe": f.LiRunZongHe, "shuiHouLiRun": f.ShuiHouLiRun,
		"jingLiRun": f.JingLiRun, "weiFenLiRun": f.WeiFenLiRun,
	})
}

func handleF10(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" { jsonErr(w, "缺少code"); return }
	mkt := parseExchange(r.URL.Query().Get("mkt"))
	catIdx := -1
	if s := r.URL.Query().Get("cat"); s != "" { catIdx, _ = strconv.Atoi(s) }
	c := cli()
	if c == nil { jsonErr(w, "未连接"); return }
	cats, err := c.GetCompanyCategory(mkt, code)
	if err != nil { jsonErr(w, err.Error()); return }
	type CatItem struct {
		Index    int    `json:"index"`
		Name     string `json:"name"`
		Filename string `json:"filename,omitempty"`
		Length   int    `json:"length"`
	}
	list := make([]CatItem, len(cats))
	for i, ct := range cats { list[i] = CatItem{i, ct.Name, ct.Filename, int(ct.Length)} }
	if catIdx >= 0 && catIdx < len(cats) {
		ct, err := c.GetCompanyContent(mkt, code, cats[catIdx].Filename, cats[catIdx].Start, cats[catIdx].Length)
		if err != nil { jsonErr(w, err.Error()); return }
		jsonResp(w, map[string]interface{}{"categories": list, "content": ct})
		return
	}
	jsonResp(w, map[string]interface{}{"categories": list})
}

// ====== 全市场统计 ======

func handleStat(w http.ResponseWriter, r *http.Request) {
	c := cli()
	if c == nil { jsonErr(w, "未连接"); return }
	stats, err := c.GetTdxStat()
	if err != nil { jsonErr(w, err.Error()); return }
	type StatItem struct {
		Code      string  `json:"code"`
		PETTM     float64 `json:"pe_ttm"`
		PEStatic  float64 `json:"pe_static"`
		DivYield  float64 `json:"div_yield"`
		ChangePct float64 `json:"change_pct"`
		TrendDays int     `json:"trend_days"`
		Chg5      float64 `json:"chg_5"`
		Chg10     float64 `json:"chg_10"`
		Chg20     float64 `json:"chg_20"`
		Chg60     float64 `json:"chg_60"`
		ChgYTD    float64 `json:"chg_ytd"`
	}
	list := make([]StatItem, 0)
	for _, s := range stats {
		list = append(list, StatItem{s.Code, s.PETTM, s.PEStatic, s.DivYield, s.ChangePct, s.TrendDays, s.Chg5, s.Chg10, s.Chg20, s.Chg60, s.ChgYTD})
	}
	limit := parseCount(r.URL.Query().Get("limit"), 0)
	if limit > 0 && limit < len(list) { list = list[:limit] }
	jsonResp(w, map[string]interface{}{"count": len(list), "list": list})
}

func handleMoneyflow(w http.ResponseWriter, r *http.Request) {
	c := cli()
	if c == nil { jsonErr(w, "未连接"); return }
	stats, err := c.GetTdxStat2()
	if err != nil { jsonErr(w, err.Error()); return }
	type FlowItem struct {
		Code       string  `json:"code"`
		BlockIndex string  `json:"block_index"`
		Amount     float64 `json:"amount"`
		AmountPrev float64 `json:"amount_prev"`
		IPOPrice   float64 `json:"ipo_price"`
		High52W    float64 `json:"high_52w"`
		Low52W     float64 `json:"low_52w"`
	}
	list := make([]FlowItem, 0)
	for _, s := range stats {
		list = append(list, FlowItem{s.Code, s.BlockIndex, s.Amount, s.AmountPrev, s.IPOPrice, s.High52W, s.Low52W})
	}
	limit := parseCount(r.URL.Query().Get("limit"), 0)
	if limit > 0 && limit < len(list) { list = list[:limit] }
	jsonResp(w, map[string]interface{}{"count": len(list), "list": list})
}

func handleBlocks(w http.ResponseWriter, r *http.Request) {
	typ := r.URL.Query().Get("type")
	c := cli()
	if c == nil { jsonErr(w, "未连接"); return }
	file := protocol.BlockFileGN
	switch typ {
	case "fg", "region", "地域":
		file = protocol.BlockFileFG
	case "zs", "index", "指数":
		file = protocol.BlockFileZS
	}
	blocks, err := c.GetBlockDataWithIndex(file)
	if err != nil { jsonErr(w, err.Error()); return }
	type BlockItem struct {
		Name  string   `json:"name"`
		Index string   `json:"index"`
		Type  uint16   `json:"type"`
		Codes []string `json:"codes"`
	}
	list := make([]BlockItem, 0)
	for _, b := range blocks {
		list = append(list, BlockItem{b.Name, b.Index, b.Type, b.Codes})
	}
	jsonResp(w, map[string]interface{}{"type": typ, "count": len(list), "list": list})
}

func handleHy(w http.ResponseWriter, r *http.Request) {
	c := cli()
	if c == nil { jsonErr(w, "未连接"); return }
	hy, err := c.GetTdxHy()
	if err != nil { jsonErr(w, err.Error()); return }
	type HyItem struct {
		Code  string `json:"code"`
		TdxHy string `json:"tdx_hy"`
		SwHy  string `json:"sw_hy"`
	}
	list := make([]HyItem, 0)
	for _, h := range hy {
		list = append(list, HyItem{h.Code, h.TdxHy, h.SwHy})
	}
	jsonResp(w, map[string]interface{}{"count": len(list), "list": list})
}

func handleCodes(w http.ResponseWriter, r *http.Request) {
	c := cli()
	if c == nil { jsonErr(w, "未连接"); return }
	codes, err := c.GetStockCodeAll()
	if err != nil { jsonErr(w, err.Error()); return }
	exFilter := r.URL.Query().Get("exchange")
	if exFilter != "" {
		filtered := make([]string, 0)
		for _, code := range codes {
			if (exFilter == "sh" && strings.HasPrefix(code, "sh")) ||
				(exFilter == "sz" && strings.HasPrefix(code, "sz")) ||
				(exFilter == "bj" && strings.HasPrefix(code, "bj")) {
				filtered = append(filtered, code)
			}
		}
		codes = filtered
	}
	limit := parseCount(r.URL.Query().Get("limit"), 0)
	if limit > 0 && limit < len(codes) { codes = codes[:limit] }
	jsonResp(w, map[string]interface{}{"count": len(codes), "list": codes})
}

// handleCodesETF 全量 ETF 代码列表
func handleCodesETF(w http.ResponseWriter, r *http.Request) {
	c := cli()
	if c == nil { jsonErr(w, "未连接"); return }
	codes, err := c.GetETFCodeAll()
	if err != nil { jsonErr(w, err.Error()); return }
	jsonResp(w, map[string]interface{}{"count": len(codes), "list": codes})
}

// handleCodesIndex 全量指数代码列表
func handleCodesIndex(w http.ResponseWriter, r *http.Request) {
	c := cli()
	if c == nil { jsonErr(w, "未连接"); return }
	codes, err := c.GetIndexCodeAll()
	if err != nil { jsonErr(w, err.Error()); return }
	jsonResp(w, map[string]interface{}{"count": len(codes), "list": codes})
}

// ====== 辅助函数 ======

type KlineItem struct {
	Date   string  `json:"date"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume int64   `json:"volume"`
	Amount float64 `json:"amount,omitempty"`
}

func toKlineList(ks protocol.Klines) []KlineItem {
	list := make([]KlineItem, len(ks))
	for i, k := range ks {
		list[i] = KlineItem{
			k.Time.Format("2006-01-02"), k.Open.Float64(), k.High.Float64(),
			k.Low.Float64(), k.Close.Float64(), k.Volume, k.Amount.Float64(),
		}
	}
	return list
}

func parseCount(s string, def int) int {
	if s == "" { return def }
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 { return def }
	if n > 800 { n = 800 }
	return n
}

func fetchKline(c *tdx.Client, code, typ string, cnt int, reverse bool) ([]KlineItem, error) {
	var resp *protocol.KlineResp
	var err error
	switch typ {
	case "minute1":
		resp, err = c.GetKlineMinute(code, 0, uint16(cnt))
	case "minute5":
		resp, err = c.GetKline5Minute(code, 0, uint16(cnt))
	case "minute15":
		resp, err = c.GetKline15Minute(code, 0, uint16(cnt))
	case "minute30":
		resp, err = c.GetKline30Minute(code, 0, uint16(cnt))
	case "hour", "minute60":
		resp, err = c.GetKline60Minute(code, 0, uint16(cnt))
	case "week":
		resp, err = c.GetKlineWeek(code, 0, uint16(cnt))
	case "month":
		resp, err = c.GetKlineMonth(code, 0, uint16(cnt))
	case "quarter":
		resp, err = c.GetKlineQuarter(code, 0, uint16(cnt))
	case "year":
		resp, err = c.GetKlineYear(code, 0, uint16(cnt))
	default:
		resp, err = c.GetKlineDay(code, 0, uint16(cnt))
	}
	if err != nil { return nil, err }
	if resp == nil || len(resp.List) == 0 { return nil, nil }
	list := toKlineList(resp.List)
	if reverse {
		for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
			list[i], list[j] = list[j], list[i]
		}
	}
	return list, nil
}

func fetchKlineAll(c *tdx.Client, code, typ string) ([]KlineItem, error) {
	typ = strings.ToLower(typ)
	var resp *protocol.KlineResp
	var err error
	switch typ {
	case "minute1":
		resp, err = c.GetKlineMinuteAll(code)
	case "minute5":
		resp, err = c.GetKline5MinuteAll(code)
	case "minute15":
		resp, err = c.GetKline15MinuteAll(code)
	case "minute30":
		resp, err = c.GetKline30MinuteAll(code)
	case "hour", "minute60":
		resp, err = c.GetKline60MinuteAll(code)
	case "week":
		resp, err = c.GetKlineWeekAll(code)
	case "month":
		resp, err = c.GetKlineMonthAll(code)
	case "quarter":
		resp, err = c.GetKlineQuarterAll(code)
	case "year":
		resp, err = c.GetKlineYearAll(code)
	default:
		resp, err = c.GetKlineDayAll(code)
	}
	if err != nil { return nil, err }
	if resp == nil { return nil, nil }
	return toKlineList(resp.List), nil
}

package main

import (
	"html/template"
	"net/http"
	"sort"
	"github.com/injoyai/tdx/protocol"
	"strings"
	"time"

	"github.com/injoyai/tdx/extend"
)

// ====== WebUI ======

func handleWebUI(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" { http.NotFound(w, r); return }
	tmpl, _ := template.ParseFS(staticFiles, "static/index.html")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.Execute(w, nil)
}

// ====== 指数 ======

func handleIndexKline(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	typ := r.URL.Query().Get("type")
	if code == "" { jsonErr(w, "缺少code"); return }
	cnt := parseCount(r.URL.Query().Get("count"), 10)
	c := cli()
	if c == nil { jsonErr(w, "未连接"); return }
	var err error; var list []KlineItem
	switch typ {
	case "minute1":
		rp, e := c.GetIndex(0x0008, code, 0, uint16(cnt))
		if e != nil { err = e } else { list = toKlineList(rp.List) }
	case "minute5":
		rp, e := c.GetIndex5Minute(code, 0, uint16(cnt))
		if e != nil { err = e } else { list = toKlineList(rp.List) }
	case "minute15":
		rp, e := c.GetIndex15Minute(code, 0, uint16(cnt))
		if e != nil { err = e } else { list = toKlineList(rp.List) }
	case "minute30":
		rp, e := c.GetIndex30Minute(code, 0, uint16(cnt))
		if e != nil { err = e } else { list = toKlineList(rp.List) }
	case "hour", "minute60":
		rp, e := c.GetIndex60Minute(code, 0, uint16(cnt))
		if e != nil { err = e } else { list = toKlineList(rp.List) }
	default:
		rp, e := c.GetIndexDay(code, 0, uint16(cnt))
		if e != nil { err = e } else { list = toKlineList(rp.List) }
	}
	if err != nil { jsonErr(w, err.Error()); return }
	jsonResp(w, map[string]interface{}{"code": code, "type": typ, "count": len(list), "list": list})
}

func handleIndexKlineAll(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	typ := r.URL.Query().Get("type")
	if code == "" { jsonErr(w, "缺少code"); return }
	c := cli()
	if c == nil { jsonErr(w, "未连接"); return }
	var err error; var list []KlineItem
	switch typ {
	case "week":
		rp, e := c.GetIndexWeekAll(code)
		if e != nil { err = e } else { list = toKlineList(rp.List) }
	case "month":
		rp, e := c.GetIndexMonthAll(code)
		if e != nil { err = e } else { list = toKlineList(rp.List) }
	case "quarter":
		rp, e := c.GetIndexQuarterAll(code)
		if e != nil { err = e } else { list = toKlineList(rp.List) }
	case "year":
		rp, e := c.GetIndexYearAll(code)
		if e != nil { err = e } else { list = toKlineList(rp.List) }
	default:
		rp, e := c.GetIndexDayAll(code)
		if e != nil { err = e } else { list = toKlineList(rp.List) }
	}
	if err != nil { jsonErr(w, err.Error()); return }
	jsonResp(w, map[string]interface{}{"code": code, "type": typ, "count": len(list), "list": list})
}

// ====== 扩展行情 ======

func handleExQuote(w http.ResponseWriter, r *http.Request) {
	ex := getEx()
	if ex == nil { jsonErr(w, "扩展行情未连接"); return }
	code := r.URL.Query().Get("code")
	mkt := uint8(parseCount(r.URL.Query().Get("market"), 31))
	if code == "" { jsonErr(w, "缺少code"); return }
	q, err := ex.ExQuote(mkt, code)
	if err != nil || q == nil { jsonErr(w, "无数据"); return }
	jsonResp(w, map[string]interface{}{
		"market": q.Market, "code": q.Code,
		"price": q.Price, "preClose": q.PreClose,
		"open": q.Open, "high": q.High, "low": q.Low,
		"volume": q.ZongLiang, "openInterest": q.ChiCang,
		"bid": q.Bid, "bidVol": q.BidVol,
		"ask": q.Ask, "askVol": q.AskVol,
	})
}

func handleExBars(w http.ResponseWriter, r *http.Request) {
	ex := getEx()
	if ex == nil { jsonErr(w, "扩展行情未连接"); return }
	code := r.URL.Query().Get("code")
	mkt := uint8(parseCount(r.URL.Query().Get("market"), 31))
	cat := uint8(parseCount(r.URL.Query().Get("category"), 9))
	cnt := uint16(parseCount(r.URL.Query().Get("count"), 20))
	if code == "" { jsonErr(w, "缺少code"); return }
	bars, err := ex.ExBars(cat, mkt, code, 0, cnt)
	if err != nil { jsonErr(w, err.Error()); return }
	jsonResp(w, map[string]interface{}{"market": mkt, "code": code, "count": len(bars), "list": bars})
}

func handleExMinute(w http.ResponseWriter, r *http.Request) {
	ex := getEx()
	if ex == nil { jsonErr(w, "扩展行情未连接"); return }
	code := r.URL.Query().Get("code")
	mkt := uint8(parseCount(r.URL.Query().Get("market"), 31))
	if code == "" { jsonErr(w, "缺少code"); return }
	mint, err := ex.ExMinute(mkt, code)
	if err != nil { jsonErr(w, err.Error()); return }
	jsonResp(w, map[string]interface{}{"market": mkt, "code": code, "count": len(mint), "list": mint})
}

func handleExTrade(w http.ResponseWriter, r *http.Request) {
	ex := getEx()
	if ex == nil { jsonErr(w, "扩展行情未连接"); return }
	code := r.URL.Query().Get("code")
	mkt := uint8(parseCount(r.URL.Query().Get("market"), 31))
	cnt := uint16(parseCount(r.URL.Query().Get("count"), 30))
	if code == "" { jsonErr(w, "缺少code"); return }
	ticks, err := ex.ExTrade(mkt, code, 0, cnt)
	if err != nil { jsonErr(w, err.Error()); return }
	jsonResp(w, map[string]interface{}{"market": mkt, "code": code, "count": len(ticks), "list": ticks})
}

func handleExMarkets(w http.ResponseWriter, r *http.Request) {
	ex := getEx()
	if ex == nil { jsonErr(w, "扩展行情未连接"); return }
	markets, err := ex.ExMarkets()
	if err != nil { jsonErr(w, err.Error()); return }
	jsonResp(w, map[string]interface{}{"count": len(markets), "list": markets})
}

func handleExInstruments(w http.ResponseWriter, r *http.Request) {
	ex := getEx()
	if ex == nil { jsonErr(w, "扩展行情未连接"); return }
	start := uint32(parseCount(r.URL.Query().Get("start"), 0))
	cnt := uint16(parseCount(r.URL.Query().Get("count"), 100))
	insts, err := ex.ExInstruments(start, cnt)
	if err != nil { jsonErr(w, err.Error()); return }
	jsonResp(w, map[string]interface{}{"start": start, "count": len(insts), "list": insts})
}

// ====== 搜索 ======

func handleSearch(w http.ResponseWriter, r *http.Request) {
	kw := r.URL.Query().Get("keyword")
	if kw == "" { jsonErr(w, "缺少keyword"); return }
	c := cli()
	if c == nil { jsonErr(w, "未连接"); return }
	allCodes, err := c.GetStockCodeAll()
	if err != nil { jsonErr(w, err.Error()); return }
	kw = strings.ToUpper(kw)
	type Item struct { Code string `json:"code"`; Exchange string `json:"exchange"` }
	results := make([]Item, 0); seen := make(map[string]bool)
	for _, code := range allCodes {
		if strings.Contains(strings.ToUpper(code), kw) {
			ex := "sh"
			if strings.HasPrefix(code, "sz") { ex = "sz" } else if strings.HasPrefix(code, "bj") { ex = "bj" }
			short := code[2:]
			if !seen[short] { results = append(results, Item{short, ex}); seen[short] = true }
		}
		if len(results) >= 50 { break }
	}
	jsonResp(w, map[string]interface{}{"keyword": kw, "count": len(results), "list": results})
}

// ====== 交易日 ======

func handleWorkday(w http.ResponseWriter, r *http.Request) {
	dateStr := r.URL.Query().Get("date")
	target := time.Now()
	if dateStr != "" {
		var err error
		target, err = time.Parse("20060102", dateStr)
		if err != nil { target, err = time.Parse("2006-01-02", dateStr) }
		if err != nil { jsonErr(w, "日期格式错误"); return }
	}
	isWorkday := target.Weekday() != time.Saturday && target.Weekday() != time.Sunday
	jsonResp(w, map[string]interface{}{"date": target.Format("2006-01-02"), "is_workday": isWorkday})
}

func handleWorkdayRange(w http.ResponseWriter, r *http.Request) {
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")
	if startStr == "" || endStr == "" { jsonErr(w, "缺少start或end"); return }
	start, _ := time.Parse("20060102", startStr)
	if start.IsZero() { start, _ = time.Parse("2006-01-02", startStr) }
	end, _ := time.Parse("20060102", endStr)
	if end.IsZero() { end, _ = time.Parse("2006-01-02", endStr) }
	if start.IsZero() || end.IsZero() { jsonErr(w, "日期格式错误"); return }
	list := make([]string, 0)
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		if d.Weekday() != time.Saturday && d.Weekday() != time.Sunday {
			list = append(list, d.Format("2006-01-02"))
		}
	}
	jsonResp(w, map[string]interface{}{"count": len(list), "list": list})
}

// ====== 历史成交 ======

func handleHistoryTrade(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	date := r.URL.Query().Get("date")
	if code == "" || date == "" { jsonErr(w, "缺少code或date"); return }
	c := cli()
	if c == nil { jsonErr(w, "未连接"); return }
	date = strings.ReplaceAll(date, "-", "")
	resp, err := c.GetHistoryMinuteTradeDay(date, code)
	if err != nil || resp == nil { jsonErr(w, "无数据"); return }
	type Item struct {
		Time string `json:"time"`; Price float64 `json:"price"`
		Volume int `json:"volume"`; Status int `json:"status"`
	}
	list := make([]Item, 0)
	for _, t := range resp.List {
		list = append(list, Item{t.Time.Format("15:04:05"), t.Price.Float64(), t.Volume, t.Status})
	}
	jsonResp(w, map[string]interface{}{"code": code, "date": date, "count": len(list), "list": list})
}

// ====== 收益率 ======

func handleIncome(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	startStr := r.URL.Query().Get("start_date")
	if code == "" || startStr == "" { jsonErr(w, "缺少code或start_date"); return }
	startDate, _ := time.Parse("20060102", startStr)
	if startDate.IsZero() { startDate, _ = time.Parse("2006-01-02", startStr) }
	if startDate.IsZero() { jsonErr(w, "start_date格式错误"); return }
	
	daysParam := r.URL.Query().Get("days")
	days := make([]int, 0)
	if daysParam == "" {
		days = []int{5, 10, 20, 60, 120}
	} else {
		for _, s := range strings.Split(daysParam, ",") {
			s = strings.TrimSpace(s)
			if n, e := parseInt(s); e == nil { days = append(days, n) }
		}
	}
	
	c := cli()
	if c == nil { jsonErr(w, "未连接"); return }
	resp, err := c.GetKlineDayAll(code)
	if err != nil || resp == nil { jsonErr(w, "无数据"); return }
	
	klines := make(extend.Klines, 0)
	for _, k := range resp.List {
		klines = append(klines, &extend.Kline{
			Kline: k,
		})
	}
	pk := make(protocol.Klines, len(klines))
	for i, k := range klines { pk[i] = k.Kline }
	sort.Slice(pk, func(i, j int) bool { return pk[i].Time.Unix() < pk[j].Time.Unix() })
	incomes := extend.DoIncomes(pk, startDate, days...)
	type Item struct {
		Offset   int     `json:"offset"`
		Rise     float64 `json:"rise"`
		RiseRate float64 `json:"rise_rate"`
		Close    float64 `json:"close"`
		RefClose float64 `json:"ref_close"`
	}
	list := make([]Item, 0)
	for _, inc := range incomes {
		if inc == nil { continue }
		list = append(list, Item{inc.Offset, inc.Rise().Float64(), inc.RiseRate(), inc.Current.Close.Float64(), inc.Source.Close.Float64()})
	}
	jsonResp(w, map[string]interface{}{"code": code, "count": len(list), "list": list})
}

// ====== 健康检查 & 状态 ======

func handleHealth(w http.ResponseWriter, r *http.Request) {
	st := "ok"; exSt := "disconnected"
	if mainClient == nil { st = "disconnected" }
	if exClient != nil { exSt = "ok" }
	jsonResp(w, map[string]string{
		"status": st, "ex_status": exSt,
		"version": "2.0.0", "server_time": time.Now().Format("2006-01-02 15:04:05"),
	})
}

func handleServerStatus(w http.ResponseWriter, r *http.Request) {
	jsonResp(w, map[string]interface{}{
		"status": func() string { if mainClient == nil { return "disconnected" }; return "running" }(),
		"version": "2.0.0", "uptime": time.Since(startTime).String(),
		"gbbq": gbbq != nil, "ex_hq": exClient != nil,
	})
}

func parseInt(s string) (int, error) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' { return 0, nil }
		n = n*10 + int(c-'0')
	}
	return n, nil
}

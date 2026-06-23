package main

import (
	"encoding/binary"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/injoyai/tdx"
	"github.com/injoyai/tdx/protocol"
)

// ====== WebUI ======

func handleWebUI(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" { http.NotFound(w, r); return }
	tmpl, _ := template.ParseFS(staticFiles, "static/index.html")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.Execute(w, nil)
}

// ====== 搜索 ======

var stockNameMap map[string]string

func loadStockNames(c *tdx.Client) map[string]string {
	if stockNameMap != nil {
		return stockNameMap
	}
	m := make(map[string]string)
	files, err := c.GetZHBFiles()
	if err != nil {
		stockNameMap = m
		return m
	}
	data, ok := files[protocol.FileHsPy]
	if !ok {
		stockNameMap = m
		return m
	}
	text := string(protocol.UTF8ToGBK(data))
	lines := strings.Split(text, "\n")
	for _, ln := range lines {
		ln = strings.TrimRight(ln, "\r")
		if ln == "" || strings.HasPrefix(ln, "#") {
			continue
		}
		f := strings.Split(ln, "|")
		if len(f) < 3 {
			continue
		}
		code := strings.TrimSpace(f[0]) + strings.TrimSpace(f[1])
		name := strings.TrimSpace(f[2])
		if code != "" && name != "" {
			m[code] = name
		}
	}

	// profile.dat 二进制文件，包含完整代码→最新名称映射
	profData, ok := files["profile.dat"]
	if ok && len(profData) > 0 {
		parseProfileNames(profData, m)
	}

	stockNameMap = m
	return m
}

func parseProfileNames(data []byte, m map[string]string) {
	// profile.dat 格式: 记录以 \x00\x00\x00 开头，含 4 字节小端代码 + \x00 + GBK名称
	// 每条记录: header(3-4字节) + code(4字节) + \x00 + name(GBK) + \x00 + binary
	i := 0
	for i < len(data)-12 {
		// 找代码区域的起始: code 由小端 uint32 编码，后跟 \x00
		for ; i < len(data)-10; i++ {
			if data[i] == 0 && data[i+1] == 0 && data[i+2] == 0 && data[i+3] <= 0x09 {
				// 可能是代码起始
				break
			}
		}
		if i >= len(data)-10 {
			break
		}
		code := int(binary.LittleEndian.Uint32(data[i : i+4]))
		if code <= 0 || code > 999999 {
			i += 4
			continue
		}
		// 跳过代码 + \x00
		j := i + 4
		if j >= len(data) || data[j] != 0 {
			i = j
			continue
		}
		j++ // skip \x00
		// 读取名称直到 \x00 或 非法字符
		start := j
		for j < len(data) && data[j] != 0 {
			j++
		}
		if j > start {
			nameBytes := data[start:j]
			if utf8.Valid(nameBytes) {
				name := string(nameBytes)
				codeStr := fmt.Sprintf("%06d", code)
				if _, exists := m[codeStr]; !exists {
					// 取最新(非历史变更)的名称
					m[codeStr] = name
				}
			} else {
				// GBK 解码
				name := string(protocol.UTF8ToGBK(nameBytes))
				codeStr := fmt.Sprintf("%06d", code)
				if _, exists := m[codeStr]; !exists {
					m[codeStr] = name
				}
			}
		}
		i = j + 1
	}
}

func handleSearch(w http.ResponseWriter, r *http.Request) {
	kw := r.URL.Query().Get("keyword")
	if kw == "" { jsonErr(w, "缺少keyword"); return }
	c := cli()
	if c == nil { jsonErr(w, "未连接"); return }

	// 加载代码→名称映射
	nameMap := loadStockNames(c)

	allCodes, err := c.GetStockCodeAll()
	if err != nil { jsonErr(w, err.Error()); return }
	kw = strings.ToUpper(kw)
	type Item struct {
		Code     string `json:"code"`
		Exchange string `json:"exchange"`
		Name     string `json:"name,omitempty"`
	}
	results := make([]Item, 0)
	seen := make(map[string]bool)
	for _, full := range allCodes {
		short := full[2:]
		if seen[short] {
			continue
		}
		ex := "sh"
		if strings.HasPrefix(full, "sz") { ex = "sz" } else if strings.HasPrefix(full, "bj") { ex = "bj" }

		name := nameMap[short]
		upperName := strings.ToUpper(name)
		matches := false
		if strings.Contains(strings.ToUpper(short), kw) {
			matches = true
		}
		if !matches && name != "" && strings.Contains(upperName, kw) {
			matches = true
		}
		if !matches && name != "" {
			// 拼音首字母匹配
			pinyinShort := ""
			for _, r := range name {
				if r > 127 {
					break
				}
				pinyinShort += string(r)
			}
			if strings.Contains(strings.ToUpper(pinyinShort), kw) {
				matches = true
			}
		}

		if matches {
			results = append(results, Item{short, ex, name})
			seen[short] = true
		}
		if len(results) >= 50 {
			break
		}
	}
	jsonResp(w, map[string]interface{}{"keyword": kw, "count": len(results), "list": results})
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

// ====== P1: 自定义周期复权 K 线 ======

func handleGbbqAdjust(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	typ := r.URL.Query().Get("type")
	adj := r.URL.Query().Get("adjust")
	cnt := parseCount(r.URL.Query().Get("count"), 60)
	if code == "" { jsonErr(w, "缺少code"); return }
	if adj == "" { adj = "qfq" }

	gb := getGbbq()
	if gb == nil { jsonErr(w, "复权模块未就绪"); return }
	c := cli()
	if c == nil { jsonErr(w, "未连接"); return }

	var resp *protocol.KlineResp
	var err error
	switch typ {
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
	if err != nil || resp == nil { jsonErr(w, "K线获取失败"); return }

	var klines protocol.Klines
	if adj == "hfq" {
		klines = gb.HFQ(code, resp.List)
	} else {
		klines = gb.QFQ(code, resp.List)
	}
	if len(klines) == 0 { jsonErr(w, "复权后无数据"); return }
	list := toKlineList(klines)
	if cnt > 0 && cnt < len(list) { list = list[len(list)-cnt:] }
	jsonResp(w, map[string]interface{}{
		"code": code, "type": typ, "adjust": adj,
		"count": len(list), "list": list,
	})
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

// ====== 辅助函数 ======

func parseInt(s string) (int, error) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' { return 0, nil }
		n = n*10 + int(c-'0')
	}
	return n, nil
}

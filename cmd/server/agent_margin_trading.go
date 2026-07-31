package main

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/glebarez/go-sqlite"
)

const (
	defaultMarginTradingDays = 30
	maxMarginTradingDays     = 120
)

var marginHTTPClient = newMarginHTTPClient()

func newMarginHTTPClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Timeout: 25 * time.Second, Jar: jar}
}

type MarginTradingRecord struct {
	Date                       string   `json:"date"`
	Code                       string   `json:"code,omitempty"`
	Name                       string   `json:"name,omitempty"`
	FinancingBalance           float64  `json:"financingBalance"`
	FinancingBuy               float64  `json:"financingBuy"`
	FinancingRepay             *float64 `json:"financingRepay"`
	SecuritiesLendingSell      float64  `json:"securitiesLendingSell"`
	SecuritiesLendingRemaining float64  `json:"securitiesLendingRemaining"`
	SecuritiesLendingRepay     *float64 `json:"securitiesLendingRepay"`
	SecuritiesLendingBalance   *float64 `json:"securitiesLendingBalance"`
	TotalBalance               *float64 `json:"totalBalance"`
}

type AgentMarginTrading struct {
	Code                   string                `json:"code"`
	Name                   string                `json:"name,omitempty"`
	Exchange               string                `json:"exchange"`
	Source                 string                `json:"source"`
	QueryTime              string                `json:"queryTime"`
	IsMarginEligible       bool                  `json:"isMarginEligible"`
	EligibilityStatus      string                `json:"eligibilityStatus"`
	EligibilityDescription string                `json:"eligibilityDescription"`
	RequestedDays          int                   `json:"requestedDays"`
	ActualDays             int                   `json:"actualDays"`
	LatestDataDate         string                `json:"latestDataDate,omitempty"`
	Records                []MarginTradingRecord `json:"records"`
	Warnings               []string              `json:"warnings,omitempty"`
}

type AgentMarginTradingText struct {
	Code    string `json:"code"`
	Format  string `json:"format"`
	Content string `json:"content"`
}

var loadMarginTrading = loadMarginTradingFromSources

func handleAgentMarginTrading(w http.ResponseWriter, r *http.Request) {
	result, err := queryMarginTradingRequest(r)
	if err != nil {
		jsonErr(w, err.Error())
		return
	}
	jsonResp(w, result)
}

func handleAgentMarginTradingText(w http.ResponseWriter, r *http.Request) {
	result, err := queryMarginTradingRequest(r)
	if err != nil {
		jsonErr(w, err.Error())
		return
	}
	jsonResp(w, AgentMarginTradingText{
		Code:    result.Code,
		Format:  "text/markdown",
		Content: buildMarginTradingText(result),
	})
}

func queryMarginTradingRequest(r *http.Request) (AgentMarginTrading, error) {
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if len(code) != 6 {
		return AgentMarginTrading{}, fmt.Errorf("code必须是6位A股或A股ETF代码")
	}
	days := defaultMarginTradingDays
	if raw := strings.TrimSpace(r.URL.Query().Get("days")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > maxMarginTradingDays {
			return AgentMarginTrading{}, fmt.Errorf("days必须是1-%d的整数", maxMarginTradingDays)
		}
		days = parsed
	}
	return loadMarginTrading(code, days)
}

func loadMarginTradingFromSources(code string, days int) (AgentMarginTrading, error) {
	exchange, source, err := marginExchange(code)
	if err != nil {
		return AgentMarginTrading{}, err
	}
	db, err := openMarginTradingDB(agentDBPath())
	if err != nil {
		return AgentMarginTrading{}, err
	}
	defer db.Close()

	if err := refreshMarginTrading(db, exchange, code, days); err != nil {
		return AgentMarginTrading{}, err
	}
	records, err := queryCachedMarginTrading(db, exchange, code, days)
	if err != nil {
		return AgentMarginTrading{}, err
	}
	result := finalizeMarginTradingResult(AgentMarginTrading{
		Code:          code,
		Exchange:      exchange,
		Source:        source,
		QueryTime:     time.Now().In(shanghaiLocation()).Format(time.RFC3339),
		RequestedDays: days,
		Records:       records,
	})
	if len(records) < days {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("请求%d个交易日，当前仅取得%d个交易所披露记录", days, len(records)))
	}
	return result, nil
}

func finalizeMarginTradingResult(result AgentMarginTrading) AgentMarginTrading {
	result.ActualDays = len(result.Records)
	if len(result.Records) == 0 {
		result.EligibilityStatus = "not_eligible"
		result.EligibilityDescription = "交易所接口可联通但无该证券明细，判定当前不是融资融券标的证券"
		return result
	}
	result.IsMarginEligible = true
	result.EligibilityStatus = "eligible"
	result.EligibilityDescription = "交易所返回该证券融资融券明细，判定当前是融资融券标的证券"
	result.Name = result.Records[0].Name
	result.LatestDataDate = result.Records[0].Date
	return result
}

func marginExchange(code string) (string, string, error) {
	switch {
	case strings.HasPrefix(code, "0"), strings.HasPrefix(code, "1"),
		strings.HasPrefix(code, "2"), strings.HasPrefix(code, "3"):
		return "SZSE", "深圳证券交易所", nil
	case strings.HasPrefix(code, "4"), strings.HasPrefix(code, "8"),
		strings.HasPrefix(code, "9"):
		return "BSE", "北京证券交易所", nil
	case strings.HasPrefix(code, "5"), strings.HasPrefix(code, "6"):
		return "SSE", "上海证券交易所", nil
	default:
		return "", "", fmt.Errorf("无法识别股票所属交易所: %s", code)
	}
}

func openMarginTradingDB(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if err := ensureMarginTradingSchema(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func ensureMarginTradingSchema(db *sql.DB) error {
	agentDBWriteMu.Lock()
	defer agentDBWriteMu.Unlock()
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS margin_trading_daily (
			exchange TEXT NOT NULL,
			trade_date TEXT NOT NULL,
			code TEXT NOT NULL,
			name TEXT NOT NULL DEFAULT '',
			financing_balance REAL NOT NULL DEFAULT 0,
			financing_buy REAL NOT NULL DEFAULT 0,
			financing_repay REAL,
			lending_sell REAL NOT NULL DEFAULT 0,
			lending_remaining REAL NOT NULL DEFAULT 0,
			lending_repay REAL,
			lending_balance REAL,
			total_balance REAL,
			fetched_at TEXT NOT NULL,
			PRIMARY KEY (exchange, trade_date, code)
		);
		CREATE INDEX IF NOT EXISTS idx_margin_trading_code_date
			ON margin_trading_daily(exchange, code, trade_date DESC);
		CREATE TABLE IF NOT EXISTS margin_trading_fetch_dates (
			exchange TEXT NOT NULL,
			trade_date TEXT NOT NULL,
			code TEXT NOT NULL,
			fetched_at TEXT NOT NULL,
			PRIMARY KEY (exchange, trade_date, code)
		);
	`)
	return err
}

func refreshMarginTrading(db *sql.DB, exchange, code string, days int) error {
	cached, err := queryCachedMarginTrading(db, exchange, code, days)
	if err != nil {
		return err
	}
	oldestNeeded := ""
	if len(cached) == days {
		oldestNeeded = cached[len(cached)-1].Date
	}

	now := time.Now().In(shanghaiLocation())
	maxLookback := days*2 + 30
	for offset := 0; offset < maxLookback; offset++ {
		date := now.AddDate(0, 0, -offset).Format("2006-01-02")
		if oldestNeeded != "" && date < oldestNeeded {
			break
		}
		checked, err := marginDateChecked(db, exchange, code, date)
		if err != nil {
			return err
		}
		if checked {
			continue
		}
		record, found, err := fetchMarginTradingDay(exchange, code, date)
		if err != nil {
			return fmt.Errorf("获取%s融资融券数据失败: %w", date, err)
		}
		if err := cacheMarginTradingDay(db, exchange, code, date, record, found); err != nil {
			return err
		}
		if found {
			cached, err = queryCachedMarginTrading(db, exchange, code, days)
			if err != nil {
				return err
			}
			if len(cached) == days {
				oldestNeeded = cached[len(cached)-1].Date
			}
		}
	}
	return nil
}

func marginDateChecked(db *sql.DB, exchange, code, date string) (bool, error) {
	var recordCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM margin_trading_daily
		WHERE exchange = ? AND trade_date = ? AND code = ?`,
		exchange, date, code).Scan(&recordCount); err != nil || recordCount > 0 {
		return recordCount > 0, err
	}
	var fetchedAt string
	err := db.QueryRow(`SELECT fetched_at FROM margin_trading_fetch_dates
		WHERE exchange = ? AND trade_date = ? AND code = ?`, exchange, date, code).Scan(&fetchedAt)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	checkedAt, err := time.Parse(time.RFC3339, fetchedAt)
	if err != nil {
		return false, nil
	}
	recentBoundary := time.Now().In(shanghaiLocation()).AddDate(0, 0, -7).Format("2006-01-02")
	if date >= recentBoundary && time.Since(checkedAt) >= time.Hour {
		return false, nil
	}
	return true, nil
}

func cacheMarginTradingDay(
	db *sql.DB,
	exchange string,
	code string,
	date string,
	record MarginTradingRecord,
	found bool,
) error {
	agentDBWriteMu.Lock()
	defer agentDBWriteMu.Unlock()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339)
	if found {
		_, err = tx.Exec(`INSERT OR REPLACE INTO margin_trading_daily (
			exchange, trade_date, code, name, financing_balance, financing_buy,
			financing_repay, lending_sell, lending_remaining, lending_repay,
			lending_balance, total_balance, fetched_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			exchange, record.Date, record.Code, record.Name, record.FinancingBalance,
			record.FinancingBuy, record.FinancingRepay, record.SecuritiesLendingSell,
			record.SecuritiesLendingRemaining, record.SecuritiesLendingRepay,
			record.SecuritiesLendingBalance, record.TotalBalance, now)
		if err != nil {
			return err
		}
	}
	if _, err = tx.Exec(`INSERT OR REPLACE INTO margin_trading_fetch_dates
		(exchange, trade_date, code, fetched_at) VALUES (?, ?, ?, ?)`,
		exchange, date, code, now); err != nil {
		return err
	}
	return tx.Commit()
}

func queryCachedMarginTrading(
	db *sql.DB,
	exchange string,
	code string,
	days int,
) ([]MarginTradingRecord, error) {
	rows, err := db.Query(`SELECT trade_date, code, name, financing_balance,
		financing_buy, financing_repay, lending_sell, lending_remaining,
		lending_repay, lending_balance, total_balance
		FROM margin_trading_daily
		WHERE exchange = ? AND code = ?
		ORDER BY trade_date DESC LIMIT ?`, exchange, code, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := make([]MarginTradingRecord, 0, days)
	for rows.Next() {
		var record MarginTradingRecord
		var financingRepay sql.NullFloat64
		var lendingRepay sql.NullFloat64
		var lendingBalance sql.NullFloat64
		var totalBalance sql.NullFloat64
		if err := rows.Scan(
			&record.Date, &record.Code, &record.Name, &record.FinancingBalance,
			&record.FinancingBuy, &financingRepay,
			&record.SecuritiesLendingSell, &record.SecuritiesLendingRemaining,
			&lendingRepay, &lendingBalance, &totalBalance,
		); err != nil {
			return nil, err
		}
		record.FinancingRepay = marginNullFloat(financingRepay)
		record.SecuritiesLendingRepay = marginNullFloat(lendingRepay)
		record.SecuritiesLendingBalance = marginNullFloat(lendingBalance)
		record.TotalBalance = marginNullFloat(totalBalance)
		records = append(records, record)
	}
	return records, rows.Err()
}

func fetchMarginTradingDay(
	exchange string,
	code string,
	date string,
) (MarginTradingRecord, bool, error) {
	switch exchange {
	case "SSE":
		return fetchSSEMarginTrading(code, date)
	case "SZSE":
		return fetchSZSEMarginTrading(code, date)
	case "BSE":
		return fetchBSEMarginTrading(code, date)
	default:
		return MarginTradingRecord{}, false, fmt.Errorf("不支持的交易所: %s", exchange)
	}
}

func fetchSSEMarginTrading(code, date string) (MarginTradingRecord, bool, error) {
	compactDate := strings.ReplaceAll(date, "-", "")
	query := url.Values{
		"isPagination":      {"true"},
		"sqlId":             {"RZRQ_MX_INFO"},
		"preStockCode":      {code},
		"beginDate":         {compactDate},
		"endDate":           {compactDate},
		"pageHelp.pageSize": {"20"},
		"pageHelp.pageNo":   {"1"},
	}
	body, err := marginHTTPGet(
		"https://query.sse.com.cn/commonSoaQuery.do?"+query.Encode(),
		"https://www.sse.com.cn/",
	)
	if err != nil {
		return MarginTradingRecord{}, false, err
	}
	var response struct {
		Result []struct {
			Date             string  `json:"opDate"`
			Code             string  `json:"stockCode"`
			Name             string  `json:"securityAbbr"`
			FinancingBalance float64 `json:"rzye"`
			FinancingBuy     float64 `json:"rzmre"`
			FinancingRepay   float64 `json:"rzche"`
			LendingSell      float64 `json:"rqmcl"`
			LendingRemaining float64 `json:"rqyl"`
			LendingRepay     float64 `json:"rqchl"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return MarginTradingRecord{}, false, err
	}
	if len(response.Result) == 0 {
		return MarginTradingRecord{}, false, nil
	}
	item := response.Result[0]
	return MarginTradingRecord{
		Date:                       formatCompactMarginDate(item.Date),
		Code:                       item.Code,
		Name:                       item.Name,
		FinancingBalance:           item.FinancingBalance,
		FinancingBuy:               item.FinancingBuy,
		FinancingRepay:             marginFloat(item.FinancingRepay),
		SecuritiesLendingSell:      item.LendingSell,
		SecuritiesLendingRemaining: item.LendingRemaining,
		SecuritiesLendingRepay:     marginFloat(item.LendingRepay),
	}, true, nil
}

func fetchSZSEMarginTrading(code, date string) (MarginTradingRecord, bool, error) {
	query := url.Values{
		"SHOWTYPE":   {"xlsx"},
		"CATALOGID":  {"1837_xxpl"},
		"txtDate":    {date},
		"TABKEY":     {"tab2"},
		"tab2PAGENO": {"1"},
	}
	body, err := marginHTTPGet(
		"https://www.szse.cn/api/report/ShowReport?"+query.Encode(),
		"https://www.szse.cn/",
	)
	if err != nil {
		return MarginTradingRecord{}, false, err
	}
	values, found, err := findInlineXLSXRow(body, code)
	if err != nil || !found {
		return MarginTradingRecord{}, found, err
	}
	if len(values) < 8 {
		return MarginTradingRecord{}, false, fmt.Errorf("深交所融资融券表字段不足")
	}
	return MarginTradingRecord{
		Date:                       date,
		Code:                       values[0],
		Name:                       values[1],
		FinancingBuy:               parseMarginNumber(values[2]),
		FinancingBalance:           parseMarginNumber(values[3]),
		SecuritiesLendingSell:      parseMarginNumber(values[4]),
		SecuritiesLendingRemaining: parseMarginNumber(values[5]),
		SecuritiesLendingBalance:   marginFloat(parseMarginNumber(values[6])),
		TotalBalance:               marginFloat(parseMarginNumber(values[7])),
	}, true, nil
}

type bseMarginPage struct {
	Content []struct {
		Code             string  `json:"zqdm"`
		Name             string  `json:"zqjc"`
		FinancingBuy     float64 `json:"rzmre"`
		FinancingBalance float64 `json:"rzye"`
		LendingSell      float64 `json:"rqmcl"`
		LendingRemaining float64 `json:"rqyl"`
		LendingBalance   float64 `json:"rqye"`
		TotalBalance     float64 `json:"rzrqye"`
	} `json:"content"`
	TotalPages int `json:"totalPages"`
}

func fetchBSEMarginTrading(code, date string) (MarginTradingRecord, bool, error) {
	page, err := fetchBSEMarginPage(date, 0)
	if err != nil || page.TotalPages == 0 {
		return MarginTradingRecord{}, false, err
	}
	if record, found := findBSEMarginRecord(page, code, date); found {
		return record, true, nil
	}
	low, high := 1, page.TotalPages-1
	for low <= high {
		mid := (low + high) / 2
		page, err = fetchBSEMarginPage(date, mid)
		if err != nil {
			return MarginTradingRecord{}, false, err
		}
		if record, found := findBSEMarginRecord(page, code, date); found {
			return record, true, nil
		}
		if len(page.Content) == 0 || code < page.Content[0].Code {
			high = mid - 1
		} else {
			low = mid + 1
		}
	}
	return MarginTradingRecord{}, false, nil
}

func fetchBSEMarginPage(date string, pageNumber int) (bseMarginPage, error) {
	query := url.Values{
		"transDate": {date},
		"page":      {strconv.Itoa(pageNumber)},
		"callback":  {"cb"},
	}
	body, err := marginHTTPGet(
		"https://www.bse.cn/rzrqjyyexxController/detailInfoResult.do?"+query.Encode(),
		"https://www.bse.cn/",
	)
	if err != nil {
		return bseMarginPage{}, err
	}
	text := strings.TrimSpace(string(body))
	text = strings.TrimPrefix(text, "cb(")
	text = strings.TrimSuffix(text, ");")
	cut := strings.LastIndex(text, "], '")
	if cut < 0 {
		return bseMarginPage{}, fmt.Errorf("北交所返回格式异常")
	}
	text = text[:cut+1] + "]"
	var wrapper [][]bseMarginPage
	if err := json.Unmarshal([]byte(text), &wrapper); err != nil {
		return bseMarginPage{}, err
	}
	if len(wrapper) == 0 || len(wrapper[0]) == 0 {
		return bseMarginPage{}, nil
	}
	return wrapper[0][0], nil
}

func findBSEMarginRecord(
	page bseMarginPage,
	code string,
	date string,
) (MarginTradingRecord, bool) {
	for _, item := range page.Content {
		if item.Code != code {
			continue
		}
		return MarginTradingRecord{
			Date:                       date,
			Code:                       item.Code,
			Name:                       item.Name,
			FinancingBalance:           item.FinancingBalance,
			FinancingBuy:               item.FinancingBuy,
			SecuritiesLendingSell:      item.LendingSell,
			SecuritiesLendingRemaining: item.LendingRemaining,
			SecuritiesLendingBalance:   marginFloat(item.LendingBalance),
			TotalBalance:               marginFloat(item.TotalBalance),
		}, true
	}
	return MarginTradingRecord{}, false
}

func marginHTTPGet(rawURL, referer string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", referer)
	req.Header.Set("User-Agent", "Mozilla/5.0 tdx-api/2.1")
	resp, err := marginHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func findInlineXLSXRow(data []byte, code string) ([]string, bool, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, false, nil
	}
	for _, file := range reader.File {
		if file.Name != "xl/worksheets/sheet1.xml" {
			continue
		}
		stream, err := file.Open()
		if err != nil {
			return nil, false, err
		}
		defer stream.Close()
		decoder := xml.NewDecoder(stream)
		for {
			token, err := decoder.Token()
			if err == io.EOF {
				return nil, false, nil
			}
			if err != nil {
				return nil, false, err
			}
			start, ok := token.(xml.StartElement)
			if !ok || start.Name.Local != "row" {
				continue
			}
			var row struct {
				Cells []struct {
					Inline struct {
						Text string `xml:"t"`
					} `xml:"is"`
					Value string `xml:"v"`
				} `xml:"c"`
			}
			if err := decoder.DecodeElement(&row, &start); err != nil {
				return nil, false, err
			}
			values := make([]string, 0, len(row.Cells))
			for _, cell := range row.Cells {
				value := cell.Inline.Text
				if value == "" {
					value = cell.Value
				}
				values = append(values, strings.TrimSpace(value))
			}
			if len(values) > 0 && values[0] == code {
				return values, true, nil
			}
		}
	}
	return nil, false, nil
}

func parseMarginNumber(value string) float64 {
	value = strings.ReplaceAll(strings.TrimSpace(value), ",", "")
	number, _ := strconv.ParseFloat(value, 64)
	return number
}

func formatCompactMarginDate(value string) string {
	if len(value) != 8 {
		return value
	}
	return value[:4] + "-" + value[4:6] + "-" + value[6:]
}

func shanghaiLocation() *time.Location {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("CST", 8*60*60)
	}
	return location
}

func buildMarginTradingText(result AgentMarginTrading) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "# %s（%s）融资融券\n\n", valueOrMarginDash(result.Name), result.Code)
	fmt.Fprintf(&builder, "查询时间：%s\n", result.QueryTime)
	fmt.Fprintf(&builder, "数据来源：%s\n", result.Source)
	fmt.Fprintf(&builder, "标的状态：%s\n", result.EligibilityDescription)
	fmt.Fprintf(&builder, "最新已披露交易日：%s\n", valueOrMarginDate(result.LatestDataDate))
	fmt.Fprintf(&builder, "请求%d个交易日，实际返回%d个交易日。\n", result.RequestedDays, result.ActualDays)
	builder.WriteString("口径：days按交易所实际披露记录计数，不按自然日计数；记录从最新已披露交易日向前排列。\n\n")
	if len(result.Records) == 0 {
		builder.WriteString("结论：该证券当前不是融资融券标的，无逐日融资融券明细。")
		return builder.String()
	}

	builder.WriteString("| 交易日 | 融资余额(万元) | 融资买入(万元) | 融资偿还(万元) | 融券卖出(万股) | 融券余量(万股) | 融券余额(万元) | 两融余额(万元) |\n")
	builder.WriteString("|---|---:|---:|---:|---:|---:|---:|---:|\n")
	for _, record := range result.Records {
		fmt.Fprintf(&builder, "| %s | %.2f | %.2f | %s | %.2f | %.2f | %s | %s |\n",
			record.Date,
			record.FinancingBalance/10000,
			record.FinancingBuy/10000,
			formatOptionalMarginAmount(record.FinancingRepay, 10000),
			record.SecuritiesLendingSell/10000,
			record.SecuritiesLendingRemaining/10000,
			formatOptionalMarginAmount(record.SecuritiesLendingBalance, 10000),
			formatOptionalMarginAmount(record.TotalBalance, 10000),
		)
	}
	if len(result.Warnings) > 0 {
		builder.WriteString("\n提示：" + strings.Join(result.Warnings, "；") + "。")
	}
	return strings.TrimSpace(builder.String())
}

func formatOptionalMarginAmount(value *float64, divisor float64) string {
	if value == nil {
		return "—"
	}
	return fmt.Sprintf("%.2f", *value/divisor)
}

func marginFloat(value float64) *float64 {
	return &value
}

func marginNullFloat(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	return marginFloat(value.Float64)
}

func valueOrMarginDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "未知证券"
	}
	return value
}

func valueOrMarginDate(value string) string {
	if strings.TrimSpace(value) == "" {
		return "无可用数据"
	}
	return value
}

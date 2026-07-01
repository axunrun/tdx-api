package main

import (
	"database/sql"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	_ "github.com/glebarez/go-sqlite"
	"github.com/injoyai/tdx"
	"github.com/injoyai/tdx/protocol"
)

var (
	stocksCache   []stockRow
	stocksCacheMu sync.RWMutex
	stocksDBPath  string
)

type stockRow struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	Exchange string `json:"exchange"`
	Pinyin   string `json:"pinyin,omitempty"`
}

type stockNameEntry struct {
	name string
	exch string
}

func initStocksDB(c *tdx.Client) {
	stocksDBPath = agentFeatureDBPath("STOCKS_DB_PATH")
	if err := os.MkdirAll(filepath.Dir(stocksDBPath), 0755); err != nil {
		log.Printf("⚠️ 股票名称库目录创建失败: %v", err)
		return
	}
	count, err := refreshStocks(c)
	if err != nil {
		log.Printf("⚠️ 股票名称库刷新失败: %v", err)
		if loaded, loadErr := loadStocksCacheFromDB(); loadErr == nil && loaded > 0 {
			log.Printf("✅ 股票名称库已从旧缓存加载: %d 条 (%s)", loaded, stocksDBPath)
		}
		return
	}
	log.Printf("✅ 股票名称库刷新完成: %d 条 (%s)", count, stocksDBPath)
}

func refreshStocks(c *tdx.Client) (int, error) {
	if c == nil {
		return 0, fmt.Errorf("TDX客户端未连接")
	}

	stocks, err := fetchStockRowsFromCodeTables(c)
	if err != nil {
		log.Printf("⚠️ 股票代码表刷新失败，尝试使用 zhb.zip 兜底: %v", err)
	}
	if len(stocks) == 0 {
		files, err := c.GetZHBFiles()
		if err != nil {
			return 0, fmt.Errorf("下载 zhb.zip 失败: %w", err)
		}
		stocks = parseStockRows(files)
	}
	if len(stocks) == 0 {
		return 0, fmt.Errorf("未解析到股票名称数据")
	}
	if err := replaceStocksDB(stocks); err != nil {
		return 0, err
	}

	stocksCacheMu.Lock()
	stocksCache = stocks
	stocksCacheMu.Unlock()
	return len(stocks), nil
}

func fetchStockRowsFromCodeTables(c *tdx.Client) ([]stockRow, error) {
	exchanges := []protocol.Exchange{protocol.ExchangeSH, protocol.ExchangeSZ, protocol.ExchangeBJ}
	stocks := make([]stockRow, 0, 6000)
	seen := make(map[string]bool)
	var warnings []string

	for _, ex := range exchanges {
		resp, err := c.GetCodeAll(ex)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", ex.String(), err))
			continue
		}
		for _, item := range resp.List {
			if item == nil {
				continue
			}
			code := normalizeStockCode(item.Code)
			name := strings.TrimSpace(item.Name)
			if code == "" || name == "" {
				continue
			}
			fullCode := ex.String() + code
			if !protocol.IsStock(fullCode) || seen[code] {
				continue
			}
			stocks = append(stocks, stockRow{
				Code:     code,
				Name:     name,
				Exchange: ex.String(),
			})
			seen[code] = true
		}
	}
	sort.Slice(stocks, func(i, j int) bool {
		return stocks[i].Code < stocks[j].Code
	})
	if len(stocks) == 0 && len(warnings) > 0 {
		return nil, fmt.Errorf(strings.Join(warnings, "; "))
	}
	return stocks, nil
}

func parseStockRows(files map[string][]byte) []stockRow {
	codeMap := make(map[string]stockNameEntry)

	if data, ok := files[protocol.FileHsPy]; ok {
		for _, line := range strings.Split(string(protocol.UTF8ToGBK(data)), "\n") {
			line = strings.TrimRight(line, "\r")
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			fields := strings.Split(line, "|")
			if len(fields) < 3 {
				continue
			}
			exPrefix := strings.TrimSpace(fields[0])
			rawCode := strings.TrimSpace(fields[1])
			shortName := strings.TrimSpace(fields[2])
			if rawCode == "" || shortName == "" {
				continue
			}
			code := normalizeStockCode(exPrefix + rawCode)
			if code == "" {
				continue
			}
			if _, exists := codeMap[code]; !exists {
				codeMap[code] = stockNameEntry{name: shortName, exch: exchangeNameForCode(code, exPrefix)}
			}
		}
	}

	if profData, ok := files["profile.dat"]; ok && len(profData) > 0 {
		parseStockProfileNames(profData, codeMap)
	}

	stocks := make([]stockRow, 0, len(codeMap))
	for code, item := range codeMap {
		if code == "" || item.name == "" {
			continue
		}
		stocks = append(stocks, stockRow{
			Code:     code,
			Name:     item.name,
			Exchange: item.exch,
		})
	}
	sort.Slice(stocks, func(i, j int) bool {
		return stocks[i].Code < stocks[j].Code
	})
	return stocks
}

func parseStockProfileNames(profData []byte, codeMap map[string]stockNameEntry) {
	i := 0
	for i < len(profData)-12 {
		for ; i < len(profData)-10; i++ {
			if profData[i] == 0 && profData[i+1] == 0 && profData[i+2] == 0 && profData[i+3] <= 0x09 {
				break
			}
		}
		if i >= len(profData)-10 {
			break
		}
		code := int(binary.LittleEndian.Uint32(profData[i : i+4]))
		if code <= 0 || code > 999999 {
			i += 4
			continue
		}
		j := i + 4
		if j >= len(profData) || profData[j] != 0 {
			i = j
			continue
		}
		j++
		start := j
		for j < len(profData) && profData[j] != 0 {
			j++
		}
		if j > start {
			nameBytes := profData[start:j]
			name := string(nameBytes)
			if !utf8.Valid(nameBytes) {
				name = string(protocol.UTF8ToGBK(nameBytes))
			}
			codeStr := fmt.Sprintf("%06d", code)
			codeMap[codeStr] = stockNameEntry{
				name: name,
				exch: exchangeNameForCode(codeStr, ""),
			}
		}
		i = j + 1
	}
}

func replaceStocksDB(stocks []stockRow) error {
	agentDBWriteMu.Lock()
	defer agentDBWriteMu.Unlock()

	db, err := sql.Open("sqlite", stocksDBPath)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := ensureStocksSchema(db); err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM stocks`); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`
		INSERT INTO stocks (code, name, exchange, pinyin)
		VALUES (?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, stock := range stocks {
		if _, err := stmt.Exec(stock.Code, stock.Name, stock.Exchange, stock.Pinyin); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func ensureStocksSchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS stocks (
			code TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			exchange TEXT NOT NULL,
			pinyin TEXT
		);
		CREATE INDEX IF NOT EXISTS idx_stocks_name ON stocks(name);
		CREATE INDEX IF NOT EXISTS idx_stocks_pinyin ON stocks(pinyin);
	`)
	return err
}

func loadStocksCacheFromDB() (int, error) {
	db, err := sql.Open("sqlite", stocksDBPath)
	if err != nil {
		return 0, err
	}
	defer db.Close()

	rows, err := db.Query(`SELECT code, name, exchange, pinyin FROM stocks ORDER BY code`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	stocks := make([]stockRow, 0)
	for rows.Next() {
		var stock stockRow
		if err := rows.Scan(&stock.Code, &stock.Name, &stock.Exchange, &stock.Pinyin); err != nil {
			return 0, err
		}
		stocks = append(stocks, stock)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	stocksCacheMu.Lock()
	stocksCache = stocks
	stocksCacheMu.Unlock()
	return len(stocks), nil
}

func searchStocks(keyword string, limit int) ([]stockRow, error) {
	if keyword == "" {
		return nil, fmt.Errorf("缺少关键词")
	}
	if limit <= 0 || limit > 50 {
		limit = 50
	}

	stocksCacheMu.RLock()
	cache := stocksCache
	stocksCacheMu.RUnlock()

	if len(cache) == 0 {
		if _, err := loadStocksCacheFromDB(); err != nil {
			return nil, fmt.Errorf("股票名称库为空，请先刷新股票数据")
		}
		stocksCacheMu.RLock()
		cache = stocksCache
		stocksCacheMu.RUnlock()
	}

	kw := strings.ToUpper(keyword)
	results := make([]stockRow, 0, limit)
	for _, stock := range cache {
		if len(results) >= limit {
			break
		}
		if strings.Contains(strings.ToUpper(stock.Code), kw) ||
			strings.Contains(strings.ToUpper(stock.Name), kw) ||
			strings.Contains(stock.Pinyin, kw) {
			results = append(results, stock)
		}
	}
	return results, nil
}

func queryStockName(code string) string {
	stocksCacheMu.RLock()
	cache := stocksCache
	stocksCacheMu.RUnlock()
	for _, stock := range cache {
		if stock.Code == code {
			return stock.Name
		}
	}

	db, err := sql.Open("sqlite", stocksDBPath)
	if err != nil {
		return ""
	}
	defer db.Close()
	var name string
	if err := db.QueryRow(`SELECT name FROM stocks WHERE code = ?`, code).Scan(&name); err != nil {
		return ""
	}
	return name
}

func normalizeStockCode(rawCode string) string {
	code := strings.TrimSpace(rawCode)
	if len(code) > 6 {
		code = code[len(code)-6:]
	}
	if len(code) != 6 {
		return ""
	}
	return code
}

func exchangeNameForCode(code, rawExchange string) string {
	switch strings.ToLower(strings.TrimSpace(rawExchange)) {
	case "0", "sz":
		return "sz"
	case "2", "bj":
		return "bj"
	case "1", "sh":
		return "sh"
	}
	if strings.HasPrefix(code, "0") || strings.HasPrefix(code, "3") {
		return "sz"
	}
	if strings.HasPrefix(code, "4") || strings.HasPrefix(code, "8") {
		return "bj"
	}
	return "sh"
}

func pinyinForName(name string) string {
	var py strings.Builder
	for _, r := range name {
		if r > 127 {
			py.WriteByte(pinyinInitial(r))
		} else if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			py.WriteRune(r)
		}
	}
	return strings.ToUpper(py.String())
}

func pinyinInitial(r rune) byte {
	c := uint16(r)
	switch {
	case c >= 0x4E00 && c <= 0x554A:
		return 'A'
	case c >= 0x5555 && c <= 0x5A31:
		return 'B'
	case c >= 0x5A32 && c <= 0x5C0F:
		return 'C'
	case c >= 0x5C10 && c <= 0x5CE6:
		return 'D'
	case c >= 0x5CE7 && c <= 0x5E7F:
		return 'E'
	case c >= 0x5E80 && c <= 0x5FD9:
		return 'F'
	case c >= 0x5FDA && c <= 0x60CE:
		return 'G'
	case c >= 0x60CF && c <= 0x6234:
		return 'H'
	case c >= 0x6235 && c <= 0x6392:
		return 'J'
	case c >= 0x6393 && c <= 0x64B0:
		return 'K'
	case c >= 0x64B1 && c <= 0x65B9:
		return 'L'
	case c >= 0x65BA && c <= 0x66B4:
		return 'M'
	case c >= 0x66B5 && c <= 0x6797:
		return 'N'
	case c >= 0x6798 && c <= 0x6B21:
		return 'O'
	case c >= 0x6B22 && c <= 0x6DBC:
		return 'P'
	case c >= 0x6DBD && c <= 0x6E2F:
		return 'Q'
	case c >= 0x6E30 && c <= 0x6F4D:
		return 'R'
	case c >= 0x6F4E && c <= 0x71D5:
		return 'S'
	case c >= 0x71D6 && c <= 0x74B0:
		return 'T'
	case c >= 0x74B1 && c <= 0x76D7:
		return 'W'
	case c >= 0x76D8 && c <= 0x78B3:
		return 'X'
	case c >= 0x78B4 && c <= 0x79C1:
		return 'Y'
	case c >= 0x79C2 && c <= 0x9FFF:
		return 'Z'
	}
	return 0
}

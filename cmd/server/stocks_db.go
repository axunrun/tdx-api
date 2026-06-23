package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/injoyai/tdx"
	_ "modernc.org/sqlite"
)

var (
	stocksDB   *sql.DB
	stocksLock sync.RWMutex
)

type stockRow struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	Exchange string `json:"exchange"`
	Pinyin   string `json:"pinyin,omitempty"`
}

func initStocksDB() {
	dbPath := os.Getenv("STOCKS_DB_PATH")
	if dbPath == "" {
		dbPath = filepath.Join(os.TempDir(), "tdx-stocks.db")
	}
	os.MkdirAll(filepath.Dir(dbPath), 0755)
	var err error
	stocksDB, err = sql.Open("sqlite", dbPath)
	if err != nil {
		log.Printf("⚠️ SQLite 打开失败: %v", err)
		return
	}
	_, err = stocksDB.Exec(`
		CREATE TABLE IF NOT EXISTS stocks (
			code     TEXT PRIMARY KEY,
			name     TEXT NOT NULL DEFAULT '',
			exchange TEXT NOT NULL DEFAULT 'sh',
			pinyin   TEXT NOT NULL DEFAULT ''
		);
		CREATE INDEX IF NOT EXISTS idx_stocks_name  ON stocks(name);
		CREATE INDEX IF NOT EXISTS idx_stocks_pinyin ON stocks(pinyin);
	`)
	if err != nil {
		log.Printf("⚠️ SQLite 建表失败: %v", err)
		return
	}
	log.Printf("✅ SQLite 股票库就绪 (%s)", dbPath)
}

func refreshStocks(c *tdx.Client) (int, error) {
	if c == nil {
		return 0, fmt.Errorf("未连接通达信")
	}
	if stocksDB == nil {
		return 0, fmt.Errorf("SQLite 未就绪")
	}
	log.Println("🔄 开始全量更新股票数据...")

	stockNameMap = nil
	nameMap := loadStockNames(c)
	if len(nameMap) == 0 {
		return 0, fmt.Errorf("未能获取股票名称数据")
	}

	tx, err := stocksDB.Begin()
	if err != nil {
		return 0, fmt.Errorf("事务开始失败: %v", err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM stocks"); err != nil {
		return 0, fmt.Errorf("清空失败: %v", err)
	}
	stmt, err := tx.Prepare("INSERT INTO stocks (code, name, exchange, pinyin) VALUES (?, ?, ?, ?)")
	if err != nil {
		return 0, fmt.Errorf("预编译失败: %v", err)
	}
	defer stmt.Close()

	count := 0
	for code, name := range nameMap {
		if len(code) != 6 || code == "" || name == "" {
			continue
		}
		ex := "sh"
		switch code[0] {
		case '0', '3':
			ex = "sz"
		case '4', '8':
			ex = "bj"
		}
		pinyin := pinyinForName(name)
		if _, err := stmt.Exec(code, name, ex, pinyin); err != nil {
			log.Printf("⚠️ 插入 %s 失败: %v", code, err)
			continue
		}
		count++
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("提交事务失败: %v", err)
	}
	log.Printf("✅ 股票数据更新完成: %d 条", count)
	return count, nil
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

func searchStocks(keyword string, limit int) ([]stockRow, error) {
	if stocksDB == nil {
		return nil, fmt.Errorf("SQLite 未就绪")
	}
	if keyword == "" {
		return nil, fmt.Errorf("缺少关键词")
	}
	if limit <= 0 || limit > 50 {
		limit = 50
	}
	kw := strings.ToUpper(keyword)
	like := "%" + kw + "%"

	stocksLock.RLock()
	defer stocksLock.RUnlock()

	rows, err := stocksDB.Query(
		`SELECT DISTINCT code, name, exchange, pinyin FROM stocks
		 WHERE code LIKE ? OR UPPER(name) LIKE ? OR UPPER(pinyin) LIKE ?
		 LIMIT ?`,
		like, like, like, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []stockRow
	for rows.Next() {
		var r stockRow
		if err := rows.Scan(&r.Code, &r.Name, &r.Exchange, &r.Pinyin); err != nil {
			continue
		}
		results = append(results, r)
	}
	return results, nil
}

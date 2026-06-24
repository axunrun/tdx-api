package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/injoyai/tdx"
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

func initStocksDB() {
	stocksDBPath = os.Getenv("STOCKS_DB_PATH")
	if stocksDBPath == "" {
		stocksDBPath = filepath.Join(os.TempDir(), "tdx-stocks.json")
	}
	// 尝试从文件加载已有缓存
	if data, err := os.ReadFile(stocksDBPath); err == nil {
		var cached []stockRow
		if json.Unmarshal(data, &cached) == nil && len(cached) > 0 {
			stocksCacheMu.Lock()
			stocksCache = cached
			stocksCacheMu.Unlock()
			log.Printf("✅ 股票库已从缓存加载: %d 条 (%s)", len(cached), stocksDBPath)
			return
		}
	}
	log.Printf("📂 股票缓存文件不存在或无效，点击「更新股票数据」初始化 (%s)", stocksDBPath)
}

func refreshStocks(c *tdx.Client) (int, error) {
	if c == nil {
		return 0, fmt.Errorf("未连接通达信")
	}
	log.Println("🔄 开始全量更新股票数据...")

	// 清除缓存，强制重新下载
	stockNameMap = nil
	nameMap := loadStockNames(c)
	if len(nameMap) == 0 {
		return 0, fmt.Errorf("未能获取股票名称数据")
	}

	var stocks []stockRow
	for code, name := range nameMap {
		if code == "" || name == "" {
			continue
		}
		// 标准化 code: hspy.dat 可能带交易所前缀(7位), profile.dat 是纯6位
		if len(code) > 6 {
			code = code[len(code)-6:]
		}
		if len(code) != 6 {
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
		stocks = append(stocks, stockRow{
			Code:     code,
			Name:     name,
			Exchange: ex,
			Pinyin:   pinyin,
		})
	}

	// 写入 JSON 文件
	data, err := json.Marshal(stocks)
	if err != nil {
		return 0, fmt.Errorf("序列化失败: %v", err)
	}
	if err := os.WriteFile(stocksDBPath, data, 0644); err != nil {
		return 0, fmt.Errorf("写入缓存文件失败: %v", err)
	}

	stocksCacheMu.Lock()
	stocksCache = stocks
	stocksCacheMu.Unlock()

	log.Printf("✅ 股票数据更新完成: %d 条 (%s)", len(stocks), stocksDBPath)
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
		return nil, fmt.Errorf("股票库为空，请先点击「更新股票数据」")
	}

	kw := strings.ToUpper(keyword)
	var results []stockRow
	for _, s := range cache {
		if len(results) >= limit {
			break
		}
		if strings.Contains(strings.ToUpper(s.Code), kw) ||
			strings.Contains(strings.ToUpper(s.Name), kw) ||
			strings.Contains(s.Pinyin, kw) {
			results = append(results, s)
		}
	}
	return results, nil
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

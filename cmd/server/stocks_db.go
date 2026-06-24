package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"

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

	// 直接下载 zhb.zip，不走 loadStockNames 缓存
	files, err := c.GetZHBFiles()
	if err != nil {
		return 0, fmt.Errorf("下载 zhb.zip 失败: %v", err)
	}

	// code6位→(exchange, name)
	type entry struct{ name, exch string }
	codeMap := make(map[string]entry)

	// 1. 解析 hspy.dat → 拼音缩写 + 交易所前缀 → 6位code
	if data, ok := files[protocol.FileHsPy]; ok {
		for _, line := range strings.Split(string(protocol.UTF8ToGBK(data)), "\n") {
			line = strings.TrimRight(line, "\r")
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			f := strings.Split(line, "|")
			if len(f) < 3 {
				continue
			}
			exPrefix := strings.TrimSpace(f[0])
			rawCode := strings.TrimSpace(f[1])
			shortName := strings.TrimSpace(f[2])
			if rawCode == "" || shortName == "" {
				continue
			}
			// 标准化为6位代码
			if len(rawCode) > 6 {
				rawCode = rawCode[len(rawCode)-6:]
			}
			if len(rawCode) != 6 {
				continue
			}
			ex := "sh"
			if exPrefix == "0" || exPrefix == "sz" {
				ex = "sz"
			} else if exPrefix == "2" || exPrefix == "bj" {
				ex = "bj"
			}
			// hspy 的名字优先用 profile 覆盖，这里作为备选
			if _, exists := codeMap[rawCode]; !exists {
				codeMap[rawCode] = entry{name: shortName, exch: ex}
			}
		}
	}

	// 2. 解析 profile.dat → 中文全名 → 覆盖 codeMap
	if profData, ok := files["profile.dat"]; ok && len(profData) > 0 {
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
				var name string
				// 尝试 UTF8 直读，否则 GBK 解码
				if !utf8.Valid(nameBytes) {
					name = string(protocol.UTF8ToGBK(nameBytes))
				} else {
					name = string(nameBytes)
				}
				codeStr := fmt.Sprintf("%06d", code)
				// 始终用 profile 的名字（中文全名）
				ex := "sh"
				if codeStr[0] == '0' || codeStr[0] == '3' {
					ex = "sz"
				} else if codeStr[0] == '4' || codeStr[0] == '8' {
					ex = "bj"
				}
				codeMap[codeStr] = entry{name: name, exch: ex}
			}
			i = j + 1
		}
	}

	// 3. 序列化到 JSON
	var stocks []stockRow
	for code, e := range codeMap {
		if code == "" || e.name == "" {
			continue
		}
		stocks = append(stocks, stockRow{
			Code:     code,
			Name:     e.name,
			Exchange: e.exch,
			Pinyin:   pinyinForName(e.name),
		})
	}

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
	results := make([]stockRow, 0)
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

package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/glebarez/go-sqlite"
	"github.com/injoyai/tdx"
	"github.com/injoyai/tdx/protocol"
)

var blocksDBPath string

type blockIndexSpec struct {
	file     string
	typ      string
	typeName string
}

type blockMembershipRow struct {
	code        string
	blockType   string
	typeName    string
	blockName   string
	blockIndex  string
	memberCount int
	sourceFile  string
}

var blockIndexSpecs = []blockIndexSpec{
	{protocol.BlockFileGN, "concept", "概念板块"},
	{protocol.BlockFileFG, "style_region", "地域/风格板块"},
	{protocol.BlockFileZS, "index", "指数板块"},
}

func initBlocksDB(c *tdx.Client) {
	blocksDBPath = agentFeatureDBPath("BLOCKS_DB_PATH")
	if err := os.MkdirAll(filepath.Dir(blocksDBPath), 0755); err != nil {
		log.Printf("⚠️ 板块索引库目录创建失败: %v", err)
		return
	}
	count, err := refreshBlocksDB(c)
	if err != nil {
		log.Printf("⚠️ 板块索引库刷新失败: %v", err)
		return
	}
	log.Printf("✅ 板块索引库刷新完成: %d 条 (%s)", count, blocksDBPath)
}

func refreshBlocksDB(c *tdx.Client) (int, error) {
	if c == nil {
		return 0, fmt.Errorf("TDX客户端未连接")
	}

	rows, err := fetchBlockMembershipRows(c)
	if err != nil {
		return 0, err
	}

	agentDBWriteMu.Lock()
	defer agentDBWriteMu.Unlock()

	db, err := sql.Open("sqlite", blocksDBPath)
	if err != nil {
		return 0, err
	}
	defer db.Close()

	if err := ensureBlocksSchema(db); err != nil {
		return 0, err
	}

	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM block_memberships`); err != nil {
		return 0, err
	}

	stmt, err := tx.Prepare(`
		INSERT INTO block_memberships (
			code, block_type, block_type_name, block_name, block_index,
			member_count, source_file, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	updatedAt := time.Now().Format(time.RFC3339)
	for _, row := range rows {
		if _, err := stmt.Exec(
			row.code,
			row.blockType,
			row.typeName,
			row.blockName,
			row.blockIndex,
			row.memberCount,
			row.sourceFile,
			updatedAt,
		); err != nil {
			return 0, err
		}
	}

	if _, err := tx.Exec(`
		INSERT INTO block_index_meta (key, value)
		VALUES ('updated_at', ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, updatedAt); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(rows), nil
}

func fetchBlockMembershipRows(c *tdx.Client) ([]blockMembershipRow, error) {
	rows := make([]blockMembershipRow, 0)
	for _, spec := range blockIndexSpecs {
		blocks, err := getBlockDataWithRetry(c, spec.file, 3)
		if err != nil {
			return nil, fmt.Errorf("获取%s失败: %w", spec.file, err)
		}
		for _, block := range blocks {
			if block == nil {
				continue
			}
			memberCount := len(block.Codes)
			for _, rawCode := range block.Codes {
				code := normalizeBlockStockCode(rawCode)
				if code == "" {
					continue
				}
				rows = append(rows, blockMembershipRow{
					code:        code,
					blockType:   spec.typ,
					typeName:    spec.typeName,
					blockName:   block.Name,
					blockIndex:  block.Index,
					memberCount: memberCount,
					sourceFile:  spec.file,
				})
			}
		}
	}
	return rows, nil
}

func getBlockDataWithRetry(c *tdx.Client, file string, retries int) ([]*protocol.Block, error) {
	if retries <= 0 {
		retries = 1
	}
	var lastErr error
	for i := 0; i < retries; i++ {
		blocks, err := c.GetBlockDataWithIndex(file)
		if err == nil {
			return blocks, nil
		}
		lastErr = err
		time.Sleep(time.Duration(i+1) * time.Second)
	}
	return nil, lastErr
}

func ensureBlocksSchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS block_memberships (
			code TEXT NOT NULL,
			block_type TEXT NOT NULL,
			block_type_name TEXT NOT NULL,
			block_name TEXT NOT NULL,
			block_index TEXT,
			member_count INTEGER NOT NULL,
			source_file TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (code, block_type, block_name)
		);
		CREATE INDEX IF NOT EXISTS idx_block_memberships_code
			ON block_memberships(code);
		CREATE INDEX IF NOT EXISTS idx_block_memberships_type
			ON block_memberships(block_type);
		CREATE TABLE IF NOT EXISTS block_index_meta (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);
	`)
	return err
}

func queryStockBlocks(code string) ([]AgentBriefBlock, error) {
	if blocksDBPath == "" {
		return nil, fmt.Errorf("板块索引库未初始化")
	}

	db, err := sql.Open("sqlite", blocksDBPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT block_type, block_type_name, block_name, block_index, member_count
		FROM block_memberships
		WHERE code = ?
		ORDER BY
			CASE block_type
				WHEN 'concept' THEN 1
				WHEN 'style_region' THEN 2
				WHEN 'index' THEN 3
				ELSE 9
			END,
			block_name
	`, code)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	blocks := make([]AgentBriefBlock, 0)
	for rows.Next() {
		var block AgentBriefBlock
		if err := rows.Scan(
			&block.Type,
			&block.TypeName,
			&block.Name,
			&block.IndexCode,
			&block.MemberCount,
		); err != nil {
			return nil, err
		}
		block.Meaning = "该股所属板块摘要，可用于后续抓取板块指数、板块成分股强弱和题材归因。"
		blocks = append(blocks, block)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return blocks, nil
}

func querySectorMemberStocks(blockType, blockName string) ([]stockRow, error) {
	if blocksDBPath == "" {
		return nil, fmt.Errorf("板块索引库未初始化")
	}
	db, err := sql.Open("sqlite", blocksDBPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT code
		FROM block_memberships
		WHERE block_type = ? AND block_name = ?
		ORDER BY code
	`, blockType, blockName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := make([]stockRow, 0)
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		stock, err := findStockRow(code)
		if err != nil {
			stock = stockRow{Code: code, Exchange: exchangeNameForCode(code, "")}
		}
		members = append(members, stock)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return members, nil
}

func querySectorMemberSets(blockType string) ([]agentSectorMemberSet, error) {
	if blocksDBPath == "" {
		return nil, fmt.Errorf("板块索引库未初始化")
	}
	db, err := sql.Open("sqlite", blocksDBPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT block_type, block_type_name, block_name, block_index, member_count, code
		FROM block_memberships
		WHERE block_type = ?
		ORDER BY block_name, code
	`, blockType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stockByCode := cachedStockRowsByCode()
	setByName := make(map[string]*agentSectorMemberSet)
	order := make([]string, 0)
	for rows.Next() {
		var block AgentBriefBlock
		var code string
		if err := rows.Scan(
			&block.Type,
			&block.TypeName,
			&block.Name,
			&block.IndexCode,
			&block.MemberCount,
			&code,
		); err != nil {
			return nil, err
		}
		key := block.Type + "\x00" + block.Name
		set := setByName[key]
		if set == nil {
			order = append(order, key)
			set = &agentSectorMemberSet{Block: block}
			setByName[key] = set
		}
		stock, ok := stockByCode[code]
		if !ok {
			stock = stockRow{Code: code, Exchange: exchangeNameForCode(code, "")}
		}
		set.Members = append(set.Members, stock)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sets := make([]agentSectorMemberSet, 0, len(order))
	for _, key := range order {
		sets = append(sets, *setByName[key])
	}
	return sets, nil
}

func cachedStockRowsByCode() map[string]stockRow {
	stocksCacheMu.RLock()
	cache := stocksCache
	stocksCacheMu.RUnlock()
	if len(cache) == 0 && stocksDBPath != "" {
		if _, err := loadStocksCacheFromDB(); err == nil {
			stocksCacheMu.RLock()
			cache = stocksCache
			stocksCacheMu.RUnlock()
		}
	}

	stockByCode := make(map[string]stockRow, len(cache))
	for _, stock := range cache {
		stockByCode[stock.Code] = stock
	}
	return stockByCode
}

func normalizeBlockStockCode(rawCode string) string {
	code := strings.TrimSpace(rawCode)
	if len(code) < 6 {
		return ""
	}
	return code[len(code)-6:]
}

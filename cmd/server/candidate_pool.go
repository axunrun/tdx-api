package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/glebarez/go-sqlite"
)

type CandidatePoolItem struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	AddedDate string `json:"addedDate"`
	Reason    string `json:"reason"`
	Themes    string `json:"themes"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type CandidatePoolUpsertRequest struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	AddedDate string `json:"addedDate"`
	Reason    string `json:"reason"`
	Themes    string `json:"themes"`
}

func candidatePoolDBPath() string {
	return agentFeatureDBPath("CANDIDATE_POOL_DB_PATH")
}

func openCandidatePoolDB(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if err := ensureCandidatePoolSchema(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func ensureCandidatePoolSchema(db *sql.DB) error {
	agentDBWriteMu.Lock()
	defer agentDBWriteMu.Unlock()

	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS candidate_pool (
			code TEXT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			added_date TEXT NOT NULL,
			reason TEXT NOT NULL,
			themes TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_candidate_pool_added_date
			ON candidate_pool(added_date);
	`)
	return err
}

func upsertCandidatePoolItem(req CandidatePoolUpsertRequest) (CandidatePoolItem, error) {
	code := normalizeStockCode(req.Code)
	if code == "" {
		return CandidatePoolItem{}, fmt.Errorf("code is required")
	}
	if req.Reason == "" {
		return CandidatePoolItem{}, fmt.Errorf("reason is required")
	}
	addedDate, err := normalizeCandidatePoolDate(req.AddedDate)
	if err != nil {
		return CandidatePoolItem{}, err
	}
	name := req.Name
	if name == "" {
		name = queryStockName(code)
	}
	now := time.Now().Format(time.RFC3339)

	db, err := openCandidatePoolDB(candidatePoolDBPath())
	if err != nil {
		return CandidatePoolItem{}, err
	}
	defer db.Close()

	agentDBWriteMu.Lock()
	defer agentDBWriteMu.Unlock()
	_, err = db.Exec(`
		INSERT INTO candidate_pool (
			code, name, added_date, reason, themes, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(code) DO UPDATE SET
			name = excluded.name,
			added_date = excluded.added_date,
			reason = excluded.reason,
			themes = excluded.themes,
			updated_at = excluded.updated_at
	`, code, name, addedDate, req.Reason, req.Themes, now, now)
	if err != nil {
		return CandidatePoolItem{}, err
	}
	return getCandidatePoolItemWithDB(db, code)
}

func getCandidatePoolItem(code string) (CandidatePoolItem, error) {
	code = normalizeStockCode(code)
	if code == "" {
		return CandidatePoolItem{}, fmt.Errorf("code is required")
	}
	db, err := openCandidatePoolDB(candidatePoolDBPath())
	if err != nil {
		return CandidatePoolItem{}, err
	}
	defer db.Close()
	return getCandidatePoolItemWithDB(db, code)
}

func getCandidatePoolItemWithDB(db *sql.DB, code string) (CandidatePoolItem, error) {
	var item CandidatePoolItem
	err := db.QueryRow(`
		SELECT code, name, added_date, reason, themes, created_at, updated_at
		FROM candidate_pool
		WHERE code = ?
	`, code).Scan(
		&item.Code,
		&item.Name,
		&item.AddedDate,
		&item.Reason,
		&item.Themes,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return CandidatePoolItem{}, err
	}
	return item, nil
}

func listCandidatePoolItems(limit int) ([]CandidatePoolItem, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	db, err := openCandidatePoolDB(candidatePoolDBPath())
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT code, name, added_date, reason, themes, created_at, updated_at
		FROM candidate_pool
		ORDER BY updated_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []CandidatePoolItem{}
	for rows.Next() {
		var item CandidatePoolItem
		if err := rows.Scan(
			&item.Code,
			&item.Name,
			&item.AddedDate,
			&item.Reason,
			&item.Themes,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func removeCandidatePoolItem(code string) error {
	code = normalizeStockCode(code)
	if code == "" {
		return fmt.Errorf("code is required")
	}
	db, err := openCandidatePoolDB(candidatePoolDBPath())
	if err != nil {
		return err
	}
	defer db.Close()

	agentDBWriteMu.Lock()
	defer agentDBWriteMu.Unlock()
	result, err := db.Exec(`DELETE FROM candidate_pool WHERE code = ?`, code)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func normalizeCandidatePoolDate(value string) (string, error) {
	if value == "" {
		return time.Now().Format("2006-01-02"), nil
	}
	if len(value) == 8 {
		value = value[:4] + "-" + value[4:6] + "-" + value[6:]
	}
	if _, err := time.Parse("2006-01-02", value); err != nil {
		return "", fmt.Errorf("addedDate must be YYYY-MM-DD or YYYYMMDD")
	}
	return value, nil
}

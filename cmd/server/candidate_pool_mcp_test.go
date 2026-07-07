package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCandidatePoolMCPToolSchema(t *testing.T) {
	tool := findPaperMCPTool(t, "tdx_candidate_pool")
	properties := tool.InputSchema["properties"].(map[string]any)

	assertMCPEnum(t, properties, "action", "add", "list", "get", "remove")
	code := properties["code"].(map[string]any)
	if code["pattern"] != `^\d{6}$` {
		t.Fatalf("code schema = %+v", code)
	}
	validUntil := properties["validUntil"].(map[string]any)
	if validUntil["pattern"] != `^(\d{4}-\d{2}-\d{2}|\d{8})$` {
		t.Fatalf("validUntil schema = %+v", validUntil)
	}
	for _, name := range []string{"code", "validUntil", "reason", "themes", "confirm"} {
		property := properties[name].(map[string]any)
		if property["description"] == "" {
			t.Fatalf("%s description is empty", name)
		}
	}
	themes := properties["themes"].(map[string]any)
	if !strings.Contains(themes["description"].(string), "板块") ||
		!strings.Contains(themes["description"].(string), "题材") {
		t.Fatalf("themes schema = %+v", themes)
	}
	if len(tool.InputSchema["allOf"].([]map[string]any)) == 0 {
		t.Fatal("candidate pool conditional schema missing")
	}
	output := tool.OutputSchema
	if !strings.Contains(output["description"].(string), "structuredContent") {
		t.Fatalf("output schema = %+v", output)
	}
}

func TestCandidatePoolMCPAddListGetRemove(t *testing.T) {
	withCandidatePoolDB(t)

	addResult, err := callMCPTool(mustMCPParams(t, "tdx_candidate_pool", map[string]any{
		"action":     "add",
		"code":       "603063",
		"name":       "禾望电气",
		"addedDate":  "20260707",
		"validUntil": "2026-08-07",
		"reason":     "储能逆变器候选",
		"themes":     "储能, 风电, 电力设备",
		"confirm":    true,
	}))
	if err != nil {
		t.Fatal(err)
	}
	item := addResult["structuredContent"].(map[string]any)["item"].(CandidatePoolItem)
	if item.Code != "603063" || item.AddedDate != "2026-07-07" ||
		item.ValidUntil != "2026-08-07" ||
		!strings.Contains(item.Themes, "储能") {
		t.Fatalf("item = %+v", item)
	}

	listResult, err := callMCPTool(mustMCPParams(t, "tdx_candidate_pool", map[string]any{
		"action": "list",
	}))
	if err != nil {
		t.Fatal(err)
	}
	listData := listResult["structuredContent"].(map[string]any)
	if listData["count"] != 1 {
		t.Fatalf("list data = %+v", listData)
	}

	getResult, err := callMCPTool(mustMCPParams(t, "tdx_candidate_pool", map[string]any{
		"action": "get",
		"code":   "603063",
	}))
	if err != nil {
		t.Fatal(err)
	}
	got := getResult["structuredContent"].(map[string]any)["item"].(CandidatePoolItem)
	if got.Reason != "储能逆变器候选" {
		t.Fatalf("got = %+v", got)
	}

	if got.ValidUntil != "2026-08-07" {
		t.Fatalf("got = %+v", got)
	}

	if _, err := callMCPTool(mustMCPParams(t, "tdx_candidate_pool", map[string]any{
		"action":  "remove",
		"code":    "603063",
		"confirm": true,
	})); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", candidatePoolDBPath())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	assertPaperRowCount(t, db, "candidate_pool", 0)
}

func TestCandidatePoolMCPRequiresConfirmForWrites(t *testing.T) {
	withCandidatePoolDB(t)

	if _, err := callMCPTool(mustMCPParams(t, "tdx_candidate_pool", map[string]any{
		"action": "add",
		"code":   "603063",
		"reason": "test",
	})); err == nil {
		t.Fatal("add without confirm error = nil, want error")
	}
}

func TestCandidatePoolSchemaMigratesValidUntil(t *testing.T) {
	path := filepath.Join(t.TempDir(), "candidate.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		CREATE TABLE candidate_pool (
			code TEXT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			added_date TEXT NOT NULL,
			reason TEXT NOT NULL,
			themes TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)
	`); err != nil {
		t.Fatal(err)
	}
	db.Close()

	opened, err := openCandidatePoolDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
	if _, err := opened.Exec(`
		INSERT INTO candidate_pool (
			code, name, added_date, valid_until, reason, themes, created_at, updated_at
		) VALUES ('603063', '', '2026-07-07', '2026-08-07', 'reason', '', 'now', 'now')
	`); err != nil {
		t.Fatal(err)
	}
}

func withCandidatePoolDB(t *testing.T) {
	t.Helper()

	old := os.Getenv("CANDIDATE_POOL_DB_PATH")
	path := filepath.Join(t.TempDir(), "candidate.sqlite")
	if err := os.Setenv("CANDIDATE_POOL_DB_PATH", path); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if old == "" {
			_ = os.Unsetenv("CANDIDATE_POOL_DB_PATH")
			return
		}
		_ = os.Setenv("CANDIDATE_POOL_DB_PATH", old)
	})
}

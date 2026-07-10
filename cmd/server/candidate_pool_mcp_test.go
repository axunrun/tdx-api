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

	for _, want := range []string{
		"上下文协议",
		"reason 只写为什么入池",
		"buySignalTier 写能不能买",
		"list/get 是只读操作",
		"按 code 加入或更新",
		"updatedAt desc",
		"remove 是硬删除",
	} {
		if !strings.Contains(tool.Description, want) {
			t.Fatalf("tool description = %s, missing %s", tool.Description, want)
		}
	}
	assertMCPEnum(t, properties, "action", "add", "list", "get", "remove")
	code := properties["code"].(map[string]any)
	if code["pattern"] != `^\d{6}$` {
		t.Fatalf("code schema = %+v", code)
	}
	validUntil := properties["validUntil"].(map[string]any)
	if validUntil["pattern"] != `^(\d{4}-\d{2}-\d{2}|\d{8})$` {
		t.Fatalf("validUntil schema = %+v", validUntil)
	}
	assertMCPEnum(t, properties, "buySignalTier", "observe_only", "setup_ready", "trade_eligible")
	buySignalTier := properties["buySignalTier"].(map[string]any)
	if buySignalTier["default"] != "observe_only" ||
		!strings.Contains(buySignalTier["description"].(string), "不传默认 observe_only") ||
		!strings.Contains(buySignalTier["description"].(string), "可购买池") ||
		!strings.Contains(buySignalTier["description"].(string), "交易窗口内独立判断买入") {
		t.Fatalf("buySignalTier schema = %+v", buySignalTier)
	}
	confirm := properties["confirm"].(map[string]any)
	if !strings.Contains(confirm["description"].(string), "list/get 是只读操作") {
		t.Fatalf("confirm schema = %+v", confirm)
	}
	reason := properties["reason"].(map[string]any)
	if !strings.Contains(reason["description"].(string), "不要混写能不能买") {
		t.Fatalf("reason schema = %+v", reason)
	}
	triggerCondition := properties["triggerCondition"].(map[string]any)
	if !strings.Contains(triggerCondition["description"].(string), "升级到 setup_ready") ||
		!strings.Contains(triggerCondition["description"].(string), "setup_ready 或 trade_eligible") ||
		!strings.Contains(triggerCondition["description"].(string), "可下单买入前必须满足") {
		t.Fatalf("triggerCondition schema = %+v", triggerCondition)
	}
	invalidationCondition := properties["invalidationCondition"].(map[string]any)
	if !strings.Contains(invalidationCondition["description"].(string), "失效/降档/移除条件") ||
		!strings.Contains(invalidationCondition["description"].(string), "覆盖旧 invalidationCondition") {
		t.Fatalf("invalidationCondition schema = %+v", invalidationCondition)
	}
	if !strings.Contains(validUntil["description"].(string), "next_review_date / deep_report_valid_until") ||
		!strings.Contains(validUntil["description"].(string), "重新分析刷新") {
		t.Fatalf("validUntil schema = %+v", validUntil)
	}
	limit := properties["limit"].(map[string]any)
	if !strings.Contains(limit["description"].(string), "updatedAt desc") {
		t.Fatalf("limit schema = %+v", limit)
	}
	for _, name := range []string{
		"code",
		"validUntil",
		"buySignalTier",
		"triggerCondition",
		"invalidationCondition",
		"reason",
		"themes",
		"confirm",
	} {
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
	outputProperties := output["properties"].(map[string]any)
	structured := outputProperties["structuredContent"].(map[string]any)
	if !strings.Contains(structured["description"].(string), "硬删除") {
		t.Fatalf("structuredContent schema = %+v", structured)
	}
	item := structured["properties"].(map[string]any)["item"].(map[string]any)
	if !strings.Contains(item["description"].(string), "字段协议") {
		t.Fatalf("item schema = %+v", item)
	}
	itemProperties := item["properties"].(map[string]any)
	assertMCPEnum(t, itemProperties, "buySignalTier", "observe_only", "setup_ready", "trade_eligible")
	itemReason := itemProperties["reason"].(map[string]any)
	if !strings.Contains(itemReason["description"].(string), "不承载买入许可") {
		t.Fatalf("item reason schema = %+v", itemReason)
	}
	itemBuySignalTier := itemProperties["buySignalTier"].(map[string]any)
	if !strings.Contains(itemBuySignalTier["description"].(string), "可购买池但需等待 triggerCondition") ||
		!strings.Contains(itemBuySignalTier["description"].(string), "交易员在窗口内独立判断买入") {
		t.Fatalf("item buySignalTier schema = %+v", itemBuySignalTier)
	}
	itemTriggerCondition := itemProperties["triggerCondition"].(map[string]any)
	if !strings.Contains(itemTriggerCondition["description"].(string), "升级到 setup_ready") ||
		!strings.Contains(itemTriggerCondition["description"].(string), "可下单买入前必须满足") {
		t.Fatalf("item triggerCondition schema = %+v", itemTriggerCondition)
	}
	itemInvalidationCondition := itemProperties["invalidationCondition"].(map[string]any)
	if !strings.Contains(itemInvalidationCondition["description"].(string), "失效/降档/移除条件") {
		t.Fatalf("item invalidationCondition schema = %+v", itemInvalidationCondition)
	}
	itemValidUntil := itemProperties["validUntil"].(map[string]any)
	if !strings.Contains(itemValidUntil["description"].(string), "深度分析报告刷新日期") ||
		!strings.Contains(itemValidUntil["description"].(string), "重新分析刷新") {
		t.Fatalf("item validUntil schema = %+v", itemValidUntil)
	}
}

func TestCandidatePoolMCPAddListGetRemove(t *testing.T) {
	withCandidatePoolDB(t)

	addResult, err := callMCPTool(mustMCPParams(t, "tdx_candidate_pool", map[string]any{
		"action":                "add",
		"code":                  "603063",
		"name":                  "禾望电气",
		"addedDate":             "20260707",
		"validUntil":            "2026-08-07",
		"buySignalTier":         "setup_ready",
		"triggerCondition":      "缩量企稳后放量突破",
		"invalidationCondition": "跌破20日线",
		"reason":                "储能逆变器候选",
		"themes":                "储能, 风电, 电力设备",
		"confirm":               true,
	}))
	if err != nil {
		t.Fatal(err)
	}
	item := addResult["structuredContent"].(map[string]any)["item"].(CandidatePoolItem)
	if item.Code != "603063" || item.AddedDate != "2026-07-07" ||
		item.ValidUntil != "2026-08-07" ||
		item.BuySignalTier != "setup_ready" ||
		item.TriggerCondition != "缩量企稳后放量突破" ||
		item.InvalidationCondition != "跌破20日线" ||
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

	if got.ValidUntil != "2026-08-07" || got.BuySignalTier != "setup_ready" {
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

func TestCandidatePoolSchemaMigratesStatusColumns(t *testing.T) {
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
			code, name, added_date, valid_until, buy_signal_tier, trigger_condition,
			invalidation_condition, reason, themes, created_at, updated_at
		) VALUES (
			'603063', '', '2026-07-07', '2026-08-07', 'observe_only',
			'trigger', 'invalid', 'reason', '', 'now', 'now'
		)
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

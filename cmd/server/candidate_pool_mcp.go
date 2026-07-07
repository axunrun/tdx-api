package main

import (
	"encoding/json"
	"errors"
	"fmt"
)

func candidatePoolMCPTools() []mcpTool {
	tool := newMCPTool(
		"tdx_candidate_pool",
		"SQLite 选股候选池工具。用于让 Agent 记录备选个股的代码、名称、添加日期、添加理由，以及板块/概念/题材；add/remove 会写入持久化数据，必须传 confirm=true。",
		"",
		nil,
		requiredEnum("action", "操作：add 加入或更新、list 列表、get 详情、remove 移除。", "add", "list", "get", "remove"),
		optionalString("code", "股票代码；add/get/remove 时必填。"),
		optionalString("name", "股票名称；add 时可选，不传则尽量从股票名称库自动补全。"),
		optionalDateString("addedDate", "添加日期，YYYY-MM-DD 或 YYYYMMDD；add 时可选，不传默认今天。"),
		optionalString("reason", "添加理由；add 时必填，用于记录 Agent 选入候选池的依据。"),
		optionalString("themes", "板块、概念、题材字段；可填写逗号分隔文本，例如 AI算力, 数据中心, 北交所。"),
		optionalIntegerDefault("limit", "list 返回条数，默认 50，最大 200。", 50, 1, 200),
		optionalBool("confirm", "add/remove 等写入候选池的操作必须为 true。"),
	)
	tool.InputSchema["properties"].(map[string]any)["code"].(map[string]any)["pattern"] = `^\d{6}$`
	tool.OutputSchema = candidatePoolOutputSchema()
	tool.InputSchema["allOf"] = []map[string]any{
		{
			"if": map[string]any{
				"properties": map[string]any{"action": map[string]any{"const": "add"}},
				"required":   []string{"action"},
			},
			"then": map[string]any{"required": []string{"code", "reason"}},
		},
		{
			"if": map[string]any{
				"properties": map[string]any{"action": map[string]any{"enum": []string{"get", "remove"}}},
				"required":   []string{"action"},
			},
			"then": map[string]any{"required": []string{"code"}},
		},
		{
			"if": map[string]any{
				"properties": map[string]any{"action": map[string]any{"enum": []string{"add", "remove"}}},
				"required":   []string{"action"},
			},
			"then": map[string]any{
				"properties": map[string]any{"confirm": map[string]any{"const": true}},
				"required":   []string{"confirm"},
			},
		},
	}
	return []mcpTool{tool}
}

func candidatePoolOutputSchema() map[string]any {
	item := map[string]any{
		"type":        "object",
		"description": "候选池单条记录。",
		"properties": map[string]any{
			"code": map[string]any{
				"type":        "string",
				"description": "6 位股票代码。",
			},
			"name": map[string]any{
				"type":        "string",
				"description": "股票名称；add 时未传则尽量由股票名称库补全。",
			},
			"addedDate": map[string]any{
				"type":        "string",
				"description": "添加日期，服务端统一返回 YYYY-MM-DD。",
			},
			"reason": map[string]any{
				"type":        "string",
				"description": "Agent 或用户加入候选池时记录的选股理由。",
			},
			"themes": map[string]any{
				"type":        "string",
				"description": "板块、概念、题材自由文本，建议用逗号分隔。",
			},
			"createdAt": map[string]any{
				"type":        "string",
				"description": "首次创建时间，RFC3339 字符串。",
			},
			"updatedAt": map[string]any{
				"type":        "string",
				"description": "最近更新时间，RFC3339 字符串。",
			},
		},
	}
	return map[string]any{
		"type":        "object",
		"description": "MCP 工具调用结果。content[0].text 是给 Agent 阅读的简短文本；structuredContent 是机器可读数据。",
		"properties": map[string]any{
			"content": map[string]any{
				"type":        "array",
				"description": "文本消息数组，通常只有一条。",
			},
			"structuredContent": map[string]any{
				"type":        "object",
				"description": "add/get 返回 item；list 返回 items/count；remove 返回 code。",
				"properties": map[string]any{
					"item":  item,
					"items": map[string]any{"type": "array", "items": item},
					"count": map[string]any{
						"type":        "integer",
						"description": "list 返回的候选股数量。",
					},
					"code": map[string]any{
						"type":        "string",
						"description": "remove 成功移除的股票代码。",
					},
				},
			},
		},
	}
}

func callCandidatePoolMCPTool(
	name string,
	args map[string]any,
) (map[string]any, bool, error) {
	if name != "tdx_candidate_pool" {
		return nil, false, nil
	}
	if args == nil {
		args = map[string]any{}
	}
	result, err := callCandidatePoolMCP(args)
	return result, true, err
}

func callCandidatePoolMCP(args map[string]any) (map[string]any, error) {
	action := paperStringArg(args, "action")
	switch action {
	case "add":
		if err := requireCandidatePoolConfirm(args); err != nil {
			return nil, err
		}
		var req CandidatePoolUpsertRequest
		if err := decodeCandidatePoolMCPArgs(args, &req); err != nil {
			return nil, err
		}
		item, err := upsertCandidatePoolItem(req)
		if err != nil {
			return nil, err
		}
		return candidatePoolMCPResult("候选股已加入或更新。", map[string]any{"item": item}), nil
	case "list":
		items, err := listCandidatePoolItems(paperLimitArg(args))
		if err != nil {
			return nil, err
		}
		return candidatePoolMCPResult("候选池列表已返回。", map[string]any{
			"items": items,
			"count": len(items),
		}), nil
	case "get":
		item, err := getCandidatePoolItem(paperStringArg(args, "code"))
		if err != nil {
			return nil, err
		}
		return candidatePoolMCPResult("候选股详情已返回。", map[string]any{"item": item}), nil
	case "remove":
		if err := requireCandidatePoolConfirm(args); err != nil {
			return nil, err
		}
		code := paperStringArg(args, "code")
		if err := removeCandidatePoolItem(code); err != nil {
			return nil, err
		}
		return candidatePoolMCPResult("候选股已移除。", map[string]any{"code": normalizeStockCode(code)}), nil
	default:
		return nil, fmt.Errorf("unsupported candidate pool action: %s", action)
	}
}

func candidatePoolMCPResult(text string, data map[string]any) map[string]any {
	return map[string]any{
		"content":           []map[string]string{{"type": "text", "text": text}},
		"structuredContent": data,
	}
}

func decodeCandidatePoolMCPArgs(args map[string]any, target any) error {
	b, err := json.Marshal(args)
	if err != nil {
		return fmt.Errorf("candidate pool MCP arguments encode failed: %w", err)
	}
	if err := json.Unmarshal(b, target); err != nil {
		return fmt.Errorf("candidate pool MCP arguments decode failed: %w", err)
	}
	return nil
}

func requireCandidatePoolConfirm(args map[string]any) error {
	value, ok := args["confirm"].(bool)
	if !ok || !value {
		return errors.New("confirm must be true for side-effect candidate pool operations")
	}
	return nil
}

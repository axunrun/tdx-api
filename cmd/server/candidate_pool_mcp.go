package main

import (
	"encoding/json"
	"errors"
	"fmt"
)

func candidatePoolMCPTools() []mcpTool {
	tool := newMCPTool(
		"tdx_candidate_pool",
		"SQLite 选股候选池工具。用于让 Agent 维护可复查的备选股池，并把“为什么入池”“什么时候触发”“什么时候失效”“能不能买”拆成独立字段；add/remove 会写入持久化数据，必须传 confirm=true。",
		"",
		nil,
		requiredEnum("action", "操作：add 按 code 加入或更新当前记录；list 只读列出候选池；get 只读查看单只候选股；remove 硬删除当前记录。", "add", "list", "get", "remove"),
		optionalString("code", "股票代码；add/get/remove 时必填。"),
		optionalString("name", "股票名称；add 时可选，不传则尽量从股票名称库自动补全。"),
		optionalDateString("addedDate", "添加日期，YYYY-MM-DD 或 YYYYMMDD；add 时可选，不传默认今天。"),
		optionalString("reason", "添加理由；add 时必填，用于记录 Agent 选入候选池的依据。"),
		optionalString("themes", "板块、概念、题材字段；可填写逗号分隔文本，例如 AI算力, 数据中心, 北交所。"),
		optionalString("triggerCondition", "触发条件；可选，用于从 reason 中拆出“什么时候触发”；setup_ready 应尽量填写，observe_only 可为空。"),
		optionalString("invalidationCondition", "失效条件；可选，用于从 reason 中拆出“什么时候失效”；setup_ready 应尽量填写，observe_only 可为空。"),
		optionalIntegerDefault("limit", "list 返回条数，默认 50，最大 200。", 50, 1, 200),
		optionalBool("confirm", "add/remove 等写入候选池的操作必须为 true。"),
	)
	properties := tool.InputSchema["properties"].(map[string]any)
	properties["code"].(map[string]any)["pattern"] = `^\d{6}$`
	properties["validUntil"] = map[string]any{
		"type":        "string",
		"description": "有效时间，YYYY-MM-DD 或 YYYYMMDD；表示该候选股记录有效到哪一天，不传表示未设置到期日。",
		"pattern":     `^(\d{4}-\d{2}-\d{2}|\d{8})$`,
	}
	properties["buySignalTier"] = map[string]any{
		"type":        "string",
		"enum":        []string{"observe_only", "setup_ready", "trade_eligible"},
		"default":     "observe_only",
		"description": "股票买入状态，对应数据库 buy_signal_tier；不传默认 observe_only。observe_only=只观察，不能买；setup_ready=有买入设定但等触发，未触发不能买，通常需要 triggerCondition 和 invalidationCondition；trade_eligible=允许交易员在窗口内独立判断买入。",
	}
	tool.Description = "SQLite 选股候选池工具。上下文协议：reason 只写为什么入池；themes 写板块/概念/题材；buySignalTier 写能不能买；triggerCondition 写什么时候触发；invalidationCondition 写什么时候失效。list/get 是只读操作，不需要 confirm；add 是按 code 加入或更新，已存在时覆盖 name/addedDate/validUntil/buySignalTier/triggerCondition/invalidationCondition/reason/themes 并刷新 updatedAt，不追加历史；list 默认按 updatedAt desc 返回；remove 是硬删除，会永久移除当前记录且不保留归档。"
	properties["action"].(map[string]any)["description"] = "操作：add 加入或更新；list 只读列出候选池；get 只读查看单只候选股；remove 硬删除当前记录。"
	properties["code"].(map[string]any)["description"] = "6 位股票代码；add/get/remove 时必填；同 code 只保留一条当前候选池记录。"
	properties["name"].(map[string]any)["description"] = "股票名称；add 时可选，不传则尽量从股票名称库自动补全；同 code 已存在时会被本次值覆盖。"
	properties["addedDate"].(map[string]any)["description"] = "添加日期，YYYY-MM-DD 或 YYYYMMDD；add 时可选，不传默认今天；同 code 已存在时会被本次值覆盖。"
	properties["reason"].(map[string]any)["description"] = "入池理由；add 时必填；只写为什么入池，不要混写能不能买；同 code 已存在时覆盖旧 reason，不追加历史。"
	properties["themes"].(map[string]any)["description"] = "板块、概念、题材字段；可填写逗号分隔文本，只放主题标签，不放买入状态；同 code 已存在时覆盖旧 themes，不追加历史。"
	properties["triggerCondition"].(map[string]any)["description"] = "触发条件；可选，说明 setup_ready 何时触发；不表示已经可以买；同 code 已存在时覆盖旧 triggerCondition，不追加历史。"
	properties["invalidationCondition"].(map[string]any)["description"] = "失效条件；可选，说明候选股何时失效或移出观察；同 code 已存在时覆盖旧 invalidationCondition，不追加历史。"
	properties["limit"].(map[string]any)["description"] = "list 返回条数，默认 50，最大 200；list 默认按 updatedAt desc 返回最近更新记录。"
	properties["confirm"].(map[string]any)["description"] = "仅 add/remove 等写入操作必须为 true；list/get 是只读操作，不需要 confirm。"
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
		"description": "候选池单条记录。字段协议：reason=入池原因，themes=题材标签，buySignalTier=买入状态，triggerCondition=触发条件，invalidationCondition=失效条件。",
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
				"description": "Agent 或用户加入候选池时记录的选股理由；只解释为什么入池，不承载买入许可。",
			},
			"themes": map[string]any{
				"type":        "string",
				"description": "板块、概念、题材自由文本，建议用逗号分隔；不承载买入状态。",
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
	item["properties"].(map[string]any)["validUntil"] = map[string]any{
		"type":        "string",
		"description": "有效时间，服务端统一返回 YYYY-MM-DD；空字符串表示未设置到期日。",
	}
	item["properties"].(map[string]any)["buySignalTier"] = map[string]any{
		"type":        "string",
		"enum":        []string{"observe_only", "setup_ready", "trade_eligible"},
		"description": "股票买入状态：observe_only 不能买；setup_ready 触发后才可考虑，未触发不能买；trade_eligible 可独立判断买入。",
	}
	item["properties"].(map[string]any)["triggerCondition"] = map[string]any{
		"type":        "string",
		"description": "触发条件；用于说明 setup_ready 何时触发，空字符串表示未设置。",
	}
	item["properties"].(map[string]any)["invalidationCondition"] = map[string]any{
		"type":        "string",
		"description": "失效条件；用于说明候选股何时失效或移出观察，空字符串表示未设置。",
	}
	schema := map[string]any{
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
	schema["properties"].(map[string]any)["structuredContent"].(map[string]any)["description"] =
		"add/get 返回 item；list 按 updatedAt desc 返回 items/count；remove 硬删除当前记录后返回 code，不保留归档。"
	return schema
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

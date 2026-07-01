package main

import (
	"fmt"
	"net/http"
	"strings"
)

type AgentSectorMembership struct {
	Code       string                       `json:"code"`
	Name       string                       `json:"name,omitempty"`
	Source     string                       `json:"source"`
	TotalCount int                          `json:"totalCount"`
	Groups     []AgentSectorMembershipGroup `json:"groups"`
	Blocks     []AgentBriefBlock            `json:"blocks"`
	Note       string                       `json:"note"`
	Warnings   []string                     `json:"warnings,omitempty"`
}

type AgentSectorMembershipGroup struct {
	Type     string   `json:"type"`
	TypeName string   `json:"typeName"`
	Count    int      `json:"count"`
	Names    []string `json:"names"`
}

type AgentSectorMembershipText struct {
	Code    string `json:"code"`
	Format  string `json:"format"`
	Content string `json:"content"`
}

func handleAgentSectorMembership(w http.ResponseWriter, r *http.Request) {
	summary, ok := loadAgentSectorMembership(w, r)
	if !ok {
		return
	}
	jsonResp(w, summary)
}

func handleAgentSectorMembershipText(w http.ResponseWriter, r *http.Request) {
	summary, ok := loadAgentSectorMembership(w, r)
	if !ok {
		return
	}
	jsonResp(w, AgentSectorMembershipText{
		Code:    summary.Code,
		Format:  "text/plain; charset=utf-8",
		Content: buildAgentSectorMembershipText(summary),
	})
}

func loadAgentSectorMembership(
	w http.ResponseWriter,
	r *http.Request,
) (AgentSectorMembership, bool) {
	code := normalizeStockCode(r.URL.Query().Get("code"))
	if code == "" {
		jsonErr(w, "缺少code")
		return AgentSectorMembership{}, false
	}
	blocks, err := queryStockBlocks(code)
	if err != nil {
		jsonErr(w, err.Error())
		return AgentSectorMembership{}, false
	}
	return buildAgentSectorMembership(code, queryStockName(code), blocks), true
}

func buildAgentSectorMembership(
	code string,
	name string,
	blocks []AgentBriefBlock,
) AgentSectorMembership {
	return AgentSectorMembership{
		Code:       code,
		Name:       name,
		Source:     "tdx_agent_sector_membership",
		TotalCount: len(blocks),
		Groups:     groupSectorMembershipBlocks(blocks),
		Blocks:     blocks,
		Note:       "个股完整板块归属接口，仅做归属识别；板块强弱、排名和成分股对比由stock-in-sector或sector-detail承担。",
	}
}

func groupSectorMembershipBlocks(blocks []AgentBriefBlock) []AgentSectorMembershipGroup {
	order := []string{"concept", "style_region", "index"}
	typeNames := make(map[string]string)
	grouped := make(map[string][]string)
	for _, block := range blocks {
		if block.Type == "" || block.Name == "" {
			continue
		}
		typeNames[block.Type] = block.TypeName
		grouped[block.Type] = append(grouped[block.Type], block.Name)
	}

	groups := make([]AgentSectorMembershipGroup, 0, len(grouped))
	for _, typ := range order {
		names := grouped[typ]
		if len(names) == 0 {
			continue
		}
		groups = append(groups, AgentSectorMembershipGroup{
			Type:     typ,
			TypeName: typeNames[typ],
			Count:    len(names),
			Names:    names,
		})
		delete(grouped, typ)
	}
	for typ, names := range grouped {
		groups = append(groups, AgentSectorMembershipGroup{
			Type:     typ,
			TypeName: typeNames[typ],
			Count:    len(names),
			Names:    names,
		})
	}
	return groups
}

func buildAgentSectorMembershipText(summary AgentSectorMembership) string {
	var b strings.Builder
	if summary.Name != "" {
		b.WriteString(fmt.Sprintf("股票：%s（%s）\n\n", summary.Name, summary.Code))
	} else {
		b.WriteString(fmt.Sprintf("股票代码：%s\n\n", summary.Code))
	}
	b.WriteString("板块归属：\n")
	if len(summary.Groups) == 0 {
		b.WriteString("未查询到板块归属。\n")
		return strings.TrimSpace(b.String())
	}
	for _, group := range summary.Groups {
		if len(group.Names) == 0 {
			continue
		}
		b.WriteString(fmt.Sprintf("%s：%s。\n", group.TypeName, strings.Join(group.Names, "、")))
	}
	b.WriteString("\n用途：该接口只说明个股属于哪些板块；板块强弱和成分股比较请使用后续stock-in-sector或sector-detail。")
	return strings.TrimSpace(b.String())
}

package main

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/injoyai/tdx"
	"github.com/injoyai/tdx/protocol"
)

type AgentAuctionSummary struct {
	Source   string              `json:"source"`
	Code     string              `json:"code"`
	Name     string              `json:"name,omitempty"`
	Session  string              `json:"session"`
	Quote    *AgentBriefQuote    `json:"quote,omitempty"`
	Auction  *AgentAuctionResult `json:"auction,omitempty"`
	Context  AgentAuctionContext `json:"context"`
	Limits   map[string]int      `json:"limits"`
	Warnings []string            `json:"warnings,omitempty"`
	Note     string              `json:"note"`
}

type AgentAuctionResult struct {
	Count             int                `json:"count"`
	LatestTime        string             `json:"latestTime,omitempty"`
	LatestPrice       float64            `json:"latestPrice,omitempty"`
	ChangePct         float64            `json:"changePct,omitempty"`
	MatchedVolume     int64              `json:"matchedVolume,omitempty"`
	UnmatchedVolume   int64              `json:"unmatchedVolume,omitempty"`
	UnmatchedSide     string             `json:"unmatchedSide,omitempty"`
	UnmatchedSideText string             `json:"unmatchedSideText,omitempty"`
	Items             []AgentAuctionItem `json:"items,omitempty"`
	Signals           []string           `json:"signals,omitempty"`
}

type AgentAuctionItem struct {
	Time      string  `json:"time"`
	Price     float64 `json:"price"`
	Match     int64   `json:"match"`
	Unmatched int64   `json:"unmatched"`
	Flag      int8    `json:"flag"`
	Side      string  `json:"side"`
	SideText  string  `json:"sideText"`
}

type AgentAuctionContext struct {
	PrevClose float64 `json:"prevClose,omitempty"`
	Ret5      float64 `json:"ret5,omitempty"`
	Ret20     float64 `json:"ret20,omitempty"`
}

type AgentAuctionText struct {
	Format  string `json:"format"`
	Content string `json:"content"`
}

func handleAgentAuction(w http.ResponseWriter, r *http.Request) {
	summary, ok := loadAgentAuction(w, r)
	if !ok {
		return
	}
	jsonResp(w, summary)
}

func handleAgentAuctionText(w http.ResponseWriter, r *http.Request) {
	summary, ok := loadAgentAuction(w, r)
	if !ok {
		return
	}
	jsonResp(w, AgentAuctionText{
		Format:  "text/plain; charset=utf-8",
		Content: buildAgentAuctionText(summary),
	})
}

func loadAgentAuction(w http.ResponseWriter, r *http.Request) (AgentAuctionSummary, bool) {
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if code == "" {
		jsonErr(w, "缺少code")
		return AgentAuctionSummary{}, false
	}
	limit := parseCount(r.URL.Query().Get("limit"), 20)
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	c := cli()
	if c == nil {
		jsonErr(w, "TDX客户端未连接")
		return AgentAuctionSummary{}, false
	}
	session := strings.TrimSpace(r.URL.Query().Get("session"))
	if session == "" {
		session = "open"
	}
	if session != "open" && session != "close" && session != "all" {
		jsonErr(w, "session仅支持open、close、all")
		return AgentAuctionSummary{}, false
	}
	return buildAgentAuction(c, code, limit, session), true
}

func buildAgentAuction(c *tdx.Client, code string, limit int, session string) AgentAuctionSummary {
	warnings := make([]string, 0)
	summary := AgentAuctionSummary{
		Source:  "tdx_agent_auction",
		Code:    code,
		Name:    queryStockName(code),
		Session: session,
		Limits: map[string]int{
			"auctionItems": limit,
			"klineDays":    20,
		},
		Note: "集合竞价聚合接口；聚焦竞价末笔、相对昨收、未匹配方向和近5/20日背景。",
	}
	if quote, err := buildAgentBriefQuote(c, code); err != nil {
		warnings = append(warnings, "GetQuote失败: "+err.Error())
	} else {
		summary.Quote = quote
		summary.Context.PrevClose = quote.LastClose
	}
	if resp, err := c.GetKlineDay(code, 0, 20); err != nil || resp == nil {
		warnings = append(warnings, "GetKlineDay失败")
	} else {
		summary.Context.Ret5 = klineReturnPct(resp.List, 5)
		summary.Context.Ret20 = klineReturnPct(resp.List, 20)
		if summary.Context.PrevClose == 0 && len(resp.List) > 0 {
			summary.Context.PrevClose = resp.List[len(resp.List)-1].Close.Float64()
		}
	}
	if resp, err := c.GetCallAuction(code); err != nil || resp == nil {
		warnings = append(warnings, "GetCallAuction失败")
	} else {
		items := filterAuctionSession(resp.List, session)
		if len(items) == 0 && len(resp.List) > 0 {
			warnings = append(warnings, "指定session无集合竞价数据")
		}
		summary.Auction = buildAgentAuctionResult(items, summary.Context.PrevClose, limit)
	}
	summary.Warnings = warnings
	return summary
}

func filterAuctionSession(items []*protocol.CallAuction, session string) []*protocol.CallAuction {
	if session == "all" {
		return items
	}
	filtered := make([]*protocol.CallAuction, 0, len(items))
	for _, item := range items {
		minute := item.Time.Hour()*60 + item.Time.Minute()
		switch session {
		case "open":
			if minute >= 9*60+20 && minute <= 9*60+25 {
				filtered = append(filtered, item)
			}
		case "close":
			if minute >= 14*60+57 && minute <= 15*60 {
				filtered = append(filtered, item)
			}
		}
	}
	return filtered
}

func buildAgentAuctionResult(
	items []*protocol.CallAuction,
	prevClose float64,
	limit int,
) *AgentAuctionResult {
	if limit <= 0 || limit > len(items) {
		limit = len(items)
	}
	result := &AgentAuctionResult{
		Count: len(items),
		Items: make([]AgentAuctionItem, 0, limit),
	}
	start := len(items) - limit
	if start < 0 {
		start = 0
	}
	for _, item := range items[start:] {
		result.Items = append(result.Items, auctionItem(item))
	}
	if len(items) == 0 {
		return result
	}
	latest := items[len(items)-1]
	result.LatestTime = latest.Time.Format("15:04:05")
	result.LatestPrice = latest.Price.Float64()
	result.MatchedVolume = latest.Match
	result.UnmatchedVolume = latest.Unmatched
	result.UnmatchedSide, result.UnmatchedSideText = auctionSide(latest.Flag)
	if prevClose > 0 {
		result.ChangePct = (result.LatestPrice - prevClose) / prevClose * 100
	}
	result.Signals = auctionSignals(result)
	return result
}

func auctionItem(item *protocol.CallAuction) AgentAuctionItem {
	side, sideText := auctionSide(item.Flag)
	return AgentAuctionItem{
		Time:      item.Time.Format("15:04:05"),
		Price:     item.Price.Float64(),
		Match:     item.Match,
		Unmatched: item.Unmatched,
		Flag:      item.Flag,
		Side:      side,
		SideText:  sideText,
	}
}

func auctionSide(flag int8) (string, string) {
	if flag < 0 {
		return "sell", "卖盘未匹配"
	}
	return "buy", "买盘未匹配"
}

func auctionSignals(result *AgentAuctionResult) []string {
	signals := make([]string, 0)
	if result.ChangePct >= 3 {
		signals = append(signals, "高开竞价")
	}
	if result.ChangePct <= -3 {
		signals = append(signals, "低开竞价")
	}
	if result.UnmatchedSide == "buy" && result.UnmatchedVolume > result.MatchedVolume {
		signals = append(signals, "买盘未匹配量大于匹配量")
	}
	if result.UnmatchedSide == "sell" && result.UnmatchedVolume > result.MatchedVolume {
		signals = append(signals, "卖盘未匹配量大于匹配量")
	}
	return signals
}

func buildAgentAuctionText(summary AgentAuctionSummary) string {
	var b strings.Builder
	name := summary.Name
	if name == "" {
		name = summary.Code
	}
	b.WriteString(fmt.Sprintf("集合竞价：%s（%s），%s\n", name, summary.Code, auctionSessionText(summary.Session)))
	if summary.Auction != nil && summary.Auction.Count > 0 {
		b.WriteString(fmt.Sprintf(
			"末笔%s，价格%.2f，较昨收%s，匹配量%d，%s%d。\n",
			summary.Auction.LatestTime,
			summary.Auction.LatestPrice,
			formatPercentText(summary.Auction.ChangePct),
			summary.Auction.MatchedVolume,
			summary.Auction.UnmatchedSideText,
			summary.Auction.UnmatchedVolume,
		))
		if len(summary.Auction.Signals) > 0 {
			b.WriteString("信号：" + strings.Join(summary.Auction.Signals, "、") + "。\n")
		}
	} else {
		b.WriteString("暂无集合竞价数据。\n")
	}
	b.WriteString(fmt.Sprintf(
		"日K走势背景：截至可用日K，近5个交易日累计%s，近20个交易日累计%s（近20日%s）。\n",
		formatPercentText(summary.Context.Ret5),
		formatPercentText(summary.Context.Ret20),
		formatPercentText(summary.Context.Ret20),
	))
	appendWarningsText(&b, summary.Warnings)
	return strings.TrimSpace(b.String())
}

func auctionSessionText(session string) string {
	switch session {
	case "close":
		return "收盘竞价"
	case "all":
		return "全日竞价记录"
	default:
		return "开盘竞价"
	}
}

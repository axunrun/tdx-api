package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/injoyai/tdx/protocol"
)

type AgentSectorRealtime struct {
	Source        string          `json:"source"`
	GeneratedAt   string          `json:"generatedAt"`
	Available     bool            `json:"available"`
	Session       string          `json:"session"`
	SessionText   string          `json:"sessionText"`
	Sector        AgentBriefBlock `json:"sector"`
	DataDate      string          `json:"dataDate,omitempty"`
	AsOf          string          `json:"asOf,omitempty"`
	Current       float64         `json:"current,omitempty"`
	PreviousClose float64         `json:"previousClose,omitempty"`
	ChangePct     float64         `json:"changePct,omitempty"`
	DataSource    string          `json:"dataSource"`
	Note          string          `json:"note"`
}

type AgentSectorRealtimeText struct {
	Format  string `json:"format"`
	Content string `json:"content"`
}

func handleAgentSectorRealtime(w http.ResponseWriter, r *http.Request) {
	summary, ok := loadAgentSectorRealtime(w, r)
	if !ok {
		return
	}
	jsonResp(w, summary)
}

func handleAgentSectorRealtimeText(w http.ResponseWriter, r *http.Request) {
	summary, ok := loadAgentSectorRealtime(w, r)
	if !ok {
		return
	}
	jsonResp(w, AgentSectorRealtimeText{
		Format:  "text/plain; charset=utf-8",
		Content: buildAgentSectorRealtimeText(summary),
	})
}

func loadAgentSectorRealtime(w http.ResponseWriter, r *http.Request) (AgentSectorRealtime, bool) {
	sectorName := strings.TrimSpace(r.URL.Query().Get("sectorName"))
	indexCode := strings.TrimSpace(r.URL.Query().Get("indexCode"))
	if sectorName == "" && indexCode == "" {
		jsonErr(w, "缺少sectorName或indexCode")
		return AgentSectorRealtime{}, false
	}
	sectorType := strings.TrimSpace(r.URL.Query().Get("sectorType"))
	if sectorType == "" {
		sectorType = "concept"
	}
	sector, err := findAgentSectorMemberSet(sectorType, sectorName, indexCode)
	if err != nil {
		jsonErr(w, err.Error())
		return AgentSectorRealtime{}, false
	}

	now := time.Now()
	session := resolveSectorRealtimeSession(now)
	if session.status != "trading" {
		return buildAgentSectorRealtimeAt(sector.Block, nil, now), true
	}
	if sector.Block.IndexCode == "" {
		jsonErr(w, "该板块缺少TDX指数代码，无法获取实时涨跌")
		return AgentSectorRealtime{}, false
	}
	c := cli()
	if c == nil {
		jsonErr(w, "TDX客户端未连接")
		return AgentSectorRealtime{}, false
	}
	resp, err := c.GetIndexDay("sh"+sector.Block.IndexCode, 0, 2)
	if err != nil || resp == nil {
		if err == nil {
			err = fmt.Errorf("TDX返回空响应")
		}
		jsonErr(w, "GetIndexDay失败: "+err.Error())
		return AgentSectorRealtime{}, false
	}
	return buildAgentSectorRealtimeAt(sector.Block, resp.List, now), true
}

func buildAgentSectorRealtimeAt(
	sector AgentBriefBlock,
	klines protocol.Klines,
	now time.Time,
) AgentSectorRealtime {
	session := resolveSectorRealtimeSession(now)
	summary := AgentSectorRealtime{
		Source:      "tdx_agent_sector_realtime",
		GeneratedAt: now.Format(time.RFC3339),
		Session:     session.status,
		SessionText: session.text,
		Sector:      sector,
		DataSource:  "TDX板块指数当日日K实时字段",
	}
	if session.status != "trading" {
		summary.Note = "当前非交易时间，无实时题材板块涨跌幅数据。"
		return summary
	}

	today := dateOnly(now)
	var current *protocol.Kline
	for _, kline := range klines {
		if kline == nil || !dateOnly(kline.Time).Equal(today) {
			continue
		}
		if current == nil || kline.Time.After(current.Time) {
			current = kline
		}
	}
	if current == nil || current.Last.Float64() <= 0 {
		summary.Note = "TDX未返回当前交易日板块指数实时数据。"
		return summary
	}

	summary.Available = true
	summary.DataDate = current.Time.Format(time.DateOnly)
	summary.AsOf = now.Format(time.RFC3339)
	summary.Current = current.Close.Float64()
	summary.PreviousClose = current.Last.Float64()
	summary.ChangePct = (summary.Current - summary.PreviousClose) /
		summary.PreviousClose * 100
	summary.Note = "仅表示查询时点的盘中实时涨跌，未收盘前不属于完整交易日数据。"
	return summary
}

func resolveSectorRealtimeSession(now time.Time) paperMarketSession {
	if now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
		return paperMarketSession{"closed", "非交易日"}
	}
	second := now.Hour()*60*60 + now.Minute()*60 + now.Second()
	switch {
	case second < 9*60*60+30*60:
		return paperMarketSession{"preopen", "开盘前"}
	case second <= 11*60*60+30*60:
		return paperMarketSession{"trading", "上午交易中"}
	case second < 13*60*60:
		return paperMarketSession{"break", "午间休市"}
	case second <= 15*60*60:
		return paperMarketSession{"trading", "下午交易中"}
	default:
		return paperMarketSession{"closed", "已收盘"}
	}
}

func buildAgentSectorRealtimeText(summary AgentSectorRealtime) string {
	if !summary.Available {
		return fmt.Sprintf(
			"实时题材板块：%s（%s）\n查询时间：%s\n市场状态：%s\n%s",
			summary.Sector.Name,
			valueOrDash(summary.Sector.IndexCode),
			summary.GeneratedAt,
			summary.SessionText,
			summary.Note,
		)
	}
	return fmt.Sprintf(
		"实时题材板块：%s（%s）\n"+
			"查询时间：%s\n"+
			"交易日期：%s\n"+
			"当前指数：%.2f，昨收：%.2f，实时涨跌幅：%s\n"+
			"数据源：%s\n"+
			"说明：%s",
		summary.Sector.Name,
		valueOrDash(summary.Sector.IndexCode),
		summary.GeneratedAt,
		summary.DataDate,
		summary.Current,
		summary.PreviousClose,
		formatPercentText(summary.ChangePct),
		summary.DataSource,
		summary.Note,
	)
}

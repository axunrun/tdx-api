package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/injoyai/tdx"
)

type PaperMarketSnapshot struct {
	Status      string             `json:"status"`
	StatusText  string             `json:"statusText"`
	Note        string             `json:"note"`
	Indexes     []PaperMarketIndex `json:"indexes"`
	Breadth     AgentMarketBreadth `json:"breadth"`
	GeneratedAt string             `json:"generatedAt"`
	NextRefresh string             `json:"nextRefresh"`
	Warnings    []string           `json:"warnings,omitempty"`
}

type PaperMarketIndex struct {
	Code      string  `json:"code"`
	Name      string  `json:"name"`
	Date      string  `json:"date,omitempty"`
	Close     float64 `json:"close,omitempty"`
	LastClose float64 `json:"lastClose,omitempty"`
	ChangePct float64 `json:"changePct,omitempty"`
}

type paperMarketCacheState struct {
	mu        sync.Mutex
	snapshot  PaperMarketSnapshot
	expiresAt time.Time
	loaded    bool
}

var paperMarketCache paperMarketCacheState

func loadPaperMarketSnapshot(c *tdx.Client, now time.Time) PaperMarketSnapshot {
	session := resolvePaperMarketSession(now)
	ttl := paperMarketSnapshotTTL(session, now)

	paperMarketCache.mu.Lock()
	if paperMarketCache.loaded && now.Before(paperMarketCache.expiresAt) {
		snapshot := paperMarketCache.snapshot
		paperMarketCache.mu.Unlock()
		return snapshot
	}
	paperMarketCache.mu.Unlock()

	snapshot := buildPaperMarketSnapshot(c, session, now, ttl)

	paperMarketCache.mu.Lock()
	paperMarketCache.snapshot = snapshot
	paperMarketCache.expiresAt = now.Add(ttl)
	paperMarketCache.loaded = true
	paperMarketCache.mu.Unlock()
	return snapshot
}

func buildPaperMarketSnapshot(
	c *tdx.Client,
	session paperMarketSession,
	now time.Time,
	ttl time.Duration,
) PaperMarketSnapshot {
	warnings := make([]string, 0)
	indexes := []PaperMarketIndex{}
	breadth := AgentMarketBreadth{
		Source:     "GetTdxStat",
		SourceNote: "市场广度使用TdxStat快照；涨停/跌停按±9.9%近似统计。",
	}

	if c == nil {
		warnings = append(warnings, "TDX客户端未连接，市场行情暂不可用")
	} else {
		agentIndexes, indexWarnings := buildMarketIndexes(c)
		warnings = append(warnings, indexWarnings...)
		indexes = paperMarketIndexes(agentIndexes)

		stats, err := c.GetTdxStat()
		if err != nil {
			warnings = append(warnings, "GetTdxStat失败: "+err.Error())
		} else {
			breadth = buildMarketBreadth(stats)
		}
	}

	return PaperMarketSnapshot{
		Status:      session.status,
		StatusText:  session.text,
		Note:        paperMarketNote(session),
		Indexes:     indexes,
		Breadth:     breadth,
		GeneratedAt: now.Format(time.RFC3339),
		NextRefresh: now.Add(ttl).Format(time.RFC3339),
		Warnings:    warnings,
	}
}

func paperMarketIndexes(items []AgentMarketIndex) []PaperMarketIndex {
	indexes := make([]PaperMarketIndex, 0, len(items))
	for _, item := range items {
		indexes = append(indexes, PaperMarketIndex{
			Code:      item.Code,
			Name:      item.Name,
			Date:      item.Date,
			Close:     item.Close,
			LastClose: item.LastClose,
			ChangePct: item.ChangePct,
		})
	}
	return indexes
}

type paperMarketSession struct {
	status string
	text   string
}

func resolvePaperMarketSession(now time.Time) paperMarketSession {
	if now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
		return paperMarketSession{"closed", "非交易日"}
	}
	minute := now.Hour()*60 + now.Minute()
	switch {
	case minute < 9*60+30:
		return paperMarketSession{"preopen", "开盘前"}
	case minute <= 11*60+30:
		return paperMarketSession{"trading", "上午交易中"}
	case minute < 13*60:
		return paperMarketSession{"break", "午间休市"}
	case minute <= 15*60:
		return paperMarketSession{"trading", "下午交易中"}
	default:
		return paperMarketSession{"closed", "已收盘"}
	}
}

func paperMarketSnapshotTTL(session paperMarketSession, now time.Time) time.Duration {
	ttl := 10 * time.Minute
	if session.status == "trading" {
		ttl = 30 * time.Second
	}
	return minDuration(ttl, durationToNextMarketBoundary(now))
}

func durationToNextMarketBoundary(now time.Time) time.Duration {
	if now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
		return 10 * time.Minute
	}
	boundaries := []time.Time{
		time.Date(now.Year(), now.Month(), now.Day(), 9, 30, 0, 0, now.Location()),
		time.Date(now.Year(), now.Month(), now.Day(), 11, 30, 0, 0, now.Location()),
		time.Date(now.Year(), now.Month(), now.Day(), 13, 0, 0, 0, now.Location()),
		time.Date(now.Year(), now.Month(), now.Day(), 15, 0, 0, 0, now.Location()),
	}
	for _, boundary := range boundaries {
		if now.Before(boundary) {
			return boundary.Sub(now)
		}
	}
	return 10 * time.Minute
}

func minDuration(a, b time.Duration) time.Duration {
	if b <= 0 || a < b {
		return a
	}
	return b
}

func paperMarketNote(session paperMarketSession) string {
	switch session.status {
	case "trading":
		return "盘中市场快照，服务端按30秒缓存刷新。"
	case "break":
		return "午间休市，展示上午收盘后的最近市场快照。"
	case "preopen":
		return "开盘前，展示最近可获取的市场快照。"
	default:
		return fmt.Sprintf("%s，展示最近可获取的收盘或历史快照。", session.text)
	}
}

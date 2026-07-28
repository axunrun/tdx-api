package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/injoyai/tdx"
	"github.com/injoyai/tdx/protocol"
)

const marketQuoteBatchSize = 80

type marketBreadthCacheState struct {
	mu        sync.Mutex
	snapshot  AgentMarketBreadth
	expiresAt time.Time
}

var currentMarketBreadthCache marketBreadthCacheState
var marketStockCodesCache *agentTTLCache[[]string]

func loadCurrentMarketBreadth(
	c *tdx.Client,
	indexes []AgentMarketIndex,
	now time.Time,
) (AgentMarketBreadth, []string) {
	date := latestMarketIndexDate(indexes)
	if date == "" || date != now.Format("2006-01-02") {
		return unavailableCurrentBreadth(
			date,
			"当前交易日尚无有效行情；未使用历史行情冒充当前数据",
		), nil
	}

	currentMarketBreadthCache.mu.Lock()
	defer currentMarketBreadthCache.mu.Unlock()
	if currentMarketBreadthCache.snapshot.Date == date &&
		now.Before(currentMarketBreadthCache.expiresAt) {
		return currentMarketBreadthCache.snapshot, nil
	}

	snapshot, warnings := fetchCurrentMarketBreadth(c, date, now)
	if !snapshot.Available &&
		currentMarketBreadthCache.snapshot.Available &&
		currentMarketBreadthCache.snapshot.Date == date {
		snapshot = currentMarketBreadthCache.snapshot
		warnings = append(warnings, "实时广度刷新失败，沿用同一交易日上一笔有效快照")
	}
	currentMarketBreadthCache.snapshot = snapshot
	currentMarketBreadthCache.expiresAt = now.Add(currentMarketBreadthTTL(now))
	return snapshot, warnings
}

func fetchCurrentMarketBreadth(
	c *tdx.Client,
	date string,
	now time.Time,
) (AgentMarketBreadth, []string) {
	codes, err := loadMarketStockCodes(c)
	if err != nil {
		return unavailableCurrentBreadth(date, "A股代码表获取失败"), []string{err.Error()}
	}

	changes := make([]float64, 0, len(codes))
	failed := 0
	failedBatches := 0
	var firstQuoteError error
	warnings := make([]string, 0)
	for start := 0; start < len(codes); start += marketQuoteBatchSize {
		end := min(start+marketQuoteBatchSize, len(codes))
		quotes, quoteErr := c.GetQuote(codes[start:end]...)
		if quoteErr != nil {
			failed += end - start
			failedBatches++
			if firstQuoteError == nil {
				firstQuoteError = quoteErr
			}
			continue
		}
		for _, quote := range quotes {
			if quote == nil || quote.Kline == nil {
				continue
			}
			kline := quote.Kline
			lastClose := kline.Last.Float64()
			price := kline.Close.Float64()
			if lastClose <= 0 || price <= 0 || kline.Volume <= 0 {
				continue
			}
			changes = append(changes, (price-lastClose)/lastClose*100)
		}
	}
	if failedBatches > 0 {
		warnings = append(warnings, fmt.Sprintf(
			"实时行情%d个批次（%d只股票）获取失败，首个错误: %v",
			failedBatches,
			failed,
			firstQuoteError,
		))
	}

	breadth := calculateMarketBreadth(changes)
	breadth.Available = breadth.ValidCount > 0
	breadth.Complete = failed == 0
	breadth.DataType = "current_snapshot"
	breadth.Date = date
	breadth.AsOf = now.Format(time.RFC3339)
	breadth.Universe = "A股股票（沪深北，不含ETF、基金、债券）"
	breadth.UniverseTotal = len(codes)
	breadth.UnavailableCount = len(codes) - breadth.ValidCount
	breadth.Source = "GetStockCodeAll+GetQuote"
	breadth.SourceNote = "当前行情快照；停牌、无成交或无有效报价证券不计入涨跌家数。"
	if !breadth.Available {
		breadth.SourceNote = "当前交易日未取得有效A股行情；未使用历史行情冒充当前数据"
	}
	return breadth, warnings
}

func loadMarketStockCodes(c *tdx.Client) ([]string, error) {
	if marketStockCodesCache == nil {
		marketStockCodesCache = newAgentTTLCache(c.GetStockCodeAll, 6*time.Hour)
	}
	return marketStockCodesCache.Get()
}

func buildLatestCompletedMarketBreadth(stats []*protocol.TdxStat) AgentMarketBreadth {
	latestDate := ""
	for _, stat := range stats {
		if isAStockStat(stat) && stat.Date > latestDate {
			latestDate = stat.Date
		}
	}
	if latestDate == "" {
		return AgentMarketBreadth{
			DataType:   "latest_completed_close",
			Available:  false,
			Universe:   "A股股票（沪深北，不含ETF、基金、债券）",
			Source:     "GetTdxStat",
			SourceNote: "最近盘后统计不可用",
		}
	}

	changes := make([]float64, 0, len(stats))
	seen := make(map[string]struct{}, len(stats))
	for _, stat := range stats {
		if !isAStockStat(stat) || stat.Date != latestDate {
			continue
		}
		key := fmt.Sprintf("%d:%s", stat.Market, stat.Code)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		changes = append(changes, stat.ChangePct)
	}

	breadth := calculateMarketBreadth(changes)
	breadth.Available = breadth.ValidCount > 0
	breadth.Complete = true
	breadth.DataType = "latest_completed_close"
	breadth.Date = formatTdxStatDate(latestDate)
	breadth.AsOf = breadth.Date
	breadth.Universe = "A股股票（沪深北，不含ETF、基金、债券）"
	breadth.UniverseTotal = len(changes)
	breadth.Source = "GetTdxStat"
	breadth.SourceNote = "TdxStat最近完整盘后统计；仅统计该文件最新日期的A股股票。"
	return breadth
}

func calculateMarketBreadth(changes []float64) AgentMarketBreadth {
	breadth := AgentMarketBreadth{
		Total:      len(changes),
		ValidCount: len(changes),
	}
	values := make([]float64, 0, len(changes))
	sum := 0.0
	for _, change := range changes {
		sum += change
		values = append(values, change)
		switch {
		case change > 0:
			breadth.Rising++
		case change < 0:
			breadth.Falling++
		default:
			breadth.Flat++
		}
		if change >= 9.9 {
			breadth.LimitUp++
		}
		if change <= -9.9 {
			breadth.LimitDown++
		}
	}
	if breadth.ValidCount > 0 {
		breadth.RisingPct = float64(breadth.Rising) / float64(breadth.ValidCount) * 100
		breadth.AverageChange = sum / float64(breadth.ValidCount)
		breadth.MedianChange = medianFloat64(values)
	}
	return breadth
}

func isAStockStat(stat *protocol.TdxStat) bool {
	if stat == nil || stat.Code == "" {
		return false
	}
	exchange := protocol.Exchange(stat.Market).String()
	return protocol.IsStock(exchange + stat.Code)
}

func formatTdxStatDate(value string) string {
	if len(value) != 8 {
		return value
	}
	return value[:4] + "-" + value[4:6] + "-" + value[6:]
}

func latestMarketIndexDate(indexes []AgentMarketIndex) string {
	latest := ""
	for _, index := range indexes {
		if index.Date > latest {
			latest = index.Date
		}
	}
	return latest
}

func unavailableCurrentBreadth(date, note string) AgentMarketBreadth {
	return AgentMarketBreadth{
		DataType:   "current_snapshot",
		Date:       date,
		Available:  false,
		Universe:   "A股股票（沪深北，不含ETF、基金、债券）",
		Source:     "GetStockCodeAll+GetQuote",
		SourceNote: note,
	}
}

func currentMarketBreadthTTL(now time.Time) time.Duration {
	minute := now.Hour()*60 + now.Minute()
	if now.Weekday() != time.Saturday &&
		now.Weekday() != time.Sunday &&
		minute >= 9*60+20 &&
		minute <= 15*60 {
		return 30 * time.Second
	}
	return 10 * time.Minute
}

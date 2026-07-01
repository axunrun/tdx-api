package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/injoyai/tdx"
	"github.com/injoyai/tdx/protocol"
)

const (
	tradeFlowSource       = "tdx_tick_estimate"
	tradeFlowLookbackDays = 200
	tradeFlowRefreshDelay = 3 * time.Second
)

type TradeFlowEstimate struct {
	Code            string              `json:"code"`
	Date            string              `json:"date"`
	Source          string              `json:"source"`
	IsOfficial      bool                `json:"isOfficial"`
	Method          string              `json:"method"`
	IsIntraday      bool                `json:"isIntraday"`
	TimeRange       TradeFlowTimeRange  `json:"timeRange"`
	Direction       TradeFlowDirection  `json:"direction"`
	Thresholds      TradeFlowThresholds `json:"thresholds"`
	ThresholdSource string              `json:"thresholdSource"`
	Summary         TradeFlowSummary    `json:"summary"`
	Levels          []TradeFlowLevel    `json:"levels"`
	Warnings        []string            `json:"warnings,omitempty"`
}

type TradeFlowTimeRange struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type TradeFlowDirection struct {
	Status0 string `json:"status0"`
	Status1 string `json:"status1"`
	Status2 string `json:"status2"`
}

type TradeFlowThresholds struct {
	SuperLarge TradeFlowThreshold `json:"superLarge"`
	Large      TradeFlowThreshold `json:"large"`
	Medium     TradeFlowThreshold `json:"medium"`
}

type TradeFlowThreshold struct {
	Amount float64 `json:"amount"`
	Volume int     `json:"volume"`
}

type TradeFlowSummary struct {
	TotalAmount      float64 `json:"totalAmount"`
	TotalBuyAmount   float64 `json:"totalBuyAmount"`
	TotalSellAmount  float64 `json:"totalSellAmount"`
	NetInflow        float64 `json:"netInflow"`
	NetInflowPct     float64 `json:"netInflowPct"`
	MainNetInflow    float64 `json:"mainNetInflow"`
	MainNetInflowPct float64 `json:"mainNetInflowPct"`
	TradeCount       int     `json:"tradeCount"`
	BuyCount         int     `json:"buyCount"`
	SellCount        int     `json:"sellCount"`
	NeutralCount     int     `json:"neutralCount"`
}

type TradeFlowLevel struct {
	Level      string  `json:"level"`
	Name       string  `json:"name"`
	BuyAmount  float64 `json:"buyAmount"`
	SellAmount float64 `json:"sellAmount"`
	NetAmount  float64 `json:"netAmount"`
	NetPct     float64 `json:"netPct"`
	TradeCount int     `json:"tradeCount"`
}

type TradeFlowEstimateText struct {
	Code    string `json:"code"`
	Date    string `json:"date"`
	Format  string `json:"format"`
	Content string `json:"content"`
}

type TradeFlowThresholdCache struct {
	Code             string  `json:"code"`
	AsOfDate         string  `json:"asOfDate"`
	LookbackDays     int     `json:"lookbackDays"`
	SampleCount      int     `json:"sampleCount"`
	SuperLargeAmount float64 `json:"superLargeAmount"`
	LargeAmount      float64 `json:"largeAmount"`
	MediumAmount     float64 `json:"mediumAmount"`
	Source           string  `json:"source"`
	Method           string  `json:"method"`
	UpdatedAt        string  `json:"updatedAt"`
}

var defaultTradeFlowThresholds = TradeFlowThresholds{
	SuperLarge: TradeFlowThreshold{Amount: 1000000, Volume: 500000},
	Large:      TradeFlowThreshold{Amount: 200000, Volume: 100000},
	Medium:     TradeFlowThreshold{Amount: 40000, Volume: 20000},
}

func handleAgentTradeFlowEstimate(w http.ResponseWriter, r *http.Request) {
	code, date, ok := parseTradeFlowParams(w, r)
	if !ok {
		return
	}
	c := cli()
	if c == nil {
		jsonErr(w, "未连接")
		return
	}
	trades, err := fetchTradeFlowTrades(c, code, date)
	if err != nil {
		jsonErr(w, err.Error())
		return
	}
	jsonResp(w, buildTradeFlowEstimate(code, date, trades))
}

func handleAgentTradeFlowEstimateText(w http.ResponseWriter, r *http.Request) {
	code, date, ok := parseTradeFlowParams(w, r)
	if !ok {
		return
	}
	c := cli()
	if c == nil {
		jsonErr(w, "未连接")
		return
	}
	trades, err := fetchTradeFlowTrades(c, code, date)
	if err != nil {
		jsonErr(w, err.Error())
		return
	}
	estimate := buildTradeFlowEstimate(code, date, trades)
	jsonResp(w, TradeFlowEstimateText{
		Code:    code,
		Date:    date,
		Format:  "text/markdown; charset=utf-8",
		Content: buildTradeFlowEstimateText(estimate),
	})
}

func handleAdminTradeFlowThresholdsRefresh(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		jsonErr(w, "缺少code")
		return
	}
	c := cli()
	if c == nil {
		jsonErr(w, "未连接")
		return
	}
	cache, err := refreshTradeFlowThresholds(c, code, tradeFlowRefreshDelay)
	if err != nil {
		jsonErr(w, err.Error())
		return
	}
	jsonResp(w, cache)
}

func parseTradeFlowParams(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	code := r.URL.Query().Get("code")
	if code == "" {
		jsonErr(w, "缺少code")
		return "", "", false
	}
	date, err := normalizeTradeFlowDate(r.URL.Query().Get("date"))
	if err != nil {
		jsonErr(w, err.Error())
		return "", "", false
	}
	return code, date, true
}

func normalizeTradeFlowDate(date string) (string, error) {
	if date == "" {
		return time.Now().Format("2006-01-02"), nil
	}
	date = strings.TrimSpace(date)
	if len(date) == 8 && !strings.Contains(date, "-") {
		date = date[:4] + "-" + date[4:6] + "-" + date[6:]
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return "", fmt.Errorf("date格式应为YYYY-MM-DD")
	}
	return date, nil
}

func fetchTradeFlowTrades(c *tdx.Client, code, date string) (protocol.Trades, error) {
	var resp *protocol.TradeResp
	var err error
	if date == time.Now().Format("2006-01-02") {
		resp, err = c.GetMinuteTradeAll(code)
	} else {
		resp, err = c.GetHistoryMinuteTradeDay(strings.ReplaceAll(date, "-", ""), code)
	}
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, fmt.Errorf("无数据")
	}
	return resp.List, nil
}

func buildTradeFlowEstimate(code, date string, trades protocol.Trades) TradeFlowEstimate {
	thresholds, source, warnings := loadTradeFlowThresholds(code)
	levels := []TradeFlowLevel{
		{Level: "super_large", Name: "超大单"},
		{Level: "large", Name: "大单"},
		{Level: "medium", Name: "中单"},
		{Level: "small", Name: "小单"},
	}
	out := TradeFlowEstimate{
		Code:       code,
		Date:       date,
		Source:     tradeFlowSource,
		IsOfficial: false,
		Method:     "按逐笔成交金额或成交量阈值估算",
		IsIntraday: date == time.Now().Format("2006-01-02"),
		Direction: TradeFlowDirection{
			Status0: "outflow",
			Status1: "inflow",
			Status2: "neutral",
		},
		Thresholds:      thresholds,
		ThresholdSource: source,
		Levels:          levels,
		Warnings: append([]string{
			"该结果为TDX逐笔成交估算，不等同于东方财富/同花顺官方资金流；方向按平台对照修正为Status=1流入、Status=0流出。",
		}, warnings...),
	}
	if len(trades) == 0 {
		out.Warnings = append(out.Warnings, "未获取到逐笔成交，可能是非交易日或TDX历史逐笔不可用。")
		return out
	}
	out.TimeRange.Start = trades[0].Time.Format("15:04:05")
	out.TimeRange.End = trades[len(trades)-1].Time.Format("15:04:05")
	for _, trade := range trades {
		if trade == nil {
			continue
		}
		amount := trade.Amount().Float64()
		level := tradeFlowLevelIndex(amount, trade.Volume*100, thresholds)
		out.Summary.TotalAmount += amount
		out.Summary.TradeCount++
		out.Levels[level].TradeCount++
		switch trade.Status {
		case 1:
			out.Summary.TotalBuyAmount += amount
			out.Summary.BuyCount++
			out.Levels[level].BuyAmount += amount
		case 0:
			out.Summary.TotalSellAmount += amount
			out.Summary.SellCount++
			out.Levels[level].SellAmount += amount
		default:
			out.Summary.NeutralCount++
		}
	}
	for i := range out.Levels {
		out.Levels[i].NetAmount = out.Levels[i].BuyAmount - out.Levels[i].SellAmount
		out.Levels[i].NetPct = percentOf(out.Levels[i].NetAmount, out.Summary.TotalAmount)
	}
	out.Summary.NetInflow = out.Summary.TotalBuyAmount - out.Summary.TotalSellAmount
	out.Summary.NetInflowPct = percentOf(out.Summary.NetInflow, out.Summary.TotalAmount)
	out.Summary.MainNetInflow = out.Levels[0].NetAmount + out.Levels[1].NetAmount
	out.Summary.MainNetInflowPct = percentOf(out.Summary.MainNetInflow, out.Summary.TotalAmount)
	return out
}

func tradeFlowLevelIndex(amount float64, volume int, t TradeFlowThresholds) int {
	switch {
	case amount >= t.SuperLarge.Amount || (t.SuperLarge.Volume > 0 && volume >= t.SuperLarge.Volume):
		return 0
	case amount >= t.Large.Amount || (t.Large.Volume > 0 && volume >= t.Large.Volume):
		return 1
	case amount >= t.Medium.Amount || (t.Medium.Volume > 0 && volume >= t.Medium.Volume):
		return 2
	default:
		return 3
	}
}

func percentOf(value, total float64) float64 {
	if total == 0 {
		return 0
	}
	return value / total * 100
}

func buildTradeFlowEstimateText(e TradeFlowEstimate) string {
	s := e.Summary
	var b strings.Builder
	b.WriteString(fmt.Sprintf("股票代码：%s\n\n", e.Code))
	if s.TradeCount == 0 {
		b.WriteString("资金流估算：\n未获取到逐笔成交数据，无法计算资金流估算。")
		return b.String()
	}

	b.WriteString("资金流估算：\n")
	b.WriteString(fmt.Sprintf(
		"%s %s-%s，成交额约%s。总流入%s，总流出%s，整体%s，占成交额%s。\n\n",
		e.Date,
		e.TimeRange.Start,
		e.TimeRange.End,
		formatCNYText(s.TotalAmount),
		formatCNYText(s.TotalBuyAmount),
		formatCNYText(s.TotalSellAmount),
		formatFlowText(s.NetInflow),
		formatPercentText(s.NetInflowPct),
	))

	b.WriteString("主力资金：\n")
	b.WriteString(fmt.Sprintf(
		"主力流入%s，主力流出%s，主力%s，占成交额%s。\n\n",
		formatCNYText(e.Levels[0].BuyAmount+e.Levels[1].BuyAmount),
		formatCNYText(e.Levels[0].SellAmount+e.Levels[1].SellAmount),
		formatFlowText(s.MainNetInflow),
		formatPercentText(s.MainNetInflowPct),
	))

	b.WriteString("分档资金：\n")
	for _, level := range e.Levels {
		b.WriteString(fmt.Sprintf(
			"%s流入%s，流出%s，%s，成交%d笔。\n",
			level.Name,
			formatCNYText(level.BuyAmount),
			formatCNYText(level.SellAmount),
			formatFlowText(level.NetAmount),
			level.TradeCount,
		))
	}
	b.WriteString("\n")

	b.WriteString("统计口径：\n")
	b.WriteString(fmt.Sprintf(
		"阈值来源为%s，基于最近%d个交易日逐笔成交金额从大到小排序，并按累计成交额占比分档：0%%~10%%为超大单，10%%~30%%为大单，30%%~55%%为中单，55%%~100%%为小单。当前阈值为超大单>=%s，大单>=%s，中单>=%s，低于中单阈值为小单。TDX Status=1计为流入，Status=0计为流出，Status=2计为中性。该结果为TDX逐笔成交估算，不等同于外部APP官方资金流。",
		e.ThresholdSource,
		tradeFlowLookbackDays,
		formatCNYText(e.Thresholds.SuperLarge.Amount),
		formatCNYText(e.Thresholds.Large.Amount),
		formatCNYText(e.Thresholds.Medium.Amount),
	))
	return strings.TrimSpace(b.String())
}

func formatFlowText(value float64) string {
	if value < 0 {
		return "净流出" + formatCNYText(-value)
	}
	return "净流入" + formatCNYText(value)
}

func loadTradeFlowThresholds(code string) (TradeFlowThresholds, string, []string) {
	cache, err := readTradeFlowThresholdCache(code)
	if err != nil {
		return defaultTradeFlowThresholds, "fixed", []string{"未找到自适应阈值缓存，已回退固定阈值。"}
	}
	return TradeFlowThresholds{
		SuperLarge: TradeFlowThreshold{Amount: cache.SuperLargeAmount},
		Large:      TradeFlowThreshold{Amount: cache.LargeAmount},
		Medium:     TradeFlowThreshold{Amount: cache.MediumAmount},
	}, "adaptive", nil
}

func readTradeFlowThresholdCache(code string) (*TradeFlowThresholdCache, error) {
	data, err := os.ReadFile(tradeFlowThresholdCachePath(code))
	if err != nil {
		return nil, err
	}
	var cache TradeFlowThresholdCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}
	if cache.Code == "" || cache.SampleCount == 0 {
		return nil, fmt.Errorf("阈值缓存无效")
	}
	return &cache, nil
}

func refreshTradeFlowThresholds(c *tdx.Client, code string, delay time.Duration) (*TradeFlowThresholdCache, error) {
	w, err := tdx.NewWorkday(tdx.WithWorkdayClient(c))
	if err != nil {
		return nil, err
	}
	if err := w.Update(); err != nil {
		return nil, err
	}
	dates := recentTradeFlowDates(w, tradeFlowLookbackDays)
	amounts := make([]float64, 0, len(dates)*1000)
	for i, date := range dates {
		resp, err := c.GetHistoryMinuteTradeDay(date.Format("20060102"), code)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", date.Format("2006-01-02"), err)
		}
		for _, trade := range resp.List {
			if trade != nil {
				amounts = append(amounts, trade.Amount().Float64())
			}
		}
		if i < len(dates)-1 {
			time.Sleep(delay)
		}
	}
	cache := buildTradeFlowAdaptiveThresholds(code, amounts)
	if err := writeTradeFlowThresholdCache(cache); err != nil {
		return nil, err
	}
	return cache, nil
}

func recentTradeFlowDates(w *tdx.Workday, limit int) []time.Time {
	end := time.Now()
	start := end.AddDate(-2, 0, 0)
	dates := make([]time.Time, 0, limit)
	for date := range w.Iter(start, end, true) {
		dates = append(dates, date)
		if len(dates) >= limit {
			break
		}
	}
	return dates
}

func buildTradeFlowAdaptiveThresholds(code string, amounts []float64) *TradeFlowThresholdCache {
	sort.Slice(amounts, func(i, j int) bool { return amounts[i] > amounts[j] })
	return &TradeFlowThresholdCache{
		Code:             code,
		AsOfDate:         time.Now().Format("2006-01-02"),
		LookbackDays:     tradeFlowLookbackDays,
		SampleCount:      len(amounts),
		SuperLargeAmount: cumulativeShareAmount(amounts, 0.10),
		LargeAmount:      cumulativeShareAmount(amounts, 0.30),
		MediumAmount:     cumulativeShareAmount(amounts, 0.55),
		Source:           "tdx_history_tick",
		Method:           "historical_tick_amount_cumulative_share",
		UpdatedAt:        time.Now().Format(time.RFC3339),
	}
}

func cumulativeShareAmount(amounts []float64, share float64) float64 {
	if len(amounts) == 0 {
		return 0
	}
	total := 0.0
	for _, amount := range amounts {
		total += amount
	}
	target := total * share
	sum := 0.0
	for _, amount := range amounts {
		sum += amount
		if sum >= target {
			return amount
		}
	}
	return amounts[len(amounts)-1]
}

func writeTradeFlowThresholdCache(cache *TradeFlowThresholdCache) error {
	path := tradeFlowThresholdCachePath(cache.Code)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func tradeFlowThresholdCachePath(code string) string {
	return filepath.Join("data", "trade_flow_thresholds", code+".json")
}

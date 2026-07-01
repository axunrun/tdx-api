package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/injoyai/tdx"
	"github.com/injoyai/tdx/protocol"
)

type AgentIntradayAlerts struct {
	Source        string                   `json:"source"`
	TradingDay    bool                     `json:"tradingDay"`
	WindowMinutes int                      `json:"windowMinutes"`
	Count         int                      `json:"count"`
	Items         []AgentIntradayAlertItem `json:"items"`
	Warnings      []string                 `json:"warnings,omitempty"`
	Note          string                   `json:"note"`
}

type AgentIntradayAlertItem struct {
	Code              string   `json:"code"`
	Name              string   `json:"name,omitempty"`
	LatestTime        string   `json:"latestTime,omitempty"`
	LatestPrice       float64  `json:"latestPrice,omitempty"`
	ChangePct         float64  `json:"changePct,omitempty"`
	OpenChangePct     float64  `json:"openChangePct,omitempty"`
	RecentChangePct   float64  `json:"recentChangePct,omitempty"`
	RecentVolume      int      `json:"recentVolume,omitempty"`
	RecentVolumeRatio float64  `json:"recentVolumeRatio,omitempty"`
	Signals           []string `json:"signals"`
	Text              string   `json:"text"`
	Warnings          []string `json:"warnings,omitempty"`
}

type AgentIntradayAlertsText struct {
	Format  string `json:"format"`
	Content string `json:"content"`
}

func handleAgentIntradayAlerts(w http.ResponseWriter, r *http.Request) {
	summary, ok := loadAgentIntradayAlerts(w, r)
	if !ok {
		return
	}
	jsonResp(w, summary)
}

func handleAgentIntradayAlertsText(w http.ResponseWriter, r *http.Request) {
	summary, ok := loadAgentIntradayAlerts(w, r)
	if !ok {
		return
	}
	jsonResp(w, AgentIntradayAlertsText{
		Format:  "text/plain; charset=utf-8",
		Content: buildAgentIntradayAlertsText(summary),
	})
}

func loadAgentIntradayAlerts(w http.ResponseWriter, r *http.Request) (AgentIntradayAlerts, bool) {
	codes := parseAgentCodeList(r)
	if len(codes) == 0 {
		jsonErr(w, "缺少codes参数")
		return AgentIntradayAlerts{}, false
	}
	if len(codes) > 20 {
		codes = codes[:20]
	}
	windowMinutes := parseCount(r.URL.Query().Get("windowMinutes"), 30)
	if windowMinutes < 5 || windowMinutes > 60 {
		windowMinutes = 30
	}
	c := cli()
	if c == nil {
		jsonErr(w, "TDX客户端未连接")
		return AgentIntradayAlerts{}, false
	}
	return buildAgentIntradayAlerts(c, codes, windowMinutes), true
}

func buildAgentIntradayAlerts(
	c *tdx.Client,
	codes []string,
	windowMinutes int,
) AgentIntradayAlerts {
	items := make([]AgentIntradayAlertItem, 0, len(codes))
	warnings := make([]string, 0)
	tradingDay := detectAgentIntradayTradingDay(c)
	for _, code := range codes {
		item, err := buildAgentIntradayAlertItem(c, code, windowMinutes, tradingDay)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s失败: %v", code, err))
			continue
		}
		items = append(items, item)
	}
	return AgentIntradayAlerts{
		Source:        "tdx_agent_intraday_alerts",
		TradingDay:    tradingDay,
		WindowMinutes: windowMinutes,
		Count:         len(items),
		Items:         items,
		Warnings:      warnings,
		Note:          "盘中异动提醒接口；调用一次计算当前分时状态，不做后台轮询和推送。",
	}
}

func buildAgentIntradayAlertItem(
	c *tdx.Client,
	code string,
	windowMinutes int,
	tradingDay bool,
) (AgentIntradayAlertItem, error) {
	item := AgentIntradayAlertItem{Code: code, Name: queryStockName(code)}
	quoteOK := false
	if quote, err := buildAgentBriefQuote(c, code); err == nil && quote != nil {
		quoteOK = true
		item.ChangePct = quote.ChangePct
		item.LatestPrice = quote.Price
	} else {
		item.Warnings = append(item.Warnings, "实时行情获取失败")
	}
	resp, err := c.GetMinute(code)
	if err != nil || resp == nil || len(resp.List) == 0 {
		item.Warnings = append(item.Warnings, intradayMinuteUnavailableWarning(tradingDay))
		item.Signals = intradaySignals(item)
		item.Text = intradayAlertText(item)
		if !quoteOK {
			return item, fmt.Errorf("GetQuote和GetMinute均失败")
		}
		return item, nil
	}
	applyIntradayMinuteSignals(&item, resp.List, windowMinutes)
	item.Text = intradayAlertText(item)
	return item, nil
}

func detectAgentIntradayTradingDay(c *tdx.Client) bool {
	now := time.Now()
	if now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
		return false
	}
	resp, err := c.GetIndexDayAll("sh000001")
	if err != nil || resp == nil || len(resp.List) == 0 {
		return true
	}
	latest := resp.List[len(resp.List)-1]
	return dateOnly(latest.Time).Equal(dateOnly(now))
}

func intradayMinuteUnavailableWarning(tradingDay bool) string {
	if !tradingDay {
		return "非交易日无可用分时数据，已退化为当前行情快照"
	}
	return "无可用分时数据，已退化为当前行情快照"
}

func applyIntradayMinuteSignals(
	item *AgentIntradayAlertItem,
	minutes []protocol.PriceNumber,
	windowMinutes int,
) {
	cleaned := validMinutePoints(minutes)
	if len(cleaned) == 0 {
		item.Warnings = append(item.Warnings, "无有效分时点")
		return
	}
	first := cleaned[0]
	latest := cleaned[len(cleaned)-1]
	item.LatestTime = latest.Time
	item.LatestPrice = latest.Price.Float64()
	firstPrice := first.Price.Float64()
	if firstPrice > 0 {
		item.OpenChangePct = (item.LatestPrice - firstPrice) / firstPrice * 100
	}
	if len(cleaned) > windowMinutes {
		base := cleaned[len(cleaned)-1-windowMinutes].Price.Float64()
		if base > 0 {
			item.RecentChangePct = (item.LatestPrice - base) / base * 100
		}
	}
	item.RecentVolume, item.RecentVolumeRatio = intradayRecentVolume(cleaned, windowMinutes)
	item.Signals = intradaySignals(*item)
}

func validMinutePoints(minutes []protocol.PriceNumber) []protocol.PriceNumber {
	out := make([]protocol.PriceNumber, 0, len(minutes))
	for _, point := range minutes {
		if point.Price > 0 {
			out = append(out, point)
		}
	}
	return out
}

func intradayRecentVolume(
	minutes []protocol.PriceNumber,
	windowMinutes int,
) (int, float64) {
	if windowMinutes <= 0 || len(minutes) == 0 {
		return 0, 0
	}
	if windowMinutes > len(minutes) {
		windowMinutes = len(minutes)
	}
	recentStart := len(minutes) - windowMinutes
	recent := sumMinuteNumber(minutes[recentStart:])
	prevStart := recentStart - windowMinutes
	if prevStart < 0 {
		prevStart = 0
	}
	prev := sumMinuteNumber(minutes[prevStart:recentStart])
	ratio := 0.0
	if prev > 0 {
		ratio = float64(recent) / float64(prev)
	}
	return recent, ratio
}

func sumMinuteNumber(minutes []protocol.PriceNumber) int {
	sum := 0
	for _, point := range minutes {
		if point.Number > 0 {
			sum += point.Number
		}
	}
	return sum
}

func intradaySignals(item AgentIntradayAlertItem) []string {
	signals := make([]string, 0)
	if item.ChangePct >= 5 {
		signals = append(signals, "当日强势")
	}
	if item.ChangePct <= -5 {
		signals = append(signals, "当日弱势")
	}
	if item.RecentChangePct >= 2 {
		signals = append(signals, "短时拉升")
	}
	if item.RecentChangePct <= -2 {
		signals = append(signals, "短时回落")
	}
	if item.RecentVolumeRatio >= 2 {
		signals = append(signals, "短时放量")
	}
	if len(signals) == 0 {
		signals = append(signals, "无明显异动")
	}
	return signals
}

func intradayAlertText(item AgentIntradayAlertItem) string {
	name := item.Name
	if name == "" {
		name = item.Code
	}
	latestTime := ""
	if item.LatestTime != "" {
		latestTime = " " + item.LatestTime
	}
	if item.LatestTime == "" && item.RecentVolume == 0 && item.RecentVolumeRatio == 0 {
		return fmt.Sprintf(
			"%s（%s），最新%.2f，当日%s；信号：%s",
			name,
			item.Code,
			item.LatestPrice,
			formatPercentText(item.ChangePct),
			strings.Join(item.Signals, "、"),
		)
	}
	return fmt.Sprintf(
		"%s（%s）%s，最新%.2f，当日%s，开盘以来%s，近段%s，近段量比%.2f；信号：%s",
		name,
		item.Code,
		latestTime,
		item.LatestPrice,
		formatPercentText(item.ChangePct),
		formatPercentText(item.OpenChangePct),
		formatPercentText(item.RecentChangePct),
		item.RecentVolumeRatio,
		strings.Join(item.Signals, "、"),
	)
}

func buildAgentIntradayAlertsText(summary AgentIntradayAlerts) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("盘中异动提醒：近%d分钟窗口，共%d只。\n", summary.WindowMinutes, summary.Count))
	for i, item := range summary.Items {
		b.WriteString(fmt.Sprintf("%d. %s\n", i+1, item.Text))
		if len(item.Warnings) > 0 {
			b.WriteString("   提示：" + strings.Join(item.Warnings, "；") + "\n")
		}
	}
	appendWarningsText(&b, summary.Warnings)
	return strings.TrimSpace(b.String())
}

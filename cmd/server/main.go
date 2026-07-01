package main

import (
	"embed"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/injoyai/tdx"
	"github.com/injoyai/tdx/protocol"
)

//go:embed static/*
var staticFiles embed.FS

var (
	mainClient *tdx.Client
	exClient   *tdx.Client
	gbbq       *tdx.Gbbq
	paperStore *PaperStore
	startTime  = time.Now()
)

func main() {
	var err error

	sorted := tdx.SortHosts()
	log.Printf("馃寪 A鑲¤鎯呮湇鍔″櫒娴嬮€熷畬鎴愶紝鍙敤 %d 鍙帮紝棣栭€?%s", len(sorted), sorted[0])

	mainClient, err = tdx.DialDefault(tdx.WithDebug(false))
	if err != nil {
		log.Printf("鈿狅笍 A鑲¤繛鎺ュけ璐? %v", err)
	} else {
		log.Println("鉁?A鑲¤鎯呭凡杩炴帴")
	}

	go func() {
		var err error
		gbbq, err = tdx.NewGbbq(tdx.WithGbbqClient(mainClient))
		if err != nil {
			log.Printf("鈿狅笍 澶嶆潈妯″潡鍒濆鍖栧け璐? %v", err)
			return
		}
		log.Println("gbbq module ready")
		if err := gbbq.Update(); err != nil {
			log.Printf("鈿狅笍 澶嶆潈鏁版嵁鏇存柊澶辫触: %v", err)
		} else {
			log.Println("gbbq data updated")
		}
	}()

	paperDB, err := openPaperDB(paperDBPath())
	if err != nil {
		log.Printf("paper db init failed: %v", err)
	} else {
		paperStore = NewPaperStore(paperDB)
		startPaperBackgroundTasks(paperStore, quotePaperFromTDX)
		log.Println("paper trading store initialized")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleWebUI)
	mux.HandleFunc("/api/quote", handleQuote)
	// quote endpoints
	mux.HandleFunc("/api/kline", handleKline)
	mux.HandleFunc("/api/kline/all", handleKlineAll)
	mux.HandleFunc("/api/kline/qfq", handleQfqKline)
	mux.HandleFunc("/api/kline/hfq", handleHfqKline)
	mux.HandleFunc("/api/minute", handleMinute)
	mux.HandleFunc("/api/trade", handleTrade)
	mux.HandleFunc("/api/call-auction", handleCallAuction)
	// 澶嶆潈绯荤粺
	mux.HandleFunc("/api/gbbq", handleGbbq)
	mux.HandleFunc("/api/adjust-factors", handleFactors)
	mux.HandleFunc("/api/gbbq/adjust", handleGbbqAdjust)
	mux.HandleFunc("/api/gbbq/all", handleGbbqAll)
	mux.HandleFunc("/api/finance", handleFinance)
	// finance endpoints
	mux.HandleFunc("/api/f10", handleF10)
	mux.HandleFunc("/api/agent/technical-summary", handleAgentTechnicalSummary)
	mux.HandleFunc("/api/agent/stock-brief", handleAgentStockBrief)
	mux.HandleFunc("/api/agent/stock-brief-text", handleAgentStockBriefText)
	mux.HandleFunc("/api/agent/f10-summary", handleAgentF10Summary)
	mux.HandleFunc("/api/agent/f10-summary-text", handleAgentF10SummaryText)
	mux.HandleFunc("/api/agent/assets/search", handleAgentAssetsSearch)
	mux.HandleFunc("/api/agent/assets/search-text", handleAgentAssetsSearchText)
	mux.HandleFunc("/api/agent/assets/detail", handleAgentAssetsDetail)
	mux.HandleFunc("/api/agent/sector-membership", handleAgentSectorMembership)
	mux.HandleFunc("/api/agent/sector-membership-text", handleAgentSectorMembershipText)
	mux.HandleFunc("/api/agent/stock-in-sector", handleAgentStockInSector)
	mux.HandleFunc("/api/agent/stock-in-sector-text", handleAgentStockInSectorText)
	mux.HandleFunc("/api/agent/sector-detail", handleAgentSectorDetail)
	mux.HandleFunc("/api/agent/sector-detail-text", handleAgentSectorDetailText)
	mux.HandleFunc("/api/agent/hotspot-scan", handleAgentHotspotScan)
	mux.HandleFunc("/api/agent/hotspot-scan-text", handleAgentHotspotScanText)
	mux.HandleFunc("/api/agent/multi-brief", handleAgentMultiBrief)
	mux.HandleFunc("/api/agent/multi-brief-text", handleAgentMultiBriefText)
	mux.HandleFunc("/api/agent/auction", handleAgentAuction)
	mux.HandleFunc("/api/agent/auction-text", handleAgentAuctionText)
	mux.HandleFunc("/api/agent/market-review", handleAgentMarketReview)
	mux.HandleFunc("/api/agent/market-review-text", handleAgentMarketReviewText)
	mux.HandleFunc("/api/agent/intraday-alerts", handleAgentIntradayAlerts)
	mux.HandleFunc("/api/agent/intraday-alerts-text", handleAgentIntradayAlertsText)
	mux.HandleFunc("/api/agent/global-market-brief", handleAgentGlobalMarketBrief)
	mux.HandleFunc("/api/agent/global-market-brief-text", handleAgentGlobalMarketBriefText)
	mux.HandleFunc("/api/agent/kline-summary", handleAgentKlineSummary)
	mux.HandleFunc("/api/agent/kline-summary-text", handleAgentKlineSummaryText)
	mux.HandleFunc("/api/agent/trade-flow-estimate", handleAgentTradeFlowEstimate)
	mux.HandleFunc("/api/agent/trade-flow-estimate-text", handleAgentTradeFlowEstimateText)
	mux.HandleFunc("/api/paper/dashboard", handlePaperDashboard)
	mux.HandleFunc("/api/paper/accounts", handlePaperAccounts)
	mux.HandleFunc("/api/paper/account", handlePaperAccount)
	mux.HandleFunc("/api/paper/activity", handlePaperActivity)
	mux.HandleFunc("/api/paper/closed-positions", handlePaperClosedPositions)
	// MCP
	mux.HandleFunc("/mcp", handleMCP)
	mux.HandleFunc("/api/stat", handleStat)
	// market statistics endpoints
	mux.HandleFunc("/api/moneyflow", handleMoneyflow)
	mux.HandleFunc("/api/blocks", handleBlocks)
	mux.HandleFunc("/api/hy", handleHy)
	mux.HandleFunc("/api/codes", handleCodes)
	mux.HandleFunc("/api/codes/etf", handleCodesETF)
	mux.HandleFunc("/api/codes/index", handleCodesIndex)
	// 鎸囨暟
	mux.HandleFunc("/api/index/kline", handleIndexKline)
	mux.HandleFunc("/api/index/all", handleIndexKlineAll)
	// 宸ュ叿
	mux.HandleFunc("/api/search", handleSearch)
	mux.HandleFunc("/api/history-trade", handleHistoryTrade)
	mux.HandleFunc("/api/stocks/refresh", handleStocksRefresh)
	// stock sqlite endpoints
	mux.HandleFunc("/api/stocks/search", handleStocksSearch)
	mux.HandleFunc("/api/admin/trade-flow-thresholds/refresh", handleAdminTradeFlowThresholdsRefresh)

	port := "8080"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}
	nEndpoints := 51
	log.Printf("馃殌 TDX API Server v2.1 鍑嗗鐩戝惉 :%s (%d endpoints)", port, nEndpoints)

	startBackgroundInitializers(
		func() { log.Println("鉁?SQLite 鍚庡彴鍒濆鍖栦换鍔″凡瀹屾垚") },
		func() { initStocksDB(mainClient) },
		func() { initBlocksDB(mainClient) },
	)

	log.Printf("鉁?TDX API Server v2.1 宸插紑濮嬬洃鍚?:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

func startBackgroundInitializers(onDone func(), tasks ...func()) {
	if len(tasks) == 0 {
		return
	}
	go func() {
		var wg sync.WaitGroup
		wg.Add(len(tasks))
		for _, task := range tasks {
			task := task
			go func() {
				defer wg.Done()
				if task != nil {
					task()
				}
			}()
		}
		wg.Wait()
		if onDone != nil {
			onDone()
		}
	}()
}

func cli() *tdx.Client   { return mainClient }
func getEx() *tdx.Client { return exClient }
func getGbbq() *tdx.Gbbq { return gbbq }

func parseMkt(s string) uint8 {
	switch s {
	case "0", "sz":
		return 0
	case "2", "bj":
		return 2
	default:
		return 1
	}
}

func parseExchange(s string) protocol.Exchange {
	switch s {
	case "0", "sz":
		return protocol.ExchangeSZ
	case "2", "bj":
		return protocol.ExchangeBJ
	default:
		return protocol.ExchangeSH
	}
}

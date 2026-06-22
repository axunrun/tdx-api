package main

import (
	"embed"
	"log"
	"net/http"
	"os"
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
	startTime  = time.Now()
)

func main() {
	var err error

	// 测速排序：按 TCP 握手耗时筛选可用地址，最快排最前
	sorted := tdx.SortHosts()
	log.Printf("🌐 A股行情服务器测速完成，可用 %d 台，首选 %s", len(sorted), sorted[0])

	mainClient, err = tdx.DialDefault(tdx.WithDebug(false))
	if err != nil {
		log.Printf("⚠️ A股连接失败: %v", err)
	} else {
		log.Println("✅ A股行情已连接")
	}

	go func() {
		var err error
		gbbq, err = tdx.NewGbbq(tdx.WithGbbqClient(mainClient))
		if err != nil {
			log.Printf("⚠️ 复权模块初始化失败: %v", err)
			return
		}
		log.Println("✅ 复权模块已就绪")
		if err := gbbq.Update(); err != nil {
			log.Printf("⚠️ 复权数据更新失败: %v", err)
		} else {
			log.Println("✅ 复权数据已更新")
		}
	}()

	go func() {
		// 测速排序：扩展行情服务器按 TCP 速度重排
		exSorted := tdx.SortExHosts()
		log.Printf("🌐 扩展行情服务器测速完成，可用 %d 台，首选 %s", len(exSorted), exSorted[0])

		for i := 0; i < 5; i++ {
			exClient, err = tdx.DialExHqDefault()
			if err == nil {
				log.Println("✅ 扩展行情(期货/港股/外盘)已连接")
				return
			}
			log.Printf("⏳ 扩展行情重试(%d/5): %v", i+1, err)
			time.Sleep(3 * time.Second)
		}
		log.Printf("⚠️ 扩展行情连接失败: %v", err)
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleWebUI)
	mux.HandleFunc("/api/quote", handleQuote)
	mux.HandleFunc("/api/kline", handleKline)
	mux.HandleFunc("/api/kline/all", handleKlineAll)
	mux.HandleFunc("/api/kline/qfq", handleQfqKline)
	mux.HandleFunc("/api/kline/hfq", handleHfqKline)
	mux.HandleFunc("/api/minute", handleMinute)
	mux.HandleFunc("/api/trade", handleTrade)
	mux.HandleFunc("/api/call-auction", handleCallAuction)
	mux.HandleFunc("/api/finance", handleFinance)
	mux.HandleFunc("/api/f10", handleF10)
	mux.HandleFunc("/api/gbbq", handleGbbq)
	mux.HandleFunc("/api/adjust-factors", handleFactors)
	mux.HandleFunc("/api/stat", handleStat)
	mux.HandleFunc("/api/moneyflow", handleMoneyflow)
	mux.HandleFunc("/api/blocks", handleBlocks)
	mux.HandleFunc("/api/hy", handleHy)
	mux.HandleFunc("/api/xgsg", handleXgsg)
	mux.HandleFunc("/api/codes", handleCodes)
	mux.HandleFunc("/api/index/kline", handleIndexKline)
	mux.HandleFunc("/api/index/all", handleIndexKlineAll)
	mux.HandleFunc("/api/ex/quote", handleExQuote)
	mux.HandleFunc("/api/ex/bars", handleExBars)
	mux.HandleFunc("/api/ex/minute", handleExMinute)
	mux.HandleFunc("/api/ex/trade", handleExTrade)
	mux.HandleFunc("/api/ex/markets", handleExMarkets)
	mux.HandleFunc("/api/ex/instruments", handleExInstruments)
	mux.HandleFunc("/api/search", handleSearch)
	mux.HandleFunc("/api/workday", handleWorkday)
	mux.HandleFunc("/api/workday/range", handleWorkdayRange)
	mux.HandleFunc("/api/history-trade", handleHistoryTrade)
	mux.HandleFunc("/api/income", handleIncome)
	mux.HandleFunc("/api/health", handleHealth)
	mux.HandleFunc("/api/server-status", handleServerStatus)
	mux.HandleFunc("/api/gbbq/adjust", handleGbbqAdjust)
	tmux.HandleFunc("/api/zhb", handleZhb)
	tmux.HandleFunc("/api/report", handleReport)


	port := "8080"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}
	log.Printf("🚀 TDX API Server v2.0 启动于 :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

func cli() *tdx.Client         { return mainClient }
func getEx() *tdx.Client       { return exClient }
func getGbbq() *tdx.Gbbq       { return gbbq }

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

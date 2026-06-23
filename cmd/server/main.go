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

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleWebUI)
	// A股行情
	mux.HandleFunc("/api/quote", handleQuote)
	mux.HandleFunc("/api/kline", handleKline)
	mux.HandleFunc("/api/kline/all", handleKlineAll)
	mux.HandleFunc("/api/kline/qfq", handleQfqKline)
	mux.HandleFunc("/api/kline/hfq", handleHfqKline)
	mux.HandleFunc("/api/minute", handleMinute)
	mux.HandleFunc("/api/trade", handleTrade)
	mux.HandleFunc("/api/call-auction", handleCallAuction)
	// 复权系统
	mux.HandleFunc("/api/gbbq", handleGbbq)
	mux.HandleFunc("/api/adjust-factors", handleFactors)
	mux.HandleFunc("/api/gbbq/adjust", handleGbbqAdjust)
	mux.HandleFunc("/api/gbbq/all", handleGbbqAll)
	// 基本面
	mux.HandleFunc("/api/finance", handleFinance)
	mux.HandleFunc("/api/f10", handleF10)
	// 全市场统计
	mux.HandleFunc("/api/stat", handleStat)
	mux.HandleFunc("/api/moneyflow", handleMoneyflow)
	mux.HandleFunc("/api/blocks", handleBlocks)
	mux.HandleFunc("/api/hy", handleHy)
	mux.HandleFunc("/api/codes", handleCodes)
	mux.HandleFunc("/api/codes/etf", handleCodesETF)
	mux.HandleFunc("/api/codes/index", handleCodesIndex)
	// 指数
	mux.HandleFunc("/api/index/kline", handleIndexKline)
	mux.HandleFunc("/api/index/all", handleIndexKlineAll)
	// 工具
	mux.HandleFunc("/api/search", handleSearch)
	mux.HandleFunc("/api/history-trade", handleHistoryTrade)

	port := "8080"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}
	nEndpoints := 24
	log.Printf("🚀 TDX API Server v2.1 启动于 :%s (%d endpoints)", port, nEndpoints)
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

package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/injoyai/tdx/protocol"
)

func TestLiveIndicatorAggregatesMatchIndependentCalculation(t *testing.T) {
	baseURL := os.Getenv("TDX_LIVE_BASE_URL")
	if baseURL == "" {
		t.Skip("set TDX_LIVE_BASE_URL to run live indicator validation")
	}

	klines := fetchLiveValidationKlines(t, baseURL, "603063", agentIndicatorWarmupBars)
	directRSI := independentWilderRSI(klines, 6)
	directK, directD, directJ := independentKDJ(klines, 9)
	directATR := independentWilderATR(klines, 14)
	directDIF, directDEA, directHist := independentMACD(klines)
	directUpper, directMiddle, directLower := independentBOLL(klines, 20)
	technical := buildAgentTechnicalPeriod("day", "日线", klines)
	summary := buildAgentKlinePeriodSummary("day", "日线", klines, 120)
	kdjRow := scoreKDJ("日线", klines)

	assertClose(t, "technical-summary RSI6", *technical.RSI["rsi6"].Value, directRSI)
	assertClose(t, "kline-summary RSI6", *summary.RSI6, directRSI)
	assertClose(t, "technical-summary ATR14", *technical.ATR.ATR14, directATR)
	assertClose(t, "kline-summary ATR14", summary.Volatility.Atr, directATR)
	assertTolerance(t, "technical-summary MACD DIF", *technical.MACD.DIF, directDIF, 0.01)
	assertTolerance(t, "technical-summary MACD DEA", *technical.MACD.DEA, directDEA, 0.01)
	assertTolerance(t, "technical-summary MACD hist", *technical.MACD.Hist, directHist, 0.01)
	assertTolerance(t, "technical-summary BOLL upper", *technical.BOLL.Upper, directUpper, 0.01)
	assertTolerance(t, "technical-summary BOLL middle", *technical.BOLL.Middle, directMiddle, 0.01)
	assertTolerance(t, "technical-summary BOLL lower", *technical.BOLL.Lower, directLower, 0.01)
	wantKDJ := fmt.Sprintf("K=%.2f D=%.2f J=%.2f", directK, directD, directJ)
	if kdjRow.Value != wantKDJ {
		t.Fatalf("technical-score KDJ = %q, want %q", kdjRow.Value, wantKDJ)
	}

	previousK, previousD, _ := independentKDJ(klines[:len(klines)-1], 9)
	wantSignal, _ := kdjSignal(previousK, previousD, directK, directD)
	if kdjRow.Signal != wantSignal {
		t.Fatalf("technical-score KDJ signal = %q, want %q", kdjRow.Signal, wantSignal)
	}

	t.Logf(
		"direct RSI6=%.4f K=%.4f D=%.4f J=%.4f ATR14=%.4f MACD=%.4f/%.4f/%.4f BOLL=%.4f/%.4f/%.4f; aggregates RSI6=%.4f/%.4f ATR14=%.4f/%.4f KDJ=%s signal=%s",
		directRSI,
		directK,
		directD,
		directJ,
		directATR,
		directDIF,
		directDEA,
		directHist,
		directUpper,
		directMiddle,
		directLower,
		*technical.RSI["rsi6"].Value,
		*summary.RSI6,
		*technical.ATR.ATR14,
		summary.Volatility.Atr,
		kdjRow.Value,
		kdjRow.Signal,
	)
}

func fetchLiveValidationKlines(
	t *testing.T,
	baseURL string,
	code string,
	count int,
) protocol.Klines {
	t.Helper()
	client := &http.Client{Timeout: 20 * time.Second}
	url := fmt.Sprintf("%s/api/kline?code=%s&type=day&count=%d", baseURL, code, count)
	response, err := client.Get(url)
	if err != nil {
		t.Fatalf("get live K-lines: %v", err)
	}
	defer response.Body.Close()

	var payload struct {
		Code int `json:"code"`
		Data struct {
			List []struct {
				Date   string  `json:"date"`
				Open   float64 `json:"open"`
				High   float64 `json:"high"`
				Low    float64 `json:"low"`
				Close  float64 `json:"close"`
				Volume int64   `json:"volume"`
				Amount float64 `json:"amount"`
			} `json:"list"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode live K-lines: %v", err)
	}
	if payload.Code != 0 || len(payload.Data.List) != count {
		t.Fatalf("unexpected live K-lines: code=%d count=%d", payload.Code, len(payload.Data.List))
	}

	klines := make(protocol.Klines, 0, len(payload.Data.List))
	for _, item := range payload.Data.List {
		date, err := time.Parse("2006-01-02", item.Date)
		if err != nil {
			t.Fatalf("parse K-line date %q: %v", item.Date, err)
		}
		klines = append(klines, &protocol.Kline{
			Open:   protocol.Yuan(item.Open),
			High:   protocol.Yuan(item.High),
			Low:    protocol.Yuan(item.Low),
			Close:  protocol.Yuan(item.Close),
			Volume: item.Volume,
			Amount: protocol.Yuan(item.Amount),
			Time:   date,
		})
	}
	return klines
}

func independentWilderRSI(klines protocol.Klines, period int) float64 {
	avgGain, avgLoss := 0.0, 0.0
	for i := 1; i <= period; i++ {
		diff := klines[i].Close.Float64() - klines[i-1].Close.Float64()
		avgGain += math.Max(diff, 0)
		avgLoss += math.Max(-diff, 0)
	}
	avgGain /= float64(period)
	avgLoss /= float64(period)
	for i := period + 1; i < len(klines); i++ {
		diff := klines[i].Close.Float64() - klines[i-1].Close.Float64()
		avgGain = (avgGain*float64(period-1) + math.Max(diff, 0)) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + math.Max(-diff, 0)) / float64(period)
	}
	if avgLoss == 0 {
		return 100
	}
	return avgGain / (avgGain + avgLoss) * 100
}

func independentKDJ(klines protocol.Klines, period int) (float64, float64, float64) {
	k, d := 50.0, 50.0
	for end := period - 1; end < len(klines); end++ {
		high := klines[end-period+1].High.Float64()
		low := klines[end-period+1].Low.Float64()
		for i := end - period + 2; i <= end; i++ {
			high = math.Max(high, klines[i].High.Float64())
			low = math.Min(low, klines[i].Low.Float64())
		}
		rsv := 50.0
		if high != low {
			rsv = (klines[end].Close.Float64() - low) / (high - low) * 100
		}
		k = k*2/3 + rsv/3
		d = d*2/3 + k/3
	}
	return k, d, 3*k - 2*d
}

func independentWilderATR(klines protocol.Klines, period int) float64 {
	trueRange := func(i int) float64 {
		high := klines[i].High.Float64()
		low := klines[i].Low.Float64()
		previousClose := klines[i-1].Close.Float64()
		return math.Max(high-low, math.Max(
			math.Abs(high-previousClose),
			math.Abs(low-previousClose),
		))
	}
	atr := 0.0
	for i := 1; i <= period; i++ {
		atr += trueRange(i)
	}
	atr /= float64(period)
	for i := period + 1; i < len(klines); i++ {
		atr = (atr*float64(period-1) + trueRange(i)) / float64(period)
	}
	return atr
}

func independentMACD(klines protocol.Klines) (float64, float64, float64) {
	ema12 := klines[0].Close.Float64()
	ema26 := ema12
	dea := 0.0
	dif := 0.0
	for i := 1; i < len(klines); i++ {
		closePrice := klines[i].Close.Float64()
		ema12 = closePrice*2/13 + ema12*11/13
		ema26 = closePrice*2/27 + ema26*25/27
		dif = ema12 - ema26
		dea = dif*2/10 + dea*8/10
	}
	return dif, dea, (dif - dea) * 2
}

func independentBOLL(klines protocol.Klines, period int) (float64, float64, float64) {
	start := len(klines) - period
	middle := 0.0
	for _, item := range klines[start:] {
		middle += item.Close.Float64()
	}
	middle /= float64(period)
	variance := 0.0
	for _, item := range klines[start:] {
		variance += math.Pow(item.Close.Float64()-middle, 2)
	}
	standardDeviation := math.Sqrt(variance / float64(period))
	return middle + 2*standardDeviation, middle, middle - 2*standardDeviation
}

func assertClose(t *testing.T, name string, got float64, want float64) {
	t.Helper()
	assertTolerance(t, name, got, want, 0.0001)
}

func assertTolerance(t *testing.T, name string, got float64, want float64, tolerance float64) {
	t.Helper()
	if math.Abs(got-want) > tolerance {
		t.Fatalf("%s = %.6f, want %.6f", name, got, want)
	}
}

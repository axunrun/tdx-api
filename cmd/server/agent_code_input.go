package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"unicode"
)

func normalizeAgentCodeRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if err := normalizeAgentCodeQuery(query); err != nil {
			jsonErr(w, err.Error())
			return
		}
		r.URL.RawQuery = query.Encode()
		next.ServeHTTP(w, r)
	})
}

func normalizeAgentHTTPRequests(next http.Handler) http.Handler {
	normalized := normalizeAgentCodeRequest(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/agent/") {
			normalized.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func normalizeAgentCodeQuery(query url.Values) error {
	if rawCodes := query["code"]; len(rawCodes) > 0 {
		normalizedCodes := make([]string, 0, len(rawCodes))
		firstMarket := ""
		for _, rawCode := range rawCodes {
			if strings.TrimSpace(rawCode) == "" {
				continue
			}
			code, market, err := normalizeAgentStockCode(rawCode, query.Get("mkt"))
			if err != nil {
				return err
			}
			if firstMarket == "" {
				firstMarket = market
			}
			normalizedCodes = append(normalizedCodes, code)
		}
		if len(normalizedCodes) > 0 {
			query["code"] = normalizedCodes
			if len(normalizedCodes) == 1 || query.Get("mkt") != "" {
				query.Set("mkt", firstMarket)
			}
		}
	}

	if rawCodes := strings.TrimSpace(query.Get("codes")); rawCodes != "" {
		codes := make([]string, 0)
		for _, rawCode := range splitAgentStockCodes(rawCodes) {
			code, _, err := normalizeAgentStockCode(rawCode, "")
			if err != nil {
				return err
			}
			codes = append(codes, code)
		}
		query.Set("codes", strings.Join(codes, ","))
	}
	return nil
}

func normalizeMCPCodeArguments(args map[string]any) error {
	query := url.Values{}
	if value, exists := args["code"]; exists && strings.TrimSpace(fmt.Sprint(value)) != "" {
		query.Set("code", fmt.Sprint(value))
		if market, ok := args["mkt"]; ok {
			query.Set("mkt", fmt.Sprint(market))
		}
	}
	if value, exists := args["codes"]; exists && strings.TrimSpace(fmt.Sprint(value)) != "" {
		query.Set("codes", fmt.Sprint(value))
	}
	if err := normalizeAgentCodeQuery(query); err != nil {
		return err
	}
	if code := query.Get("code"); code != "" {
		args["code"] = code
		args["mkt"] = query.Get("mkt")
	}
	if codes := query.Get("codes"); codes != "" {
		args["codes"] = codes
	}
	return nil
}

func normalizeAgentStockCode(rawCode, rawMarket string) (string, string, error) {
	code := strings.ToLower(strings.TrimSpace(rawCode))
	embeddedMarket := ""
	switch {
	case len(code) == 8 && isAgentMarket(code[:2]):
		embeddedMarket, code = code[:2], code[2:]
	case len(code) == 9 && code[6] == '.' && isAgentMarket(code[7:]):
		embeddedMarket, code = code[7:], code[:6]
	}
	if len(code) != 6 || !allASCIIDigits(code) {
		return "", "", fmt.Errorf(
			"code格式错误：%q；支持300476、sz300476或300476.SZ",
			rawCode,
		)
	}

	explicitMarket, err := normalizeAgentMarket(rawMarket)
	if err != nil {
		return "", "", fmt.Errorf("code格式错误：%w", err)
	}
	if explicitMarket != "" && embeddedMarket != "" && explicitMarket != embeddedMarket {
		return "", "", fmt.Errorf(
			"code格式错误：代码市场%s与mkt=%s冲突",
			embeddedMarket,
			explicitMarket,
		)
	}
	market := explicitMarket
	if market == "" {
		market = embeddedMarket
	}
	if market == "" {
		market = exchangeNameForCode(code, "")
	}
	return code, market, nil
}

func normalizeAgentMarket(rawMarket string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(rawMarket)) {
	case "":
		return "", nil
	case "0", "sz":
		return "sz", nil
	case "1", "sh":
		return "sh", nil
	case "2", "bj":
		return "bj", nil
	default:
		return "", fmt.Errorf("mkt仅支持sh、sz或bj")
	}
}

func splitAgentStockCodes(rawCodes string) []string {
	return strings.FieldsFunc(rawCodes, func(r rune) bool {
		return r == ',' || r == '，' || unicode.IsSpace(r)
	})
}

func isAgentMarket(value string) bool {
	return value == "sh" || value == "sz" || value == "bj"
}

func allASCIIDigits(value string) bool {
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

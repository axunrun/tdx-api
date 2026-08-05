package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNormalizeAgentCodeRequestAcceptsCommonAStockFormats(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		wantCode   string
		wantMarket string
	}{
		{name: "plain", query: "code=300476", wantCode: "300476", wantMarket: "sz"},
		{name: "prefix", query: "code=SZ300476", wantCode: "300476", wantMarket: "sz"},
		{name: "suffix", query: "code=300476.SZ", wantCode: "300476", wantMarket: "sz"},
		{name: "explicit market", query: "code=300476&mkt=sz", wantCode: "300476", wantMarket: "sz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotCode, gotMarket string
			handler := normalizeAgentCodeRequest(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotCode = r.URL.Query().Get("code")
				gotMarket = r.URL.Query().Get("mkt")
				w.WriteHeader(http.StatusNoContent)
			}))
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/agent/test?"+tt.query, nil)

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusNoContent {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if gotCode != tt.wantCode || gotMarket != tt.wantMarket {
				t.Fatalf("got code=%q mkt=%q, want code=%q mkt=%q", gotCode, gotMarket, tt.wantCode, tt.wantMarket)
			}
		})
	}
}

func TestNormalizeAgentCodeRequestRejectsInvalidOrConflictingInput(t *testing.T) {
	for _, query := range []string{
		"code=300476.SZ&mkt=sh",
		"code=30047X",
		"code=300476.US",
	} {
		t.Run(query, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/agent/test?"+query, nil)
			normalizeAgentCodeRequest(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Fatal("invalid request reached handler")
			})).ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "code格式") {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestNormalizeAgentCodeRequestNormalizesCodeLists(t *testing.T) {
	var got string
	handler := normalizeAgentCodeRequest(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query().Get("codes")
		w.WriteHeader(http.StatusNoContent)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/agent/test?codes=SZ300476,600000.SH,bj430047",
		nil,
	)

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent || got != "300476,600000,430047" {
		t.Fatalf("status = %d, codes = %q, body = %s", rec.Code, got, rec.Body.String())
	}
}

func TestNormalizeAgentCodeRequestPreservesRepeatedCodeParameters(t *testing.T) {
	var got []string
	handler := normalizeAgentCodeRequest(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()["code"]
		w.WriteHeader(http.StatusNoContent)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/agent/multi-brief?code=SZ300476&code=600000.SH",
		nil,
	)

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent || strings.Join(got, ",") != "300476,600000" {
		t.Fatalf("status = %d, codes = %v, body = %s", rec.Code, got, rec.Body.String())
	}
}

func TestNormalizeMCPCodeArgumentsUsesSameContract(t *testing.T) {
	args := map[string]any{
		"code":  "300476.SZ",
		"codes": "sh600000,SZ300476",
	}
	if err := normalizeMCPCodeArguments(args); err != nil {
		t.Fatal(err)
	}
	if args["code"] != "300476" || args["mkt"] != "sz" ||
		args["codes"] != "600000,300476" {
		t.Fatalf("unexpected normalized MCP args: %+v", args)
	}
}

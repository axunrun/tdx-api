package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMCPInitializeAndToolsList(t *testing.T) {
	initBody := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`)
	initReq := httptest.NewRequest(http.MethodPost, "/mcp", initBody)
	initRec := httptest.NewRecorder()
	handleMCP(initRec, initReq)
	if initRec.Code != http.StatusOK {
		t.Fatalf("initialize status=%d body=%s", initRec.Code, initRec.Body.String())
	}

	var initResp map[string]any
	if err := json.Unmarshal(initRec.Body.Bytes(), &initResp); err != nil {
		t.Fatal(err)
	}
	if initResp["error"] != nil {
		t.Fatalf("initialize returned error: %v", initResp["error"])
	}

	listBody := bytes.NewBufferString(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	listReq := httptest.NewRequest(http.MethodPost, "/mcp", listBody)
	listRec := httptest.NewRecorder()
	handleMCP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("tools/list status=%d body=%s", listRec.Code, listRec.Body.String())
	}

	var listResp struct {
		Result struct {
			Tools []mcpTool `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listResp); err != nil {
		t.Fatal(err)
	}
	if len(listResp.Result.Tools) == 0 {
		t.Fatal("tools/list returned no tools")
	}
	seen := map[string]bool{}
	for _, tool := range listResp.Result.Tools {
		if tool.Name == "" || tool.Description == "" {
			t.Fatalf("tool missing name or description: %+v", tool)
		}
		if seen[tool.Name] {
			t.Fatalf("duplicate tool name: %s", tool.Name)
		}
		seen[tool.Name] = true
	}
	if !seen["tdx_stock_brief_text"] || !seen["tdx_global_market_brief_text"] {
		t.Fatalf("expected core tools missing: %+v", seen)
	}
}

func TestMCPCallUnknownToolReturnsError(t *testing.T) {
	body := bytes.NewBufferString(`{
		"jsonrpc":"2.0",
		"id":3,
		"method":"tools/call",
		"params":{"name":"missing_tool","arguments":{}}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/mcp", body)
	rec := httptest.NewRecorder()
	handleMCP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var resp mcpResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error == nil {
		t.Fatalf("expected error, got %s", rec.Body.String())
	}
}

package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleWebUI(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		wantStatus  int
		contentType string
		contains    string
	}{
		{
			name:        "index",
			path:        "/",
			wantStatus:  http.StatusOK,
			contentType: "text/html",
			contains:    "只读模拟交易看板",
		},
		{
			name:        "css",
			path:        "/static/styles.css",
			wantStatus:  http.StatusOK,
			contentType: "text/css",
			contains:    ":root",
		},
		{
			name:        "js",
			path:        "/static/app.js",
			wantStatus:  http.StatusOK,
			contentType: "application/javascript",
			contains:    "/api/paper/dashboard?range=20d",
		},
		{
			name:       "not found",
			path:       "/static/missing.js",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)

			handleWebUI(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.contentType != "" && !strings.HasPrefix(
				rec.Header().Get("Content-Type"),
				tt.contentType,
			) {
				t.Fatalf("content type = %q, want prefix %q",
					rec.Header().Get("Content-Type"), tt.contentType)
			}
			if tt.contains != "" && !strings.Contains(rec.Body.String(), tt.contains) {
				t.Fatalf("body does not contain %q", tt.contains)
			}
		})
	}
}

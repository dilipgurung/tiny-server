package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLiveReloadMiddleware(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		method      string
		content     string
		contentType string
		wantScript  bool
		wantStatus  int
	}{
		{
			name:        "HTML file should inject script",
			path:        "/test.html",
			method:      "GET",
			content:     "<html><body>Test</body></html>",
			contentType: "text/html",
			wantScript:  true,
			wantStatus:  http.StatusOK,
		},
		{
			name:        "Non-HTML file should not inject script",
			path:        "/style.css",
			method:      "GET",
			content:     "body { color: red; }",
			contentType: "text/css",
			wantScript:  false,
			wantStatus:  http.StatusOK,
		},
		{
			name:        "POST request should not inject script",
			path:        "/test.html",
			method:      "POST",
			content:     "<html><body>Test</body></html>",
			contentType: "text/html",
			wantScript:  false,
			wantStatus:  http.StatusOK,
		},
		{
			name:        "API path should not inject script",
			path:        "/api/data",
			method:      "GET",
			content:     `{"data": "test"}`,
			contentType: "application/json",
			wantScript:  false,
			wantStatus:  http.StatusOK,
		},
	}

	hub := NewWebSocketHub()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock file server
			mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tt.contentType)
				w.WriteHeader(tt.wantStatus)
				w.Write([]byte(tt.content))
			})

			handler := liveReload(mockHandler, hub, "8080")
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, rr.Code)
			}

			body := rr.Body.String()
			hasScript := strings.Contains(body, "connectWebSocket()")

			if hasScript != tt.wantScript {
				t.Errorf("Script injection mismatch: want %v, got %v", tt.wantScript, hasScript)
			}

		})
	}
}

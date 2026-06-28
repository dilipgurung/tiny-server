package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestLiveReloadSizeGuard verifies that HTML responses larger than
// maxBufferedHTML are streamed without injecting the live-reload script.
func TestLiveReloadSizeGuard(t *testing.T) {
	// Build a body just over the threshold, with a </body> so injection
	// would normally happen.
	inner := strings.Repeat("x", maxBufferedHTML+1)
	body := "<html><body>" + inner + "</body></html>"

	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	})

	handler := liveReload(mockHandler)
	req := httptest.NewRequest(http.MethodGet, "/big.html", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if strings.Contains(rr.Body.String(), "EventSource") {
		t.Errorf("oversized HTML should not have the live-reload script injected")
	}
	if rr.Body.Len() != len(body) {
		t.Errorf("body length = %d, want %d (content must be preserved)", rr.Body.Len(), len(body))
	}
}

// TestLiveReloadUnderSizeGuard verifies that HTML under the threshold still
// gets the script injected.
func TestLiveReloadUnderSizeGuard(t *testing.T) {
	body := "<html><body>" + strings.Repeat("y", 100) + "</body></html>"
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	})

	handler := liveReload(mockHandler)
	req := httptest.NewRequest(http.MethodGet, "/small.html", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !strings.Contains(rr.Body.String(), "EventSource") {
		t.Errorf("small HTML should have the live-reload script injected")
	}
}

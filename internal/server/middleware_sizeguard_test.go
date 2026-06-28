package server

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// TestLiveReloadInjectsLargeHTML verifies that large HTML responses are no
// longer skipped: stream-inject finds </body> regardless of body size and
// injects the live-reload script, while streaming the body through.
func TestLiveReloadInjectsLargeHTML(t *testing.T) {
	inner := strings.Repeat("x", 4<<20) // 4 MB
	body := "<html><body>" + inner + "</body></html>"

	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		w.WriteHeader(http.StatusOK)
		// Write in 32 KB chunks, like io.Copy from http.FileServer.
		chunk := make([]byte, 32<<10)
		for off := 0; off < len(body); off += len(chunk) {
			end := off + len(chunk)
			if end > len(body) {
				end = len(body)
			}
			_, _ = w.Write([]byte(body[off:end]))
		}
	})

	handler := liveReload(mockHandler)
	req := httptest.NewRequest(http.MethodGet, "/big.html", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	out := rr.Body.String()
	if !strings.Contains(out, "EventSource") {
		t.Errorf("large HTML should have the live-reload script injected")
	}
	scriptBeforeBodyClose(t, out)
	if len(out) != len(body)+len(liveReloadScript) {
		t.Errorf("body length = %d, want %d", len(out), len(body)+len(liveReloadScript))
	}
	if cl := rr.Header().Get("Content-Length"); cl != strconv.Itoa(len(body)+len(liveReloadScript)) {
		t.Errorf("Content-Length = %q, want %d", cl, len(body)+len(liveReloadScript))
	}
}

// TestLiveReloadSmallHTMLStillInjected verifies small HTML is still injected.
func TestLiveReloadSmallHTMLStillInjected(t *testing.T) {
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

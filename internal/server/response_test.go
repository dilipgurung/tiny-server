package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// TestFullStackHTMLResponse runs a real .html file through the full
// middleware chain (blockDotfiles -> liveReload -> file server) and asserts
// the response has the correct status, Content-Type, Content-Length, and
// injected live-reload script.
func TestFullStackHTMLResponse(t *testing.T) {
	dir := mkdirFiles(t, map[string]string{
		"index.html": "<html><body>hello</body></html>",
	})
	srv, err := NewServer("0", dir)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/index.html")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html prefix", ct)
	}
	buf := make([]byte, 0, 1024)
	tmp := make([]byte, 1024)
	for {
		n, err := resp.Body.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			break
		}
	}
	if !strings.Contains(string(buf), "EventSource") {
		t.Errorf("response body missing injected live-reload script")
	}
	if cl := resp.Header.Get("Content-Length"); cl != strconv.Itoa(len(buf)) {
		t.Errorf("Content-Length = %q, want %d", cl, len(buf))
	}
}

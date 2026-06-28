package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestLiveReloadInjectsSubdirectoryIndex reproduces the bug where HTML
// served from a subfolder index (e.g. GET /sub/, which http.FileServer
// resolves to sub/index.html) was skipped by the live-reload middleware
// because the request path has no ".html" suffix and is not "/".
func TestLiveReloadInjectsSubdirectoryIndex(t *testing.T) {
	dir := mkdirFiles(t, map[string]string{
		"index.html":     "<html><body>root</body></html>",
		"sub/index.html": "<html><body>sub</body></html>",
		"sub/page.html":  "<html><body>page</body></html>",
		"sub/style.css":  "body { color: red; }",
	})

	srv, err := NewServer("0", dir)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	cases := []struct {
		name   string
		path   string
		want   string
		inject bool
	}{
		{"root index", "/", "root", true},
		{"root index explicit", "/index.html", "root", true},
		{"subfolder index trailing slash", "/sub/", "sub", true},
		{"subfolder explicit html", "/sub/page.html", "page", true},
		{"css passthrough", "/sub/style.css", "body { color: red; }", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := http.Get(ts.URL + tc.path)
			if err != nil {
				t.Fatalf("Get %s: %v", tc.path, err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("GET %s status = %d, want 200", tc.path, resp.StatusCode)
			}

			buf := make([]byte, 0, 4096)
			tmp := make([]byte, 1024)
			for {
				n, rerr := resp.Body.Read(tmp)
				if n > 0 {
					buf = append(buf, tmp[:n]...)
				}
				if rerr != nil {
					break
				}
			}
			body := string(buf)

			if !strings.Contains(body, tc.want) {
				t.Errorf("GET %s body missing %q; got %q", tc.path, tc.want, body)
			}
			if hasScript := strings.Contains(body, "EventSource"); hasScript != tc.inject {
				t.Errorf("GET %s script injected = %v, want %v; body=%q", tc.path, hasScript, tc.inject, body)
			}
		})
	}
}

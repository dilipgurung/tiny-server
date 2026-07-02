package server

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestIsDotfilePath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/", false},
		{"/index.html", false},
		{"/.env", true},
		{"/.gitignore", true},
		{"/.git/config", true},
		{"/sub/.env", true},
		{"/sub/dir/page.html", false},
		{"/.hidden/file.txt", true},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := isDotfilePath(tt.path); got != tt.want {
				t.Errorf("isDotfilePath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestBlockDotfiles(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	handler := blockDotfiles(next)

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantCalled bool
	}{
		{"dotfile env", "/.env", http.StatusForbidden, false},
		{"dotfile git dir", "/.git/config", http.StatusForbidden, false},
		{"dotfile gitignore", "/.gitignore", http.StatusForbidden, false},
		{"nested dotfile", "/sub/.env", http.StatusForbidden, false},
		{"normal file", "/index.html", http.StatusOK, true},
		{"root", "/", http.StatusOK, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called = false
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rr.Code, tt.wantStatus)
			}
			if called != tt.wantCalled {
				t.Errorf("next handler called = %v, want %v", called, tt.wantCalled)
			}
		})
	}
}

// TestBlockDotfilesEndToEnd verifies the full server stack rejects a real
// .env file served from disk.
func TestBlockDotfilesEndToEnd(t *testing.T) {
	dir := mkdirFiles(t, map[string]string{
		".env":       "SECRET=do-not-leak",
		"index.html": "<html><body>hello</body></html>",
	})
	srv, err := NewServer("0", dir)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/.env")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("GET /.env status = %d, want 403", resp.StatusCode)
	}
}

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

// TestLogRequest verifies the logging middleware forwards to the next
// handler, captures the status code, and writes a log line.
func TestLogRequest(t *testing.T) {
	// Capture log output so the test stays quiet.
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(log.Writer())

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusTeapot)
	})
	handler := logRequest(next)

	req := httptest.NewRequest(http.MethodGet, "/some/path", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("next handler was not called")
	}
	if rr.Code != http.StatusTeapot {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusTeapot)
	}
	if !strings.Contains(buf.String(), "/some/path") {
		t.Errorf("log output missing request path; got %q", buf.String())
	}
}

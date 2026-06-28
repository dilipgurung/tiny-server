package server

import (
	"context"
	"net/http"
	"net/http/httptest"
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
	srv, err := NewServer("0", "testdata_dotfiles")
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer srv.Shutdown(context.Background())

	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/.env")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("GET /.env status = %d, want 403", resp.StatusCode)
	}
}
package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newTestServer builds a server over dir and returns its handler, shutting
// the server down when the test ends.
func newTestServer(t *testing.T, dir string) http.Handler {
	t.Helper()
	srv, err := NewServer("0", dir)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { _ = srv.Shutdown(t.Context()) })
	return srv.httpServer.Handler
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

// TestServedPathsCannotEscapeRootViaSymlink verifies that a symlink inside
// the served directory pointing outside it is not followed. http.Dir would
// happily serve the target's contents.
func TestServedPathsCannotEscapeRootViaSymlink(t *testing.T) {
	outside := mkdirFiles(t, map[string]string{
		"secret.txt":     "top secret",
		"sub/nested.txt": "also secret",
	})
	dir := mkdirFiles(t, map[string]string{"index.html": "<html><body>hi</body></html>"})

	if err := os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(dir, "leak")); err != nil {
		t.Fatalf("Symlink file: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "leakdir")); err != nil {
		t.Fatalf("Symlink dir: %v", err)
	}

	h := newTestServer(t, dir)

	for _, path := range []string{"/leak", "/leakdir/secret.txt", "/leakdir/sub/nested.txt"} {
		rec := get(t, h, path)
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want %d (body: %q)", path, rec.Code, http.StatusNotFound, rec.Body.String())
		}
		if body := rec.Body.String(); body != "" && (strings.Contains(body, "top secret") || strings.Contains(body, "also secret")) {
			t.Errorf("GET %s leaked outside-root content: %q", path, body)
		}
	}
}

// TestServedPathsAllowSymlinksInsideRoot verifies containment does not break
// the legitimate case of a symlink whose target stays inside the root.
func TestServedPathsAllowSymlinksInsideRoot(t *testing.T) {
	dir := mkdirFiles(t, map[string]string{"sub/real.txt": "inside"})
	if err := os.Symlink(filepath.Join("sub", "real.txt"), filepath.Join(dir, "link.txt")); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	rec := get(t, newTestServer(t, dir), "/link.txt")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /link.txt = %d, want 200", rec.Code)
	}
	if got := rec.Body.String(); got != "inside" {
		t.Errorf("GET /link.txt body = %q, want %q", got, "inside")
	}
}

// TestServedFilesAndListingsStillWork guards against the containment change
// regressing ordinary serving: files, injected HTML and directory listings.
func TestServedFilesAndListingsStillWork(t *testing.T) {
	dir := mkdirFiles(t, map[string]string{
		"index.html": "<html><body>hi</body></html>",
		"sub/a.txt":  "plain",
		"style.css":  "body{}",
		".env":       "SECRET=1",
	})
	h := newTestServer(t, dir)

	if rec := get(t, h, "/"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "hi") {
		t.Errorf("GET / = %d %q", rec.Code, rec.Body.String())
	}
	if rec := get(t, h, "/sub/a.txt"); rec.Code != http.StatusOK || rec.Body.String() != "plain" {
		t.Errorf("GET /sub/a.txt = %d %q", rec.Code, rec.Body.String())
	}
	if rec := get(t, h, "/sub/"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "a.txt") {
		t.Errorf("GET /sub/ listing = %d %q", rec.Code, rec.Body.String())
	}
	if rec := get(t, h, "/.env"); rec.Code != http.StatusForbidden {
		t.Errorf("GET /.env = %d, want 403", rec.Code)
	}
	if rec := get(t, h, "/missing.txt"); rec.Code != http.StatusNotFound {
		t.Errorf("GET /missing.txt = %d, want 404", rec.Code)
	}
}

// TestNewServerRejectsNonDirectory verifies a bad -d is reported at startup
// rather than turning every request into a 404.
func TestNewServerRejectsNonDirectory(t *testing.T) {
	dir := mkdirFiles(t, map[string]string{"public": "not a directory"})

	if _, err := NewServer("0", filepath.Join(dir, "public")); err == nil {
		t.Error("NewServer with a regular file as the served dir: want error, got nil")
	}
	if _, err := NewServer("0", filepath.Join(dir, "does-not-exist")); err == nil {
		t.Error("NewServer with a missing served dir: want error, got nil")
	}
}

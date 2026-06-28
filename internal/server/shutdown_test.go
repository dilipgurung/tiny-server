package server

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"
)

// TestServerShutdownReturnsPromptlyWithActiveSSE reproduces the
// "Forced shutdown: context deadline exceeded" issue: with a browser tab
// open, the live-reload EventSource holds a long-lived connection that
// http.Server.Shutdown waits for until its deadline. With the fix, the SSE
// handler exits on the server shutdown context, so Shutdown returns
// quickly and without error.
func TestServerShutdownReturnsPromptlyWithActiveSSE(t *testing.T) {
	dir := mkdirFiles(t, map[string]string{
		"index.html": "<html><body>hi</body></html>",
	})

	srv, err := NewServer("0", dir)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	go func() { _ = srv.httpServer.Serve(ln) }()

	base := "http://" + ln.Addr().String()

	// Open a long-lived SSE connection, like an EventSource would.
	sseCtx, sseCancel := context.WithCancel(context.Background())
	defer sseCancel()
	req, _ := http.NewRequestWithContext(sseCtx, http.MethodGet, base+"/.live-reload", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("SSE connect: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Give the handler a moment to register the client (connection in-flight).
	time.Sleep(100 * time.Millisecond)

	// Shutdown must return well before the 3s deadline.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutdownCancel()

	start := time.Now()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("Shutdown returned error: %v (expected nil)", err)
	}
	if elapsed := time.Since(start); elapsed > 1*time.Second {
		t.Errorf("Shutdown took %v, expected < 1s", elapsed)
	}
}

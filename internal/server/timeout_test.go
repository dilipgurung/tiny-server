package server

import (
	"testing"
)

// TestServerHasSlowClientTimeouts verifies the listener is protected against
// clients that hold connections open without completing a request. The
// server binds every interface, so leaving these at zero lets any peer on
// the network exhaust sockets and goroutines.
func TestServerHasSlowClientTimeouts(t *testing.T) {
	dir := mkdirFiles(t, map[string]string{"index.html": "hi"})
	srv, err := NewServer("0", dir)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { _ = srv.Shutdown(t.Context()) })

	if got := srv.httpServer.ReadHeaderTimeout; got <= 0 {
		t.Errorf("ReadHeaderTimeout = %v, want > 0", got)
	}
	if got := srv.httpServer.ReadTimeout; got <= 0 {
		t.Errorf("ReadTimeout = %v, want > 0", got)
	}
	if got := srv.httpServer.IdleTimeout; got <= 0 {
		t.Errorf("IdleTimeout = %v, want > 0", got)
	}
	// A write deadline would truncate live-reload SSE streams.
	if got := srv.httpServer.WriteTimeout; got != 0 {
		t.Errorf("WriteTimeout = %v, want 0 so SSE streams are not cut off", got)
	}
	if srv.httpServer.ReadHeaderTimeout > srv.httpServer.ReadTimeout {
		t.Errorf("ReadHeaderTimeout (%v) should not exceed ReadTimeout (%v)",
			srv.httpServer.ReadHeaderTimeout, srv.httpServer.ReadTimeout)
	}
}

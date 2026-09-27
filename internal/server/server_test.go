package server

import (
	"testing"
)

func TestSetupServer(t *testing.T) {
	server, err := NewServer("8080", ".")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if server.httpServer.Addr != ":8080" {
		t.Errorf("Expected server address :8080, got %s", server.httpServer.Addr)
	}
}

// TestPortReportsBoundPort verifies Port returns the kernel-assigned port
// after Listen, so "-p 0" does not advertise addresses ending in ":0".
func TestPortReportsBoundPort(t *testing.T) {
	dir := mkdirFiles(t, map[string]string{"index.html": "hi"})
	srv, err := NewServer("0", dir)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { _ = srv.Shutdown(t.Context()) })

	if got := srv.Port(); got != "0" {
		t.Errorf("Port before Listen = %q, want %q", got, "0")
	}
	if err := srv.Listen(); err != nil {
		t.Fatalf("Listen: %v", err)
	}
	if got := srv.Port(); got == "0" || got == "" {
		t.Errorf("Port after Listen = %q, want the assigned port", got)
	}
}

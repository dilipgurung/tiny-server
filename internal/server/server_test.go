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

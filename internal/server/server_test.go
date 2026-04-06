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

func TestSSEHub(t *testing.T) {
	hub := NewSSEHub()

	// Test addClient
	ch := make(chan string, 1)
	hub.addClient(ch)
	if len(hub.clients) != 1 {
		t.Errorf("Expected 1 client, got %d", len(hub.clients))
	}

	// Test Broadcast delivers message
	hub.Broadcast("reload")
	if msg := <-ch; msg != "reload" {
		t.Errorf("Expected 'reload', got %q", msg)
	}

	// Test removeClient
	hub.removeClient(ch)
	if len(hub.clients) != 0 {
		t.Errorf("Expected 0 clients, got %d", len(hub.clients))
	}
}

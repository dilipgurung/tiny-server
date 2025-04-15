package server

import (
	"testing"

	"github.com/gorilla/websocket"
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

func TestWebSocketHub(t *testing.T) {
	hub := NewWebSocketHub()

	// Test AddConnection
	testConn := &websocket.Conn{}
	hub.AddConnection(testConn)
	if len(hub.connections) != 1 {
		t.Errorf("Expected 1 connection, got %d", len(hub.connections))
	}

	// Test RemoveConnection
	hub.RemoveConnection(testConn)
	if len(hub.connections) != 0 {
		t.Errorf("Expected 0 connections, got %d", len(hub.connections))
	}
}

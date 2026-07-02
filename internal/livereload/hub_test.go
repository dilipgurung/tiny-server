package livereload

import "testing"

func TestHub(t *testing.T) {
	hub := NewHub()

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

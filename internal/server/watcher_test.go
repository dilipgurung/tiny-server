package server

import "testing"

func TestNewWatcher(t *testing.T) {
	hub := NewWebSocketHub()
	watcher, err := NewWatcher(hub)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Close()

	// Test watching current directory
	err = watcher.WatchDirectory(".")
	if err != nil {
		t.Errorf("Failed to watch directory: %v", err)
	}
}

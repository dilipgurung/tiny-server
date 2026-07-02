package watcher

import "testing"

func TestNewWatcher(t *testing.T) {
	b := &chanBroadcaster{ch: make(chan string, 1)}
	watcher, err := NewWatcher(b)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer func() { _ = watcher.Close() }()

	// Test watching current directory
	err = watcher.WatchDirectory(".")
	if err != nil {
		t.Errorf("Failed to watch directory: %v", err)
	}
}

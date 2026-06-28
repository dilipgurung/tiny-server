package server

import (
	"context"
	"testing"
	"time"
)

// TestWatcherStartCtxStopsOnCancel verifies the event loop goroutine
// exits when the provided context is cancelled.
func TestWatcherStartCtxStopsOnCancel(t *testing.T) {
	hub := NewSSEHub()
	watcher, err := NewWatcher(hub)
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	defer func() { _ = watcher.Close() }()

	if err := watcher.WatchDirectory("testdata_watcher"); err != nil {
		t.Fatalf("WatchDirectory: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		watcher.StartCtx(ctx)
		close(done)
	}()

	cancel()
	select {
	case <-done:
		// goroutine exited as expected
	case <-time.After(time.Second):
		t.Fatal("StartCtx goroutine did not exit after context cancellation")
	}
}

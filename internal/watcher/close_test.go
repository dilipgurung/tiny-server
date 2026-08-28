package watcher

import (
	"path/filepath"
	"testing"
	"time"
)

// TestScheduleReloadAfterCloseDoesNotPanic verifies shutdown is safe while
// the event loop is still processing an event it dequeued beforehand.
// Close nils reloadTimers, so an unguarded scheduleReload would panic with
// "assignment to entry in nil map" -- crashing the process on Ctrl+C
// whenever a file happened to change at the same moment.
func TestScheduleReloadAfterCloseDoesNotPanic(t *testing.T) {
	ch := make(chan string, 4)
	w, err := NewWatcher(&chanBroadcaster{ch: ch})
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	w.debounce = 10 * time.Millisecond

	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Would panic before the closed guard was added.
	w.scheduleReload(filepath.Join(t.TempDir(), "index.html"))

	select {
	case msg := <-ch:
		t.Errorf("broadcast %q after Close; want none", msg)
	case <-time.After(50 * time.Millisecond):
	}
}

// TestCloseDuringFileChangeIsSafe exercises the real race: a live event
// loop watching a directory that is being written while Close runs.
func TestCloseDuringFileChangeIsSafe(t *testing.T) {
	dir := setupWatcherTempDir(t)
	w, err := NewWatcher(&chanBroadcaster{ch: make(chan string, 64)})
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	w.debounce = time.Millisecond
	if err := w.WatchDirectory(dir); err != nil {
		t.Fatalf("WatchDirectory: %v", err)
	}
	w.Start()

	target := filepath.Join(dir, "index.html")
	go func() {
		for i := 0; i < 200; i++ {
			_ = appendFile(target, "x\n")
		}
	}()
	time.Sleep(5 * time.Millisecond)
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	time.Sleep(20 * time.Millisecond)
}

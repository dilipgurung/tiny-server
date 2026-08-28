package watcher

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// setupWatcherTempDir creates a temp directory with a .gitignore and a
// couple of seed files, and returns its path.
func setupWatcherTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("ignored.log\n"), 0644); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("hello\n"), 0644); err != nil {
		t.Fatalf("write index.html: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "note.txt"), []byte("note\n"), 0644); err != nil {
		t.Fatalf("write note.txt: %v", err)
	}
	return dir
}

// TestWatcherDebounceCoalescesBursts verifies that a burst of change events
// for the same file results in a single reload broadcast after the debounce
// window, rather than one broadcast per event.
func TestWatcherDebounceCoalescesBursts(t *testing.T) {
	dir := setupWatcherTempDir(t)

	ch := make(chan string, 16)
	watcher, err := NewWatcher(&chanBroadcaster{ch: ch})
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	defer func() { _ = watcher.Close() }()
	// Use a short debounce to keep the test fast.
	watcher.debounce = 30 * time.Millisecond

	if err := watcher.WatchDirectory(dir); err != nil {
		t.Fatalf("WatchDirectory: %v", err)
	}
	watcher.Start()

	// Write a burst of changes to index.html.
	target := filepath.Join(dir, "index.html")
	for i := 0; i < 5; i++ {
		if err := appendFile(target, "burst\n"); err != nil {
			t.Fatalf("write: %v", err)
		}
		time.Sleep(5 * time.Millisecond)
	}

	// Collect reloads over a window comfortably longer than the debounce.
	deadline := time.After(300 * time.Millisecond)
	var reloads int
loop:
	for {
		select {
		case <-ch:
			reloads++
		case <-deadline:
			break loop
		}
	}

	if reloads != 1 {
		t.Errorf("expected exactly 1 debounced reload, got %d", reloads)
	}
}

// TestWatcherDebounceSeparatesChangesOverTime verifies that changes spaced
// further apart than the debounce window still produce their own reload.
// The debounce is watcher-wide, so this is about separation in time, not
// about which file changed.
func TestWatcherDebounceSeparatesChangesOverTime(t *testing.T) {
	dir := setupWatcherTempDir(t)

	ch := make(chan string, 16)
	watcher, err := NewWatcher(&chanBroadcaster{ch: ch})
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	defer func() { _ = watcher.Close() }()
	watcher.debounce = 30 * time.Millisecond

	if err := watcher.WatchDirectory(dir); err != nil {
		t.Fatalf("WatchDirectory: %v", err)
	}
	watcher.Start()

	if err := appendFile(filepath.Join(dir, "index.html"), "a\n"); err != nil {
		t.Fatalf("write: %v", err)
	}
	// Wait past the debounce so the two files are not coalesced together.
	time.Sleep(60 * time.Millisecond)
	if err := appendFile(filepath.Join(dir, "sub", "note.txt"), "b\n"); err != nil {
		t.Fatalf("write: %v", err)
	}

	deadline := time.After(300 * time.Millisecond)
	var reloads int
loop:
	for {
		select {
		case <-ch:
			reloads++
		case <-deadline:
			break loop
		}
	}

	if reloads != 2 {
		t.Errorf("expected 2 debounced reloads (one per debounce window), got %d", reloads)
	}
}

// TestWatcherDebounceCoalescesMultiFileBurst verifies that a burst touching
// many different files produces a single reload. Every event broadcasts the
// same global "reload", so the per-path timers this replaced fired one
// redundant reload per file -- a build or checkout could flood the browser
// with reload messages and cause reconnect churn.
func TestWatcherDebounceCoalescesMultiFileBurst(t *testing.T) {
	dir := setupWatcherTempDir(t)

	ch := make(chan string, 256)
	watcher, err := NewWatcher(&chanBroadcaster{ch: ch})
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	defer func() { _ = watcher.Close() }()
	watcher.debounce = 50 * time.Millisecond

	if err := watcher.WatchDirectory(dir); err != nil {
		t.Fatalf("WatchDirectory: %v", err)
	}
	watcher.Start()

	// Write many distinct files well inside the debounce window, the way a
	// build step or a git checkout would.
	for i := 0; i < 25; i++ {
		name := filepath.Join(dir, fmt.Sprintf("page%d.html", i))
		if err := os.WriteFile(name, []byte("built\n"), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	deadline := time.After(500 * time.Millisecond)
	var reloads int
loop:
	for {
		select {
		case <-ch:
			reloads++
		case <-deadline:
			break loop
		}
	}

	if reloads != 1 {
		t.Errorf("expected exactly 1 reload for a 25-file burst, got %d", reloads)
	}
}

func appendFile(path, content string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, err = f.WriteString(content)
	return err
}

package server

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestWatcherDebounceCoalescesBursts verifies that a burst of change events
// for the same file results in a single reload broadcast after the debounce
// window, rather than one broadcast per event.
func TestWatcherDebounceCoalescesBursts(t *testing.T) {
	hub := NewSSEHub()
	watcher, err := NewWatcher(hub)
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	defer watcher.Close()
	// Use a short debounce to keep the test fast.
	watcher.debounce = 30 * time.Millisecond

	if err := watcher.WatchDirectory("testdata_watcher"); err != nil {
		t.Fatalf("WatchDirectory: %v", err)
	}
	watcher.Start()

	ch := make(chan string, 16)
	hub.addClient(ch)
	defer hub.removeClient(ch)

	// Write a burst of changes to index.html.
	target := filepath.Join("testdata_watcher", "index.html")
	for i := 0; i < 5; i++ {
		if err := writeAppend(target, "burst\n"); err != nil {
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

// TestWatcherDebounceSeparatesDistinctFiles verifies that changes to two
// different files each produce their own reload.
func TestWatcherDebounceSeparatesDistinctFiles(t *testing.T) {
	hub := NewSSEHub()
	watcher, err := NewWatcher(hub)
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	defer watcher.Close()
	watcher.debounce = 30 * time.Millisecond

	if err := watcher.WatchDirectory("testdata_watcher"); err != nil {
		t.Fatalf("WatchDirectory: %v", err)
	}
	watcher.Start()

	ch := make(chan string, 16)
	hub.addClient(ch)
	defer hub.removeClient(ch)

	if err := writeAppend(filepath.Join("testdata_watcher", "index.html"), "a\n"); err != nil {
		t.Fatalf("write: %v", err)
	}
	// Wait past the debounce so the two files are not coalesced together.
	time.Sleep(60 * time.Millisecond)
	if err := writeAppend(filepath.Join("testdata_watcher", "sub", "note.txt"), "b\n"); err != nil {
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
		t.Errorf("expected 2 debounced reloads (one per file), got %d", reloads)
	}
}

func writeAppend(path, content string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(content)
	return err
}
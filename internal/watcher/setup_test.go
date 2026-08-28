package watcher

import (
	"path/filepath"
	"testing"
)

// TestWatchDirectoryReportsUnwatchableRoot verifies that failing to watch
// the served root is an error rather than a silent success. Add failures
// used to be logged and swallowed, so hitting a watch limit or a
// permission error started a server whose live reload never worked.
func TestWatchDirectoryReportsUnwatchableRoot(t *testing.T) {
	dir := mkdirFiles(t, map[string]string{"index.html": "hello"})

	w, err := NewWatcher(&chanBroadcaster{ch: make(chan string, 1)})
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	// Closing the underlying watcher makes every Add fail, standing in for
	// a watch-limit or permission failure on the root.
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if err := w.WatchDirectory(dir); err == nil {
		t.Error("WatchDirectory on an unwatchable root: want error, got nil")
	}
}

// TestWatchDirectoryReportsMissingRoot verifies a nonexistent served
// directory is reported rather than watched as an empty tree.
func TestWatchDirectoryReportsMissingRoot(t *testing.T) {
	w, err := NewWatcher(&chanBroadcaster{ch: make(chan string, 1)})
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	defer func() { _ = w.Close() }()

	if err := w.WatchDirectory(filepath.Join(t.TempDir(), "does-not-exist")); err == nil {
		t.Error("WatchDirectory on a missing root: want error, got nil")
	}
}

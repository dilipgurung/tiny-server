package server

import (
	"path/filepath"
	"testing"
)

// TestWatcherReadsGitignoreFromServedDir verifies the watcher loads
// .gitignore patterns from the served directory rather than the process
// working directory.
func TestWatcherReadsGitignoreFromServedDir(t *testing.T) {
	hub := NewSSEHub()
	watcher, err := NewWatcher(hub)
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	defer func() { _ = watcher.Close() }()

	if err := watcher.WatchDirectory("testdata_watcher"); err != nil {
		t.Fatalf("WatchDirectory: %v", err)
	}

	want := map[string]bool{
		".git":        true,
		"ignored.log": true,
		"secret.txt":  true,
	}
	got := map[string]bool{}
	for _, p := range watcher.gitignorePatterns {
		got[p] = true
	}
	for k := range want {
		if !got[k] {
			t.Errorf("missing gitignore pattern %q; got %v", k, watcher.gitignorePatterns)
		}
	}
}

// TestWatcherIgnoresGitignoredFiles verifies that files matching the served
// directory's .gitignore are not watched.
func TestWatcherIgnoresGitignoredFiles(t *testing.T) {
	hub := NewSSEHub()
	watcher, err := NewWatcher(hub)
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	defer func() { _ = watcher.Close() }()

	if err := watcher.WatchDirectory("testdata_watcher"); err != nil {
		t.Fatalf("WatchDirectory: %v", err)
	}

	abs, err := filepath.Abs("testdata_watcher")
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}

	// ignored.log and secret.txt match the served dir's .gitignore and
	// must not be watched. index.html must be served normally.
	watched := map[string]bool{}
	for _, p := range watcher.watcher.WatchList() {
		watched[filepath.Base(p)] = true
	}
	if watched["ignored.log"] {
		t.Errorf("ignored.log should not be watched")
	}
	if watched["secret.txt"] {
		t.Errorf("secret.txt should not be watched")
	}
	// The root directory itself should be watched.
	if !watched[filepath.Base(abs)] && !watched["testdata_watcher"] {
		// WatchList returns absolute paths; the dir base may collide with
		// other test dirs, so just verify the root path is present.
		found := false
		for _, p := range watcher.watcher.WatchList() {
			if p == abs {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("served directory %s should be watched; got %v", abs, watcher.watcher.WatchList())
		}
	}
}
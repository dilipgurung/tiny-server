package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestWatchDirectoryWatchesHiddenRoot verifies the served root is watched
// even when its own name would be ignored inside the tree. The hidden-name
// skip used to apply to the walk root, so "tiny-server -d ~/.local/site"
// returned success while watching nothing at all -- live reload was dead
// with no error anywhere.
func TestWatchDirectoryWatchesHiddenRoot(t *testing.T) {
	for _, name := range []string{".site", "dist", "~backup"} {
		t.Run(name, func(t *testing.T) {
			base := t.TempDir()
			root := filepath.Join(base, name)
			if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
				t.Fatalf("MkdirAll: %v", err)
			}
			if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("dist/\n"), 0o644); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}
			if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("hi"), 0o644); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}

			w, err := NewWatcher(&chanBroadcaster{ch: make(chan string, 1)})
			if err != nil {
				t.Fatalf("NewWatcher: %v", err)
			}
			defer func() { _ = w.Close() }()

			if err := w.WatchDirectory(root); err != nil {
				t.Fatalf("WatchDirectory: %v", err)
			}

			watched := map[string]bool{}
			for _, p := range w.watcher.WatchList() {
				watched[p] = true
			}
			if !watched[root] {
				t.Errorf("served root %q not watched; watch list %v", root, w.watcher.WatchList())
			}
			if !watched[filepath.Join(root, "sub")] {
				t.Errorf("subdirectory of %q not watched; watch list %v", root, w.watcher.WatchList())
			}
		})
	}
}

// TestHiddenRootStillReloadsOnChange is the behaviour that matters: editing
// a file under a hidden served root broadcasts a reload.
func TestHiddenRootStillReloadsOnChange(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, ".site")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	target := filepath.Join(root, "index.html")
	if err := os.WriteFile(target, []byte("hi\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	ch := make(chan string, 8)
	w, err := NewWatcher(&chanBroadcaster{ch: ch})
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	defer func() { _ = w.Close() }()
	w.debounce = 30 * time.Millisecond

	if err := w.WatchDirectory(root); err != nil {
		t.Fatalf("WatchDirectory: %v", err)
	}
	w.Start()

	if err := appendFile(target, "change\n"); err != nil {
		t.Fatalf("appendFile: %v", err)
	}

	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("no reload broadcast for a change under a hidden served root")
	}
}

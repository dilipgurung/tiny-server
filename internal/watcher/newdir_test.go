package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestMovedInTreeIsWatched verifies that a populated directory tree moved
// into the served root is watched all the way down. fsnotify only reports
// the top of the tree, and only that top directory used to be added, so
// editing a nested file afterwards never reloaded the page.
func TestMovedInTreeIsWatched(t *testing.T) {
	dir := setupWatcherTempDir(t)

	// Build the tree outside the served root, then move it in whole.
	staging := t.TempDir()
	tree := filepath.Join(staging, "docs")
	nested := filepath.Join(tree, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	target := filepath.Join(nested, "file.html")
	if err := os.WriteFile(target, []byte("<html></html>\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	ch := startWatcher(t, dir, 30*time.Millisecond)

	moved := filepath.Join(dir, "docs")
	if err := os.Rename(tree, moved); err != nil {
		t.Fatalf("Rename: %v", err)
	}

	// Moving the tree in is itself a visible change.
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("no reload broadcast after moving a directory tree into the served root")
	}

	// Give the recursive watch time to settle, then edit a nested file.
	time.Sleep(150 * time.Millisecond)
	drain(ch)

	if err := appendFile(filepath.Join(moved, "a", "b", "file.html"), "edit\n"); err != nil {
		t.Fatalf("appendFile: %v", err)
	}

	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("no reload broadcast after editing a file nested in a moved-in tree")
	}
}

// TestNewNestedDirectoryIsWatched covers directories created in place,
// including one made with MkdirAll where only the outermost level produces
// a Create event in the served root.
func TestNewNestedDirectoryIsWatched(t *testing.T) {
	dir := setupWatcherTempDir(t)
	ch := startWatcher(t, dir, 30*time.Millisecond)

	deep := filepath.Join(dir, "x", "y", "z")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("no reload broadcast after creating a directory")
	}

	time.Sleep(150 * time.Millisecond)
	drain(ch)

	if err := os.WriteFile(filepath.Join(deep, "page.html"), []byte("hi"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("no reload broadcast after creating a file in a newly created nested directory")
	}
}

// TestIgnoredDirectoryTreeIsNotWatched verifies the recursive add still
// honours the ignore rules for entries inside the new tree.
func TestIgnoredDirectoryTreeIsNotWatched(t *testing.T) {
	dir := mkdirFiles(t, map[string]string{
		".gitignore": "dist/\n",
		"index.html": "hello",
	})
	w, err := NewWatcher(&chanBroadcaster{ch: make(chan string, 16)})
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	defer func() { _ = w.Close() }()
	if err := w.WatchDirectory(dir); err != nil {
		t.Fatalf("WatchDirectory: %v", err)
	}

	// A tree containing an ignored subdirectory appears later.
	newTree := filepath.Join(dir, "site")
	if err := os.MkdirAll(filepath.Join(newTree, "dist", "inner"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(newTree, "css"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := w.watchTree(newTree); err != nil {
		t.Fatalf("watchTree: %v", err)
	}

	watched := map[string]bool{}
	for _, p := range w.watcher.WatchList() {
		watched[p] = true
	}
	if !watched[filepath.Join(newTree, "css")] {
		t.Errorf("css should be watched; watch list %v", w.watcher.WatchList())
	}
	if watched[filepath.Join(newTree, "dist")] || watched[filepath.Join(newTree, "dist", "inner")] {
		t.Errorf("gitignored dist should not be watched; watch list %v", w.watcher.WatchList())
	}
}

func drain(ch chan string) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

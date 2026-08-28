package watcher

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// startWatcher returns a started watcher over dir and the channel its
// broadcasts arrive on.
func startWatcher(t *testing.T, dir string, debounce time.Duration) chan string {
	t.Helper()
	ch := make(chan string, 64)
	w, err := NewWatcher(&chanBroadcaster{ch: ch})
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	t.Cleanup(func() { _ = w.Close() })
	w.debounce = debounce
	if err := w.WatchDirectory(dir); err != nil {
		t.Fatalf("WatchDirectory: %v", err)
	}
	w.Start()
	return ch
}

// countReloads drains ch for window and reports how many reloads arrived.
func countReloads(ch chan string, window time.Duration) int {
	deadline := time.After(window)
	n := 0
	for {
		select {
		case <-ch:
			n++
		case <-deadline:
			return n
		}
	}
}

// TestReloadOnFileRemoved verifies deleting a served file reloads the page.
// Remove events used to be filtered out entirely, so the browser kept
// showing content and links for files that no longer existed.
func TestReloadOnFileRemoved(t *testing.T) {
	dir := setupWatcherTempDir(t)
	ch := startWatcher(t, dir, 30*time.Millisecond)

	if err := os.Remove(filepath.Join(dir, "index.html")); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("no reload broadcast after deleting a served file")
	}
}

// TestReloadOnFileRenamed verifies renaming a served file reloads the page.
func TestReloadOnFileRenamed(t *testing.T) {
	dir := setupWatcherTempDir(t)
	ch := startWatcher(t, dir, 30*time.Millisecond)

	from := filepath.Join(dir, "index.html")
	to := filepath.Join(dir, "home.html")
	if err := os.Rename(from, to); err != nil {
		t.Fatalf("Rename: %v", err)
	}

	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("no reload broadcast after renaming a served file")
	}
}

// TestBulkDeleteProducesOneReload verifies the global debounce keeps a
// large delete (removing a build directory's contents) to a single reload.
func TestBulkDeleteProducesOneReload(t *testing.T) {
	dir := t.TempDir()
	var pages []string
	for i := 0; i < 25; i++ {
		name := filepath.Join(dir, fmt.Sprintf("page%d.html", i))
		if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		pages = append(pages, name)
	}

	ch := startWatcher(t, dir, 50*time.Millisecond)
	for _, name := range pages {
		if err := os.Remove(name); err != nil {
			t.Fatalf("Remove: %v", err)
		}
	}

	if got := countReloads(ch, 500*time.Millisecond); got != 1 {
		t.Errorf("expected exactly 1 reload for a bulk delete, got %d", got)
	}
}

// TestChmodDoesNotReload verifies permission-only changes are still ignored.
func TestChmodDoesNotReload(t *testing.T) {
	dir := setupWatcherTempDir(t)
	ch := startWatcher(t, dir, 30*time.Millisecond)

	if err := os.Chmod(filepath.Join(dir, "index.html"), 0o600); err != nil {
		t.Fatalf("Chmod: %v", err)
	}

	if got := countReloads(ch, 300*time.Millisecond); got != 0 {
		t.Errorf("expected no reload for a chmod, got %d", got)
	}
}

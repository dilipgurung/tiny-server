package watcher

import (
	"path/filepath"
	"testing"
)

// TestWatcherReadsGitignoreFromServedDir verifies the watcher loads
// .gitignore patterns from the served directory rather than the process
// working directory.
func TestWatcherReadsGitignoreFromServedDir(t *testing.T) {
	dir := mkdirFiles(t, map[string]string{
		".gitignore":   "ignored.log\nsecret.txt\n",
		"index.html":   "hello",
		"sub/note.txt": "note",
	})
	b := &chanBroadcaster{ch: make(chan string, 1)}
	watcher, err := NewWatcher(b)
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	defer func() { _ = watcher.Close() }()

	if err := watcher.WatchDirectory(dir); err != nil {
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
	dir := mkdirFiles(t, map[string]string{
		".gitignore":   "ignored.log\nsecret.txt\n",
		"index.html":   "hello",
		"sub/note.txt": "note",
	})
	b := &chanBroadcaster{ch: make(chan string, 1)}
	watcher, err := NewWatcher(b)
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	defer func() { _ = watcher.Close() }()

	if err := watcher.WatchDirectory(dir); err != nil {
		t.Fatalf("WatchDirectory: %v", err)
	}

	abs, err := filepath.Abs(dir)
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
	if !watched[filepath.Base(abs)] && !watched[filepath.Base(dir)] {
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

// TestWatcherIgnoresDirectoryPatterns verifies that the trailing-slash
// directory patterns that dominate real .gitignore files actually take
// effect. filepath.Match("dist/", "dist") is false, so keeping the slash
// silently watched every build output directory -- turning a single build
// into a storm of reloads.
func TestWatcherIgnoresDirectoryPatterns(t *testing.T) {
	dir := mkdirFiles(t, map[string]string{
		".gitignore":            "dist/\nnode_modules/\n/vendor\n",
		"index.html":            "hello",
		"dist/bundle.js":        "built",
		"node_modules/pkg/i.js": "dep",
		"vendor/lib/x.go":       "vendored",
		"src/app.js":            "source",
	})
	b := &chanBroadcaster{ch: make(chan string, 1)}
	watcher, err := NewWatcher(b)
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	defer func() { _ = watcher.Close() }()

	if err := watcher.WatchDirectory(dir); err != nil {
		t.Fatalf("WatchDirectory: %v", err)
	}

	watched := map[string]bool{}
	for _, p := range watcher.watcher.WatchList() {
		watched[filepath.Base(p)] = true
	}
	for _, name := range []string{"dist", "node_modules", "vendor"} {
		if watched[name] {
			t.Errorf("%q matches a .gitignore directory pattern but is watched; watch list %v",
				name, watcher.watcher.WatchList())
		}
	}
	if !watched["src"] {
		t.Errorf("src is not ignored and should be watched; watch list %v", watcher.watcher.WatchList())
	}
}

// TestGitignorePatternsAreMatchable verifies loaded patterns can actually
// match a basename via filepath.Match.
func TestGitignorePatternsAreMatchable(t *testing.T) {
	dir := mkdirFiles(t, map[string]string{
		".gitignore": "dist/\n/vendor\n  tmp/  \n#comment\n\n*.log\n",
		"index.html": "hello",
	})
	watcher, err := NewWatcher(&chanBroadcaster{ch: make(chan string, 1)})
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	defer func() { _ = watcher.Close() }()
	watcher.loadGitignore(dir)

	for pattern, name := range map[string]string{"dist": "dist", "vendor": "vendor", "tmp": "tmp", "*.log": "debug.log"} {
		found := false
		for _, p := range watcher.gitignorePatterns {
			if p == pattern {
				found = true
			}
		}
		if !found {
			t.Errorf("pattern %q not loaded; got %v", pattern, watcher.gitignorePatterns)
			continue
		}
		if ok, err := filepath.Match(pattern, name); err != nil || !ok {
			t.Errorf("filepath.Match(%q, %q) = %v, %v; want true", pattern, name, ok, err)
		}
	}
	for _, p := range watcher.gitignorePatterns {
		if p == "#comment" || p == "" {
			t.Errorf("comment or blank line loaded as a pattern: %v", watcher.gitignorePatterns)
		}
	}
}

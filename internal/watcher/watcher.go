package watcher

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Broadcaster is the dependency the watcher needs to signal file changes.
// It is satisfied by the live-reload SSE hub; tests can substitute a fake.
type Broadcaster interface {
	Broadcast(message string)
}

type Watcher struct {
	watcher           *fsnotify.Watcher
	broadcaster       Broadcaster
	gitignorePatterns []string
	debounce          time.Duration
	mu                sync.Mutex
	closed            bool
	reloadTimers      map[string]*time.Timer
}

func NewWatcher(b Broadcaster) (*Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		watcher:           watcher,
		broadcaster:       b,
		gitignorePatterns: []string{".git"},
		debounce:          200 * time.Millisecond,
		reloadTimers:      make(map[string]*time.Timer),
	}

	return w, nil
}

// loadGitignore reads .gitignore patterns from the served directory (root)
// rather than the process working directory. Only simple basename patterns
// are supported: no paths, no negation, no **. See README for details.
func (w *Watcher) loadGitignore(root string) {
	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		pattern := strings.TrimSpace(line)
		if pattern == "" || strings.HasPrefix(pattern, "#") {
			continue
		}
		// Patterns are matched against a path's basename, so surrounding
		// slashes must go: filepath.Match("dist/", "dist") is false, which
		// silently disabled the most common .gitignore entries there are
		// ("dist/", "node_modules/", "/vendor").
		pattern = strings.Trim(pattern, "/")
		if pattern == "" {
			continue
		}
		w.gitignorePatterns = append(w.gitignorePatterns, pattern)
	}
}

func (w *Watcher) WatchDirectory(root string) error {
	w.loadGitignore(root)
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip symlinks to avoid duplicate events
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}

		// The served root is always watched. The ignore rules below apply to
		// what is found *inside* it: applying them to the root itself made
		// "tiny-server -d ~/.local/site", or serving a directory named
		// "dist", walk nothing and start with live reload silently dead.
		if path != root {
			if w.ignored(filepath.Base(path)) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		if info.IsDir() {
			if err := w.watcher.Add(path); err != nil {
				log.Printf("Failed to watch %q: %v", path, err)
			}
		}
		return nil
	})
}

// ignored reports whether a path's basename should be excluded from
// watching: hidden and editor-backup names, or a .gitignore pattern.
func (w *Watcher) ignored(base string) bool {
	if strings.HasPrefix(base, "~") || strings.HasPrefix(base, ".") {
		return true
	}
	for _, pattern := range w.gitignorePatterns {
		matched, err := filepath.Match(pattern, base)
		if err != nil {
			log.Printf("Invalid gitignore pattern %q: %v", pattern, err)
			continue
		}
		if matched {
			return true
		}
	}
	return false
}

func (w *Watcher) Start() {
	w.StartCtx(context.Background())
}

// StartCtx launches the event loop goroutine. It returns when ctx is
// cancelled or the underlying fsnotify watcher is closed.
func (w *Watcher) StartCtx(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-w.watcher.Events:
				if !ok {
					return
				}
				// Skip hidden, editor-backup and gitignored names.
				if w.ignored(filepath.Base(event.Name)) {
					continue
				}

				// Handle new directories
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					if event.Op&fsnotify.Create != 0 {
						if err := w.watcher.Add(event.Name); err != nil {
							log.Printf("Failed to watch new directory %q: %v", event.Name, err)
						}
						continue
					}
				}

				// Handle all file operations except Remove and Chmod.
				// Debounce: coalesce bursts of events for the same file so we
				// only broadcast a single reload per burst.
				if event.Op&fsnotify.Remove == 0 && event.Op&fsnotify.Chmod == 0 {
					w.scheduleReload(event.Name)
				}
			case err, ok := <-w.watcher.Errors:
				if !ok {
					return
				}
				log.Printf("Watcher error: %v", err)
			}
		}
	}()
}

// scheduleReload coalesces bursts of file-change events for the same
// path into a single broadcast after the debounce window elapses.
func (w *Watcher) scheduleReload(path string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	// The event loop can still be handling an event that was dequeued
	// before Close ran. Without this guard it would write to the nil map
	// Close leaves behind and panic during shutdown.
	if w.closed {
		return
	}
	if t, ok := w.reloadTimers[path]; ok {
		t.Stop()
	}
	w.reloadTimers[path] = time.AfterFunc(w.debounce, func() {
		log.Println("File changed:", path)
		w.broadcaster.Broadcast("reload")
		w.mu.Lock()
		delete(w.reloadTimers, path)
		w.mu.Unlock()
	})
}

func (w *Watcher) Close() error {
	w.mu.Lock()
	w.closed = true
	for _, t := range w.reloadTimers {
		t.Stop()
	}
	w.reloadTimers = nil
	w.mu.Unlock()
	return w.watcher.Close()
}

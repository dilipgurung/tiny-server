package server

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

type Watcher struct {
	watcher           *fsnotify.Watcher
	hub               *SSEHub
	gitignorePatterns []string
	debounce          time.Duration
	mu                sync.Mutex
	reloadTimers      map[string]*time.Timer
}

func NewWatcher(hub *SSEHub) (*Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		watcher:           watcher,
		hub:               hub,
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
	if data, err := os.ReadFile(filepath.Join(root, ".gitignore")); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				w.gitignorePatterns = append(w.gitignorePatterns, line)
			}
		}
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

		// Skip directories starting with ~ or .
		base := filepath.Base(path)
		if strings.HasPrefix(base, "~") || (base != "." && strings.HasPrefix(base, ".")) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip .git and gitignore patterns
		for _, pattern := range w.gitignorePatterns {
			matched, err := filepath.Match(pattern, base)
			if err != nil {
				log.Printf("Invalid gitignore pattern %q: %v", pattern, err)
				continue
			}
			if matched {
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
				// Skip directories starting with ~ or .
				base := filepath.Base(event.Name)
				if strings.HasPrefix(base, "~") || strings.HasPrefix(base, ".") {
					continue
				}

				// Check if file matches gitignore patterns
				skip := false
				for _, pattern := range w.gitignorePatterns {
					matched, err := filepath.Match(pattern, base)
					if err != nil {
						log.Printf("Invalid gitignore pattern %q: %v", pattern, err)
						continue
					}
					if matched {
						skip = true
						break
					}
				}
				if skip {
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
	if t, ok := w.reloadTimers[path]; ok {
		t.Stop()
	}
	w.reloadTimers[path] = time.AfterFunc(w.debounce, func() {
		log.Println("File changed:", path)
		w.hub.Broadcast("reload")
		w.mu.Lock()
		delete(w.reloadTimers, path)
		w.mu.Unlock()
	})
}

func (w *Watcher) Close() error {
	w.mu.Lock()
	for _, t := range w.reloadTimers {
		t.Stop()
	}
	w.reloadTimers = nil
	w.mu.Unlock()
	return w.watcher.Close()
}

package server

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

type Watcher struct {
	watcher           *fsnotify.Watcher
	hub               *WebSocketHub
	gitignorePatterns []string
}

func NewWatcher(hub *WebSocketHub) (*Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		watcher:           watcher,
		hub:               hub,
		gitignorePatterns: []string{".git"},
	}

	// Read .gitignore patterns
	if data, err := os.ReadFile(".gitignore"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				w.gitignorePatterns = append(w.gitignorePatterns, line)
			}
		}
	}

	return w, nil
}

func (w *Watcher) WatchDirectory(root string) error {
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
	go func() {
		for {
			select {
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

				// Handle all file operations except Remove and Chmod
				if event.Op&fsnotify.Remove == 0 && event.Op&fsnotify.Chmod == 0 {
					log.Println("File changed:", event.Name)
					w.hub.Broadcast("reload")
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

func (w *Watcher) Close() error {
	return w.watcher.Close()
}

package server

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"sync"
)

type SSEHub struct {
	clients map[chan string]struct{}
	mu      sync.Mutex
}

func NewSSEHub() *SSEHub {
	return &SSEHub{
		clients: make(map[chan string]struct{}),
	}
}

func (h *SSEHub) addClient(ch chan string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[ch] = struct{}{}
}

func (h *SSEHub) removeClient(ch chan string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, ch)
}

// Broadcast sends a message to all connected SSE clients
func (h *SSEHub) Broadcast(message string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for ch := range h.clients {
		select {
		case ch <- message:
		default:
			// client too slow; skip rather than block
		}
	}
}

func SSEHandler(hub *SSEHub, shutdownCtx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		// Send an initial comment to establish the connection
		if _, err := fmt.Fprint(w, ": connected\n\n"); err != nil {
			return
		}
		flusher.Flush()

		ch := make(chan string, 1)
		hub.addClient(ch)
		defer hub.removeClient(ch)

		for {
			select {
			case msg := <-ch:
				if _, err := fmt.Fprintf(w, "data: %s\n\n", msg); err != nil {
					return
				}
				flusher.Flush()
			case <-r.Context().Done():
				return
			case <-shutdownCtx.Done():
				// Server is shutting down: exit promptly so http.Server.Shutdown
				// doesn't wait for this long-lived SSE connection until its
				// deadline (which produced "Forced shutdown: context deadline
				// exceeded" whenever a browser tab was open).
				return
			}
		}
	}
}

type Server struct {
	httpServer *http.Server
	watcher    *Watcher
	cancel     context.CancelFunc
}

func NewServer(port, dir string) (*Server, error) {
	absPath, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("error getting absolute path: %w", err)
	}

	hub := NewSSEHub()
	mux := http.NewServeMux()

	shutdownCtx, cancel := context.WithCancel(context.Background())

	fs := http.FileServer(http.Dir(absPath))
	wrappedHandler := logRequest(blockDotfiles(liveReload(fs)))
	mux.Handle("/", wrappedHandler)

	mux.HandleFunc("/.live-reload", SSEHandler(hub, shutdownCtx))

	watcher, err := NewWatcher(hub)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("error creating watcher: %w", err)
	}

	if err := watcher.WatchDirectory(absPath); err != nil {
		cancel()
		return nil, fmt.Errorf("error watching directory: %w", err)
	}
	watcher.Start()

	return &Server{
		httpServer: &http.Server{
			Addr:    ":" + port,
			Handler: mux,
		},
		watcher: watcher,
		cancel:  cancel,
	}, nil
}

func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	// Cancel the SSE shutdown context first so live-reload EventSource
	// goroutines exit and release their connections before we wait on
	// http.Server.Shutdown. Otherwise Shutdown blocks until ctx expires
	// ("context deadline exceeded") whenever a browser tab is open.
	s.cancel()
	_ = s.watcher.Close()
	return s.httpServer.Shutdown(ctx)
}



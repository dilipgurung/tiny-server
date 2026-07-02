package server

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/dilipgurung/tiny-server/internal/livereload"
	"github.com/dilipgurung/tiny-server/internal/watcher"
)

type Server struct {
	httpServer *http.Server
	watcher    *watcher.Watcher
	cancel     context.CancelFunc
}

func NewServer(port, dir string) (*Server, error) {
	absPath, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("error getting absolute path: %w", err)
	}

	hub := livereload.NewHub()
	mux := http.NewServeMux()

	shutdownCtx, cancel := context.WithCancel(context.Background())

	fs := http.FileServer(http.Dir(absPath))
	wrappedHandler := logRequest(blockDotfiles(livereload.LiveReload(fs)))
	mux.Handle("/", wrappedHandler)

	mux.HandleFunc("/.live-reload", livereload.SSEHandler(hub, shutdownCtx))

	w, err := watcher.NewWatcher(hub)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("error creating watcher: %w", err)
	}

	if err := w.WatchDirectory(absPath); err != nil {
		cancel()
		return nil, fmt.Errorf("error watching directory: %w", err)
	}
	w.Start()

	return &Server{
		httpServer: &http.Server{
			Addr:    ":" + port,
			Handler: mux,
		},
		watcher: w,
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

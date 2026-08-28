package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/dilipgurung/tiny-server/internal/livereload"
	"github.com/dilipgurung/tiny-server/internal/watcher"
)

type Server struct {
	httpServer *http.Server
	watcher    *watcher.Watcher
	root       *os.Root
	cancel     context.CancelFunc
}

func NewServer(port, dir string) (*Server, error) {
	absPath, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("error getting absolute path: %w", err)
	}

	// Serve through an *os.Root rather than http.Dir. http.Dir follows
	// symlinks out of the served directory, so a link like
	// "config -> ../../.env" would expose arbitrary readable files to every
	// client on the network. os.Root resolves every path within the root.
	root, err := openServedRoot(absPath)
	if err != nil {
		return nil, err
	}

	hub := livereload.NewHub()
	mux := http.NewServeMux()

	shutdownCtx, cancel := context.WithCancel(context.Background())

	fs := http.FileServerFS(containedFS{fsys: root.FS()})
	wrappedHandler := logRequest(blockDotfiles(livereload.LiveReload(fs)))
	mux.Handle("/", wrappedHandler)

	mux.HandleFunc("/.live-reload", livereload.SSEHandler(hub, shutdownCtx))

	w, err := watcher.NewWatcher(hub)
	if err != nil {
		cancel()
		_ = root.Close()
		return nil, fmt.Errorf("error creating watcher: %w", err)
	}

	if err := w.WatchDirectory(absPath); err != nil {
		cancel()
		_ = root.Close()
		return nil, fmt.Errorf("error watching directory: %w", err)
	}
	w.Start()

	return &Server{
		httpServer: &http.Server{
			Addr:    ":" + port,
			Handler: mux,
			// Bound how long a client may hold a connection while sending.
			// The server listens on every interface, so without these a
			// single peer can pin sockets and goroutines indefinitely by
			// dribbling out a request. This only serves GET/HEAD, so the
			// whole request should arrive well inside ReadTimeout.
			//
			// WriteTimeout is deliberately left unset: live-reload responses
			// are long-lived SSE streams and a write deadline would cut them
			// off. The read deadlines above do not affect them -- they still
			// stream, and client disconnects are still detected.
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       15 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
		watcher: w,
		root:    root,
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
	_ = s.root.Close()
	return s.httpServer.Shutdown(ctx)
}

package livereload

import (
	"context"
	"fmt"
	"net/http"
)

func SSEHandler(hub *Hub, shutdownCtx context.Context) http.HandlerFunc {
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

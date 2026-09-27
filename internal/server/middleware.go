package server

import (
	"log"
	"net/http"
	"strings"
	"time"
)

// blockDotfiles rejects requests whose path contains a segment starting
// with "." (e.g. .env, .git, .gitignore). This prevents the static file
// server from leaking dotfiles that live in the served directory.
func blockDotfiles(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isDotfilePath(r.URL.Path) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// isDotfilePath reports whether any segment of the cleaned path begins with
// ".". The root path "/" is allowed.
func isDotfilePath(path string) bool {
	// A segment starts with "." exactly when a "." is at the very start of
	// the path or directly follows a "/". Scanning for that avoids splitting
	// (and allocating) the path on every request.
	if strings.HasPrefix(path, ".") {
		return true
	}
	return strings.Contains(path, "/.")
}

func logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rec := &StatusRecorder{ResponseWriter: w, StatusCode: 200}
		next.ServeHTTP(rec, r)
		duration := time.Since(start)

		// Log the escaped path: r.URL.Path is percent-decoded, so a request
		// for "/%0Afake" would otherwise let any client on the network write
		// forged lines into the log.
		log.Printf("%-6s %3d %12s %-40s",
			r.Method,
			rec.StatusCode,
			duration,
			r.URL.EscapedPath(),
		)
	})
}

// StatusRecorder wraps http.ResponseWriter to capture the status code so
// the request logger can report it.
type StatusRecorder struct {
	http.ResponseWriter
	StatusCode int
}

func (rec *StatusRecorder) WriteHeader(code int) {
	rec.StatusCode = code
	rec.ResponseWriter.WriteHeader(code)
}

// Unwrap exposes the underlying ResponseWriter to http.ResponseController.
func (rec *StatusRecorder) Unwrap() http.ResponseWriter {
	return rec.ResponseWriter
}

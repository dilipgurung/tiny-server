package server

import "net/http"

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

package server

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
)

// ResponseRecorder wraps ResponseWriter to capture status and content type
type ResponseRecorder struct {
	http.ResponseWriter
	buf        *bytes.Buffer
	statusCode int
	header     http.Header
}

func (w *ResponseRecorder) Header() http.Header {
	return w.header
}

func (w *ResponseRecorder) Write(b []byte) (int, error) {
	return w.buf.Write(b)
}

func (w *ResponseRecorder) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func NewResponseRecorder(w http.ResponseWriter) *ResponseRecorder {
	return &ResponseRecorder{
		ResponseWriter: w,
		buf:            &bytes.Buffer{},
		statusCode:     http.StatusOK,
		header:         make(http.Header),
	}
}

func WriteResponse(w http.ResponseWriter, recorder *ResponseRecorder, body []byte) {
	// Copy original headers
	for k, v := range recorder.Header() {
		w.Header()[k] = v
	}

	// Set proper content type if not already set
	if w.Header().Get("Content-Type") == "" {
		contentType := http.DetectContentType(body)
		w.Header().Set("Content-Type", contentType)
	}

	// Update content length
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))

	// Write the body (status code was already written by the recorder)
	if _, err := w.Write(body); err != nil {
		log.Printf("Error writing response: %v", err)
	}
}

// StatusRecorder wraps http.ResponseWriter to capture the status code
type StatusRecorder struct {
	http.ResponseWriter
	StatusCode int
}

func (rec *StatusRecorder) WriteHeader(code int) {
	rec.StatusCode = code
	rec.ResponseWriter.WriteHeader(code)
}

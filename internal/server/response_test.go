package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestResponseRecorder(t *testing.T) {
	rr := httptest.NewRecorder()
	recorder := NewResponseRecorder(rr)

	// Test Header()
	h := recorder.Header()
	h.Set("Test", "Value")

	// Test Write()
	testBody := []byte("test body")
	_, err := recorder.Write(testBody)
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}

	// Test WriteHeader()
	recorder.WriteHeader(http.StatusNotFound)

	if recorder.statusCode != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, recorder.statusCode)
	}

	if !bytes.Equal(recorder.buf.Bytes(), testBody) {
		t.Errorf("Expected body %q, got %q", testBody, recorder.buf.Bytes())
	}
}

// TestResponseRecorderDoesNotForwardWriteHeader verifies the recorder no
// longer forwards WriteHeader to the underlying writer. The middleware is
// responsible for writing the final status via WriteResponse.
func TestResponseRecorderDoesNotForwardWriteHeader(t *testing.T) {
	underlying := httptest.NewRecorder()
	recorder := NewResponseRecorder(underlying)

	recorder.WriteHeader(http.StatusNotFound)

	if recorder.statusCode != http.StatusNotFound {
		t.Errorf("recorder status = %d, want %d", recorder.statusCode, http.StatusNotFound)
	}
	if underlying.Code != http.StatusOK {
		t.Errorf("underlying writer status = %d, want %d (should not be forwarded)",
			underlying.Code, http.StatusOK)
	}
}

// TestWriteResponseHeaderAndBody verifies WriteResponse copies headers,
// writes the recorded status code, then writes the body in the correct
// order so nothing is lost.
func TestWriteResponseHeaderAndBody(t *testing.T) {
	w := httptest.NewRecorder()
	rec := NewResponseRecorder(w)
	rec.Header().Set("Content-Type", "text/html; charset=utf-8")
	rec.Header().Set("X-Custom", "custom-value")
	rec.WriteHeader(http.StatusTeapot)
	_, _ = rec.Write([]byte("ignored-buffer"))

	body := []byte("real-body")
	WriteResponse(w, rec, body)

	if w.Code != http.StatusTeapot {
		t.Errorf("status = %d, want %d", w.Code, http.StatusTeapot)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q, want %q", ct, "text/html; charset=utf-8")
	}
	if x := w.Header().Get("X-Custom"); x != "custom-value" {
		t.Errorf("X-Custom = %q, want %q", x, "custom-value")
	}
	if cl := w.Header().Get("Content-Length"); cl != strconv.Itoa(len(body)) {
		t.Errorf("Content-Length = %q, want %d", cl, len(body))
	}
	if w.Body.String() != string(body) {
		t.Errorf("body = %q, want %q", w.Body.String(), body)
	}
}

// TestFullStackHTMLResponse runs a real .html file through the full
// middleware chain (blockDotfiles -> liveReload -> file server) and asserts
// the response has the correct status, Content-Type, Content-Length, and
// injected live-reload script.
func TestFullStackHTMLResponse(t *testing.T) {
	srv, err := NewServer("0", "testdata_dotfiles")
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/index.html")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html prefix", ct)
	}
	buf := make([]byte, 0, 1024)
	tmp := make([]byte, 1024)
	for {
		n, err := resp.Body.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			break
		}
	}
	if !strings.Contains(string(buf), "EventSource") {
		t.Errorf("response body missing injected live-reload script")
	}
	if cl := resp.Header.Get("Content-Length"); cl != strconv.Itoa(len(buf)) {
		t.Errorf("Content-Length = %q, want %d", cl, len(buf))
	}
}

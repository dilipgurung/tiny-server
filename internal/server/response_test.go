package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
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

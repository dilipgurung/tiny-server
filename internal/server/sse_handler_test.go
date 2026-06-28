package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestSSEHandlerRejectsNonGet verifies the SSE endpoint only accepts GET.
func TestSSEHandlerRejectsNonGet(t *testing.T) {
	hub := NewSSEHub()
	handler := SSEHandler(hub)

	req := httptest.NewRequest(http.MethodPost, "/.live-reload", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST status = %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

// TestSSEHandlerStreamsBroadcast verifies that a broadcast message is
// delivered to a connected SSE client as a "data: ..." frame.
func TestSSEHandlerStreamsBroadcast(t *testing.T) {
	hub := NewSSEHub()
	handler := SSEHandler(hub)

	ts := httptest.NewServer(handler)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Give the handler a moment to register the client.
	time.Sleep(50 * time.Millisecond)
	hub.Broadcast("reload")

	// Read until we see the data frame or the context deadline.
	deadline := time.Now().Add(2 * time.Second)
	var got strings.Builder
	buf := make([]byte, 1024)
	for time.Now().Before(deadline) {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			got.Write(buf[:n])
			if strings.Contains(got.String(), "data: reload") {
				break
			}
		}
		if err != nil && err != io.EOF {
			t.Fatalf("Read: %v", err)
		}
	}
	if !strings.Contains(got.String(), "data: reload") {
		t.Errorf("expected SSE frame 'data: reload', got %q", got.String())
	}
}

// TestSSEHandlerSetsHeaders verifies the SSE response headers.
func TestSSEHandlerSetsHeaders(t *testing.T) {
	hub := NewSSEHub()
	handler := SSEHandler(hub)

	ts := httptest.NewServer(handler)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", cc)
	}
}
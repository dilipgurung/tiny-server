package server

import (
	"bytes"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

// countingWriter is a minimal http.ResponseWriter that counts bytes written
// without allocating copies, so benchmarks measure only the middleware.
type countingWriter struct {
	header http.Header
	n      int64
	status int
}

func (c *countingWriter) Header() http.Header {
	if c.header == nil {
		c.header = make(http.Header)
	}
	return c.header
}
func (c *countingWriter) Write(b []byte) (int, error) {
	c.n += int64(len(b))
	return len(b), nil
}
func (c *countingWriter) WriteHeader(code int) { c.status = code }

// recordingWriter captures each Write as a separate chunk so tests can
// assert on streaming behavior (bytes arriving before the body is complete).
type recordingWriter struct {
	header http.Header
	status int
	chunks [][]byte
}

func (r *recordingWriter) Header() http.Header {
	if r.header == nil {
		r.header = make(http.Header)
	}
	return r.header
}
func (r *recordingWriter) Write(b []byte) (int, error) {
	r.chunks = append(r.chunks, append([]byte(nil), b...))
	return len(b), nil
}
func (r *recordingWriter) WriteHeader(code int) { r.status = code }
func (r *recordingWriter) Body() string {
	var sb strings.Builder
	for _, c := range r.chunks {
		sb.Write(c)
	}
	return sb.String()
}

// TestStreamingBufferIsBounded verifies that serving a large HTML body in
// chunks never holds more than injectionMarkerLen-1 bytes, and that the full
// body plus the script is delivered.
func TestStreamingBufferIsBounded(t *testing.T) {
	inner := strings.Repeat("x", 8<<20) // 8 MB
	body := []byte("<html><body>" + inner + "</body></html>")

	rw := &recordingWriter{}
	iw := &injectWriter{ResponseWriter: rw, status: http.StatusOK}
	iw.Header().Set("Content-Type", "text/html; charset=utf-8")
	iw.Header().Set("Content-Length", strconv.Itoa(len(body)))

	chunk := make([]byte, 32<<10) // 32 KB, like io.Copy
	for off := 0; off < len(body); off += len(chunk) {
		end := off + len(chunk)
		if end > len(body) {
			end = len(body)
		}
		if _, err := iw.Write(body[off:end]); err != nil {
			t.Fatalf("Write: %v", err)
		}
		if iw.pendingLen > injectionMarkerLen-1 {
			t.Errorf("held back %d bytes, exceeds %d", iw.pendingLen, injectionMarkerLen-1)
		}
	}
	iw.close()

	got := rw.Body()
	if !strings.Contains(got, "EventSource") {
		t.Errorf("large HTML should still get the live-reload script injected")
	}
	scriptBeforeBodyClose(t, got)
	if len(got) != len(body)+len(liveReloadScript) {
		t.Errorf("body length = %d, want %d", len(got), len(body)+len(liveReloadScript))
	}
}

// TestStreamingBytesArriveBeforeBodyComplete verifies that bytes reach the
// underlying writer before the full body has been produced (i.e. the
// response is streamed, not buffered until the handler returns).
func TestStreamingBytesArriveBeforeBodyComplete(t *testing.T) {
	rw := &recordingWriter{}
	iw := &injectWriter{ResponseWriter: rw, status: http.StatusOK}
	iw.Header().Set("Content-Type", "text/html; charset=utf-8")

	// A first chunk with no </body> must flush through immediately (modulo
	// the held-back marker tail).
	first := bytes.Repeat([]byte("a"), 4096)
	if _, err := iw.Write(first); err != nil {
		t.Fatalf("Write first: %v", err)
	}
	if len(rw.chunks) == 0 {
		t.Fatal("expected bytes to reach the client after the first chunk, got none")
	}
	if iw.injected {
		t.Error("should not have injected before </body>")
	}

	if _, err := iw.Write([]byte("second</body></html>")); err != nil {
		t.Fatalf("Write second: %v", err)
	}
	iw.close()

	if !iw.injected {
		t.Error("expected injection after </body> in second chunk")
	}
	got := rw.Body()
	if !strings.Contains(got, "EventSource") {
		t.Errorf("missing script; got %q", got)
	}
}

// BenchmarkLiveReloadStreaming measures allocations while streaming an 8 MB
// HTML body through the middleware. Memory is O(marker), so allocations must
// be tiny and independent of the body size.
func BenchmarkLiveReloadStreaming(b *testing.B) {
	body := make([]byte, 8<<20) // 8 MB
	for i := range body {
		body[i] = 'x'
	}
	body = append([]byte("<html><body>"), body...)
	body = append(body, []byte("</body></html>")...)
	chunk := make([]byte, 32<<10) // 32 KB, like io.Copy

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rw := &countingWriter{}
		iw := &injectWriter{ResponseWriter: rw, status: http.StatusOK}
		iw.Header().Set("Content-Type", "text/html; charset=utf-8")
		iw.Header().Set("Content-Length", strconv.Itoa(len(body)))
		for off := 0; off < len(body); off += len(chunk) {
			end := off + len(chunk)
			if end > len(body) {
				end = len(body)
			}
			_, _ = iw.Write(body[off:end])
		}
		iw.close()
		if rw.n != int64(len(body)+len(liveReloadScript)) {
			b.Fatalf("delivered %d bytes, want %d", rw.n, len(body)+len(liveReloadScript))
		}
	}
}

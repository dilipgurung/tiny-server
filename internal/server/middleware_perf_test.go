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

// TestInjectWriterBufferIsBounded verifies that serving a large HTML body in
// chunks never grows the internal buffer beyond the cap, and that the full
// body is delivered verbatim (no injection) to the client.
func TestInjectWriterBufferIsBounded(t *testing.T) {
	const cap = 1 << 10 // 1 KB
	inner := strings.Repeat("x", cap*8)
	body := []byte("<html><body>" + inner + "</body></html>")

	rw := &recordingWriter{}
	iw := &injectWriter{ResponseWriter: rw, status: http.StatusOK, cap: cap, mode: modeBuffer}
	iw.Header().Set("Content-Type", "text/html; charset=utf-8")

	chunk := make([]byte, 256)
	for off := 0; off < len(body); off += len(chunk) {
		end := off + len(chunk)
		if end > len(body) {
			end = len(body)
		}
		if _, err := iw.Write(body[off:end]); err != nil {
			t.Fatalf("Write: %v", err)
		}
		if iw.buf.Len() > cap {
			t.Errorf("buffer grew to %d bytes, exceeding cap %d", iw.buf.Len(), cap)
		}
	}
	iw.close()

	got := rw.Body()
	if got != string(body) {
		t.Errorf("body not preserved: got len %d, want %d", len(got), len(body))
	}
	if strings.Contains(got, "EventSource") {
		t.Errorf("oversized body should not have the live-reload script injected")
	}
}

// TestInjectWriterStreamsBeforeBodyComplete verifies that bytes reach the
// underlying writer before the full body has been produced (i.e. the
// response is streamed, not buffered until the handler returns).
func TestInjectWriterStreamsBeforeBodyComplete(t *testing.T) {
	const cap = 1 << 10 // 1 KB
	rw := &recordingWriter{}
	iw := &injectWriter{ResponseWriter: rw, status: http.StatusOK, cap: cap, mode: modeBuffer}
	iw.Header().Set("Content-Type", "text/html; charset=utf-8")

	// First chunk alone exceeds the cap, so it must flush through immediately.
	first := bytes.Repeat([]byte("a"), cap*2)
	if _, err := iw.Write(first); err != nil {
		t.Fatalf("Write first: %v", err)
	}
	if len(rw.chunks) == 0 {
		t.Fatal("expected bytes to reach the client after the first chunk, got none")
	}
	if iw.mode != modePassthrough {
		t.Errorf("expected modePassthrough after exceeding cap, got %v", iw.mode)
	}

	// Second chunk should also stream straight through.
	second := []byte("second")
	if _, err := iw.Write(second); err != nil {
		t.Fatalf("Write second: %v", err)
	}
	iw.close()

	got := rw.Body()
	want := string(first) + string(second)
	if got != want {
		t.Errorf("streamed body = %q, want %q", got, want)
	}
}

// TestInjectWriterInjectsSmallBody verifies a body that fits within the cap
// still gets the live-reload script injected.
func TestInjectWriterInjectsSmallBody(t *testing.T) {
	const cap = 1 << 10
	body := []byte("<html><body>hi</body></html>")
	rw := &recordingWriter{}
	iw := &injectWriter{ResponseWriter: rw, status: http.StatusOK, cap: cap, mode: modeBuffer}
	iw.Header().Set("Content-Type", "text/html; charset=utf-8")
	iw.Header().Set("Content-Length", strconv.Itoa(len(body)))

	if _, err := iw.Write(body); err != nil {
		t.Fatalf("Write: %v", err)
	}
	iw.close()

	got := rw.Body()
	if !strings.Contains(got, "EventSource") {
		t.Errorf("small HTML should have the script injected; got %q", got)
	}
	// Content-Length must reflect the injected body size.
	if cl := iw.Header().Get("Content-Length"); cl != strconv.Itoa(len(body)+len(liveReloadScript)) {
		t.Errorf("Content-Length = %q, want %d", cl, len(body)+len(liveReloadScript))
	}
}

// TestInjectWriterNonHTMLPassthrough verifies non-HTML responses stream
// through untouched from the first write.
func TestInjectWriterNonHTMLPassthrough(t *testing.T) {
	const cap = 1 << 10
	body := []byte("body { color: red; }")
	rw := &recordingWriter{}
	iw := &injectWriter{ResponseWriter: rw, status: http.StatusOK, cap: cap, mode: modeBuffer}
	iw.Header().Set("Content-Type", "text/css")

	if _, err := iw.Write(body); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if iw.mode != modePassthrough {
		t.Errorf("non-HTML should be passthrough, got %v", iw.mode)
	}
	iw.close()

	if got := rw.Body(); got != string(body) {
		t.Errorf("non-HTML body changed: got %q, want %q", got, body)
	}
}

// BenchmarkLiveReloadStreaming measures allocations while streaming an 8 MB
// HTML body through the middleware. The buffer is capped, so allocations
// must be bounded (~cap) rather than proportional to the body size.
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
		iw := &injectWriter{ResponseWriter: rw, status: http.StatusOK, cap: injectionBufferCap, mode: modeBuffer}
		iw.Header().Set("Content-Type", "text/html; charset=utf-8")
		for off := 0; off < len(body); off += len(chunk) {
			end := off + len(chunk)
			if end > len(body) {
				end = len(body)
			}
			_, _ = iw.Write(body[off:end])
		}
		iw.close()
		if rw.n != int64(len(body)) {
			b.Fatalf("delivered %d bytes, want %d", rw.n, len(body))
		}
	}
}

package livereload

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// newInjectWriter creates an injectWriter wrapping a recorder with HTML
// content type and an accurate Content-Length, mirroring http.FileServer.
func newInjectWriter(body []byte) (*injectWriter, *httptest.ResponseRecorder) {
	rw := httptest.NewRecorder()
	iw := &injectWriter{ResponseWriter: rw, status: http.StatusOK}
	iw.Header().Set("Content-Type", "text/html; charset=utf-8")
	iw.Header().Set("Content-Length", strconv.Itoa(len(body)))
	return iw, rw
}

// scriptBeforeBodyClose asserts the live-reload script appears before the
// first </body> in out.
func scriptBeforeBodyClose(t *testing.T, out string) {
	t.Helper()
	si := strings.Index(out, "EventSource")
	bi := strings.Index(out, "</body>")
	if si < 0 {
		t.Fatal("missing live-reload script")
	}
	if bi < 0 {
		t.Fatal("missing </body>")
	}
	if si > bi {
		t.Errorf("script should be before </body>: script@%d, </body>@%d", si, bi)
	}
}

func TestLiveReloadMiddleware(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		method      string
		content     string
		contentType string
		wantScript  bool
		wantStatus  int
	}{
		{
			name:        "HTML file should inject script",
			path:        "/test.html",
			method:      "GET",
			content:     "<html><body>Test</body></html>",
			contentType: "text/html",
			wantScript:  true,
			wantStatus:  http.StatusOK,
		},
		{
			name:        "Non-HTML file should not inject script",
			path:        "/style.css",
			method:      "GET",
			content:     "body { color: red; }",
			contentType: "text/css",
			wantScript:  false,
			wantStatus:  http.StatusOK,
		},
		{
			name:        "POST request should not inject script",
			path:        "/test.html",
			method:      "POST",
			content:     "<html><body>Test</body></html>",
			contentType: "text/html",
			wantScript:  false,
			wantStatus:  http.StatusOK,
		},
		{
			name:        "API path should not inject script",
			path:        "/api/data",
			method:      "GET",
			content:     `{"data": "test"}`,
			contentType: "application/json",
			wantScript:  false,
			wantStatus:  http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tt.contentType)
				w.WriteHeader(tt.wantStatus)
				_, _ = w.Write([]byte(tt.content))
			})

			handler := LiveReload(mockHandler)
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, rr.Code)
			}

			body := rr.Body.String()
			hasScript := strings.Contains(body, "EventSource")

			if hasScript != tt.wantScript {
				t.Errorf("Script injection mismatch: want %v, got %v", tt.wantScript, hasScript)
			}
		})
	}
}

func TestStreamingInjectLowercaseBody(t *testing.T) {
	body := []byte("<html><body>hi</body></html>")
	iw, rw := newInjectWriter(body)
	_, _ = iw.Write(body)
	iw.close()

	out := rw.Body.String()
	scriptBeforeBodyClose(t, out)
	if !strings.Contains(out, "hi") {
		t.Errorf("original content lost: %q", out)
	}
	if cl := rw.Header().Get("Content-Length"); cl != strconv.Itoa(len(body)+len(liveReloadScript)) {
		t.Errorf("Content-Length = %q, want %d", cl, len(body)+len(liveReloadScript))
	}
	if rw.Body.Len() != len(body)+len(liveReloadScript) {
		t.Errorf("body length = %d, want %d", rw.Body.Len(), len(body)+len(liveReloadScript))
	}
}

func TestStreamingInjectUppercaseBody(t *testing.T) {
	body := []byte("<HTML><BODY>hi</BODY></HTML>")
	iw, rw := newInjectWriter(body)
	_, _ = iw.Write(body)
	iw.close()

	out := rw.Body.String()
	if !strings.Contains(out, "EventSource") {
		t.Errorf("uppercase </BODY> should still get the script; got %q", out)
	}
	si := strings.Index(out, "EventSource")
	bi := strings.Index(out, "</BODY>")
	if si < 0 || bi < 0 || si > bi {
		t.Errorf("script should be before </BODY>: script@%d, </BODY>@%d", si, bi)
	}
}

// TestStreamingInjectSplitMarker verifies the marker is found even when it is
// split across write boundaries at every possible offset.
func TestStreamingInjectSplitMarker(t *testing.T) {
	prefix := []byte("<html><body>content")
	suffix := []byte("more</body></html>")
	marker := []byte("</body>")
	full := append(append([]byte{}, prefix...), append(marker, suffix...)...)

	for split := 0; split <= len(marker); split++ {
		t.Run("split/"+strconv.Itoa(split), func(t *testing.T) {
			cut := len(prefix) + split
			first := full[:cut]
			second := full[cut:]

			iw, rw := newInjectWriter(full)
			if len(first) > 0 {
				_, _ = iw.Write(first)
			}
			if len(second) > 0 {
				_, _ = iw.Write(second)
			}
			iw.close()

			out := rw.Body.String()
			scriptBeforeBodyClose(t, out)
			if rw.Body.Len() != len(full)+len(liveReloadScript) {
				t.Errorf("body length = %d, want %d (split=%d)", rw.Body.Len(), len(full)+len(liveReloadScript), split)
			}
		})
	}
}

func TestStreamingInjectNoBodyTagHasHTML(t *testing.T) {
	body := []byte("<html><body>hi</html>")
	iw, rw := newInjectWriter(body)
	_, _ = iw.Write(body)
	iw.close()

	out := rw.Body.String()
	if !strings.Contains(out, "EventSource") {
		t.Errorf("should append script at EOF; got %q", out)
	}
	// Script must come after </html> (the last tag in the body).
	hi := strings.Index(out, "</html>")
	si := strings.Index(out, "EventSource")
	if hi < 0 || si < 0 || si < hi {
		t.Errorf("script should follow </html>: </html>@%d, script@%d", hi, si)
	}
	if rw.Body.Len() != len(body)+len(liveReloadScript) {
		t.Errorf("body length = %d, want %d", rw.Body.Len(), len(body)+len(liveReloadScript))
	}
}

func TestStreamingInjectNoClosingTags(t *testing.T) {
	body := []byte("<html><body>hi")
	iw, rw := newInjectWriter(body)
	_, _ = iw.Write(body)
	iw.close()

	out := rw.Body.String()
	if !strings.Contains(out, "EventSource") {
		t.Errorf("should append script at EOF; got %q", out)
	}
	if rw.Body.Len() != len(body)+len(liveReloadScript) {
		t.Errorf("body length = %d, want %d", rw.Body.Len(), len(body)+len(liveReloadScript))
	}
}

func TestStreamingNonHTMLPassthrough(t *testing.T) {
	body := []byte("body { color: red; }")
	rw := httptest.NewRecorder()
	iw := &injectWriter{ResponseWriter: rw, status: http.StatusOK}
	iw.Header().Set("Content-Type", "text/css")
	iw.Header().Set("Content-Length", strconv.Itoa(len(body)))
	_, _ = iw.Write(body)
	iw.close()

	if rw.Body.String() != string(body) {
		t.Errorf("non-HTML body changed: got %q, want %q", rw.Body.String(), body)
	}
	if cl := rw.Header().Get("Content-Length"); cl != strconv.Itoa(len(body)) {
		t.Errorf("Content-Length should be unchanged for non-HTML: got %q, want %d", cl, len(body))
	}
}

// TestLiveReloadInjectsLargeHTML verifies that large HTML responses are no
// longer skipped: stream-inject finds </body> regardless of body size and
// injects the live-reload script, while streaming the body through.
func TestLiveReloadInjectsLargeHTML(t *testing.T) {
	inner := strings.Repeat("x", 4<<20) // 4 MB
	body := "<html><body>" + inner + "</body></html>"

	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		w.WriteHeader(http.StatusOK)
		// Write in 32 KB chunks, like io.Copy from http.FileServer.
		chunk := make([]byte, 32<<10)
		for off := 0; off < len(body); off += len(chunk) {
			end := off + len(chunk)
			if end > len(body) {
				end = len(body)
			}
			_, _ = w.Write([]byte(body[off:end]))
		}
	})

	handler := LiveReload(mockHandler)
	req := httptest.NewRequest(http.MethodGet, "/big.html", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	out := rr.Body.String()
	if !strings.Contains(out, "EventSource") {
		t.Errorf("large HTML should have the live-reload script injected")
	}
	scriptBeforeBodyClose(t, out)
	if len(out) != len(body)+len(liveReloadScript) {
		t.Errorf("body length = %d, want %d", len(out), len(body)+len(liveReloadScript))
	}
	if cl := rr.Header().Get("Content-Length"); cl != strconv.Itoa(len(body)+len(liveReloadScript)) {
		t.Errorf("Content-Length = %q, want %d", cl, len(body)+len(liveReloadScript))
	}
}

// TestLiveReloadSmallHTMLStillInjected verifies small HTML is still injected.
func TestLiveReloadSmallHTMLStillInjected(t *testing.T) {
	body := "<html><body>" + strings.Repeat("y", 100) + "</body></html>"
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	})

	handler := LiveReload(mockHandler)
	req := httptest.NewRequest(http.MethodGet, "/small.html", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !strings.Contains(rr.Body.String(), "EventSource") {
		t.Errorf("small HTML should have the live-reload script injected")
	}
}

// TestLiveReloadInjectsSubdirectoryIndex reproduces the bug where HTML
// served from a subfolder index (e.g. GET /sub/, which http.FileServer
// resolves to sub/index.html) was skipped by the live-reload middleware
// because the request path has no ".html" suffix and is not "/".
func TestLiveReloadInjectsSubdirectoryIndex(t *testing.T) {
	dir := mkdirFiles(t, map[string]string{
		"index.html":     "<html><body>root</body></html>",
		"sub/index.html": "<html><body>sub</body></html>",
		"sub/page.html":  "<html><body>page</body></html>",
		"sub/style.css":  "body { color: red; }",
	})

	handler := LiveReload(http.FileServer(http.Dir(dir)))
	ts := httptest.NewServer(handler)
	defer ts.Close()

	cases := []struct {
		name   string
		path   string
		want   string
		inject bool
	}{
		{"root index", "/", "root", true},
		{"root index explicit", "/index.html", "root", true},
		{"subfolder index trailing slash", "/sub/", "sub", true},
		{"subfolder explicit html", "/sub/page.html", "page", true},
		{"css passthrough", "/sub/style.css", "body { color: red; }", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := http.Get(ts.URL + tc.path)
			if err != nil {
				t.Fatalf("Get %s: %v", tc.path, err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("GET %s status = %d, want 200", tc.path, resp.StatusCode)
			}

			buf := make([]byte, 0, 4096)
			tmp := make([]byte, 1024)
			for {
				n, rerr := resp.Body.Read(tmp)
				if n > 0 {
					buf = append(buf, tmp[:n]...)
				}
				if rerr != nil {
					break
				}
			}
			body := string(buf)

			if !strings.Contains(body, tc.want) {
				t.Errorf("GET %s body missing %q; got %q", tc.path, tc.want, body)
			}
			if hasScript := strings.Contains(body, "EventSource"); hasScript != tc.inject {
				t.Errorf("GET %s script injected = %v, want %v; body=%q", tc.path, hasScript, tc.inject, body)
			}
		})
	}
}

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

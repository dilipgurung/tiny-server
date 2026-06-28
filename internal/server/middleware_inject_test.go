package server

import (
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

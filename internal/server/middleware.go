package server

import (
	"bytes"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// liveReloadScript is the snippet injected into HTML responses to open the
// Server-Sent Events connection and reload the page on change.
var liveReloadScript = []byte(`
				<script>
					// Live reload via Server-Sent Events
					(function () {
						const evtSource = new EventSource("/.live-reload");

						evtSource.onopen = () => console.log('Live reload connected');
						evtSource.onmessage = (event) => {
							if (event.data === 'reload') {
								console.log('Reloading page...');
								window.location.reload();
							}
						};
						evtSource.onerror = (err) => {
							console.error('Live reload error - reconnecting...', err);
						};
					})();
				</script>
			`)

// bodyCloseTag is the case-insensitive marker scanned for in the stream.
var bodyCloseTag = []byte("</body>")

// injectionMarkerLen is the length of the marker scanned for. The streaming
// writer holds back at most injectionMarkerLen-1 bytes to detect a marker
// split across write boundaries, so its memory is O(marker) regardless of
// the response size.
const injectionMarkerLen = len("</body>") // 7

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
	if path == "/" {
		return false
	}
	for _, seg := range strings.Split(path, "/") {
		if seg == "" {
			continue
		}
		if strings.HasPrefix(seg, ".") {
			return true
		}
	}
	return false
}

func liveReload(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip non-GET requests and API calls. HEAD bypasses injection so
		// Content-Length reflects the real file size.
		if r.Method != http.MethodGet || strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}

		// Wrap every other GET response and let injectWriter decide whether
		// to inject based on the response's Content-Type. Deciding by path
		// suffix is unreliable: a subfolder index served at "/sub/" has no
		// ".html" suffix, and some HTML files use ".htm" or no extension at
		// all. injectWriter streams non-HTML responses through untouched.
		iw := &injectWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(iw, r)
		iw.close()
	})
}

// injectWriter wraps the real ResponseWriter and injects the live-reload
// script into an HTML stream by scanning for </body> as bytes flow through.
//
// It holds back at most injectionMarkerLen-1 bytes to detect a marker split
// across write boundaries, so memory is O(marker) regardless of body size
// and every HTML response gets the script (no size-based skip). When the
// script is injected the body grows by len(liveReloadScript); Content-Length
// is adjusted up front when the upstream set it, otherwise it is left absent
// so net/http chunk-encodes the response.
//
// WriteHeader is delayed until the first byte is sent (or the handler
// returns) so headers can be adjusted. Non-injectable responses (non-200,
// non-HTML, range/206, etc.) stream through verbatim untouched.
type injectWriter struct {
	http.ResponseWriter
	status      int
	headerSent  bool
	evaluated   bool
	canInject   bool
	passthrough bool
	injected    bool
	clAdjusted  bool
	pending     [injectionMarkerLen]byte
	pendingLen  int
}

func (w *injectWriter) pendingSlice() []byte {
	return w.pending[:w.pendingLen]
}

func (w *injectWriter) WriteHeader(code int) {
	if w.headerSent {
		return
	}
	// Record the status but don't forward yet; we may adjust headers.
	w.status = code
}

func (w *injectWriter) Write(b []byte) (int, error) {
	n := len(b)
	if !w.evaluated {
		w.evaluate(b)
	}
	if w.injected || w.passthrough {
		return w.ResponseWriter.Write(b)
	}

	m := len(bodyCloseTag)
	pend := w.pendingSlice()

	// Search for the marker in (pending ++ b).
	idx := -1
	idxInPending := false

	// (a) marker straddling the pending|b boundary: starts in pending, ends in b.
	if w.pendingLen > 0 && len(b) > 0 {
		startMin := w.pendingLen - (m - 1)
		if startMin < 0 {
			startMin = 0
		}
		for s := startMin; s < w.pendingLen; s++ {
			plen := w.pendingLen - s // bytes contributed by pending
			need := m - plen         // bytes required from b
			if need > len(b) {
				continue
			}
			if bytes.EqualFold(pend[s:], bodyCloseTag[:plen]) &&
				bytes.EqualFold(b[:need], bodyCloseTag[plen:]) {
				idx = s
				idxInPending = true
				break
			}
		}
	}
	// (b) marker fully within b.
	if idx == -1 {
		if j := indexFold(b, bodyCloseTag); j != -1 {
			idx = j
			idxInPending = false
		}
	}

	if idx != -1 {
		w.ensureHeader()
		if idxInPending {
			// Marker starts at pending[idx] and ends in b.
			plen := w.pendingLen - idx
			need := m - plen
			if idx > 0 {
				_, _ = w.ResponseWriter.Write(pend[:idx])
			}
			_, _ = w.ResponseWriter.Write(liveReloadScript)
			_, _ = w.ResponseWriter.Write(pend[idx:]) // marker head
			_, _ = w.ResponseWriter.Write(b[:need])   // marker tail
			_, _ = w.ResponseWriter.Write(b[need:])   // remainder of b
		} else {
			// Marker fully in b at offset idx.
			if w.pendingLen > 0 {
				_, _ = w.ResponseWriter.Write(pend)
			}
			_, _ = w.ResponseWriter.Write(b[:idx])
			_, _ = w.ResponseWriter.Write(liveReloadScript)
			_, _ = w.ResponseWriter.Write(b[idx:]) // marker + remainder
		}
		w.pendingLen = 0
		w.injected = true
		return n, nil
	}

	// Not found: flush the safe prefix of (pending ++ b) and hold back the
	// last m-1 bytes, which may start a marker continuing into the next write.
	w.ensureHeader()
	L := w.pendingLen + len(b)
	holdBack := m - 1
	if holdBack > L {
		holdBack = L
	}
	safeLen := L - holdBack
	if safeLen <= w.pendingLen {
		// Flush only from pending; hold pending[safeLen:] ++ b.
		if safeLen > 0 {
			_, _ = w.ResponseWriter.Write(pend[:safeLen])
		}
		np := w.pendingLen - safeLen
		copy(w.pending[:], pend[safeLen:])
		copy(w.pending[np:], b)
		w.pendingLen = np + len(b)
	} else {
		// Flush all pending plus the safe prefix of b; hold b's tail.
		_, _ = w.ResponseWriter.Write(pend)
		k := safeLen - w.pendingLen
		_, _ = w.ResponseWriter.Write(b[:k])
		copy(w.pending[:], b[k:])
		w.pendingLen = len(b) - k
	}
	return n, nil
}

// close finalizes the response after the handler returns.
func (w *injectWriter) close() {
	if w.injected || w.passthrough {
		if w.pendingLen > 0 {
			_, _ = w.ResponseWriter.Write(w.pendingSlice())
			w.pendingLen = 0
		}
		return
	}
	if !w.evaluated {
		// No body was written (empty 200, 304, redirect without body, etc.):
		// forward the original headers and status untouched.
		w.flushHeaderOriginal()
		return
	}
	// Searching reached EOF without a </body>: flush the held-back bytes and
	// append the script. Content-Length was already adjusted to account for it.
	w.ensureHeader()
	if w.pendingLen > 0 {
		_, _ = w.ResponseWriter.Write(w.pendingSlice())
		w.pendingLen = 0
	}
	if w.canInject {
		_, _ = w.ResponseWriter.Write(liveReloadScript)
	}
}

// evaluate decides on the first Write whether injection is possible, based on
// the recorded status and the response's Content-Type.
func (w *injectWriter) evaluate(b []byte) {
	w.evaluated = true
	ct := w.Header().Get("Content-Type")
	if ct == "" && len(b) > 0 {
		peek := b
		if len(peek) > 512 {
			peek = peek[:512]
		}
		ct = http.DetectContentType(peek)
	}
	w.canInject = w.status == http.StatusOK && strings.HasPrefix(ct, "text/html")
	if !w.canInject {
		w.passthrough = true
		w.flushHeaderOriginal()
	}
}

// ensureHeader adjusts Content-Length (when injecting) and forwards the
// recorded status and headers exactly once.
func (w *injectWriter) ensureHeader() {
	if w.headerSent {
		return
	}
	w.headerSent = true
	if w.canInject {
		w.adjustContentLength()
	}
	w.ResponseWriter.WriteHeader(w.status)
}

// flushHeaderOriginal forwards the original headers and status without any
// Content-Length adjustment.
func (w *injectWriter) flushHeaderOriginal() {
	if w.headerSent {
		return
	}
	w.headerSent = true
	w.ResponseWriter.WriteHeader(w.status)
}

// adjustContentLength grows the upstream Content-Length by len(liveReloadScript)
// so it matches the injected body. If the upstream set no Content-Length it is
// left absent so net/http chunk-encodes the response.
func (w *injectWriter) adjustContentLength() {
	if w.clAdjusted {
		return
	}
	w.clAdjusted = true
	if cl := w.Header().Get("Content-Length"); cl != "" {
		if n, err := strconv.Atoi(cl); err == nil {
			w.Header().Set("Content-Length", strconv.Itoa(n+len(liveReloadScript)))
		}
	}
}

// indexFold returns the index of needle in haystack using case-insensitive
// comparison, or -1. It performs no allocations.
func indexFold(haystack, needle []byte) int {
	n := len(needle)
	if n == 0 || len(haystack) < n {
		return -1
	}
	for i := 0; i+n <= len(haystack); i++ {
		if bytes.EqualFold(haystack[i:i+n], needle) {
			return i
		}
	}
	return -1
}

func logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rec := &StatusRecorder{ResponseWriter: w, StatusCode: 200}
		next.ServeHTTP(rec, r)
		duration := time.Since(start)

		log.Printf("%-6s %3d %12s %-40s",
			r.Method,
			rec.StatusCode,
			duration,
			r.URL.Path,
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

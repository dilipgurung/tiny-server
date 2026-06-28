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

// bodyCloseTag is the case-insensitive marker searched for in the stream.
var bodyCloseTag = []byte("</body>")

// injectionBufferCap is the maximum number of response bytes buffered while
// looking for a </body> injection point. Responses that exceed this cap are
// streamed through verbatim (no live-reload script), which bounds peak
// memory per request to roughly this size plus the script.
const injectionBufferCap = 256 << 10 // 256 KB

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

		// Skip non-HTML files (CSS, JS, images, etc.).
		if !strings.HasSuffix(r.URL.Path, ".html") && r.URL.Path != "/" {
			next.ServeHTTP(w, r)
			return
		}

		iw := &injectWriter{
			ResponseWriter: w,
			status:         http.StatusOK,
			cap:            injectionBufferCap,
			mode:           modeBuffer,
		}
		next.ServeHTTP(iw, r)
		iw.close()
	})
}

// injectMode tracks the streaming writer's current phase.
type injectMode int

const (
	modeBuffer      injectMode = iota // buffering up to cap looking for </body>
	modeInjected                      // script already injected; streaming the rest
	modePassthrough                   // gave up on injection; streaming verbatim
)

// injectWriter wraps the real ResponseWriter and buffers up to cap bytes to
// find a </body> injection point for the live-reload script. If the body
// exceeds the cap (or isn't injectable HTML), it flushes the buffer and
// streams the remainder verbatim, bounding memory to cap + script size.
//
// It delays WriteHeader until the first byte is sent (or the handler
// returns) so it can adjust Content-Length when the script is injected.
type injectWriter struct {
	http.ResponseWriter
	status     int
	headerSent bool
	mode       injectMode
	buf        bytes.Buffer
	cap        int
	canInject  bool
	evaluated  bool
}

func (w *injectWriter) WriteHeader(code int) {
	if w.headerSent {
		return
	}
	// Record the status but don't forward yet; we may inject.
	w.status = code
}

func (w *injectWriter) Write(b []byte) (int, error) {
	n := len(b)
	if !w.evaluated {
		w.evaluate(b)
	}
	if w.mode == modePassthrough || w.mode == modeInjected {
		return w.ResponseWriter.Write(b)
	}

	// modeBuffer: buffer up to cap, scanning for </body>.
	room := w.cap - w.buf.Len()
	if room <= 0 {
		w.switchToPassthrough()
		return w.ResponseWriter.Write(b)
	}
	take := n
	if take > room {
		take = room
	}
	w.buf.Write(b[:take])

	if w.canInject {
		if idx := indexFold(w.buf.Bytes(), bodyCloseTag); idx != -1 {
			w.injectAt(idx)
			if take < n {
				_, _ = w.ResponseWriter.Write(b[take:])
			}
			return n, nil
		}
	}

	if w.buf.Len() >= w.cap {
		// Cap reached without a </body>: stream the rest verbatim.
		w.switchToPassthrough()
		if take < n {
			_, _ = w.ResponseWriter.Write(b[take:])
		}
		return n, nil
	}

	return n, nil
}

// close finalizes the response after the handler returns.
func (w *injectWriter) close() {
	if w.mode == modeInjected || w.mode == modePassthrough {
		return
	}
	// modeBuffer: the full body fit in the buffer and no </body> was found.
	body := w.buf.Bytes()
	if len(body) == 0 {
		// No body was written (e.g. 304 Not Modified). Forward the original
		// headers and status untouched.
		w.flushHeader()
		return
	}
	if w.canInject {
		body = injectLiveReloadScript(body)
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.flushHeader()
	_, _ = w.ResponseWriter.Write(body)
}

// evaluate decides on the first Write whether injection is possible, based
// on the recorded status and the response's Content-Type.
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
		w.mode = modePassthrough
		w.flushHeader()
	}
}

// flushHeader forwards the recorded status (and the headers already set on
// the real writer) exactly once.
func (w *injectWriter) flushHeader() {
	if w.headerSent {
		return
	}
	w.headerSent = true
	w.ResponseWriter.WriteHeader(w.status)
}

// switchToPassthrough flushes the buffered content verbatim and switches to
// streaming the remainder directly to the client.
func (w *injectWriter) switchToPassthrough() {
	w.mode = modePassthrough
	w.flushHeader()
	if w.buf.Len() > 0 {
		_, _ = w.ResponseWriter.Write(w.buf.Bytes())
		w.buf.Reset()
	}
}

// injectAt writes the buffered content with the script inserted before the
// </body> at idx, then streams any subsequent bytes through unchanged.
func (w *injectWriter) injectAt(idx int) {
	w.mode = modeInjected
	// The injected script grows the body by len(liveReloadScript). Adjust
	// Content-Length if the upstream handler set one; otherwise drop it so
	// net/http chunk-encodes the response.
	if cl := w.Header().Get("Content-Length"); cl != "" {
		if n, err := strconv.Atoi(cl); err == nil {
			w.Header().Set("Content-Length", strconv.Itoa(n+len(liveReloadScript)))
		}
	} else {
		w.Header().Del("Content-Length")
	}
	w.flushHeader()
	body := w.buf.Bytes()
	_, _ = w.ResponseWriter.Write(body[:idx])
	_, _ = w.ResponseWriter.Write(liveReloadScript)
	_, _ = w.ResponseWriter.Write(body[idx:])
	w.buf.Reset()
}

// injectLiveReloadScript inserts the live-reload snippet into a complete,
// in-memory HTML body (used when the whole body fit in the buffer). It
// matches </body> case-insensitively and injects before it; if no </body>
// is present it falls back to injecting after </html>, and finally appends
// at the end of the body. Non-HTML content is returned unchanged. It
// allocates a single copy rather than lowercasing the whole body.
func injectLiveReloadScript(body []byte) []byte {
	if !strings.Contains(http.DetectContentType(body), "text/html") {
		return body
	}
	if idx := indexFold(body, bodyCloseTag); idx != -1 {
		return splice(body, idx, liveReloadScript)
	}
	if idx := indexFold(body, []byte("</html>")); idx != -1 {
		return splice(body, idx+len("</html>"), liveReloadScript)
	}
	return append(body, liveReloadScript...)
}

// splice returns a new byte slice with ins inserted into body at position at.
func splice(body []byte, at int, ins []byte) []byte {
	out := make([]byte, 0, len(body)+len(ins))
	out = append(out, body[:at]...)
	out = append(out, ins...)
	out = append(out, body[at:]...)
	return out
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

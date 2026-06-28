package server

import (
	"bytes"
	"log"
	"net/http"
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

// maxBufferedHTML is the upper bound for buffering an HTML response so the
// live-reload script can be injected. Responses larger than this are streamed
// as-is without injection, trading the live-reload feature for memory safety.
const maxBufferedHTML = 1 << 20 // 1 MB

func liveReload(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip non-GET requests and API calls
		if r.Method != http.MethodGet || strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}

		// Skip non-HTML files (CSS, JS, images, etc.)
		if !strings.HasSuffix(r.URL.Path, ".html") && r.URL.Path != "/" {
			next.ServeHTTP(w, r)
			return
		}

		recorder := NewResponseRecorder(w)
		next.ServeHTTP(recorder, r)

		body := recorder.buf.Bytes()
		// Skip injection for oversized responses: buffering unbounded HTML
		// would risk high memory use. Stream the captured body as-is.
		if recorder.statusCode == http.StatusOK && len(body) <= maxBufferedHTML {
			body = injectLiveReloadScript(body)
		}

		if r.Method != http.MethodHead {
			WriteResponse(w, recorder, body)
		}
	})
}

// injectLiveReloadScript inserts the live-reload snippet into an HTML body.
// It matches </body> case-insensitively and injects before it; if no
// </body> is present it falls back to injecting after </html>, and finally
// appends at the end of the body. Non-HTML content is returned unchanged.
func injectLiveReloadScript(body []byte) []byte {
	if !strings.Contains(http.DetectContentType(body), "text/html") {
		return body
	}

	lower := bytes.ToLower(body)

	// Inject before the first closing </body> (case-insensitive).
	if idx := bytes.Index(lower, []byte("</body>")); idx != -1 {
		return append(body[:idx], append(liveReloadScript, body[idx:]...)...)
	}
	// Fall back to injecting after </html> (case-insensitive).
	if idx := bytes.Index(lower, []byte("</html>")); idx != -1 {
		after := idx + len("</html>")
		return append(body[:after], append(liveReloadScript, body[after:]...)...)
	}
	// No closing tags at all: append at the end of the body.
	return append(body, liveReloadScript...)
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
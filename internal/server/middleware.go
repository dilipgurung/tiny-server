package server

import (
	"bytes"
	"log"
	"net/http"
	"strings"
	"time"
)

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
		if recorder.statusCode == http.StatusOK {
			body = injectLiveReloadScript(body)
		}

		if r.Method != http.MethodHead {
			WriteResponse(w, recorder, body)
		}
	})
}

func injectLiveReloadScript(body []byte) []byte {
	if strings.Contains(http.DetectContentType(body), "text/html") {
		if idx := bytes.LastIndex(body, []byte("</body>")); idx != -1 {
			script := []byte(`
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
			return append(body[:idx], append(script, body[idx:]...)...)
		}
	}
	return body
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

package server

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// liveReload wraps an http.Handler to inject live reload script
func liveReload(next http.Handler, port string) http.Handler {
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
			body = injectLiveReloadScript(body, port)
		}

		if r.Method != http.MethodHead {
			WriteResponse(w, recorder, body)
		}
	})
}

// injectLiveReload injects the WebSocket live reload script into HTML responses
func injectLiveReloadScript(body []byte, port string) []byte {
	if strings.Contains(http.DetectContentType(body), "text/html") {
		if idx := bytes.LastIndex(body, []byte("</body>")); idx != -1 {
			script := []byte(fmt.Sprintf(`
				<script>
					// Live reload via WebSocket
					(function () {
						function connectWebSocket() {
							const wsProtocol = location.protocol === 'https:' ? 'wss' : 'ws';
							const ws = new WebSocket(wsProtocol + "://" + location.hostname + ":%s/.live-reload");

							ws.onopen = () => console.log('Live reload connected');
							ws.onmessage = (event) => {
								if (event.data === 'reload') {
									console.log('Reloading page...');
									window.location.reload();
								}
							};
							ws.onclose = () => {
								console.log('Live reload disconnected - reconnecting...');
								setTimeout(connectWebSocket, 1000);
							};
							ws.onerror = (err) => {
								console.error('WebSocket error:', err);
							};
						}

						connectWebSocket();
					})();
				</script>
			`, port))
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

package server

import (
	"strings"
	"testing"
)

func TestInjectLiveReloadScript(t *testing.T) {
	hasScript := func(b []byte) bool { return strings.Contains(string(b), "EventSource") }

	tests := []struct {
		name  string
		body  string
		want  string // substring expected to be present
		check func(b []byte) bool
	}{
		{
			name:  "lowercase body tag",
			body:  "<html><body>hi</body></html>",
			check: hasScript,
		},
		{
			name: "uppercase BODY tag",
			body: "<HTML><BODY>hi</BODY></HTML>",
			check: func(b []byte) bool {
				return hasScript(b) && strings.Contains(string(b), "</BODY>")
			},
		},
		{
			name:  "no closing body but has html",
			body:  "<html><body>hi</html>",
			check: hasScript,
		},
		{
			name:  "no closing tags at all",
			body:  "<html><body>hi",
			check: hasScript,
		},
		{
			name:  "non-HTML empty body unchanged",
			body:  "",
			check: func(b []byte) bool { return len(b) == 0 },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := injectLiveReloadScript([]byte(tt.body))
			if !tt.check(out) {
				t.Errorf("injectLiveReloadScript(%q) = %q; check failed", tt.body, out)
			}
			if tt.name == "non-HTML empty body unchanged" && len(out) != 0 {
				t.Errorf("empty body should be unchanged; got len %d", len(out))
			}
		})
	}
}

func TestInjectLiveReloadScriptNonHTML(t *testing.T) {
	body := []byte("body { color: red; }")
	out := injectLiveReloadScript(body)
	if string(out) != string(body) {
		t.Errorf("non-HTML body should be unchanged; got %q", out)
	}
}

func TestInjectLiveReloadScriptPreservesBodyContent(t *testing.T) {
	body := []byte("<html><body>Hello World</body></html>")
	out := injectLiveReloadScript(body)
	if !strings.Contains(string(out), "Hello World") {
		t.Errorf("original content lost: %q", out)
	}
	// Script must appear before </body>.
	idxScript := strings.Index(string(out), "EventSource")
	idxBody := strings.Index(string(out), "</body>")
	if idxScript < 0 || idxBody < 0 || idxScript > idxBody {
		t.Errorf("script should be injected before </body>; script=%d body=%d",
			idxScript, idxBody)
	}
}
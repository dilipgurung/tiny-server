package server

import (
	"os"
	"path/filepath"
	"testing"
)

// mkdirFiles creates a temporary directory containing the given files
// (mapping relative path -> content) and returns its path. Nested paths
// (e.g. "sub/note.txt") create intermediate directories. The directory
// and its contents are cleaned up automatically when the test ends.
func mkdirFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("MkdirAll %s: %v", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile %s: %v", rel, err)
		}
	}
	return dir
}

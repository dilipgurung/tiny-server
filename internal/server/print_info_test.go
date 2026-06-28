package server

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
)

// TestPrintInfo verifies PrintInfo reports the served directory and at
// least one listen address without error.
func TestPrintInfo(t *testing.T) {
	srv, err := NewServer("0", "testdata_dotfiles")
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	// Capture stdout by swapping os.Stdout for a pipe.
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	srv.PrintInfo("8123", "testdata_dotfiles")
	_ = w.Close()

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	out := buf.String()
	for _, want := range []string{"testdata_dotfiles", "Available on:", "Press CTRL+C"} {
		if !strings.Contains(out, want) {
			t.Errorf("PrintInfo output missing %q; got %q", want, out)
		}
	}
}
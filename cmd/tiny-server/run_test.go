package main

import (
	"net"
	"strings"
	"testing"
)

// TestRunExitStatuses verifies that startup failures are reported with a
// non-zero exit status. The custom flag.Usage used to call os.Exit(0),
// which made every unknown option and every listener failure look like a
// successful run to scripts and service supervisors.
func TestRunExitStatuses(t *testing.T) {
	// Bind the wildcard address so the "already in use" case conflicts with
	// the server's own ":port" bind. On BSD/macOS a wildcard bind can coexist
	// with a loopback bind on the same port, so 127.0.0.1:0 would not clash.
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer func() { _ = ln.Close() }()
	busyPort := portOf(t, ln)

	tests := []struct {
		name string
		args []string
		want int
	}{
		{"help", []string{"-h"}, exitOK},
		{"version", []string{"-v"}, exitOK},
		{"unknown flag", []string{"--definitely-invalid"}, exitUsage},
		{"unparsable port", []string{"-p", "notaport", "-d", "."}, exitFail},
		{"port already in use", []string{"-p", busyPort, "-d", "."}, exitFail},
		{"missing served dir", []string{"-d", "./does-not-exist"}, exitFail},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr strings.Builder
			if got := run(tt.args, &stdout, &stderr); got != tt.want {
				t.Errorf("run(%v) = %d, want %d (stderr: %q)", tt.args, got, tt.want, stderr.String())
			}
		})
	}
}

// TestRunHelpGoesToStdout verifies -h prints usage on stdout and stays
// silent on stderr, so `tiny-server -h | less` still works.
func TestRunHelpGoesToStdout(t *testing.T) {
	var stdout, stderr strings.Builder
	if got := run([]string{"-h"}, &stdout, &stderr); got != exitOK {
		t.Fatalf("run(-h) = %d, want %d", got, exitOK)
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Errorf("stdout missing usage text: %q", stdout.String())
	}
	for _, flagName := range []string{"-p", "-d", "-v"} {
		if !strings.Contains(stdout.String(), flagName) {
			t.Errorf("usage missing %s flag: %q", flagName, stdout.String())
		}
	}
	if stderr.String() != "" {
		t.Errorf("stderr should be empty for -h, got %q", stderr.String())
	}
}

// TestRunUnknownFlagReportsOnStderr verifies parse errors are diagnosed on
// stderr rather than silently succeeding.
func TestRunUnknownFlagReportsOnStderr(t *testing.T) {
	var stdout, stderr strings.Builder
	if got := run([]string{"--definitely-invalid"}, &stdout, &stderr); got != exitUsage {
		t.Fatalf("run(--definitely-invalid) = %d, want %d", got, exitUsage)
	}
	if !strings.Contains(stderr.String(), "definitely-invalid") {
		t.Errorf("stderr should name the bad flag, got %q", stderr.String())
	}
	if stdout.String() != "" {
		t.Errorf("stdout should be empty for a parse error, got %q", stdout.String())
	}
}

func portOf(t *testing.T, ln net.Listener) string {
	t.Helper()
	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("SplitHostPort: %v", err)
	}
	return port
}

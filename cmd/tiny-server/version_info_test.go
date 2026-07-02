package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewVersionInfoDefaults(t *testing.T) {
	v := NewVersionInfo("", "")
	if v == nil {
		t.Fatal("expected non-nil VersionInfo")
	}
	if v.tinyServerVersion != "(unknown)" {
		t.Errorf("tinyServerVersion = %q, want %q", v.tinyServerVersion, "(unknown)")
	}
	if !strings.HasPrefix(v.goVersion, "go") {
		t.Errorf("goVersion = %q, want a go runtime version", v.goVersion)
	}
}

func TestNewVersionInfoExplicit(t *testing.T) {
	v := NewVersionInfo("v1.2.3", "go1.24.2")
	if v.tinyServerVersion != "v1.2.3" {
		t.Errorf("tinyServerVersion = %q, want %q", v.tinyServerVersion, "v1.2.3")
	}
	if v.goVersion != "go1.24.2" {
		t.Errorf("goVersion = %q, want %q", v.goVersion, "go1.24.2")
	}
}

func TestVersionInfoPrintSplashTo(t *testing.T) {
	v := NewVersionInfo("v9.9.9", "go1.24.2")
	var buf bytes.Buffer
	v.PrintSplashTo(&buf)

	out := buf.String()
	for _, want := range []string{"v9.9.9", "go1.24.2", "lightweight static HTTP server"} {
		if !strings.Contains(out, want) {
			t.Errorf("splash missing %q; got %q", want, out)
		}
	}
}

func TestVersionInfoPrintSplashDefaults(t *testing.T) {
	v := NewVersionInfo("", "")
	var buf bytes.Buffer
	v.PrintSplashTo(&buf)
	out := buf.String()
	if !strings.Contains(out, "(unknown)") {
		t.Errorf("default splash should mention (unknown); got %q", out)
	}
}

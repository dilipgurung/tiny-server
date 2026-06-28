package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetServeDirDefault(t *testing.T) {
	// In a directory with no ./public, getServeDir returns ".".
	dir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	defer os.Chdir(cwd)

	if got := getServeDir(""); got != "." {
		t.Errorf("getServeDir(\"\") = %q, want %q", got, ".")
	}
}

func TestGetServeDirPublic(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "public"), 0755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	defer os.Chdir(cwd)

	if got := getServeDir(""); got != "./public" {
		t.Errorf("getServeDir(\"\") = %q, want %q", got, "./public")
	}
}

func TestGetServeDirExplicit(t *testing.T) {
	if got := getServeDir("/tmp/some-dir"); got != "/tmp/some-dir" {
		t.Errorf("getServeDir explicit = %q, want %q", got, "/tmp/some-dir")
	}
}
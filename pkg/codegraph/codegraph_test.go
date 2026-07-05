package codegraph

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHasIndexTrue(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".codegraph"), 0755)
	if !HasIndex(dir) {
		t.Error("expected HasIndex true when .codegraph/ exists")
	}
}

func TestHasIndexFalse(t *testing.T) {
	dir := t.TempDir()
	if HasIndex(dir) {
		t.Error("expected HasIndex false when no .codegraph/")
	}
}

func TestWriteMCPConfigSkipsWhenNoIndex(t *testing.T) {
	dir := t.TempDir()
	path, err := WriteMCPConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if path != "" {
		t.Errorf("expected empty path when no .codegraph/, got %q", path)
	}
}

func TestWriteMCPConfigSkipsWhenNoCLI(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".codegraph"), 0755)
	// Simulate no codegraph on PATH and no fallback at ~/.local/bin by pointing
	// HOME to the temp dir (so the ~/.local/bin fallback resolves to nowhere).
	t.Setenv("PATH", "/usr/bin:/bin")
	t.Setenv("HOME", dir)

	path, err := WriteMCPConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if path != "" {
		t.Errorf("expected empty path when codegraph CLI not installed, got %q", path)
	}
}

func TestWriteMCPConfigCreatesFile(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".codegraph"), 0755)
	// Create a fake codegraph binary on PATH.
	fakeBin := filepath.Join(dir, "fakebin")
	os.MkdirAll(fakeBin, 0755)
	fake := filepath.Join(fakeBin, "codegraph")
	os.WriteFile(fake, []byte("#!/bin/sh\nexit 0\n"), 0755)
	t.Setenv("PATH", fakeBin)

	path, err := WriteMCPConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if path == "" {
		t.Fatal("expected non-empty path when codegraph installed and index exists")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config file not written: %v", err)
	}
	data, _ := os.ReadFile(path)
	content := string(data)
	if !contains(content, "codegraph") || !contains(content, "serve") {
		t.Errorf("config missing codegraph server entry, got %q", content)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

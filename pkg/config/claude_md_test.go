package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteCLAUDEMDCreatesWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CLAUDE.md")

	action, err := WriteCLAUDEMD(path)
	if err != nil {
		t.Fatal(err)
	}
	if action != "created" {
		t.Errorf("expected created, got %s", action)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), karpathyMarker) {
		t.Errorf("file should contain guidelines, got %q", string(data))
	}
}

func TestWriteCLAUDEMDSkipsWhenAlreadyPresent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CLAUDE.md")
	existing := "# My Project\n\nSome rules.\n\n" + KarpathyGuidelines
	os.WriteFile(path, []byte(existing), 0644)

	action, err := WriteCLAUDEMD(path)
	if err != nil {
		t.Fatal(err)
	}
	if action != "skipped" {
		t.Errorf("expected skipped, got %s", action)
	}
	data, _ := os.ReadFile(path)
	if string(data) != existing {
		t.Errorf("file should be unchanged, got %q", string(data))
	}
	// Ensure not duplicated.
	if strings.Count(string(data), karpathyMarker) != 1 {
		t.Errorf("marker should appear once, got %d", strings.Count(string(data), karpathyMarker))
	}
}

func TestWriteCLAUDEMDAppendsWhenDifferent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CLAUDE.md")
	existing := "# My Project\n\nSome custom rules."
	os.WriteFile(path, []byte(existing), 0644)

	action, err := WriteCLAUDEMD(path)
	if err != nil {
		t.Fatal(err)
	}
	if action != "appended" {
		t.Errorf("expected appended, got %s", action)
	}
	data, _ := os.ReadFile(path)
	content := string(data)
	if !strings.HasPrefix(content, "# My Project") {
		t.Errorf("existing content should be preserved at top, got %q", content)
	}
	if !strings.Contains(content, karpathyMarker) {
		t.Errorf("appended guidelines missing, got %q", content)
	}
	if !strings.Contains(content, "---") {
		t.Errorf("separator missing, got %q", content)
	}
	if strings.Count(content, karpathyMarker) != 1 {
		t.Errorf("marker should appear once after append, got %d", strings.Count(content, karpathyMarker))
	}
}

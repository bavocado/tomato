package archive

import (
	"os"
	"path/filepath"
	"testing"
)

func TestArchiveTrio(t *testing.T) {
	dir := t.TempDir()

	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "architecture.md"), []byte("# Arch v1"), 0644)
	os.WriteFile(filepath.Join(dir, "ui-spec.md"), []byte("# UI v1"), 0644)
	os.WriteFile(filepath.Join(dir, "implementation.md"), []byte("# Impl v1"), 0644)

	ver, err := ArchiveTrio(dir)
	if err != nil {
		t.Fatal(err)
	}

	if ver != 1 {
		t.Errorf("expected version 1, got %d", ver)
	}

	v1Dir := filepath.Join(dir, "v1")
	if _, err := os.Stat(filepath.Join(v1Dir, "architecture.md")); os.IsNotExist(err) {
		t.Error("architecture.md was not archived")
	}
	if _, err := os.Stat(filepath.Join(v1Dir, "ui-spec.md")); os.IsNotExist(err) {
		t.Error("ui-spec.md was not archived")
	}

	// Root files must be retained (copy, not move) — design §3.5: the root
	// trio is always the "latest" set and is read by downstream steps.
	if _, err := os.Stat(filepath.Join(dir, "architecture.md")); os.IsNotExist(err) {
		t.Error("architecture.md should remain in root after archiving")
	}

	// The archived copy must hold the pre-rewrite (design-intent) content.
	arch, _ := os.ReadFile(filepath.Join(v1Dir, "architecture.md"))
	if string(arch) != "# Arch v1" {
		t.Errorf("archived architecture.md = %q, want %q", string(arch), "# Arch v1")
	}
}

func TestArchiveNextVersion(t *testing.T) {
	dir := t.TempDir()

	os.MkdirAll(filepath.Join(dir, "v1"), 0755)
	os.WriteFile(filepath.Join(dir, "architecture.md"), []byte("# Arch v2"), 0644)
	os.WriteFile(filepath.Join(dir, "ui-spec.md"), []byte("# UI v2"), 0644)
	os.WriteFile(filepath.Join(dir, "implementation.md"), []byte("# Impl v2"), 0644)

	ver, err := ArchiveTrio(dir)
	if err != nil {
		t.Fatal(err)
	}

	if ver != 2 {
		t.Errorf("expected version 2 (v1 already exists), got %d", ver)
	}
}

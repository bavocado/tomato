package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bavocado/tomato/pkg/config"
)

func TestInitCommand(t *testing.T) {
	t.Setenv("TOMATO_SKIP_CODEGRAPH", "1") // skip codegraph install in tests
	tempDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(origWd)

	initCmd := NewInitCmd()
	initCmd.SetArgs([]string{})
	if err := initCmd.Execute(); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(tempDir, "tomato.yaml")); os.IsNotExist(err) {
		t.Error("tomato.yaml was not created")
	}
	if _, err := os.Stat(filepath.Join(tempDir, ".tomato", "runs")); os.IsNotExist(err) {
		t.Error(".tomato/runs was not created")
	}
	if _, err := os.Stat(filepath.Join(tempDir, "CLAUDE.md")); os.IsNotExist(err) {
		t.Error("CLAUDE.md was not created")
	}
}

func TestInitAppendsCLAUDEMDWhenDifferent(t *testing.T) {
	t.Setenv("TOMATO_SKIP_CODEGRAPH", "1")
	tempDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(origWd)

	// Pre-existing CLAUDE.md with user content but no Karpathy guidelines.
	os.WriteFile(filepath.Join(tempDir, "CLAUDE.md"), []byte("# My Project\n\nCustom rules."), 0644)

	initCmd := NewInitCmd()
	initCmd.SetArgs([]string{})
	if err := initCmd.Execute(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(tempDir, "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.HasPrefix(content, "# My Project") {
		t.Errorf("existing content should be preserved, got %q", content)
	}
	if !strings.Contains(content, "# Karpathy Guidelines") {
		t.Errorf("Karpathy guidelines should be appended, got %q", content)
	}
}

func TestInitSkipsCLAUDEMDWhenAlreadyPresent(t *testing.T) {
	t.Setenv("TOMATO_SKIP_CODEGRAPH", "1")
	tempDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(origWd)

	// Pre-existing CLAUDE.md that already contains the guidelines.
	existing := "# My Project\n\n" + config.KarpathyGuidelines
	os.WriteFile(filepath.Join(tempDir, "CLAUDE.md"), []byte(existing), 0644)

	initCmd := NewInitCmd()
	initCmd.SetArgs([]string{})
	if err := initCmd.Execute(); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(tempDir, "CLAUDE.md"))
	if string(data) != existing {
		t.Errorf("CLAUDE.md should be unchanged when guidelines already present, got %q", string(data))
	}
}

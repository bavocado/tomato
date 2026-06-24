package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitCommand(t *testing.T) {
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
}
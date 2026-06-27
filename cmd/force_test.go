package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestForceFlagExists(t *testing.T) {
	root := NewRootCmd("0.1.0")
	for _, name := range []string{"spec", "design", "impl", "review", "test"} {
		cmd, _, err := root.Find([]string{name})
		if err != nil {
			t.Fatalf("command %s not found: %v", name, err)
		}
		flag := cmd.Flags().Lookup("force")
		if flag == nil {
			t.Errorf("command %s should have --force flag", name)
		}
	}
}

func TestSpecRefusesWithoutForce(t *testing.T) {
	tempDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(origWd)

	yaml := `
models:
  default: openai/gpt-5
workflows:
  default:
    steps: [spec]
`
	os.WriteFile(filepath.Join(tempDir, "tomato.yaml"), []byte(yaml), 0644)
	featureDir := filepath.Join(tempDir, "docs", "specs", "current-feature")
	os.MkdirAll(featureDir, 0755)

	// Create an existing prd.md
	existingPath := filepath.Join(featureDir, "prd.md")
	os.WriteFile(existingPath, []byte("# Existing PRD"), 0644)

	// Run spec WITHOUT --force — should refuse
	cmd := NewSpecCmd()
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when running spec without --force and prd.md exists")
	}

	// The existing file should be unchanged
	data, _ := os.ReadFile(existingPath)
	if string(data) != "# Existing PRD" {
		t.Errorf("existing file was modified without --force: %q", string(data))
	}
}

func TestSpecForceFlagValue(t *testing.T) {
	cmd := &cobra.Command{Use: "spec"}
	addForceFlag(cmd)

	force, err := cmd.Flags().GetBool("force")
	if err != nil {
		t.Fatal(err)
	}
	if force {
		t.Error("expected force to default to false")
	}

	cmd.SetArgs([]string{"--force"})
	cmd.Execute()
	force, _ = cmd.Flags().GetBool("force")
	if !force {
		t.Error("expected force to be true after --force flag")
	}
}

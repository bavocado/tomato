package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bavocado/tomato/pkg/model"
	"github.com/bavocado/tomato/pkg/steps"
)

func TestDecomposeInputAndApplyMutuallyExclusive(t *testing.T) {
	// Create a minimal temp repo with tomato.yaml so config loading succeeds
	dir := t.TempDir()
	tomatoYaml := `models:
  default: claude/sonnet-4-20250514
workflows:
  default:
    steps: [spec, design]
`
	os.WriteFile(filepath.Join(dir, "tomato.yaml"), []byte(tomatoYaml), 0644)

	// Create minimal docs/specs directory structure
	os.MkdirAll(filepath.Join(dir, "docs", "specs"), 0755)

	c := NewDecomposeCmd()
	c.SetArgs([]string{"--input", "x.md", "--apply"})
	c.SetOut(&bytes.Buffer{})
	c.SetErr(&bytes.Buffer{})

	// Change to temp dir so withFeatureAndModel finds tomato.yaml
	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("expected mutually-exclusive error, got %v", err)
	}
}

func TestRunDecomposeGenerateWritesSourceDesign(t *testing.T) {
	dir := t.TempDir()
	featureDir := filepath.Join(dir, "docs", "specs", "feat")
	os.MkdirAll(featureDir, 0755)

	inputPath := filepath.Join(dir, "design.md")
	inputContent := "# Design\n\nThis is a test design."
	os.WriteFile(inputPath, []byte(inputContent), 0644)

	cfg := &steps.StepConfig{
		RepoDir:    dir,
		FeatureDir: featureDir,
		Feature:    "feat",
	}

	// Save the original "decompose" step handler and restore it after test
	originalStep, err := steps.Get("decompose")
	if err == nil {
		t.Cleanup(func() {
			steps.Register("decompose", originalStep)
		})
	}

	// Register a mock "decompose" step that returns success
	steps.Register("decompose", func(cfg *steps.StepConfig, args []string) *model.StepResult {
		return &model.StepResult{
			StepName: "decompose",
			Success:  true,
		}
	})

	err = runDecomposeGenerate(cfg, inputPath, false)
	if err != nil {
		t.Fatalf("runDecomposeGenerate failed: %v", err)
	}

	// Verify source-design.md was written
	sourceDesignPath := filepath.Join(featureDir, "source-design.md")
	data, err := os.ReadFile(sourceDesignPath)
	if err != nil {
		t.Fatalf("source-design.md not written: %v", err)
	}

	if string(data) != inputContent {
		t.Fatalf("source-design.md content mismatch: got %q, want %q", string(data), inputContent)
	}
}

func TestRunDecomposeApplyHappyPath(t *testing.T) {
	dir := t.TempDir()
	featureDir := filepath.Join(dir, "docs", "specs", "feat")
	os.MkdirAll(featureDir, 0755)

	// Create a valid decomposition.md with YAML block
	decompContent := "# Decomposition\n\n" +
		"```yaml\n" +
		"source: source-design.md\n" +
		"total_features: 2\n" +
		"dag_check: passed\n" +
		"critical_path: [F-001, F-002]\n" +
		"spikes: []\n" +
		"features:\n" +
		"  - id: F-001\n" +
		"    title: Sub 1\n" +
		"    goal: First sub-feature\n" +
		"    user_value: Value\n" +
		"    slice_type: workflow\n" +
		"    c4_level: container\n" +
		"    priority: must\n" +
		"    depends_on: []\n" +
		"    acceptance_criteria:\n" +
		"      - Criterion 1\n" +
		"    complexity: M\n" +
		"    is_spike: false\n" +
		"    out_of_scope: []\n" +
		"  - id: F-002\n" +
		"    title: Sub 2\n" +
		"    goal: Second sub-feature\n" +
		"    user_value: Value\n" +
		"    slice_type: workflow\n" +
		"    c4_level: container\n" +
		"    priority: should\n" +
		"    depends_on:\n" +
		"      - F-001\n" +
		"    acceptance_criteria:\n" +
		"      - Criterion 2\n" +
		"    complexity: S\n" +
		"    is_spike: false\n" +
		"    out_of_scope: []\n" +
		"```\n"
	decompPath := filepath.Join(featureDir, "decomposition.md")
	os.WriteFile(decompPath, []byte(decompContent), 0644)

	// Create source-design.md
	sourceDesignContent := "# Parent design\n\nThis is the parent design."
	sourceDesignPath := filepath.Join(featureDir, "source-design.md")
	os.WriteFile(sourceDesignPath, []byte(sourceDesignContent), 0644)

	cfg := &steps.StepConfig{
		RepoDir:    dir,
		FeatureDir: featureDir,
		Feature:    "feat",
	}

	err := runDecomposeApply(cfg, false)
	if err != nil {
		t.Fatalf("runDecomposeApply failed: %v", err)
	}

	// Verify sub-feature directories were created
	f001Dir := filepath.Join(dir, "docs", "specs", "feat-f001")
	f002Dir := filepath.Join(dir, "docs", "specs", "feat-f002")

	if _, err := os.Stat(f001Dir); os.IsNotExist(err) {
		t.Fatalf("sub-feature directory %s not created", f001Dir)
	}
	if _, err := os.Stat(f002Dir); os.IsNotExist(err) {
		t.Fatalf("sub-feature directory %s not created", f002Dir)
	}

	// Verify idea.md was created in each sub-feature
	for _, sfDir := range []string{f001Dir, f002Dir} {
		ideaPath := filepath.Join(sfDir, "idea.md")
		if _, err := os.Stat(ideaPath); os.IsNotExist(err) {
			t.Fatalf("idea.md not created in %s", sfDir)
		}
	}

	// Verify orchestration.yaml was created
	orchPath := filepath.Join(featureDir, "orchestration.yaml")
	if _, err := os.Stat(orchPath); os.IsNotExist(err) {
		t.Fatalf("orchestration.yaml not created in %s", featureDir)
	}
}

func TestRunDecomposeGenerateExistingNoForce(t *testing.T) {
	dir := t.TempDir()
	featureDir := filepath.Join(dir, "docs", "specs", "feat")
	os.MkdirAll(featureDir, 0755)

	// Create existing decomposition.md
	existingDecompPath := filepath.Join(featureDir, "decomposition.md")
	os.WriteFile(existingDecompPath, []byte("# Existing decomposition"), 0644)

	inputPath := filepath.Join(dir, "design.md")
	os.WriteFile(inputPath, []byte("# New design"), 0644)

	cfg := &steps.StepConfig{
		RepoDir:    dir,
		FeatureDir: featureDir,
		Feature:    "feat",
	}

	err := runDecomposeGenerate(cfg, inputPath, false)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected 'already exists' error, got %v", err)
	}
}

func TestDecomposeRequiresInputOrApply(t *testing.T) {
	dir := t.TempDir()
	tomatoYaml := `models:
  default: claude/sonnet-4-20250514
workflows:
  default:
    steps: [spec, design]
`
	if err := os.WriteFile(filepath.Join(dir, "tomato.yaml"), []byte(tomatoYaml), 0644); err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(dir, "docs", "specs"), 0755)

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(oldWd) })

	c := NewDecomposeCmd()
	c.SetArgs([]string{}) // neither --input nor --apply
	buf := new(bytes.Buffer)
	c.SetOut(buf)
	c.SetErr(buf)
	err = c.Execute()
	if err == nil || !strings.Contains(err.Error(), "usage") {
		t.Fatalf("expected usage error when no --input/--apply, got %v", err)
	}
}

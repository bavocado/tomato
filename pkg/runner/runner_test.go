package runner

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRunStepProducesArtifacts(t *testing.T) {
	dir := t.TempDir()

	// Create input file
	os.MkdirAll(filepath.Join(dir, "docs", "specs", "my-feature"), 0755)
	os.WriteFile(filepath.Join(dir, "docs", "specs", "my-feature", "prd.md"), []byte("Build a todo app"), 0644)

	// Mock LLM that returns a fixed response
	mockLLM := func(messages []Message, onChunk func(string)) error {
		onChunk("# Architecture\n## Components\n- Frontend: React\n- Backend: Go\n---\n# UI Spec\n...\n---\n# Implementation\n...")
		return nil
	}

result := Execute(
			"design",
			"Design the architecture for: {{.prd.md}}",
			[]string{filepath.Join("docs", "specs", "my-feature", "prd.md")},
			[]string{
				filepath.Join("docs", "specs", "my-feature", "architecture.md"),
				filepath.Join("docs", "specs", "my-feature", "ui-spec.md"),
				filepath.Join("docs", "specs", "my-feature", "implementation.md"),
			},
			dir,
			"gpt-5",
			mockLLM,
			"v1",
			nil,
		)

	if !result.Success {
		t.Fatalf("step failed: %s", result.Error)
	}

	// Verify output files exist
	for _, out := range []string{"architecture.md", "ui-spec.md", "implementation.md"} {
		path := filepath.Join(dir, "docs", "specs", "my-feature", out)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("output %s was not created", out)
		}
	}

	// Verify run log was written
	metaPath := filepath.Join(dir, ".tomato", "runs", result.RunID, "meta.json")
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		t.Errorf("run log meta.json not found at %s", metaPath)
	}
}

func TestRunStepWithMissingInput(t *testing.T) {
	dir := t.TempDir()

	mockLLM := func(messages []Message, onChunk func(string)) error {
		onChunk("Some output")
		return nil
	}

result := Execute(
			"spec",
			"Write PRD for {{.input.md}}",
			[]string{filepath.Join(dir, "nonexistent.md")},
			[]string{filepath.Join(dir, "prd.md")},
			dir,
			"gpt-5",
			mockLLM,
			"v1",
			nil,
		)

	if !result.Success {
		t.Fatalf("step should handle missing input: %s", result.Error)
	}
}

func TestRunStepTimeAndModel(t *testing.T) {
	dir := t.TempDir()

	mockLLM := func(messages []Message, onChunk func(string)) error {
		time.Sleep(10 * time.Millisecond)
		onChunk("output")
		return nil
	}

result := Execute(
			"design",
			"test",
			nil,
			[]string{filepath.Join(dir, "out.md")},
			dir,
			"glm/glm-5.2",
			mockLLM,
			"v1",
			nil,
		)

	if !result.Success {
		t.Fatalf("step failed: %s", result.Error)
	}

	if result.ModelUsed != "glm/glm-5.2" {
		t.Errorf("expected model glm/glm-5.2, got %s", result.ModelUsed)
	}

	if result.DurationMs < 5 {
		t.Errorf("expected duration >= 10ms (mock sleep), got %dms", result.DurationMs)
	}
}
package customstep

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bavocado/tomato/pkg/config"
	"github.com/bavocado/tomato/pkg/runner"
)

// TestRunWritesOutputFromPromptAndInputs drives the real runner.Execute path:
// the prompt is read, input globs are expanded, the (injected) LLM stream is
// called, and the response is written to the declared output file.
func TestRunWritesOutputFromPromptAndInputs(t *testing.T) {
	dir := t.TempDir()

	// Prompt template references the input file by its basename token;
	// runner.Execute substitutes {{.app.ts}} with the file's contents.
	if err := os.MkdirAll(filepath.Join(dir, "prompts"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "prompts", "echo.md"), []byte("input: {{.app.ts}}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "app.ts"), []byte("export const x = 1;"), 0644); err != nil {
		t.Fatal(err)
	}

	def := config.CustomStepDef{
		Prompt:  "prompts/echo.md",
		Inputs:  []string{"src/*.ts"},
		Outputs: []string{"out/result.txt"},
		Model:   "glm/glm-5.2",
	}

	cfg := Config{
		RepoDir:   dir,
		ModelName: "glm/glm-5.2",
		LLMStream: func(messages []runner.Message, onChunk func(string)) error {
			onChunk("generated artifact")
			return nil
		},
	}

	result := Run("echo", def, cfg)
	if !result.Success {
		t.Fatalf("expected custom step to succeed, got error: %s", result.Error)
	}

	out, err := os.ReadFile(filepath.Join(dir, "out", "result.txt"))
	if err != nil {
		t.Fatalf("expected output file written: %v", err)
	}
	if string(out) != "generated artifact" {
		t.Errorf("expected output 'generated artifact', got %q", string(out))
	}
}

// TestRunFailsWhenPromptMissing verifies a missing prompt file yields a failed
// result with a descriptive error rather than a panic.
func TestRunFailsWhenPromptMissing(t *testing.T) {
	dir := t.TempDir()
	def := config.CustomStepDef{Prompt: "prompts/missing.md", Outputs: []string{"out/x.txt"}}
	cfg := Config{
		RepoDir:   dir,
		ModelName: "glm/glm-5.2",
		LLMStream: func([]runner.Message, func(string)) error { return nil },
	}

	result := Run("missing", def, cfg)
	if result.Success {
		t.Fatal("expected failure when prompt file is missing")
	}
	if result.Error == "" {
		t.Error("expected non-empty error message")
	}
}

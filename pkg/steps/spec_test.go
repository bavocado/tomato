package steps

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bavocado/tomato/pkg/runner"
)

func TestSpecPrompt(t *testing.T) {
	assertContainsAll(t, SpecPrompt, []string{
		"PRD",
		"Functional Requirements",
		"Acceptance Criteria",
		"Given/When/Then",
		"Edge Cases",
		"Non-Goals",
		"idea.txt",
	})
}

func TestDesignPrompt(t *testing.T) {
	assertContainsAll(t, DesignPrompt, []string{
		"architecture.md",
		"ui-spec.md",
		"implementation.md",
		"Mermaid",
		"Error Handling Strategy",
		"Testing Strategy",
		"File and Package Plan",
	})
}

func TestImplPrompt(t *testing.T) {
	assertContainsAll(t, ImplPrompt, []string{
		"Implementation Output",
		"Files to Change",
		"Patch Plan",
		"Commands to Run",
		"Risk Notes",
		"Post-Implementation Architecture Update",
	})
}

func TestReviewPrompt(t *testing.T) {
	assertContainsAll(t, ReviewPrompt, []string{
		"blocking",
		"major",
		"minor",
		"has_blocking",
		"REQUEST_CHANGES",
		"APPROVE",
		"suggestion",
	})
}

func TestTestPrompt(t *testing.T) {
	assertContainsAll(t, TestPrompt, []string{
		"Test Plan",
		"Unit Tests",
		"Integration Tests",
		"Error and Edge Cases",
		"Regression Tests",
		"Commands",
		"Coverage Gaps",
	})
}

func TestRewriteArchPrompt(t *testing.T) {
	assertContainsAll(t, RewriteArchPrompt, []string{
		"architecture.md",
		"impl-output.md",
		"{{.diff}}",
		"Divergences from Original Design",
		"REAL architecture",
	})
}

func TestStepRegistration(t *testing.T) {
	for _, name := range []string{"spec", "design", "impl", "review", "test", "fast"} {
		fn, err := Get(name)
		if err != nil {
			t.Fatalf("expected %s step to be registered: %v", name, err)
		}
		if fn == nil {
			t.Fatalf("expected non-nil step function for %s", name)
		}
	}
}

func TestRunFastBypassesResponseCache(t *testing.T) {
	dir := t.TempDir()
	featureDir := filepath.Join(dir, "docs", "specs", "feat")
	if err := os.MkdirAll(featureDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(featureDir, "idea.txt"), []byte("do it"), 0644); err != nil {
		t.Fatal(err)
	}
	calls := 0
	cfg := &StepConfig{
		RepoDir:    dir,
		FeatureDir: featureDir,
		Feature:    "feat",
		ModelName:  "glm/glm-5.2",
		LLMStream: func(messages []runner.Message, onChunk func(string)) error {
			calls++
			onChunk("ok")
			return nil
		},
	}

	for i := 0; i < 2; i++ {
		if res := runFast(cfg, nil); !res.Success {
			t.Fatalf("runFast failed: %s", res.Error)
		}
	}
	if calls != 2 {
		t.Fatalf("fast should call LLM every run, got %d calls", calls)
	}
}

func TestUnknownStep(t *testing.T) {
	_, err := Get("nonexistent-step")
	if err == nil {
		t.Error("expected error for unknown step")
	}
}

func assertContainsAll(t *testing.T, prompt string, required []string) {
	t.Helper()
	if strings.TrimSpace(prompt) == "" {
		t.Fatal("prompt should not be empty")
	}
	for _, substr := range required {
		if !strings.Contains(prompt, substr) {
			t.Errorf("prompt should contain %q", substr)
		}
	}
}

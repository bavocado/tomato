package steps

import (
	"strings"
	"testing"
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

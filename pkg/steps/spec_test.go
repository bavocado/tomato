package steps

import (
	"strings"
	"testing"
)

func TestSpecPrompt(t *testing.T) {
	if len(SpecPrompt) == 0 {
		t.Error("spec prompt should not be empty")
	}
	if !contains(SpecPrompt, "PRD") {
		t.Error("spec prompt should mention PRD")
	}
}

func TestStepRegistration(t *testing.T) {
	fn, err := Get("spec")
	if err != nil {
		t.Fatal(err)
	}
	if fn == nil {
		t.Error("expected non-nil step function")
	}
}

func TestUnknownStep(t *testing.T) {
	_, err := Get("nonexistent-step")
	if err == nil {
		t.Error("expected error for unknown step")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && strings.Contains(s, substr)
}
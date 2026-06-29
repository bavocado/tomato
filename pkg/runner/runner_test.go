package runner

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bavocado/tomato/pkg/budget"
	"github.com/bavocado/tomato/pkg/model"
)

func TestSplitArtifactsNoMarkers(t *testing.T) {
	text := "Just a plain response with no markers."
	parts := splitArtifacts(text)

	if len(parts) != 1 {
		t.Errorf("expected 1 part, got %d", len(parts))
	}
	// When no markers, all output files get the same content
	if parts[""] != text {
		t.Errorf("expected default part to be the full text")
	}
}

func TestSplitArtifactsWithMarkers(t *testing.T) {
	text := `# Header

---TOMATO-ARTIFACT: architecture.md---
# Architecture
content for arch

---TOMATO-ARTIFACT: ui-spec.md---
# UI Spec
content for ui

---TOMATO-ARTIFACT: implementation.md---
# Implementation
content for impl
`
	parts := splitArtifacts(text)

	if len(parts) != 3 {
		t.Errorf("expected 3 parts, got %d", len(parts))
	}
	if !strings.Contains(parts["architecture.md"], "content for arch") {
		t.Errorf("architecture.md part wrong: %q", parts["architecture.md"])
	}
	if !strings.Contains(parts["ui-spec.md"], "content for ui") {
		t.Errorf("ui-spec.md part wrong: %q", parts["ui-spec.md"])
	}
	if !strings.Contains(parts["implementation.md"], "content for impl") {
		t.Errorf("implementation.md part wrong: %q", parts["implementation.md"])
	}
}

func TestSplitArtifactsPartialMarkers(t *testing.T) {
	text := `Preamble text

---TOMATO-ARTIFACT: architecture.md---
# Architecture
`
	parts := splitArtifacts(text)

	// Should have 1 named part
	if _, ok := parts["architecture.md"]; !ok {
		t.Error("expected architecture.md part")
	}
}

func TestExecuteWritesSplitArtifacts(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "docs", "specs", "f"), 0755)

	mockLLM := func(messages []Message, onChunk func(string)) error {
		onChunk(`---TOMATO-ARTIFACT: architecture.md---
# Architecture
arch content

---TOMATO-ARTIFACT: ui-spec.md---
# UI
ui content

---TOMATO-ARTIFACT: implementation.md---
# Impl
impl content
`)
		return nil
	}

	featureDir := filepath.Join("docs", "specs", "f")
	result := Execute(
		"design",
		"test {{.prd.md}}",
		[]string{filepath.Join(featureDir, "prd.md")},
		[]string{
			filepath.Join(featureDir, "architecture.md"),
			filepath.Join(featureDir, "ui-spec.md"),
			filepath.Join(featureDir, "implementation.md"),
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

	arch, _ := os.ReadFile(filepath.Join(dir, featureDir, "architecture.md"))
	ui, _ := os.ReadFile(filepath.Join(dir, featureDir, "ui-spec.md"))
	impl, _ := os.ReadFile(filepath.Join(dir, featureDir, "implementation.md"))

	archStr := string(arch)
	uiStr := string(ui)
	implStr := string(impl)

	if !strings.Contains(archStr, "arch content") || strings.Contains(archStr, "ui content") {
		t.Errorf("architecture.md should contain only arch content: %q", archStr)
	}
	if !strings.Contains(uiStr, "ui content") || strings.Contains(uiStr, "impl content") {
		t.Errorf("ui-spec.md should contain only ui content: %q", uiStr)
	}
	if !strings.Contains(implStr, "impl content") {
		t.Errorf("implementation.md should contain impl content: %q", implStr)
	}
}

func TestExecuteSingleOutputNoMarkers(t *testing.T) {
	dir := t.TempDir()

	mockLLM := func(messages []Message, onChunk func(string)) error {
		onChunk("plain output without markers")
		return nil
	}

	result := Execute(
		"spec",
		"test",
		nil,
		[]string{filepath.Join(dir, "out.md")},
		dir,
		"gpt-5",
		mockLLM,
		"v1",
		nil,
	)

	if !result.Success {
		t.Fatalf("step failed: %s", result.Error)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "out.md"))
	if string(data) != "plain output without markers" {
		t.Errorf("expected plain output, got %q", string(data))
	}
}

func TestExecuteRecordsTokenEstimates(t *testing.T) {
	dir := t.TempDir()
	mockLLM := func(messages []Message, onChunk func(string)) error {
		onChunk("hello world this is a test response")
		return nil
	}

	result := Execute(
		"spec",
		"test prompt",
		nil,
		[]string{filepath.Join(dir, "out.md")},
		dir,
		"gpt-5",
		mockLLM,
		"v1",
		nil,
	)

	if !result.Success {
		t.Fatalf("step failed: %s", result.Error)
	}

	if result.TokensOut == 0 {
		t.Error("expected non-zero tokens out")
	}
}

func TestExecuteWithDuration(t *testing.T) {
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
		"gpt-5",
		mockLLM,
		"v1",
		nil,
	)

	if result.DurationMs < 5 {
		t.Errorf("expected duration >= 10ms, got %d", result.DurationMs)
	}
}

// TestExecuteInjectsInputContentViaAbsolutePath is the regression test for the
// P0 data-flow bug: production code passes ABSOLUTE input paths
// (cfg.FeatureDir = filepath.Join(repoDir, "docs/specs/<feature>")) into
// Execute, and prompts reference inputs via dotted tokens like {{.prd.md}}.
//
// Two bugs combined to make every step receive empty prompts:
//  1. buildMessages did filepath.Join(repoDir, absPath), producing a bogus
//     doubled path that never existed → every input read as empty.
//  2. Go's text/template parses {{.prd.md}} as a field chain (prd→md), which
//     never matches a map key "prd.md" → renders <no value> even when the
//     file WAS read.
//
// This test mirrors the production path layout and asserts the file's real
// content reaches the LLM prompt. The earlier suite missed this because its
// mock LLMs ignored prompt content and used relative paths.
func TestExecuteInjectsInputContentViaAbsolutePath(t *testing.T) {
	repoDir := t.TempDir()
	// Absolute feature dir, exactly as the engine/cmd build it in production.
	featureDir := filepath.Join(repoDir, "docs", "specs", "current-feature")
	os.MkdirAll(featureDir, 0755)

	// Write a real input with distinctive content.
	const prdContent = "UNIQUE-PRD-MARKER-42"
	os.WriteFile(filepath.Join(featureDir, "prd.md"), []byte(prdContent), 0644)

	var capturedPrompt string
	mockLLM := func(messages []Message, onChunk func(string)) error {
		for _, m := range messages {
			capturedPrompt += m.Content
		}
		onChunk("# Design\n")
		return nil
	}

	result := Execute(
		"design",
		"PRD follows:\n{{.prd.md}}\nend",
		[]string{filepath.Join(featureDir, "prd.md")}, // absolute path, as in production
		[]string{filepath.Join(featureDir, "architecture.md")},
		repoDir,
		"gpt-5",
		mockLLM,
		"v1",
		nil,
	)

	if !result.Success {
		t.Fatalf("step failed: %s", result.Error)
	}
	if !strings.Contains(capturedPrompt, prdContent) {
		t.Errorf("input content was NOT injected into the prompt.\nprompt: %q", capturedPrompt)
	}
	if strings.Contains(capturedPrompt, "<no value>") {
		t.Errorf("prompt contained <no value> (template token not resolved): %q", capturedPrompt)
	}
	if strings.Contains(capturedPrompt, "{{.prd.md}}") {
		t.Errorf("prompt still contained the unresolved token {{.prd.md}}: %q", capturedPrompt)
	}
}

// TestExecuteInjectsMissingInputAsEmpty verifies that a referenced-but-absent
// input file resolves to empty rather than leaving a literal token or erroring
// (the spec step runs before idea.txt may exist).
func TestExecuteInjectsMissingInputAsEmpty(t *testing.T) {
	repoDir := t.TempDir()
	featureDir := filepath.Join(repoDir, "docs", "specs", "f")
	os.MkdirAll(featureDir, 0755)

	var capturedPrompt string
	mockLLM := func(messages []Message, onChunk func(string)) error {
		capturedPrompt = messages[1].Content
		onChunk("out")
		return nil
	}

	result := Execute(
		"spec",
		"idea: [{{.idea.txt}}]",
		[]string{filepath.Join(featureDir, "idea.txt")}, // does not exist
		[]string{filepath.Join(featureDir, "prd.md")},
		repoDir,
		"gpt-5",
		mockLLM,
		"v1",
		nil,
	)

	if !result.Success {
		t.Fatalf("step failed: %s", result.Error)
	}
	if strings.Contains(capturedPrompt, "{{.idea.txt}}") {
		t.Errorf("missing input left an unresolved token: %q", capturedPrompt)
	}
}

// runWithBudget runs an Execute with a per-step budget that the prompt exceeds,
// under the given on_exceed policy. The LLM is only invoked when the step
// proceeds.
func runWithBudget(t *testing.T, onExceed string) *model.StepResult {
	t.Helper()
	repoDir := t.TempDir()
	tracker := budget.NewTracker()
	tracker.InitFromConfig("balanced", map[string]int{"spec": 10}, 0, onExceed, "")

	llmCalled := false
	mockLLM := func(messages []Message, onChunk func(string)) error {
		llmCalled = true
		onChunk("out")
		return nil
	}

	res := Execute(
		"spec",
		strings.Repeat("x", 1000), // ~250 estimated tokens, far over the 10 budget
		nil,
		[]string{filepath.Join(repoDir, "out.md")},
		repoDir,
		"gpt-5",
		mockLLM,
		"v1",
		tracker,
	)
	res.ModelUsed = strconv.FormatBool(llmCalled) // repurpose to report invocation
	return res
}

// TestBudgetWarnProceeds verifies on_exceed=warn lets the step proceed (the
// design default is "warn but continue"). The old code hard-failed here,
// reversing the documented behavior.
func TestBudgetWarnProceeds(t *testing.T) {
	res := runWithBudget(t, "warn")
	if !res.Success {
		t.Errorf("warn should proceed, got failure: %s", res.Error)
	}
	if res.ModelUsed != "true" {
		t.Error("warn should still invoke the LLM")
	}
}

// TestBudgetFailAborts verifies on_exceed=fail aborts the step before calling
// the LLM.
func TestBudgetFailAborts(t *testing.T) {
	res := runWithBudget(t, "fail")
	if res.Success {
		t.Error("fail should abort the step, but it succeeded")
	}
	if res.ModelUsed != "false" {
		t.Error("fail should NOT invoke the LLM")
	}
}

// TestBudgetDegradeProceeds verifies on_exceed=degrade proceeds in v1 (the
// automatic model downgrade itself is v1.x, per §2.9.7).
func TestBudgetDegradeProceeds(t *testing.T) {
	res := runWithBudget(t, "degrade")
	if !res.Success {
		t.Errorf("degrade should proceed in v1, got failure: %s", res.Error)
	}
	if res.ModelUsed != "true" {
		t.Error("degrade should still invoke the LLM in v1")
	}
}

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

func TestExecuteMultiOutputWithoutMarkersWarnsAndProceeds(t *testing.T) {
	dir := t.TempDir()
	featureDir := filepath.Join(dir, "docs", "specs", "f")
	os.MkdirAll(featureDir, 0755)

	mockLLM := func(messages []Message, onChunk func(string)) error {
		onChunk("plain output without artifact markers")
		return nil
	}

	result := Execute(
		"design",
		"test",
		nil,
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
		t.Fatalf("expected success for multi-output response without markers (should warn and proceed), got: %s", result.Error)
	}
	// All output files should be written with the full response.
	for _, name := range []string{"architecture.md", "ui-spec.md", "implementation.md"} {
		data, err := os.ReadFile(filepath.Join(featureDir, name))
		if err != nil {
			t.Fatalf("expected %s to be written: %v", name, err)
		}
		if string(data) != "plain output without artifact markers" {
			t.Errorf("%s: expected full response, got %q", name, string(data))
		}
	}
	// Verify it's not cached — a subsequent run should still call the LLM.
	if result.CacheHit {
		t.Error("marker-less multi-output response should not be cached")
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

func TestExecuteAddsSubagentInstruction(t *testing.T) {
	dir := t.TempDir()
	var systemPrompt string
	mockLLM := func(messages []Message, onChunk func(string)) error {
		systemPrompt = messages[0].Content
		onChunk("out")
		return nil
	}

	result := Execute(
		"design",
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
	if !strings.Contains(systemPrompt, "Task subagent") {
		t.Fatalf("system prompt should instruct Claude to use a Task subagent, got %q", systemPrompt)
	}
	if !strings.Contains(systemPrompt, "design") {
		t.Fatalf("system prompt should name the step, got %q", systemPrompt)
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

// TestExecuteSnapshotsArtifacts verifies Execute writes a stable copy of each
// output artifact into .tomato/runs/<run-id>/artifacts/<basename> so that
// `tomato history diff` can compare two runs even after the working-tree
// outputs change.
func TestExecuteSnapshotsArtifacts(t *testing.T) {
	dir := t.TempDir()

	mockLLM := func(messages []Message, onChunk func(string)) error {
		onChunk("artifact snapshot content")
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

	// The snapshot lives under .tomato/runs/<run-id>/artifacts/out.md. The
	// run-id is generated, so resolve it by glob rather than guessing.
	matches, err := filepath.Glob(filepath.Join(dir, ".tomato", "runs", "*", "artifacts", "out.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected exactly 1 snapshot, got %d: %v", len(matches), matches)
	}

	snapshot, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	if string(snapshot) != "artifact snapshot content" {
		t.Errorf("expected snapshot to mirror the output, got %q", string(snapshot))
	}
}

// TestExecuteCacheHitSkipsLLM verifies that a second Execute with the same
// prompt + model + promptVersion is served from the local cache without
// invoking the LLM again.
func TestExecuteCacheHitSkipsLLM(t *testing.T) {
	dir := t.TempDir()

	callCount := 0
	mockLLM := func(messages []Message, onChunk func(string)) error {
		callCount++
		onChunk("cached response")
		return nil
	}

	inFile := filepath.Join(dir, "in.txt")
	outFile := filepath.Join(dir, "out.md")

	// First call: cold miss, LLM invoked, response cached.
	os.WriteFile(inFile, []byte("hello"), 0644)
	first := Execute("spec", "prompt {{.in.txt}}", []string{inFile}, []string{outFile}, dir, "glm/glm-5.2", mockLLM, "v1", nil)
	if !first.Success {
		t.Fatalf("first call failed: %s", first.Error)
	}
	if callCount != 1 {
		t.Fatalf("expected LLM invoked once on cold miss, got %d", callCount)
	}
	if first.CacheHit {
		t.Error("first call should not be a cache hit")
	}

	// Second call: same prompt + model + version → cache hit, LLM NOT invoked.
	second := Execute("spec", "prompt {{.in.txt}}", []string{inFile}, []string{outFile}, dir, "glm/glm-5.2", mockLLM, "v1", nil)
	if !second.Success {
		t.Fatalf("second call failed: %s", second.Error)
	}
	if callCount != 1 {
		t.Errorf("expected LLM NOT invoked on cache hit, call count=%d", callCount)
	}
	if !second.CacheHit {
		t.Error("second call should be a cache hit")
	}

	// Cached content must match the original response.
	data, _ := os.ReadFile(outFile)
	if string(data) != "cached response" {
		t.Errorf("expected cached response, got %q", string(data))
	}
}

func TestExecuteIgnoresInvalidMultiOutputCache(t *testing.T) {
	dir := t.TempDir()
	featureDir := filepath.Join(dir, "docs", "specs", "f")
	os.MkdirAll(featureDir, 0755)

	callCount := 0
	mockLLM := func(messages []Message, onChunk func(string)) error {
		callCount++
		if callCount == 1 {
			onChunk("bad cached response")
			return nil
		}
		onChunk(`---TOMATO-ARTIFACT: architecture.md---
# Architecture

---TOMATO-ARTIFACT: ui-spec.md---
# UI

---TOMATO-ARTIFACT: implementation.md---
# Impl
`)
		return nil
	}

	input := filepath.Join(featureDir, "prd.md")
	outputs := []string{
		filepath.Join(featureDir, "architecture.md"),
		filepath.Join(featureDir, "ui-spec.md"),
		filepath.Join(featureDir, "implementation.md"),
	}
	os.WriteFile(input, []byte("hello"), 0644)

	first := Execute("design", "prompt {{.prd.md}}", []string{input}, outputs, dir, "glm/glm-5.2", mockLLM, "v1", nil)
	if !first.Success {
		t.Fatal("first response without markers should succeed (warn and write full response to each file)")
	}
	if first.CacheHit {
		t.Error("marker-less multi-output response should not be cached")
	}

	second := Execute("design", "prompt {{.prd.md}}", []string{input}, outputs, dir, "glm/glm-5.2", mockLLM, "v1", nil)
	if !second.Success {
		t.Fatalf("second response should succeed: %s", second.Error)
	}
	if second.CacheHit {
		t.Fatal("first response was not cached, so second should be a cache miss")
	}
	if callCount != 2 {
		t.Fatalf("expected LLM called again (first not cached), got %d", callCount)
	}
}

// TestExecuteCacheMissOnDifferentPrompt verifies that changing the prompt
// invalidates the cache and forces a fresh LLM call.
func TestExecuteCacheMissOnDifferentPrompt(t *testing.T) {
	dir := t.TempDir()

	callCount := 0
	mockLLM := func(messages []Message, onChunk func(string)) error {
		callCount++
		onChunk("response")
		return nil
	}

	os.WriteFile(filepath.Join(dir, "in.txt"), []byte("input-a"), 0644)
	Execute("spec", "prompt A {{.in.txt}}", []string{filepath.Join(dir, "in.txt")},
		[]string{filepath.Join(dir, "out.md")}, dir, "glm/glm-5.2", mockLLM, "v1", nil)
	if callCount != 1 {
		t.Fatalf("first call should invoke LLM, got %d", callCount)
	}

	// Different input content → different prompt → cache miss.
	os.WriteFile(filepath.Join(dir, "in.txt"), []byte("input-b"), 0644)
	Execute("spec", "prompt A {{.in.txt}}", []string{filepath.Join(dir, "in.txt")},
		[]string{filepath.Join(dir, "out.md")}, dir, "glm/glm-5.2", mockLLM, "v1", nil)
	if callCount != 2 {
		t.Errorf("different prompt should miss cache and invoke LLM, call count=%d", callCount)
	}
}

// TestExecuteCacheMissOnDifferentModel verifies that switching the model
// invalidates the cache (the cache key includes model_id).
func TestExecuteCacheMissOnDifferentModel(t *testing.T) {
	dir := t.TempDir()

	callCount := 0
	mockLLM := func(messages []Message, onChunk func(string)) error {
		callCount++
		onChunk("response")
		return nil
	}

	os.WriteFile(filepath.Join(dir, "in.txt"), []byte("same input"), 0644)
	common := []string{filepath.Join(dir, "in.txt")}
	out := []string{filepath.Join(dir, "out.md")}

	Execute("spec", "prompt {{.in.txt}}", common, out, dir, "glm/glm-5.2", mockLLM, "v1", nil)
	Execute("spec", "prompt {{.in.txt}}", common, out, dir, "deepseek/deepseek-v4-pro", mockLLM, "v1", nil)
	if callCount != 2 {
		t.Errorf("different model should miss cache, call count=%d", callCount)
	}
}

// TestExecuteFailsOnEmptyResponse verifies that when the LLM returns an empty
// response (0 content), Execute reports a failure rather than writing an empty
// artifact and claiming success. An empty response means the LLM call did not
// produce usable output and downstream steps must not treat it as a valid
// artifact.
func TestExecuteFailsOnEmptyResponse(t *testing.T) {
	dir := t.TempDir()

	mockLLM := func(messages []Message, onChunk func(string)) error {
		// Simulate claude returning an empty text field (system init only).
		return nil
	}

	result := Execute(
		"review",
		"test prompt",
		nil,
		[]string{filepath.Join(dir, "out.md")},
		dir,
		"glm/glm-5.2",
		mockLLM,
		"v1",
		nil,
	)

	if result.Success {
		t.Fatal("expected failure for empty LLM response, got success")
	}
	if result.Error == "" {
		t.Error("expected non-empty error message")
	}
	// The empty artifact must NOT be written.
	if _, err := os.Stat(filepath.Join(dir, "out.md")); err == nil {
		t.Error("empty output file should not be written")
	}
}

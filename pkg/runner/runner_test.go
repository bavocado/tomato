package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
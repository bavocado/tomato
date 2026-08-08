package steps

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bavocado/tomato/pkg/llm"
	"github.com/bavocado/tomato/pkg/runner"
)

// fakeClaude writes the args it was invoked with to argsLog (one line per
// invocation) and emits a fixed stream-json response carrying a session id.
func fakeClaude(t *testing.T, dir, argsLog string) {
	t.Helper()
	fake := filepath.Join(dir, "claude")
	script := "#!/bin/sh\necho \"$@\" >> " + argsLog + "\necho '[{\"type\":\"system\",\"session_id\":\"shared-sess\"},{\"type\":\"text\",\"content\":\"ok\"}]'\n"
	if err := os.WriteFile(fake, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// TestNewLLMStreamSharesSession verifies that when ShareSession is true, the
// second invocation resumes the session saved by the first (passes
// --resume <id> to claude).
func TestNewLLMStreamSharesSession(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".tomato", "runs"), 0755)
	argsLog := filepath.Join(dir, "args.log")
	fakeClaude(t, dir, argsLog)

	cfg := &StepConfig{RepoDir: dir, ModelName: "test/model", ShareSession: true}
	stream := NewLLMStream(cfg)

	if err := stream([]runner.Message{{Role: "user", Content: "step1"}}, func(string) {}); err != nil {
		t.Fatalf("first stream: %v", err)
	}
	if ref := llm.LoadSession(dir); ref.SessionID != "shared-sess" {
		t.Fatalf("expected session saved as shared-sess after step 1, got %q", ref.SessionID)
	}
	if err := stream([]runner.Message{{Role: "user", Content: "step2"}}, func(string) {}); err != nil {
		t.Fatalf("second stream: %v", err)
	}

	data, _ := os.ReadFile(argsLog)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 claude invocations, got %d: %q", len(lines), string(data))
	}
	if strings.Contains(lines[0], "--resume") {
		t.Fatalf("first step must start fresh (no --resume), got %q", lines[0])
	}
	if !strings.Contains(lines[1], "--resume") || !strings.Contains(lines[1], "shared-sess") {
		t.Fatalf("second step must --resume shared-sess, got %q", lines[1])
	}
}

// TestNewLLMStreamSingleShotStartsFresh verifies ShareSession=false does not
// resume a persisted session even if one exists on disk.
func TestNewLLMStreamSingleShotStartsFresh(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".tomato", "runs"), 0755)
	// Pre-seed a session so we can prove it is NOT resumed.
	llm.SaveSession(dir, llm.SessionRef{SessionID: "stale-sess"})
	argsLog := filepath.Join(dir, "args.log")
	fakeClaude(t, dir, argsLog)

	cfg := &StepConfig{RepoDir: dir, ModelName: "test/model", ShareSession: false}
	stream := NewLLMStream(cfg)
	if err := stream([]runner.Message{{Role: "user", Content: "hi"}}, func(string) {}); err != nil {
		t.Fatalf("stream: %v", err)
	}
	data, _ := os.ReadFile(argsLog)
	if strings.Contains(string(data), "--resume") {
		t.Fatalf("single-shot must not --resume, got %q", string(data))
	}
}

package runner

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecuteEmitsProgressLogs(t *testing.T) {
	dir := t.TempDir()

	mockLLM := func(messages []Message, onChunk func(string)) error {
		onChunk("hello progress")
		return nil
	}

	stderr := captureStderr(t, func() {
		result := Execute(
			"spec",
			"test prompt",
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
	})

	for _, want := range []string{
		"[spec] run=",
		"building prompt",
		"model=glm/glm-5.2",
		"calling LLM",
		"writing 1 artifact",
		"tokens in=",
		"run log",
	} {
		if !strings.Contains(stderr, want) {
			t.Errorf("expected stderr to contain %q, got:\n%s", want, stderr)
		}
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w

	fn()

	w.Close()
	os.Stderr = old
	data, _ := io.ReadAll(r)
	r.Close()
	return string(data)
}
package steps

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bavocado/tomato/pkg/llm"
)

func TestNewLLMStreamDoesNotResumeOrPersistClaudeSession(t *testing.T) {
	dir := t.TempDir()
	if err := llm.SaveSession(dir, llm.SessionRef{SessionID: "old-session"}); err != nil {
		t.Fatal(err)
	}

	binDir := t.TempDir()
	argsFile := filepath.Join(dir, "args.txt")
	fakeClaude := filepath.Join(binDir, "claude")
	script := `#!/bin/sh
printf '%s\n' "$@" > "` + argsFile + `"
echo '[{"type":"system","session_id":"new-session"},{"type":"text","content":"ok"}]'
`
	if err := os.WriteFile(fakeClaude, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	stream := NewLLMStream(&StepConfig{
		RepoDir:        dir,
		ModelName:      "deepseek/deepseek-v4-pro",
		AnthropicURL:   "https://example.test",
		AnthropicKey:   "token",
		AnthropicModel: "deepseek-v4-pro",
	})

	var got strings.Builder
	if err := stream(nil, func(chunk string) { got.WriteString(chunk) }); err != nil {
		t.Fatal(err)
	}
	if got.String() != "ok" {
		t.Fatalf("expected ok response, got %q", got.String())
	}
	args, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(args), "--resume") {
		t.Fatalf("claude should not receive --resume, got args:\n%s", string(args))
	}
	if _, err := os.Stat(llm.SessionPath(dir)); !os.IsNotExist(err) {
		t.Fatalf("session file should be removed/not persisted, stat err=%v", err)
	}
}

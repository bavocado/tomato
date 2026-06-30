package llm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestClaudeCLIProviderTimesOut(t *testing.T) {
	dir := t.TempDir()
	fake := filepath.Join(dir, "fake-claude")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nsleep 5\necho done\n"), 0755); err != nil {
		t.Fatal(err)
	}

	p := &ClaudeCLIProvider{
		ModelName: "test-model",
		CLIPath:   fake,
		Timeout:   100 * time.Millisecond,
	}

	start := time.Now()
	err := p.Stream([]Message{{Role: "user", Content: "hello"}}, func(chunk string) {})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout error, got: %v", err)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("expected timeout to return quickly, took %v", elapsed)
	}
}

func TestClaudeCLIProviderPassesAnthropicEnv(t *testing.T) {
	dir := t.TempDir()
	fake := filepath.Join(dir, "fake-claude")
	outFile := filepath.Join(dir, "env.txt")
	script := `#!/bin/sh
printf '%s\n' "$ANTHROPIC_BASE_URL" "$ANTHROPIC_AUTH_TOKEN" "$ANTHROPIC_MODEL" > "` + outFile + `"
echo ok
`
	if err := os.WriteFile(fake, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	p := &ClaudeCLIProvider{
		ModelName:   "model-from-flag",
		BaseURL:     "https://example.test",
		AuthToken:   "token-123",
		ClaudeModel: "model-from-env",
		CLIPath:     fake,
		Timeout:     5 * time.Second,
	}

	var got strings.Builder
	err := p.Stream([]Message{{Role: "user", Content: "hello"}}, func(chunk string) { got.WriteString(chunk) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.String(), "ok") {
		t.Fatalf("expected output ok, got %q", got.String())
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	for _, want := range []string{"https://example.test", "token-123", "model-from-env"} {
		if !strings.Contains(content, want) {
			t.Fatalf("expected env output to contain %q, got %q", want, content)
		}
	}
}

func TestClaudeTimeoutFromEnv(t *testing.T) {
	t.Setenv("TOMATO_CLAUDE_TIMEOUT", "2s")
	if got := claudeTimeout(); got != 2*time.Second {
		t.Fatalf("expected 2s, got %v", got)
	}
}

func TestClaudeTimeoutDefault(t *testing.T) {
	t.Setenv("TOMATO_CLAUDE_TIMEOUT", "")
	if got := claudeTimeout(); got != 30*time.Minute {
		t.Fatalf("expected 30m default, got %v", got)
	}
}
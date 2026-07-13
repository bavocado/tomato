package llm

import (
	"io"
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
	echo '[{"type":"system","session_id":"s-1"},{"type":"text","content":"ok"}]'
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

func TestClaudeCLIProviderStripsAmbientAnthropicEnv(t *testing.T) {
	t.Setenv("ANTHROPIC_DEFAULT_SONNET_MODEL", "glm-5.2")

	dir := t.TempDir()
	fake := filepath.Join(dir, "fake-claude")
	outFile := filepath.Join(dir, "env.txt")
	script := `#!/bin/sh
	env | grep '^ANTHROPIC_' | sort > "` + outFile + `"
	echo '[{"type":"system","session_id":"s-1"},{"type":"text","content":"ok"}]'
`
	if err := os.WriteFile(fake, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	p := &ClaudeCLIProvider{
		ModelName:   "deepseek-v4-pro",
		BaseURL:     "https://api.deepseek.com/anthropic",
		AuthToken:   "deepseek-token",
		ClaudeModel: "deepseek-v4-pro",
		CLIPath:     fake,
		Timeout:     5 * time.Second,
	}

	if err := p.Stream([]Message{{Role: "user", Content: "hello"}}, func(string) {}); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if strings.Contains(content, "ANTHROPIC_DEFAULT_SONNET_MODEL") {
		t.Fatalf("ambient Anthropic default model leaked into child env:\n%s", content)
	}
	for _, want := range []string{
		"ANTHROPIC_BASE_URL=https://api.deepseek.com/anthropic",
		"ANTHROPIC_AUTH_TOKEN=deepseek-token",
		"ANTHROPIC_MODEL=deepseek-v4-pro",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("expected child env to contain %q, got:\n%s", want, content)
		}
	}
}

func TestClaudeCLIProviderPrintsClaudeLogs(t *testing.T) {
	dir := t.TempDir()
	fake := filepath.Join(dir, "fake-claude")
	script := `#!/bin/sh
echo '{"type":"system","session_id":"s-1"}'
echo '{"type":"system","session_id":"s-1","model":null}'
echo '{"type":"system","session_id":"s-1","model":"test-model"}'
echo '{"type":"system","session_id":"s-1","model":null}'
printf '%s\n' '{"type":"assistant","message":{"content":[{"type":"text","text":"TOMATO_STEP: 1/5 inspect - reading relevant code\nignore this line\nTOMATO_STEP: 2/5 plan - choosing smallest change"}]}}'
printf '%s\n' '{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"printf \"one\ntwo\""}}]}}'
printf '%s\n' '{"type":"user","message":{"content":[{"type":"tool_result","content":"one\ntwo\n"}]}}'
echo 'child log line' >&2
echo '{"type":"result","is_error":false,"result":"done","session_id":"s-1","duration_ms":12,"num_turns":1}'
`
	if err := os.WriteFile(fake, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w

	p := &ClaudeCLIProvider{
		ModelName: "test-model",
		CLIPath:   fake,
		Timeout:   5 * time.Second,
	}

	var got strings.Builder
	streamErr := p.Stream([]Message{{Role: "user", Content: "hello"}}, func(chunk string) { got.WriteString(chunk) })
	os.Stderr = oldStderr
	w.Close()
	logData, _ := io.ReadAll(r)
	r.Close()
	if streamErr != nil {
		t.Fatal(streamErr)
	}
	logs := string(logData)
	for _, want := range []string{
		"child log line",
		"[claude]\n  step:\n    1/5 inspect - reading relevant code",
		"[claude]\n  step:\n    2/5 plan - choosing smallest change",
		"[claude]\n  tool: Bash\n  command:\n    printf \"one\n    two\"",
		"[claude]\n  tool_result: ok\n  output:\n    one\n    two\n    ",
		"done turns=1",
	} {
		if !strings.Contains(logs, want) {
			t.Fatalf("expected live logs to contain %q, got:\n%s", want, logs)
		}
	}
	if strings.Contains(logs, "model=<nil>") {
		t.Fatalf("expected nil model system events to be hidden, got:\n%s", logs)
	}
	if strings.Count(logs, "session=s-1") != 1 {
		t.Fatalf("expected session to be printed once, got:\n%s", logs)
	}
	if got.String() != "done" {
		t.Fatalf("expected final result, got %q", got.String())
	}
}

func TestParseClaudeJSONExtractsNestedAssistantText(t *testing.T) {
	data := []byte(`[
		{"type":"system","session_id":"s-1"},
		{"type":"assistant","message":{"content":[{"type":"thinking","thinking":"skip"},{"type":"text","text":"hello"}]}},
		{"type":"result","is_error":false,"result":"hello","session_id":"s-1"}
	]`)

	text, sid, err := parseClaudeJSON(data, "")
	if err != nil {
		t.Fatal(err)
	}
	if text != "hello" {
		t.Fatalf("expected nested assistant text, got %q", text)
	}
	if sid != "s-1" {
		t.Fatalf("expected session id s-1, got %q", sid)
	}
}

func TestParseClaudeJSONExtractsTextFromTruncatedArray(t *testing.T) {
	data := []byte(`[
		{"type":"system","session_id":"s-1"},
		{"type":"assistant","message":{"content":[{"type":"text","text":"hello"}]}}`)

	text, sid, err := parseClaudeJSON(data, "")
	if err != nil {
		t.Fatal(err)
	}
	if text != "hello" {
		t.Fatalf("expected truncated assistant text, got %q", text)
	}
	if sid != "s-1" {
		t.Fatalf("expected session id s-1, got %q", sid)
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

// TestClaudeCLIProviderIgnoresSessionID verifies tomato starts each claude
// invocation fresh even if an old caller still passes SessionID.
func TestClaudeCLIProviderIgnoresSessionID(t *testing.T) {
	dir := t.TempDir()
	fake := filepath.Join(dir, "fake-claude")
	argFile := filepath.Join(dir, "args.txt")
	script := `#!/bin/sh
	printf '%s\n' "$@" > "` + argFile + `"
	echo '[{"type":"system","session_id":"new-session-xyz"},{"type":"text","content":"hello back"}]'
`
	if err := os.WriteFile(fake, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	p := &ClaudeCLIProvider{
		ModelName: "test-model",
		CLIPath:   fake,
		Timeout:   5 * time.Second,
		SessionID: "prior-session-abc",
	}

	var got strings.Builder
	if err := p.Stream([]Message{{Role: "user", Content: "hi"}}, func(chunk string) { got.WriteString(chunk) }); err != nil {
		t.Fatal(err)
	}

	args, _ := os.ReadFile(argFile)
	argsStr := string(args)
	if strings.Contains(argsStr, "--resume") || strings.Contains(argsStr, "prior-session-abc") {
		t.Errorf("expected no session resume args, got %q", argsStr)
	}
	if p.LastSessionID != "new-session-xyz" {
		t.Errorf("expected LastSessionID=new-session-xyz, got %q", p.LastSessionID)
	}
	if !strings.Contains(got.String(), "hello back") {
		t.Errorf("expected text content, got %q", got.String())
	}
}

// TestClaudeCLIProviderNoResumeWhenSessionEmpty verifies that --resume is NOT
// passed when SessionID is empty (first step in a run starts a fresh session).
func TestClaudeCLIProviderNoResumeWhenSessionEmpty(t *testing.T) {
	dir := t.TempDir()
	fake := filepath.Join(dir, "fake-claude")
	argFile := filepath.Join(dir, "args.txt")
	script := `#!/bin/sh
	printf '%s\n' "$@" > "` + argFile + `"
	echo '[{"type":"system","session_id":"first-session"},{"type":"text","content":"ok"}]'
`
	if err := os.WriteFile(fake, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	p := &ClaudeCLIProvider{
		ModelName: "test-model",
		CLIPath:   fake,
		Timeout:   5 * time.Second,
	}

	if err := p.Stream([]Message{{Role: "user", Content: "hi"}}, func(string) {}); err != nil {
		t.Fatal(err)
	}

	args, _ := os.ReadFile(argFile)
	if strings.Contains(string(args), "--resume") {
		t.Errorf("expected no --resume for empty SessionID, got %q", string(args))
	}
	if p.LastSessionID != "first-session" {
		t.Errorf("expected LastSessionID=first-session, got %q", p.LastSessionID)
	}
}

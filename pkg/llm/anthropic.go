package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/bavocado/tomato/pkg/codegraph"
)

// ClaudeCLIProvider runs the `claude` CLI tool to execute AI tasks.
// It forks the `claude` binary with:
//
//	--print --permission-mode auto --effort high --output-format json
//
// The prompt is passed via stdin.
// ANTHROPIC_BASE_URL, ANTHROPIC_AUTH_TOKEN, ANTHROPIC_MODEL are set
// as environment variables from the yaml config (can be overridden by env).
type ClaudeCLIProvider struct {
	ModelName   string
	BaseURL     string
	AuthToken   string
	ClaudeModel string
	CLIPath     string
	Timeout     time.Duration
	// SessionID is kept for compatibility with older callers. It is ignored:
	// tomato starts every claude invocation in a fresh session.
	SessionID string
	// LastSessionID is set by Stream to the session id of this invocation.
	LastSessionID string
	// RepoDir, when set and containing a .codegraph/ index, causes Stream to
	// pass a --mcp-config that mounts the codegraph MCP server so the LLM can
	// query the code knowledge graph during this step.
	RepoDir string
}

func (p *ClaudeCLIProvider) Model() string {
	return p.ModelName
}

func (p *ClaudeCLIProvider) effectiveTimeout() time.Duration {
	if p.Timeout > 0 {
		return p.Timeout
	}
	return claudeTimeout()
}

func claudeTimeout() time.Duration {
	if raw := strings.TrimSpace(os.Getenv("TOMATO_CLAUDE_TIMEOUT")); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil && d > 0 {
			return d
		}
	}
	return 30 * time.Minute
}

func (p *ClaudeCLIProvider) Stream(messages []Message, onChunk func(string)) error {
	prompt := buildClaudePrompt(messages)

	args := []string{
		"--print",
		"--permission-mode", "auto",
		"--effort", "high",
		"--output-format", "stream-json",
		"--verbose",
	}
	if p.ModelName != "" {
		args = append(args, "--model", p.ModelName)
	}
	// When the repo has a codegraph index, mount it as an MCP server so the
	// LLM can call codegraph_explore for surgical code context (fewer file
	// reads, call-path awareness). --strict-mcp-config keeps the agent from
	// accidentally loading unrelated global MCP servers.
	if p.RepoDir != "" {
		if mcpPath, mcpErr := codegraph.WriteMCPConfig(p.RepoDir); mcpErr == nil && mcpPath != "" {
			args = append(args, "--mcp-config", mcpPath, "--strict-mcp-config")
		}
	}

	cliPath := p.CLIPath
	if cliPath == "" {
		cliPath = "claude"
	}
	cmd := exec.Command(cliPath, args...)
	cmd.Stdin = strings.NewReader(prompt)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Build a clean env for the claude subprocess. We strip any pre-existing
	// ANTHROPIC_* vars from the parent environment so they cannot interfere
	// with the provider config tomato resolved from tomato.yaml (e.g. a stale
	// ANTHROPIC_AUTH_TOKEN exported in the user's shell would otherwise cause
	// "Not logged in" errors). Then we inject the yaml values.
	env := os.Environ()
	var clean []string
	for _, e := range env {
		if strings.HasPrefix(e, "ANTHROPIC_") {
			continue
		}
		clean = append(clean, e)
	}
	if p.BaseURL != "" {
		clean = append(clean, "ANTHROPIC_BASE_URL="+p.BaseURL)
	}
	if p.AuthToken != "" {
		clean = append(clean, "ANTHROPIC_AUTH_TOKEN="+p.AuthToken)
		clean = append(clean, "ANTHROPIC_API_KEY="+p.AuthToken)
	}
	if p.ClaudeModel != "" {
		clean = append(clean, "ANTHROPIC_MODEL="+p.ClaudeModel)
	}
	cmd.Env = clean

	fmt.Fprintf(os.Stderr, "[claude] command=%s %s timeout=%s\n", cliPath, strings.Join(args, " "), p.effectiveTimeout())

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	var stderrBuf bytes.Buffer
	stderrDone := make(chan error, 1)
	go func() {
		_, err := io.Copy(io.MultiWriter(os.Stderr, &stderrBuf), stderrPipe)
		stderrDone <- err
	}()

	var stdout bytes.Buffer
	var events []map[string]interface{}
	stdoutDone := make(chan error, 1)
	go func() {
		events, err = readClaudeStream(io.TeeReader(stdoutPipe, &stdout))
		stdoutDone <- err
	}()

	waitDone := make(chan error, 1)
	go func() {
		waitDone <- cmd.Wait()
	}()

	select {
	case waitErr := <-waitDone:
		<-stderrDone
		decodeErr := <-stdoutDone
		if waitErr != nil {
			// Even on non-zero exit, claude may have written a JSON array to
			// stdout containing a "result" entry with is_error=true (e.g. API
			// 429 quota exceeded). Try to parse it for a better error message
			// before falling back to the generic exit error.
			stderrStr := strings.TrimSpace(stderrBuf.String())
			if len(events) > 0 || stdout.Len() > 0 {
				if text, sid, perr := parseClaudeOutput(events, stdout.Bytes(), stderrStr, decodeErr); perr == nil {
					if sid != "" {
						p.LastSessionID = sid
					}
					if text != "" {
						onChunk(text)
						return nil
					}
					// JSON parsed but no text — treat as error.
					if stderrStr != "" {
						return fmt.Errorf("claude exited with error: %s", stderrStr)
					}
					return fmt.Errorf("claude exited: %w", waitErr)
				} else if perr != nil {
					// JSON had is_error result — return that specific error.
					return perr
				}
			}
			if stderrStr != "" {
				return fmt.Errorf("claude exited with error: %s", stderrStr)
			}
			return fmt.Errorf("claude exited: %w", waitErr)
		}
	case <-time.After(p.effectiveTimeout()):
		// Kill the whole process group so child commands spawned by Claude (e.g.
		// make/go build) don't survive the timeout.
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		<-waitDone
		<-stderrDone
		<-stdoutDone
		return fmt.Errorf("claude timed out after %s", p.effectiveTimeout())
	}

	// Parse the JSON array and extract the session id + text content.
	text, sid, err := parseClaudeOutput(events, stdout.Bytes(), stderrBuf.String(), nil)
	if err != nil {
		return err
	}
	if sid != "" {
		p.LastSessionID = sid
	}
	if text != "" {
		onChunk(text)
	}
	return nil
}

func readClaudeStream(r io.Reader) ([]map[string]interface{}, error) {
	dec := json.NewDecoder(r)
	var events []map[string]interface{}
	for {
		var v interface{}
		if err := dec.Decode(&v); err != nil {
			if err == io.EOF {
				return events, nil
			}
			return events, err
		}
		switch x := v.(type) {
		case map[string]interface{}:
			events = append(events, x)
			logClaudeEvent(x)
		case []interface{}:
			for _, item := range x {
				if m, ok := item.(map[string]interface{}); ok {
					events = append(events, m)
					logClaudeEvent(m)
				}
			}
		}
	}
}

func parseClaudeOutput(events []map[string]interface{}, stdout []byte, stderr string, streamErr error) (string, string, error) {
	if streamErr == nil && len(events) > 0 {
		return collectClaudeText(events)
	}
	if len(stdout) > 0 {
		return parseClaudeJSON(stdout, stderr)
	}
	if streamErr != nil {
		return "", "", streamErr
	}
	return "", "", nil
}

func logClaudeEvent(m map[string]interface{}) {
	switch m["type"] {
	case "system":
		fmt.Fprintf(os.Stderr, "[claude] session=%v model=%v\n", m["session_id"], m["model"])
	case "assistant":
		logClaudeAssistant(m)
	case "result":
		fmt.Fprintf(os.Stderr, "[claude] done turns=%v duration_ms=%v\n", m["num_turns"], m["duration_ms"])
	}
}

func logClaudeAssistant(m map[string]interface{}) {
	msg, ok := m["message"].(map[string]interface{})
	if !ok {
		return
	}
	items, ok := msg["content"].([]interface{})
	if !ok {
		return
	}
	for _, item := range items {
		part, ok := item.(map[string]interface{})
		if !ok || part["type"] != "tool_use" {
			continue
		}
		name, _ := part["name"].(string)
		input, _ := part["input"].(map[string]interface{})
		detail := ""
		if command, _ := input["command"].(string); command != "" {
			detail = ": " + command
		}
		fmt.Fprintf(os.Stderr, "[claude] tool %s%s\n", name, detail)
	}
}

// parseClaudeJSON extracts the assistant text and session id from claude's
// --output-format json output. Claude versions differ: older output may use
// top-level text entries, while newer output nests text under
// assistant.message.content. When a "result" entry reports is_error=true, we
// return its result string as an error so the caller surfaces the real cause
// (e.g. "Not logged in") instead of an opaque "empty response".
//
// When the JSON is truncated (claude exited early after printing only the
// init/system message), json.Unmarshal fails. We then check whether the
// fragment contains a session_id (so the caller can still resume) and return a
// concise error — never the full stdout, which can be hundreds of KB.
func parseClaudeJSON(data []byte, stderr string) (text, sessionID string, err error) {
	var arr []map[string]interface{}
	if jErr := json.Unmarshal(data, &arr); jErr == nil {
		return collectClaudeText(arr)
	}
	var obj map[string]interface{}
	if jErr := json.Unmarshal(data, &obj); jErr == nil {
		return collectClaudeText([]map[string]interface{}{obj})
	}
	if text, sessionID, ok, err := parseClaudeJSONPrefix(data); ok || err != nil {
		return text, sessionID, err
	}

	// JSON is truncated or unparseable. Try to salvage a session_id from the
	// fragment so the caller can persist it for resume. Then return a concise
	// error: never embed the full stdout (can be 100s of KB).
	salvagedSID := extractSessionIDFragment(string(data))
	if salvagedSID != "" {
		sessionID = salvagedSID
	}
	// Include the first 200 chars of stderr if present — that's where the
	// real cause usually is.
	detail := strings.TrimSpace(stderr)
	if len(detail) > 200 {
		detail = detail[:200] + "…"
	}
	if detail != "" {
		return "", sessionID, fmt.Errorf("claude output was truncated/incomplete (stdout_bytes=%d, session_id=%s, stderr: %s)", len(data), sessionID, detail)
	}
	return "", sessionID, fmt.Errorf("claude output was truncated/incomplete (stdout_bytes=%d, session_id=%s, no stderr)", len(data), sessionID)
}

func collectClaudeText(arr []map[string]interface{}) (text, sessionID string, err error) {
	var resultText string
	for _, m := range arr {
		if m["type"] == "system" {
			if sid, ok := m["session_id"].(string); ok && sid != "" {
				sessionID = sid
			}
		}
		if m["type"] == "text" {
			if c, ok := m["content"].(string); ok {
				text += c
			}
		}
		if m["type"] == "assistant" {
			text += nestedAssistantText(m)
		}
		if m["type"] == "result" {
			if isErr, ok := m["is_error"].(bool); ok && isErr {
				if r, ok := m["result"].(string); ok && r != "" {
					return "", "", fmt.Errorf("claude error: %s", r)
				}
			}
			if sid, ok := m["session_id"].(string); ok && sid != "" && sessionID == "" {
				sessionID = sid
			}
			if r, ok := m["result"].(string); ok && r != "" {
				resultText = r
			}
		}
	}
	if text == "" {
		text = resultText
	}
	return text, sessionID, nil
}

func parseClaudeJSONPrefix(data []byte) (text, sessionID string, ok bool, err error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		return "", "", false, nil
	}
	delim, ok := tok.(json.Delim)
	if !ok || delim != '[' {
		return "", "", false, nil
	}
	var arr []map[string]interface{}
	for dec.More() {
		var m map[string]interface{}
		if err := dec.Decode(&m); err != nil {
			break
		}
		arr = append(arr, m)
	}
	if len(arr) == 0 {
		return "", "", false, nil
	}
	text, sessionID, err = collectClaudeText(arr)
	if err != nil {
		return "", "", true, err
	}
	return text, sessionID, text != "" || sessionID != "", nil
}

func nestedAssistantText(m map[string]interface{}) string {
	msg, ok := m["message"].(map[string]interface{})
	if !ok {
		return ""
	}
	items, ok := msg["content"].([]interface{})
	if !ok {
		return ""
	}
	var b strings.Builder
	for _, item := range items {
		part, ok := item.(map[string]interface{})
		if !ok || part["type"] != "text" {
			continue
		}
		if s, ok := part["text"].(string); ok {
			b.WriteString(s)
		}
	}
	return b.String()
}

// extractSessionIDFragment tries to pull a "session_id":"…" value out of a
// potentially-truncated JSON fragment using a regex, so the caller can still
// persist the session id for resume even when the full JSON is unparseable.
func extractSessionIDFragment(s string) string {
	idx := strings.Index(s, `"session_id"`)
	if idx < 0 {
		return ""
	}
	rest := s[idx+len(`"session_id"`):]
	colon := strings.Index(rest, ":")
	if colon < 0 {
		return ""
	}
	rest = rest[colon+1:]
	quote := strings.Index(rest, `"`)
	if quote < 0 {
		return ""
	}
	rest = rest[quote+1:]
	end := strings.Index(rest, `"`)
	if end < 0 {
		return ""
	}
	return rest[:end]
}

// buildClaudePrompt concatenates messages into a single prompt text.
func buildClaudePrompt(messages []Message) string {
	var b strings.Builder
	for _, msg := range messages {
		if msg.Role == "system" {
			b.WriteString("System: ")
			b.WriteString(msg.Content)
			b.WriteString("\n\n")
		} else if msg.Role == "assistant" {
			b.WriteString("Assistant: ")
			b.WriteString(msg.Content)
			b.WriteString("\n\n")
		} else {
			b.WriteString("User: ")
			b.WriteString(msg.Content)
			b.WriteString("\n\n")
		}
	}
	b.WriteString("Assistant:")
	return b.String()
}

// NewClaudeCLIProvider creates a provider that shells out to the `claude` CLI.
// baseURL / authToken / claudeModel come from tomato.yaml's provider section.
// sessionID is accepted for compatibility and ignored.
// It does not require the CLI to exist until Stream is called, so config parsing
// and unit tests work in environments where Claude Code is not installed.
func NewClaudeCLIProvider(modelID, baseURL, authToken, claudeModel, sessionID, repoDir string) (*ClaudeCLIProvider, error) {
	modelName := claudeModel
	if modelName == "" {
		modelName = os.Getenv("ANTHROPIC_MODEL")
	}
	if modelName == "" {
		parts := strings.SplitN(modelID, "/", 2)
		if len(parts) == 2 {
			modelName = parts[1]
		}
	}

	return &ClaudeCLIProvider{
		ModelName:   modelName,
		BaseURL:     baseURL,
		AuthToken:   authToken,
		ClaudeModel: claudeModel,
		CLIPath:     "claude",
		Timeout:     claudeTimeout(),
		SessionID:   sessionID,
		RepoDir:     repoDir,
	}, nil
}

package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
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
//
// When SessionID is non-empty, the provider resumes that claude session
// (--resume <id>) so prior conversation context is reused instead of
// re-sending the full prompt history. After Stream completes, LastSessionID
// holds the session id of this invocation (which may differ from SessionID
// when a new session was started) so the caller can persist it for the next
// step.
type ClaudeCLIProvider struct {
	ModelName   string
	BaseURL     string
	AuthToken   string
	ClaudeModel string
	CLIPath     string
	Timeout     time.Duration
	// SessionID, when non-empty, resumes an existing claude session.
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
		"--output-format", "json",
	}
	if p.ModelName != "" {
		args = append(args, "--model", p.ModelName)
	}
	if p.SessionID != "" {
		args = append(args, "--resume", p.SessionID)
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

	// Forward env from parent, then override with yaml config values
	env := os.Environ()
	if p.BaseURL != "" {
		env = append(env, "ANTHROPIC_BASE_URL="+p.BaseURL)
	}
	if p.AuthToken != "" {
		env = append(env, "ANTHROPIC_AUTH_TOKEN="+p.AuthToken)
	}
	if p.ClaudeModel != "" {
		env = append(env, "ANTHROPIC_MODEL="+p.ClaudeModel)
	}
	cmd.Env = env

	var stdout, stderrBuf bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderrBuf

	fmt.Fprintf(os.Stderr, "[claude] command=%s %s timeout=%s\n", cliPath, strings.Join(args, " "), p.effectiveTimeout())

	// Run claude to completion. JSON output cannot be streamed incrementally
	// (the full array must be parseable), so we wait for the process to exit
	// and then parse.
	waitDone := make(chan error, 1)
	go func() {
		waitDone <- cmd.Run()
	}()

	select {
	case waitErr := <-waitDone:
		if waitErr != nil {
			stderrStr := strings.TrimSpace(stderrBuf.String())
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
		return fmt.Errorf("claude timed out after %s", p.effectiveTimeout())
	}

	// Parse the JSON array and extract the session id + text content.
	text, sid, err := parseClaudeJSON(stdout.Bytes())
	if err != nil {
		return fmt.Errorf("parsing claude output: %w (stdout=%q)", err, stdout.String())
	}
	if sid != "" {
		p.LastSessionID = sid
	}
	if text != "" {
		onChunk(text)
	}
	return nil
}

// parseClaudeJSON extracts the assistant text and session id from claude's
// --output-format json output. The output is a JSON array of message objects;
// we collect every "text" entry's content and the first "system" entry's
// session_id.
func parseClaudeJSON(data []byte) (text, sessionID string, err error) {
	// claude may emit multiple JSON objects or a JSON array depending on
	// version; handle both by attempting array first.
	var arr []map[string]interface{}
	if jErr := json.Unmarshal(data, &arr); jErr == nil {
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
		}
		return text, sessionID, nil
	}
	// Fall back to a single object.
	var obj map[string]interface{}
	if jErr := json.Unmarshal(data, &obj); jErr != nil {
		return "", "", jErr
	}
	if sid, ok := obj["session_id"].(string); ok {
		sessionID = sid
	}
	if c, ok := obj["content"].(string); ok {
		text = c
	}
	if r, ok := obj["result"].(string); ok && text == "" {
		text = r
	}
	return text, sessionID, nil
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
// sessionID, when non-empty, resumes an existing claude session.
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

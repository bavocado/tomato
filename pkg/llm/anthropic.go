package llm

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ClaudeCLIProvider runs the `claude` CLI tool to execute AI tasks.
// It forks the `claude` binary with:
//   --print --permission-mode auto --effort high
// The prompt is passed via stdin.
// ANTHROPIC_BASE_URL, ANTHROPIC_AUTH_TOKEN, ANTHROPIC_MODEL are set
// as environment variables from the yaml config (can be overridden by env).
type ClaudeCLIProvider struct {
	ModelName    string
	BaseURL      string
	AuthToken    string
	ClaudeModel  string
}

func (p *ClaudeCLIProvider) Model() string {
	return p.ModelName
}

func (p *ClaudeCLIProvider) Stream(messages []Message, onChunk func(string)) error {
	prompt := buildClaudePrompt(messages)

	args := []string{
		"--print",
		"--permission-mode", "auto",
		"--effort", "high",
	}
	if p.ModelName != "" {
		args = append(args, "--model", p.ModelName)
	}

	cmd := exec.Command("claude", args...)
	cmd.Stdin = strings.NewReader(prompt)

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

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("creating claude stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting claude: %w", err)
	}

	// Stream stdout in chunks to onChunk for progressive output
	const chunkSize = 120
	reader := bufio.NewReader(stdout)
	buf := make([]byte, chunkSize)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			onChunk(string(buf[:n]))
		}
		if err != nil {
			break
		}
	}

	waitErr := cmd.Wait()
	if waitErr != nil {
		stderrStr := strings.TrimSpace(stderrBuf.String())
		if stderrStr != "" {
			return fmt.Errorf("claude exited with error: %s", stderrStr)
		}
		return fmt.Errorf("claude exited: %w", waitErr)
	}

	return nil
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
// baseURL / authToken / claudeModel come from tomato.yaml's `anthropic:` section.
func NewClaudeCLIProvider(modelID, baseURL, authToken, claudeModel string) (*ClaudeCLIProvider, error) {
	// Check if claude is available on PATH
	if _, err := exec.LookPath("claude"); err != nil {
		return nil, fmt.Errorf("claude CLI not found on PATH (install via: npm i -g @anthropic-ai/claude-code): %w", err)
	}

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
	}, nil
}
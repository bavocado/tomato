package llm

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// ClaudeCLIProvider runs the `claude` CLI tool to execute AI tasks.
// It forks the `claude` binary with:
//
//	--print --permission-mode auto --effort high
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
	}
	if p.ModelName != "" {
		args = append(args, "--model", p.ModelName)
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

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("creating claude stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting claude: %w", err)
	}

	fmt.Fprintf(os.Stderr, "[claude] command=%s %s timeout=%s\n", cliPath, strings.Join(args, " "), p.effectiveTimeout())

	// Stream stdout in chunks to onChunk for progressive output.
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
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
	}()

	waitDone := make(chan error, 1)
	go func() {
		waitDone <- cmd.Wait()
	}()

	select {
	case waitErr := <-waitDone:
		<-readDone
		if waitErr != nil {
			stderrStr := strings.TrimSpace(stderrBuf.String())
			if stderrStr != "" {
				return fmt.Errorf("claude exited with error: %s", stderrStr)
			}
			return fmt.Errorf("claude exited: %w", waitErr)
		}
		return nil
	case <-time.After(p.effectiveTimeout()):
		// Kill the whole process group so child commands spawned by Claude (e.g.
		// make/go build) don't survive the timeout.
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		<-waitDone
		<-readDone
		return fmt.Errorf("claude timed out after %s", p.effectiveTimeout())
	}
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
// It does not require the CLI to exist until Stream is called, so config parsing
// and unit tests work in environments where Claude Code is not installed.
func NewClaudeCLIProvider(modelID, baseURL, authToken, claudeModel string) (*ClaudeCLIProvider, error) {
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
	}, nil
}

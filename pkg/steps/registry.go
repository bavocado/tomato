package steps

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bavocado/tomato/pkg/adapter"
	"github.com/bavocado/tomato/pkg/budget"
	"github.com/bavocado/tomato/pkg/llm"
	"github.com/bavocado/tomato/pkg/model"
	"github.com/bavocado/tomato/pkg/runner"
)

// StepConfig is the minimal config for running a step.
type StepConfig struct {
	RepoDir       string
	FeatureDir    string
	Feature       string
	ModelName     string
	APIKey        string
	PromptVersion string
	LLMStream     runner.LLMFunc
	BudgetTracker *budget.Tracker
	// Adapters resolves a workflow role (e.g. "pr", "task") to the adapter
	// Bridge that serves it. May be nil when no adapter is configured.
	Adapters *adapter.Registry
	// Anthropic-specific connection parameters (from tomato.yaml)
	AnthropicURL string
	AnthropicKey string
	// DisableCodegraph, when true, builds the LLM provider without the codegraph
	// MCP mount. Steps that analyze a document (e.g. decompose) rather than the
	// codebase set this to stop claude from exploring the repo via MCP, which
	// otherwise stalls the step for minutes on indexed repos.
	DisableCodegraph bool
	// ShareSession, when true, resumes the claude session saved by the prior
	// step (LoadSession) and persists this step's session id afterwards
	// (SaveSession), so all steps in one `tomato run` share one session.
	// Single-shot commands leave this false to start fresh.
	ShareSession bool
	// CreateIssue, when true, makes the task step create a tracker issue via
	// the task adapter (gh issue create). Defaults to false: review findings
	// live as PR comments + fix rounds, not as GitHub issues.
	CreateIssue bool
}

// StepFunc is a function that executes a step and returns a result.
type StepFunc func(cfg *StepConfig, args []string) *model.StepResult

var registry = map[string]StepFunc{}

// Register adds a step to the global registry.
func Register(name string, fn StepFunc) {
	registry[name] = fn
}

// Get returns a registered step function by name.
func Get(name string) (StepFunc, error) {
	fn, ok := registry[name]
	if !ok {
		names := make([]string, 0, len(registry))
		for n := range registry {
			names = append(names, n)
		}
		return nil, fmt.Errorf("unknown step %q (available: %s)", name, strings.Join(names, ", "))
	}
	return fn, nil
}

// NewLLMStream creates a streaming function from a StepConfig.
//
// When cfg.ShareSession is true (workflow steps), it resumes the claude session
// saved by the prior step and persists this step's session id afterwards, so all
// steps in one `tomato run` share one claude session's context. Each step still
// pins its own model via cfg.ModelName, so per-step model routing is preserved.
// When false (single-shot commands), each invocation starts a fresh session.
func NewLLMStream(cfg *StepConfig) runner.LLMFunc {
	return func(messages []runner.Message, onChunk func(string)) error {
		llmMessages := make([]llm.Message, len(messages))
		for i, m := range messages {
			llmMessages[i] = llm.Message{Role: m.Role, Content: m.Content}
		}

		// codegraph MCP is mounted by the provider when RepoDir is set. Steps that
		// set DisableCodegraph (e.g. decompose, which analyzes a doc not the repo)
		// pass an empty RepoDir so claude does not explore the codebase via MCP.
		providerRepoDir := cfg.RepoDir
		if cfg.DisableCodegraph {
			providerRepoDir = ""
		}

		// Session sharing: resume the prior step's claude session so the whole
		// workflow shares one context. Single-shot commands start fresh instead.
		var sessionID string
		if cfg.ShareSession {
			sessionID = llm.LoadSession(cfg.RepoDir).SessionID
		} else {
			_ = llm.ClearSession(cfg.RepoDir)
		}

		provider, err := llm.NewProvider(llm.ProviderConfig{
			ModelID:   cfg.ModelName,
			APIKey:    cfg.APIKey,
			BaseURL:   cfg.AnthropicURL,
			AuthToken: cfg.AnthropicKey,
			RepoDir:   providerRepoDir,
			SessionID: sessionID,
		})
		if err != nil {
			return err
		}
		streamErr := provider.Stream(llmMessages, onChunk)
		// Persist this invocation's session id so the next step can resume it.
		// Best-effort: a Stream error may still have produced a session id (e.g.
		// truncated JSON), and saving it lets later steps recover the context.
		if cfg.ShareSession {
			if cp, ok := provider.(*llm.ClaudeCLIProvider); ok {
				if sid := cp.LastSessionID; sid != "" {
					_ = llm.SaveSession(cfg.RepoDir, llm.SessionRef{SessionID: sid})
				}
			}
		}
		return streamErr
	}
}

// fileJoin is a helper to join paths relative to the config's RepoDir or FeatureDir.
func fileJoin(dir, name string) string {
	if filepath.IsAbs(name) {
		return name
	}
	return filepath.Join(dir, name)
}

package steps

import (
	"fmt"
	"path/filepath"
	"strings"

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
	// Anthropic-specific connection parameters (from tomato.yaml)
	AnthropicURL   string
	AnthropicKey   string
	AnthropicModel string
}

// StepFunc is a function that executes a step and returns a result.
type StepFunc func(cfg *StepConfig, args []string) *model.StepResult

// GlobalAdapterBin stores the path to the adapter binary set by the engine.
var GlobalAdapterBin string

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

// NewLLMStream creates a streaming function from a StepConfig (supports all providers).
func NewLLMStream(cfg *StepConfig) runner.LLMFunc {
	return func(messages []runner.Message, onChunk func(string)) error {
		llmMessages := make([]llm.Message, len(messages))
		for i, m := range messages {
			llmMessages[i] = llm.Message{Role: m.Role, Content: m.Content}
		}
		provider, err := llm.NewProvider(llm.ProviderConfig{
			ModelID:        cfg.ModelName,
			APIKey:         cfg.APIKey,
			AnthropicURL:   cfg.AnthropicURL,
			AnthropicKey:   cfg.AnthropicKey,
			AnthropicModel: cfg.AnthropicModel,
		})
		if err != nil {
			return err
		}
		return provider.Stream(llmMessages, onChunk)
	}
}

// fileJoin is a helper to join paths relative to the config's RepoDir or FeatureDir.
func fileJoin(dir, name string) string {
	if filepath.IsAbs(name) {
		return name
	}
	return filepath.Join(dir, name)
}

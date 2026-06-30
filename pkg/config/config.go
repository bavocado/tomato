package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is the root of tomato.yaml.
type Config struct {
	Feature     string                              `yaml:"feature,omitempty"`
	Models      ModelsConfig                        `yaml:"models"`
	Providers   map[string]ProviderConnectionConfig `yaml:"providers"`
	Anthropic   AnthropicConfig                     `yaml:"anthropic"` // legacy compatibility
	Budget      BudgetConfig                        `yaml:"budget"`
	Impl        ImplConfig                          `yaml:"impl"`
	Workflows   map[string]WorkflowDef              `yaml:"workflows"`
	CustomSteps map[string]CustomStepDef            `yaml:"custom_steps"`
	Adapters    map[string]AdapterDef               `yaml:"adapters"`
	Roles       map[string]string                   `yaml:"roles"`
}

// ModelsConfig defines per-step model routing.
type ModelsConfig struct {
	Default string            `yaml:"default"`
	Steps   map[string]string `yaml:"steps"`
}

// ProviderConnectionConfig defines Claude Code compatible provider settings.
// These values are passed to the `claude` CLI as ANTHROPIC_* environment
// variables, even when the logical provider is glm/deepseek.
type ProviderConnectionConfig struct {
	BaseURL   string `yaml:"base_url"`
	AuthToken string `yaml:"auth_token"`
	Model     string `yaml:"model"`
}

// AnthropicConfig defines legacy connection parameters for Anthropic.
type AnthropicConfig = ProviderConnectionConfig

// ResolvedAuthToken returns the effective auth token, preferring the
// ANTHROPIC_AUTH_TOKEN environment variable over the yaml value. Design §2.4
// mandates keys come from the environment (not git); the yaml field remains as
// a fallback for local convenience.
func (a AnthropicConfig) ResolvedAuthToken() string {
	if env := os.Getenv("ANTHROPIC_AUTH_TOKEN"); env != "" {
		return env
	}
	return a.AuthToken
}

// ResolvedBaseURL returns the effective base URL, preferring ANTHROPIC_BASE_URL.
func (a AnthropicConfig) ResolvedBaseURL() string {
	if env := os.Getenv("ANTHROPIC_BASE_URL"); env != "" {
		return env
	}
	return a.BaseURL
}

// ResolvedModel returns the effective model, preferring ANTHROPIC_MODEL.
func (a AnthropicConfig) ResolvedModel() string {
	if env := os.Getenv("ANTHROPIC_MODEL"); env != "" {
		return env
	}
	return a.Model
}

// ResolveProviderConfig returns the provider connection config for a modelID.
// It first checks providers.<provider>, then falls back to legacy anthropic.
func (c *Config) ResolveProviderConfig(modelID string) ProviderConnectionConfig {
	provider := modelID
	for i := 0; i < len(modelID); i++ {
		if modelID[i] == '/' {
			provider = modelID[:i]
			break
		}
	}
	if c.Providers != nil {
		if p, ok := c.Providers[provider]; ok {
			return p
		}
	}
	if provider == "anthropic" {
		return c.Anthropic
	}
	return ProviderConnectionConfig{}
}

// BudgetConfig defines token budget limits.
type BudgetConfig struct {
	Mode         string         `yaml:"mode"`
	GlobalPerRun int            `yaml:"global_per_run"`
	PerStep      map[string]int `yaml:"per_step"`
	OnExceed     string         `yaml:"on_exceed"`
	DegradeTo    string         `yaml:"degrade_to"`
}

// ImplConfig holds toggleable optional behaviors for the impl step.
type ImplConfig struct {
	// RewriteArch controls the §2.8 post-impl rewrite of architecture.md to
	// reflect the real, as-implemented architecture. Defaults to true when
	// unset (design §2.9.6); set false to skip the extra LLM call.
	RewriteArch *bool `yaml:"rewrite_arch"`
}

// RewriteArchEnabled reports whether the post-impl architecture rewrite runs.
func (c ImplConfig) RewriteArchEnabled() bool {
	if c.RewriteArch == nil {
		return true
	}
	return *c.RewriteArch
}

// WorkflowDef defines a named workflow.
type WorkflowDef struct {
	Steps []WorkflowStep `yaml:"steps"`
}

// WorkflowStep is either a step name or a meta-step with params.
type WorkflowStep struct {
	Name       string
	RunsOn     string
	MaxRounds  int
	OnFail     string
	FixStep    string
	IsMetaStep bool
}

// MarshalYAML serializes a step back to YAML.
func (s WorkflowStep) MarshalYAML() (interface{}, error) {
	if s.IsMetaStep && s.Name == "review_loop" {
		return map[string]interface{}{
			"review_loop": map[string]interface{}{
				"max_rounds": s.MaxRounds,
				"on_fail":    s.OnFail,
			},
		}, nil
	}

	// Simple step — just the name as a string
	return s.Name, nil
}

// UnmarshalYAML handles the step being either a string or a map.

// AdapterDef configures a driver CLI adapter.
type AdapterDef struct {
	Bin string            `yaml:"bin"`
	Env map[string]string `yaml:"env,omitempty"`
}

// CustomStepDef declares a user-defined workflow step backed by a prompt +
// file inputs/outputs, executed through the same runner as built-in steps
// (design §3.3, Task 4). The step name (the map key) is referenced from a
// workflow's steps list.
type CustomStepDef struct {
	Prompt  string   `yaml:"prompt"`
	Inputs  []string `yaml:"inputs"`
	Outputs []string `yaml:"outputs"`
	Model   string   `yaml:"model"`
}

// UnmarshalYAML handles the step being either a string or a map.
func (s *WorkflowStep) UnmarshalYAML(value *yaml.Node) error {
	// Try string first — e.g., "- spec"
	var strVal string
	if err := value.Decode(&strVal); err == nil {
		s.Name = strVal
		s.IsMetaStep = false
		return nil
	}

	// It's a map. Structure is either:
	//   - step_name: { runs_on: ... }   (a step with runs_on)
	//   - review_loop: { max_rounds: }  (a meta-step)
	// The outer map has ONE key (the step name), and its value is the params.
	var rawMap map[string]yaml.Node
	if err := value.Decode(&rawMap); err != nil {
		return fmt.Errorf("step must be a string or a map")
	}
	if len(rawMap) != 1 {
		return fmt.Errorf("step map must have exactly one key")
	}

	// Extract the step name (the single key) and its params node
	var stepName string
	var paramsNode yaml.Node
	for k, v := range rawMap {
		stepName = k
		paramsNode = v
	}

	if stepName == "review_loop" {
		s.Name = "review_loop"
		s.IsMetaStep = true
		var loop struct {
			MaxRounds int    `yaml:"max_rounds"`
			OnFail    string `yaml:"on_fail"`
			FixStep   string `yaml:"fix_step"`
		}
		if err := paramsNode.Decode(&loop); err != nil {
			return fmt.Errorf("parsing review_loop params: %w", err)
		}
		s.MaxRounds = loop.MaxRounds
		s.OnFail = loop.OnFail
		s.FixStep = loop.FixStep
		return nil
	}

	// Regular step with optional runs_on
	s.Name = stepName
	s.IsMetaStep = false
	var params struct {
		RunsOn string `yaml:"runs_on"`
	}
	if err := paramsNode.Decode(&params); err == nil {
		s.RunsOn = params.RunsOn
	}
	return nil
}

// Parse reads and validates a tomato.yaml.
func Parse(data []byte) (*Config, error) {
	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing tomato.yaml: %w", err)
	}

	if len(cfg.Workflows) == 0 {
		return nil, fmt.Errorf("tomato.yaml must define at least one workflow")
	}
	for name, wf := range cfg.Workflows {
		if len(wf.Steps) == 0 {
			return nil, fmt.Errorf("workflow %q has no steps", name)
		}
		for _, s := range wf.Steps {
			// runs_on is reserved syntax for v2 remote agents (design §2.8,
			// §5). v1 accepts only "local" (or unset); any other value is a
			// hard error so workflows cannot silently target non-existent
			// remote agents.
			if s.RunsOn != "" && s.RunsOn != "local" {
				return nil, fmt.Errorf("workflow %q step %q: runs_on: %q is reserved for v2 remote agents (only \"local\" is accepted in v1)", name, s.Name, s.RunsOn)
			}
		}
	}
	if cfg.Models.Default == "" {
		cfg.Models.Default = "glm/glm-5.2"
	}

	return cfg, nil
}

// Load reads tomato.yaml from the given directory.
func Load(dir string) (*Config, error) {
	path := filepath.Join(dir, "tomato.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return Parse(data)
}

// Save writes a Config to a file.
func Save(cfg *Config, path string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// Default returns the default configuration with the balanced preset:
// GLM-5.2 across the board, with DeepSeek for implementation.
func Default() *Config {
	return &Config{
			Models: ModelsConfig{
				Default: "glm/glm-5.2",
				Steps: map[string]string{
					"spec":   "glm/glm-5.2",
					"design": "glm/glm-5.2",
					"impl":   "deepseek/deepseek-v4-pro",
					"review": "glm/glm-5.2",
					"test":   "glm/glm-5.2",
				},
			},
			Providers: map[string]ProviderConnectionConfig{
				"glm": {
					BaseURL:   "",
					AuthToken: "",
					Model:     "glm-5.2",
				},
				"deepseek": {
					BaseURL:   "",
					AuthToken: "",
					Model:     "deepseek-v4-pro",
				},
			},
			Anthropic: AnthropicConfig{
			// Optional provider: only used when a step is routed to
			// anthropic/* (runs via the claude CLI). Token comes from
			// ANTHROPIC_AUTH_TOKEN env by preference (design §2.4).
			BaseURL:   "https://api.anthropic.com",
			AuthToken: "",
			Model:     "claude-sonnet-4-20250514",
		},
		Budget: BudgetConfig{
			Mode:         "balanced",
			GlobalPerRun: 300000,
			PerStep:      map[string]int{"spec": 50000, "design": 100000, "impl": 100000, "review": 30000, "test": 20000},
			OnExceed:     "warn",
			DegradeTo:    "deepseek/deepseek-4pro",
		},
		Workflows: map[string]WorkflowDef{
			"default": {
				Steps: []WorkflowStep{
					{Name: "spec"},
					{Name: "task"},
					{Name: "design"},
					{Name: "impl"},
					{Name: "pr"},
					{Name: "review_loop", IsMetaStep: true, MaxRounds: 2, OnFail: "stop"},
					{Name: "test"},
				},
			},
		},
	}
}

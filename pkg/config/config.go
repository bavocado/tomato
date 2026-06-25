package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is the root of tomato.yaml.
type Config struct {
	Models    ModelsConfig            `yaml:"models"`
	Budget    BudgetConfig            `yaml:"budget"`
	Workflows map[string]WorkflowDef  `yaml:"workflows"`
	Adapters  map[string]AdapterDef   `yaml:"adapters"`
	Roles     map[string]string       `yaml:"roles"`
}

// ModelsConfig defines per-step model routing.
type ModelsConfig struct {
	Default string            `yaml:"default"`
	Steps   map[string]string `yaml:"steps"`
}

// BudgetConfig defines token budget limits.
type BudgetConfig struct {
	Mode         string         `yaml:"mode"`
	GlobalPerRun int            `yaml:"global_per_run"`
	PerStep      map[string]int `yaml:"per_step"`
	OnExceed     string         `yaml:"on_exceed"`
	DegradeTo    string         `yaml:"degrade_to"`
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
	}
if cfg.Models.Default == "" {
			cfg.Models.Default = "anthropic/claude-sonnet-4-20250514"
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

// Default returns the default configuration with the balanced preset.
	func Default() *Config {
		return &Config{
			Models: ModelsConfig{
				Default: "openai/gpt-5",
				Steps: map[string]string{
					"spec":   "anthropic/claude-sonnet-4-20250514",
					"design": "anthropic/claude-sonnet-4-20250514",
					"impl":   "anthropic/claude-sonnet-4-20250514",
					"review": "anthropic/claude-sonnet-4-20250514",
					"test":   "openai/gpt-5",
				},
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
					{Name: "design"},
					{Name: "impl"},
					{Name: "pr"},
					{Name: "review_loop", IsMetaStep: true, MaxRounds: 2, OnFail: "stop"},
					{Name: "test"},
					{Name: "task"},
				},
			},
		},
	}
}
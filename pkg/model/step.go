package model

import "time"

// Step defines a single execution unit in a workflow.
type Step struct {
	Name           string   `yaml:"-"`  // set from YAML key
	PromptTemplate string   `yaml:"prompt,omitempty"`
	InputPaths     []string `yaml:"inputs,omitempty"`
	OutputPaths    []string `yaml:"outputs,omitempty"`
	Model          string   `yaml:"model,omitempty"`
	RunsOn         string   `yaml:"runs_on,omitempty"`
}

// StepResult captures the outcome of running a step.
type StepResult struct {
	StepName   string    `json:"step_name"`
	RunID      string    `json:"run_id"`
	StartedAt  time.Time `json:"started_at"`
	DurationMs int64     `json:"duration_ms"`
	TokensIn   int       `json:"tokens_in"`
	TokensOut  int       `json:"tokens_out"`
	ModelUsed  string    `json:"model_used"`
	CacheHit   bool      `json:"cache_hit,omitempty"`
	Error      string    `json:"error,omitempty"`
	Success    bool      `json:"success"`
}

// RunMeta is the structure written to .tomato/runs/<id>/meta.json.
type RunMeta struct {
	RunID       string    `json:"run_id"`
	StepName    string    `json:"step_name"`
	StartedAt   time.Time `json:"started_at"`
	FinishedAt  time.Time `json:"finished_at"`
	DurationMs  int64     `json:"duration_ms"`
	ModelUsed   string    `json:"model_used"`
	TokensIn    int       `json:"tokens_in"`
	TokensOut   int       `json:"tokens_out"`
	CacheHit    bool      `json:"cache_hit"`
	Success     bool      `json:"success"`
	Error       string    `json:"error,omitempty"`
	InputFiles  []string  `json:"input_files"`
	OutputFiles []string  `json:"output_files"`
}
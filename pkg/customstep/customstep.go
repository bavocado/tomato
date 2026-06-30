package customstep

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bavocado/tomato/pkg/budget"
	"github.com/bavocado/tomato/pkg/config"
	"github.com/bavocado/tomato/pkg/model"
	"github.com/bavocado/tomato/pkg/runner"
)

// Config carries the dependencies a custom step needs to execute.
type Config struct {
	RepoDir       string
	ModelName     string
	LLMStream     runner.LLMFunc
	BudgetTracker *budget.Tracker
}

// Run executes a user-defined step: it reads the prompt template, expands input
// globs against the repo, and delegates to runner.Execute so custom steps share
// the same prompt-substitution, budget, and artifact-writing behavior as
// built-in steps (design §3.3, Task 4). The step name is the custom_steps map
// key referenced from a workflow's steps list.
func Run(name string, def config.CustomStepDef, cfg Config) *model.StepResult {
	promptBytes, err := os.ReadFile(filepath.Join(cfg.RepoDir, def.Prompt))
	if err != nil {
		return &model.StepResult{StepName: name, Success: false, Error: fmt.Sprintf("reading prompt %s: %v", def.Prompt, err)}
	}
	inputs := expandInputs(cfg.RepoDir, def.Inputs)
	outputs := make([]string, 0, len(def.Outputs))
	for _, out := range def.Outputs {
		outputs = append(outputs, filepath.Join(cfg.RepoDir, out))
	}
	return runner.Execute(name, string(promptBytes), inputs, outputs, cfg.RepoDir, cfg.ModelName, cfg.LLMStream, "custom-v1", cfg.BudgetTracker)
}

// expandInputs resolves input glob patterns relative to the repo dir. Go's
// filepath.Glob does not support recursive **, so patterns match a single path
// component (matching the documented v1 behavior).
func expandInputs(repoDir string, patterns []string) []string {
	var out []string
	for _, p := range patterns {
		matches, _ := filepath.Glob(filepath.Join(repoDir, p))
		out = append(out, matches...)
	}
	return out
}

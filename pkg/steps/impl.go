package steps

import (
	"path/filepath"

	"github.com/bavocado/tomato/pkg/model"
	"github.com/bavocado/tomato/pkg/runner"
)

var ImplPrompt = `You are a software engineer. Implement the code based on the following design documents.

Architecture:
{{.architecture.md}}

UI Specification:
{{.ui-spec.md}}

Implementation Plan:
{{.implementation.md}}

Output the actual source code files. Include meaningful comments.`

func init() {
	Register("impl", runImpl)
}

func runImpl(cfg *StepConfig, args []string) *model.StepResult {
	inputFiles := []string{
		filepath.Join(cfg.FeatureDir, "architecture.md"),
		filepath.Join(cfg.FeatureDir, "ui-spec.md"),
		filepath.Join(cfg.FeatureDir, "implementation.md"),
	}
	return runner.Execute(
		"impl",
		ImplPrompt,
		inputFiles,
		[]string{filepath.Join(cfg.FeatureDir, "impl-output.md")},
		cfg.RepoDir,
		cfg.ModelName,
		cfg.LLMStream,
		cfg.PromptVersion,
		cfg.BudgetTracker,
	)
}
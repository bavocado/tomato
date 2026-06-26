package steps

import (
	"path/filepath"

	"github.com/bavocado/tomato/pkg/model"
	"github.com/bavocado/tomato/pkg/runner"
)

// SpecPrompt is the prompt template for the spec step.
var SpecPrompt = `You are a product manager. Based on the user's rough idea below, write a clear PRD (Product Requirements Document) in markdown.

User's idea:
{{.prd.md}}

Output a structured PRD with sections: Overview, Goals & Success Metrics, Scope, User Stories, Open Questions.`

func init() {
	Register("spec", runSpec)
}

func runSpec(cfg *StepConfig, args []string) *model.StepResult {
	prdPath := filepath.Join(cfg.FeatureDir, "prd.md")
	inputFiles := []string{prdPath}
	outputFiles := []string{prdPath}
	return runner.Execute(
		"spec",
		SpecPrompt,
		inputFiles,
		outputFiles,
		cfg.RepoDir,
		cfg.ModelName,
		cfg.LLMStream,
		cfg.PromptVersion,
		cfg.BudgetTracker,
	)
}
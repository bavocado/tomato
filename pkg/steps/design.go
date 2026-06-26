package steps

import (
	"path/filepath"

	"github.com/bavocado/tomato/pkg/model"
	"github.com/bavocado/tomato/pkg/runner"
)

var DesignPrompt = `You are a software architect. Based on the PRD below, produce three design documents in markdown.

PRD:
{{.prd.md}}

Output these sections, separated by "---":
1. Architecture: System components, data flow, technology choices. Include a mermaid diagram.
2. UI Specification: Page list, component spec, interaction flows (text description, no mockup).
3. Implementation plan: File structure, key function signatures, key process pseudocode.`

func init() {
	Register("design", runDesign)
}

func runDesign(cfg *StepConfig, args []string) *model.StepResult {
	prdPath := filepath.Join(cfg.FeatureDir, "prd.md")
	return runner.Execute(
		"design",
		DesignPrompt,
		[]string{prdPath},
		[]string{
			filepath.Join(cfg.FeatureDir, "architecture.md"),
			filepath.Join(cfg.FeatureDir, "ui-spec.md"),
			filepath.Join(cfg.FeatureDir, "implementation.md"),
		},
		cfg.RepoDir,
		cfg.ModelName,
		cfg.LLMStream,
		cfg.PromptVersion,
		cfg.BudgetTracker,
	)
}
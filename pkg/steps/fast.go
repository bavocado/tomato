package steps

import (
	"path/filepath"

	"github.com/bavocado/tomato/pkg/model"
	"github.com/bavocado/tomato/pkg/runner"
)

var FastPrompt = `You are tomato running in fast mode inside one Claude Code invocation.

` + PonytailRuleset + `

Complete the feature end-to-end without asking tomato to call Claude again.

Feature idea:
{{.idea.txt}}

Existing PRD, if any:
{{.prd.md}}

Existing architecture, if any:
{{.architecture.md}}

Existing implementation notes, if any:
{{.implementation.md}}

Do this in one pass:
1. Inspect the repo and infer the smallest correct implementation.
2. Edit the source and tests directly.
3. Run the smallest useful test command, widening only if needed.
4. Fix failures you caused.
5. Return a short markdown report.

Before each phase, emit one short progress line exactly like:
TOMATO_STEP: 1/5 inspect - reading relevant code

Use these phase names:
- inspect
- plan
- implement
- test
- summarize

Rules:
- Do not run "tomato run" or spawn another tomato workflow.
- Keep the change minimal.
- Tests are required unless the change is documentation-only; say exactly what ran.
- If CodeDB/codegraph MCP tools are available, query them before broad file reads.
- Use Claude Code's internal task/todo tools if helpful, but keep this as one tomato LLM call.
- Do not create the PR yourself; tomato will run its PR step after this fast step.

Output markdown with this exact structure:

# Fast Implementation Report

## Summary

## Files Changed

## Tests Run

## Risks`

func init() {
	Register("fast", runFast)
}

func runFast(cfg *StepConfig, args []string) *model.StepResult {
	return runner.Execute(
		"fast",
		FastPrompt,
		[]string{
			filepath.Join(cfg.FeatureDir, "idea.txt"),
			filepath.Join(cfg.FeatureDir, "prd.md"),
			filepath.Join(cfg.FeatureDir, "architecture.md"),
			filepath.Join(cfg.FeatureDir, "implementation.md"),
		},
		[]string{filepath.Join(cfg.FeatureDir, "fast-output.md")},
		cfg.RepoDir,
		cfg.ModelName,
		cfg.LLMStream,
		cfg.PromptVersion,
		cfg.BudgetTracker,
	)
}

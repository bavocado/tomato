package steps

import (
	"path/filepath"
	"time"

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
2. Plan the smallest safe change.
3. Edit the source and tests directly.
4. Review the change using a fresh Claude Code subagent/task.
5. Fix blocking review findings in this same invocation.
6. Run the smallest useful test command, widening only if needed.
7. Commit and push the branch.
8. Create or update the PR.
9. Comment the review result and fix result on the PR.
10. Return a short markdown report.

Before each phase, emit one short progress line exactly like:
TOMATO_STEP: 1/10 inspect - reading relevant code

Use these phase names:
- inspect
- plan
- implement
- review
- fix
- test
- pr
- comment
- summarize

Rules:
- Do not run "tomato run" or spawn another tomato workflow.
- Keep the change minimal.
- Run review/fix inside this one Claude Code invocation, using subagents/tasks where useful.
- Tests are required unless the change is documentation-only; say exactly what ran.
- Commit source, tests, and relevant tomato artifacts before pushing.
- Create or update the PR yourself with the repo's normal GitHub tooling.
- Comment both the review result and the fix result on the PR.
- If CodeDB/codegraph MCP tools are available, query them before broad file reads.
- Use Claude Code's internal task/todo tools if helpful, but keep this as one tomato LLM call.

Output markdown with this exact structure:

# Fast Implementation Report

## Summary

## Files Changed

## Review Result

## Fix Result

## Tests Run

## PR

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
		cfg.PromptVersion+"-"+time.Now().UTC().Format(time.RFC3339Nano),
		cfg.BudgetTracker,
	)
}

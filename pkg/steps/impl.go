package steps

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/bavocado/tomato/pkg/model"
	"github.com/bavocado/tomato/pkg/runner"
)

var ImplPrompt = `You are tomato's implementation engineer.

` + PonytailRuleset + `

Implement according to the design documents below. Produce a clear, reviewable implementation plan and code-oriented output that can be applied to the repository.

Architecture:
{{.architecture.md}}

UI Specification:
{{.ui-spec.md}}

Implementation Plan:
{{.implementation.md}}

Output markdown with this exact structure:

# Implementation Output

## 1. Summary
Briefly describe what is being implemented.

## 2. Files to Change
For each file, include:
- Path
- Create / Modify / Delete
- Responsibility

## 3. Patch Plan
For each file, describe the exact change.
When writing or modifying code, output a fenced code block whose opening-fence info string is "<lang>:<path>" — for example a fence line of: go:internal/admin/handler.go — followed by the COMPLETE new file content, then the closing fence. Only these path-fenced blocks are extracted and written to disk automatically; code in plain fences without a path is NOT applied. One block per file.

## 4. Commands to Run
List build/test/lint commands that should verify the change.

## 5. Risk Notes
List risky areas, compatibility concerns, or places needing human review.

## 6. Post-Implementation Architecture Update
Summarize how the actual implementation changes or confirms architecture.md.

Rules:
- Follow the design; do not invent a different architecture.
- Keep changes minimal and cohesive.
- Prefer small functions, explicit errors, and testable boundaries.
- If design conflicts with current repo structure, explain the conflict and choose the least invasive fix.
- Do not hide uncertainty — mark it clearly under Risk Notes.`

func init() {
	Register("impl", runImpl)
}

func runImpl(cfg *StepConfig, args []string) *model.StepResult {
	inputFiles := []string{
		filepath.Join(cfg.FeatureDir, "architecture.md"),
		filepath.Join(cfg.FeatureDir, "ui-spec.md"),
		filepath.Join(cfg.FeatureDir, "implementation.md"),
	}
	// Fold fix-round args into the prompt version so each review_loop fix
	// round has a distinct cache key. Without this, fix-r2 would hit fix-r1's
	// cache (same prompt + model) and the fix would be a no-op.
	promptVersion := cfg.PromptVersion
	prompt := ImplPrompt
	if len(args) > 0 {
		promptVersion = promptVersion + "-" + args[0]
		// In a review_loop fix round (args[0]="fix-r<N>"), feed the prior
		// round's review comments so the model fixes the blocking issues
		// rather than regenerating from scratch. Without this, fix rounds
		// reproduce the same impl-output and the next review finds the same
		// blocking issues again.
		if strings.HasPrefix(args[0], "fix-r") {
			round := strings.TrimPrefix(args[0], "fix-r")
			reviewPath := filepath.Join(cfg.FeatureDir, "reviews", "r"+round+"-comments.md")
			if comments, err := os.ReadFile(reviewPath); err == nil && strings.TrimSpace(string(comments)) != "" {
				prompt = prompt + "\n\n## Prior Review Comments (round r" + round + ")\nAddress every blocking issue below. The path-fenced code blocks you output in §3 must land the fixes in the actual source files (not just described in prose).\n\n" + string(comments)
			}
		}
	}
	result := runner.Execute(
		"impl",
		prompt,
		inputFiles,
		[]string{filepath.Join(cfg.FeatureDir, "impl-output.md")},
		cfg.RepoDir,
		cfg.ModelName,
		cfg.LLMStream,
		promptVersion,
		cfg.BudgetTracker,
	)

	// Post-hook: extract code blocks from impl-output.md and write to actual source files
	if result.Success {
		implOutputPath := filepath.Join(cfg.FeatureDir, "impl-output.md")
		data, err := os.ReadFile(implOutputPath)
		if err == nil {
			blocks := extractCodeBlocks(string(data))
			if len(blocks) > 0 {
				if err := writeCodeBlocks(cfg.RepoDir, blocks); err != nil {
					// Non-fatal: log warning but don't fail the step
					os.WriteFile(filepath.Join(cfg.FeatureDir, "impl-extract-errors.txt"), []byte(err.Error()), 0644)
				}
			}
		}
	}

	return result
}

package steps

import (
	"os"
	"path/filepath"

	"github.com/bavocado/tomato/pkg/model"
	"github.com/bavocado/tomato/pkg/runner"
)

var ImplPrompt = `You are tomato's implementation engineer.

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
If writing code, include complete code blocks or unified diff-style snippets.

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
	result := runner.Execute(
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

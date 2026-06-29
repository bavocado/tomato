package steps

import (
	"os"
	"path/filepath"

	"github.com/bavocado/tomato/pkg/model"
	"github.com/bavocado/tomato/pkg/runner"
)

// RewriteArchPrompt regenerates architecture.md from the actual implementation
// rather than the original design intent (design §2.8). It runs as a post-hook
// after impl succeeds.
var RewriteArchPrompt = `You are tomato's architecture documenter.

Implementation has just completed and the code may have diverged from the original design. Rewrite architecture.md so it reflects the REAL architecture as actually implemented — not the idealized design.

Prior architecture (design intent):
{{.architecture.md}}

Implementation output (what was built):
{{.impl-output.md}}

Actual code diff:
{{.diff}}

Write the new architecture.md with this exact structure:

# Architecture
## 1. System Overview
## 2. Components and Responsibilities
## 3. Data Flow
Include a Mermaid flowchart if it aids understanding.
## 4. Interfaces and Contracts
## 5. Persistence / Files / State
## 6. Error Handling Strategy
## 7. Security and Privacy Considerations
## 8. Testing Strategy
## 9. Divergences from Original Design
List concrete places where the implementation differs from the prior architecture and why. If it matches, say so briefly.

Rules:
- Describe what the code actually does now, not what was planned.
- Be concrete: reference real packages, files, and functions where they exist.
- Keep it tight — this is a living reference, not a rewrite of the full design doc.
- Do not invent components that are not present in the implementation or diff.`

// RewriteArchitecture regenerates the root architecture.md to reflect the
// as-implemented architecture. It is invoked by the engine after a successful
// impl step (and after the design trio has been archived), so the root
// architecture.md always represents the latest truth (design §2.8).
//
// It reads the prior architecture.md plus the impl output and git diff, and
// overwrites architecture.md in place. runner.Execute reads inputs before
// writing outputs, so reading and writing the same path within one call is
// safe.
func RewriteArchitecture(cfg *StepConfig) *model.StepResult {
	diffText, err := getGitDiff(cfg.RepoDir)
	if err != nil {
		diffText = ""
	}

	// Base name "diff" matches the {{.diff}} token; keep it under .tomato/
	// (not git-tracked) and remove it after the step.
	diffPath := filepath.Join(cfg.RepoDir, ".tomato", "diff")
	os.MkdirAll(filepath.Dir(diffPath), 0755)
	os.WriteFile(diffPath, []byte(diffText), 0644)
	defer os.Remove(diffPath)

	inputFiles := []string{
		filepath.Join(cfg.FeatureDir, "architecture.md"),
		filepath.Join(cfg.FeatureDir, "impl-output.md"),
		diffPath,
	}
	return runner.Execute(
		"rewrite-arch",
		RewriteArchPrompt,
		inputFiles,
		[]string{filepath.Join(cfg.FeatureDir, "architecture.md")},
		cfg.RepoDir,
		cfg.ModelName,
		cfg.LLMStream,
		cfg.PromptVersion,
		cfg.BudgetTracker,
	)
}

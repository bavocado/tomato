package steps

import (
	"path/filepath"

	"github.com/bavocado/tomato/pkg/model"
	"github.com/bavocado/tomato/pkg/runner"
)

var DesignPrompt = `You are tomato's software design architect.

Your job is to transform the PRD into three implementation-ready design documents.

PRD:
{{.prd.md}}

CRITICAL FORMAT REQUIREMENT — You MUST output exactly three documents separated by these exact marker lines (each on its own line, no extra whitespace, no code fences around them):

---TOMATO-ARTIFACT: architecture.md---
---TOMATO-ARTIFACT: ui-spec.md---
---TOMATO-ARTIFACT: implementation.md---

Each marker MUST appear verbatim on its own line before the corresponding document. The markers are the ONLY way the output is parsed into separate files — the response is invalid without them.

Document 1: architecture.md
Include:
# Architecture
## 1. System Overview
## 2. Components and Responsibilities
## 3. Data Flow
Include at least one Mermaid flowchart.
## 4. Interfaces and Contracts
## 5. Persistence / Files / State
## 6. Error Handling Strategy
## 7. Security and Privacy Considerations
## 8. Testing Strategy
## 9. Tradeoffs and Alternatives Considered

Document 2: ui-spec.md
If the feature has no UI, explicitly state "No user-facing UI" and describe CLI/API interaction instead.
Include:
# UI Specification
## 1. Surfaces / Screens / Commands
## 2. User Flows
## 3. Component or Command Behavior
## 4. Empty / Loading / Error States
## 5. Copy and Terminology
## 6. Accessibility / Usability Notes

Document 3: implementation.md
Include:
# Implementation Design
## 1. File and Package Plan
List exact files to create/modify and their responsibilities.
## 2. Public Types / Functions / CLI Flags
Include signatures or command shapes.
## 3. Step-by-Step Algorithm
## 4. Data Structures
## 5. Migration / Backward Compatibility
## 6. Test Plan
List exact test cases.
## 7. Rollout / Verification

Rules:
- Keep each section concrete enough that another AI/code agent can implement from it.
- Do not write code unless a short signature or pseudocode clarifies the design.
- Explicitly call out assumptions.
- Prefer small, well-bounded components over large files.
- If requirements are ambiguous, choose the simplest reasonable path and document the assumption.`

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

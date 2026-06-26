package steps

import (
	"path/filepath"

	"github.com/bavocado/tomato/pkg/model"
	"github.com/bavocado/tomato/pkg/runner"
)

// SpecPrompt is the prompt template for the spec step.
var SpecPrompt = `You are tomato's requirements analyst.

Your job is to turn the user's rough idea into a precise, implementation-ready PRD.

User's idea:
{{.idea.txt}}

Write a markdown PRD with this exact structure:

# PRD: <feature name>

## 1. Problem Statement
- What user problem are we solving?
- Why does it matter now?

## 2. Goals and Non-Goals
### Goals
- 3-5 measurable outcomes.

### Non-Goals
- Explicitly state what is out of scope.

## 3. Target Users
- Primary users
- Secondary users
- Assumptions about their workflow and constraints

## 4. User Stories
Use this format:
- As a <user>, I want <capability>, so that <benefit>.

## 5. Functional Requirements
Number each requirement as FR-001, FR-002, ...
Each requirement must be testable.

## 6. Acceptance Criteria
Use Given/When/Then format where possible.

## 7. Edge Cases and Error States
- Empty inputs
- Invalid inputs
- Permission/auth failures
- Network/API failures
- Partial success / retry cases

## 8. Data and State
- What data is read?
- What data is written?
- What state transitions happen?

## 9. Dependencies and Integrations
- External services
- APIs
- CLIs
- Filesystem/git interactions

## 10. Open Questions
List only questions that materially affect implementation.
If there are none, write "None".

Rules:
- Be concrete and concise.
- Do not invent business requirements beyond the input.
- If the input is ambiguous, choose a sensible default and record it under Assumptions.
- Prefer explicit requirements over vague goals.
- Avoid implementation details unless they affect requirements.`

func init() {
	Register("spec", runSpec)
}

func runSpec(cfg *StepConfig, args []string) *model.StepResult {
	// Input: user's rough idea (idea.txt); Output: generated PRD (prd.md)
	ideaPath := filepath.Join(cfg.FeatureDir, "idea.txt")
	prdPath := filepath.Join(cfg.FeatureDir, "prd.md")
	return runner.Execute(
		"spec",
		SpecPrompt,
		[]string{ideaPath},
		[]string{prdPath},
		cfg.RepoDir,
		cfg.ModelName,
		cfg.LLMStream,
		cfg.PromptVersion,
		cfg.BudgetTracker,
	)
}
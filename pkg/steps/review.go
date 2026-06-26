package steps

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/bavocado/tomato/pkg/model"
	"github.com/bavocado/tomato/pkg/runner"
)

var ReviewPrompt = `You are tomato's senior code reviewer.

Review the implementation against the design documents. Your goal is to identify correctness, safety, maintainability, and spec-compliance issues — not stylistic preferences.

Architecture:
{{.architecture.md}}

Implementation Plan:
{{.implementation.md}}

Code diff / implementation output:
{{.diff}}

Severity definitions:
- blocking: Must be fixed before merge. Examples: compile failure, data loss, security issue, broken requirement, incorrect state transition, missing required behavior.
- major: Should be fixed soon but does not block this PR. Examples: weak test coverage, confusing API, maintainability concern.
- minor: Nice-to-have improvement. Examples: naming, comments, small cleanup.

Output MUST start with a JSON object and then a markdown summary.
The JSON object must match this schema exactly:

{
  "comments": [
    {
      "file": "path/to/file.go",
      "line": 123,
      "severity": "blocking",
      "message": "Specific issue and why it matters",
      "suggestion": "Concrete fix suggestion"
    }
  ],
  "summary": "One-paragraph review summary",
  "has_blocking": true
}

Then append:

# Review Summary
## Blocking Issues
## Major Issues
## Minor Issues
## Positive Notes
## Final Recommendation
Write one of: APPROVE, REQUEST_CHANGES.

Rules:
- Only mark issues as blocking if they are truly merge-blocking.
- Every comment must be actionable and tied to a file when possible.
- Avoid speculative issues; if uncertain, mark as major or minor, not blocking.
- If there are no issues in a category, write "None".
- Do not request large refactors unless required by the spec.`

func init() {
	Register("review", runReview)
}

func runReview(cfg *StepConfig, args []string) *model.StepResult {
	outputDir := filepath.Join(cfg.FeatureDir, "reviews")
	round := "r1"
	if len(args) > 0 {
		round = args[0]
	}

	inputFiles := []string{
		filepath.Join(cfg.FeatureDir, "architecture.md"),
		filepath.Join(cfg.FeatureDir, "implementation.md"),
	}
	return runner.Execute(
		"review",
		ReviewPrompt,
		inputFiles,
		[]string{filepath.Join(outputDir, round+"-comments.md")},
		cfg.RepoDir,
		cfg.ModelName,
		cfg.LLMStream,
		cfg.PromptVersion,
		cfg.BudgetTracker,
	)
}

// HasBlockingIssues scans a review output file for blocking issues.
// It checks multiple signals:
// 1. JSON field "has_blocking": true
// 2. Any "severity" value containing "blocking" (handles spacing variations)
// 3. Plain text mention of "blocking" in markdown context
// Returns false only if none of these signals are found, or explicitly "has_blocking": false.
func HasBlockingIssues(reviewPath string) bool {
	data, err := os.ReadFile(reviewPath)
	if err != nil {
		return false
	}
	content := string(data)

	// Check for explicit has_blocking: false first (takes precedence)
	if strings.Contains(content, `"has_blocking":false`) || strings.Contains(content, `"has_blocking": false`) {
		return false
	}

	// Check for has_blocking: true
	if strings.Contains(content, `"has_blocking":true`) || strings.Contains(content, `"has_blocking": true`) {
		return true
	}

	// Check for severity value containing "blocking" (handles both "severity":"blocking" and "severity": "blocking")
	// Also catches plain text mentions of "blocking" in markdown
	if strings.Contains(content, "blocking") {
		return true
	}

	return false
}
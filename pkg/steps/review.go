package steps

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/bavocado/tomato/pkg/model"
	"github.com/bavocado/tomato/pkg/runner"
)

var ReviewPrompt = `You are a senior code reviewer. Review the following code diff against the design documents and identify issues.

Design documents:
- Architecture: {{.architecture.md}}
- Implementation Plan: {{.implementation.md}}

Code diff:
{{.diff}}

Classify each issue with severity: "blocking", "major", or "minor".
Output as JSON with this structure:
{
  "comments": [
    { "file": "...", "line": 0, "severity": "blocking|major|minor", "message": "..." }
  ],
  "summary": "..."
}

Then append a human-readable markdown summary below the JSON.`

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

// HasBlockingIssues scans a review output file for "blocking" severity.
func HasBlockingIssues(reviewPath string) bool {
	data, err := os.ReadFile(reviewPath)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), `"severity": "blocking"`)
}
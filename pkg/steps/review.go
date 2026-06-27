package steps

import (
	"encoding/json"
	"fmt"
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

	// Get real git diff and write to a temp file for the prompt template.
	// The file's base name must be "diff" so it matches the {{.diff}} token
	// in the prompt (token substitution keys by base name). It lives under
	// .tomato/ (not git-tracked) so it never leaks into the repo.
	diffText, err := getGitDiff(cfg.RepoDir)
	if err != nil {
		diffText = ""
	}

	diffPath := filepath.Join(cfg.RepoDir, ".tomato", "diff")
	os.MkdirAll(filepath.Dir(diffPath), 0755)
	os.WriteFile(diffPath, []byte(diffText), 0644)
	defer os.Remove(diffPath)

	inputFiles := []string{
		filepath.Join(cfg.FeatureDir, "architecture.md"),
		filepath.Join(cfg.FeatureDir, "implementation.md"),
		diffPath,
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

// HasBlockingIssues reports whether a review output contains blocking issues.
//
// The review step emits a JSON object followed by a markdown summary. The JSON
// object's "has_blocking" field is the authoritative signal; we parse it
// rather than substring-matching the word "blocking", because the markdown
// summary always contains a "## Blocking Issues" section header (even when
// empty) — bare substring matching would therefore always return true and the
// review_loop could never converge.
//
// If the JSON object is absent or unparseable, we fall back to an explicit
// "has_blocking: true|false" text marker. If no signal can be found at all we
// return false (and warn) so a malformed review does not loop forever; the raw
// file remains on disk for manual inspection.
func HasBlockingIssues(reviewPath string) bool {
	data, err := os.ReadFile(reviewPath)
	if err != nil {
		return false
	}
	has, ok := parseHasBlocking(string(data))
	if !ok {
		fmt.Fprintf(os.Stderr, "⚠  review output %q has no parseable blocking signal; assuming no blocking issues\n", reviewPath)
		return false
	}
	return has
}

// parseHasBlocking extracts the blocking signal from review output.
// Returns (value, true) when a signal is found, or (false, false) when none.
func parseHasBlocking(content string) (bool, bool) {
	// 1. Parse the leading JSON object, if present.
	if idx := strings.Index(content, "{"); idx >= 0 {
		dec := json.NewDecoder(strings.NewReader(content[idx:]))
		var v struct {
			HasBlocking *bool `json:"has_blocking"`
			Comments    []struct {
				Severity string `json:"severity"`
			} `json:"comments"`
		}
		if err := dec.Decode(&v); err == nil {
			if v.HasBlocking != nil {
				return *v.HasBlocking, true
			}
			// No explicit field: derive from comment severities.
			for _, c := range v.Comments {
				if strings.EqualFold(strings.TrimSpace(c.Severity), "blocking") {
					return true, true
				}
			}
			return false, true
		}
	}

	// 2. Fallback: scan for an explicit has_blocking marker in plain text.
	// Normalize whitespace so spacing variants collapse to one form.
	compact := strings.Join(strings.Fields(content), " ")
	switch {
	case strings.Contains(compact, "has_blocking: true"), strings.Contains(compact, `"has_blocking": true`):
		return true, true
	case strings.Contains(compact, "has_blocking: false"), strings.Contains(compact, `"has_blocking": false`):
		return false, true
	}
	return false, false
}

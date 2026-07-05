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
	commentsPath := filepath.Join(outputDir, round+"-comments.md")
	// Fold the round into the prompt version so each review round has a
	// distinct cache key. Without this, round 2/3 would hit round 1's cache
	// (same prompt + model) and review_loop could never converge.
	promptVersion := cfg.PromptVersion
	if len(args) > 0 {
		promptVersion = promptVersion + "-" + args[0]
	}
	result := runner.Execute(
		"review",
		ReviewPrompt,
		inputFiles,
		[]string{commentsPath},
		cfg.RepoDir,
		cfg.ModelName,
		cfg.LLMStream,
		promptVersion,
		cfg.BudgetTracker,
	)

	// The LLM emits a JSON object followed by a markdown summary. Strip the
	// JSON block from the human-readable .md file and persist it as a sidecar
	// .json so HasBlockingIssues can parse it without polluting the comments
	// file. Best-effort: a post-processing failure must not reverse a
	// successful review.
	if result.Success {
		stripJSONFromCommentsFile(commentsPath)
	}
	return result
}

// stripJSONFromCommentsFile reads the comments .md, extracts the leading JSON
// block (if any) into a sibling .json file, and rewrites the .md with only the
// remaining markdown. When there is no JSON block the file is left unchanged.
func stripJSONFromCommentsFile(commentsPath string) {
	data, err := os.ReadFile(commentsPath)
	if err != nil {
		return
	}
	content := string(data)
	jsonPart, mdPart := splitReviewJSON(content)
	if jsonPart == "" {
		return
	}
	// Persist the JSON sidecar.
	jsonPath := strings.TrimSuffix(commentsPath, ".md") + ".json"
	os.WriteFile(jsonPath, []byte(jsonPart), 0644)
	// Rewrite the .md with only the markdown summary.
	os.WriteFile(commentsPath, []byte(strings.TrimSpace(mdPart)+"\n"), 0644)
}

// splitReviewJSON separates the leading JSON object from the trailing markdown
// in a review response. The JSON may appear:
//   - as a bare leading object:  { ... }
//   - inside a ```json fenced block anywhere in the response
//
// Returns (json, markdown). If no JSON object is found, returns ("", content).
func splitReviewJSON(content string) (jsonPart, mdPart string) {
	// 1. Try a bare leading JSON object.
	trimmed := strings.TrimLeft(content, " \t\r\n")
	if strings.HasPrefix(trimmed, "{") {
		if j, md := extractBalancedJSON(trimmed); j != "" {
			return j, md
		}
	}

	// 2. Look for a ```json fenced block anywhere in the content.
	jsonPart, mdPart = extractFencedJSON(content)
	if jsonPart != "" {
		return jsonPart, mdPart
	}

	return "", content
}

// extractBalancedJSON finds the leading {...} by depth counting, tolerating
// braces inside strings. Returns ("", "") when no balanced object is found.
func extractBalancedJSON(s string) (json, rest string) {
	depth := 0
	inString := false
	escaped := false
	end := -1
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if ch == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		if ch == '{' {
			depth++
		} else if ch == '}' {
			depth--
			if depth == 0 {
				end = i + 1
				break
			}
		}
	}
	if end < 0 {
		return "", ""
	}
	return s[:end], strings.TrimLeft(s[end:], " \t\r\n")
}

// extractFencedJSON finds a ```json ... ``` fenced block, extracts its content
// as the JSON part, and returns the rest as markdown (with the fence removed).
func extractFencedJSON(content string) (json, md string) {
	const openFence = "```json"
	idx := strings.Index(content, openFence)
	if idx < 0 {
		return "", ""
	}
	afterOpen := content[idx+len(openFence):]
	closeIdx := strings.Index(afterOpen, "```")
	if closeIdx < 0 {
		return "", ""
	}
	jsonBody := strings.TrimSpace(afterOpen[:closeIdx])
	// Only accept it if it looks like a JSON object.
	if !strings.HasPrefix(jsonBody, "{") {
		return "", ""
	}
	// Reconstruct markdown: content before the fence + content after the
	// closing fence.
	before := content[:idx]
	after := afterOpen[closeIdx+3:]
	md = strings.TrimRight(before, " \t\r\n") + "\n\n" + strings.TrimLeft(after, " \t\r\n")
	return jsonBody, md
}

// HasBlockingIssues reports whether a review output contains blocking issues.
//
// The review step writes the JSON block (with has_blocking / comments) to a
// sibling .json sidecar and the markdown summary to the .md file. This function
// reads the .json sidecar for the authoritative signal. If the sidecar is
// absent (older runs wrote JSON inside the .md), it falls back to scanning the
// .md itself.
//
// If no signal can be found we return false (and warn) so a malformed review
// does not loop forever; the raw file remains on disk for manual inspection.
func HasBlockingIssues(reviewPath string) bool {
	// Prefer the JSON sidecar.
	jsonPath := strings.TrimSuffix(reviewPath, ".md") + ".json"
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		// Fallback: the .md file itself (covers older outputs and tests).
		data, err = os.ReadFile(reviewPath)
		if err != nil {
			return false
		}
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

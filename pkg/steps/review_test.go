package steps

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bavocado/tomato/pkg/runner"
)

func TestHasBlockingIssuesExactJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "review.md")
	os.WriteFile(path, []byte(`{"comments":[{"severity":"blocking","message":"bug"}],"has_blocking":true}`), 0644)

	if !HasBlockingIssues(path) {
		t.Error("expected blocking with exact JSON spacing")
	}
}

func TestHasBlockingIssuesCompactJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "review.md")
	os.WriteFile(path, []byte(`{"comments":[{"severity":"blocking","message":"bug"}]}`), 0644)

	if !HasBlockingIssues(path) {
		t.Error("expected blocking with compact JSON (no spaces)")
	}
}

func TestHasBlockingIssuesHasBlockingField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "review.md")
	os.WriteFile(path, []byte(`{"comments":[],"has_blocking":true,"summary":"issues found"}`), 0644)

	if !HasBlockingIssues(path) {
		t.Error("expected blocking via has_blocking field")
	}
}

func TestHasBlockingIssuesHasBlockingFieldFalse(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "review.md")
	os.WriteFile(path, []byte(`{"comments":[{"severity":"minor"}],"has_blocking":false}`), 0644)

	if HasBlockingIssues(path) {
		t.Error("expected no blocking when has_blocking is false")
	}
}

func TestHasBlockingIssuesOnlyMinor(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "review.md")
	os.WriteFile(path, []byte(`{"comments":[{"severity":"minor","message":"naming"}],"has_blocking":false}`), 0644)

	if HasBlockingIssues(path) {
		t.Error("expected no blocking for only minor issues")
	}
}

func TestHasBlockingIssuesMarkdownWithBlocking(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "review.md")
	content := `# Review Summary

## Blocking Issues
- File: main.go, Line: 42
  Severity: blocking
  Issue: nil pointer dereference

## Major Issues
None

has_blocking: true
`
	os.WriteFile(path, []byte(content), 0644)

	if !HasBlockingIssues(path) {
		t.Error("expected blocking from markdown with 'blocking' text")
	}
}

func TestHasBlockingIssuesNoBlocking(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "review.md")
	os.WriteFile(path, []byte(`{"comments":[{"severity":"major"}],"has_blocking":false,"summary":"looks good"}`), 0644)

	if HasBlockingIssues(path) {
		t.Error("expected no blocking")
	}
}

func TestHasBlockingIssuesMissingFile(t *testing.T) {
	if HasBlockingIssues("/nonexistent/path/review.md") {
		t.Error("expected false for missing file")
	}
}

// TestHasBlockingIssuesRealisticOutput mirrors the actual review step output:
// a JSON object followed by a markdown summary that ALWAYS contains a
// "## Blocking Issues" header (even when empty). The old substring matcher
// returned true for this header unconditionally, so review_loop could never
// converge. The parser must trust the JSON has_blocking field instead.
func TestHasBlockingIssuesRealisticOutput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "review.md")
	content := `{"comments":[{"severity":"minor","message":"rename"}],"summary":"looks fine","has_blocking":false}

# Review Summary
## Blocking Issues
None
## Major Issues
None
## Minor Issues
- rename a variable
## Positive Notes
Clean code.
## Final Recommendation
APPROVE
`
	os.WriteFile(path, []byte(content), 0644)

	if HasBlockingIssues(path) {
		t.Error("expected no blocking: has_blocking is false despite the 'Blocking Issues' header")
	}
}

// TestHasBlockingIssuesRealisticBlocking verifies a realistic blocking case
// (JSON with has_blocking:true + markdown) is detected.
func TestHasBlockingIssuesRealisticBlocking(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "review.md")
	content := `{"comments":[{"severity":"blocking","message":"nil deref"}],"summary":"blocking found","has_blocking":true}

# Review Summary
## Blocking Issues
- nil deref in main.go
`
	os.WriteFile(path, []byte(content), 0644)

	if !HasBlockingIssues(path) {
		t.Error("expected blocking when has_blocking is true")
	}
}

// TestHasBlockingIssuesUnparseableDefaultsFalse ensures a malformed review
// output does not loop review_loop forever — it defaults to no-blocking.
func TestHasBlockingIssuesUnparseableDefaultsFalse(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "review.md")
	os.WriteFile(path, []byte("totally unstructured prose with no JSON at all"), 0644)

	if HasBlockingIssues(path) {
		t.Error("expected false for unparseable review output (avoid infinite loop)")
	}
}

// TestRunReviewStripsJSONFromCommentsFile verifies that the review output
// written to r<N>-comments.md does NOT contain the JSON object block — only
// the markdown summary. The JSON is parsed for HasBlockingIssues but must not
// pollute the human-readable comments file.
func TestRunReviewStripsJSONFromCommentsFile(t *testing.T) {
	repo := t.TempDir()
	featureDir := filepath.Join(repo, "docs", "specs", "f")
	os.MkdirAll(featureDir, 0755)

	// Provide minimal input files so buildMessages doesn't error.
	os.WriteFile(filepath.Join(featureDir, "architecture.md"), []byte("# arch"), 0644)
	os.WriteFile(filepath.Join(featureDir, "implementation.md"), []byte("# impl"), 0644)

	// Mock LLM that returns JSON + markdown, exactly as the prompt asks.
	mockResponse := `{
  "comments": [
    {"file": "main.go", "line": 10, "severity": "blocking", "message": "bug", "suggestion": "fix it"}
  ],
  "summary": "found a bug",
  "has_blocking": true
}

# Review Summary
## Blocking Issues
- main.go:10 — bug
## Major Issues
None
## Minor Issues
None
## Positive Notes
Clean structure.
## Final Recommendation
REQUEST_CHANGES
`

	cfg := &StepConfig{
		RepoDir:    repo,
		FeatureDir: featureDir,
		Feature:    "f",
		ModelName:  "glm/glm-5.2",
		LLMStream: func(messages []runner.Message, onChunk func(string)) error {
			onChunk(mockResponse)
			return nil
		},
	}

	result := runReview(cfg, []string{"r1"})
	if !result.Success {
		t.Fatalf("runReview failed: %s", result.Error)
	}

	commentsPath := filepath.Join(featureDir, "reviews", "r1-comments.md")
	data, err := os.ReadFile(commentsPath)
	if err != nil {
		t.Fatalf("comments file not written: %v", err)
	}
	content := string(data)

	// The .md file must NOT contain the JSON block.
	if strings.Contains(content, `"has_blocking"`) || strings.Contains(content, `"comments"`) {
		t.Errorf("comments .md should not contain JSON, got %q", content)
	}
	// The .md file MUST contain the markdown summary.
	if !strings.Contains(content, "# Review Summary") {
		t.Errorf("comments .md should contain markdown summary, got %q", content)
	}
	if !strings.Contains(content, "REQUEST_CHANGES") {
		t.Errorf("comments .md should contain final recommendation, got %q", content)
	}

	// HasBlockingIssues must still work (reads the JSON sidecar).
	if !HasBlockingIssues(commentsPath) {
		t.Error("expected HasBlockingIssues to return true")
	}
}

// TestRunReviewStripsFencedJSON verifies that JSON wrapped in a ```json code
// fence (as LLMs often emit) is stripped from the .md and written to the
// .json sidecar. This is the real-world review output shape.
func TestRunReviewStripsFencedJSON(t *testing.T) {
	repo := t.TempDir()
	featureDir := filepath.Join(repo, "docs", "specs", "f")
	os.MkdirAll(featureDir, 0755)
	os.WriteFile(filepath.Join(featureDir, "architecture.md"), []byte("# arch"), 0644)
	os.WriteFile(filepath.Join(featureDir, "implementation.md"), []byte("# impl"), 0644)

	mockResponse := `核查完毕。关键事实已验证。

` + "```json" + `
{
  "comments": [
    {"file": "main.go", "line": 1, "severity": "blocking", "message": "bug"}
  ],
  "summary": "found a bug",
  "has_blocking": true
}
` + "```" + `

# Review Summary
## Blocking Issues
- main.go:1 — bug
## Final Recommendation
REQUEST_CHANGES
`

	cfg := &StepConfig{
		RepoDir:    repo,
		FeatureDir: featureDir,
		Feature:    "f",
		ModelName:  "glm/glm-5.2",
		LLMStream: func(messages []runner.Message, onChunk func(string)) error {
			onChunk(mockResponse)
			return nil
		},
	}

	result := runReview(cfg, []string{"r1"})
	if !result.Success {
		t.Fatalf("runReview failed: %s", result.Error)
	}

	commentsPath := filepath.Join(featureDir, "reviews", "r1-comments.md")
	data, _ := os.ReadFile(commentsPath)
	content := string(data)

	if strings.Contains(content, `"has_blocking"`) || strings.Contains(content, "```json") {
		t.Errorf("comments .md should not contain fenced JSON, got %q", content)
	}
	if !strings.Contains(content, "# Review Summary") {
		t.Errorf("comments .md should contain markdown summary, got %q", content)
	}
	// JSON sidecar should exist and be parseable.
	jsonPath := filepath.Join(featureDir, "reviews", "r1-comments.json")
	jdata, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("json sidecar not written: %v", err)
	}
	if !strings.Contains(string(jdata), `"has_blocking": true`) {
		t.Errorf("json sidecar missing has_blocking, got %q", string(jdata))
	}
	if !HasBlockingIssues(commentsPath) {
		t.Error("expected HasBlockingIssues to return true from sidecar")
	}
}

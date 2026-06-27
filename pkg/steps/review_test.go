package steps

import (
	"os"
	"path/filepath"
	"testing"
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

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
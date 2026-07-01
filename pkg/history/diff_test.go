package history

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeArtifact places an artifact snapshot for a run so Diff can read it.
func writeArtifact(t *testing.T, repoDir, runID, artifact, content string) {
	t.Helper()
	dir := filepath.Join(repoDir, ".tomato", "runs", runID, "artifacts")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, artifact), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestDiffReportsChanges(t *testing.T) {
	dir := t.TempDir()
	writeArtifact(t, dir, "run-a", "prd.md", "line one\nline two\nline three\n")
	writeArtifact(t, dir, "run-b", "prd.md", "line one\nline TWO\nline three\n")

	out, err := Diff(dir, "run-a", "run-b", "prd.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "line two") || !strings.Contains(out, "line TWO") {
		t.Errorf("expected diff to show both old and new lines:\n%s", out)
	}
	if !strings.Contains(out, "- line two") {
		t.Errorf("expected removed line marker, got:\n%s", out)
	}
	if !strings.Contains(out, "+ line TWO") {
		t.Errorf("expected added line marker, got:\n%s", out)
	}
}

func TestDiffNoChanges(t *testing.T) {
	dir := t.TempDir()
	writeArtifact(t, dir, "run-a", "prd.md", "identical content\n")
	writeArtifact(t, dir, "run-b", "prd.md", "identical content\n")

	out, err := Diff(dir, "run-a", "run-b", "prd.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "no changes") {
		t.Errorf("expected no-changes message, got:\n%s", out)
	}
}

// TestDiffPreservesBlankLines guards against a regression where the diff
// skipped empty lines (a removed blank line would silently vanish instead of
// being reported as a change).
func TestDiffPreservesBlankLines(t *testing.T) {
	dir := t.TempDir()
	writeArtifact(t, dir, "run-a", "prd.md", "keep\n\nremoved-blank-after\n")
	writeArtifact(t, dir, "run-b", "prd.md", "keep\nremoved-blank-after\n")

	out, err := Diff(dir, "run-a", "run-b", "prd.md")
	if err != nil {
		t.Fatal(err)
	}
	// The blank line was removed from run-b, so the diff must report a change
	// rather than claim "no changes".
	if strings.Contains(out, "no changes") {
		t.Errorf("blank-line removal must be reported as a change, got 'no changes':\n%s", out)
	}
}

func TestDiffMissingArtifactErrors(t *testing.T) {
	dir := t.TempDir()
	writeArtifact(t, dir, "run-a", "prd.md", "content\n")
	// run-b has no snapshot at all.

	_, err := Diff(dir, "run-a", "run-b", "prd.md")
	if err == nil {
		t.Fatal("expected error when an artifact snapshot is missing")
	}
}

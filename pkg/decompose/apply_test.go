package decompose

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSource(t *testing.T, dir string) string {
	t.Helper()
	p := filepath.Join(dir, "source.md")
	if err := os.WriteFile(p, []byte("# Parent design\nbody"), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestApplyCreatesSubFeatureDirs(t *testing.T) {
	specs := t.TempDir()
	src := writeSource(t, specs)
	d := &Decomposition{Features: []SubFeature{validFeature("F-001")}}
	if err := Apply(d, "main", specs, src, false); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	dir := filepath.Join(specs, "main-f001")
	for _, name := range []string{"idea.md", "parent-context.md", "parent.json"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("expected %s to exist: %v", name, err)
		}
	}
	idea, _ := os.ReadFile(filepath.Join(dir, "idea.md"))
	if !strings.Contains(string(idea), "# t") {
		t.Errorf("idea.md missing title, got: %s", idea)
	}
	if !strings.Contains(string(idea), "g") {
		t.Errorf("idea.md missing goal, got: %s", idea)
	}
	ctx, _ := os.ReadFile(filepath.Join(dir, "parent-context.md"))
	if !strings.Contains(string(ctx), "# Parent design") {
		t.Errorf("parent-context.md missing full source doc, got: %s", ctx)
	}
}

func TestApplyDirExistsNoForce(t *testing.T) {
	specs := t.TempDir()
	src := writeSource(t, specs)
	dir := filepath.Join(specs, "main-f001")
	os.MkdirAll(dir, 0755)
	d := &Decomposition{Features: []SubFeature{validFeature("F-001")}}
	err := Apply(d, "main", specs, src, false)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected already-exists error, got %v", err)
	}
}

func TestApplyDirExistsWithForce(t *testing.T) {
	specs := t.TempDir()
	src := writeSource(t, specs)
	dir := filepath.Join(specs, "main-f001")
	os.MkdirAll(dir, 0755)
	d := &Decomposition{Features: []SubFeature{validFeature("F-001")}}
	if err := Apply(d, "main", specs, src, true); err != nil {
		t.Fatalf("Apply with --force failed: %v", err)
	}
}

func TestSubFeatureName(t *testing.T) {
	if got := subFeatureName("main", "F-001"); got != "main-f001" {
		t.Errorf("expected main-f001, got %s", got)
	}
}
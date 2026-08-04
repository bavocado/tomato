package engine

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bavocado/tomato/pkg/model"
	"github.com/bavocado/tomato/pkg/steps"
)

func TestFormatSummaryWithPR(t *testing.T) {
	prRef := steps.PRRef{
		PRRef:  "bavocado/grape#9",
		URL:    "https://github.com/bavocado/grape/pull/9",
		Branch: "tomato/grape-f003",
	}
	out := formatSummary("default", "grape-f003", prRef, 12*time.Minute+34*time.Second)

	checks := map[string]string{
		"workflow":  `workflow "default" complete: grape-f003`,
		"PR url":    "PR:     https://github.com/bavocado/grape/pull/9",
		"title":     "title:  feat: grape-f003",
		"branch":    "branch: tomato/grape-f003",
		"time":      "time:   12m34s",
	}
	for name, want := range checks {
		if !strings.Contains(out, want) {
			t.Errorf("%s: summary missing %q\ngot:\n%s", name, want, out)
		}
	}
}

func TestFormatSummaryHourDuration(t *testing.T) {
	out := formatSummary("default", "grape-f010", steps.PRRef{URL: "u", Branch: "b"},
		1*time.Hour+2*time.Minute+3*time.Second)
	if !strings.Contains(out, "time:   1h2m3s") {
		t.Errorf("expected 1h2m3s duration, got:\n%s", out)
	}
}

func TestFormatSummaryWithoutPR(t *testing.T) {
	// No pr step / no pr.json -> zero PRRef. Must not panic, degrades gracefully.
	out := formatSummary("default", "grape-f099", steps.PRRef{}, 5*time.Second)
	if !strings.Contains(out, "PR:     (none)") {
		t.Errorf("expected PR (none), got:\n%s", out)
	}
	if !strings.Contains(out, "branch: (unknown)") {
		t.Errorf("expected branch (unknown), got:\n%s", out)
	}
	if !strings.Contains(out, "title:  feat: grape-f099") {
		t.Errorf("expected title feat: grape-f099, got:\n%s", out)
	}
}

// TestRunWithOptionsPrintsSummary runs a real workflow (in-process) whose step
// writes pr.json, captures stdout, and asserts the end-of-run summary is
// printed with PR URL, title, branch, and time.
func TestRunWithOptionsPrintsSummary(t *testing.T) {
	dir := t.TempDir()
	initGitRepoForEngine(t, dir)

	// Bare remote so SwitchBackToMain's origin/main sync works.
	bare := t.TempDir()
	runGitCmd2(t, bare, "init", "--bare")
	runGitCmd2(t, dir, "remote", "add", "origin", bare)
	runGitCmd2(t, dir, "push", "-u", "origin", "main")

	yamlContent := `
workflows:
  default:
    steps: [alpha]
`
	os.WriteFile(filepath.Join(dir, "tomato.yaml"), []byte(yamlContent), 0644)
	os.MkdirAll(filepath.Join(dir, ".tomato", "runs"), 0755)

	eng, err := NewEngine(dir)
	if err != nil {
		t.Fatal(err)
	}

	steps.Register("alpha", func(cfg *steps.StepConfig, _ []string) *model.StepResult {
		os.MkdirAll(cfg.FeatureDir, 0755)
		// Simulate the pr step writing pr.json on the feature branch.
		_ = steps.WritePRRef(cfg.FeatureDir, steps.PRRef{
			PRRef:  "bavocado/grape#9",
			URL:    "https://github.com/bavocado/grape/pull/9",
			Branch: "tomato/grape-f003",
		})
		return &model.StepResult{StepName: "alpha", Success: true, RunID: "r-alpha"}
	})

	// Capture stdout (fmt.Printf from RunWithOptions + summary).
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	runErr := eng.RunWithOptions("default", RunOptions{})
	w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)

	if runErr != nil {
		t.Fatalf("workflow failed: %v", runErr)
	}
	s := string(out)
	for _, want := range []string{
		`workflow "default" complete:`,
		"PR:     https://github.com/bavocado/grape/pull/9",
		"title:  feat:",
		"branch: tomato/grape-f003",
		"time:",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("summary missing %q\nfull output:\n%s", want, s)
		}
	}
}

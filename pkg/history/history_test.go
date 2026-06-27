package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bavocado/tomato/pkg/model"
)

func TestListRuns(t *testing.T) {
	dir := t.TempDir()

	runsDir := filepath.Join(dir, ".tomato", "runs", "2026-06-24-test123")
	os.MkdirAll(runsDir, 0755)

	meta := model.RunMeta{
		RunID:     "2026-06-24-test123",
		StepName:  "design",
		Success:   true,
		ModelUsed: "gpt-5",
		TokensIn:  100,
		TokensOut: 50,
	}
	data, _ := json.Marshal(meta)
	os.WriteFile(filepath.Join(runsDir, "meta.json"), data, 0644)

	runs, err := List(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(runs) != 1 {
		t.Errorf("expected 1 run, got %d", len(runs))
	}

	if runs[0].StepName != "design" {
		t.Errorf("expected step name 'design', got '%s'", runs[0].StepName)
	}
}

func TestListEmpty(t *testing.T) {
	dir := t.TempDir()
	runs, err := List(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 0 {
		t.Errorf("expected 0 runs, got %d", len(runs))
	}
}

func TestShowRun(t *testing.T) {
	dir := t.TempDir()

	runsDir := filepath.Join(dir, ".tomato", "runs", "run-abc")
	os.MkdirAll(runsDir, 0755)

	meta := model.RunMeta{
		RunID:      "run-abc",
		StepName:   "spec",
		Success:    true,
		ModelUsed:  "gpt-5",
		TokensIn:   100,
		TokensOut:  200,
		DurationMs: 1500,
	}
	data, _ := json.Marshal(meta)
	os.WriteFile(filepath.Join(runsDir, "meta.json"), data, 0644)

	output, err := Show(dir, "run-abc")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(output, "spec") {
		t.Errorf("output should contain 'spec': %s", output)
	}
	if !strings.Contains(output, "gpt-5") {
		t.Errorf("output should contain 'gpt-5': %s", output)
	}
	if !strings.Contains(output, "1500") {
		t.Errorf("output should contain '1500': %s", output)
	}
}

func TestShowNonexistent(t *testing.T) {
	dir := t.TempDir()
	_, err := Show(dir, "nonexistent-run")
	if err == nil {
		t.Error("expected error for nonexistent run")
	}
}

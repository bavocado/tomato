package cost

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bavocado/tomato/pkg/model"
)

func TestComputeEmpty(t *testing.T) {
	dir := t.TempDir()
	s, err := Compute(dir)
	if err != nil {
		t.Fatal(err)
	}

	if s.RunCount != 0 {
		t.Errorf("expected 0 runs, got %d", s.RunCount)
	}
}

func TestComputeWithRuns(t *testing.T) {
	dir := t.TempDir()

	// Create two run logs
	writeMeta(dir, "run-1", model.RunMeta{RunID: "run-1", StepName: "spec", TokensIn: 100, TokensOut: 50, Success: true})
	writeMeta(dir, "run-2", model.RunMeta{RunID: "run-2", StepName: "design", TokensIn: 200, TokensOut: 100, Success: true})

	s, err := Compute(dir)
	if err != nil {
		t.Fatal(err)
	}

	if s.RunCount != 2 {
		t.Errorf("expected 2 runs, got %d", s.RunCount)
	}
	if s.TotalIn != 300 {
		t.Errorf("expected 300 total in, got %d", s.TotalIn)
	}
	if s.TotalOut != 150 {
		t.Errorf("expected 150 total out, got %d", s.TotalOut)
	}
}

func TestComputeByStep(t *testing.T) {
	dir := t.TempDir()

	writeMeta(dir, "r1", model.RunMeta{RunID: "r1", StepName: "spec", TokensIn: 100, TokensOut: 50, Success: true})
	writeMeta(dir, "r2", model.RunMeta{RunID: "r2", StepName: "design", TokensIn: 200, TokensOut: 100, Success: true})
	writeMeta(dir, "r3", model.RunMeta{RunID: "r3", StepName: "spec", TokensIn: 150, TokensOut: 75, Success: true})

	s, err := Compute(dir)
	if err != nil {
		t.Fatal(err)
	}

	specStep := s.ByStep["spec"]
	if specStep.Count != 2 {
		t.Errorf("expected 2 spec runs, got %d", specStep.Count)
	}
	if specStep.TokensIn != 250 {
		t.Errorf("expected 250 spec tokens in, got %d", specStep.TokensIn)
	}
}

func TestFormatOutput(t *testing.T) {
	s := &Summary{
		TotalIn:  300,
		TotalOut: 150,
		RunCount: 2,
		ByStep: map[string]StepCost{
			"spec":   {Count: 1, TokensIn: 100, TokensOut: 50},
			"design": {Count: 1, TokensIn: 200, TokensOut: 100},
		},
	}

	output := s.Format()
	if !strings.Contains(output, "300") {
		t.Error("output should contain total token count")
	}
	if !strings.Contains(output, "spec") {
		t.Error("output should contain step name 'spec'")
	}
}

func writeMeta(dir, runID string, meta model.RunMeta) {
	runsDir := filepath.Join(dir, ".tomato", "runs", runID)
	os.MkdirAll(runsDir, 0755)
	data, _ := json.Marshal(meta)
	os.WriteFile(filepath.Join(runsDir, "meta.json"), data, 0644)
}

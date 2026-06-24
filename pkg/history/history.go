package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bavocado/tomato/pkg/model"
)

// List returns all run meta entries, newest first.
func List(repoDir string) ([]model.RunMeta, error) {
	runsDir := filepath.Join(repoDir, ".tomato", "runs")
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var runs []model.RunMeta
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		metaPath := filepath.Join(runsDir, entry.Name(), "meta.json")
		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		var meta model.RunMeta
		if err := json.Unmarshal(data, &meta); err != nil {
			continue
		}
		runs = append(runs, meta)
	}

	sort.Slice(runs, func(i, j int) bool {
		return runs[i].StartedAt.After(runs[j].StartedAt)
	})

	return runs, nil
}

// Show returns a human-readable description of a single run.
func Show(repoDir, runID string) (string, error) {
	metaPath := filepath.Join(repoDir, ".tomato", "runs", runID, "meta.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return "", err
	}

	var meta model.RunMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return "", err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Run:      %s\n", meta.RunID)
	fmt.Fprintf(&b, "Step:     %s\n", meta.StepName)
	fmt.Fprintf(&b, "Model:    %s\n", meta.ModelUsed)
	fmt.Fprintf(&b, "Status:   ")
	if meta.Success {
		fmt.Fprintf(&b, "✓ success\n")
	} else {
		fmt.Fprintf(&b, "✗ failed: %s\n", meta.Error)
	}
	fmt.Fprintf(&b, "Duration: %d ms\n", meta.DurationMs)
	fmt.Fprintf(&b, "Tokens:   %d in / %d out\n", meta.TokensIn, meta.TokensOut)
	if meta.CacheHit {
		fmt.Fprintf(&b, "Cache:    hit\n")
	}

	return b.String(), nil
}
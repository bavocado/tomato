package steps

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bavocado/tomato/pkg/model"
)

func init() {
	Register("task", runTask)
}

func runTask(cfg *StepConfig, args []string) *model.StepResult {
	adapterBin := GlobalAdapterBin
	if adapterBin == "" {
		return &model.StepResult{
			Success: false,
			Error:   "no adapter configured for 'task' role. Set TOMATO_ADAPTER_BIN env or configure in tomato.yaml",
		}
	}

	prdContent := readFileOrEmpty(filepath.Join(cfg.FeatureDir, "prd.md"))
	archContent := readFileOrEmpty(filepath.Join(cfg.FeatureDir, "architecture.md"))

	input := struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Status      string `json:"status"`
	}{
		Title:       cfg.Feature,
		Description: fmt.Sprintf("# %s\n\n## PRD\n%s\n\n## Architecture\n%s\n\n", cfg.Feature, prdContent, archContent),
		Status:      "specified",
	}
	inputJSON, _ := json.Marshal(input)

	cmd := exec.Command(adapterBin, "create-task")
	cmd.Dir = cfg.RepoDir
	cmd.Env = os.Environ()
	cmd.Stdin = strings.NewReader(string(inputJSON))

	output, err := cmd.Output()
	if err != nil {
		return &model.StepResult{Success: false, Error: fmt.Sprintf("adapter create-task failed: %v", err)}
	}

	var taskResult struct {
		TaskRef string `json:"task_ref"`
		URL     string `json:"url"`
	}
	if err := json.Unmarshal(output, &taskResult); err != nil {
		return &model.StepResult{Success: false, Error: fmt.Sprintf("invalid adapter output: %v", err)}
	}

	return &model.StepResult{StepName: "task", Success: true}
}

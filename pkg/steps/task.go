package steps

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/bavocado/tomato/pkg/adapter"
	"github.com/bavocado/tomato/pkg/model"
)

func init() {
	Register("task", runTask)
}

func runTask(cfg *StepConfig, args []string) *model.StepResult {
	br := cfg.Adapters.For("task")
	if br == nil {
		return &model.StepResult{
			Success: false,
			Error:   "no adapter configured for 'task' role. Configure adapters+roles in tomato.yaml or set TOMATO_ADAPTER_BIN",
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

	output, err := br.Execute(adapter.CmdCreateTask, string(inputJSON), nil)
	if err != nil {
		return &model.StepResult{Success: false, Error: fmt.Sprintf("adapter create-task failed: %v", err)}
	}

	var taskResult struct {
		TaskRef string `json:"task_ref"`
		URL     string `json:"url"`
	}
	if err := json.Unmarshal([]byte(output), &taskResult); err != nil {
		return &model.StepResult{Success: false, Error: fmt.Sprintf("invalid adapter output: %v", err)}
	}

	// Persist the task ref so the status lifecycle hook can update it later.
	if err := WriteTaskRef(cfg.FeatureDir, TaskRef{TaskRef: taskResult.TaskRef, URL: taskResult.URL}); err != nil {
		return &model.StepResult{Success: false, Error: fmt.Sprintf("writing task.json: %v", err)}
	}

	return &model.StepResult{StepName: "task", Success: true}
}

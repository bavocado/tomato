package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// WorkflowState records resumable workflow progress.
type WorkflowState struct {
	Workflow       string    `json:"workflow"`
	Feature        string    `json:"feature"`
	CurrentStep    string    `json:"current_step"`
	FailedStep     string    `json:"failed_step,omitempty"`
	CompletedSteps []string  `json:"completed_steps"`
	LastRunID      string    `json:"last_run_id,omitempty"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// Path returns the on-disk state file path for a workflow/feature pair.
func Path(repoDir, workflow, feature string) string {
	name := sanitize(workflow) + "-" + sanitize(feature) + ".json"
	return filepath.Join(repoDir, ".tomato", "state", name)
}

// Save writes workflow state to .tomato/state/.
func Save(repoDir string, s WorkflowState) error {
	s.UpdatedAt = time.Now().UTC()
	path := Path(repoDir, s.Workflow, s.Feature)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// Load reads workflow state for workflow/feature.
func Load(repoDir, workflow, feature string) (*WorkflowState, error) {
	path := Path(repoDir, workflow, feature)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("loading workflow state %s: %w", path, err)
	}
	var s WorkflowState
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// Clear removes workflow state for workflow/feature.
func Clear(repoDir, workflow, feature string) error {
	path := Path(repoDir, workflow, feature)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func sanitize(s string) string {
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, "\\", "-")
	s = strings.ReplaceAll(s, " ", "-")
	if s == "" {
		return "default"
	}
	return s
}
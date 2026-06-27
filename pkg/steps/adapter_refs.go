package steps

import (
	"encoding/json"
	"path/filepath"
)

// PRRef is the machine-readable PR reference written by the pr step and read
// back by review_loop. It lives alongside the human-readable pr.md so that
// later adapter calls (comment-pr, update-pr, mark-pr-ready/failed) know which
// PR to operate on — steps communicate only through files.
type PRRef struct {
	PRRef  string `json:"pr_ref"`
	URL    string `json:"url"`
	Branch string `json:"branch"`
}

// TaskRef is the machine-readable task reference written by the task step and
// read back by the status lifecycle hook (update-status).
type TaskRef struct {
	TaskRef string `json:"task_ref"`
	URL     string `json:"url"`
}

// WritePRRef persists a PRRef to <featureDir>/pr.json.
func WritePRRef(featureDir string, ref PRRef) error {
	data, err := json.MarshalIndent(ref, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(featureDir, "pr.json"), string(data))
}

// ReadPRRef reads <featureDir>/pr.json. A missing or malformed file yields a
// zero PRRef and no error — callers treat an empty PRRef as "unknown".
func ReadPRRef(featureDir string) PRRef {
	var ref PRRef
	_ = json.Unmarshal([]byte(readFileOrEmpty(filepath.Join(featureDir, "pr.json"))), &ref)
	return ref
}

// WriteTaskRef persists a TaskRef to <featureDir>/task.json.
func WriteTaskRef(featureDir string, ref TaskRef) error {
	data, err := json.MarshalIndent(ref, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(featureDir, "task.json"), string(data))
}

// ReadTaskRef reads <featureDir>/task.json. A missing or malformed file yields
// a zero TaskRef and no error.
func ReadTaskRef(featureDir string) TaskRef {
	var ref TaskRef
	_ = json.Unmarshal([]byte(readFileOrEmpty(filepath.Join(featureDir, "task.json"))), &ref)
	return ref
}

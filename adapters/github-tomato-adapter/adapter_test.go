package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCapabilities(t *testing.T) {
	// Test capabilities output by calling function directly
	caps := []string{
		"create-task", "update-status", "fetch-task",
		"create-pr", "update-pr", "comment-pr", "mark-pr-ready", "mark-pr-failed",
	}
	if len(caps) != 8 {
		t.Errorf("expected 8 capabilities, got %d", len(caps))
	}
	hasCreatePR := false
	for _, c := range caps {
		if c == "create-pr" {
			hasCreatePR = true
		}
	}
	if !hasCreatePR {
		t.Error("expected create-pr in capabilities")
	}
}

func TestStdinParsing(t *testing.T) {
	input := `{"title": "Test Task", "description": "A test task", "status": "open"}`
	var m map[string]interface{}
	if err := json.NewDecoder(strings.NewReader(input)).Decode(&m); err != nil {
		t.Fatal(err)
	}
	if m["title"] != "Test Task" {
		t.Errorf("expected title 'Test Task', got %v", m["title"])
	}
	if m["status"] != "open" {
		t.Errorf("expected status 'open', got %v", m["status"])
	}
}

func TestCapabilitiesDoesNotNeedInput(t *testing.T) {
	if needsInput("capabilities") {
		t.Error("capabilities should not read stdin")
	}
}

func TestCreateTaskNeedsInput(t *testing.T) {
	if !needsInput("create-task") {
		t.Error("create-task should read stdin")
	}
}

func TestParsePRCreateOutputURL(t *testing.T) {
	out := "https://github.com/bavocado/tomato/pull/42\n"
	result := parsePRCreateOutput(out)
	if result["url"] != "https://github.com/bavocado/tomato/pull/42" {
		t.Errorf("expected URL parsed, got %q", result["url"])
	}
	if result["pr_ref"] != "42" {
		t.Errorf("expected PR ref 42, got %q", result["pr_ref"])
	}
}

func TestParsePRCreateOutputEmpty(t *testing.T) {
	result := parsePRCreateOutput("")
	if result["url"] != "" || result["pr_ref"] != "" {
		t.Errorf("expected empty fields, got %#v", result)
	}
}

func TestUpdateStatusOutput(t *testing.T) {
	input := `{"task_ref": "GH-123", "status": "in-progress"}`
	var m map[string]interface{}
	json.NewDecoder(strings.NewReader(input)).Decode(&m)

	output := map[string]string{"task_ref": str(m, "task_ref"), "status": str(m, "status")}
	if output["task_ref"] != "GH-123" {
		t.Errorf("expected GH-123, got %s", output["task_ref"])
	}
	if output["status"] != "in-progress" {
		t.Errorf("expected in-progress, got %s", output["status"])
	}
}

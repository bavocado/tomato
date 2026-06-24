package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseDefaultWorkflow(t *testing.T) {
	yamlContent := `
models:
  default: deepseek/deepseek-4pro
  steps:
    spec: openai/gpt-5
    design: openai/gpt-5
    impl: glm/glm-5.2

budget:
  mode: balanced
  global_per_run: 300000

workflows:
  default:
    steps:
      - spec
      - design
      - impl
      - pr
      - review_loop: { max_rounds: 2, on_fail: stop }
      - test
      - task
`
	cfg, err := Parse([]byte(yamlContent))
	if err != nil {
		t.Fatal(err)
	}

	if len(cfg.Workflows) != 1 {
		t.Errorf("expected 1 workflow, got %d", len(cfg.Workflows))
	}
	wf, ok := cfg.Workflows["default"]
	if !ok {
		t.Fatal("default workflow not found")
	}
	if len(wf.Steps) != 7 {
		t.Errorf("expected 7 steps in default workflow, got %d", len(wf.Steps))
	}
}

func TestParseReviewLoopStep(t *testing.T) {
	yamlContent := `
workflows:
  test:
    steps:
      - review_loop: { max_rounds: 2, on_fail: stop }
`
	cfg, err := Parse([]byte(yamlContent))
	if err != nil {
		t.Fatal(err)
	}
	step := cfg.Workflows["test"].Steps[0]
	if !step.IsMetaStep {
		t.Error("expected review_loop to be a meta-step")
	}
	if step.MaxRounds != 2 {
		t.Errorf("expected max_rounds=2, got %d", step.MaxRounds)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := Default()
	if cfg.Models.Default != "deepseek/deepseek-4pro" {
		t.Errorf("expected default model deepseek/deepseek-4pro, got %s", cfg.Models.Default)
	}
	if _, ok := cfg.Workflows["default"]; !ok {
		t.Error("default workflow should exist")
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tomato.yaml")

	cfg := Default()
	if err := Save(cfg, path); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	// Verify key content is present
	content := string(data)
	if !strings.Contains(content, "openai/gpt-5") {
		t.Error("expected model config in saved file")
	}
	if !strings.Contains(content, "deepseek/deepseek-4pro") {
		t.Error("expected default model in saved file")
	}
}

func TestRoundTripParseFromFile(t *testing.T) {
	// Parse from yaml that was written manually (the same as what saves)
	yamlContent := `
models:
    default: deepseek/deepseek-4pro
    steps:
        spec: openai/gpt-5

workflows:
    default:
        steps:
            - spec
            - design
            - pr
            - review_loop: { max_rounds: 2, on_fail: stop }
            - test
            - task
`
	cfg, err := Parse([]byte(yamlContent))
	if err != nil {
		t.Fatal(err)
	}

	_ = cfg
}

func TestParseWithAgents(t *testing.T) {
	yamlContent := `
workflows:
  test-only:
    steps:
      - test
`
	cfg, err := Parse([]byte(yamlContent))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.Workflows["test-only"]; !ok {
		t.Error("expected test-only workflow")
	}
}
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
	if cfg.Models.Default != "glm/glm-5.2" {
		t.Errorf("expected default model glm/glm-5.2, got %s", cfg.Models.Default)
	}
	if cfg.Models.Steps["spec"] != "glm/glm-5.2" {
		t.Errorf("expected spec=glm/glm-5.2, got %s", cfg.Models.Steps["spec"])
	}
	if cfg.Models.Steps["impl"] != "deepseek/deepseek-v4-pro" {
		t.Errorf("expected impl=deepseek/deepseek-v4-pro, got %s", cfg.Models.Steps["impl"])
	}
	defaultWF, ok := cfg.Workflows["default"]
	if !ok {
		t.Fatal("default workflow should exist")
	}
	wantOrder := []string{"spec", "task", "design", "impl", "pr", "review_loop", "test"}
	if len(defaultWF.Steps) != len(wantOrder) {
		t.Fatalf("default workflow length = %d, want %d", len(defaultWF.Steps), len(wantOrder))
	}
	for i, want := range wantOrder {
		if got := defaultWF.Steps[i].Name; got != want {
			t.Errorf("default step[%d] = %s, want %s", i, got, want)
		}
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
	if !strings.Contains(content, "glm/glm-5.2") {
		t.Error("expected model config in saved file")
	}
	if !strings.Contains(content, "deepseek/deepseek-v4-pro") {
		t.Error("expected impl model in saved file")
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

// TestParseRejectsNonLocalRunsOn verifies the v2-reserved runs_on field is
// rejected for any value other than "local" (design §2.8).
func TestParseRejectsNonLocalRunsOn(t *testing.T) {
	yamlContent := `
workflows:
  default:
    steps:
      - spec
      - impl: { runs_on: server-a }
`
	_, err := Parse([]byte(yamlContent))
	if err == nil {
		t.Fatal("expected error for non-local runs_on, got nil")
	}
	if !strings.Contains(err.Error(), "reserved for v2") {
		t.Errorf("error should mention v2 reservation, got: %v", err)
	}
}

// TestAnthropicEnvOverride verifies environment variables take precedence over
// yaml values for Anthropic connection params (design §2.4: keys via env).
func TestAnthropicEnvOverride(t *testing.T) {
	// Isolate from any ambient env on the dev/CI machine.
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "")
	t.Setenv("ANTHROPIC_BASE_URL", "")
	t.Setenv("ANTHROPIC_MODEL", "")

	a := AnthropicConfig{
		BaseURL:   "https://yaml.example.com",
		AuthToken: "yaml-token",
		Model:     "yaml-model",
	}

	// With env cleared, yaml values are used.
	if got := a.ResolvedAuthToken(); got != "yaml-token" {
		t.Errorf("expected yaml-token, got %q", got)
	}

	t.Setenv("ANTHROPIC_AUTH_TOKEN", "env-token")
	t.Setenv("ANTHROPIC_BASE_URL", "https://env.example.com")
	t.Setenv("ANTHROPIC_MODEL", "env-model")

	if got := a.ResolvedAuthToken(); got != "env-token" {
		t.Errorf("env should override yaml token, got %q", got)
	}
	if got := a.ResolvedBaseURL(); got != "https://env.example.com" {
		t.Errorf("env should override yaml base_url, got %q", got)
	}
	if got := a.ResolvedModel(); got != "env-model" {
		t.Errorf("env should override yaml model, got %q", got)
	}
}

// TestAnthropicEnvFallbackWhenYamlEmpty verifies env is used even when yaml is
// blank (the recommended setup: nothing sensitive in git).
func TestAnthropicEnvFallbackWhenYamlEmpty(t *testing.T) {
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "")
	a := AnthropicConfig{}
	if a.ResolvedAuthToken() != "" {
		t.Error("expected empty token with no yaml and no env")
	}
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "only-env")
	if got := a.ResolvedAuthToken(); got != "only-env" {
		t.Errorf("expected only-env, got %q", got)
	}
}

// TestParseAcceptsLocalRunsOn verifies "local" is accepted (and unset is the
// default).
func TestParseAcceptsLocalRunsOn(t *testing.T) {
	yamlContent := `
workflows:
  default:
    steps:
      - spec
      - impl: { runs_on: local }
`
	cfg, err := Parse([]byte(yamlContent))
	if err != nil {
		t.Fatalf("local runs_on should be accepted: %v", err)
	}
	if cfg.Workflows["default"].Steps[1].RunsOn != "local" {
		t.Errorf("expected RunsOn 'local', got %q", cfg.Workflows["default"].Steps[1].RunsOn)
	}
}

package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bavocado/tomato/pkg/config"
)

func TestEngineLoadsDefaultWorkflow(t *testing.T) {
	dir := t.TempDir()

	cfg := config.Default()
	config.Save(cfg, filepath.Join(dir, "tomato.yaml"))
	os.MkdirAll(filepath.Join(dir, ".tomato", "runs"), 0755)

	eng, err := NewEngine(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(eng.Workflows) != 1 {
		t.Errorf("expected 1 workflow, got %d", len(eng.Workflows))
	}

	wf := eng.Workflows["default"]
	if len(wf.Steps) != 7 {
		t.Errorf("expected 7 steps in default workflow, got %d", len(wf.Steps))
	}
}

func TestEngineHasWorkflow(t *testing.T) {
	dir := t.TempDir()

	yamlContent := `
workflows:
  default:
    steps: [spec, design, impl, pr, review, test, task]
  hotfix:
    steps: [spec, impl, review]
`
	os.WriteFile(filepath.Join(dir, "tomato.yaml"), []byte(yamlContent), 0644)
	os.MkdirAll(filepath.Join(dir, ".tomato", "runs"), 0755)

	eng, err := NewEngine(dir)
	if err != nil {
		t.Fatal(err)
	}

	if !eng.HasWorkflow("hotfix") {
		t.Error("engine should have hotfix workflow")
	}
	if eng.HasWorkflow("nonexistent") {
		t.Error("engine should not have nonexistent workflow")
	}
}

func TestGetSteps(t *testing.T) {
	dir := t.TempDir()

	yamlContent := `
workflows:
  quick:
    steps: [spec, impl, task]
`
	os.WriteFile(filepath.Join(dir, "tomato.yaml"), []byte(yamlContent), 0644)
	os.MkdirAll(filepath.Join(dir, ".tomato", "runs"), 0755)

	eng, err := NewEngine(dir)
	if err != nil {
		t.Fatal(err)
	}

	steps := eng.GetSteps("quick")
	if len(steps) != 3 {
		t.Errorf("expected 3 steps, got %d", len(steps))
	}
	if steps[0] != "spec" {
		t.Errorf("expected first step 'spec', got '%s'", steps[0])
	}
}

func TestGetStepsNonexistent(t *testing.T) {
	dir := t.TempDir()

	yamlContent := `workflows: { default: { steps: [spec] } }`
	os.WriteFile(filepath.Join(dir, "tomato.yaml"), []byte(yamlContent), 0644)
	os.MkdirAll(filepath.Join(dir, ".tomato", "runs"), 0755)

	eng, _ := NewEngine(dir)
	steps := eng.GetSteps("ghost")
	if steps != nil {
		t.Error("expected nil for nonexistent workflow")
	}
}

func TestResolveModel(t *testing.T) {
	dir := t.TempDir()

	yamlContent := `
models:
  default: openai/gpt-5
  steps:
    spec: anthropic/claude-sonnet-4
    impl: deepseek/deepseek-4pro

workflows:
  default:
    steps: [spec, impl]
`
	os.WriteFile(filepath.Join(dir, "tomato.yaml"), []byte(yamlContent), 0644)
	os.MkdirAll(filepath.Join(dir, ".tomato", "runs"), 0755)

	eng, err := NewEngine(dir)
	if err != nil {
		t.Fatal(err)
	}

	model := eng.resolveModel("spec")
	if model != "anthropic/claude-sonnet-4" {
		t.Errorf("expected anthropic/claude-sonnet-4, got %s", model)
	}

	model = eng.resolveModel("nonexistent-step")
	if model != "openai/gpt-5" {
		t.Errorf("expected fallback to openai/gpt-5, got %s", model)
	}
}
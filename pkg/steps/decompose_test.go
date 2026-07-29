package steps

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bavocado/tomato/pkg/runner"
)

func TestDecomposePromptMethodology(t *testing.T) {
	assertContainsAll(t, DecomposePrompt, []string{
		"decomposition analyst",
		"Vertical slice",
		"INVEST",
		"DAG",
		"spike",
		"contract",
		"MoSCoW",
		"C4",
		"out_of_scope",
		"{{.source-design.md}}",
		"```yaml",
	})
}

func TestDecomposeStepRegistered(t *testing.T) {
	if _, err := Get("decompose"); err != nil {
		t.Fatalf("expected decompose step registered: %v", err)
	}
}

func TestRunDecomposeWritesDecomposition(t *testing.T) {
	dir := t.TempDir()
	featureDir := filepath.Join(dir, "docs", "specs", "feat")
	os.MkdirAll(featureDir, 0755)
	os.WriteFile(filepath.Join(featureDir, "source-design.md"), []byte("# Design"), 0644)
	calls := 0
	cfg := &StepConfig{
		RepoDir:    dir,
		FeatureDir: featureDir,
		Feature:    "feat",
		ModelName:  "glm/glm-5.2",
		LLMStream: func(messages []runner.Message, onChunk func(string)) error {
			calls++
			onChunk("# Decomposition\n\n```yaml\nfeatures: []\n```")
			return nil
		},
	}
	res := runDecompose(cfg, nil)
	if !res.Success {
		t.Fatalf("runDecompose failed: %s", res.Error)
	}
	if calls != 1 {
		t.Fatalf("expected 1 LLM call, got %d", calls)
	}
	out, _ := os.ReadFile(filepath.Join(featureDir, "decomposition.md"))
	if !strings.Contains(string(out), "```yaml") {
		t.Errorf("decomposition.md missing yaml block, got: %s", out)
	}
}
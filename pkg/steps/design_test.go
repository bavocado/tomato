package steps

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bavocado/tomato/pkg/runner"
)

func TestRunDesignReadsParentContext(t *testing.T) {
	dir := t.TempDir()
	featureDir := filepath.Join(dir, "docs", "specs", "feat")
	os.MkdirAll(featureDir, 0755)
	os.WriteFile(filepath.Join(featureDir, "prd.md"), []byte("# PRD"), 0644)
	os.WriteFile(filepath.Join(featureDir, "parent-context.md"), []byte("PARENT ARCH"), 0644)
	var got string
	cfg := &StepConfig{
		RepoDir: dir, FeatureDir: featureDir, Feature: "feat", ModelName: "glm/glm-5.2",
		LLMStream: func(m []runner.Message, onChunk func(string)) error {
			got = m[1].Content
			onChunk("---TOMATO-ARTIFACT: architecture.md---\nx")
			return nil
		},
	}
	if res := runDesign(cfg, nil); !res.Success {
		t.Fatalf("runDesign failed: %s", res.Error)
	}
	if !strings.Contains(got, "PARENT ARCH") {
		t.Errorf("expected parent-context.md injected, got: %s", got)
	}
}

func TestRunDesignNoParentContextUnchanged(t *testing.T) {
	dir := t.TempDir()
	featureDir := filepath.Join(dir, "docs", "specs", "feat")
	os.MkdirAll(featureDir, 0755)
	os.WriteFile(filepath.Join(featureDir, "prd.md"), []byte("# PRD"), 0644)
	var got string
	cfg := &StepConfig{
		RepoDir: dir, FeatureDir: featureDir, Feature: "feat", ModelName: "glm/glm-5.2",
		LLMStream: func(m []runner.Message, onChunk func(string)) error {
			got = m[1].Content
			onChunk("---TOMATO-ARTIFACT: architecture.md---\nx")
			return nil
		},
	}
	if res := runDesign(cfg, nil); !res.Success {
		t.Fatalf("runDesign failed: %s", res.Error)
	}
	if strings.Contains(got, "parent-context.md") {
		t.Errorf("legacy design should not reference parent-context.md, got: %s", got)
	}
}

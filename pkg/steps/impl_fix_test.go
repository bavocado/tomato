package steps

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bavocado/tomato/pkg/runner"
)

// TestRunImplFixRoundFeedsPriorReviewComments verifies that a fix-r<N> round
// injects the prior round's review comments into the prompt, so the model
// fixes the blocking issues instead of regenerating from scratch.
func TestRunImplFixRoundFeedsPriorReviewComments(t *testing.T) {
	repo := t.TempDir()
	featureDir := filepath.Join(repo, "docs", "specs", "f")
	reviewsDir := filepath.Join(featureDir, "reviews")
	os.MkdirAll(reviewsDir, 0755)
	os.WriteFile(filepath.Join(featureDir, "architecture.md"), []byte("arch"), 0644)
	os.WriteFile(filepath.Join(featureDir, "ui-spec.md"), []byte("ui"), 0644)
	os.WriteFile(filepath.Join(featureDir, "implementation.md"), []byte("plan"), 0644)
	os.WriteFile(filepath.Join(reviewsDir, "r1-comments.md"), []byte("# Review r1\n## Blocking Issues\n1. bug X in handler"), 0644)

	var captured strings.Builder
	cfg := &StepConfig{
		RepoDir:    repo,
		FeatureDir: featureDir,
		Feature:    "f",
		LLMStream: func(messages []runner.Message, onChunk func(string)) error {
			for _, m := range messages {
				captured.WriteString(m.Content)
			}
			onChunk("# Implementation Output\n## 1. Summary\nnone\n")
			return nil
		},
	}
	result := runImpl(cfg, []string{"fix-r1"})
	if !result.Success {
		t.Fatalf("runImpl fix-r1 failed: %s", result.Error)
	}
	prompt := captured.String()
	if !strings.Contains(prompt, "Prior Review Comments") {
		t.Errorf("fix-r1 prompt must reference prior review comments, got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "bug X in handler") {
		t.Errorf("fix-r1 prompt must include the blocking issue text, got:\n%s", prompt)
	}
}

// TestRunImplNonFixRoundOmitsReviewComments verifies a non-fix round (no args)
// does not inject review comments.
func TestRunImplNonFixRoundOmitsReviewComments(t *testing.T) {
	repo := t.TempDir()
	featureDir := filepath.Join(repo, "docs", "specs", "f")
	os.MkdirAll(filepath.Join(featureDir, "reviews"), 0755)
	os.WriteFile(filepath.Join(featureDir, "architecture.md"), []byte("arch"), 0644)
	os.WriteFile(filepath.Join(featureDir, "ui-spec.md"), []byte("ui"), 0644)
	os.WriteFile(filepath.Join(featureDir, "implementation.md"), []byte("plan"), 0644)
	os.WriteFile(filepath.Join(featureDir, "reviews", "r1-comments.md"), []byte("bug X"), 0644)

	var captured strings.Builder
	cfg := &StepConfig{
		RepoDir:    repo,
		FeatureDir: featureDir,
		Feature:    "f",
		LLMStream: func(messages []runner.Message, onChunk func(string)) error {
			for _, m := range messages {
				captured.WriteString(m.Content)
			}
			onChunk("# Implementation Output\n## 1. Summary\nnone\n")
			return nil
		},
	}
	if !runImpl(cfg, nil).Success {
		t.Fatal("runImpl failed")
	}
	if strings.Contains(captured.String(), "Prior Review Comments") {
		t.Errorf("non-fix round must not include review comments, got:\n%s", captured.String())
	}
}

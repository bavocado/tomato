package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/bavocado/tomato/pkg/budget"
	"github.com/bavocado/tomato/pkg/model"
	"github.com/bavocado/tomato/pkg/runid"
)

// Message mirrors llm.Message for clean API surface.
type Message struct {
	Role    string
	Content string
}

// LLMFunc is a function type for calling the LLM with streaming.
type LLMFunc func(messages []Message, onChunk func(string)) error

// Execute runs one step and returns the result.
// If tracker is non-nil, it performs budget checks before and after execution.
func Execute(
	stepName string,
	promptTemplate string,
	inputFiles []string,
	outputFiles []string,
	repoDir string,
	modelName string,
	llmStream LLMFunc,
	promptVersion string,
	tracker *budget.Tracker,
) *model.StepResult {
	start := time.Now()
	runID := runid.Generate()

	// Build prompts from input files
	messages, err := buildMessages(promptTemplate, inputFiles, repoDir)
	if err != nil {
		return failure(stepName, runID, start, modelName, err)
	}

	// Estimate input tokens from prompt text for budget check
	var promptBuilder strings.Builder
	for _, m := range messages {
		promptBuilder.WriteString(m.Content)
	}
	promptText := promptBuilder.String()

	if tracker != nil {
		estimatedIn := budget.EstimateTokens(promptText)
		if !tracker.Check(stepName, estimatedIn) {
			return budgetExceeded(stepName, runID, start, modelName, stepName, estimatedIn, tracker)
		}
		if !tracker.CheckGlobal(estimatedIn) {
			return budgetExceeded(stepName, runID, start, modelName, "global", estimatedIn, tracker)
		}
	}

	// Call LLM
	var response strings.Builder
	err = llmStream(messages, func(chunk string) {
		response.WriteString(chunk)
	})
	if err != nil {
		return failure(stepName, runID, start, modelName, err)
	}

	// Record actual token usage
	responseText := response.String()
	tokensIn := budget.EstimateTokens(promptText)
	tokensOut := budget.EstimateTokens(responseText)

	if tracker != nil {
		tracker.Record(stepName, tokensIn, tokensOut)
		// Check global after recording
		if !tracker.CheckGlobal(0) {
			// Global exceeded — already recorded, just warn
			fmt.Fprintf(os.Stderr, "⚠  Global token budget exceeded (on_exceed: %s)\n", tracker.OnExceed())
		}
	}

	// Write output artifacts — support artifact splitting via ---TOMATO-ARTIFACT: filename--- markers
	artifactParts := splitArtifacts(responseText)
	for _, outPath := range outputFiles {
		fullPath := outPath
		if !filepath.IsAbs(fullPath) {
			fullPath = filepath.Join(repoDir, outPath)
		}
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return failure(stepName, runID, start, modelName, err)
		}

		// Determine content: prefer the named artifact, fall back to full response
		content := responseText
		baseName := filepath.Base(outPath)
		if part, ok := artifactParts[baseName]; ok {
			content = part
		}

		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return failure(stepName, runID, start, modelName, err)
		}
	}

	// Write run log
	duration := time.Since(start)
	meta := model.RunMeta{
		RunID:       runID,
		StepName:    stepName,
		StartedAt:   start,
		FinishedAt:  time.Now(),
		DurationMs:  duration.Milliseconds(),
		ModelUsed:   modelName,
		TokensIn:    tokensIn,
		TokensOut:   tokensOut,
		Success:     true,
		InputFiles:  inputFiles,
		OutputFiles: outputFiles,
	}
	writeMeta(meta, repoDir, runID)

	return &model.StepResult{
		StepName:   stepName,
		RunID:      runID,
		StartedAt:  start,
		DurationMs: duration.Milliseconds(),
		ModelUsed:  modelName,
		TokensIn:   tokensIn,
		TokensOut:  tokensOut,
		Success:    true,
	}
}

func budgetExceeded(stepName, runID string, start time.Time, modelName, scope string, estimated int, t *budget.Tracker) *model.StepResult {
	duration := time.Since(start)
	errMsg := fmt.Sprintf("%s token budget exceeded (estimated: %d tokens, on_exceed: %s)", scope, estimated, t.OnExceed())
	fmt.Fprintf(os.Stderr, "✗ %s\n", errMsg)
	fmt.Fprintf(os.Stderr, "  Action: %s\n", t.OnExceed())
	if t.OnExceed() == "degrade" {
		// The caller should handle degrade — we just fail here and let engine retry with a cheaper model
	}
	return &model.StepResult{
		StepName:   stepName,
		RunID:      runID,
		StartedAt:  start,
		DurationMs: duration.Milliseconds(),
		ModelUsed:  modelName,
		Success:    false,
		Error:      errMsg,
	}
}

func buildMessages(promptTemplate string, inputFiles []string, repoDir string) ([]Message, error) {
	context := make(map[string]string)
	for _, inPath := range inputFiles {
		fullPath := filepath.Join(repoDir, inPath)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			// File may not exist yet (first run of spec step)
			context[inPath] = ""
			continue
		}
		context[filepath.Base(inPath)] = string(data)
	}

	tmpl, err := template.New("prompt").Parse(promptTemplate)
	if err != nil {
		return nil, fmt.Errorf("parsing prompt template: %w", err)
	}
	var promptBuf strings.Builder
	if err := tmpl.Execute(&promptBuf, context); err != nil {
		return nil, fmt.Errorf("rendering prompt template: %w", err)
	}

	return []Message{
		{Role: "system", Content: "You are tomato, an AI software development assistant. Output in markdown."},
		{Role: "user", Content: promptBuf.String()},
	}, nil
}

func failure(stepName, runID string, start time.Time, modelName string, err error) *model.StepResult {
	duration := time.Since(start)
	return &model.StepResult{
		StepName:   stepName,
		RunID:      runID,
		StartedAt:  start,
		DurationMs: duration.Milliseconds(),
		ModelUsed:  modelName,
		Success:    false,
		Error:      err.Error(),
	}
}

func writeMeta(meta model.RunMeta, repoDir, runID string) {
	runDir := filepath.Join(repoDir, ".tomato", "runs", runID)
	os.MkdirAll(runDir, 0755)

	metaPath := filepath.Join(runDir, "meta.json")
	data, _ := json.MarshalIndent(meta, "", "  ")
	os.WriteFile(metaPath, data, 0644)
}

// splitArtifacts parses an LLM response that contains ---TOMATO-ARTIFACT: filename--- markers
// and returns a map of filename -> content. If no markers are found, returns a single
// entry with key "" containing the full text (unmodified).
func splitArtifacts(text string) map[string]string {
	const markerPrefix = "---TOMATO-ARTIFACT: "

	if !strings.Contains(text, markerPrefix) {
		return map[string]string{"": text}
	}

	parts := make(map[string]string)
	lines := strings.Split(text, "\n")

	var currentName string
	var currentContent strings.Builder

	flush := func() {
		if currentName != "" {
			parts[currentName] = strings.TrimSpace(currentContent.String())
			currentContent.Reset()
		}
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, markerPrefix) && strings.HasSuffix(trimmed, "---") {
			flush()
			name := strings.TrimSuffix(strings.TrimPrefix(trimmed, markerPrefix), "---")
			currentName = strings.TrimSpace(name)
			continue
		}
		if currentName != "" {
			currentContent.WriteString(line)
			currentContent.WriteString("\n")
		}
	}
	flush()

	return parts
}
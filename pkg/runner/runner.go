package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bavocado/tomato/pkg/budget"
	"github.com/bavocado/tomato/pkg/llm"
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
	logStep(stepName, "run=%s model=%s start", runID, modelName)

	// Build prompts from input files
	logStep(stepName, "building prompt from %d input file(s)", len(inputFiles))
	messages, err := buildMessages(stepName, promptTemplate, inputFiles, repoDir)
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
		logStep(stepName, "estimated input tokens=%d", estimatedIn)
		// Global check first. Only "fail" aborts; "warn" (the design default)
		// and "degrade" proceed (design §2.9.4). The old code hard-failed on
		// every policy, which reversed "warn" — the documented default is
		// "warn but continue".
		if !tracker.CheckGlobal(estimatedIn) && !budgetShouldProceed(tracker, "global", estimatedIn) {
			return budgetExceeded(stepName, runID, start, modelName, "global", estimatedIn, tracker)
		}
		if !tracker.Check(stepName, estimatedIn) && !budgetShouldProceed(tracker, stepName, estimatedIn) {
			return budgetExceeded(stepName, runID, start, modelName, stepName, estimatedIn, tracker)
		}
	}

	// Local response cache: same prompt + model + promptVersion → reuse the
	// prior response and skip the LLM call entirely (design §2.9.3). The cache
	// key folds in the rendered prompt text (which already embeds input file
	// contents), so any input change invalidates it.
	cache, _ := llm.NewCache(filepath.Join(repoDir, ".tomato", "cache"))
	cacheKey := llm.CacheKey{
		TemplateVersion: promptVersion,
		PromptContent:   promptText,
		ModelID:         modelName,
	}
	var responseText string
	cacheHit := false
	if cache != nil {
		if cached, ok := cache.Get(cacheKey); ok {
			if multiOutputWithoutMarkers(cached, outputFiles) {
				logStep(stepName, "cache entry missing artifact markers — ignoring")
			} else {
				logStep(stepName, "cache hit — skipping LLM call")
				responseText = cached
				cacheHit = true
			}
		}
	}

	if !cacheHit {
		// Call LLM
		logStep(stepName, "calling LLM model=%s", modelName)
		var response strings.Builder
		err = llmStream(messages, func(chunk string) {
			response.WriteString(chunk)
		})
		if err != nil {
			return failure(stepName, runID, start, modelName, err)
		}
		responseText = response.String()

		// An empty response means the LLM produced no usable output (e.g.
		// claude returned only a system init message with no text). Treat this
		// as a failure so downstream steps don't consume an empty artifact.
		if strings.TrimSpace(responseText) == "" {
			errMsg := "LLM returned an empty response"
			logStep(stepName, "✗ %s", errMsg)
			return &model.StepResult{
				StepName:   stepName,
				RunID:      runID,
				StartedAt:  start,
				DurationMs: time.Since(start).Milliseconds(),
				ModelUsed:  modelName,
				Success:    false,
				Error:      errMsg,
			}
		}
		missingMarkers := multiOutputWithoutMarkers(responseText, outputFiles)
		if missingMarkers {
			logStep(stepName, "response missing ---TOMATO-ARTIFACT--- markers for %d output file(s); writing full response to each", len(outputFiles))
		}

		if cache != nil && !missingMarkers {
			if err := cache.Set(cacheKey, responseText); err != nil {
				fmt.Fprintf(os.Stderr, "⚠  warning: failed to cache response: %v\n", err)
			}
		}
	}

	// Record actual token usage
	tokensIn := budget.EstimateTokens(promptText)
	tokensOut := budget.EstimateTokens(responseText)
	if cacheHit {
		tokensIn = 0
		tokensOut = 0
	}
	logStep(stepName, "LLM completed: tokens in=%d out=%d response_chars=%d", tokensIn, tokensOut, len(responseText))

	if tracker != nil {
		tracker.Record(stepName, tokensIn, tokensOut)
		// Check global after recording
		if !tracker.CheckGlobal(0) {
			// Global exceeded — already recorded, just warn
			fmt.Fprintf(os.Stderr, "⚠  Global token budget exceeded (on_exceed: %s)\n", tracker.OnExceed())
		}
	}

	// Write output artifacts — support artifact splitting via ---TOMATO-ARTIFACT: filename--- markers
	logStep(stepName, "writing %d artifact(s)", len(outputFiles))
	artifactParts := splitArtifacts(responseText)
	// Snapshot dir: a stable copy of each output lives under
	// .tomato/runs/<run-id>/artifacts/ so `tomato history diff` can compare two
	// runs even after the working-tree outputs change (design §3.4, Task 5).
	snapshotDir := filepath.Join(repoDir, ".tomato", "runs", runID, "artifacts")
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
		logStep(stepName, "wrote artifact %s (%d bytes)", fullPath, len(content))

		// Mirror the artifact into the run snapshot dir for history diff.
		if err := os.MkdirAll(snapshotDir, 0755); err != nil {
			return failure(stepName, runID, start, modelName, err)
		}
		snapshotPath := filepath.Join(snapshotDir, baseName)
		if err := os.WriteFile(snapshotPath, []byte(content), 0644); err != nil {
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
		CacheHit:    cacheHit,
		Success:     true,
		InputFiles:  inputFiles,
		OutputFiles: outputFiles,
	}
	writeMeta(meta, repoDir, runID)
	logStep(stepName, "run log written: .tomato/runs/%s/meta.json", runID)

	return &model.StepResult{
		StepName:   stepName,
		RunID:      runID,
		StartedAt:  start,
		DurationMs: duration.Milliseconds(),
		ModelUsed:  modelName,
		TokensIn:   tokensIn,
		TokensOut:  tokensOut,
		CacheHit:   cacheHit,
		Success:    true,
	}
}

func multiOutputWithoutMarkers(text string, outputFiles []string) bool {
	if len(outputFiles) <= 1 {
		return false
	}
	_, unsplit := splitArtifacts(text)[""]
	return unsplit
}

// logStep writes a concise progress line to stderr.
func logStep(stepName, format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "[%s] "+format+"\n", append([]interface{}{stepName}, args...)...)
}

// budgetShouldProceed is consulted when a pre-call budget check would be
// exceeded. It honors the on_exceed policy (design §2.9.4):
//   - "fail": report and return false so the caller aborts the step.
//   - "warn" (default): report a warning and return true (proceed).
//   - "degrade": report a warning and return true (proceed). Automatic
//     model downgrade is a v1.x capability (§2.9.7); v1 does not abort.
func budgetShouldProceed(t *budget.Tracker, scope string, estimated int) bool {
	switch t.OnExceed() {
	case "fail":
		fmt.Fprintf(os.Stderr, "✗ %s token budget exceeded (estimated %d tokens, on_exceed: fail) — aborting step\n", scope, estimated)
		return false
	case "degrade":
		fmt.Fprintf(os.Stderr, "⚠  %s token budget exceeded (estimated %d tokens, on_exceed: degrade) — proceeding; auto-downgrade is v1.x\n", scope, estimated)
		return true
	default: // "warn" and any unrecognized value
		fmt.Fprintf(os.Stderr, "⚠  %s token budget exceeded (estimated %d tokens, on_exceed: warn) — proceeding\n", scope, estimated)
		return true
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

func buildMessages(stepName, promptTemplate string, inputFiles []string, repoDir string) ([]Message, error) {
	// Read each input file, keyed by its base name. The base name is the
	// token used in the prompt template (e.g. {{.prd.md}}).
	//
	// We substitute tokens manually rather than via text/template: Go's
	// template engine parses {{.prd.md}} as a field chain (field "prd" then
	// field "md"), which never matches a map key like "prd.md" — it silently
	// renders <no value>. Manual replacement keeps the readable {{.file}}
	// syntax in prompts and actually injects the content.
	context := make(map[string]string)
	for _, inPath := range inputFiles {
		fullPath := inPath
		// Input paths may be absolute (cfg.FeatureDir is joined against the
		// repo root) or relative. Only join relative paths; joining an
		// absolute path would produce a bogus doubled path.
		if !filepath.IsAbs(fullPath) {
			fullPath = filepath.Join(repoDir, inPath)
		}
		data, err := os.ReadFile(fullPath)
		if err != nil {
			// File may not exist yet (e.g. first run of the spec step) —
			// substitute an empty value so the token still resolves.
			context[filepath.Base(inPath)] = ""
			continue
		}
		context[filepath.Base(inPath)] = string(data)
	}

	// Substitute {{.basename}} tokens with the corresponding file contents.
	// Tokens without a matching input file are left intact so the gap is
	// visible in the prompt rather than silently empty.
	prompt := promptTemplate
	for key, val := range context {
		prompt = strings.ReplaceAll(prompt, "{{."+key+"}}", val)
	}

	return []Message{
		{Role: "system", Content: systemPrompt(stepName)},
		{Role: "user", Content: prompt},
	}, nil
}

func systemPrompt(stepName string) string {
	return fmt.Sprintf("You are tomato, an AI software development assistant. Output in markdown.\n\nFor this %s step, delegate the substantive work to a fresh Task subagent suited to the step. Do not rely on any prior Claude session or conversation state; use only the prompt, files, and tools available in this invocation. If CodeDB/codegraph MCP tools are available, query them for relevant code before reading broad file ranges. Return the final artifact text requested by tomato.", stepName)
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

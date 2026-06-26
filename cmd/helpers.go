package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bavocado/tomato/pkg/budget"
	"github.com/bavocado/tomato/pkg/config"
	"github.com/bavocado/tomato/pkg/llm"
	"github.com/bavocado/tomato/pkg/model"
	"github.com/bavocado/tomato/pkg/steps"
	"github.com/spf13/cobra"
)

// addForceFlag adds a --force boolean flag to a command.
func addForceFlag(cmd *cobra.Command) {
	cmd.Flags().Bool("force", false, "overwrite existing artifacts")
}

// outputsExist returns true if any of the named files/dirs exist under featureDir.
func outputsExist(featureDir string, names ...string) bool {
	for _, name := range names {
		path := filepath.Join(featureDir, name)
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
}

func withFeatureAndModel(fn func(*steps.StepConfig, []string) error) func(*cobra.Command, []string) error {
		return func(cmd *cobra.Command, args []string) error {
			dir, _ := os.Getwd()
			cfg, err := config.Load(dir)
			if err != nil {
				return fmt.Errorf("loading config: %w\nRun `tomato init` first", err)
			}

			stepName := cmd.Use
			modelID := resolveModelForStep(stepName, cfg)
			apiKey := os.Getenv(llm.EnvKeyName(extractProvider(modelID)))

			// Initialize per-command budget tracker
			tracker := budget.NewTracker()
			tracker.InitFromConfig(
				cfg.Budget.Mode,
				cfg.Budget.PerStep,
				cfg.Budget.GlobalPerRun,
				cfg.Budget.OnExceed,
				cfg.Budget.DegradeTo,
			)

			featureDir := filepath.Join(dir, "docs", "specs", "current-feature")
			stepCfg := &steps.StepConfig{
				RepoDir:        dir,
				FeatureDir:     featureDir,
				Feature:        "current-feature",
				ModelName:      modelID,
				APIKey:         apiKey,
				AnthropicURL:   cfg.Anthropic.BaseURL,
				AnthropicKey:   cfg.Anthropic.AuthToken,
				AnthropicModel: cfg.Anthropic.Model,
				BudgetTracker:  tracker,
			}
			stepCfg.LLMStream = steps.NewLLMStream(stepCfg)

			return fn(stepCfg, args)
		}
	}

func resolveModelForStep(stepName string, cfg *config.Config) string {
	if m, ok := cfg.Models.Steps[stepName]; ok {
		return m
	}
	return cfg.Models.Default
}

func extractProvider(modelID string) string {
	for i := 0; i < len(modelID); i++ {
		if modelID[i] == '/' {
			return modelID[:i]
		}
	}
	return "openai"
}

func runStepWithName(name string, cfg *steps.StepConfig) *model.StepResult {
	stepFn, err := steps.Get(name)
	if err != nil {
		return &model.StepResult{Success: false, Error: err.Error()}
	}
	return stepFn(cfg, nil)
}

func printResult(result *model.StepResult) {
	if result.Success {
		fmt.Printf("✓ %s completed (run: %s, model: %s)\n",
			result.StepName, result.RunID, result.ModelUsed)
	} else {
		fmt.Fprintf(os.Stderr, "✗ %s failed: %s\n", result.StepName, result.Error)
	}
}
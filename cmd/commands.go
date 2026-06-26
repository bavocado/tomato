package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/bavocado/tomato/pkg/config"
	"github.com/bavocado/tomato/pkg/cost"
	"github.com/bavocado/tomato/pkg/engine"
	"github.com/bavocado/tomato/pkg/history"
	"github.com/bavocado/tomato/pkg/steps"
	"github.com/spf13/cobra"
)

func NewInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize tomato.yaml in the current repo",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := os.Getwd()
			if err != nil {
				return err
			}
			path := filepath.Join(dir, "tomato.yaml")
			if _, err := os.Stat(path); err == nil {
				return fmt.Errorf("tomato.yaml already exists in %s", dir)
			}
			cfg := config.Default()
			if err := config.Save(cfg, path); err != nil {
				return err
			}
			fmt.Printf("✓ Initialized tomato.yaml in %s\n", dir)
			runsDir := filepath.Join(dir, ".tomato", "runs")
			if err := os.MkdirAll(runsDir, 0755); err != nil {
				return fmt.Errorf("creating .tomato/runs: %w", err)
			}
			fmt.Printf("✓ Created .tomato/runs/\n")

			// Warn about auth_token in git-tracked file
			gitignorePath := filepath.Join(dir, ".gitignore")
			if !isTomatoYamlIgnored(gitignorePath) {
				fmt.Println("⚠  WARNING: tomato.yaml contains auth_token in plain text.")
				fmt.Println("   Add 'tomato.yaml' to your .gitignore or use env vars in CI.")
			}

			return nil
		},
	}
}

func isTomatoYamlIgnored(gitignorePath string) bool {
	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		return false
	}
	content := string(data)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "tomato.yaml" {
			return true
		}
	}
	return false
}

func NewRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run [workflow]",
		Short: "Run a workflow (default: default)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, _ := os.Getwd()
			eng, err := engine.NewEngine(dir)
			if err != nil {
				return err
			}
			workflowName := "default"
			if len(args) > 0 {
				workflowName = args[0]
			}
			if err := eng.Run(workflowName); err != nil {
				fmt.Fprintf(os.Stderr, "✗ workflow %q failed: %v\n", workflowName, err)
				os.Exit(1)
			}
			fmt.Printf("✓ workflow %q completed\n", workflowName)
			return nil
		},
	}
}

func NewSpecCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "spec",
		Short: "Run requirements analysis (generate PRD)",
	}
	cmd.RunE = withFeatureAndModel(func(cfg *steps.StepConfig, args []string) error {
		force, _ := cmd.Flags().GetBool("force")
		if !force && outputsExist(cfg.FeatureDir, "prd.md") {
			return fmt.Errorf("prd.md already exists. Use --force to overwrite")
		}
		result := runStepWithName("spec", cfg)
		printResult(result)
		if !result.Success {
			os.Exit(1)
		}
		return nil
	})
	addForceFlag(cmd)
	return cmd
}

func NewDesignCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "design",
		Short: "Run design (architecture + UI + implementation)",
	}
	cmd.RunE = withFeatureAndModel(func(cfg *steps.StepConfig, args []string) error {
		force, _ := cmd.Flags().GetBool("force")
		if !force && outputsExist(cfg.FeatureDir, "architecture.md", "ui-spec.md", "implementation.md") {
			return fmt.Errorf("design artifacts already exist. Use --force to overwrite")
		}
		result := runStepWithName("design", cfg)
		printResult(result)
		if !result.Success {
			os.Exit(1)
		}
		return nil
	})
	addForceFlag(cmd)
	return cmd
}

func NewImplCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "impl",
		Short: "Run code implementation",
	}
	cmd.RunE = withFeatureAndModel(func(cfg *steps.StepConfig, args []string) error {
		force, _ := cmd.Flags().GetBool("force")
		if !force && outputsExist(cfg.FeatureDir, "impl-output.md") {
			return fmt.Errorf("impl-output.md already exists. Use --force to overwrite")
		}
		result := runStepWithName("impl", cfg)
		printResult(result)
		if !result.Success {
			os.Exit(1)
		}
		return nil
	})
	addForceFlag(cmd)
	return cmd
}

func NewReviewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "review",
		Short: "Single-shot code review (no loop)",
	}
	cmd.RunE = withFeatureAndModel(func(cfg *steps.StepConfig, args []string) error {
		force, _ := cmd.Flags().GetBool("force")
		if !force && outputsExist(cfg.FeatureDir, "reviews") {
			return fmt.Errorf("review artifacts already exist. Use --force to overwrite")
		}
		result := runStepWithName("review", cfg)
		printResult(result)
		if !result.Success {
			os.Exit(1)
		}
		return nil
	})
	addForceFlag(cmd)
	return cmd
}

func NewTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Generate and run tests",
	}
	cmd.RunE = withFeatureAndModel(func(cfg *steps.StepConfig, args []string) error {
		force, _ := cmd.Flags().GetBool("force")
		if !force && outputsExist(cfg.FeatureDir, "test-report.md") {
			return fmt.Errorf("test-report.md already exists. Use --force to overwrite")
		}
		result := runStepWithName("test", cfg)
		printResult(result)
		if !result.Success {
			os.Exit(1)
		}
		return nil
	})
	addForceFlag(cmd)
	return cmd
}

func NewPRCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pr",
		Short: "Push branch + open/update PR (draft)",
		RunE: withFeatureAndModel(func(cfg *steps.StepConfig, args []string) error {
			result := runStepWithName("pr", cfg)
			printResult(result)
			if !result.Success {
				os.Exit(1)
			}
			return nil
		}),
	}
}

func NewTaskCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "task",
		Short: "Sync external tasks via adapter",
		RunE: withFeatureAndModel(func(cfg *steps.StepConfig, args []string) error {
			result := runStepWithName("task", cfg)
			printResult(result)
			if !result.Success {
				os.Exit(1)
			}
			return nil
		}),
	}
}

func NewHistoryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "history [run-id]",
		Short: "List past runs or show one run",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, _ := os.Getwd()
			if len(args) > 0 {
				output, err := history.Show(dir, args[0])
				if err != nil {
					return err
				}
				fmt.Print(output)
			} else {
				runs, err := history.List(dir)
				if err != nil {
					return err
				}
				fmt.Printf("%-30s %-12s %-12s %6s %s\n", "Run ID", "Step", "Model", "Tokens", "Status")
				for _, r := range runs {
					status := "✓"
					if !r.Success {
						status = "✗"
					}
					cache := ""
					if r.CacheHit {
						cache = " [cache]"
					}
					fmt.Printf("%-30s %-12s %-12s %6d %s%s\n",
						r.RunID, r.StepName, r.ModelUsed, r.TokensIn+r.TokensOut, status, cache)
				}
			}
			return nil
		},
	}
}

func NewCostCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cost",
		Short: "Cumulative cost summary",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, _ := os.Getwd()
			s, err := cost.Compute(dir)
			if err != nil {
				return err
			}
			fmt.Print(s.Format())
			return nil
		},
	}
}

func NewConfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "View config and API key status",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, _ := os.Getwd()
			cfg, err := config.Load(dir)
			if err != nil {
				return fmt.Errorf("loading config: %w\nRun `tomato init` first", err)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Models:\n")
			fmt.Fprintf(out, "  default: %s\n", cfg.Models.Default)
			for step, model := range cfg.Models.Steps {
				fmt.Fprintf(out, "  %s: %s\n", step, model)
			}
			fmt.Fprintf(out, "\nAnthropic:\n")
			printConfiguredValue(out, "  base_url", cfg.Anthropic.BaseURL)
			if cfg.Anthropic.AuthToken != "" {
				fmt.Fprintf(out, "  auth_token: ✓ configured (%s)\n", maskSecret(cfg.Anthropic.AuthToken))
			} else {
				fmt.Fprintf(out, "  auth_token: ✗ not set\n")
			}
			printConfiguredValue(out, "  model", cfg.Anthropic.Model)

			fmt.Fprintf(out, "\nBudget: %s\n", cfg.Budget.Mode)
			fmt.Fprintf(out, "\nAPI keys:\n")
			for _, provider := range []string{"OPENAI", "GLM", "DEEPSEEK"} {
				key := os.Getenv(provider + "_API_KEY")
				if key != "" {
					fmt.Fprintf(out, "  %s: ✓ configured (%s)\n", provider, maskSecret(key))
				} else {
					fmt.Fprintf(out, "  %s: ✗ not set\n", provider)
				}
			}
			return nil
		},
	}
}

func printConfiguredValue(out io.Writer, name, value string) {
	if value != "" {
		fmt.Fprintf(out, "%s: ✓ %s\n", name, value)
	} else {
		fmt.Fprintf(out, "%s: ✗ not set\n", name)
	}
}

func maskSecret(secret string) string {
	if len(secret) <= 8 {
		return secret + "..."
	}
	return secret[:8] + "..."
}
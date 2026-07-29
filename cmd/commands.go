package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/bavocado/tomato/pkg/codegraph"
	"github.com/bavocado/tomato/pkg/config"
	"github.com/bavocado/tomato/pkg/cost"
	"github.com/bavocado/tomato/pkg/decompose"
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

			// Ensure .tomato/ and tomato.yaml are in .gitignore
			gitignorePath := filepath.Join(dir, ".gitignore")
			ensureGitignore(gitignorePath, []string{".tomato/", "tomato.yaml"})
			fmt.Printf("✓ Updated .gitignore (.tomato/ and tomato.yaml)\n")

			// Write CLAUDE.md with Karpathy working protocols so the `claude`
			// CLI reads them as project-level guidance during every step.
			// Creates when absent, appends when the file exists but lacks the
			// guidelines, and skips (idempotent) when already present.
			claudeMDPath := filepath.Join(dir, "CLAUDE.md")
			action, err := config.WriteCLAUDEMD(claudeMDPath)
			if err != nil {
				return fmt.Errorf("writing CLAUDE.md: %w", err)
			}
			switch action {
			case "created":
				fmt.Printf("✓ Created CLAUDE.md (Karpathy guidelines)\n")
			case "appended":
				fmt.Printf("✓ Appended Karpathy guidelines to CLAUDE.md\n")
			case "skipped":
				fmt.Printf("✓ CLAUDE.md already contains Karpathy guidelines (skipped)\n")
			}

			// Build a CodeDB/codegraph index when available, so LLM steps can
			// query code context via MCP during tomato run. Set
			// TOMATO_SKIP_CODEGRAPH=1 to skip (used by tests).
			if os.Getenv("TOMATO_SKIP_CODEGRAPH") == "1" {
				// skip codegraph entirely
			} else if codegraph.CodeDBCLIPath() != "" {
				if codegraph.HasCodeDBIndex(dir) {
					fmt.Printf("✓ codedb index already exists in %s\n", dir)
				} else if err := codegraph.InitCodeDBIndex(dir); err != nil {
					fmt.Fprintf(os.Stderr, "⚠  warning: codedb init failed: %v\n", err)
				} else {
					fmt.Printf("✓ Built codedb index (.codedb/)\n")
				}
			} else {
				cgBin, wasInstalled, err := codegraph.EnsureCLI()
				if err != nil {
					fmt.Fprintf(os.Stderr, "⚠  warning: codegraph install failed: %v\n", err)
					fmt.Println("   Install manually: curl -fsSL https://raw.githubusercontent.com/colbymchenry/codegraph/main/install.sh | sh")
				} else if wasInstalled {
					fmt.Printf("✓ Installed codegraph CLI (%s)\n", cgBin)
					if codegraph.CLIPath() == "" {
						fmt.Println("⚠  codegraph installed to ~/.local/bin — add it to PATH:")
						fmt.Println("   export PATH=\"$HOME/.local/bin:$PATH\"")
					}
				}
				if cgBin != "" {
					if codegraph.HasIndex(dir) {
						fmt.Printf("✓ codegraph index already exists in %s\n", dir)
					} else if err := codegraph.InitIndex(dir); err != nil {
						fmt.Fprintf(os.Stderr, "⚠  warning: codegraph init failed: %v\n", err)
					} else {
						fmt.Printf("✓ Built codegraph index (.codegraph/)\n")
					}
				}
			}

			// Warn about auth_token in git-tracked file
			if !isTomatoYamlIgnored(gitignorePath) {
				fmt.Println("⚠  WARNING: tomato.yaml contains auth_token in plain text.")
				fmt.Println("   It is now in .gitignore. Verify before committing.")
			}

			return nil
		},
	}
}

// ensureGitignore appends entries to .gitignore if they are not already present.
func ensureGitignore(gitignorePath string, entries []string) {
	var content string
	if data, err := os.ReadFile(gitignorePath); err == nil {
		content = string(data)
	}

	var toAdd []string
	for _, entry := range entries {
		found := false
		for _, line := range strings.Split(content, "\n") {
			if strings.TrimSpace(line) == entry {
				found = true
				break
			}
		}
		if !found {
			toAdd = append(toAdd, entry)
		}
	}

	if len(toAdd) == 0 {
		return
	}

	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += "# tomato\n"
	for _, entry := range toAdd {
		content += entry + "\n"
	}
	os.WriteFile(gitignorePath, []byte(content), 0644)
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
	cmd := &cobra.Command{
		Use:   "run [workflow]",
		Short: "Run a workflow (default: default)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, _ := os.Getwd()
			eng, err := engine.NewEngine(dir)
			if err != nil {
				return err
			}
			flagFeature, _ := cmd.Flags().GetString("feature")
			eng.Feature = steps.ResolveFeature(flagFeature, eng.Config.Feature, dir)
			from, _ := cmd.Flags().GetString("from")
			resume, _ := cmd.Flags().GetBool("resume")
			fast, _ := cmd.Flags().GetBool("fast")
			workflowName := "default"
			if len(args) > 0 {
				workflowName = args[0]
			}
			if err := eng.RunWithOptions(workflowName, engine.RunOptions{From: from, Resume: resume, Fast: fast}); err != nil {
				fmt.Fprintf(os.Stderr, "✗ workflow %q failed: %v\n", workflowName, err)
				os.Exit(1)
			}
			fmt.Printf("✓ workflow %q completed\n", workflowName)
			return nil
		},
	}
	addFeatureFlag(cmd)
	cmd.Flags().String("from", "", "start workflow from the named step")
	cmd.Flags().Bool("resume", false, "resume from the last failed step")
	cmd.Flags().Bool("fast", false, "run as one Claude Code pass with tests")
	return cmd
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
	addFeatureFlag(cmd)
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
	addFeatureFlag(cmd)
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
	addFeatureFlag(cmd)
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
	addFeatureFlag(cmd)
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
	addFeatureFlag(cmd)
	return cmd
}

func NewPRCmd() *cobra.Command {
	cmd := &cobra.Command{
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
	addFeatureFlag(cmd)
	return cmd
}

func NewTaskCmd() *cobra.Command {
	cmd := &cobra.Command{
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
	addFeatureFlag(cmd)
	return cmd
}

func NewHistoryCmd() *cobra.Command {
	cmd := &cobra.Command{
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
	cmd.AddCommand(newHistoryDiffCmd())
	return cmd
}

func newHistoryDiffCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "diff <run-a> <run-b> <artifact>",
		Short: "Diff an artifact between two runs",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, _ := os.Getwd()
			out, err := history.Diff(dir, args[0], args[1], args[2])
			if err != nil {
				return err
			}
			fmt.Print(out)
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
			fmt.Fprintf(out, "\nAnthropic (legacy):\n")
			printConfiguredValue(out, "  base_url", cfg.Anthropic.ResolvedBaseURL())
			if token := cfg.Anthropic.ResolvedAuthToken(); token != "" {
				src := "yaml"
				if os.Getenv("ANTHROPIC_AUTH_TOKEN") != "" {
					src = "env"
				}
				fmt.Fprintf(out, "  auth_token: ✓ configured (%s, from %s)\n", maskSecret(token), src)
			} else {
				fmt.Fprintf(out, "  auth_token: ✗ not set (set ANTHROPIC_AUTH_TOKEN or anthropic.auth_token)\n")
			}
			printConfiguredValue(out, "  model", cfg.Anthropic.ResolvedModel())

			fmt.Fprintf(out, "\nProviders:\n")
			if len(cfg.Providers) == 0 {
				fmt.Fprintf(out, "  (none configured)\n")
			}
			for name, p := range cfg.Providers {
				fmt.Fprintf(out, "  %s:\n", name)
				printConfiguredValue(out, "    base_url", p.BaseURL)
				if token := p.AuthToken; token != "" {
					fmt.Fprintf(out, "    auth_token: ✓ configured (%s, from yaml)\n", maskSecret(token))
				} else {
					fmt.Fprintf(out, "    auth_token: ✗ not set\n")
				}
				printConfiguredValue(out, "    model", p.Model)
			}

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

func NewDecomposeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "decompose",
		Short: "Decompose a design doc into sub-features",
	}
	cmd.RunE = withFeatureAndModel(func(cfg *steps.StepConfig, args []string) error {
		input, _ := cmd.Flags().GetString("input")
		apply, _ := cmd.Flags().GetBool("apply")
		force, _ := cmd.Flags().GetBool("force")
		featureExplicit := cmd.Flags().Changed("feature")

		if input != "" && apply {
			return fmt.Errorf("--input and --apply are mutually exclusive")
		}
		if input == "" && !apply {
			return fmt.Errorf("usage: tomato decompose --input <doc> | tomato decompose --apply")
		}
		if apply {
			return runDecomposeApply(cfg, force, featureExplicit)
		}
		return runDecomposeGenerate(cfg, input, force, featureExplicit)
	})
	cmd.Flags().String("input", "", "path to the design document to decompose")
	cmd.Flags().Bool("apply", false, "materialize sub-features from decomposition.md")
	addForceFlag(cmd)
	addFeatureFlag(cmd)
	return cmd
}

func runDecomposeGenerate(cfg *steps.StepConfig, input string, force bool, featureExplicit bool) error {
	data, err := os.ReadFile(input)
	if err != nil {
		return fmt.Errorf("reading --input: %w", err)
	}
	if strings.TrimSpace(string(data)) == "" {
		return fmt.Errorf("--input %s is empty", input)
	}
	// The parent feature name comes from the design doc's title, not the git
	// branch — otherwise running decompose on branch "feat/x" labels every
	// sub-feature "x-f001". An explicit --feature still wins.
	if !featureExplicit {
		applyParentFeature(cfg, decompose.ParentFeatureFromDesign(string(data)))
	}
	if !force && outputsExist(cfg.FeatureDir, "decomposition.md") {
		return fmt.Errorf("decomposition.md already exists. Use --force to overwrite")
	}
	sourcePath := filepath.Join(cfg.FeatureDir, "source-design.md")
	if err := os.MkdirAll(cfg.FeatureDir, 0755); err != nil {
		return fmt.Errorf("creating feature dir %s: %w", cfg.FeatureDir, err)
	}
	if err := os.WriteFile(sourcePath, data, 0644); err != nil {
		return fmt.Errorf("writing source-design.md: %w", err)
	}
	result := runStepWithName("decompose", cfg)
	printResult(result)
	if !result.Success {
		os.Exit(1)
	}
	return nil
}

func runDecomposeApply(cfg *steps.StepConfig, force bool, featureExplicit bool) error {
	// Re-derive the parent feature from the design doc so apply lands sub-features
	// under the same parent dir that generate used, regardless of the current branch.
	if !featureExplicit {
		if design, err := os.ReadFile(filepath.Join(cfg.FeatureDir, "source-design.md")); err == nil {
			applyParentFeature(cfg, decompose.ParentFeatureFromDesign(string(design)))
		}
	}
	decompPath := filepath.Join(cfg.FeatureDir, "decomposition.md")
	content, err := os.ReadFile(decompPath)
	if err != nil {
		return fmt.Errorf("reading decomposition.md (run `tomato decompose --input` first): %w", err)
	}
	d, err := decompose.ParseDecomposition(string(content))
	if err != nil {
		return fmt.Errorf("parsing decomposition.md: %w", err)
	}
	if err := decompose.Validate(d); err != nil {
		return err
	}
	sourcePath := filepath.Join(cfg.FeatureDir, "source-design.md")
	if _, err := os.Stat(sourcePath); err != nil {
		fmt.Fprintf(os.Stderr, "⚠  source-design.md missing; parent-context.md will lack the full parent doc\n")
	}
	specsDir := filepath.Join(cfg.RepoDir, "docs", "specs")
	if err := decompose.Apply(d, cfg.Feature, specsDir, sourcePath, force); err != nil {
		return err
	}
	if err := decompose.WriteOrchestration(d, cfg.Feature, cfg.FeatureDir); err != nil {
		return err
	}
	fmt.Printf("✓ decomposed into %d sub-features; orchestration: %s/orchestration.yaml\n", len(d.Features), cfg.FeatureDir)
	return nil
}

// applyParentFeature overrides the resolved feature and its artifact dir.
// Use it only when the feature was not given explicitly via --feature.
func applyParentFeature(cfg *steps.StepConfig, feature string) {
	if feature == "" {
		return
	}
	cfg.Feature = feature
	cfg.FeatureDir = steps.FeatureDir(cfg.RepoDir, feature)
}

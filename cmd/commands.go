package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bavocado/tomato/pkg/config"
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

			// Create .tomato/runs directory
			runsDir := filepath.Join(dir, ".tomato", "runs")
			if err := os.MkdirAll(runsDir, 0755); err != nil {
				return fmt.Errorf("creating .tomato/runs: %w", err)
			}
			fmt.Printf("✓ Created .tomato/runs/\n")

			return nil
		},
	}
}

func NewRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run [workflow]",
		Short: "Run a workflow (default: default)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("not implemented yet")
		},
	}
}

func NewSpecCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "spec",
		Short: "Run requirements analysis",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("not implemented yet")
		},
	}
}

func NewDesignCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "design",
		Short: "Run design (architecture + UI + implementation)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("not implemented yet")
		},
	}
}

func NewImplCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "impl",
		Short: "Run code implementation",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("not implemented yet")
		},
	}
}

func NewPRCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pr",
		Short: "Push branch + open/update PR (draft)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("not implemented yet")
		},
	}
}

func NewReviewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "review",
		Short: "Single-shot code review (no loop)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("not implemented yet")
		},
	}
}

func NewTestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test",
		Short: "Generate and run tests",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("not implemented yet")
		},
	}
}

func NewTaskCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "task",
		Short: "Sync external tasks",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("not implemented yet")
		},
	}
}

func NewHistoryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "history [run-id]",
		Short: "List past runs or show one run",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("not implemented yet")
		},
	}
}

func NewCostCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cost",
		Short: "Cumulative cost summary",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("not implemented yet")
		},
	}
}

func NewConfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "View/edit config (including API key status)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("not implemented yet")
		},
	}
}
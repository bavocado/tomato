package cmd

import (
	"fmt"
	"os"

	"github.com/bavocado/tomato/pkg/runid"
	"github.com/spf13/cobra"
)

func NewRootCmd(version string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "tomato",
		Short: "AI software development workflow engine",
		Long: `tomato is a CLI-first AI software development workflow engine.
It turns requirements → design → implementation → review → testing → tasks
into a declarative, composable, adaptable pipeline.

Documentation: https://github.com/bavocado/tomato`,
		Version: version,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	rootCmd.AddCommand(NewInitCmd())
	rootCmd.AddCommand(NewRunCmd())
	rootCmd.AddCommand(NewSpecCmd())
	rootCmd.AddCommand(NewDesignCmd())
	rootCmd.AddCommand(NewImplCmd())
	rootCmd.AddCommand(NewPRCmd())
	rootCmd.AddCommand(NewReviewCmd())
	rootCmd.AddCommand(NewTestCmd())
	rootCmd.AddCommand(NewTaskCmd())
	rootCmd.AddCommand(NewHistoryCmd())
	rootCmd.AddCommand(NewCostCmd())
	rootCmd.AddCommand(NewConfigCmd())

	return rootCmd
}

func Execute(version string) {
	_ = runid.Generate() // ensure package compiles
	if err := NewRootCmd(version).Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
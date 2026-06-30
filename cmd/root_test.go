package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestHelpOutput(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd := NewRootCmd("0.1.0")
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"--help"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	if !strings.Contains(output, "tomato") || !strings.Contains(output, "init") {
		t.Errorf("help output missing expected commands: %s", output)
	}
}

func TestRunFromFlagInHelp(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd := NewRootCmd("0.1.0")
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"run", "--help"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	if !strings.Contains(output, "--from") {
		t.Errorf("run --help missing --from flag: %s", output)
	}
	if !strings.Contains(output, "start workflow from the named step") {
		t.Errorf("run --help missing --from description: %s", output)
	}
}

func TestRunResumeFlagInHelp(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd := NewRootCmd("0.1.0")
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"run", "--help"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	if !strings.Contains(output, "--resume") {
		t.Errorf("run --help missing --resume flag: %s", output)
	}
	if !strings.Contains(output, "resume from the last failed step") {
		t.Errorf("run --help missing --resume description: %s", output)
	}
}

func TestHistoryDiffSubcommandInHelp(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd := NewRootCmd("0.1.0")
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"history", "--help"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	if !strings.Contains(output, "diff") {
		t.Errorf("history --help missing diff subcommand: %s", output)
	}
}

func TestVersionFlag(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd := NewRootCmd("0.1.0")
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"--version"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	if !strings.Contains(output, "0.1.0") {
		t.Errorf("version output missing 0.1.0: %s", output)
	}
}

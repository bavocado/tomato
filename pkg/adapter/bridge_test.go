package adapter

import (
	"testing"
)

func TestEchoAdapter(t *testing.T) {
	bridge := &Bridge{
		Bin: "echo",
		Env: nil,
	}

	output, err := bridge.Execute(CmdCreateTask, `{"spec_title":"Build login"}`, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(output) == 0 {
		t.Error("expected output from echo adapter")
	}
}

func TestNonexistentAdapter(t *testing.T) {
	bridge := &Bridge{
		Bin: "nonexistent-adapter-xyz",
	}

	_, err := bridge.Execute(CmdCreateTask, `{}`, nil)
	if err == nil {
		t.Error("expected error for nonexistent adapter binary")
	}
}

func TestSubcommandConstants(t *testing.T) {
	if string(CmdCreatePR) != "create-pr" {
		t.Errorf("expected create-pr, got %s", CmdCreatePR)
	}
	if string(CmdMarkPRFailed) != "mark-pr-failed" {
		t.Errorf("expected mark-pr-failed, got %s", CmdMarkPRFailed)
	}
	if len(AllSubcommands) != 8 {
		t.Errorf("expected 8 subcommands, got %d", len(AllSubcommands))
	}
}

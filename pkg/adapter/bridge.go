package adapter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
)

// Bridge executes driver CLI adapters as subprocesses.
type Bridge struct {
	Bin string
	Env map[string]string
}

// Execute runs an adapter subcommand with stdin JSON and returns stdout.
func (b *Bridge) Execute(subcommand Subcommand, stdinJSON string, envOverrides map[string]string) (string, error) {
	cmd := exec.Command(b.Bin, string(subcommand))

	env := os.Environ()
	for k, v := range b.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	for k, v := range envOverrides {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env

	cmd.Stdin = bytes.NewBufferString(stdinJSON)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return "", fmt.Errorf("executing %s %s: %w", b.Bin, subcommand, err)
		}
	}

	if exitCode != 0 {
		return "", fmt.Errorf("%s %s exited with code %d: %s", b.Bin, subcommand, exitCode, stderr.String())
	}

	return stdout.String(), nil
}

// DetectCapabilities asks the adapter what subcommands it supports.
func (b *Bridge) DetectCapabilities() ([]Subcommand, error) {
	output, err := b.Execute("capabilities", "", nil)
	if err != nil {
		return nil, err
	}

	var caps []Subcommand
	if err := json.Unmarshal([]byte(output), &caps); err != nil {
		return AllSubcommands, nil // assume all if no capabilities endpoint
	}
	return caps, nil
}
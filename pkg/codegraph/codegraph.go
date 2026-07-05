package codegraph

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// installURL is the official standalone installer. It is non-interactive and
// installs to ~/.codegraph with a symlink to ~/.local/bin/codegraph.
const installURL = "https://raw.githubusercontent.com/colbymchenry/codegraph/main/install.sh"

// CLIPath returns the path to the codegraph binary, or "" if not found.
func CLIPath() string {
	p, err := exec.LookPath("codegraph")
	if err != nil {
		return ""
	}
	return p
}

// Install downloads and installs the codegraph CLI via the official standalone
// installer. It is non-interactive. The installer exits non-zero on failure.
// After install the binary is at ~/.local/bin/codegraph, which the caller is
// responsible for ensuring is on PATH (tomato init prints a reminder when that
// dir is not on PATH).
func Install() error {
	cmd := exec.Command("sh", "-c", "curl -fsSL "+installURL+" | sh")
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("codegraph installer failed: %w", err)
	}
	return nil
}

// EnsureCLI installs codegraph when it is not already on PATH. Returns the
// resolved binary path and whether a fresh install was performed.
func EnsureCLI() (binPath string, installed bool, err error) {
	if p := CLIPath(); p != "" {
		return p, false, nil
	}
	if err := Install(); err != nil {
		return "", false, err
	}
	// The installer symlinks to ~/.local/bin/codegraph. LookPath may not find
	// it if that dir is not on PATH yet, so fall back to the known location.
	if p := CLIPath(); p != "" {
		return p, true, nil
	}
	home, _ := os.UserHomeDir()
	fallback := filepath.Join(home, ".local", "bin", "codegraph")
	if _, err := os.Stat(fallback); err == nil {
		return fallback, true, nil
	}
	return "", true, fmt.Errorf("codegraph installed but binary not found on PATH or at %s; add ~/.local/bin to PATH and retry", fallback)
}

// HasIndex reports whether the given repo has a .codegraph/ index directory.
func HasIndex(repoDir string) bool {
	info, err := os.Stat(filepath.Join(repoDir, ".codegraph"))
	return err == nil && info.IsDir()
}

// InitIndex runs `codegraph init` in repoDir to build the code knowledge graph.
// It is best-effort: a failure is returned as an error but never blocks the
// tomato init flow (callers surface it as a warning).
func InitIndex(repoDir string) error {
	bin := CLIPath()
	if bin == "" {
		return fmt.Errorf("codegraph not found on PATH; install it from https://github.com/colbymchenry/codegraph")
	}
	cmd := exec.Command(bin, "init")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("codegraph init failed: %s: %w", string(out), err)
	}
	return nil
}

// MCPConfig is the MCP server config structure claude --mcp-config expects.
type MCPConfig struct {
	McpServers map[string]McpServer `json:"mcpServers"`
}

// McpServer describes a single stdio MCP server.
type McpServer struct {
	Type    string   `json:"type"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

// WriteMCPConfig writes a temporary MCP config file that mounts the codegraph
// server, and returns its path. The file is written under .tomato/ (not
// git-tracked). Returns ("", nil) when codegraph is not installed or the repo
// has no .codegraph/ index, so callers can skip passing --mcp-config.
func WriteMCPConfig(repoDir string) (string, error) {
	bin := CLIPath()
	if bin == "" {
		// Fall back to the default install location so a freshly installed
		// codegraph works even before ~/.local/bin is on PATH.
		home, _ := os.UserHomeDir()
		fallback := filepath.Join(home, ".local", "bin", "codegraph")
		if _, err := os.Stat(fallback); err == nil {
			bin = fallback
		}
	}
	if bin == "" || !HasIndex(repoDir) {
		return "", nil
	}
	cfg := MCPConfig{
		McpServers: map[string]McpServer{
			"codegraph": {
				Type:    "stdio",
				Command: bin,
				Args:    []string{"serve", "--mcp"},
			},
		},
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	path := filepath.Join(repoDir, ".tomato", "codegraph-mcp.json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", err
	}
	return path, nil
}

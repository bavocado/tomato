package steps

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractCodeBlocks(t *testing.T) {
	markdown := `# Implementation

## main.go
` + "```go:main.go" + `
package main

func main() {}
` + "```" + `

## utils.go
` + "```go:pkg/utils.go" + `
package utils

func Hello() string { return "hi" }
` + "```" + `
`

	blocks := extractCodeBlocks(markdown)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 code blocks, got %d", len(blocks))
	}
	if blocks["main.go"] != "package main\n\nfunc main() {}\n" {
		t.Errorf("main.go content wrong: %q", blocks["main.go"])
	}
	if blocks["pkg/utils.go"] != "package utils\n\nfunc Hello() string { return \"hi\" }\n" {
		t.Errorf("utils.go content wrong: %q", blocks["pkg/utils.go"])
	}
}

func TestExtractCodeBlocksNoLanguage(t *testing.T) {
	markdown := "```main.go\npackage main\n```"
	blocks := extractCodeBlocks(markdown)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 code block, got %d", len(blocks))
	}
	if blocks["main.go"] != "package main\n" {
		t.Errorf("content wrong: %q", blocks["main.go"])
	}
}

func TestExtractCodeBlocksEmpty(t *testing.T) {
	blocks := extractCodeBlocks("no code blocks here")
	if len(blocks) != 0 {
		t.Errorf("expected 0 blocks, got %d", len(blocks))
	}
}

func TestWriteCodeBlocks(t *testing.T) {
	dir := t.TempDir()
	blocks := map[string]string{
		"main.go":           "package main\n",
		"pkg/utils/util.go": "package util\n",
	}

	err := writeCodeBlocks(dir, blocks)
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "main.go"))
	if string(data) != "package main\n" {
		t.Errorf("main.go wrong: %q", string(data))
	}

	data, _ = os.ReadFile(filepath.Join(dir, "pkg/utils/util.go"))
	if string(data) != "package util\n" {
		t.Errorf("util.go wrong: %q", string(data))
	}
}

// TestWriteCodeBlocksRejectsTraversal verifies that "../" paths from LLM output
// cannot escape the repo root: the safe sibling is written, the escaping path
// is skipped and reported, and no file lands outside baseDir.
func TestWriteCodeBlocksRejectsTraversal(t *testing.T) {
	parent := t.TempDir()
	baseDir := filepath.Join(parent, "repo")
	os.MkdirAll(baseDir, 0755)

	blocks := map[string]string{
		"safe.go":           "package safe\n",
		"../escape.go":      "package evil\n",
		"../../etc/evil.go": "package evil\n",
	}

	err := writeCodeBlocks(baseDir, blocks)
	if err == nil {
		t.Fatal("expected error reporting skipped unsafe paths")
	}

	// Safe file written inside baseDir.
	if _, err := os.Stat(filepath.Join(baseDir, "safe.go")); err != nil {
		t.Errorf("safe.go should have been written: %v", err)
	}
	// Escaping files must NOT exist outside baseDir.
	if _, err := os.Stat(filepath.Join(parent, "escape.go")); !os.IsNotExist(err) {
		t.Error("../escape.go escaped the repo root")
	}
}

// TestWriteCodeBlocksRejectsAbsolute verifies absolute paths are skipped.
func TestWriteCodeBlocksRejectsAbsolute(t *testing.T) {
	baseDir := t.TempDir()
	target := filepath.Join(t.TempDir(), "abs-evil.go")

	err := writeCodeBlocks(baseDir, map[string]string{target: "package evil\n"})
	if err == nil {
		t.Fatal("expected error for absolute path")
	}
	if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
		t.Error("absolute path was written outside baseDir")
	}
}

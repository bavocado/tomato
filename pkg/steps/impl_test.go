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
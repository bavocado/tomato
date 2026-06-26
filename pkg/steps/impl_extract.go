package steps

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// codeBlockRe matches markdown code fences with an optional filename suffix.
// Supported formats:
//   ```go:main.go          → filename = "main.go"
//   ```main.go             → filename = "main.go"
//   ```go                  → no filename (skipped)
var codeBlockRe = regexp.MustCompile("(?s)```(?:[a-zA-Z0-9]+:)?([^\\n]+)\\n(.*?)```")

// extractCodeBlocks parses markdown and returns a map of filename → code content.
// Only code blocks with a recognizable filename (contains a dot, no spaces) are extracted.
func extractCodeBlocks(markdown string) map[string]string {
	blocks := make(map[string]string)

	matches := codeBlockRe.FindAllStringSubmatch(markdown, -1)
	for _, m := range matches {
		filename := strings.TrimSpace(m[1])
		content := m[2]

		// Only accept filenames that look like paths (contain a dot, no spaces)
		if !strings.Contains(filename, ".") || strings.Contains(filename, " ") {
			continue
		}

		blocks[filename] = content
	}

	return blocks
}

// writeCodeBlocks writes each code block to its path relative to baseDir.
// Creates parent directories as needed.
func writeCodeBlocks(baseDir string, blocks map[string]string) error {
	for filename, content := range blocks {
		fullPath := filepath.Join(baseDir, filename)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return err
		}
	}
	return nil
}
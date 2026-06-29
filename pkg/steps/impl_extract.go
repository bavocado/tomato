package steps

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// codeBlockRe matches markdown code fences with an optional filename suffix.
// Supported formats:
//
//	```go:main.go          → filename = "main.go"
//	```main.go             → filename = "main.go"
//	```go                  → no filename (skipped)
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
//
// Paths come from LLM output and are untrusted: a malicious or hallucinated
// filename could escape the repo (absolute path, or "../" traversal) and
// clobber arbitrary files. Each path is confined to baseDir; anything that
// resolves outside is skipped and reported in the returned error.
func writeCodeBlocks(baseDir string, blocks map[string]string) error {
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return err
	}

	var skipped []string
	for filename, content := range blocks {
		// Reject absolute paths outright.
		if filepath.IsAbs(filename) {
			skipped = append(skipped, filename+" (absolute path)")
			continue
		}
		fullPath := filepath.Join(absBase, filename)
		// Confirm the cleaned path stays under baseDir (guards against "../").
		rel, err := filepath.Rel(absBase, fullPath)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			skipped = append(skipped, filename+" (escapes repo root)")
			continue
		}

		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return err
		}
	}

	if len(skipped) > 0 {
		return fmt.Errorf("skipped %d unsafe code-block path(s): %s", len(skipped), strings.Join(skipped, ", "))
	}
	return nil
}

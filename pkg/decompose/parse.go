package decompose

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseDecomposition extracts the ```yaml manifest block from a decomposition.md
// (section 3, the authoritative machine-readable part) and unmarshals it.
func ParseDecomposition(content string) (*Decomposition, error) {
	block, err := extractYAMLBlock(content)
	if err != nil {
		return nil, err
	}
	var d Decomposition
	if err := yaml.Unmarshal([]byte(block), &d); err != nil {
		return nil, fmt.Errorf("parsing manifest yaml: %w", err)
	}
	return &d, nil
}

// extractYAMLBlock returns the first ```yaml ... ``` fenced block, trimmed.
func extractYAMLBlock(content string) (string, error) {
	const fence = "```yaml"
	idx := strings.Index(content, fence)
	if idx < 0 {
		return "", fmt.Errorf("no ```yaml manifest block found in decomposition.md")
	}
	rest := content[idx+len(fence):]
	end := strings.Index(rest, "```")
	if end < 0 {
		return "", fmt.Errorf("yaml manifest block not terminated (missing closing ```")
	}
	return strings.TrimSpace(rest[:end]), nil
}
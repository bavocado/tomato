package archive

import (
	"fmt"
	"os"
	"path/filepath"
)

// ArchiveTrio moves architecture.md, ui-spec.md, implementation.md into v<N>/.
func ArchiveTrio(featureDir string) (int, error) {
	trio := []string{"architecture.md", "ui-spec.md", "implementation.md"}

	// Auto-increment version
	v := 1
	for {
		vDir := filepath.Join(featureDir, fmt.Sprintf("v%d", v))
		if _, err := os.Stat(vDir); os.IsNotExist(err) {
			break
		}
		v++
	}

	vDir := filepath.Join(featureDir, fmt.Sprintf("v%d", v))
	if err := os.MkdirAll(vDir, 0755); err != nil {
		return 0, fmt.Errorf("creating version dir: %w", err)
	}

	for _, name := range trio {
		src := filepath.Join(featureDir, name)
		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue // skip if trio file doesn't exist
		}
		dst := filepath.Join(vDir, name)
		if err := os.Rename(src, dst); err != nil {
			return 0, fmt.Errorf("archiving %s: %w", name, err)
		}
	}

	return v, nil
}
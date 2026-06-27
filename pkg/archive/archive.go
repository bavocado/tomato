package archive

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ArchiveTrio copies architecture.md, ui-spec.md, implementation.md into v<N>/.
//
// It COPIES (not moves) so the root trio remains in place as the "always
// latest" set (see design §3.5): downstream steps (review, test) read the trio
// from the root, and the §2.8 architecture rewrite writes a fresh
// architecture.md back to the root after archiving. The v<N>/ directory holds
// a frozen snapshot of the design-intent trio for traceability.
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
		if err := copyFile(src, dst); err != nil {
			return 0, fmt.Errorf("archiving %s: %w", name, err)
		}
	}

	return v, nil
}

// copyFile copies src to dst, preserving neither mode nor permissions beyond
// the default file creation mode.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

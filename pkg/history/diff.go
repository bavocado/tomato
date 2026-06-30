package history

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Diff compares a single artifact between two runs and returns a textual diff.
// It reads the stable snapshots written by runner.Execute under
// .tomato/runs/<run-id>/artifacts/<artifact> (design §3.4, Task 5), so the
// comparison is unaffected by later changes to the working-tree outputs.
func Diff(repoDir, runA, runB, artifact string) (string, error) {
	a, err := os.ReadFile(filepath.Join(repoDir, ".tomato", "runs", runA, "artifacts", artifact))
	if err != nil {
		return "", fmt.Errorf("reading artifact %q from run %q: %w", artifact, runA, err)
	}
	b, err := os.ReadFile(filepath.Join(repoDir, ".tomato", "runs", runB, "artifacts", artifact))
	if err != nil {
		return "", fmt.Errorf("reading artifact %q from run %q: %w", artifact, runB, err)
	}
	return simpleDiff(string(a), string(b)), nil
}

// simpleDiff produces a unified-style line diff of two strings. Unlike a naive
// per-line comparison, it reports every differing line position (including
// blank lines: a removed blank line is reported, not silently dropped).
func simpleDiff(a, b string) string {
	if a == b {
		return "(no changes)\n"
	}
	al := strings.Split(a, "\n")
	bl := strings.Split(b, "\n")
	var out strings.Builder
	fmt.Fprintln(&out, "---", "old")
	fmt.Fprintln(&out, "+++", "new")
	max := len(al)
	if len(bl) > max {
		max = len(bl)
	}
	for i := 0; i < max; i++ {
		var av, bv string
		aHas, bHas := i < len(al), i < len(bl)
		if aHas {
			av = al[i]
		}
		if bHas {
			bv = bl[i]
		}
		// Only emit when the position differs. Lines present in one side but
		// not the other (trailing newline differences) count as a change.
		if aHas && bHas {
			if av != bv {
				fmt.Fprintf(&out, "- %s\n", av)
				fmt.Fprintf(&out, "+ %s\n", bv)
			}
		} else if aHas {
			fmt.Fprintf(&out, "- %s\n", av)
		} else if bHas {
			fmt.Fprintf(&out, "+ %s\n", bv)
		}
	}
	return out.String()
}

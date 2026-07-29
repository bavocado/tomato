package decompose

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteOrchestration writes orchestration.yaml (execution plan) into parentDir.
// Order is topo x MoSCoW (spike-first), matching Apply.
func WriteOrchestration(d *Decomposition, parentFeature, parentDir string) error {
	var b strings.Builder
	fmt.Fprintf(&b, "parent_feature: %s\n", parentFeature)
	fmt.Fprintf(&b, "source: %s/decomposition.md\n", parentDir)
	fmt.Fprintf(&b, "# 按依赖拓扑序 × MoSCoW 排序;spike 优先(去风险)\n")
	b.WriteString("order:\n")
	for _, f := range Order(d) {
		fmt.Fprintf(&b, "  - feature: %s\n", subFeatureName(parentFeature, f.ID))
		fmt.Fprintf(&b, "    id: %s\n", f.ID)
		fmt.Fprintf(&b, "    title: %q\n", f.Title)
		fmt.Fprintf(&b, "    priority: %s\n", f.Priority)
		fmt.Fprintf(&b, "    depends_on: [%s]\n", strings.Join(f.DependsOn, ", "))
	}
	return os.WriteFile(filepath.Join(parentDir, "orchestration.yaml"), []byte(b.String()), 0644)
}
package decompose

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Apply materializes each sub-feature under <specsDir>/<parent>-<lowerID>/.
// It writes idea.md, parent-context.md, and parent.json per sub-feature.
// If a target dir exists and force is false, it returns an error (no overwrite).
func Apply(d *Decomposition, parentFeature, specsDir, sourceDesignPath string, force bool) error {
	for _, f := range Order(d) {
		dir := filepath.Join(specsDir, subFeatureName(parentFeature, f.ID))
		if !force {
			if _, err := os.Stat(dir); err == nil {
				return fmt.Errorf("sub-feature dir already exists: %s (use --force to overwrite)", dir)
			}
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating %s: %w", dir, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "idea.md"), []byte(ideaMD(f)), 0644); err != nil {
			return err
		}
		ctx, _ := os.ReadFile(sourceDesignPath) // missing -> empty (caller warns)
		if err := os.WriteFile(filepath.Join(dir, "parent-context.md"), []byte(parentContextMD(f, string(ctx))), 0644); err != nil {
			return err
		}
		pj := fmt.Sprintf(`{"parent_feature":%q,"id":%q,"source":"decomposition.md"}`+"\n", parentFeature, f.ID)
		if err := os.WriteFile(filepath.Join(dir, "parent.json"), []byte(pj), 0644); err != nil {
			return err
		}
	}
	return nil
}

func subFeatureName(parentFeature, id string) string {
	return parentFeature + "-" + strings.ReplaceAll(strings.ToLower(id), "-", "")
}

// ideaMD renders the sub-feature's need as the spec step's input.
func ideaMD(f SubFeature) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", f.Title)
	fmt.Fprintf(&b, "## 目标\n%s\n\n", f.Goal)
	fmt.Fprintf(&b, "## 用户价值\n%s\n\n", f.UserValue)
	fmt.Fprintf(&b, "## 验收标准\n")
	for _, ac := range f.AcceptanceCriteria {
		fmt.Fprintf(&b, "- %s\n", ac)
	}
	fmt.Fprintf(&b, "\n## 不在范围内\n")
	for _, o := range f.OutOfScope {
		fmt.Fprintf(&b, "- %s\n", o)
	}
	fmt.Fprintf(&b, "\n## 优先级\n%s\n\n", f.Priority)
	fmt.Fprintf(&b, "## 依赖\n%s\n", strings.Join(f.DependsOn, ", "))
	return b.String()
}

// parentContextMD renders the sub-feature boundary plus the full parent design.
func parentContextMD(f SubFeature, sourceDesign string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# 子 feature 上下文: %s (%s)\n\n", f.Title, f.ID)
	fmt.Fprintf(&b, "## 本子 feature 的边界(来自父拆解清单)\n")
	b.WriteString(ideaMD(f))
	fmt.Fprintf(&b, "\n## 父 feature 设计文档(完整)\n%s\n", sourceDesign)
	return b.String()
}
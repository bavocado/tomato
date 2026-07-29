package decompose

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestWriteOrchestration(t *testing.T) {
	dir := t.TempDir()
	a := validFeature("F-001")
	b := validFeature("F-002")
	b.DependsOn = []string{"F-001"}
	d := &Decomposition{Features: []SubFeature{b, a}}
	if err := WriteOrchestration(d, "main", dir); err != nil {
		t.Fatalf("WriteOrchestration failed: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "orchestration.yaml"))
	var orch struct {
		ParentFeature string `yaml:"parent_feature"`
		Order         []struct {
			Feature string `yaml:"feature"`
			ID      string `yaml:"id"`
		} `yaml:"order"`
	}
	if err := yaml.Unmarshal(data, &orch); err != nil {
		t.Fatalf("orchestration.yaml not valid yaml: %v\n%s", err, data)
	}
	if orch.ParentFeature != "main" {
		t.Errorf("expected parent_feature main, got %s", orch.ParentFeature)
	}
	if len(orch.Order) != 2 || orch.Order[0].ID != "F-001" || orch.Order[1].ID != "F-002" {
		t.Errorf("expected F-001 then F-002, got %+v", orch.Order)
	}
	if !strings.Contains(string(data), "feature: main-f001") {
		t.Errorf("expected feature: main-f001 in output, got %s", data)
	}
}

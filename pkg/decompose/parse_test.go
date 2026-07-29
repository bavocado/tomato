package decompose

import (
	"strings"
	"testing"
)

func TestParseDecompositionValid(t *testing.T) {
	md := `# Decomposition: X

## 3. Machine-readable manifest
` + "```yaml" + `
source: source-design.md
total_features: 2
dag_check: passed
critical_path: [F-001, F-002]
spikes: []
features:
  - id: F-001
    title: Login
    goal: users log in
    user_value: access account
    slice_type: workflow
    c4_level: container
    priority: must
    depends_on: []
    acceptance_criteria:
      - Given a user When credentials match Then return token
    complexity: M
    is_spike: false
    out_of_scope:
      - OAuth
  - id: F-002
    title: Profile
    goal: view profile
    user_value: see info
    slice_type: workflow
    c4_level: component
    priority: should
    depends_on: [F-001]
    acceptance_criteria:
      - Given logged in When opening profile Then see data
    complexity: S
    is_spike: false
    out_of_scope: []
` + "```" + `
`
	d, err := ParseDecomposition(md)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(d.Features) != 2 {
		t.Fatalf("expected 2 features, got %d", len(d.Features))
	}
	if d.Features[0].ID != "F-001" || d.Features[1].DependsOn[0] != "F-001" {
		t.Errorf("parsed fields wrong: %+v", d.Features)
	}
}

func TestParseDecompositionNoYAMLBlock(t *testing.T) {
	_, err := ParseDecomposition("# Decomposition\n\nno block here")
	if err == nil || !strings.Contains(err.Error(), "no ```yaml manifest block") {
		t.Fatalf("expected missing-block error, got %v", err)
	}
}

func TestParseDecompositionUnterminatedBlock(t *testing.T) {
	_, err := ParseDecomposition("# D\n\n" + "```yaml\nfeatures: []\n")
	if err == nil || !strings.Contains(err.Error(), "not terminated") {
		t.Fatalf("expected unterminated error, got %v", err)
	}
}

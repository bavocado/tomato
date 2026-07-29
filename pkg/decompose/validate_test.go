package decompose

import (
	"strings"
	"testing"
)

func validFeature(id string) SubFeature {
	return SubFeature{
		ID: id, Title: "t", Goal: "g", UserValue: "v",
		SliceType: "workflow", C4Level: "container", Priority: "must",
		AcceptanceCriteria: []string{"ac"}, Complexity: "M", OutOfScope: []string{},
	}
}

func TestValidateOK(t *testing.T) {
	d := &Decomposition{Features: []SubFeature{validFeature("F-001")}}
	if err := Validate(d); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidateDuplicateID(t *testing.T) {
	d := &Decomposition{Features: []SubFeature{validFeature("F-001"), validFeature("F-001")}}
	err := Validate(d)
	if err == nil || !strings.Contains(err.Error(), "duplicate id") {
		t.Fatalf("expected duplicate-id error, got %v", err)
	}
}

func TestValidateMissingFields(t *testing.T) {
	f := validFeature("F-001")
	f.Goal = ""
	d := &Decomposition{Features: []SubFeature{f}}
	err := Validate(d)
	if err == nil || !strings.Contains(err.Error(), "goal is empty") {
		t.Fatalf("expected empty-goal error, got %v", err)
	}
}

func TestValidateDanglingDep(t *testing.T) {
	f := validFeature("F-001")
	f.DependsOn = []string{"F-999"}
	d := &Decomposition{Features: []SubFeature{f}}
	err := Validate(d)
	if err == nil || !strings.Contains(err.Error(), `F-999" does not exist`) {
		t.Fatalf("expected dangling-dep error, got %v", err)
	}
}

func TestValidateCycle(t *testing.T) {
	a := validFeature("F-001")
	a.DependsOn = []string{"F-002"}
	b := validFeature("F-002")
	b.DependsOn = []string{"F-001"}
	d := &Decomposition{Features: []SubFeature{a, b}}
	err := Validate(d)
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("expected cycle error, got %v", err)
	}
}

func TestValidateSpikeNoTimebox(t *testing.T) {
	f := validFeature("F-001")
	f.IsSpike = true
	d := &Decomposition{Features: []SubFeature{f}}
	err := Validate(d)
	if err == nil || !strings.Contains(err.Error(), "spike missing timebox") {
		t.Fatalf("expected spike-timebox error, got %v", err)
	}
}

package decompose

import "testing"

func TestOrderTopological(t *testing.T) {
	// F-002 depends on F-001; F-003 depends on F-001. F-001 must come first.
	a := validFeature("F-001")
	b := validFeature("F-002")
	b.DependsOn = []string{"F-001"}
	b.Priority = "must"
	c := validFeature("F-003")
	c.DependsOn = []string{"F-001"}
	c.Priority = "must"
	d := &Decomposition{Features: []SubFeature{b, a, c}} // shuffled input
	got := Order(d)
	if got[0].ID != "F-001" {
		t.Fatalf("F-001 must be first, got %s", got[0].ID)
	}
	// F-002 and F-003 both depend only on F-001; both ready after it.
	ids := []string{got[1].ID, got[2].ID}
	if ids[0] != "F-002" && ids[0] != "F-003" {
		t.Fatalf("expected F-002/F-003 next, got %v", ids)
	}
}

func TestOrderMoSCoWTieBreak(t *testing.T) {
	// Two independent features: must should come before should.
	must := validFeature("F-001")
	must.Priority = "must"
	should := validFeature("F-002")
	should.Priority = "should"
	d := &Decomposition{Features: []SubFeature{should, must}}
	got := Order(d)
	if got[0].ID != "F-001" || got[1].ID != "F-002" {
		t.Fatalf("expected must(F-001) before should(F-002), got %s %s", got[0].ID, got[1].ID)
	}
}

func TestOrderSpikeFirst(t *testing.T) {
	// Two musts, one is a spike -> spike first.
	spike := validFeature("F-001")
	spike.IsSpike = true
	spike.Priority = "must"
	impl := validFeature("F-002")
	impl.Priority = "must"
	impl.DependsOn = []string{"F-001"}
	d := &Decomposition{Features: []SubFeature{impl, spike}}
	got := Order(d)
	if got[0].ID != "F-001" {
		t.Fatalf("expected spike F-001 first, got %s", got[0].ID)
	}
}

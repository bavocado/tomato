package adapter

import "testing"

func TestRegistryFor(t *testing.T) {
	r := NewRegistry()
	prBridge := &Bridge{Bin: "pr-adapter"}
	r.Set("pr", prBridge)

	if got := r.For("pr"); got != prBridge {
		t.Errorf("For(pr) = %v, want %v", got, prBridge)
	}
	if got := r.For("task"); got != nil {
		t.Errorf("For(task) = %v, want nil (unconfigured)", got)
	}
}

func TestRegistryForNilSafe(t *testing.T) {
	var r *Registry
	if r.For("pr") != nil {
		t.Error("For on nil Registry should return nil")
	}
	if r.ForAny("pr", "review") != nil {
		t.Error("ForAny on nil Registry should return nil")
	}
}

func TestRegistryForAnyPriority(t *testing.T) {
	r := NewRegistry()
	reviewBridge := &Bridge{Bin: "review-adapter"}
	r.Set("review", reviewBridge)

	// "pr" not configured, should fall back to "review".
	if got := r.ForAny("pr", "review"); got != reviewBridge {
		t.Errorf("ForAny(pr, review) = %v, want review bridge", got)
	}

	// "pr" configured: it wins over "review".
	prBridge := &Bridge{Bin: "pr-adapter"}
	r.Set("pr", prBridge)
	if got := r.ForAny("pr", "review"); got != prBridge {
		t.Errorf("ForAny(pr, review) = %v, want pr bridge (higher priority)", got)
	}

	// none configured.
	if got := r.ForAny("nope", "nada"); got != nil {
		t.Errorf("ForAny with no matches = %v, want nil", got)
	}
}

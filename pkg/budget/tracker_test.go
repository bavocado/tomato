package budget

import (
	"testing"
)

func TestTrackerBasicRecord(t *testing.T) {
	tracker := NewTracker()

	tracker.Record("spec", 500, 200)
	tracker.Record("design", 1000, 500)
	tracker.Record("impl", 800, 300)

	if tracker.TotalIn() != 2300 {
		t.Errorf("expected 2300 tokens in, got %d", tracker.TotalIn())
	}

	if tracker.TotalOut() != 1000 {
		t.Errorf("expected 1000 tokens out, got %d", tracker.TotalOut())
	}
}

func TestTrackerPerStepBudgetPass(t *testing.T) {
	tracker := NewTracker()
	tracker.SetPerStepBudget("spec", 50000)

	tracker.Record("spec", 30000, 0)

	ok := tracker.Check("spec", 15000)
	if !ok {
		t.Error("expected check to pass (45000 <= 50000)")
	}
}

func TestTrackerPerStepBudgetFail(t *testing.T) {
	tracker := NewTracker()
	tracker.SetPerStepBudget("spec", 50000)

	tracker.Record("spec", 40000, 0)

	ok := tracker.Check("spec", 15000)
	if ok {
		t.Error("expected check to fail (55000 > 50000)")
	}
}

func TestTrackerPerStepNoBudget(t *testing.T) {
	tracker := NewTracker()
	// No budget set for "impl" — should always pass
	tracker.Record("impl", 999999, 0)

	ok := tracker.Check("impl", 999999)
	if !ok {
		t.Error("expected check to pass when no per-step budget is configured")
	}
}

func TestTrackerGlobalBudgetPass(t *testing.T) {
	tracker := NewTracker()
	tracker.SetGlobalBudget(100000)

	tracker.Record("spec", 40000, 0)

	ok := tracker.CheckGlobal(50000)
	if !ok {
		t.Error("expected global check to pass (40000+50000=90000 <= 100000)")
	}
}

func TestTrackerGlobalBudgetFail(t *testing.T) {
	tracker := NewTracker()
	tracker.SetGlobalBudget(100000)

	tracker.Record("spec", 80000, 0)

	ok := tracker.CheckGlobal(50000)
	if ok {
		t.Error("expected global check to fail (80000+50000=130000 > 100000)")
	}
}

func TestTrackerGlobalBudgetZero(t *testing.T) {
	tracker := NewTracker()
	// Global budget is 0 by default — always pass
	ok := tracker.CheckGlobal(999999)
	if !ok {
		t.Error("expected global check to pass when budget is 0 (unlimited)")
	}
}

func TestTrackerInitFromConfig(t *testing.T) {
	tracker := NewTracker()

	perStep := map[string]int{"spec": 50000, "design": 100000}
	tracker.InitFromConfig("balanced", perStep, 300000, "degrade", "deepseek/deepseek-4pro")

	if tracker.OnExceed() != "degrade" {
		t.Errorf("expected on_exceed 'degrade', got '%s'", tracker.OnExceed())
	}

	ok := tracker.Check("spec", 50000)
	if !ok {
		t.Error("expected check to pass (0+50000 <= 50000)")
	}

	ok = tracker.CheckGlobal(300000)
	if !ok {
		t.Error("expected global check to pass (0+300000 <= 300000)")
	}
}

func TestTrackerInitFromConfigDefaults(t *testing.T) {
	tracker := NewTracker()
	tracker.InitFromConfig("", nil, 0, "", "")
	// Should default to "warn"
	if tracker.OnExceed() != "warn" {
		t.Errorf("expected default on_exceed 'warn', got '%s'", tracker.OnExceed())
	}
}

func TestEstimateTokens(t *testing.T) {
	text := "Hello world, this is a test with some tokens to estimate."
	estimated := EstimateTokens(text)
	if estimated < 5 || estimated > 30 {
		t.Errorf("expected reasonable estimate (5-30), got %d for text len=%d", estimated, len(text))
	}

	// Empty string
	if EstimateTokens("") != 0 {
		t.Error("expected 0 for empty string")
	}

	// Very short string
	if EstimateTokens("ab") != 0 {
		t.Error("expected 0 for very short string (len/4 = 0)")
	}

	// Exactly 4 chars
	if EstimateTokens("abcd") != 1 {
		t.Errorf("expected 1 for 'abcd', got %d", EstimateTokens("abcd"))
	}
}

func TestTrackerMultithreadSafe(t *testing.T) {
	tracker := NewTracker()

	done := make(chan bool, 2)
	go func() {
		for i := 0; i < 100; i++ {
			tracker.Record("spec", 1, 1)
		}
		done <- true
	}()
	go func() {
		for i := 0; i < 100; i++ {
			tracker.Check("spec", 1)
			tracker.CheckGlobal(1)
			_ = tracker.TotalIn()
			_ = tracker.TotalOut()
			_ = tracker.OnExceed()
		}
		done <- true
	}()

	<-done
	<-done

	if tracker.TotalIn() != 100 {
		t.Errorf("expected 100 total in, got %d", tracker.TotalIn())
	}
}

func TestTrackerOnExceedSetAndGet(t *testing.T) {
	tracker := NewTracker()

	tracker.SetOnExceed("fail")
	if tracker.OnExceed() != "fail" {
		t.Errorf("expected 'fail', got '%s'", tracker.OnExceed())
	}

	tracker.SetOnExceed("degrade")
	if tracker.OnExceed() != "degrade" {
		t.Errorf("expected 'degrade', got '%s'", tracker.OnExceed())
	}
}

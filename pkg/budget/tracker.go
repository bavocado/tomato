package budget

import "sync"

// Tracker tracks token usage and enforces budgets.
type Tracker struct {
	mu             sync.Mutex
	totalIn        int
	totalOut       int
	perStepIn      map[string]int
	perStepBudget  map[string]int
	globalBudget   int
	onExceed       string
	degradeToModel string
}

// NewTracker creates a new token tracker.
func NewTracker() *Tracker {
	return &Tracker{
		perStepIn:     make(map[string]int),
		perStepBudget: make(map[string]int),
		onExceed:      "warn",
	}
}

// Record adds a step's token usage.
func (t *Tracker) Record(stepName string, tokensIn, tokensOut int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.totalIn += tokensIn
	t.totalOut += tokensOut
	t.perStepIn[stepName] += tokensIn
}

// Check returns true if adding tokensIn for stepName stays within budget.
func (t *Tracker) Check(stepName string, tokensIn int) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	budget, ok := t.perStepBudget[stepName]
	if !ok {
		return true
	}
	return t.perStepIn[stepName]+tokensIn <= budget
}

// CheckGlobal returns true if adding tokens stays within global budget.
func (t *Tracker) CheckGlobal(tokens int) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.globalBudget == 0 {
		return true
	}
	return t.totalIn+tokens <= t.globalBudget
}

// SetPerStepBudget configures the per-step budget.
func (t *Tracker) SetPerStepBudget(step string, budget int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.perStepBudget[step] = budget
}

// SetGlobalBudget configures the per-run budget.
func (t *Tracker) SetGlobalBudget(budget int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.globalBudget = budget
}

// SetOnExceed configures the on-exceed strategy.
func (t *Tracker) SetOnExceed(strategy string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onExceed = strategy
}

// InitFromConfig configures the tracker from a budget config struct.
func (t *Tracker) InitFromConfig(mode string, perStep map[string]int, global int, onExceed, degradeTo string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.globalBudget = global
	t.onExceed = onExceed
	t.degradeToModel = degradeTo
	for step, budget := range perStep {
		t.perStepBudget[step] = budget
	}

	if t.onExceed == "" {
		t.onExceed = "warn"
	}
}

// EstimateTokens returns a rough token estimate (4 chars ≈ 1 token).
func EstimateTokens(text string) int {
	return len(text) / 4
}

// OnExceed returns the on-exceed strategy.
func (t *Tracker) OnExceed() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.onExceed
}

// TotalIn returns cumulative input tokens.
func (t *Tracker) TotalIn() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.totalIn
}

// TotalOut returns cumulative output tokens.
func (t *Tracker) TotalOut() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.totalOut
}
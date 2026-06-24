package cost

import (
	"fmt"
	"strings"

	"github.com/bavocado/tomato/pkg/history"
)

// StepCost tracks tokens for a single step type.
type StepCost struct {
	Count     int
	TokensIn  int
	TokensOut int
}

// Summary computes cumulative token usage across all runs.
type Summary struct {
	TotalIn  int
	TotalOut int
	RunCount int
	ByStep   map[string]StepCost
}

// Compute reads all runs and returns a cumulative summary.
func Compute(repoDir string) (*Summary, error) {
	runs, err := history.List(repoDir)
	if err != nil {
		return nil, err
	}

	s := &Summary{
		ByStep: make(map[string]StepCost),
	}

	for _, r := range runs {
		s.TotalIn += r.TokensIn
		s.TotalOut += r.TokensOut
		s.RunCount++

		sc := s.ByStep[r.StepName]
		sc.Count++
		sc.TokensIn += r.TokensIn
		sc.TokensOut += r.TokensOut
		s.ByStep[r.StepName] = sc
	}

	return s, nil
}

// Format returns a human-readable cost report.
func (s *Summary) Format() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Token Usage Summary\n")
	fmt.Fprintf(&b, "===================\n")
	fmt.Fprintf(&b, "Total runs:  %d\n", s.RunCount)
	fmt.Fprintf(&b, "Tokens in:   %d\n", s.TotalIn)
	fmt.Fprintf(&b, "Tokens out:  %d\n", s.TotalOut)
	fmt.Fprintf(&b, "Total:       %d\n\n", s.TotalIn+s.TotalOut)

	if len(s.ByStep) > 0 {
		fmt.Fprintf(&b, "By step:\n")
		for step, sc := range s.ByStep {
			fmt.Fprintf(&b, "  %-10s %3d runs  %6d in  %6d out\n", step, sc.Count, sc.TokensIn, sc.TokensOut)
		}
	}

	return b.String()
}
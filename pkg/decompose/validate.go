package decompose

import (
	"fmt"
	"strings"
)

// Validate runs the Definition-of-Ready checks: unique ids, existing dependency
// refs, acyclic depends_on (DAG), spike timebox, required fields non-empty.
// Returns nil if all pass, else an error listing every problem found.
func Validate(d *Decomposition) error {
	var problems []string

	seen := map[string]bool{}
	for i, f := range d.Features {
		if f.ID == "" {
			problems = append(problems, fmt.Sprintf("feature %d: id is empty", i))
			continue
		}
		if seen[f.ID] {
			problems = append(problems, fmt.Sprintf("duplicate id %q", f.ID))
		}
		seen[f.ID] = true
	}

	for _, f := range d.Features {
		if f.ID == "" {
			continue
		}
		if f.Title == "" {
			problems = append(problems, fmt.Sprintf("%s: title is empty", f.ID))
		}
		if f.Goal == "" {
			problems = append(problems, fmt.Sprintf("%s: goal is empty", f.ID))
		}
		if f.UserValue == "" {
			problems = append(problems, fmt.Sprintf("%s: user_value is empty", f.ID))
		}
		if f.SliceType == "" {
			problems = append(problems, fmt.Sprintf("%s: slice_type is empty", f.ID))
		}
		if f.C4Level == "" {
			problems = append(problems, fmt.Sprintf("%s: c4_level is empty", f.ID))
		}
		if f.Priority == "" {
			problems = append(problems, fmt.Sprintf("%s: priority is empty", f.ID))
		}
		if f.Complexity == "" {
			problems = append(problems, fmt.Sprintf("%s: complexity is empty", f.ID))
		}
		if len(f.AcceptanceCriteria) == 0 {
			problems = append(problems, fmt.Sprintf("%s: acceptance_criteria is empty", f.ID))
		}
		if f.IsSpike && strings.TrimSpace(f.Timebox) == "" {
			problems = append(problems, fmt.Sprintf("%s: spike missing timebox", f.ID))
		}
		for _, dep := range f.DependsOn {
			if !seen[dep] {
				problems = append(problems, fmt.Sprintf("%s: depends_on %q does not exist", f.ID, dep))
			}
		}
	}

	if cycle := detectCycle(d); cycle != "" {
		problems = append(problems, fmt.Sprintf("dependency cycle: %s", cycle))
	}

	if len(problems) > 0 {
		return fmt.Errorf("DoR validation failed:\n  - %s", strings.Join(problems, "\n  - "))
	}
	return nil
}

// detectCycle returns a human-readable cycle chain (a -> b -> a) if the
// depends_on graph has a cycle, "" otherwise. Edge a->dep means a depends on dep.
func detectCycle(d *Decomposition) string {
	adj := map[string][]string{}
	for _, f := range d.Features {
		adj[f.ID] = f.DependsOn
	}
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := map[string]int{}
	var stack []string
	var dfs func(node string) []string
	dfs = func(node string) []string {
		color[node] = gray
		stack = append(stack, node)
		for _, dep := range adj[node] {
			switch color[dep] {
			case gray:
				for i, n := range stack {
					if n == dep {
						cycle := append([]string{}, stack[i:]...)
						cycle = append(cycle, dep)
						return cycle
					}
				}
			case white:
				if c := dfs(dep); c != nil {
					return c
				}
			}
		}
		stack = stack[:len(stack)-1]
		color[node] = black
		return nil
	}
	for _, f := range d.Features {
		if color[f.ID] == white {
			if c := dfs(f.ID); c != nil {
				return strings.Join(c, " -> ")
			}
		}
	}
	return ""
}

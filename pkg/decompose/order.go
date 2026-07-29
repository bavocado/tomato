package decompose

import "sort"

// priorityRank orders MoSCoW buckets for sorting (lower = earlier).
func priorityRank(p string) int {
	switch p {
	case "must":
		return 0
	case "should":
		return 1
	case "could":
		return 2
	default:
		return 3 // "won't" and anything else last
	}
}

// Order returns sub-features in execution order: topological (dependencies
// first), ties broken by MoSCoW priority, then spikes before non-spikes, then
// id for stability. Uses Kahn's algorithm layered by "all deps satisfied".
func Order(d *Decomposition) []SubFeature {
	deps := map[string]int{}            // remaining unsatisfied deps per id
	dependents := map[string][]string{} // dep -> ids that need it
	for _, f := range d.Features {
		deps[f.ID] = len(f.DependsOn)
		for _, dep := range f.DependsOn {
			dependents[dep] = append(dependents[dep], f.ID)
		}
	}

	var ordered []SubFeature
	remaining := append([]SubFeature{}, d.Features...)
	for len(remaining) > 0 {
		var ready, next []SubFeature
		for _, f := range remaining {
			if deps[f.ID] == 0 {
				ready = append(ready, f)
			} else {
				next = append(next, f)
			}
		}
		sort.SliceStable(ready, func(i, j int) bool {
			ri, rj := priorityRank(ready[i].Priority), priorityRank(ready[j].Priority)
			if ri != rj {
				return ri < rj
			}
			if ready[i].IsSpike != ready[j].IsSpike {
				return ready[i].IsSpike // spikes first
			}
			return ready[i].ID < ready[j].ID
		})
		for _, f := range ready {
			ordered = append(ordered, f)
			for _, dep := range dependents[f.ID] {
				deps[dep]--
			}
		}
		if len(ready) == 0 {
			// No progress (cycle) — Validate should have caught it; append rest as-is.
			ordered = append(ordered, next...)
			break
		}
		remaining = next
	}
	return ordered
}

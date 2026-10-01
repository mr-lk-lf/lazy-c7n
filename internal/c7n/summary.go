package c7n

import (
	"sort"
)

// RunSummary condenses the results of every policy of one run.
type RunSummary struct {
	Policies    int // policy results (one per policy and region)
	WithMatches int // results with at least one resource
	Resources   int // resources matched, all results together
	ByType      []Count
	Regions     []string

	// Resources matched by policies whose actions are destructive /
	// mutating (a policy with both counts as destructive), and which
	// actions those are.
	DestructiveResources int
	DestructiveActions   []Count
	MutatingResources    int
	MutatingActions      []Count

	Errors   []string // policies that failed
	Deployed []string // non-pull policies a live run deployed
	DryRun   bool     // every result was a dry run
}

// Count is a name and how many.
type Count struct {
	Name string
	N    int
}

// Summarize builds the summary of one run's policy results.
func Summarize(runs []PolicyRun) RunSummary {
	s := RunSummary{Policies: len(runs), DryRun: len(runs) > 0}
	byType := map[string]int{}
	regions := map[string]bool{}
	destructive := map[string]int{}
	mutating := map[string]int{}
	for _, r := range runs {
		s.DryRun = s.DryRun && r.DryRun
		if r.Region != "" {
			regions[r.Region] = true
		}
		switch r.Status() {
		case "error":
			s.Errors = append(s.Errors, r.Policy)
		case "deployed":
			s.Deployed = append(s.Deployed, r.Policy)
		}
		n := max(r.ResourceCount, 0)
		if n == 0 {
			continue
		}
		s.WithMatches++
		s.Resources += n
		byType[r.Resource] += n

		var hasDestructive, hasMutating bool
		for _, a := range r.Actions {
			switch ClassifyAction(a) {
			case ActionDestructive:
				hasDestructive = true
				destructive[a] += n
			case ActionMutating:
				hasMutating = true
				mutating[a] += n
			case ActionNotify:
			}
		}
		switch {
		case hasDestructive:
			s.DestructiveResources += n
		case hasMutating:
			s.MutatingResources += n
		}
	}
	s.ByType = sortedCounts(byType)
	s.DestructiveActions = sortedCounts(destructive)
	s.MutatingActions = sortedCounts(mutating)
	for r := range regions {
		s.Regions = append(s.Regions, r)
	}
	sort.Strings(s.Regions)
	return s
}

// sortedCounts orders counts by size, then name.
func sortedCounts(m map[string]int) []Count {
	out := make([]Count, 0, len(m))
	for name, n := range m {
		out = append(out, Count{name, n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].N != out[j].N {
			return out[i].N > out[j].N
		}
		return out[i].Name < out[j].Name
	})
	return out
}

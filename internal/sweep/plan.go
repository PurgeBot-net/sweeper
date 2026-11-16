package sweep

import (
	"fmt"
	"slices"
	"time"
)

const (
	retention = 30 * 24 * time.Hour
	// Caps the GetGuild probes, whose 403s count toward Discord's invalid-request limit.
	maxDeletionsPerRun = 100
)

type sweepPlan struct {
	gone []int64
	due  []int64
}

func rejoined(listed map[int64]struct{}, departed map[int64]time.Time) []int64 {
	var back []int64
	for id := range departed {
		if _, ok := listed[id]; ok {
			back = append(back, id)
		}
	}
	return back
}

func plan(listed map[int64]struct{}, stored []int64, departed map[int64]time.Time, now time.Time) sweepPlan {
	var p sweepPlan
	marked := make(map[int64]struct{})
	for _, id := range stored {
		if _, ok := listed[id]; ok {
			continue
		}
		if _, ok := departed[id]; ok {
			continue
		}
		if _, ok := marked[id]; !ok {
			marked[id] = struct{}{}
			p.gone = append(p.gone, id)
		}
	}

	cutoff := now.Add(-retention)
	for id, leftAt := range departed {
		if _, ok := listed[id]; !ok && leftAt.Before(cutoff) {
			p.due = append(p.due, id)
		}
	}
	slices.SortFunc(p.due, func(a, b int64) int { return departed[a].Compare(departed[b]) })
	if len(p.due) > maxDeletionsPerRun {
		p.due = p.due[:maxDeletionsPerRun]
	}
	return p
}

// A short list is the one way to mark present servers as gone, so under 90% of Discord's count is refused.
func listingComplete(listed int, approx *int) bool {
	return approx != nil && listed*10 >= *approx*9
}

// listGuildIDs pages by the highest ID seen, so it does not depend on the order Discord returns a page in.
func listGuildIDs(fetch func(after int64) ([]int64, error)) (map[int64]struct{}, error) {
	listed := make(map[int64]struct{})
	var after int64
	for {
		page, err := fetch(after)
		if err != nil {
			return listed, err
		}
		if len(page) == 0 {
			return listed, nil
		}
		next := after
		for _, id := range page {
			listed[id] = struct{}{}
			next = max(next, id)
		}
		if next == after {
			return listed, fmt.Errorf("guild list did not advance past %d", after)
		}
		after = next
	}
}

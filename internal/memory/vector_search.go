package memory

import (
	"sort"
	"time"
)

// ArchivalResult represents a single result from archival memory search.
type ArchivalResult struct {
	Content   string
	Tags      []string
	Score     float32
	CreatedAt time.Time
}

// rrfK is the standard Reciprocal Rank Fusion constant.
const rrfK = 60

// rankedItem holds an item ID and its position in a single ranked list.
type rankedItem struct {
	id    int // index into a results slice
	score float32
}

// rrfFuse merges multiple ranked lists using Reciprocal Rank Fusion.
// Each input list is a slice of item IDs ordered by relevance (best first).
// Returns a map of item ID → fused RRF score.
func rrfFuse(rankedLists ...[]int) map[int]float32 {
	scores := make(map[int]float32)
	for _, list := range rankedLists {
		for rank, id := range list {
			// rank is 0-based; RRF uses 1-based ranks
			scores[id] += 1.0 / float32(rrfK+rank+1)
		}
	}
	return scores
}

// sortByScoreDesc sorts ArchivalResult slice by Score descending.
func sortByScoreDesc(results []ArchivalResult) {
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
}

package merger

import (
	"cmp"
	"slices"

	"github.com/personal-know/internal/domain"
)

type ScoreMerger struct{}

func New() *ScoreMerger { return &ScoreMerger{} }

func (m *ScoreMerger) Merge(hits []domain.SearchHit, limit int) []domain.SearchHit {
	seen := make(map[string]int)
	var merged []domain.SearchHit

	for _, h := range hits {
		if idx, ok := seen[h.Item.ID]; ok {
			if h.Score > merged[idx].Score {
				merged[idx] = h
			}
			continue
		}
		seen[h.Item.ID] = len(merged)
		merged = append(merged, h)
	}

	slices.SortFunc(merged, func(a, b domain.SearchHit) int {
		return cmp.Compare(b.Score, a.Score)
	})

	if limit > 0 && len(merged) > limit {
		merged = merged[:limit]
	}
	return merged
}

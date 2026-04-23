package keyword

import (
	"cmp"
	"context"
	"slices"
	"strings"

	"github.com/personal-know/internal/domain"
	"github.com/personal-know/internal/port"
)

type Retriever struct {
	store      port.Store
	fetchLimit int
}

func New(store port.Store, fetchLimit int) *Retriever {
	if fetchLimit <= 0 {
		fetchLimit = 500
	}
	return &Retriever{store: store, fetchLimit: fetchLimit}
}

func (r *Retriever) Name() string { return "keyword" }

func (r *Retriever) Search(ctx context.Context, query domain.SearchQuery) ([]domain.SearchHit, error) {
	limit := query.Limit
	if limit <= 0 {
		limit = 5
	}

	items, err := r.store.List(ctx, query.OwnerID, 0, r.fetchLimit)
	if err != nil {
		return nil, err
	}

	words := strings.Fields(strings.ToLower(query.Text))
	if len(words) == 0 {
		return nil, nil
	}

	var hits []domain.SearchHit
	for _, item := range items {
		if item.Status != domain.StatusActive {
			continue
		}
		score := calcScore(item, words)
		if score > 0 {
			hits = append(hits, domain.SearchHit{
				Item:      *item,
				Score:     score,
				Retriever: "keyword",
			})
		}
	}

	slices.SortFunc(hits, func(a, b domain.SearchHit) int {
		return cmp.Compare(b.Score, a.Score)
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, nil
}

func calcScore(item *domain.Knowledge, words []string) float64 {
	titleLower := strings.ToLower(item.Title)
	contentLower := strings.ToLower(item.Content)
	tagsLower := strings.ToLower(strings.Join(item.Tags, " "))

	var score float64
	for _, w := range words {
		if strings.Contains(titleLower, w) {
			score += 2.0
		}
		if strings.Contains(contentLower, w) {
			score += 1.0
		}
		if strings.Contains(tagsLower, w) {
			score += 1.5
		}
	}

	maxScore := float64(len(words)) * 4.5
	if maxScore == 0 {
		return 0
	}
	return score / maxScore
}


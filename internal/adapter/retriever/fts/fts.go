package fts

import (
	"context"

	"github.com/personal-know/internal/adapter/store/pgstore"
	"github.com/personal-know/internal/domain"
)

type Retriever struct {
	pg *pgstore.PgStore
}

func New(pg *pgstore.PgStore) *Retriever {
	return &Retriever{pg: pg}
}

func (r *Retriever) Name() string { return "fts" }

func (r *Retriever) Search(ctx context.Context, query domain.SearchQuery) ([]domain.SearchHit, error) {
	limit := query.Limit
	if limit <= 0 {
		limit = 5
	}

	results, err := r.pg.SearchByFTS(ctx, query.OwnerID, query.Text, limit)
	if err != nil {
		return nil, err
	}

	hits := make([]domain.SearchHit, 0, len(results))
	for _, r := range results {
		hits = append(hits, domain.SearchHit{
			Item:      r.Item,
			Score:     r.Score,
			Retriever: "fts",
		})
	}
	return hits, nil
}

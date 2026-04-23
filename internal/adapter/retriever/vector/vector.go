package vector

import (
	"context"

	"github.com/personal-know/internal/adapter/store/pgstore"
	"github.com/personal-know/internal/domain"
	"github.com/personal-know/internal/port"
)

type Retriever struct {
	pg             *pgstore.PgStore
	embedder       port.Embedder
	scoreThreshold float64
}

func New(pg *pgstore.PgStore, embedder port.Embedder, scoreThreshold float64) *Retriever {
	if scoreThreshold <= 0 {
		scoreThreshold = 0.7
	}
	return &Retriever{pg: pg, embedder: embedder, scoreThreshold: scoreThreshold}
}

func (r *Retriever) Name() string { return "vector" }

func (r *Retriever) Search(ctx context.Context, query domain.SearchQuery) ([]domain.SearchHit, error) {
	limit := query.Limit
	if limit <= 0 {
		limit = 5
	}

	embedding, err := r.embedder.Embed(ctx, query.Text)
	if err != nil {
		return nil, err
	}

	results, err := r.pg.SearchByVector(ctx, query.OwnerID, embedding, limit, r.scoreThreshold)
	if err != nil {
		return nil, err
	}

	hits := make([]domain.SearchHit, 0, len(results))
	for _, r := range results {
		hits = append(hits, domain.SearchHit{
			Item:      r.Item,
			Score:     r.Score,
			Retriever: "vector",
		})
	}
	return hits, nil
}

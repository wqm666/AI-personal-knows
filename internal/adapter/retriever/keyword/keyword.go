package keyword

import (
	"context"
	"strings"

	"github.com/personal-know/internal/adapter/store/pgstore"
	"github.com/personal-know/internal/domain"
)

type Retriever struct {
	pg         *pgstore.PgStore
	fetchLimit int
}

func New(pg *pgstore.PgStore, fetchLimit int) *Retriever {
	if fetchLimit <= 0 {
		fetchLimit = 20
	}
	return &Retriever{pg: pg, fetchLimit: fetchLimit}
}

func (r *Retriever) Name() string { return "keyword" }

func (r *Retriever) Search(ctx context.Context, query domain.SearchQuery) ([]domain.SearchHit, error) {
	limit := query.Limit
	if limit <= 0 {
		limit = r.fetchLimit
	}

	words := strings.Fields(strings.ToLower(query.Text))
	if len(words) == 0 {
		return nil, nil
	}

	results, err := r.pg.SearchByKeyword(ctx, query.OwnerID, words, limit)
	if err != nil {
		return nil, err
	}

	hits := make([]domain.SearchHit, 0, len(results))
	for _, h := range results {
		hits = append(hits, domain.SearchHit{
			Item:      h.Item,
			Score:     h.Score,
			Retriever: "keyword",
		})
	}
	return hits, nil
}

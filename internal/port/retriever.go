package port

import (
	"context"

	"github.com/personal-know/internal/domain"
)

type Retriever interface {
	Name() string
	Search(ctx context.Context, query domain.SearchQuery) ([]domain.SearchHit, error)
}

type Orchestrator interface {
	Search(ctx context.Context, query domain.SearchQuery) ([]domain.SearchHit, error)
	Register(r Retriever)
}

type Merger interface {
	Merge(hits []domain.SearchHit, limit int) []domain.SearchHit
}

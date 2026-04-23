package port

import (
	"context"

	"github.com/personal-know/internal/domain"
)

type Deduplicator interface {
	Check(ctx context.Context, ownerID, content string, embedding []float64) (*domain.DedupResult, error)
}

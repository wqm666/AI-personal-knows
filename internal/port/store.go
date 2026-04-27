package port

import (
	"context"

	"github.com/personal-know/internal/domain"
)

type Store interface {
	Save(ctx context.Context, k *domain.Knowledge) error
	Get(ctx context.Context, ownerID, id string) (*domain.Knowledge, error)
	Update(ctx context.Context, k *domain.Knowledge) error
	Delete(ctx context.Context, ownerID, id string) error

	List(ctx context.Context, ownerID string, offset, limit int) ([]*domain.Knowledge, error)
	ListByStatus(ctx context.Context, ownerID, status string, offset, limit int) ([]*domain.Knowledge, error)
	ListByIDs(ctx context.Context, ownerID string, ids []string) ([]*domain.Knowledge, error)

	IncrementHitCount(ctx context.Context, ownerID, id string) error
	IncrementUsefulCount(ctx context.Context, ownerID, id string) error
	UpdateRelatedIDs(ctx context.Context, ownerID, id string, relatedIDs []string) error
	UpdateSupersededBy(ctx context.Context, ownerID, id string, supersededBy string) error
	UpdateStatus(ctx context.Context, ownerID, id string, status string) error
	UpdateTags(ctx context.Context, ownerID, id string, tags []string) error
	UpdateEmbedding(ctx context.Context, ownerID, id string, embedding []float64) error
	UpdateReviewStatus(ctx context.Context, ownerID, id, reviewStatus string, confidence int, reason string) error

	ListByReviewStatus(ctx context.Context, ownerID, reviewStatus string, offset, limit int) ([]*domain.Knowledge, error)

	Count(ctx context.Context, ownerID string) (int, error)
	CountByStatus(ctx context.Context, ownerID, status string) (int, error)
	CountByReviewStatus(ctx context.Context, ownerID, reviewStatus string) (int, error)

	ListWithoutEmbedding(ctx context.Context, ownerID string, limit int) ([]*domain.Knowledge, error)
	TagStats(ctx context.Context, ownerID, status string) (map[string]int, error)

	// AllOwnerIDs returns distinct owner IDs (for batch maintenance tasks)
	AllOwnerIDs(ctx context.Context) ([]string, error)

	// Transaction
	RunInTx(ctx context.Context, fn func(ctx context.Context) error) error

	// Search log
	SaveSearchLog(ctx context.Context, log *domain.SearchLog) error
	ListSearchLogs(ctx context.Context, ownerID string, offset, limit int) ([]*domain.SearchLog, error)
	ListSearchLogsFiltered(ctx context.Context, ownerID, source string, offset, limit int) ([]*domain.SearchLog, error)
	CountSearchLogs(ctx context.Context, ownerID, source string) (int, error)
	SearchLogStats(ctx context.Context, ownerID string) (*domain.SearchLogStats, error)
	MarkSearchBadFeedback(ctx context.Context, ownerID, logID, badItemID string) error
	KnowledgeHitRanking(ctx context.Context, ownerID string, limit int) ([]domain.KnowledgeHitRank, error)

	Close() error
}

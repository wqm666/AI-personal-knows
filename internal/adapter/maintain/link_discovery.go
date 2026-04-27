package maintain

import (
	"context"
	"log/slog"

	"github.com/personal-know/internal/adapter/store/pgstore"
	"github.com/personal-know/internal/domain"
	"github.com/personal-know/internal/port"
)

const (
	defaultLinkThreshold        = 0.75
	defaultLinkScanLimit        = 500
	defaultLinkSimilarLimit     = 10
	defaultLinkExcludeThreshold = 0.95
)

type LinkDiscovery struct {
	pg               *pgstore.PgStore
	embedder         port.Embedder
	threshold        float64
	scanLimit        int
	similarLimit     int
	excludeThreshold float64
}

func NewLinkDiscovery(pg *pgstore.PgStore, embedder port.Embedder, threshold float64, scanLimit int) *LinkDiscovery {
	if threshold <= 0 {
		threshold = defaultLinkThreshold
	}
	if scanLimit <= 0 {
		scanLimit = defaultLinkScanLimit
	}
	return &LinkDiscovery{
		pg: pg, embedder: embedder,
		threshold: threshold, scanLimit: scanLimit,
		similarLimit: defaultLinkSimilarLimit, excludeThreshold: defaultLinkExcludeThreshold,
	}
}

func (t *LinkDiscovery) Name() string        { return "link_discovery" }
func (t *LinkDiscovery) Description() string { return "Discover and link related knowledge items" }

func (t *LinkDiscovery) Run(ctx context.Context, ownerID string) (*domain.MaintainResult, error) {
	items, err := t.pg.ListActiveWithEmbedding(ctx, ownerID, t.scanLimit)
	if err != nil {
		return nil, err
	}

	affected := 0
	for _, item := range items {
		if item.ReviewStatus != domain.ReviewApproved {
			continue
		}

		similar, err := t.pg.FindSimilar(ctx, ownerID, item.Embedding, t.threshold, item.ID, t.similarLimit)
		if err != nil {
			slog.Warn("link discovery: find similar failed", "id", item.ID, "error", err)
			continue
		}

		newRelated := make(map[string]bool)
		for _, id := range item.RelatedIDs {
			newRelated[id] = true
		}

		changed := false
		for _, hit := range similar {
			if hit.Score >= t.excludeThreshold {
				continue
			}
			if !newRelated[hit.Item.ID] {
				newRelated[hit.Item.ID] = true
				changed = true
			}
		}

		if changed {
			ids := make([]string, 0, len(newRelated))
			for id := range newRelated {
				ids = append(ids, id)
			}
			if err := t.pg.UpdateRelatedIDs(ctx, ownerID, item.ID, ids); err != nil {
				slog.Warn("link discovery: update related failed", "id", item.ID, "error", err)
				continue
			}
			affected++
		}
	}

	return &domain.MaintainResult{
		TaskName: t.Name(),
		Success:  true,
		Message:  "link discovery completed",
		Affected: affected,
	}, nil
}

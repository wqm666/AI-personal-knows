package maintain

import (
	"context"
	"log/slog"

	"github.com/personal-know/internal/adapter/store/pgstore"
	"github.com/personal-know/internal/domain"
	"github.com/personal-know/internal/port"
)

const (
	linkThreshold        = 0.75
	linkScanLimit        = 500
	linkSimilarLimit     = 10
	linkExcludeThreshold = 0.95
)

type LinkDiscovery struct {
	pg       *pgstore.PgStore
	embedder port.Embedder
}

func NewLinkDiscovery(pg *pgstore.PgStore, embedder port.Embedder) *LinkDiscovery {
	return &LinkDiscovery{pg: pg, embedder: embedder}
}

func (t *LinkDiscovery) Name() string        { return "link_discovery" }
func (t *LinkDiscovery) Description() string { return "Discover and link related knowledge items" }

func (t *LinkDiscovery) Run(ctx context.Context, ownerID string) (*domain.MaintainResult, error) {
	items, err := t.pg.ListByStatus(ctx, ownerID, domain.StatusActive, 0, linkScanLimit)
	if err != nil {
		return nil, err
	}

	affected := 0
	for _, item := range items {
		if len(item.Embedding) == 0 {
			continue
		}

		similar, err := t.pg.FindSimilar(ctx, ownerID, item.Embedding, linkThreshold, item.ID, linkSimilarLimit)
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
			if hit.Score >= linkExcludeThreshold {
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

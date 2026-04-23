package maintain

import (
	"context"
	"log/slog"
	"time"

	"github.com/personal-know/internal/domain"
	"github.com/personal-know/internal/port"
)

const (
	decayDays      = 90
	decayScanLimit = 1000
)

type Decay struct {
	store port.Store
}

func NewDecay(store port.Store) *Decay {
	return &Decay{store: store}
}

func (t *Decay) Name() string        { return "decay" }
func (t *Decay) Description() string { return "Reduce weight of long-unused knowledge" }

func (t *Decay) Run(ctx context.Context, ownerID string) (*domain.MaintainResult, error) {
	items, err := t.store.ListByStatus(ctx, ownerID, domain.StatusActive, 0, decayScanLimit)
	if err != nil {
		return nil, err
	}

	threshold := time.Now().Add(-decayDays * 24 * time.Hour)
	affected := 0

	for _, item := range items {
		if item.HitCount > 0 && item.LastHitAt != nil && item.LastHitAt.After(threshold) {
			continue
		}
		if item.UsefulCount > 0 {
			continue
		}
		if item.CreatedAt.After(threshold) {
			continue
		}
		if err := t.store.UpdateStatus(ctx, ownerID, item.ID, domain.StatusDecayed); err != nil {
			slog.Warn("decay: update status failed", "id", item.ID, "error", err)
			continue
		}
		affected++
	}

	return &domain.MaintainResult{
		TaskName: t.Name(),
		Success:  true,
		Message:  "decay scan completed",
		Affected: affected,
	}, nil
}

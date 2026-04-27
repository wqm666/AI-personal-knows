package maintain

import (
	"context"
	"log/slog"
	"time"

	"github.com/personal-know/internal/domain"
	"github.com/personal-know/internal/port"
)

const (
	defaultDecayDays      = 90
	defaultDecayScanLimit = 1000
)

type Decay struct {
	store     port.Store
	days      int
	scanLimit int
}

func NewDecay(store port.Store, days, scanLimit int) *Decay {
	if days <= 0 {
		days = defaultDecayDays
	}
	if scanLimit <= 0 {
		scanLimit = defaultDecayScanLimit
	}
	return &Decay{store: store, days: days, scanLimit: scanLimit}
}

func (t *Decay) Name() string        { return "decay" }
func (t *Decay) Description() string { return "Reduce weight of long-unused knowledge" }

func (t *Decay) Run(ctx context.Context, ownerID string) (*domain.MaintainResult, error) {
	items, err := t.store.ListByStatus(ctx, ownerID, domain.StatusActive, 0, t.scanLimit)
	if err != nil {
		return nil, err
	}

	threshold := time.Now().Add(-time.Duration(t.days) * 24 * time.Hour)
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

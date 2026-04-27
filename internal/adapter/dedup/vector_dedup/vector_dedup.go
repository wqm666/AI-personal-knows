package vector_dedup

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/personal-know/internal/adapter/store/pgstore"
	"github.com/personal-know/internal/domain"
	"github.com/personal-know/internal/port"
)

const (
	defaultReinforceThreshold = 0.95
	defaultRelateThreshold    = 0.75
	contradictionTruncateLen  = 1500
)

type Dedup struct {
	pg                 *pgstore.PgStore
	llm                port.LLMClient
	reinforceThreshold float64
	relateThreshold    float64
}

func New(pg *pgstore.PgStore, reinforceThreshold, relateThreshold float64) *Dedup {
	if reinforceThreshold <= 0 {
		reinforceThreshold = defaultReinforceThreshold
	}
	if relateThreshold <= 0 {
		relateThreshold = defaultRelateThreshold
	}
	return &Dedup{pg: pg, reinforceThreshold: reinforceThreshold, relateThreshold: relateThreshold}
}

func (d *Dedup) SetLLM(llm port.LLMClient) {
	d.llm = llm
}

func (d *Dedup) Check(ctx context.Context, ownerID, content string, embedding []float64) (*domain.DedupResult, error) {
	if len(embedding) == 0 {
		return &domain.DedupResult{Action: domain.ActionNew}, nil
	}

	hits, err := d.pg.FindSimilar(ctx, ownerID, embedding, d.relateThreshold, "", 1)
	if err != nil {
		return nil, fmt.Errorf("dedup similarity check: %w", err)
	}

	if len(hits) == 0 {
		return &domain.DedupResult{Action: domain.ActionNew}, nil
	}

	top := hits[0]
	if top.Score >= d.reinforceThreshold {
		if d.llm != nil {
			contradicts, err := d.checkContradiction(ctx, content, top.Item.Content)
			if err != nil {
				slog.Warn("contradiction check failed, falling back to reinforce", "error", err)
			} else if contradicts {
				return &domain.DedupResult{
					Action:  domain.ActionSupersede,
					ExistID: top.Item.ID,
					Score:   top.Score,
				}, nil
			}
		}

		return &domain.DedupResult{
			Action:  domain.ActionReinforce,
			ExistID: top.Item.ID,
			Score:   top.Score,
		}, nil
	}

	if d.llm != nil {
		contradicts, err := d.checkContradiction(ctx, content, top.Item.Content)
		if err != nil {
			slog.Warn("contradiction check failed, falling back to relate", "error", err)
		} else if contradicts {
			return &domain.DedupResult{
				Action:  domain.ActionSupersede,
				ExistID: top.Item.ID,
				Score:   top.Score,
			}, nil
		}
	}

	return &domain.DedupResult{
		Action:  domain.ActionRelate,
		ExistID: top.Item.ID,
		Score:   top.Score,
	}, nil
}

func (d *Dedup) checkContradiction(ctx context.Context, newContent, existingContent string) (bool, error) {
	prompt := fmt.Sprintf(`Compare these two knowledge items and determine if they CONTRADICT each other.
Contradiction means: same topic but opposite conclusions, outdated vs updated information, or conflicting recommendations.
NOT contradiction: different aspects of the same topic, complementary information, or different topics.

--- Existing knowledge ---
%s

--- New knowledge ---
%s

Return JSON only: {"contradicts": true/false, "reason": "brief explanation"}`,
		truncate(existingContent, contradictionTruncateLen), truncate(newContent, contradictionTruncateLen))

	resp, err := d.llm.ChatJSON(ctx, []port.LLMMessage{
		{Role: "system", Content: "You detect contradictions between knowledge items. Respond only with JSON."},
		{Role: "user", Content: prompt},
	}, nil)
	if err != nil {
		return false, err
	}

	resp = strings.TrimSpace(resp)
	var result struct {
		Contradicts bool   `json:"contradicts"`
		Reason      string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		return false, fmt.Errorf("parse contradiction response: %w", err)
	}

	if result.Contradicts {
		slog.Info("contradiction detected", "reason", result.Reason)
	}
	return result.Contradicts, nil
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen])
}

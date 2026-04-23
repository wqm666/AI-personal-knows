package maintain

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/personal-know/internal/domain"
	"github.com/personal-know/internal/port"
)

type TagCluster struct {
	store port.Store
	llm   port.LLMClient
}

const (
	tagClusterScanLimit = 1000
	minTagsForCluster   = 5
)

func NewTagCluster(store port.Store, llm port.LLMClient) *TagCluster {
	return &TagCluster{store: store, llm: llm}
}

func (t *TagCluster) Name() string        { return "tag_cluster" }
func (t *TagCluster) Description() string { return "Normalize synonymous tags via LLM" }

func (t *TagCluster) Run(ctx context.Context, ownerID string) (*domain.MaintainResult, error) {
	items, err := t.store.ListByStatus(ctx, ownerID, domain.StatusActive, 0, tagClusterScanLimit)
	if err != nil {
		return nil, err
	}

	tagCounts := make(map[string]int)
	for _, item := range items {
		for _, tag := range item.Tags {
			tagCounts[tag]++
		}
	}

	if len(tagCounts) < minTagsForCluster {
		return &domain.MaintainResult{
			TaskName: t.Name(),
			Success:  true,
			Message:  "too few tags to cluster",
			Affected: 0,
		}, nil
	}

	tags := make([]string, 0, len(tagCounts))
	for tag := range tagCounts {
		tags = append(tags, tag)
	}

	mapping, err := t.askLLMForMapping(ctx, tags)
	if err != nil {
		return nil, err
	}

	affected := 0
	for _, item := range items {
		changed := false
		newTags := make([]string, len(item.Tags))
		for i, tag := range item.Tags {
			if canonical, ok := mapping[tag]; ok && canonical != tag {
				newTags[i] = canonical
				changed = true
			} else {
				newTags[i] = tag
			}
		}

		if changed {
			newTags = dedupStrings(newTags)
			if err := t.store.UpdateTags(ctx, ownerID, item.ID, newTags); err != nil {
				slog.Warn("tag cluster: update failed", "id", item.ID, "error", err)
				continue
			}
			affected++
		}
	}

	return &domain.MaintainResult{
		TaskName: t.Name(),
		Success:  true,
		Message:  fmt.Sprintf("normalized tags for %d items", affected),
		Affected: affected,
	}, nil
}

func (t *TagCluster) askLLMForMapping(ctx context.Context, tags []string) (map[string]string, error) {
	prompt := fmt.Sprintf(`Below is a list of tags from a personal knowledge base.
Find synonymous or near-duplicate tags and map them to a canonical form.

Tags: %v

Return JSON object where keys are original tags and values are their canonical form.
Only include tags that need mapping (synonyms). Keep the most common/clear form as canonical.
Example: {"golang": "Go", "go语言": "Go", "Go": "Go"}

Return ONLY valid JSON, no markdown.`, tags)

	resp, err := t.llm.ChatJSON(ctx, []port.LLMMessage{
		{Role: "user", Content: prompt},
	}, nil)
	if err != nil {
		return nil, err
	}

	var mapping map[string]string
	if err := json.Unmarshal([]byte(resp), &mapping); err != nil {
		return nil, fmt.Errorf("parse tag mapping: %w", err)
	}
	return mapping, nil
}

func dedupStrings(ss []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

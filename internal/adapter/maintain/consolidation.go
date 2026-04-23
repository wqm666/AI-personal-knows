package maintain

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/personal-know/internal/adapter/store/pgstore"
	"github.com/personal-know/internal/domain"
	"github.com/personal-know/internal/port"
)

const (
	minClusterSize         = 3
	consolidationScanLimit = 500
	synthesisCheckLimit    = 100
	synthesisOverlapRatio  = 0.8
)

type Consolidation struct {
	pg    *pgstore.PgStore
	llm   port.LLMClient
	idGen port.IDGenerator
}

func NewConsolidation(pg *pgstore.PgStore, llm port.LLMClient, idGen port.IDGenerator) *Consolidation {
	return &Consolidation{pg: pg, llm: llm, idGen: idGen}
}

func (t *Consolidation) Name() string        { return "consolidation" }
func (t *Consolidation) Description() string { return "Create index nodes for related knowledge clusters" }

func (t *Consolidation) Run(ctx context.Context, ownerID string) (*domain.MaintainResult, error) {
	items, err := t.pg.ListByStatus(ctx, ownerID, domain.StatusActive, 0, consolidationScanLimit)
	if err != nil {
		return nil, err
	}

	clusters := t.findClusters(items)
	affected := 0

	for _, cluster := range clusters {
		if len(cluster) < minClusterSize {
			continue
		}

		if t.alreadyHasSynthesis(ctx, ownerID, cluster) {
			continue
		}

		indexNode, err := t.buildIndexNode(ctx, ownerID, cluster)
		if err != nil {
			slog.Warn("consolidation failed", "error", err)
			continue
		}

		if err := t.pg.Save(ctx, indexNode); err != nil {
			slog.Warn("save index node failed", "error", err)
			continue
		}

		// Link the index node back to all fragments
		indexID := indexNode.ID
		for _, item := range cluster {
			relIDs := appendUnique(item.RelatedIDs, indexID)
			_ = t.pg.UpdateRelatedIDs(ctx, ownerID, item.ID, relIDs)
		}

		affected++
	}

	return &domain.MaintainResult{
		TaskName: t.Name(),
		Success:  true,
		Message:  fmt.Sprintf("created %d index nodes", affected),
		Affected: affected,
	}, nil
}

func (t *Consolidation) findClusters(items []*domain.Knowledge) [][]*domain.Knowledge {
	visited := make(map[string]bool)
	var clusters [][]*domain.Knowledge

	itemMap := make(map[string]*domain.Knowledge)
	for _, item := range items {
		itemMap[item.ID] = item
	}

	for _, item := range items {
		if visited[item.ID] || len(item.RelatedIDs) < minClusterSize-1 {
			continue
		}

		cluster := []*domain.Knowledge{item}
		visited[item.ID] = true

		for _, relID := range item.RelatedIDs {
			if rel, ok := itemMap[relID]; ok && !visited[relID] && rel.Status == domain.StatusActive {
				cluster = append(cluster, rel)
				visited[relID] = true
			}
		}

		if len(cluster) >= minClusterSize {
			clusters = append(clusters, cluster)
		}
	}
	return clusters
}

// alreadyHasSynthesis checks if a synthesis node already covers this cluster.
func (t *Consolidation) alreadyHasSynthesis(ctx context.Context, ownerID string, cluster []*domain.Knowledge) bool {
	syntheses, err := t.pg.ListByStatus(ctx, ownerID, domain.StatusSynthesis, 0, synthesisCheckLimit)
	if err != nil {
		return false
	}

	clusterIDs := make(map[string]bool)
	for _, item := range cluster {
		clusterIDs[item.ID] = true
	}

	for _, s := range syntheses {
		overlap := 0
		for _, id := range s.ConsolidatedFrom {
			if clusterIDs[id] {
				overlap++
			}
		}
		if float64(overlap) >= float64(len(cluster))*synthesisOverlapRatio {
			return true
		}
	}
	return false
}

// buildIndexNode creates a synthesis node that is purely an index —
// its content is the original fragments concatenated, NOT LLM-rewritten.
// LLM only generates a title and summary as navigational labels.
func (t *Consolidation) buildIndexNode(ctx context.Context, ownerID string, cluster []*domain.Knowledge) (*domain.Knowledge, error) {
	ids := make([]string, 0, len(cluster))
	allTags := make(map[string]bool)
	var contentParts []string

	for _, item := range cluster {
		ids = append(ids, item.ID)
		contentParts = append(contentParts, fmt.Sprintf("## %s\n%s", item.Title, item.Content))
		for _, tag := range item.Tags {
			allTags[tag] = true
		}
	}

	// Content = original fragments concatenated verbatim
	content := strings.Join(contentParts, "\n\n---\n\n")

	// LLM only generates a title and summary as index labels
	prompt := fmt.Sprintf(`Below are %d related knowledge fragments.
Generate ONLY a title and a 1-2 sentence summary that describes what this cluster is about.
Do NOT rewrite or synthesize the content. Just describe what these fragments collectively cover.

%s

Return JSON: {"title": "...", "summary": "..."}
Return ONLY valid JSON, no markdown.`, len(cluster), content)

	tags := make([]string, 0, len(allTags))
	for tag := range allTags {
		tags = append(tags, tag)
	}

	title := fmt.Sprintf("Cluster: %d fragments", len(cluster))
	summary := ""

	resp, err := t.llm.ChatJSON(ctx, []port.LLMMessage{
		{Role: "user", Content: prompt},
	}, nil)
	if err != nil {
		slog.Warn("llm index label failed, using fallback title", "error", err)
	} else {
		var result struct {
			Title   string `json:"title"`
			Summary string `json:"summary"`
		}
		if err := json.Unmarshal([]byte(resp), &result); err == nil {
			title = result.Title
			summary = result.Summary
		}
	}

	now := time.Now()
	return &domain.Knowledge{
		ID:               t.idGen.Generate(),
		OwnerID:          ownerID,
		Title:            title,
		Content:          content,
		Summary:          summary,
		Source:           "consolidation",
		Tags:             tags,
		Status:           domain.StatusSynthesis,
		ConsolidatedFrom: ids,
		RelatedIDs:       ids,
		CreatedAt:        now,
		UpdatedAt:        now,
	}, nil
}

func appendUnique(slice []string, val string) []string {
	for _, s := range slice {
		if s == val {
			return slice
		}
	}
	result := make([]string, len(slice), len(slice)+1)
	copy(result, slice)
	return append(result, val)
}

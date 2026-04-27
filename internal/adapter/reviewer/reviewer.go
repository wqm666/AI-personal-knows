package reviewer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/personal-know/internal/domain"
	"github.com/personal-know/internal/port"
)

type Reviewer struct {
	llm   port.LLMClient
	store port.Store
}

func New(llm port.LLMClient, store port.Store) *Reviewer {
	return &Reviewer{llm: llm, store: store}
}

type llmSuggestion struct {
	Recommendation string   `json:"recommendation"`
	Confidence     int      `json:"confidence"`
	Reason         string   `json:"reason"`
	Issues         []string `json:"issues"`
}

func (r *Reviewer) Review(ctx context.Context, k *domain.Knowledge) (*domain.ReviewResult, error) {
	if r.llm == nil {
		return &domain.ReviewResult{
			Status:     domain.ReviewPending,
			Confidence: 0,
			Reason:     "no LLM available — manual review required",
		}, nil
	}

	existingContext := r.gatherContext(ctx, k)
	prompt := buildSuggestionPrompt(k, existingContext)

	resp, err := r.llm.Chat(ctx, []port.LLMMessage{
		{Role: "system", Content: `You are a knowledge quality advisor. Analyze the knowledge item and provide a recommendation to help the human reviewer decide.

You do NOT make the final decision — the human does. Your job is to flag potential issues.

Respond only with JSON: {"recommendation": "approve|reject|needs_revision", "confidence": 0-100, "reason": "brief explanation", "issues": ["issue1", "issue2"]}`},
		{Role: "user", Content: prompt},
	})
	if err != nil {
		return nil, fmt.Errorf("LLM suggestion failed: %w", err)
	}

	resp = cleanJSON(resp)
	var suggestion llmSuggestion
	if err := json.Unmarshal([]byte(resp), &suggestion); err != nil {
		return nil, fmt.Errorf("parse suggestion: %w", err)
	}

	switch suggestion.Recommendation {
	case "approve", "reject", "needs_revision":
	default:
		slog.Warn("LLM returned invalid recommendation, defaulting to needs_revision", "value", suggestion.Recommendation)
		suggestion.Recommendation = "needs_revision"
	}

	if suggestion.Confidence < 0 {
		suggestion.Confidence = 0
	}
	if suggestion.Confidence > 100 {
		suggestion.Confidence = 100
	}

	reason := fmt.Sprintf("[LLM suggestion: %s] %s", suggestion.Recommendation, suggestion.Reason)
	if len(suggestion.Issues) > 0 {
		reason += " | Issues: " + strings.Join(suggestion.Issues, "; ")
	}

	return &domain.ReviewResult{
		Status:     domain.ReviewPending,
		Confidence: suggestion.Confidence,
		Reason:     reason,
	}, nil
}

const (
	reviewContextFetchLimit = 200
	reviewContextMaxItems   = 5
	reviewContextTruncate   = 200
)

func (r *Reviewer) gatherContext(ctx context.Context, k *domain.Knowledge) string {
	if r.store == nil {
		return ""
	}

	ownerID := k.OwnerID
	if ownerID == "" {
		ownerID = "default"
	}

	items, err := r.store.ListByReviewStatus(ctx, ownerID, domain.ReviewApproved, 0, reviewContextFetchLimit)
	if err != nil {
		slog.Warn("gather context failed", "error", err)
		return ""
	}

	var related []string
	for _, item := range items {
		if item.ID == k.ID || item.Status != domain.StatusActive {
			continue
		}
		if hasOverlap(k.Tags, item.Tags) || containsKeywords(k.Content, item.Title) {
			related = append(related, fmt.Sprintf("- [%s] %s: %s", item.KnowledgeType, item.Title, truncate(item.Content, reviewContextTruncate)))
			if len(related) >= reviewContextMaxItems {
				break
			}
		}
	}

	if len(related) == 0 {
		return ""
	}
	return "Existing approved knowledge on similar topics:\n" + strings.Join(related, "\n")
}

func buildSuggestionPrompt(k *domain.Knowledge, existingContext string) string {
	var sb strings.Builder

	sb.WriteString("Analyze this knowledge item and provide a review suggestion for the human reviewer.\n\n")
	sb.WriteString(fmt.Sprintf("Title: %s\n", k.Title))
	sb.WriteString(fmt.Sprintf("Type: %s\n", k.KnowledgeType))
	sb.WriteString(fmt.Sprintf("Category: %s\n", k.KnowledgeCategory))
	sb.WriteString(fmt.Sprintf("Source: %s\n", k.Source))
	if len(k.Tags) > 0 {
		sb.WriteString(fmt.Sprintf("Tags: %s\n", strings.Join(k.Tags, ", ")))
	}
	sb.WriteString(fmt.Sprintf("\nContent:\n%s\n", k.Content))

	if existingContext != "" {
		sb.WriteString(fmt.Sprintf("\n%s\n", existingContext))
	}

	sb.WriteString(`
Check for:
1. CORRECTNESS: Any factual errors or misleading claims?
2. CONSISTENCY: Does it contradict existing approved knowledge above?
3. COMPLETENESS: Enough context to be useful standalone?
4. VALUE: Worth storing long-term, or too trivial/ephemeral?

List any specific issues found in the "issues" array.`)

	return sb.String()
}

func hasOverlap(a, b []string) bool {
	set := make(map[string]bool, len(a))
	for _, v := range a {
		set[strings.ToLower(v)] = true
	}
	for _, v := range b {
		if set[strings.ToLower(v)] {
			return true
		}
	}
	return false
}

func containsKeywords(content, title string) bool {
	contentLower := strings.ToLower(content)
	words := strings.Fields(strings.ToLower(title))
	matches := 0
	for _, w := range words {
		if len(w) > 2 && strings.Contains(contentLower, w) {
			matches++
		}
	}
	return len(words) > 0 && matches >= len(words)/2
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

func cleanJSON(resp string) string {
	resp = strings.TrimSpace(resp)
	resp = strings.TrimPrefix(resp, "```json")
	resp = strings.TrimPrefix(resp, "```")
	resp = strings.TrimSuffix(resp, "```")
	return strings.TrimSpace(resp)
}

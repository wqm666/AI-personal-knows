package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/personal-know/internal/domain"
	"github.com/personal-know/internal/port"
)

type Reviewer interface {
	Review(ctx context.Context, k *domain.Knowledge) (*domain.ReviewResult, error)
}

type Service struct {
	store        port.Store
	orchestrator port.Orchestrator
	embedder     port.Embedder
	dedup        port.Deduplicator
	maintainer   port.Maintainer
	idGen        port.IDGenerator
	llm          port.LLMClient
	reviewer     Reviewer
}

func New(
	store port.Store,
	orchestrator port.Orchestrator,
	embedder port.Embedder,
	dedup port.Deduplicator,
	maintainer port.Maintainer,
	idGen port.IDGenerator,
) *Service {
	return &Service{
		store:        store,
		orchestrator: orchestrator,
		embedder:     embedder,
		dedup:        dedup,
		maintainer:   maintainer,
		idGen:        idGen,
	}
}

func (s *Service) SetLLM(llm port.LLMClient) {
	s.llm = llm
}

func (s *Service) SetReviewer(r Reviewer) {
	s.reviewer = r
}

type SaveResult struct {
	Saved        bool     `json:"saved"`
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Tags         []string `json:"tags"`
	Action       string   `json:"action"`
	Message      string   `json:"message,omitempty"`
	ReviewStatus string   `json:"review_status,omitempty"`
	Confidence   int      `json:"confidence,omitempty"`
	ReviewReason string   `json:"review_reason,omitempty"`
}

func (s *Service) Save(ctx context.Context, title, content, source, sourceRef string, tags []string) (*SaveResult, error) {
	identity := port.IdentityFromContext(ctx)
	ownerID := identity.OwnerID

	if content == "" {
		return nil, fmt.Errorf("content is required")
	}
	if len(content) > maxContentSize {
		return nil, fmt.Errorf("content too large: %d bytes (max %d)", len(content), maxContentSize)
	}
	if !utf8.ValidString(content) {
		content = sanitizeUTF8(content)
	}
	if !utf8.ValidString(title) {
		title = sanitizeUTF8(title)
	}
	if title == "" {
		title = generateTitle(content)
	}
	if source == "" {
		source = domain.SourceManual
	}

	var embedding []float64
	if s.embedder != nil {
		var err error
		embedding, err = s.embedder.Embed(ctx, title+" "+content)
		if err != nil {
			slog.Warn("embedding failed, saving without vector", "error", err)
		}
	}

	if s.dedup != nil && len(embedding) > 0 {
		result, err := s.dedup.Check(ctx, ownerID, content, embedding)
		if err != nil {
			slog.Warn("dedup check failed, skipping deduplication", "error", err)
		} else if result != nil {
			switch result.Action {
			case domain.ActionReinforce:
				if err := s.store.IncrementHitCount(ctx, ownerID, result.ExistID); err != nil {
					slog.Warn("reinforce hit count failed", "error", err)
				}
				return &SaveResult{
					Saved:   false,
					ID:      result.ExistID,
					Action:  domain.ActionReinforce,
					Message: fmt.Sprintf("similar knowledge exists (score=%.2f), reinforced", result.Score),
				}, nil

			case domain.ActionSupersede:
				id := s.idGen.Generate()
				now := time.Now()
				k := &domain.Knowledge{
					ID:           id,
					OwnerID:      ownerID,
					Title:        title,
					Content:      content,
					Source:       source,
					SourceRef:    sourceRef,
					Tags:         tags,
					Embedding:    embedding,
					RelatedIDs:   []string{result.ExistID},
					Status:       domain.StatusActive,
					ReviewStatus: domain.ReviewPending,
					CreatedAt:    now,
					UpdatedAt:    now,
				}
				txErr := s.store.RunInTx(ctx, func(txCtx context.Context) error {
					if err := s.store.Save(txCtx, k); err != nil {
						return err
					}
					return s.store.UpdateSupersededBy(txCtx, ownerID, result.ExistID, id)
				})
				if txErr != nil {
					return nil, txErr
				}
				return &SaveResult{
					Saved:        true,
					ID:           id,
					Title:        title,
					Tags:         tags,
					Action:       domain.ActionSupersede,
					Message:      fmt.Sprintf("contradicts existing item %s (score=%.2f), saved as newer version — pending review", result.ExistID, result.Score),
					ReviewStatus: domain.ReviewPending,
				}, nil

			case domain.ActionRelate:
				id := s.idGen.Generate()
				now := time.Now()
				k := &domain.Knowledge{
					ID:           id,
					OwnerID:      ownerID,
					Title:        title,
					Content:      content,
					Source:       source,
					SourceRef:    sourceRef,
					Tags:         tags,
					Embedding:    embedding,
					RelatedIDs:   []string{result.ExistID},
					Status:       domain.StatusActive,
					ReviewStatus: domain.ReviewPending,
					CreatedAt:    now,
					UpdatedAt:    now,
				}
				txErr := s.store.RunInTx(ctx, func(txCtx context.Context) error {
					if err := s.store.Save(txCtx, k); err != nil {
						return err
					}
					existing, err := s.store.Get(txCtx, ownerID, result.ExistID)
					if err != nil {
						return err
					}
					if existing == nil {
						slog.Warn("related item disappeared during transaction", "exist_id", result.ExistID)
						return nil
					}
					relIDs := make([]string, len(existing.RelatedIDs), len(existing.RelatedIDs)+1)
					copy(relIDs, existing.RelatedIDs)
					relIDs = append(relIDs, id)
					return s.store.UpdateRelatedIDs(txCtx, ownerID, result.ExistID, relIDs)
				})
				if txErr != nil {
					return nil, txErr
				}

				return &SaveResult{
					Saved:        true,
					ID:           id,
					Title:        title,
					Tags:         tags,
					Action:       domain.ActionRelate,
					Message:      fmt.Sprintf("saved and linked to related item %s (score=%.2f) — pending review", result.ExistID, result.Score),
					ReviewStatus: domain.ReviewPending,
				}, nil
			}
		}
	}

	id := s.idGen.Generate()
	now := time.Now()
	k := &domain.Knowledge{
		ID:           id,
		OwnerID:      ownerID,
		Title:        title,
		Content:      content,
		Source:       source,
		SourceRef:    sourceRef,
		Tags:         tags,
		Embedding:    embedding,
		Status:       domain.StatusActive,
		ReviewStatus: domain.ReviewPending,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.store.Save(ctx, k); err != nil {
		return nil, err
	}

	return &SaveResult{
		Saved:        true,
		ID:           id,
		Title:        title,
		Tags:         tags,
		Action:       domain.ActionNew,
		ReviewStatus: domain.ReviewPending,
	}, nil
}

type SearchResult struct {
	Items       []domain.SearchHit `json:"items"`
	Total       int                `json:"total"`
	EntryCount  int                `json:"entry_count"`
	SearchLogID string             `json:"search_log_id,omitempty"`
}

func (s *Service) Search(ctx context.Context, query string, limit int, source string) (*SearchResult, error) {
	identity := port.IdentityFromContext(ctx)

	if limit <= 0 {
		limit = defaultSearchLimit
	}

	// Step 1: Find entry points via multi-strategy retrieval
	hits, err := s.orchestrator.Search(ctx, domain.SearchQuery{
		OwnerID: identity.OwnerID,
		Text:    query,
		Limit:   limit,
	})
	if err != nil {
		return nil, err
	}

	entryCount := len(hits)

	// Step 2: Expand — follow related_ids to pull the full knowledge cluster
	seen := make(map[string]bool)
	var expanded []domain.SearchHit

	for _, h := range hits {
		seen[h.Item.ID] = true
		expanded = append(expanded, h)
	}

	// BFS expansion: 2 layers deep along related_ids
	var frontier []string
	for _, h := range hits {
		for _, rid := range h.Item.RelatedIDs {
			if !seen[rid] {
				frontier = append(frontier, rid)
				seen[rid] = true
			}
		}
	}

	for depth := 0; depth < bfsExpansionDepth && len(frontier) > 0 && len(expanded) < maxExpandedItems; depth++ {
		related, err := s.store.ListByIDs(ctx, identity.OwnerID, frontier)
		if err != nil {
			break
		}

		var nextFrontier []string
		for _, item := range related {
			if item.Status == domain.StatusActive || item.Status == domain.StatusSynthesis || item.Status == domain.StatusSuperseded {
				expanded = append(expanded, domain.SearchHit{
					Item:      *item,
					Score:     expansionBaseScore - float64(depth)*expansionScoreDecay,
					Retriever: domain.RetrieverExpansion,
				})
			}
			// Keep expanding
			for _, rid := range item.RelatedIDs {
				if !seen[rid] {
					nextFrontier = append(nextFrontier, rid)
					seen[rid] = true
				}
			}
		}
		frontier = nextFrontier
	}

	// Step 3: For synthesis items, also pull their source fragments
	var synthSources []string
	for _, h := range expanded {
		if h.Item.Status == domain.StatusSynthesis && len(h.Item.ConsolidatedFrom) > 0 {
			for _, srcID := range h.Item.ConsolidatedFrom {
				if !seen[srcID] {
					seen[srcID] = true
					synthSources = append(synthSources, srcID)
				}
			}
		}
	}
	if len(synthSources) > 0 {
		sources, err := s.store.ListByIDs(ctx, identity.OwnerID, synthSources)
		if err == nil {
			for _, item := range sources {
				expanded = append(expanded, domain.SearchHit{
					Item:      *item,
					Score:     sourceFragmentScore,
					Retriever: domain.RetrieverSourceFragment,
				})
			}
		}
	}

	// Step 4: Annotate conflicts and sort with newest-first for contradictions
	expanded = annotateConflicts(expanded)

	for i := range expanded {
		expanded[i].Confidence = expanded[i].Item.Confidence
	}

	// Collect result IDs for search log
	resultIDs := make([]string, 0, len(expanded))
	for _, h := range expanded {
		resultIDs = append(resultIDs, h.Item.ID)
	}
	searchLogID := s.idGen.Generate()
	logSource := source
	if logSource == "" {
		logSource = domain.SearchSourceAPI
	}

	// Async: increment hit counts + save search log
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), asyncTimeout)
		defer cancel()
		for _, h := range expanded {
			if err := s.store.IncrementHitCount(bgCtx, identity.OwnerID, h.Item.ID); err != nil {
				slog.Warn("async hit count increment failed", "id", h.Item.ID, "error", err)
			}
		}
		if err := s.store.SaveSearchLog(bgCtx, &domain.SearchLog{
			ID:          searchLogID,
			OwnerID:     identity.OwnerID,
			Query:       query,
			Source:      logSource,
			ResultCount: entryCount,
			ResultIDs:   resultIDs,
			CreatedAt:   time.Now(),
		}); err != nil {
			slog.Warn("async search log save failed", "error", err)
		}
	}()

	return &SearchResult{
		Items:       expanded,
		Total:       len(expanded),
		EntryCount:  entryCount,
		SearchLogID: searchLogID,
	}, nil
}

func annotateConflicts(hits []domain.SearchHit) []domain.SearchHit {
	idIdx := make(map[string]int, len(hits))
	for i := range hits {
		idIdx[hits[i].Item.ID] = i
	}

	groupID := 0
	for i := range hits {
		item := &hits[i]

		if item.Item.SupersededBy != "" {
			group := fmt.Sprintf("conflict_%d", groupID)
			item.Freshness = domain.FreshnessOutdated
			item.ConflictGroup = group

			if j, ok := idIdx[item.Item.SupersededBy]; ok {
				hits[j].Freshness = domain.FreshnessLatest
				hits[j].ConflictGroup = group
			}
			groupID++
		}

		if item.Item.Status == domain.StatusSuperseded && item.Freshness == "" {
			item.Freshness = domain.FreshnessOutdated
		}
	}

	sort.SliceStable(hits, func(i, j int) bool {
		gi, gj := hits[i].ConflictGroup, hits[j].ConflictGroup
		if gi != "" && gi == gj {
			if hits[i].Freshness == domain.FreshnessLatest && hits[j].Freshness != domain.FreshnessLatest {
				return true
			}
			if hits[i].Freshness != domain.FreshnessLatest && hits[j].Freshness == domain.FreshnessLatest {
				return false
			}
			return hits[i].Item.UpdatedAt.After(hits[j].Item.UpdatedAt)
		}
		return hits[i].Score > hits[j].Score
	})

	return hits
}

type ImportResult struct {
	Imported bool   `json:"imported"`
	Count    int    `json:"count"`
	File     string `json:"file"`
}

func (s *Service) Import(ctx context.Context, fileContent, fileName, chunkMode string) (*ImportResult, error) {
	if chunkMode == "" {
		chunkMode = domain.ChunkModeAuto
	}

	var chunks []chunk
	if chunkMode == domain.ChunkModeSingle {
		chunks = []chunk{{title: fileName, content: fileContent}}
	} else {
		chunks = splitDocument(fileContent, fileName)
	}

	count := 0
	var lastErr error
	for _, c := range chunks {
		_, err := s.Save(ctx, c.title, c.content, domain.SourceDocument, fileName, c.tags)
		if err != nil {
			slog.Warn("import chunk failed", "title", c.title, "error", err)
			lastErr = err
			continue
		}
		count++
	}

	if count == 0 && len(chunks) > 0 {
		return nil, fmt.Errorf("all %d chunks failed to import, last error: %w", len(chunks), lastErr)
	}

	return &ImportResult{
		Imported: count > 0,
		Count:    count,
		File:     fileName,
	}, nil
}

type CaptureResult struct {
	Captured   bool         `json:"captured"`
	Count      int          `json:"count"`
	Items      []SaveResult `json:"items"`
	HasSignals bool         `json:"has_signals"`
	Signals    []string     `json:"signals,omitempty"`
	Extracted  int          `json:"extracted,omitempty"`
}

func (s *Service) Capture(ctx context.Context, sessionSummary string, itemsJSON string) (*CaptureResult, error) {
	var items []captureItem
	if itemsJSON != "" {
		if err := json.Unmarshal([]byte(itemsJSON), &items); err != nil {
			return nil, fmt.Errorf("parse items_json: %w", err)
		}
	}

	// Signal detection on session summary
	signals := detectSignals(sessionSummary)
	hasSignals := len(signals) > 0

	// If no explicit items but session has signals, use LLM to extract knowledge
	llmExtracted := 0
	if len(items) == 0 && hasSignals && s.llm != nil {
		extracted, err := s.extractKnowledgeFromSession(ctx, sessionSummary, signals)
		if err != nil {
			slog.Warn("LLM extraction failed, falling back to raw capture", "error", err)
		} else {
			items = extracted
			llmExtracted = len(extracted)
		}
	}

	// Fallback: if still no items, save raw summary
	if len(items) == 0 && sessionSummary != "" {
		items = []captureItem{{
			Title:         generateTitle(sessionSummary),
			Content:       sessionSummary,
			KnowledgeType: domain.TypeGeneral,
		}}
	}

	var results []SaveResult
	for _, item := range items {
		result, err := s.saveWithType(ctx, item.Title, item.Content, domain.SourceConversation, "", item.Tags, item.KnowledgeType)
		if err != nil {
			slog.Warn("capture item failed", "title", item.Title, "error", err)
			continue
		}
		results = append(results, *result)
	}

	return &CaptureResult{
		Captured:   len(results) > 0,
		Count:      len(results),
		Items:      results,
		HasSignals: hasSignals,
		Signals:    signals,
		Extracted:  llmExtracted,
	}, nil
}

var errorKeywords = []string{
	"error", "exception", "报错", "failed", "undefined", "cannot read",
	"panic", "fatal", "crash", "timeout", "nil pointer", "stack trace",
	"not found", "404", "500", "502", "503",
}

var negationKeywords = []string{
	"不对", "不是", "错了", "换一个", "重新", "还原", "不行", "不work", "不生效",
	"wrong", "incorrect", "revert", "undo", "rollback", "doesn't work", "not working",
}

func detectSignals(text string) []string {
	if text == "" {
		return nil
	}
	lower := strings.ToLower(text)
	var signals []string

	for _, kw := range errorKeywords {
		if strings.Contains(lower, kw) {
			signals = append(signals, "error:"+kw)
			break
		}
	}
	for _, kw := range negationKeywords {
		if strings.Contains(lower, kw) {
			signals = append(signals, "negation:"+kw)
			break
		}
	}

	// Multi-round debugging signal: long text suggests extensive session
	if len(text) > longSessionThreshold {
		signals = append(signals, "behavior:long_session")
	}

	return signals
}

func (s *Service) extractKnowledgeFromSession(ctx context.Context, summary string, signals []string) ([]captureItem, error) {
	prompt := fmt.Sprintf(`Analyze this development session and extract valuable knowledge items.
The session had these signals: %s

Session content:
%s

Extract knowledge as a JSON array. Each item has:
- "title": concise title
- "content": detailed description with context and solution
- "knowledge_type": one of "pitfall" (bugs/traps encountered), "decision" (tech choices made), "faq" (common questions answered), "procedure" (step-by-step operational workflow extracted from debugging/deployment/troubleshooting process)
- "tags": relevant keyword tags as string array

Rules:
- Only extract genuinely valuable knowledge (specific scenario + edge case + not in official docs)
- Skip basic API usage or standard configurations
- If nothing valuable, return empty array []
- Respond with ONLY the JSON array, no other text`, strings.Join(signals, ", "), summary)

	resp, err := s.llm.Chat(ctx, []port.LLMMessage{
		{Role: "system", Content: "You extract structured knowledge from development sessions. Respond only with a JSON array."},
		{Role: "user", Content: prompt},
	})
	if err != nil {
		return nil, err
	}

	resp = strings.TrimSpace(resp)
	// Strip markdown code fences if present
	resp = strings.TrimPrefix(resp, "```json")
	resp = strings.TrimPrefix(resp, "```")
	resp = strings.TrimSuffix(resp, "```")
	resp = strings.TrimSpace(resp)

	var items []captureItem
	if err := json.Unmarshal([]byte(resp), &items); err != nil {
		return nil, fmt.Errorf("parse LLM response: %w", err)
	}
	return items, nil
}

func (s *Service) saveWithType(ctx context.Context, title, content, source, sourceRef string, tags []string, knowledgeType string) (*SaveResult, error) {
	return s.saveWithTypeAndCategory(ctx, title, content, source, sourceRef, tags, knowledgeType, "")
}

func (s *Service) saveWithTypeAndCategory(ctx context.Context, title, content, source, sourceRef string, tags []string, knowledgeType, knowledgeCategory string) (*SaveResult, error) {
	result, err := s.Save(ctx, title, content, source, sourceRef, tags)
	if err != nil {
		return nil, err
	}
	if result.Saved {
		needsUpdate := false
		identity := port.IdentityFromContext(ctx)
		existing, getErr := s.store.Get(ctx, identity.OwnerID, result.ID)
		if getErr != nil || existing == nil {
			return result, nil
		}
		if knowledgeType != "" && knowledgeType != domain.TypeGeneral {
			existing.KnowledgeType = knowledgeType
			needsUpdate = true
		}
		if knowledgeCategory != "" {
			existing.KnowledgeCategory = knowledgeCategory
			needsUpdate = true
		}
		if needsUpdate {
			existing.UpdatedAt = time.Now()
			if err := s.store.Update(ctx, existing); err != nil {
				slog.Warn("failed to update type/category", "id", result.ID, "error", err)
			}
		}
	}
	return result, nil
}

// --- AutoCapture: three-stage funnel ---

type AutoCaptureResult struct {
	Captured bool         `json:"captured"`
	Count    int          `json:"count"`
	Items    []SaveResult `json:"items"`
	Stats    CaptureStats `json:"stats"`
}

type CaptureStats struct {
	Segments   int `json:"segments"`
	Valuable   int `json:"valuable"`
	Discarded  int `json:"discarded"`
	Subjective int `json:"subjective"`
	Objective  int `json:"objective"`
}

type segmentEval struct {
	Index    int    `json:"index"`
	Score    int    `json:"score"`
	Category string `json:"category"`
	Reason   string `json:"reason"`
}

func (s *Service) AutoCapture(ctx context.Context, conversation, projectCtx string) (*AutoCaptureResult, error) {
	if conversation == "" {
		return nil, fmt.Errorf("conversation is required")
	}

	if s.llm == nil {
		return s.autoCaptureWithoutLLM(ctx, conversation, projectCtx)
	}

	segments, err := s.segmentConversation(ctx, conversation)
	if err != nil {
		slog.Warn("segmentation failed, treating as single segment", "error", err)
		segments = []string{conversation}
	}

	evals, err := s.evaluateSegments(ctx, segments, projectCtx)
	if err != nil {
		slog.Warn("evaluation failed, falling back to no-LLM path", "error", err)
		return s.autoCaptureWithoutLLM(ctx, conversation, projectCtx)
	}

	stats := CaptureStats{Segments: len(segments)}
	var allItems []SaveResult

	for _, ev := range evals {
		if ev.Score < evalScoreThreshold || ev.Category == "none" {
			stats.Discarded++
			continue
		}
		stats.Valuable++

		if ev.Index < 0 || ev.Index >= len(segments) {
			continue
		}
		segment := segments[ev.Index]

		items, err := s.extractFromSegment(ctx, segment, ev.Category, projectCtx)
		if err != nil {
			slog.Warn("extraction failed for segment", "index", ev.Index, "error", err)
			continue
		}

		for _, item := range items {
			category := item.KnowledgeCategory
			if category == "" {
				category = ev.Category
				if category == "mixed" {
					category = categoryFromType(item.KnowledgeType)
				}
			}

			result, err := s.saveWithTypeAndCategory(ctx, item.Title, item.Content, domain.SourceConversation, "", item.Tags, item.KnowledgeType, category)
			if err != nil {
				slog.Warn("auto capture save failed", "title", item.Title, "error", err)
				continue
			}
			allItems = append(allItems, *result)

			switch category {
			case domain.CategorySubjective:
				stats.Subjective++
			case domain.CategoryObjective:
				stats.Objective++
			}
		}
	}

	return &AutoCaptureResult{
		Captured: len(allItems) > 0,
		Count:    len(allItems),
		Items:    allItems,
		Stats:    stats,
	}, nil
}

func categoryFromType(knowledgeType string) string {
	switch knowledgeType {
	case domain.TypePreference, domain.TypeThinking, domain.TypeLesson, domain.TypeDecision, domain.TypePitfall, domain.TypeProcedure:
		return domain.CategorySubjective
	case domain.TypeBusiness, domain.TypeArchitecture, domain.TypeFact, domain.TypeFAQ:
		return domain.CategoryObjective
	default:
		return ""
	}
}

const segmentThreshold = 2000

func (s *Service) segmentConversation(ctx context.Context, conversation string) ([]string, error) {
	if len(conversation) < segmentThreshold {
		return []string{conversation}, nil
	}

	prompt := `Split this AI conversation into segments by topic/task boundaries.
Each segment should be a complete topic with enough context to understand independently.
Return a JSON array of strings, each being one segment's text.
If the conversation has only one topic, return a single-element array.

Conversation:
` + conversation

	resp, err := s.llm.Chat(ctx, []port.LLMMessage{
		{Role: "system", Content: "You split conversations into topical segments. Respond only with a JSON array of strings."},
		{Role: "user", Content: prompt},
	})
	if err != nil {
		return nil, err
	}

	resp = cleanJSONResponse(resp)
	var segments []string
	if err := json.Unmarshal([]byte(resp), &segments); err != nil {
		return nil, fmt.Errorf("parse segments: %w", err)
	}
	if len(segments) == 0 {
		return []string{conversation}, nil
	}
	return segments, nil
}

func (s *Service) evaluateSegments(ctx context.Context, segments []string, projectCtx string) ([]segmentEval, error) {
	var sb strings.Builder
	sb.WriteString(`Evaluate each conversation segment's knowledge value.

Categories:
- subjective: contains personal decisions, preferences, thinking/reasoning process, lessons learned, experience summaries
- objective: contains business rules, architecture design, technical facts, workflow descriptions, domain knowledge
- mixed: contains both subjective and objective value
- none: no knowledge value (API lookups, simple code generation, chitchat, formatting)

Score (0-10):
  8-10: High value (clear decision reasoning / important business rules / architecture design / hard-won lessons)
  5-7: Medium value (useful experience / general business knowledge / non-obvious technical details)
  0-4: Low value (discard)

`)
	if projectCtx != "" {
		sb.WriteString("Project context: ")
		sb.WriteString(projectCtx)
		sb.WriteString("\n\n")
	}
	sb.WriteString("Segments:\n")
	for i, seg := range segments {
		sb.WriteString(fmt.Sprintf("[%d]: ", i))
		r := []rune(seg)
		if len(r) > segmentTruncateLen {
			sb.WriteString(string(r[:segmentTruncateLen]))
			sb.WriteString("...")
		} else {
			sb.WriteString(seg)
		}
		sb.WriteString("\n\n")
	}
	sb.WriteString(`Return JSON array: [{"index": 0, "score": 7, "category": "subjective", "reason": "..."}]`)

	resp, err := s.llm.Chat(ctx, []port.LLMMessage{
		{Role: "system", Content: "You evaluate conversation segments for knowledge value. Respond only with a JSON array."},
		{Role: "user", Content: sb.String()},
	})
	if err != nil {
		return nil, err
	}

	resp = cleanJSONResponse(resp)
	var evals []segmentEval
	if err := json.Unmarshal([]byte(resp), &evals); err != nil {
		return nil, fmt.Errorf("parse evaluations: %w", err)
	}
	return evals, nil
}

func (s *Service) extractFromSegment(ctx context.Context, segment, category, projectCtx string) ([]captureItem, error) {
	var items []captureItem

	if category == "subjective" || category == "mixed" {
		extracted, err := s.extractSubjective(ctx, segment, projectCtx)
		if err != nil {
			slog.Warn("subjective extraction failed", "error", err)
		} else {
			items = append(items, extracted...)
		}
	}

	if category == "objective" || category == "mixed" {
		extracted, err := s.extractObjective(ctx, segment, projectCtx)
		if err != nil {
			slog.Warn("objective extraction failed", "error", err)
		} else {
			items = append(items, extracted...)
		}
	}

	return items, nil
}

func (s *Service) extractSubjective(ctx context.Context, segment, projectCtx string) ([]captureItem, error) {
	var ctxLine string
	if projectCtx != "" {
		ctxLine = "\nProject context: " + projectCtx + "\n"
	}

	prompt := fmt.Sprintf(`Extract personal/subjective knowledge from this conversation segment.
Focus on the WHY behind decisions, not just WHAT was decided.

Extract as JSON array, each item:
- "title": concise title
- "content": detailed description, must include reasoning/context/motivation
- "knowledge_type": one of "preference" (personal style/preference), "thinking" (reasoning process), "lesson" (hard-won lesson/pitfall), "decision" (technical choice with rationale), "procedure" (step-by-step workflow the user followed to accomplish a task)
- "knowledge_category": "subjective"
- "tags": relevant keyword tags as string array

Rules:
- Extract the reasoning chain, not just the conclusion
- "Chose X because Y" is valuable; "Used X" alone is not
- Skip if it's just following standard practice with no personal judgment
- If nothing valuable, return empty array []
%s
Conversation segment:
%s`, ctxLine, segment)

	resp, err := s.llm.Chat(ctx, []port.LLMMessage{
		{Role: "system", Content: "You extract personal knowledge from conversations. Respond only with a JSON array."},
		{Role: "user", Content: prompt},
	})
	if err != nil {
		return nil, err
	}

	resp = cleanJSONResponse(resp)
	var items []captureItem
	if err := json.Unmarshal([]byte(resp), &items); err != nil {
		return nil, fmt.Errorf("parse subjective items: %w", err)
	}
	return items, nil
}

func (s *Service) extractObjective(ctx context.Context, segment, projectCtx string) ([]captureItem, error) {
	var ctxLine string
	if projectCtx != "" {
		ctxLine = "\nProject context: " + projectCtx + "\n"
	}

	prompt := fmt.Sprintf(`Extract objective/factual knowledge from this conversation segment.
Focus on business rules, architecture facts, and reusable technical details.

Extract as JSON array, each item:
- "title": concise title
- "content": detailed description with complete context
- "knowledge_type": one of "business" (business rule/workflow/domain logic), "architecture" (system design/component relationships), "fact" (technical fact/configuration/parameter), "faq" (common question with answer), "procedure" (reusable step-by-step operational process or checklist)
- "knowledge_category": "objective"
- "tags": relevant keyword tags as string array

Rules:
- Business rules must include conditions and behaviors (IF X THEN Y)
- Architecture knowledge must describe component relationships
- Facts must be specific and actionable, not generic
- If nothing valuable, return empty array []
%s
Conversation segment:
%s`, ctxLine, segment)

	resp, err := s.llm.Chat(ctx, []port.LLMMessage{
		{Role: "system", Content: "You extract factual knowledge from conversations. Respond only with a JSON array."},
		{Role: "user", Content: prompt},
	})
	if err != nil {
		return nil, err
	}

	resp = cleanJSONResponse(resp)
	var items []captureItem
	if err := json.Unmarshal([]byte(resp), &items); err != nil {
		return nil, fmt.Errorf("parse objective items: %w", err)
	}
	return items, nil
}

func cleanJSONResponse(resp string) string {
	resp = strings.TrimSpace(resp)
	resp = strings.TrimPrefix(resp, "```json")
	resp = strings.TrimPrefix(resp, "```")
	resp = strings.TrimSuffix(resp, "```")
	return strings.TrimSpace(resp)
}

// --- AutoCapture without LLM (keyword-based fallback) ---

var decisionKeywords = []string{
	"决定", "选择", "采用", "放弃", "对比", "权衡", "chose", "decided",
	"prefer", "instead of", "rather than", "because", "方案", "架构",
}

var insightKeywords = []string{
	"发现", "原来", "其实", "关键是", "根本原因", "root cause",
	"realized", "turns out", "actually", "lesson", "总结", "经验", "教训",
}

var businessKeywords = []string{
	"业务", "规则", "流程", "需求", "产品", "订单", "支付",
	"business", "rule", "workflow", "requirement",
}

func (s *Service) autoCaptureWithoutLLM(ctx context.Context, conversation, projectCtx string) (*AutoCaptureResult, error) {
	lower := strings.ToLower(conversation)
	stats := CaptureStats{Segments: 1}

	var category string
	hasSignal := false

	for _, kw := range decisionKeywords {
		if strings.Contains(lower, kw) {
			category = domain.CategorySubjective
			hasSignal = true
			break
		}
	}
	if !hasSignal {
		for _, kw := range insightKeywords {
			if strings.Contains(lower, kw) {
				category = domain.CategorySubjective
				hasSignal = true
				break
			}
		}
	}
	if !hasSignal {
		for _, kw := range businessKeywords {
			if strings.Contains(lower, kw) {
				category = domain.CategoryObjective
				hasSignal = true
				break
			}
		}
	}
	if !hasSignal {
		for _, kw := range errorKeywords {
			if strings.Contains(lower, kw) {
				category = domain.CategorySubjective
				hasSignal = true
				break
			}
		}
	}
	if !hasSignal {
		for _, kw := range negationKeywords {
			if strings.Contains(lower, kw) {
				category = domain.CategorySubjective
				hasSignal = true
				break
			}
		}
	}

	if !hasSignal {
		stats.Discarded = 1
		return &AutoCaptureResult{Stats: stats}, nil
	}

	stats.Valuable = 1
	title := generateTitle(conversation)
	knowledgeType := domain.TypeGeneral
	if category == domain.CategorySubjective {
		knowledgeType = domain.TypeLesson
		stats.Subjective = 1
	} else {
		knowledgeType = domain.TypeBusiness
		stats.Objective = 1
	}

	result, err := s.saveWithTypeAndCategory(ctx, title, conversation, domain.SourceConversation, "", nil, knowledgeType, category)
	if err != nil {
		return nil, err
	}

	return &AutoCaptureResult{
		Captured: result.Saved,
		Count:    1,
		Items:    []SaveResult{*result},
		Stats:    stats,
	}, nil
}

// --- Human review: list / approve / reject ---

type PendingItem struct {
	ID                string               `json:"id"`
	Title             string               `json:"title"`
	Content           string               `json:"content"`
	KnowledgeType     string               `json:"knowledge_type"`
	KnowledgeCategory string               `json:"knowledge_category"`
	Tags              []string             `json:"tags"`
	Source            string               `json:"source"`
	CreatedAt         string               `json:"created_at"`
	LLMSuggestion     *domain.ReviewResult `json:"llm_suggestion,omitempty"`
}

type ListPendingResult struct {
	Items []PendingItem `json:"items"`
	Total int           `json:"total"`
}

func (s *Service) ListPending(ctx context.Context, limit int) (*ListPendingResult, error) {
	identity := port.IdentityFromContext(ctx)

	if limit <= 0 {
		limit = defaultPendingLimit
	}

	pending, err := s.store.ListByReviewStatus(ctx, identity.OwnerID, domain.ReviewPending, 0, limit)
	if err != nil {
		return nil, err
	}

	total, _ := s.store.CountByReviewStatus(ctx, identity.OwnerID, domain.ReviewPending)

	items := make([]PendingItem, 0, len(pending))
	for _, k := range pending {
		item := PendingItem{
			ID:                k.ID,
			Title:             k.Title,
			Content:           k.Content,
			KnowledgeType:     k.KnowledgeType,
			KnowledgeCategory: k.KnowledgeCategory,
			Tags:              k.Tags,
			Source:            k.Source,
			CreatedAt:         k.CreatedAt.Format(time.RFC3339),
		}

		if k.ReviewReason != "" {
			item.LLMSuggestion = &domain.ReviewResult{
				Status:     k.ReviewStatus,
				Confidence: k.Confidence,
				Reason:     k.ReviewReason,
			}
		}

		items = append(items, item)
	}

	return &ListPendingResult{
		Items: items,
		Total: total,
	}, nil
}

func (s *Service) SuggestReview(ctx context.Context, id string) (*domain.ReviewResult, error) {
	identity := port.IdentityFromContext(ctx)
	if s.reviewer == nil {
		return nil, fmt.Errorf("reviewer not configured")
	}

	k, err := s.store.Get(ctx, identity.OwnerID, id)
	if err != nil {
		return nil, err
	}
	if k == nil {
		return nil, fmt.Errorf("knowledge item not found")
	}

	suggestion, err := s.reviewer.Review(ctx, k)
	if err != nil {
		return nil, err
	}

	if err := s.store.UpdateReviewStatus(ctx, identity.OwnerID, id, domain.ReviewPending, suggestion.Confidence, suggestion.Reason); err != nil {
		slog.Warn("failed to persist LLM suggestion", "id", id, "error", err)
	}

	return suggestion, nil
}

func (s *Service) SetReviewStatus(ctx context.Context, id string, status string, reason string) error {
	identity := port.IdentityFromContext(ctx)

	k, err := s.store.Get(ctx, identity.OwnerID, id)
	if err != nil {
		return err
	}
	if k == nil {
		return fmt.Errorf("knowledge item not found")
	}

	valid := map[string]bool{
		domain.ReviewPending:       true,
		domain.ReviewApproved:      true,
		domain.ReviewRejected:      true,
		domain.ReviewNeedsRevision: true,
	}
	if !valid[status] {
		return fmt.Errorf("invalid review status: %s", status)
	}

	confidence := 0
	if status == domain.ReviewApproved {
		confidence = 100
	}
	if reason == "" {
		reason = "manually set to " + status
	}
	return s.store.UpdateReviewStatus(ctx, identity.OwnerID, id, status, confidence, reason)
}

func (s *Service) ApproveKnowledge(ctx context.Context, id string, reason string) error {
	return s.SetReviewStatus(ctx, id, domain.ReviewApproved, reason)
}

func (s *Service) RejectKnowledge(ctx context.Context, id string, reason string) error {
	return s.SetReviewStatus(ctx, id, domain.ReviewRejected, reason)
}

func (s *Service) RequestRevision(ctx context.Context, id string, reason string) error {
	return s.SetReviewStatus(ctx, id, domain.ReviewNeedsRevision, reason)
}

func (s *Service) Feedback(ctx context.Context, itemID string) error {
	identity := port.IdentityFromContext(ctx)
	return s.store.IncrementUsefulCount(ctx, identity.OwnerID, itemID)
}

func (s *Service) Maintain(ctx context.Context, taskNames ...string) ([]domain.MaintainResult, error) {
	identity := port.IdentityFromContext(ctx)
	if s.maintainer == nil {
		return nil, fmt.Errorf("maintainer not configured")
	}
	return s.maintainer.Run(ctx, identity.OwnerID, taskNames...)
}

func (s *Service) ListMaintainTasks() []string {
	if s.maintainer == nil {
		return nil
	}
	return s.maintainer.ListTasks()
}

// UpdateKnowledge updates an existing knowledge item's editable fields.
func (s *Service) UpdateKnowledge(ctx context.Context, id, title, content string, tags []string, knowledgeType string) (*SaveResult, error) {
	identity := port.IdentityFromContext(ctx)
	existing, err := s.store.Get(ctx, identity.OwnerID, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, fmt.Errorf("knowledge item not found")
	}

	if title != "" {
		existing.Title = title
	}
	if content != "" {
		existing.Content = content
		if s.embedder != nil {
			emb, err := s.embedder.Embed(ctx, existing.Title+" "+content)
			if err != nil {
				slog.Warn("re-embed failed on update", "id", id, "error", err)
			} else if err := s.store.UpdateEmbedding(ctx, identity.OwnerID, id, emb); err != nil {
				slog.Warn("update embedding failed", "id", id, "error", err)
			}
		}
	}
	if tags != nil {
		existing.Tags = tags
	}
	if knowledgeType != "" {
		existing.KnowledgeType = knowledgeType
	}
	existing.UpdatedAt = time.Now()

	if err := s.store.Update(ctx, existing); err != nil {
		return nil, err
	}

	return &SaveResult{
		Saved:  true,
		ID:     id,
		Title:  existing.Title,
		Tags:   existing.Tags,
		Action: domain.ActionUpdated,
	}, nil
}

// SearchLogStats returns search query analytics.
func (s *Service) SearchLogStats(ctx context.Context) (*domain.SearchLogStats, error) {
	identity := port.IdentityFromContext(ctx)
	return s.store.SearchLogStats(ctx, identity.OwnerID)
}

func (s *Service) KnowledgeHitRanking(ctx context.Context, limit int) ([]domain.KnowledgeHitRank, error) {
	identity := port.IdentityFromContext(ctx)
	return s.store.KnowledgeHitRanking(ctx, identity.OwnerID, limit)
}

func (s *Service) ListSearchLogsDetailed(ctx context.Context, offset, limit int, sourceFilter string) ([]domain.SearchLogDetail, int, error) {
	identity := port.IdentityFromContext(ctx)
	if limit <= 0 {
		limit = defaultSearchLogsLimit
	}
	if offset < 0 {
		offset = 0
	}

	total, err := s.store.CountSearchLogs(ctx, identity.OwnerID, sourceFilter)
	if err != nil {
		return nil, 0, err
	}

	logs, err := s.store.ListSearchLogsFiltered(ctx, identity.OwnerID, sourceFilter, offset, limit)
	if err != nil {
		return nil, 0, err
	}

	details := make([]domain.SearchLogDetail, 0, len(logs))
	for _, l := range logs {
		details = append(details, domain.SearchLogDetail{
			ID:             l.ID,
			Query:          l.Query,
			Source:         l.Source,
			ResultCount:    l.ResultCount,
			ResultIDs:      l.ResultIDs,
			FeedbackBadIDs: l.FeedbackBadIDs,
			CreatedAt:      l.CreatedAt.Format(time.RFC3339),
		})
	}
	return details, total, nil
}

func (s *Service) MarkBadRecall(ctx context.Context, searchLogID, badItemID string) error {
	identity := port.IdentityFromContext(ctx)
	return s.store.MarkSearchBadFeedback(ctx, identity.OwnerID, searchLogID, badItemID)
}

// GetKnowledge retrieves a single knowledge item.
func (s *Service) GetKnowledge(ctx context.Context, id string) (*domain.Knowledge, error) {
	identity := port.IdentityFromContext(ctx)
	return s.store.Get(ctx, identity.OwnerID, id)
}

// ListKnowledge returns paginated knowledge items.
func (s *Service) ListKnowledge(ctx context.Context, offset, limit int) ([]*domain.Knowledge, int, error) {
	identity := port.IdentityFromContext(ctx)
	items, err := s.store.List(ctx, identity.OwnerID, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.store.Count(ctx, identity.OwnerID)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// DeleteKnowledge deletes a knowledge item.
func (s *Service) DeleteKnowledge(ctx context.Context, id string) error {
	identity := port.IdentityFromContext(ctx)
	return s.store.Delete(ctx, identity.OwnerID, id)
}

type ExportData struct {
	Version  string       `json:"version"`
	ExportAt string       `json:"export_at"`
	Total    int          `json:"total"`
	Items    []ExportItem `json:"items"`
}

type ExportItem struct {
	Title             string   `json:"title"`
	Content           string   `json:"content"`
	Summary           string   `json:"summary,omitempty"`
	Source            string   `json:"source"`
	SourceRef         string   `json:"source_ref,omitempty"`
	KnowledgeType     string   `json:"knowledge_type"`
	KnowledgeCategory string   `json:"knowledge_category,omitempty"`
	Tags              []string `json:"tags"`
	Status            string   `json:"status"`
	ReviewStatus      string   `json:"review_status"`
	Confidence        int      `json:"confidence"`
	HitCount          int      `json:"hit_count"`
	UsefulCount       int      `json:"useful_count"`
	CreatedAt         string   `json:"created_at"`
}

func (s *Service) Export(ctx context.Context) (*ExportData, error) {
	identity := port.IdentityFromContext(ctx)

	total, err := s.store.Count(ctx, identity.OwnerID)
	if err != nil {
		return nil, err
	}

	var allItems []ExportItem
	for offset := 0; offset < total; offset += exportBatchSize {
		items, err := s.store.List(ctx, identity.OwnerID, offset, exportBatchSize)
		if err != nil {
			return nil, err
		}
		for _, k := range items {
			allItems = append(allItems, ExportItem{
				Title:             k.Title,
				Content:           k.Content,
				Summary:           k.Summary,
				Source:            k.Source,
				SourceRef:         k.SourceRef,
				KnowledgeType:     k.KnowledgeType,
				KnowledgeCategory: k.KnowledgeCategory,
				Tags:              k.Tags,
				Status:            k.Status,
				ReviewStatus:      k.ReviewStatus,
				Confidence:        k.Confidence,
				HitCount:          k.HitCount,
				UsefulCount:       k.UsefulCount,
				CreatedAt:         k.CreatedAt.Format(time.RFC3339),
			})
		}
	}

	return &ExportData{
		Version:  "1.0",
		ExportAt: time.Now().Format(time.RFC3339),
		Total:    len(allItems),
		Items:    allItems,
	}, nil
}

type BulkImportResult struct {
	Total    int `json:"total"`
	Imported int `json:"imported"`
	Skipped  int `json:"skipped"`
	Failed   int `json:"failed"`
}

func (s *Service) BulkImport(ctx context.Context, data *ExportData) (*BulkImportResult, error) {
	result := &BulkImportResult{Total: len(data.Items)}
	identity := port.IdentityFromContext(ctx)

	for _, item := range data.Items {
		tags := item.Tags
		if tags == nil {
			tags = []string{}
		}

		saveResult, err := s.Save(ctx, item.Title, item.Content, item.Source, item.SourceRef, tags)
		if err != nil {
			slog.Warn("bulk import item failed", "title", item.Title, "error", err)
			result.Failed++
			continue
		}

		if !saveResult.Saved {
			result.Skipped++
			continue
		}

		if item.KnowledgeType != "" || item.KnowledgeCategory != "" {
			if existing, err := s.store.Get(ctx, identity.OwnerID, saveResult.ID); err == nil && existing != nil {
				if item.KnowledgeType != "" {
					existing.KnowledgeType = item.KnowledgeType
				}
				if item.KnowledgeCategory != "" {
					existing.KnowledgeCategory = item.KnowledgeCategory
				}
				existing.UpdatedAt = time.Now()
				if err := s.store.Update(ctx, existing); err != nil {
					slog.Warn("bulk import: update type/category failed", "id", saveResult.ID, "error", err)
				}
			}
		}

		if item.ReviewStatus == domain.ReviewApproved {
			if err := s.store.UpdateReviewStatus(ctx, identity.OwnerID, saveResult.ID, domain.ReviewApproved, item.Confidence, "imported as approved"); err != nil {
				slog.Warn("bulk import: update review status failed", "id", saveResult.ID, "error", err)
			}
		}

		result.Imported++
	}

	return result, nil
}

// Stats returns knowledge base statistics.
func (s *Service) Stats(ctx context.Context) (map[string]any, error) {
	identity := port.IdentityFromContext(ctx)
	total, err := s.store.Count(ctx, identity.OwnerID)
	if err != nil {
		return nil, err
	}
	active, err := s.store.CountByStatus(ctx, identity.OwnerID, domain.StatusActive)
	if err != nil {
		return nil, err
	}
	synthesis, err := s.store.CountByStatus(ctx, identity.OwnerID, domain.StatusSynthesis)
	if err != nil {
		return nil, err
	}
	consolidated, err := s.store.CountByStatus(ctx, identity.OwnerID, domain.StatusConsolidated)
	if err != nil {
		return nil, err
	}

	pending, err := s.store.CountByReviewStatus(ctx, identity.OwnerID, domain.ReviewPending)
	if err != nil {
		return nil, err
	}
	approved, err := s.store.CountByReviewStatus(ctx, identity.OwnerID, domain.ReviewApproved)
	if err != nil {
		return nil, err
	}
	rejected, err := s.store.CountByReviewStatus(ctx, identity.OwnerID, domain.ReviewRejected)
	if err != nil {
		return nil, err
	}

	tagCounts, err := s.store.TagStats(ctx, identity.OwnerID, domain.StatusActive)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"total":        total,
		"active":       active,
		"synthesis":    synthesis,
		"consolidated": consolidated,
		"review": map[string]int{
			"pending":  pending,
			"approved": approved,
			"rejected": rejected,
		},
		"tags": tagCounts,
	}, nil
}

func (s *Service) BackfillEmbeddings(ctx context.Context) {
	if s.embedder == nil {
		return
	}

	ownerIDs, err := s.store.AllOwnerIDs(ctx)
	if err != nil {
		slog.Error("backfill: list owners failed", "error", err)
		return
	}

	for _, ownerID := range ownerIDs {
		items, err := s.store.ListWithoutEmbedding(ctx, ownerID, maxBackfillItems)
		if err != nil {
			slog.Error("backfill: list failed", "owner", ownerID, "error", err)
			continue
		}

		count := 0
		for _, item := range items {
			emb, err := s.embedder.Embed(ctx, item.Title+" "+item.Content)
			if err != nil {
				slog.Warn("backfill: embed failed", "id", item.ID, "error", err)
				continue
			}
			if err := s.store.UpdateEmbedding(ctx, ownerID, item.ID, emb); err != nil {
				slog.Warn("backfill: update failed", "id", item.ID, "error", err)
				continue
			}
			count++
		}
		if count > 0 {
			slog.Info("backfill completed", "owner", ownerID, "count", count)
		}
	}
}

type captureItem struct {
	Title             string   `json:"title"`
	Content           string   `json:"content"`
	Tags              []string `json:"tags"`
	KnowledgeType     string   `json:"knowledge_type,omitempty"`
	KnowledgeCategory string   `json:"knowledge_category,omitempty"`
}

type chunk struct {
	title   string
	content string
	tags    []string
}

func generateTitle(content string) string {
	lines := strings.SplitN(content, "\n", 2)
	title := strings.TrimSpace(lines[0])
	runes := []rune(title)
	if len(runes) > maxTitleRunes {
		title = string(runes[:maxTitleRunes])
	}
	if title == "" {
		title = "Untitled"
	}
	return title
}

// --- WorkLog ---

func (s *Service) AddWorkLog(ctx context.Context, date, content, project string, tags []string, duration int) (*domain.WorkLog, error) {
	identity := port.IdentityFromContext(ctx)
	if content == "" {
		return nil, fmt.Errorf("content is required")
	}
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	now := time.Now()
	w := &domain.WorkLog{
		ID:        s.idGen.Generate(),
		OwnerID:   identity.OwnerID,
		Date:      date,
		Content:   content,
		Project:   project,
		Tags:      tags,
		Duration:  duration,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.store.SaveWorkLog(ctx, w); err != nil {
		return nil, err
	}
	return w, nil
}

func (s *Service) UpdateWorkLog(ctx context.Context, id, date, content, project string, tags []string, duration int) (*domain.WorkLog, error) {
	identity := port.IdentityFromContext(ctx)
	existing, err := s.store.GetWorkLog(ctx, identity.OwnerID, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, fmt.Errorf("worklog not found")
	}
	if date != "" {
		existing.Date = date
	}
	if content != "" {
		existing.Content = content
	}
	if project != "" {
		existing.Project = project
	}
	if tags != nil {
		existing.Tags = tags
	}
	if duration >= 0 {
		existing.Duration = duration
	}
	existing.UpdatedAt = time.Now()
	if err := s.store.UpdateWorkLog(ctx, existing); err != nil {
		return nil, err
	}
	return existing, nil
}

func (s *Service) DeleteWorkLog(ctx context.Context, id string) error {
	identity := port.IdentityFromContext(ctx)
	return s.store.DeleteWorkLog(ctx, identity.OwnerID, id)
}

func (s *Service) ListWorkLogs(ctx context.Context, dateFrom, dateTo string, offset, limit int) ([]*domain.WorkLog, int, error) {
	identity := port.IdentityFromContext(ctx)
	if limit <= 0 {
		limit = 50
	}
	items, err := s.store.ListWorkLogs(ctx, identity.OwnerID, dateFrom, dateTo, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.store.CountWorkLogs(ctx, identity.OwnerID, dateFrom, dateTo)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *Service) GetWorkLog(ctx context.Context, id string) (*domain.WorkLog, error) {
	identity := port.IdentityFromContext(ctx)
	return s.store.GetWorkLog(ctx, identity.OwnerID, id)
}

const (
	maxChunkSize         = 3000
	maxTitleRunes        = 80
	longSessionThreshold = 2000

	defaultSearchLimit  = 5
	bfsExpansionDepth   = 2
	maxExpandedItems    = 100
	expansionBaseScore  = 0.5
	expansionScoreDecay = 0.1
	sourceFragmentScore = 0.3

	maxBackfillItems = 1000
	maxContentSize   = 1 << 20 // 1 MB
	exportBatchSize  = 500

	defaultPendingLimit    = 20
	defaultSearchLogsLimit = 50
	asyncTimeout           = 10 * time.Second
	evalScoreThreshold     = 5
	segmentTruncateLen     = 1500
)

func splitDocument(content, fileName string) []chunk {
	lines := strings.Split(content, "\n")
	if len(lines) <= 3 {
		return []chunk{{title: fileName, content: content, tags: []string{fileName}}}
	}

	var chunks []chunk
	var current strings.Builder
	var currentTitle string
	sectionIdx := 0

	flushChunk := func() {
		text := strings.TrimSpace(current.String())
		if text == "" {
			return
		}
		title := currentTitle
		if title == "" {
			title = fmt.Sprintf("%s - Section %d", fileName, sectionIdx+1)
			sectionIdx++
		}
		chunks = append(chunks, chunk{
			title:   title,
			content: text,
			tags:    []string{fileName},
		})
		current.Reset()
		currentTitle = ""
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Detect markdown headings (# through ####)
		headingLevel := 0
		if strings.HasPrefix(trimmed, "####") {
			headingLevel = 4
		} else if strings.HasPrefix(trimmed, "###") {
			headingLevel = 3
		} else if strings.HasPrefix(trimmed, "##") {
			headingLevel = 2
		} else if strings.HasPrefix(trimmed, "#") {
			headingLevel = 1
		}

		// Split on headings level 1-3 (keep #### within current chunk)
		if headingLevel > 0 && headingLevel <= 3 && current.Len() > 0 {
			flushChunk()
		}

		if headingLevel > 0 && headingLevel <= 3 {
			currentTitle = strings.TrimSpace(trimmed[headingLevel:])
		}

		if current.Len() > 0 {
			current.WriteString("\n")
		}
		current.WriteString(line)

		// Split on size limit, but prefer paragraph boundaries
		if current.Len() > maxChunkSize && trimmed == "" {
			flushChunk()
		} else if current.Len() > maxChunkSize*2 {
			flushChunk()
		}
	}

	flushChunk()

	if len(chunks) == 0 {
		return []chunk{{title: fileName, content: content, tags: []string{fileName}}}
	}
	return chunks
}

func sanitizeUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			i++
			continue
		}
		b.WriteRune(r)
		i += size
	}
	return b.String()
}

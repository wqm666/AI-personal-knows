package domain

import "time"

type SearchQuery struct {
	OwnerID string
	Text    string
	Tags    []string
	Limit   int
}

type SearchHit struct {
	Item          Knowledge
	Score         float64
	Retriever     string // keyword / fts / vector
	Freshness     string `json:"freshness,omitempty"`      // latest / outdated
	ConflictGroup string `json:"conflict_group,omitempty"` // groups contradictory items together
}

const (
	FreshnessLatest   = "latest"
	FreshnessOutdated = "outdated"
)

type SearchLog struct {
	ID          string
	OwnerID     string
	Query       string
	ResultCount int
	HadFeedback bool
	CreatedAt   time.Time
}

type SearchLogStats struct {
	TotalQueries    int            `json:"total_queries"`
	WithResults     int            `json:"with_results"`
	ZeroResults     int            `json:"zero_results"`
	RecallRate      float64        `json:"recall_rate"`
	AvgResultCount  float64        `json:"avg_result_count"`
	TopQueries      []QueryCount   `json:"top_queries"`
	ZeroResultTerms []string       `json:"zero_result_terms"`
}

type QueryCount struct {
	Query string `json:"query"`
	Count int    `json:"count"`
}

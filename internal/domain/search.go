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
	Confidence    int    `json:"confidence,omitempty"`     // 0-100, from review
}

const (
	FreshnessLatest   = "latest"
	FreshnessOutdated = "outdated"
)

const (
	SearchSourceAPI = "api"
	SearchSourceWeb = "web"
	SearchSourceMCP = "mcp"
)

type SearchLog struct {
	ID             string
	OwnerID        string
	Query          string
	Source         string // mcp / web / api
	ResultCount    int
	ResultIDs      []string
	HadFeedback    bool
	FeedbackBadIDs []string
	CreatedAt      time.Time
}

type SearchLogStats struct {
	TotalQueries    int          `json:"total_queries"`
	WithResults     int          `json:"with_results"`
	ZeroResults     int          `json:"zero_results"`
	RecallRate      float64      `json:"recall_rate"`
	AvgResultCount  float64      `json:"avg_result_count"`
	TopQueries      []QueryCount `json:"top_queries"`
	ZeroResultTerms []string     `json:"zero_result_terms"`
}

type QueryCount struct {
	Query string `json:"query"`
	Count int    `json:"count"`
}

type KnowledgeHitRank struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	HitCount    int     `json:"hit_count"`
	UsefulCount int     `json:"useful_count"`
	BadCount    int     `json:"bad_count"`
	UsefulRate  float64 `json:"useful_rate"`
}

type SearchLogDetail struct {
	ID             string   `json:"id"`
	Query          string   `json:"query"`
	Source         string   `json:"source"`
	ResultCount    int      `json:"result_count"`
	ResultIDs      []string `json:"result_ids"`
	FeedbackBadIDs []string `json:"feedback_bad_ids"`
	CreatedAt      string   `json:"created_at"`
}

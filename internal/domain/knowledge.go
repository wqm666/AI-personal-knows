package domain

import "time"

type Knowledge struct {
	ID      string
	OwnerID string
	Title   string
	Content string
	Summary string

	Source    string // conversation / document / manual
	SourceRef string

	KnowledgeType     string // pitfall / decision / faq / general / preference / thinking / lesson / business / architecture / fact
	KnowledgeCategory string // subjective / objective

	Tags []string

	Embedding []float64

	RelatedIDs       []string
	SupersededBy     string // ID of newer item that supersedes this one
	Status           string // active / consolidated / synthesis / superseded
	ConsolidatedFrom []string

	ReviewStatus string // pending / approved / rejected / needs_revision
	Confidence   int    // 0-100, LLM-assessed quality score
	ReviewReason string // explanation from the review process

	HitCount    int
	UsefulCount int
	LastHitAt   *time.Time

	CreatedAt  time.Time
	UpdatedAt  time.Time
	ReviewedAt *time.Time
}

const (
	StatusActive       = "active"
	StatusConsolidated = "consolidated"
	StatusSynthesis    = "synthesis"
	StatusDecayed      = "decayed"
	StatusSuperseded   = "superseded"
)

const (
	SourceConversation = "conversation"
	SourceDocument     = "document"
	SourceManual       = "manual"
)

const (
	TypePitfall  = "pitfall"
	TypeDecision = "decision"
	TypeFAQ      = "faq"
	TypeGeneral  = "general"

	TypePreference   = "preference"
	TypeThinking     = "thinking"
	TypeLesson       = "lesson"
	TypeBusiness     = "business"
	TypeArchitecture = "architecture"
	TypeFact         = "fact"
)

const (
	CategorySubjective = "subjective"
	CategoryObjective  = "objective"
)

const (
	ReviewPending       = "pending"
	ReviewApproved      = "approved"
	ReviewRejected      = "rejected"
	ReviewNeedsRevision = "needs_revision"
)

type ReviewResult struct {
	Status     string `json:"status"`     // approved / rejected / needs_revision
	Confidence int    `json:"confidence"` // 0-100
	Reason     string `json:"reason"`
}

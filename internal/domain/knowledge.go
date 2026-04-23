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

	KnowledgeType string // pitfall / decision / faq / general

	Tags []string

	Embedding []float64

	RelatedIDs       []string
	SupersededBy     string // ID of newer item that supersedes this one
	Status           string // active / consolidated / synthesis / superseded
	ConsolidatedFrom []string

	HitCount    int
	UsefulCount int
	LastHitAt   *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
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
)

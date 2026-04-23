package domain

type MaintainResult struct {
	TaskName string
	Success  bool
	Message  string
	Affected int
}

type DedupResult struct {
	Action   string // new / reinforce / relate
	ExistID  string // non-empty when reinforce or relate
	Score    float64
}

const (
	ActionNew        = "new"
	ActionReinforce  = "reinforce"
	ActionRelate     = "relate"
	ActionUpdated    = "updated"
	ActionSupersede  = "supersede"
)

const (
	RetrieverExpansion      = "expansion"
	RetrieverSourceFragment = "source_fragment"
)

const (
	ChunkModeAuto   = "auto"
	ChunkModeSingle = "single"
)

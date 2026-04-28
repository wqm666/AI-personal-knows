package domain

import "time"

type WorkLog struct {
	ID        string
	OwnerID   string
	Date      string // YYYY-MM-DD
	Content   string
	Project   string
	Tags      []string
	Duration  int // minutes
	CreatedAt time.Time
	UpdatedAt time.Time
}

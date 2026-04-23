package port

import (
	"context"

	"github.com/personal-know/internal/domain"
)

type Maintainer interface {
	Run(ctx context.Context, ownerID string, taskNames ...string) ([]domain.MaintainResult, error)
	Register(task MaintainTask)
	ListTasks() []string
}

type MaintainTask interface {
	Name() string
	Description() string
	Run(ctx context.Context, ownerID string) (*domain.MaintainResult, error)
}

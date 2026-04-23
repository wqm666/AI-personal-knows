package maintain

import (
	"context"
	"log/slog"

	"github.com/personal-know/internal/domain"
	"github.com/personal-know/internal/port"
)

type Runner struct {
	tasks map[string]port.MaintainTask
}

func NewRunner() *Runner {
	return &Runner{tasks: make(map[string]port.MaintainTask)}
}

func (r *Runner) Register(task port.MaintainTask) {
	r.tasks[task.Name()] = task
}

func (r *Runner) ListTasks() []string {
	names := make([]string, 0, len(r.tasks))
	for n := range r.tasks {
		names = append(names, n)
	}
	return names
}

func (r *Runner) Run(ctx context.Context, ownerID string, taskNames ...string) ([]domain.MaintainResult, error) {
	targets := taskNames
	if len(targets) == 0 {
		targets = r.ListTasks()
	}

	var results []domain.MaintainResult
	for _, name := range targets {
		task, ok := r.tasks[name]
		if !ok {
			results = append(results, domain.MaintainResult{
				TaskName: name,
				Success:  false,
				Message:  "task not found",
			})
			continue
		}

		slog.Info("maintain task starting", "task", name, "owner", ownerID)
		result, err := task.Run(ctx, ownerID)
		if err != nil {
			slog.Error("maintain task failed", "task", name, "owner", ownerID, "error", err)
			results = append(results, domain.MaintainResult{
				TaskName: name,
				Success:  false,
				Message:  err.Error(),
			})
			continue
		}
		slog.Info("maintain task done", "task", name, "owner", ownerID, "affected", result.Affected)
		results = append(results, *result)
	}
	return results, nil
}

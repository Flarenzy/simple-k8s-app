package reporting

import (
	"context"
	"log/slog"
	"time"

	"github.com/Flarenzy/simple-k8s-app/internal/domain"
)

const defaultCheckInterval = time.Minute

type Runner struct {
	service  domain.ReportingService
	logger   *slog.Logger
	interval time.Duration
}

func NewRunner(service domain.ReportingService, logger *slog.Logger) *Runner {
	return &Runner{service: service, logger: logger, interval: defaultCheckInterval}
}

func (r *Runner) Run(ctx context.Context) {
	r.runCycle(ctx)
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.runCycle(ctx)
		}
	}
}

func (r *Runner) runCycle(ctx context.Context) {
	result, err := r.service.RunSnapshotCycle(ctx)
	if err != nil {
		if ctx.Err() == nil {
			r.logger.ErrorContext(ctx, "subnet usage snapshot cycle failed", "err", err)
		}
		return
	}
	if result.Captured > 0 || result.Deleted > 0 {
		r.logger.InfoContext(ctx, "subnet usage snapshot cycle completed", "captured", result.Captured, "deleted", result.Deleted)
	}
}

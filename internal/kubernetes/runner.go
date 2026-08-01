package kubernetes

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/Flarenzy/simple-k8s-app/internal/domain"
)

type Runner struct {
	config  Config
	lister  ServiceLister
	service domain.KubernetesDiscoveryService
	logger  *slog.Logger
	now     func() time.Time
}

func NewRunner(config Config, lister ServiceLister, service domain.KubernetesDiscoveryService, logger *slog.Logger) *Runner {
	return &Runner{config: config, lister: lister, service: service, logger: logger, now: time.Now}
}

func (r *Runner) Run(ctx context.Context) {
	failureDelay := min(r.config.ReconcileInterval, 30*time.Second)
	for {
		err := r.ReconcileOnce(ctx)
		delay := r.config.ReconcileInterval
		if err != nil && !errors.Is(err, domain.ErrDiscoveryBusy) {
			delay = jitter(failureDelay)
			failureDelay = min(failureDelay*2, r.config.ReconcileInterval)
		} else {
			failureDelay = min(r.config.ReconcileInterval, 30*time.Second)
		}

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (r *Runner) ReconcileOnce(ctx context.Context) error {
	startedAt := r.now().UTC()
	services, err := r.lister.ListServices(ctx)
	if err != nil {
		r.recordFailure(ctx, startedAt, err)
		r.logger.WarnContext(ctx, "kubernetes service discovery failed", "source", r.config.Source.Key, "err", err)
		return err
	}
	observedAt := r.now().UTC()
	result, err := r.service.Reconcile(ctx, r.config.Source, services, observedAt)
	if err != nil {
		if errors.Is(err, domain.ErrDiscoveryBusy) {
			r.logger.DebugContext(ctx, "kubernetes service discovery skipped; source lock held", "source", r.config.Source.Key)
			return err
		}
		r.recordFailure(ctx, startedAt, err)
		r.logger.WarnContext(ctx, "kubernetes service reconciliation failed", "source", r.config.Source.Key, "err", err)
		return err
	}
	r.logger.InfoContext(ctx, "kubernetes service discovery reconciled",
		"source", r.config.Source.Key, "services", result.Services, "matched", result.Matched,
		"unmatched", result.Unmatched, "ambiguous", result.Ambiguous,
	)
	return nil
}

func (r *Runner) recordFailure(ctx context.Context, attemptedAt time.Time, discoveryErr error) {
	if ctx.Err() != nil {
		return
	}
	recordCtx, cancel := context.WithTimeout(ctx, r.config.RequestTimeout)
	defer cancel()
	if err := r.service.RecordFailure(recordCtx, r.config.Source, attemptedAt, discoveryErr); err != nil && !errors.Is(err, domain.ErrDiscoveryBusy) {
		r.logger.ErrorContext(ctx, "recording kubernetes discovery failure", "source", r.config.Source.Key, "err", err)
	}
}

func jitter(delay time.Duration) time.Duration {
	if delay <= 0 {
		return time.Millisecond
	}
	factor := 0.8 + rand.Float64()*0.4
	return time.Duration(float64(delay) * factor)
}

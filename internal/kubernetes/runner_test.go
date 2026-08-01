package kubernetes

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Flarenzy/simple-k8s-app/internal/domain"
)

type stubLister struct {
	services []domain.KubernetesServiceSnapshot
	err      error
}

func (s stubLister) ListServices(context.Context) ([]domain.KubernetesServiceSnapshot, error) {
	return s.services, s.err
}

type stubDiscoveryService struct {
	reconcileCalls int
	failureCalls   int
}

func (s *stubDiscoveryService) Reconcile(context.Context, domain.KubernetesSourceConfig, []domain.KubernetesServiceSnapshot, time.Time) (domain.KubernetesReconcileResult, error) {
	s.reconcileCalls++
	return domain.KubernetesReconcileResult{}, nil
}

func (s *stubDiscoveryService) RecordFailure(context.Context, domain.KubernetesSourceConfig, time.Time, error) error {
	s.failureCalls++
	return nil
}

func (s *stubDiscoveryService) ListSourceStatuses(context.Context) ([]domain.KubernetesSourceStatus, error) {
	return nil, nil
}

func TestRunnerDoesNotPublishPartialList(t *testing.T) {
	service := &stubDiscoveryService{}
	runner := NewRunner(validTestConfig("one", "two"), stubLister{err: errors.New("namespace two failed")}, service, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := runner.ReconcileOnce(context.Background()); err == nil {
		t.Fatal("expected list failure")
	}
	if service.reconcileCalls != 0 || service.failureCalls != 1 {
		t.Fatalf("partial list reached persistence: reconcile=%d failures=%d", service.reconcileCalls, service.failureCalls)
	}
}

func TestRunnerPublishesCompleteEmptySnapshot(t *testing.T) {
	service := &stubDiscoveryService{}
	runner := NewRunner(validTestConfig("apps"), stubLister{services: []domain.KubernetesServiceSnapshot{}}, service, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := runner.ReconcileOnce(context.Background()); err != nil {
		t.Fatalf("ReconcileOnce: %v", err)
	}
	if service.reconcileCalls != 1 || service.failureCalls != 0 {
		t.Fatalf("unexpected calls: reconcile=%d failures=%d", service.reconcileCalls, service.failureCalls)
	}
}

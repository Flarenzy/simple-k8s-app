package domain

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const maxDiscoveryErrorLength = 2048

type kubernetesDiscoveryService struct {
	repository KubernetesDiscoveryRepository
}

func NewKubernetesDiscoveryService(repository KubernetesDiscoveryRepository) KubernetesDiscoveryService {
	return &kubernetesDiscoveryService{repository: repository}
}

func (s *kubernetesDiscoveryService) Reconcile(ctx context.Context, source KubernetesSourceConfig, services []KubernetesServiceSnapshot, observedAt time.Time) (KubernetesReconcileResult, error) {
	return s.repository.Reconcile(ctx, source, services, observedAt)
}

func (s *kubernetesDiscoveryService) RecordFailure(ctx context.Context, source KubernetesSourceConfig, attemptedAt time.Time, discoveryErr error) error {
	message := strings.TrimSpace(discoveryErr.Error())
	if len(message) > maxDiscoveryErrorLength {
		message = message[:maxDiscoveryErrorLength]
	}
	if err := s.repository.RecordFailure(ctx, source, attemptedAt, message); err != nil {
		return fmt.Errorf("record kubernetes discovery failure: %w", err)
	}
	return nil
}

func (s *kubernetesDiscoveryService) ListSourceStatuses(ctx context.Context) ([]KubernetesSourceStatus, error) {
	statuses, err := s.repository.ListSourceStatuses(ctx)
	if statuses == nil {
		statuses = make([]KubernetesSourceStatus, 0)
	}
	return statuses, err
}

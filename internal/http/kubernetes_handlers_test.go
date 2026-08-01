package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Flarenzy/simple-k8s-app/internal/domain"
	"github.com/google/uuid"
)

type statusServiceStub struct {
	statuses []domain.KubernetesSourceStatus
	err      error
}

func (s statusServiceStub) Reconcile(context.Context, domain.KubernetesSourceConfig, []domain.KubernetesServiceSnapshot, time.Time) (domain.KubernetesReconcileResult, error) {
	return domain.KubernetesReconcileResult{}, nil
}

func (s statusServiceStub) RecordFailure(context.Context, domain.KubernetesSourceConfig, time.Time, error) error {
	return nil
}

func (s statusServiceStub) ListSourceStatuses(context.Context) ([]domain.KubernetesSourceStatus, error) {
	return s.statuses, s.err
}

func TestKubernetesSourcesStatus(t *testing.T) {
	api := newHandlerTestAPI(stubService{}, nil)
	api.DiscoveryService = statusServiceStub{statuses: []domain.KubernetesSourceStatus{{
		Source:        domain.KubernetesSource{Key: "prod", Name: "Production"},
		SiteID:        uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"),
		ClusterDomain: "cluster.local", Namespaces: []string{"commerce"}, State: "healthy", Services: 2, Matched: 1,
	}}}
	recorder := httptest.NewRecorder()
	api.Router().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/kubernetes/sources", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	var response []KubernetesDiscoveryStatusResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(response) != 1 || response[0].Source.Key != "prod" || response[0].State != "healthy" {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestKubernetesSourcesStatusFailure(t *testing.T) {
	api := newHandlerTestAPI(stubService{}, nil)
	api.DiscoveryService = statusServiceStub{err: errors.New("boom")}
	recorder := httptest.NewRecorder()
	api.Router().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/kubernetes/sources", nil))
	assertJSONError(t, recorder, http.StatusInternalServerError, "internal server error")
}

package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"github.com/Flarenzy/simple-k8s-app/internal/domain"
	"github.com/google/uuid"
)

type statusServiceStub struct {
	statuses   []domain.KubernetesSourceStatus
	services   []domain.KubernetesServiceObservation
	err        error
	serviceErr error
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

func (s statusServiceStub) ListServicesBySubnetID(context.Context, int64) ([]domain.KubernetesServiceObservation, error) {
	return s.services, s.serviceErr
}

func TestKubernetesServicesBySubnetIncludesEveryMatchState(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	matchedID := domain.IPAddressID("550e8400-e29b-41d4-a716-446655440000")
	matchedSubnetID := int64(42)
	api := newHandlerTestAPI(stubService{getSubnetFn: func(context.Context, int64) (domain.Subnet, error) {
		return domain.Subnet{ID: 42}, nil
	}}, nil)
	api.DiscoveryService = statusServiceStub{services: []domain.KubernetesServiceObservation{
		{
			Source: domain.KubernetesSource{Key: "prod", Name: "Production"}, UID: "uid-matched",
			Name: "orders", Namespace: "commerce", Type: "LoadBalancer", DNSName: "orders.commerce.svc.cluster.local",
			MatchStatus: domain.KubernetesMatchMatched, ObservedAt: now,
			Addresses: []domain.KubernetesAddressObservation{
				{IP: netip.MustParseAddr("10.0.0.10"), Kind: "cluster_ip", MatchStatus: domain.KubernetesMatchMatched, MatchCount: 1, MatchedIPAddressID: &matchedID, MatchedSubnetID: &matchedSubnetID},
				{IP: netip.MustParseAddr("192.0.2.10"), Kind: "load_balancer", IPMode: "VIP", MatchStatus: domain.KubernetesMatchUnmatched},
			},
			Ports: []domain.KubernetesServicePort{{Name: "https", Protocol: "TCP", Port: 443, TargetPort: "8443"}},
		},
		{
			Source: domain.KubernetesSource{Key: "prod", Name: "Production"}, UID: "uid-unmatched",
			Name: "payments", Namespace: "commerce", Type: "ClusterIP", DNSName: "payments.commerce.svc.cluster.local",
			MatchStatus: domain.KubernetesMatchUnmatched, ObservedAt: now,
			Addresses: []domain.KubernetesAddressObservation{{IP: netip.MustParseAddr("10.0.0.20"), Kind: "cluster_ip", MatchStatus: domain.KubernetesMatchUnmatched}},
		},
		{
			Source: domain.KubernetesSource{Key: "prod", Name: "Production"}, UID: "uid-ambiguous",
			Name: "shared", Namespace: "commerce", Type: "ClusterIP", DNSName: "shared.commerce.svc.cluster.local",
			MatchStatus: domain.KubernetesMatchAmbiguous, ObservedAt: now,
			Addresses: []domain.KubernetesAddressObservation{{IP: netip.MustParseAddr("10.0.0.30"), Kind: "cluster_ip", MatchStatus: domain.KubernetesMatchAmbiguous, MatchCount: 2}},
		},
		{
			Source: domain.KubernetesSource{Key: "prod", Name: "Production"}, UID: "uid-headless",
			Name: "headless", Namespace: "commerce", Type: "ClusterIP", DNSName: "headless.commerce.svc.cluster.local",
			MatchStatus: domain.KubernetesMatchNoUsableIP, ObservedAt: now,
			Hostnames: []domain.KubernetesServiceHostname{{Kind: "load_balancer", Hostname: "lb.example.test"}},
		},
	}}

	recorder := httptest.NewRecorder()
	api.Router().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/subnets/42/kubernetes-services", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response []KubernetesServiceObservationResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(response) != 4 {
		t.Fatalf("expected four services, got %+v", response)
	}
	if response[0].MatchStatus != string(domain.KubernetesMatchMatched) || len(response[0].Addresses) != 2 || response[0].Addresses[0].MatchedIPAddressID == nil {
		t.Fatalf("matched service lost observations or link: %+v", response[0])
	}
	if response[1].MatchStatus != string(domain.KubernetesMatchUnmatched) || response[2].MatchStatus != string(domain.KubernetesMatchAmbiguous) {
		t.Fatalf("unmatched or ambiguous state missing: %+v", response)
	}
	if response[3].MatchStatus != string(domain.KubernetesMatchNoUsableIP) || response[3].Addresses == nil || len(response[3].Hostnames) != 1 {
		t.Fatalf("no-usable-IP service was not represented: %+v", response[3])
	}
}

func TestKubernetesSourcesStatus(t *testing.T) {
	api := newHandlerTestAPI(stubService{}, nil)
	api.DiscoveryService = statusServiceStub{statuses: []domain.KubernetesSourceStatus{{
		Source:        domain.KubernetesSource{Key: "prod", Name: "Production"},
		SiteID:        uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"),
		ClusterDomain: "cluster.local", Namespaces: []string{"commerce"}, State: "healthy", Services: 2, Matched: 1, NoUsableIP: 1,
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
	if len(response) != 1 || response[0].Source.Key != "prod" || response[0].State != "healthy" || response[0].NoUsableIP != 1 {
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

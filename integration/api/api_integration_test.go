//go:build integration

package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	containerruntime "github.com/Flarenzy/simple-k8s-app/integration/containerruntime"
	app "github.com/Flarenzy/simple-k8s-app/internal/app"
	appdb "github.com/Flarenzy/simple-k8s-app/internal/db"
	"github.com/Flarenzy/simple-k8s-app/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	postgresPort = "5432/tcp"
	keycloakPort = "8080/tcp"
	testRealm    = "ipam-integration"
	testClientID = "ipam-test"
	testUsername = "integration-user"
	testPassword = "integration-password"
	testAudience = "ipam-api"
	httpReady    = 30 * time.Second
)

type managedContainer interface {
	Terminate(context.Context, ...testcontainers.TerminateOption) error
}

type integrationSuite struct {
	httpClient *http.Client
	baseURL    string
	issuerURL  string
	dsn        string

	postgres managedContainer
	keycloak managedContainer

	apiCancel context.CancelFunc
	apiErrCh  chan error
	closeOnce sync.Once
	closeErr  error
}

type subnetResponse struct {
	ID          int64  `json:"id"`
	CIDR        string `json:"cidr"`
	Description string `json:"description"`
}

type siteResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	SubnetCount  int64  `json:"subnet_count"`
	UsedIPCount  int64  `json:"used_ip_count"`
	TotalIPCount int64  `json:"total_ip_count"`
	FreeIPCount  int64  `json:"free_ip_count"`
}

type ipResponse struct {
	ID                 string `json:"id"`
	IP                 string `json:"ip"`
	Hostname           string `json:"hostname"`
	SubnetID           int64  `json:"subnet_id"`
	KubernetesServices []struct {
		Source struct {
			Key  string `json:"key"`
			Name string `json:"name"`
		} `json:"source"`
		UID              string `json:"uid"`
		Name             string `json:"name"`
		Namespace        string `json:"namespace"`
		Type             string `json:"type"`
		DNSName          string `json:"dns_name"`
		MatchedAddresses []struct {
			IP   string `json:"ip"`
			Kind string `json:"kind"`
		} `json:"matched_addresses"`
		Ports []struct {
			Name       string `json:"name"`
			Protocol   string `json:"protocol"`
			Port       int32  `json:"port"`
			TargetPort string `json:"target_port"`
		} `json:"ports"`
	} `json:"kubernetes_services"`
}

type kubernetesStatusResponse struct {
	Source struct {
		Key string `json:"key"`
	} `json:"source"`
	State         string     `json:"state"`
	LastSuccessAt *time.Time `json:"last_success_at"`
	NoUsableIP    int        `json:"no_usable_ip"`
}

type kubernetesServiceObservationResponse struct {
	Source struct {
		Key  string `json:"key"`
		Name string `json:"name"`
	} `json:"source"`
	UID          string `json:"uid"`
	Name         string `json:"name"`
	Namespace    string `json:"namespace"`
	Type         string `json:"type"`
	ExternalName string `json:"external_name"`
	DNSName      string `json:"dns_name"`
	MatchStatus  string `json:"match_status"`
	Addresses    []struct {
		IP                 string `json:"ip"`
		Kind               string `json:"kind"`
		IPMode             string `json:"ip_mode"`
		MatchStatus        string `json:"match_status"`
		MatchCount         int    `json:"match_count"`
		MatchedIPAddressID string `json:"matched_ip_address_id"`
		MatchedSubnetID    int64  `json:"matched_subnet_id"`
	} `json:"addresses"`
	Hostnames []struct {
		Kind     string `json:"kind"`
		Hostname string `json:"hostname"`
	} `json:"hostnames"`
	Ports []struct {
		Name       string `json:"name"`
		Protocol   string `json:"protocol"`
		Port       int32  `json:"port"`
		TargetPort string `json:"target_port"`
	} `json:"ports"`
	ObservedAt time.Time `json:"observed_at"`
}

type importResponse struct {
	Processed int `json:"processed"`
	Created   int `json:"created"`
	Updated   int `json:"updated"`
	Failed    int `json:"failed"`
	Errors    []struct {
		Row     int    `json:"row"`
		Message string `json:"message"`
	} `json:"errors"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
}

var (
	suiteOnce             sync.Once
	suite                 *integrationSuite
	suiteErr              error
	activeAppleContainers sync.Map
)

func TestMain(m *testing.M) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	go func() {
		sig := <-signals
		if suite != nil {
			closeCtx, closeCancel := context.WithTimeout(context.Background(), time.Minute)
			if err := suite.Close(closeCtx); err != nil {
				fmt.Printf("integration teardown after %s failed: %v\n", sig, err)
			}
			closeCancel()
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		if err := terminateActiveAppleContainers(cleanupCtx); err != nil {
			fmt.Printf("integration active-container teardown after %s failed: %v\n", sig, err)
		}
		cleanupCancel()
		if sig == syscall.SIGTERM {
			os.Exit(143)
		}
		os.Exit(130)
	}()

	code := m.Run()

	if suite != nil {
		closeCtx, closeCancel := context.WithTimeout(context.Background(), time.Minute)
		defer closeCancel()
		if err := suite.Close(closeCtx); err != nil {
			fmt.Printf("integration teardown failed: %v\n", err)
			if code == 0 {
				code = 1
			}
		}
	}
	activeCleanupCtx, activeCleanupCancel := context.WithTimeout(context.Background(), time.Minute)
	if err := terminateActiveAppleContainers(activeCleanupCtx); err != nil {
		fmt.Printf("integration active-container teardown failed: %v\n", err)
		if code == 0 {
			code = 1
		}
	}
	activeCleanupCancel()

	os.Exit(code)
}

func TestAPIStartupFailsWhenJWKSIsUnavailable(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = app.Serve(ctx, app.Config{
		DSN:          "postgres://ipam:ipam@127.0.0.1:5432/ipam?sslmode=disable",
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		AuthEnabled:  true,
		Issuer:       "http://127.0.0.1:1/realms/does-not-exist",
		JWKSURL:      "http://127.0.0.1:1/realms/does-not-exist/protocol/openid-connect/certs",
		Audience:     testAudience,
	}, listener)
	if err == nil {
		t.Fatal("expected startup to fail when jwks cannot be reached")
	}
}

func TestInfrastructureAndAuthBoundaries(t *testing.T) {
	s := mustSuite(t)

	resp, err := s.get(t, "/healthz", "")
	if err != nil {
		t.Fatalf("healthz request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from /healthz, got %d", resp.StatusCode)
	}
	body := s.readBody(t, resp)
	if strings.TrimSpace(body) != "ok" {
		t.Fatalf("expected ok body, got %q", body)
	}

	resp, err = s.get(t, "/readyz", "")
	if err != nil {
		t.Fatalf("readyz request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from /readyz, got %d", resp.StatusCode)
	}
	s.closeBody(t, resp)

	resp, err = s.get(t, "/api/v1/kubernetes/sources", "")
	if err != nil {
		t.Fatalf("unauthenticated discovery status request: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for discovery status without token, got %d", resp.StatusCode)
	}
	s.closeBody(t, resp)

	resp, err = s.get(t, "/api/v1/subnets", "")
	if err != nil {
		t.Fatalf("unauthenticated request: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing token, got %d", resp.StatusCode)
	}
	s.closeBody(t, resp)

	resp, err = s.get(t, "/api/v1/subnets", "not-a-token")
	if err != nil {
		t.Fatalf("invalid-token request: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for invalid token, got %d", resp.StatusCode)
	}
	s.closeBody(t, resp)

	token := s.mustToken(t)
	resp, err = s.get(t, "/api/v1/subnets", token)
	if err != nil {
		t.Fatalf("authenticated request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for authenticated list request, got %d", resp.StatusCode)
	}

	var subnets []subnetResponse
	s.decodeJSON(t, resp, &subnets)
}

func TestKubernetesDiscoveryReconciliationAndEnrichment(t *testing.T) {
	s := mustSuite(t)
	token := s.mustToken(t)

	createSiteResp, err := s.jsonRequest(t, http.MethodPost, "/api/v1/sites", token, map[string]any{"name": "Kubernetes discovery site"})
	if err != nil || createSiteResp.StatusCode != http.StatusCreated {
		t.Fatalf("create site: status=%v err=%v", createSiteResp.StatusCode, err)
	}
	var site siteResponse
	s.decodeJSON(t, createSiteResp, &site)

	createSubnetResp, err := s.jsonRequest(t, http.MethodPost, "/api/v1/subnets", token, map[string]any{
		"cidr": "10.88.0.0/24", "site_id": site.ID, "description": "service addresses",
	})
	if err != nil || createSubnetResp.StatusCode != http.StatusCreated {
		t.Fatalf("create subnet: status=%v err=%v", createSubnetResp.StatusCode, err)
	}
	var subnet subnetResponse
	s.decodeJSON(t, createSubnetResp, &subnet)

	createIPResp, err := s.jsonRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/subnets/%d/ips", subnet.ID), token, map[string]any{
		"ip": "10.88.0.10", "hostname": "manual-orders-name",
	})
	if err != nil || createIPResp.StatusCode != http.StatusCreated {
		t.Fatalf("create ip: status=%v err=%v", createIPResp.StatusCode, err)
	}
	s.closeBody(t, createIPResp)

	pool, err := appdb.NewPool(context.Background(), s.dsn)
	if err != nil {
		t.Fatalf("open discovery repository pool: %v", err)
	}
	defer pool.Close()
	repository := appdb.NewKubernetesDiscoveryRepository(pool)
	siteID := uuid.MustParse(site.ID)
	source := domain.KubernetesSourceConfig{Key: "integration-cluster", Name: "Integration cluster", SiteID: siteID, ClusterDomain: "cluster.test", Namespaces: []string{"commerce"}}
	source.StaleRetention = 7 * 24 * time.Hour
	observedAt := time.Now().UTC().Truncate(time.Microsecond)
	result, err := repository.Reconcile(context.Background(), source, []domain.KubernetesServiceSnapshot{
		{
			UID: "service-uid-1", Namespace: "commerce", Name: "orders", Type: "LoadBalancer", ResourceVersion: "1",
			DNSName:   "orders.commerce.svc.cluster.test",
			Addresses: []domain.KubernetesServiceAddress{{Kind: "cluster_ip", Address: netip.MustParseAddr("10.88.0.10")}, {Kind: "load_balancer", Address: netip.MustParseAddr("192.0.2.88"), IPMode: "VIP"}},
			Hostnames: []domain.KubernetesServiceHostname{{Kind: "load_balancer", Hostname: "orders.example.test"}},
			Ports:     []domain.KubernetesServicePort{{Name: "https", Protocol: "TCP", Port: 443, TargetPort: "8443"}},
		},
		{
			UID: "service-uid-unmatched", Namespace: "commerce", Name: "payments", Type: "ClusterIP", ResourceVersion: "1",
			DNSName:   "payments.commerce.svc.cluster.test",
			Addresses: []domain.KubernetesServiceAddress{{Kind: "cluster_ip", Address: netip.MustParseAddr("192.0.2.99")}},
		},
	}, observedAt)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if result.Matched != 1 || result.Unmatched != 2 || result.Ambiguous != 0 || result.NoUsableIP != 0 {
		t.Fatalf("unexpected match result: %+v", result)
	}

	listResp, err := s.get(t, fmt.Sprintf("/api/v1/subnets/%d/ips", subnet.ID), token)
	if err != nil || listResp.StatusCode != http.StatusOK {
		t.Fatalf("list enriched ips: status=%v err=%v", listResp.StatusCode, err)
	}
	var ips []ipResponse
	s.decodeJSON(t, listResp, &ips)
	if len(ips) != 1 || ips[0].Hostname != "manual-orders-name" || len(ips[0].KubernetesServices) != 1 {
		t.Fatalf("unexpected enriched IP: %+v", ips)
	}
	service := ips[0].KubernetesServices[0]
	if service.Source.Key != source.Key || service.UID != "service-uid-1" || service.DNSName != "orders.commerce.svc.cluster.test" || len(service.MatchedAddresses) != 1 || len(service.Ports) != 1 {
		t.Fatalf("unexpected service enrichment: %+v", service)
	}

	servicesResp, err := s.get(t, fmt.Sprintf("/api/v1/subnets/%d/kubernetes-services", subnet.ID), token)
	if err != nil || servicesResp.StatusCode != http.StatusOK {
		t.Fatalf("list discovered services: status=%v err=%v", servicesResp.StatusCode, err)
	}
	var discoveredServices []kubernetesServiceObservationResponse
	s.decodeJSON(t, servicesResp, &discoveredServices)
	if len(discoveredServices) != 2 {
		t.Fatalf("expected matched and unmatched services, got %+v", discoveredServices)
	}
	matchedService := discoveredServices[0]
	unmatchedService := discoveredServices[1]
	if matchedService.Source.Key != source.Key || matchedService.UID != "service-uid-1" || matchedService.Namespace != "commerce" || matchedService.Name != "orders" || matchedService.Type != "LoadBalancer" || matchedService.MatchStatus != "matched" || len(matchedService.Addresses) != 2 || len(matchedService.Hostnames) != 1 || len(matchedService.Ports) != 1 || !matchedService.ObservedAt.Equal(observedAt) {
		t.Fatalf("matched Service contract incomplete: %+v", matchedService)
	}
	if matchedService.Addresses[0].MatchStatus != "matched" || matchedService.Addresses[0].MatchedIPAddressID == "" || matchedService.Addresses[0].MatchedSubnetID != subnet.ID || matchedService.Addresses[1].MatchStatus != "unmatched" || matchedService.Addresses[1].IPMode != "VIP" {
		t.Fatalf("address observations or links are inaccurate: %+v", matchedService.Addresses)
	}
	if unmatchedService.UID != "service-uid-unmatched" || unmatchedService.MatchStatus != "unmatched" || len(unmatchedService.Addresses) != 1 || unmatchedService.Addresses[0].MatchedIPAddressID != "" {
		t.Fatalf("unmatched Service was filtered or linked: %+v", unmatchedService)
	}

	if err := repository.RecordFailure(context.Background(), source, observedAt.Add(time.Minute), "forbidden"); err != nil {
		t.Fatalf("record failure: %v", err)
	}
	listResp, err = s.get(t, fmt.Sprintf("/api/v1/subnets/%d/ips", subnet.ID), token)
	if err != nil {
		t.Fatalf("list after failure: %v", err)
	}
	s.decodeJSON(t, listResp, &ips)
	if len(ips[0].KubernetesServices) != 1 || ips[0].Hostname != "manual-orders-name" {
		t.Fatalf("last-known-good enrichment was not preserved: %+v", ips[0])
	}

	statusResp, err := s.get(t, "/api/v1/kubernetes/sources", token)
	if err != nil || statusResp.StatusCode != http.StatusOK {
		t.Fatalf("discovery status: status=%v err=%v", statusResp.StatusCode, err)
	}
	var statuses []kubernetesStatusResponse
	s.decodeJSON(t, statusResp, &statuses)
	var sourceStatus kubernetesStatusResponse
	for _, status := range statuses {
		if status.Source.Key == source.Key {
			sourceStatus = status
		}
	}
	if sourceStatus.State != "degraded" || sourceStatus.LastSuccessAt == nil {
		t.Fatalf("unexpected degraded status: %+v", sourceStatus)
	}

	otherSiteResp, err := s.jsonRequest(t, http.MethodPost, "/api/v1/sites", token, map[string]any{"name": "Other Kubernetes discovery site"})
	if err != nil || otherSiteResp.StatusCode != http.StatusCreated {
		t.Fatalf("create reassignment site: status=%v err=%v", otherSiteResp.StatusCode, err)
	}
	var otherSite siteResponse
	s.decodeJSON(t, otherSiteResp, &otherSite)
	reassignResp, err := s.jsonRequest(t, http.MethodPatch, fmt.Sprintf("/api/v1/subnets/%d/site", subnet.ID), token, map[string]any{"site_id": otherSite.ID})
	if err != nil || reassignResp.StatusCode != http.StatusOK {
		t.Fatalf("reassign subnet site: status=%v err=%v", reassignResp.StatusCode, err)
	}
	s.closeBody(t, reassignResp)
	listResp, err = s.get(t, fmt.Sprintf("/api/v1/subnets/%d/ips", subnet.ID), token)
	if err != nil {
		t.Fatalf("list after subnet reassignment: %v", err)
	}
	s.decodeJSON(t, listResp, &ips)
	if len(ips[0].KubernetesServices) != 0 {
		t.Fatalf("stale enrichment crossed site scope: %+v", ips[0])
	}
	reassignResp, err = s.jsonRequest(t, http.MethodPatch, fmt.Sprintf("/api/v1/subnets/%d/site", subnet.ID), token, map[string]any{"site_id": site.ID})
	if err != nil || reassignResp.StatusCode != http.StatusOK {
		t.Fatalf("restore subnet site: status=%v err=%v", reassignResp.StatusCode, err)
	}
	s.closeBody(t, reassignResp)
	listResp, err = s.get(t, fmt.Sprintf("/api/v1/subnets/%d/ips", subnet.ID), token)
	if err != nil {
		t.Fatalf("list after subnet site restoration: %v", err)
	}
	s.decodeJSON(t, listResp, &ips)
	if len(ips[0].KubernetesServices) != 1 {
		t.Fatalf("matched enrichment was not retained for rematching: %+v", ips[0])
	}

	overlapResp, err := s.jsonRequest(t, http.MethodPost, "/api/v1/subnets", token, map[string]any{
		"cidr": "10.88.0.0/25", "site_id": site.ID, "description": "overlap",
	})
	if err != nil || overlapResp.StatusCode != http.StatusCreated {
		t.Fatalf("create overlapping subnet: status=%v err=%v", overlapResp.StatusCode, err)
	}
	var overlap subnetResponse
	s.decodeJSON(t, overlapResp, &overlap)
	overlapIPResp, err := s.jsonRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/subnets/%d/ips", overlap.ID), token, map[string]any{"ip": "10.88.0.10", "hostname": "other-manual-name"})
	if err != nil || overlapIPResp.StatusCode != http.StatusCreated {
		t.Fatalf("create overlapping ip: status=%v err=%v", overlapIPResp.StatusCode, err)
	}
	s.closeBody(t, overlapIPResp)

	result, err = repository.Reconcile(context.Background(), source, []domain.KubernetesServiceSnapshot{{
		UID: "service-uid-1", Namespace: "commerce", Name: "orders", Type: "ClusterIP", ResourceVersion: "2",
		DNSName: "orders.commerce.svc.cluster.test", Addresses: []domain.KubernetesServiceAddress{{Kind: "cluster_ip", Address: netip.MustParseAddr("10.88.0.10")}},
	}}, observedAt.Add(2*time.Minute))
	if err != nil || result.Ambiguous != 1 || result.Matched != 0 {
		t.Fatalf("expected ambiguous site-scoped match, result=%+v err=%v", result, err)
	}
	listResp, err = s.get(t, fmt.Sprintf("/api/v1/subnets/%d/ips", subnet.ID), token)
	if err != nil {
		t.Fatalf("list ambiguous ip: %v", err)
	}
	s.decodeJSON(t, listResp, &ips)
	if len(ips[0].KubernetesServices) != 0 || ips[0].Hostname != "manual-orders-name" {
		t.Fatalf("ambiguous observation linked or rewrote manual data: %+v", ips[0])
	}
	servicesResp, err = s.get(t, fmt.Sprintf("/api/v1/subnets/%d/kubernetes-services", subnet.ID), token)
	if err != nil {
		t.Fatalf("list ambiguous services: %v", err)
	}
	discoveredServices = nil
	s.decodeJSON(t, servicesResp, &discoveredServices)
	if len(discoveredServices) != 1 || discoveredServices[0].MatchStatus != "ambiguous" || len(discoveredServices[0].Addresses) != 1 || discoveredServices[0].Addresses[0].MatchCount != 2 || discoveredServices[0].Addresses[0].MatchedIPAddressID != "" {
		t.Fatalf("ambiguous Service contract is inaccurate: %+v", discoveredServices)
	}

	if _, err := repository.Reconcile(context.Background(), source, []domain.KubernetesServiceSnapshot{}, observedAt.Add(3*time.Minute)); err != nil {
		t.Fatalf("reconcile complete empty snapshot: %v", err)
	}
	var activeServices int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM kubernetes_services WHERE active`).Scan(&activeServices); err != nil {
		t.Fatalf("count active services: %v", err)
	}
	if activeServices != 0 {
		t.Fatalf("complete empty snapshot left %d active services", activeServices)
	}
	if _, err := repository.Reconcile(context.Background(), source, []domain.KubernetesServiceSnapshot{{
		UID: "service-uid-2", Namespace: "commerce", Name: "orders", Type: "ExternalName", ResourceVersion: "1",
		ExternalName: "orders.example.test", DNSName: "orders.commerce.svc.cluster.test",
	}}, observedAt.Add(4*time.Minute)); err != nil {
		t.Fatalf("reconcile recreated service: %v", err)
	}
	var oldActive, newActive bool
	if err := pool.QueryRow(context.Background(), `
		SELECT bool_or(active) FILTER (WHERE kubernetes_uid = 'service-uid-1'),
		       bool_or(active) FILTER (WHERE kubernetes_uid = 'service-uid-2')
		FROM kubernetes_services`).Scan(&oldActive, &newActive); err != nil {
		t.Fatalf("read recreated service identities: %v", err)
	}
	if oldActive || !newActive {
		t.Fatalf("UID identity did not distinguish recreation: old=%t new=%t", oldActive, newActive)
	}
	servicesResp, err = s.get(t, fmt.Sprintf("/api/v1/subnets/%d/kubernetes-services", subnet.ID), token)
	if err != nil {
		t.Fatalf("list no-usable-IP services: %v", err)
	}
	discoveredServices = nil
	s.decodeJSON(t, servicesResp, &discoveredServices)
	if len(discoveredServices) != 1 || discoveredServices[0].UID != "service-uid-2" || discoveredServices[0].ExternalName != "orders.example.test" || discoveredServices[0].MatchStatus != "no_usable_ip" || discoveredServices[0].Addresses == nil || len(discoveredServices[0].Addresses) != 0 {
		t.Fatalf("no-usable-IP Service contract is inaccurate: %+v", discoveredServices)
	}
	statusResp, err = s.get(t, "/api/v1/kubernetes/sources", token)
	if err != nil {
		t.Fatalf("status after no-usable-IP reconcile: %v", err)
	}
	s.decodeJSON(t, statusResp, &statuses)
	for _, status := range statuses {
		if status.Source.Key == source.Key && status.NoUsableIP != 1 {
			t.Fatalf("expected no_usable_ip source count, got %+v", status)
		}
	}
}

func TestSitesCRUDAndStatistics(t *testing.T) {
	s := mustSuite(t)
	token := s.mustToken(t)

	createResp, err := s.jsonRequest(t, http.MethodPost, "/api/v1/sites", token, map[string]any{
		"name":        "Integration site",
		"description": "Site API coverage",
	})
	if err != nil {
		t.Fatalf("create site: %v", err)
	}
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 creating site, got %d", createResp.StatusCode)
	}
	var site siteResponse
	s.decodeJSON(t, createResp, &site)
	if site.ID == "" || site.Name != "Integration site" {
		t.Fatalf("unexpected created site: %+v", site)
	}

	updateResp, err := s.jsonRequest(t, http.MethodPatch, "/api/v1/sites/"+site.ID, token, map[string]any{
		"name":        "Updated integration site",
		"description": "Updated site API coverage",
	})
	if err != nil {
		t.Fatalf("update site: %v", err)
	}
	if updateResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 updating site, got %d", updateResp.StatusCode)
	}
	s.decodeJSON(t, updateResp, &site)
	if site.Name != "Updated integration site" {
		t.Fatalf("unexpected updated site: %+v", site)
	}

	statisticsResp, err := s.get(t, "/api/v1/sites/statistics", token)
	if err != nil {
		t.Fatalf("site statistics: %v", err)
	}
	if statisticsResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 reading site statistics, got %d", statisticsResp.StatusCode)
	}
	var statistics []siteResponse
	s.decodeJSON(t, statisticsResp, &statistics)
	if len(statistics) == 0 {
		t.Fatal("expected site statistics")
	}

	createSubnetResp, err := s.jsonRequest(t, http.MethodPost, "/api/v1/subnets", token, map[string]any{
		"cidr":        "10.60.0.0/24",
		"site_id":     site.ID,
		"description": "Associated integration subnet",
	})
	if err != nil {
		t.Fatalf("create associated subnet: %v", err)
	}
	if createSubnetResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 creating associated subnet, got %d", createSubnetResp.StatusCode)
	}
	var subnet subnetResponse
	s.decodeJSON(t, createSubnetResp, &subnet)

	createIPResp, err := s.jsonRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/subnets/%d/ips", subnet.ID), token, map[string]any{
		"ip":       "10.60.0.10",
		"hostname": "associated-host",
	})
	if err != nil {
		t.Fatalf("create associated subnet IP: %v", err)
	}
	if createIPResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 creating associated subnet IP, got %d", createIPResp.StatusCode)
	}
	var associatedIP ipResponse
	s.decodeJSON(t, createIPResp, &associatedIP)

	statisticsResp, err = s.get(t, "/api/v1/sites/statistics", token)
	if err != nil {
		t.Fatalf("read associated site statistics: %v", err)
	}
	if statisticsResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 reading associated site statistics, got %d", statisticsResp.StatusCode)
	}
	s.decodeJSON(t, statisticsResp, &statistics)
	var associated siteResponse
	for _, item := range statistics {
		if item.ID == site.ID {
			associated = item
			break
		}
	}
	if associated.SubnetCount != 1 || associated.UsedIPCount != 1 || associated.FreeIPCount != 253 || associated.TotalIPCount != 254 {
		t.Fatalf("unexpected associated site statistics: %+v", associated)
	}

	deleteIPResp, err := s.request(t, http.MethodDelete, fmt.Sprintf("/api/v1/subnets/%d/ips/%s", subnet.ID, associatedIP.ID), token, nil)
	if err != nil {
		t.Fatalf("delete associated subnet IP: %v", err)
	}
	if deleteIPResp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 deleting associated subnet IP, got %d", deleteIPResp.StatusCode)
	}
	s.closeBody(t, deleteIPResp)

	deleteSubnetResp, err := s.request(t, http.MethodDelete, fmt.Sprintf("/api/v1/subnets/%d", subnet.ID), token, nil)
	if err != nil {
		t.Fatalf("delete associated subnet: %v", err)
	}
	if deleteSubnetResp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 deleting associated subnet, got %d", deleteSubnetResp.StatusCode)
	}
	s.closeBody(t, deleteSubnetResp)

	deleteResp, err := s.jsonRequest(t, http.MethodDelete, "/api/v1/sites/"+site.ID, token, nil)
	if err != nil {
		t.Fatalf("delete site: %v", err)
	}
	if deleteResp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 deleting site, got %d", deleteResp.StatusCode)
	}
}

func TestCustomerJourney(t *testing.T) {
	s := mustSuite(t)
	token := s.mustToken(t)
	createSiteResp, err := s.jsonRequest(t, http.MethodPost, "/api/v1/sites", token, map[string]any{
		"name": "Customer journey site",
	})
	if err != nil {
		t.Fatalf("create journey site: %v", err)
	}
	if createSiteResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 creating journey site, got %d: %s", createSiteResp.StatusCode, s.readBody(t, createSiteResp))
	}
	var journeySite siteResponse
	s.decodeJSON(t, createSiteResp, &journeySite)

	createSubnetResp, err := s.jsonRequest(
		t,
		http.MethodPost,
		"/api/v1/subnets",
		token,
		map[string]any{
			"cidr":        "10.42.0.0/24",
			"site_id":     journeySite.ID,
			"description": "Integration subnet",
		},
	)
	if err != nil {
		t.Fatalf("create subnet: %v", err)
	}
	if createSubnetResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 creating subnet, got %d", createSubnetResp.StatusCode)
	}

	var subnet subnetResponse
	s.decodeJSON(t, createSubnetResp, &subnet)
	if subnet.ID == 0 {
		t.Fatal("expected subnet id to be populated")
	}
	if subnet.CIDR != "10.42.0.0/24" {
		t.Fatalf("unexpected subnet cidr: %q", subnet.CIDR)
	}

	getSubnetResp, err := s.get(t, fmt.Sprintf("/api/v1/subnets/%d", subnet.ID), token)
	if err != nil {
		t.Fatalf("get subnet: %v", err)
	}
	if getSubnetResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 reading subnet, got %d", getSubnetResp.StatusCode)
	}

	var fetchedSubnet subnetResponse
	s.decodeJSON(t, getSubnetResp, &fetchedSubnet)
	if fetchedSubnet.ID != subnet.ID {
		t.Fatalf("expected subnet id %d, got %d", subnet.ID, fetchedSubnet.ID)
	}

	createIPResp, err := s.jsonRequest(
		t,
		http.MethodPost,
		fmt.Sprintf("/api/v1/subnets/%d/ips", subnet.ID),
		token,
		map[string]any{
			"ip":       "10.42.0.10",
			"hostname": "integration-host",
		},
	)
	if err != nil {
		t.Fatalf("create ip: %v", err)
	}
	if createIPResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 creating ip, got %d", createIPResp.StatusCode)
	}

	var createdIP ipResponse
	s.decodeJSON(t, createIPResp, &createdIP)
	if createdIP.ID == "" {
		t.Fatal("expected ip id to be populated")
	}

	duplicateIPResp, err := s.jsonRequest(
		t,
		http.MethodPost,
		fmt.Sprintf("/api/v1/subnets/%d/ips", subnet.ID),
		token,
		map[string]any{
			"ip":       "10.42.0.10",
			"hostname": "integration-host",
		},
	)
	if err != nil {
		t.Fatalf("duplicate ip request: %v", err)
	}
	if duplicateIPResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for duplicate ip, got %d", duplicateIPResp.StatusCode)
	}

	var duplicateErr errorResponse
	s.decodeJSON(t, duplicateIPResp, &duplicateErr)
	if duplicateErr.Error != "bad request, ip exists" {
		t.Fatalf("unexpected duplicate ip error: %q", duplicateErr.Error)
	}

	outsideIPResp, err := s.jsonRequest(
		t,
		http.MethodPost,
		fmt.Sprintf("/api/v1/subnets/%d/ips", subnet.ID),
		token,
		map[string]any{
			"ip":       "10.43.0.10",
			"hostname": "outside-host",
		},
	)
	if err != nil {
		t.Fatalf("outside ip request: %v", err)
	}
	if outsideIPResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for out-of-subnet ip, got %d", outsideIPResp.StatusCode)
	}

	var outsideErr errorResponse
	s.decodeJSON(t, outsideIPResp, &outsideErr)
	if outsideErr.Error != "bad request" {
		t.Fatalf("unexpected outside ip error: %q", outsideErr.Error)
	}

	listIPResp, err := s.get(t, fmt.Sprintf("/api/v1/subnets/%d/ips", subnet.ID), token)
	if err != nil {
		t.Fatalf("list ips: %v", err)
	}
	if listIPResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 listing ips, got %d", listIPResp.StatusCode)
	}

	var ips []ipResponse
	s.decodeJSON(t, listIPResp, &ips)
	if len(ips) != 1 {
		t.Fatalf("expected 1 ip, got %d", len(ips))
	}
	if ips[0].ID != createdIP.ID {
		t.Fatalf("expected listed ip id %q, got %q", createdIP.ID, ips[0].ID)
	}

	updateIPResp, err := s.jsonRequest(
		t,
		http.MethodPatch,
		fmt.Sprintf("/api/v1/subnets/%d/ips/%s", subnet.ID, createdIP.ID),
		token,
		map[string]any{
			"hostname": "renamed-host",
		},
	)
	if err != nil {
		t.Fatalf("update ip: %v", err)
	}
	if updateIPResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 updating ip, got %d", updateIPResp.StatusCode)
	}

	var updatedIP ipResponse
	s.decodeJSON(t, updateIPResp, &updatedIP)
	if updatedIP.Hostname != "renamed-host" {
		t.Fatalf("expected updated hostname, got %q", updatedIP.Hostname)
	}

	deleteIPResp, err := s.request(t, http.MethodDelete, fmt.Sprintf("/api/v1/subnets/%d/ips/%s", subnet.ID, createdIP.ID), token, nil)
	if err != nil {
		t.Fatalf("delete ip: %v", err)
	}
	if deleteIPResp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 deleting ip, got %d", deleteIPResp.StatusCode)
	}
	s.closeBody(t, deleteIPResp)

	deleteSubnetResp, err := s.request(t, http.MethodDelete, fmt.Sprintf("/api/v1/subnets/%d", subnet.ID), token, nil)
	if err != nil {
		t.Fatalf("delete subnet: %v", err)
	}
	if deleteSubnetResp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 deleting subnet, got %d", deleteSubnetResp.StatusCode)
	}
	s.closeBody(t, deleteSubnetResp)

	deleteSiteResp, err := s.request(t, http.MethodDelete, "/api/v1/sites/"+journeySite.ID, token, nil)
	if err != nil {
		t.Fatalf("delete journey site: %v", err)
	}
	if deleteSiteResp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 deleting journey site, got %d", deleteSiteResp.StatusCode)
	}
	s.closeBody(t, deleteSiteResp)
}

func TestCSVImportJourney(t *testing.T) {
	s := mustSuite(t)
	token := s.mustToken(t)
	siteName := "CSV integration site"
	cidr := "10.77.0.0/24"
	firstCSV := "site,cidr,ip,description\n" +
		siteName + "," + cidr + ",10.77.0.10,printer\n" +
		siteName + "," + cidr + ",10.77.0.11,phone\n"

	resp, err := s.csvRequest(t, token, firstCSV)
	if err != nil {
		t.Fatalf("initial csv import: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from initial csv import, got %d: %s", resp.StatusCode, s.readBody(t, resp))
	}
	var result importResponse
	s.decodeJSON(t, resp, &result)
	if result.Processed != 2 || result.Created != 2 || result.Updated != 0 || result.Failed != 0 {
		t.Fatalf("unexpected initial import result: %+v", result)
	}

	secondCSV := "site,cidr,ip,description\n" +
		siteName + "," + cidr + ",10.77.0.10,updated-printer\n" +
		siteName + "," + cidr + ",10.77.0.11,phone\n"
	resp, err = s.csvRequest(t, token, secondCSV)
	if err != nil {
		t.Fatalf("idempotent csv import: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from repeat csv import, got %d: %s", resp.StatusCode, s.readBody(t, resp))
	}
	s.decodeJSON(t, resp, &result)
	if result.Processed != 2 || result.Created != 0 || result.Updated != 1 || result.Failed != 0 {
		t.Fatalf("unexpected repeat import result: %+v", result)
	}

	resp, err = s.csvRequest(t, token, "site,cidr,ip,description\n"+siteName+",not-a-cidr,10.77.0.12,bad\n")
	if err != nil {
		t.Fatalf("invalid csv import: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for row-level csv error, got %d: %s", resp.StatusCode, s.readBody(t, resp))
	}
	s.decodeJSON(t, resp, &result)
	if result.Processed != 1 || result.Created != 0 || result.Updated != 0 || result.Failed != 1 || len(result.Errors) != 1 || result.Errors[0].Row != 2 {
		t.Fatalf("unexpected invalid import result: %+v", result)
	}

	resp, err = s.get(t, "/api/v1/sites/statistics", token)
	if err != nil {
		t.Fatalf("read csv site statistics: %v", err)
	}
	var statistics []siteResponse
	s.decodeJSON(t, resp, &statistics)
	var found siteResponse
	for _, item := range statistics {
		if item.Name == siteName {
			found = item
			break
		}
	}
	if found.ID == "" || found.SubnetCount != 1 || found.UsedIPCount != 2 || found.TotalIPCount != 254 {
		t.Fatalf("unexpected persisted csv hierarchy statistics: %+v", found)
	}

	resp, err = s.get(t, "/api/v1/subnets", token)
	if err != nil {
		t.Fatalf("list csv subnets: %v", err)
	}
	var subnets []subnetResponse
	s.decodeJSON(t, resp, &subnets)
	var importedSubnet subnetResponse
	for _, subnet := range subnets {
		if subnet.CIDR == cidr {
			importedSubnet = subnet
			break
		}
	}
	if importedSubnet.ID == 0 {
		t.Fatalf("expected imported subnet %q in persisted inventory", cidr)
	}
	resp, err = s.get(t, fmt.Sprintf("/api/v1/subnets/%d/ips", importedSubnet.ID), token)
	if err != nil {
		t.Fatalf("list imported ips: %v", err)
	}
	var ips []ipResponse
	s.decodeJSON(t, resp, &ips)
	if len(ips) != 2 {
		t.Fatalf("expected 2 imported ips, got %d", len(ips))
	}
}

func mustSuite(t *testing.T) *integrationSuite {
	t.Helper()

	suiteOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		suite, suiteErr = newIntegrationSuite(ctx)
	})
	if suiteErr != nil {
		t.Fatalf("integration setup failed: %v", suiteErr)
	}
	if suite == nil {
		t.Fatal("integration suite was not initialized")
	}

	return suite
}

func newIntegrationSuite(ctx context.Context) (*integrationSuite, error) {
	if _, err := exec.LookPath("goose"); err != nil {
		return nil, fmt.Errorf("goose not found in PATH: %w", err)
	}
	runtimeConfig, err := containerruntime.Load(ctx)
	if err != nil {
		return nil, err
	}
	fmt.Printf("integration container runtime: %s\n", runtimeConfig.Summary())
	if runtimeConfig.Runtime == containerruntime.RuntimeTestcontainers {
		if err := os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true"); err != nil {
			return nil, fmt.Errorf("disable testcontainers ryuk: %w", err)
		}
	}

	s := &integrationSuite{
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}

	var dsn string
	s.postgres, dsn, err = startPostgres(ctx, runtimeConfig)
	if err != nil {
		return nil, fmt.Errorf("%w (%s); %s", err, runtimeConfig.Summary(), runtimeConfig.Help())
	}
	s.dsn = dsn

	if err = runGooseMigrations(ctx, dsn); err != nil {
		_ = s.postgres.Terminate(ctx)
		return nil, fmt.Errorf("%w (%s); %s", err, runtimeConfig.Summary(), runtimeConfig.Help())
	}

	s.keycloak, s.issuerURL, err = startKeycloak(ctx, runtimeConfig)
	if err != nil {
		_ = s.postgres.Terminate(ctx)
		return nil, fmt.Errorf("%w (%s); %s", err, runtimeConfig.Summary(), runtimeConfig.Help())
	}

	if err = s.startAPI(ctx, dsn); err != nil {
		_ = s.keycloak.Terminate(ctx)
		_ = s.postgres.Terminate(ctx)
		return nil, err
	}

	return s, nil
}

func (s *integrationSuite) startAPI(ctx context.Context, dsn string) error {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("listen for api: %w", err)
	}

	s.baseURL = "http://" + listener.Addr().String()
	apiCtx, apiCancel := context.WithCancel(context.Background())
	s.apiCancel = apiCancel
	s.apiErrCh = make(chan error, 1)

	go func() {
		s.apiErrCh <- app.Serve(apiCtx, app.Config{
			DSN:          dsn,
			ReadTimeout:  3 * time.Second,
			WriteTimeout: 3 * time.Second,
			AuthEnabled:  true,
			Issuer:       s.issuerURL,
			Audience:     testAudience,
			JWKSURL:      s.issuerURL + "/protocol/openid-connect/certs",
		}, listener)
	}()

	return s.waitForAPIReady(ctx)
}

func (s *integrationSuite) waitForAPIReady(ctx context.Context) error {
	deadline := time.Now().Add(httpReady)
	for time.Now().Before(deadline) {
		select {
		case err := <-s.apiErrCh:
			if err != nil {
				return fmt.Errorf("api exited before becoming ready: %w", err)
			}
			return errors.New("api exited before becoming ready")
		default:
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+"/healthz", nil)
		if err != nil {
			return err
		}

		resp, err := s.httpClient.Do(req)
		if err == nil {
			s.closeBodyNoTest(resp)
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}

		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("timed out waiting for api at %s", s.baseURL)
}

func (s *integrationSuite) Close(ctx context.Context) error {
	s.closeOnce.Do(func() {
		var errs []error

		if s.apiCancel != nil {
			s.apiCancel()
			select {
			case err := <-s.apiErrCh:
				if err != nil {
					errs = append(errs, err)
				}
			case <-time.After(10 * time.Second):
				errs = append(errs, errors.New("timed out waiting for api shutdown"))
			}
		}

		if s.keycloak != nil {
			if err := s.keycloak.Terminate(ctx); err != nil {
				errs = append(errs, err)
			}
		}

		if s.postgres != nil {
			if err := s.postgres.Terminate(ctx); err != nil {
				errs = append(errs, err)
			}
		}

		s.closeErr = errors.Join(errs...)
	})
	return s.closeErr
}

func startPostgres(ctx context.Context, config containerruntime.Config) (managedContainer, string, error) {
	if config.Runtime == containerruntime.RuntimeApple {
		container, port, err := startAppleContainer(ctx, config, appleContainerRequest{
			service:       "postgres",
			image:         config.PostgresImage,
			containerPort: 5432,
			env: map[string]string{
				"POSTGRES_DB":       "ipam",
				"POSTGRES_USER":     "ipam",
				"POSTGRES_PASSWORD": "ipam",
			},
		})
		if err != nil {
			return nil, "", fmt.Errorf("start postgres with Apple Container: %w", err)
		}
		dsn := fmt.Sprintf("postgres://ipam:ipam@127.0.0.1:%d/ipam?sslmode=disable", port)
		if err := waitForPostgres(ctx, dsn, config.StartupTimeout); err != nil {
			logs, cleanupErr := diagnoseAndTerminateAppleContainer(container)
			return nil, "", fmt.Errorf("%w; postgres logs:\n%s%s", err, logs, cleanupFailure(cleanupErr))
		}
		return container, dsn, nil
	}

	req := testcontainers.ContainerRequest{
		Image:        config.PostgresImage,
		ExposedPorts: []string{postgresPort},
		Env: map[string]string{
			"POSTGRES_DB":       "ipam",
			"POSTGRES_USER":     "ipam",
			"POSTGRES_PASSWORD": "ipam",
		},
		WaitingFor: wait.ForListeningPort(postgresPort).WithStartupTimeout(config.StartupTimeout),
	}

	container, err := startTestcontainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, "", fmt.Errorf("start postgres with Testcontainers: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, "", fmt.Errorf("postgres host: %w", err)
	}
	port, err := container.MappedPort(ctx, postgresPort)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, "", fmt.Errorf("postgres mapped port: %w", err)
	}

	return container, fmt.Sprintf("postgres://ipam:ipam@%s:%s/ipam?sslmode=disable", host, port.Port()), nil
}

func runGooseMigrations(ctx context.Context, dsn string) error {
	migrationsDir, err := repoPath("db", "migrations")
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "goose", "-dir", migrationsDir, "postgres", dsn, "up")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("goose migrations failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func waitForPostgres(ctx context.Context, dsn string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		attemptTimeout := min(3*time.Second, time.Until(deadline))
		attemptCtx, attemptCancel := context.WithTimeout(ctx, attemptTimeout)
		connection, err := pgx.Connect(attemptCtx, dsn)
		if err == nil {
			err = connection.Ping(attemptCtx)
			connection.Close(attemptCtx)
			attemptCancel()
			if err == nil {
				return nil
			}
		} else {
			attemptCancel()
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	return fmt.Errorf("timed out after %s waiting for PostgreSQL readiness", timeout)
}

func startKeycloak(ctx context.Context, config containerruntime.Config) (managedContainer, string, error) {
	realmPath, err := repoPath("integration", "api", "testdata", "ipam-integration-realm.json")
	if err != nil {
		return nil, "", fmt.Errorf("resolve realm fixture: %w", err)
	}

	if config.Runtime == containerruntime.RuntimeApple {
		container, port, err := startAppleContainer(ctx, config, appleContainerRequest{
			service:       "keycloak",
			image:         config.KeycloakImage,
			containerPort: 8080,
			memory:        "2g",
			env: map[string]string{
				"KEYCLOAK_ADMIN":          "admin",
				"KEYCLOAK_ADMIN_PASSWORD": "admin",
			},
			mountSource: filepath.Dir(realmPath),
			mountTarget: "/opt/keycloak/data/import",
			command:     []string{"start-dev", "--http-port=8080", "--import-realm"},
		})
		if err != nil {
			return nil, "", fmt.Errorf("start keycloak with Apple Container: %w", err)
		}

		issuerURL := fmt.Sprintf("http://127.0.0.1:%d/realms/%s", port, testRealm)
		if err = waitForHTTP200(ctx, issuerURL+"/.well-known/openid-configuration", config.StartupTimeout); err != nil {
			logs, cleanupErr := diagnoseAndTerminateAppleContainer(container)
			return nil, "", fmt.Errorf("%w; keycloak logs:\n%s%s", err, logs, cleanupFailure(cleanupErr))
		}
		return container, issuerURL, nil
	}

	req := testcontainers.ContainerRequest{
		Image:        config.KeycloakImage,
		ExposedPorts: []string{keycloakPort},
		Env: map[string]string{
			"KEYCLOAK_ADMIN":          "admin",
			"KEYCLOAK_ADMIN_PASSWORD": "admin",
		},
		Cmd: []string{"start-dev", "--http-port=8080", "--import-realm"},
		Files: []testcontainers.ContainerFile{
			{
				HostFilePath:      realmPath,
				ContainerFilePath: "/opt/keycloak/data/import/ipam-integration-realm.json",
				FileMode:          0o644,
			},
		},
		WaitingFor: wait.ForListeningPort(keycloakPort).WithStartupTimeout(config.StartupTimeout),
	}

	container, err := startTestcontainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, "", fmt.Errorf("start keycloak container: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, "", fmt.Errorf("keycloak host: %w", err)
	}
	port, err := container.MappedPort(ctx, keycloakPort)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, "", fmt.Errorf("keycloak mapped port: %w", err)
	}

	issuerURL := fmt.Sprintf("http://%s:%s/realms/%s", host, port.Port(), testRealm)
	if err = waitForHTTP200(ctx, issuerURL+"/.well-known/openid-configuration", config.StartupTimeout); err != nil {
		_ = container.Terminate(ctx)
		return nil, "", err
	}

	return container, issuerURL, nil
}

func startTestcontainer(ctx context.Context, request testcontainers.GenericContainerRequest) (container testcontainers.Container, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("container provider initialization panicked: %v", recovered)
		}
	}()
	return testcontainers.GenericContainer(ctx, request)
}

func waitForHTTP200(ctx context.Context, endpoint string, timeout time.Duration) error {
	client := &http.Client{Timeout: 5 * time.Second}
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return err
		}

		resp, err := client.Do(req)
		if err == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}

	return fmt.Errorf("timed out waiting for %s", endpoint)
}

type appleContainerRequest struct {
	service       string
	image         string
	containerPort int
	memory        string
	env           map[string]string
	mountSource   string
	mountTarget   string
	command       []string
}

type appleContainer struct {
	binary string
	name   string
}

func startAppleContainer(ctx context.Context, config containerruntime.Config, request appleContainerRequest) (*appleContainer, int, error) {
	hostPort, err := availableLoopbackPort()
	if err != nil {
		return nil, 0, fmt.Errorf("reserve loopback port: %w", err)
	}

	name := fmt.Sprintf("ipam-it-%s-%d-%d", request.service, os.Getpid(), time.Now().UnixNano())
	args := []string{
		"run", "--detach", "--rm", "--progress", "none", "--name", name,
		"--publish", fmt.Sprintf("127.0.0.1:%d:%d", hostPort, request.containerPort),
	}
	if request.memory != "" {
		args = append(args, "--memory", request.memory)
	}
	keys := make([]string, 0, len(request.env))
	for key := range request.env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		args = append(args, "--env", key+"="+request.env[key])
	}
	if request.mountSource != "" {
		args = append(args, "--mount", fmt.Sprintf("type=bind,source=%s,target=%s,readonly", request.mountSource, request.mountTarget))
	}
	args = append(args, request.image)
	args = append(args, request.command...)

	container := &appleContainer{binary: config.AppleBinary, name: name}
	activeAppleContainers.Store(name, container)
	runCtx, runCancel := context.WithTimeout(ctx, config.StartupTimeout)
	defer runCancel()
	cmd := exec.CommandContext(runCtx, config.AppleBinary, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		cleanupErr := terminateAppleContainerAfterFailure(container)
		return nil, 0, fmt.Errorf("%s run failed for image %s: %w: %s%s", config.AppleBinary, request.image, err, strings.TrimSpace(string(output)), cleanupFailure(cleanupErr))
	}

	return container, hostPort, nil
}

func availableLoopbackPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func (c *appleContainer) Logs(ctx context.Context) string {
	output, err := exec.CommandContext(ctx, c.binary, "logs", c.name).CombinedOutput()
	if err != nil {
		return fmt.Sprintf("unable to read logs: %v: %s", err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output))
}

func (c *appleContainer) Terminate(ctx context.Context, _ ...testcontainers.TerminateOption) error {
	stopOutput, stopErr := exec.CommandContext(ctx, c.binary, "stop", "--time", "10", c.name).CombinedOutput()
	deleteOutput, deleteErr := exec.CommandContext(ctx, c.binary, "delete", "--force", c.name).CombinedOutput()
	if deleteErr == nil || isAppleContainerNotFound(deleteOutput) {
		activeAppleContainers.Delete(c.name)
		return nil
	}
	return fmt.Errorf("clean up Apple container %s: stop: %v: %s; delete: %v: %s", c.name, stopErr, strings.TrimSpace(string(stopOutput)), deleteErr, strings.TrimSpace(string(deleteOutput)))
}

func terminateActiveAppleContainers(ctx context.Context) error {
	var errs []error
	activeAppleContainers.Range(func(_, value any) bool {
		container, ok := value.(*appleContainer)
		if ok {
			if err := container.Terminate(ctx); err != nil {
				errs = append(errs, err)
			}
		}
		return true
	})
	return errors.Join(errs...)
}

func diagnoseAndTerminateAppleContainer(container *appleContainer) (string, error) {
	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cleanupCancel()
	logs := container.Logs(cleanupCtx)
	return logs, container.Terminate(cleanupCtx)
}

func terminateAppleContainerAfterFailure(container *appleContainer) error {
	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cleanupCancel()
	return container.Terminate(cleanupCtx)
}

func cleanupFailure(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("; cleanup failed: %v", err)
}

func isAppleContainerNotFound(output []byte) bool {
	message := strings.ToLower(string(output))
	return strings.Contains(message, "not found") || strings.Contains(message, "does not exist")
}

func (s *integrationSuite) mustToken(t *testing.T) string {
	t.Helper()

	form := url.Values{
		"grant_type": {"password"},
		"client_id":  {testClientID},
		"username":   {testUsername},
		"password":   {testPassword},
	}

	req, err := http.NewRequest(http.MethodPost, s.issuerURL+"/protocol/openid-connect/token", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("build token request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		t.Fatalf("fetch token: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		body := s.readBody(t, resp)
		t.Fatalf("expected 200 from token endpoint, got %d: %s", resp.StatusCode, body)
	}

	var token tokenResponse
	s.decodeJSON(t, resp, &token)
	if token.AccessToken == "" {
		t.Fatal("expected access token in token response")
	}

	return token.AccessToken
}

func (s *integrationSuite) get(t *testing.T, path string, token string) (*http.Response, error) {
	t.Helper()
	return s.request(t, http.MethodGet, path, token, nil)
}

func (s *integrationSuite) jsonRequest(t *testing.T, method string, path string, token string, payload any) (*http.Response, error) {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	return s.request(t, method, path, token, bytes.NewReader(body))
}

func (s *integrationSuite) csvRequest(t *testing.T, token string, content string) (*http.Response, error) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "inventory.csv")
	if err != nil {
		return nil, err
	}
	if _, err = io.WriteString(part, content); err != nil {
		return nil, err
	}
	if err = writer.Close(); err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, s.baseURL+"/api/v1/import/csv", &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return s.httpClient.Do(req)
}

func (s *integrationSuite) request(t *testing.T, method string, path string, token string, body io.Reader) (*http.Response, error) {
	t.Helper()

	req, err := http.NewRequest(method, s.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	return s.httpClient.Do(req)
}

func (s *integrationSuite) decodeJSON(t *testing.T, resp *http.Response, target any) {
	t.Helper()
	defer s.closeBody(t, resp)

	if ct := resp.Header.Get("Content-Type"); ct != "" && !strings.Contains(ct, "application/json") {
		t.Fatalf("expected json response, got %q", ct)
	}

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

func (s *integrationSuite) readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer s.closeBody(t, resp)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(body)
}

func (s *integrationSuite) closeBody(t *testing.T, resp *http.Response) {
	t.Helper()
	if resp == nil || resp.Body == nil {
		return
	}
	io.Copy(io.Discard, resp.Body)
	if err := resp.Body.Close(); err != nil {
		t.Fatalf("close body: %v", err)
	}
}

func (s *integrationSuite) closeBodyNoTest(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
}

func repoPath(parts ...string) (string, error) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("unable to resolve current file path")
	}

	allParts := append([]string{filepath.Dir(currentFile), "..", ".."}, parts...)
	return filepath.Clean(filepath.Join(allParts...)), nil
}

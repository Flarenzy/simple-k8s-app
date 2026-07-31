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
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	app "github.com/Flarenzy/simple-k8s-app/internal/app"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	postgresPort   = "5432/tcp"
	keycloakPort   = "8080/tcp"
	testRealm      = "ipam-integration"
	testClientID   = "ipam-test"
	testUsername   = "integration-user"
	testPassword   = "integration-password"
	testAudience   = "ipam-api"
	containerReady = 2 * time.Minute
	httpReady      = 30 * time.Second
)

type integrationSuite struct {
	httpClient *http.Client
	baseURL    string
	issuerURL  string

	postgres testcontainers.Container
	keycloak testcontainers.Container

	apiCancel context.CancelFunc
	apiErrCh  chan error
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
	ID       string `json:"id"`
	IP       string `json:"ip"`
	Hostname string `json:"hostname"`
	SubnetID int64  `json:"subnet_id"`
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
	suiteOnce   sync.Once
	suite       *integrationSuite
	suiteErr    error
	suiteClosed bool
)

func TestMain(m *testing.M) {
	code := m.Run()

	if suite != nil && !suiteClosed {
		closeCtx, closeCancel := context.WithTimeout(context.Background(), time.Minute)
		defer closeCancel()
		if err := suite.Close(closeCtx); err != nil {
			fmt.Printf("integration teardown failed: %v\n", err)
			if code == 0 {
				code = 1
			}
		}
		suiteClosed = true
	}

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
	if err := os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true"); err != nil {
		return nil, fmt.Errorf("disable testcontainers ryuk: %w", err)
	}

	s := &integrationSuite{
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}

	var err error
	s.postgres, err = startPostgres(ctx)
	if err != nil {
		return nil, err
	}

	dsn, err := buildPostgresDSN(ctx, s.postgres)
	if err != nil {
		_ = s.postgres.Terminate(ctx)
		return nil, err
	}

	if err = runGooseMigrations(ctx, dsn); err != nil {
		_ = s.postgres.Terminate(ctx)
		return nil, err
	}

	s.keycloak, s.issuerURL, err = startKeycloak(ctx)
	if err != nil {
		_ = s.postgres.Terminate(ctx)
		return nil, err
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

	return errors.Join(errs...)
}

func startPostgres(ctx context.Context) (testcontainers.Container, error) {
	req := testcontainers.ContainerRequest{
		Image:        "postgres:16",
		ExposedPorts: []string{postgresPort},
		Env: map[string]string{
			"POSTGRES_DB":       "ipam",
			"POSTGRES_USER":     "ipam",
			"POSTGRES_PASSWORD": "ipam",
		},
		WaitingFor: wait.ForListeningPort(postgresPort).WithStartupTimeout(containerReady),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("start postgres container: %w", err)
	}

	return container, nil
}

func buildPostgresDSN(ctx context.Context, container testcontainers.Container) (string, error) {
	host, err := container.Host(ctx)
	if err != nil {
		return "", fmt.Errorf("postgres host: %w", err)
	}
	port, err := container.MappedPort(ctx, postgresPort)
	if err != nil {
		return "", fmt.Errorf("postgres mapped port: %w", err)
	}

	return fmt.Sprintf("postgres://ipam:ipam@%s:%s/ipam?sslmode=disable", host, port.Port()), nil
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

func startKeycloak(ctx context.Context) (testcontainers.Container, string, error) {
	realmPath, err := repoPath("integration", "api", "testdata", "ipam-integration-realm.json")
	if err != nil {
		return nil, "", fmt.Errorf("resolve realm fixture: %w", err)
	}

	req := testcontainers.ContainerRequest{
		Image:        "quay.io/keycloak/keycloak:24.0.5",
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
		WaitingFor: wait.ForListeningPort(keycloakPort).WithStartupTimeout(containerReady),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
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
	if err = waitForHTTP200(ctx, issuerURL+"/.well-known/openid-configuration"); err != nil {
		_ = container.Terminate(ctx)
		return nil, "", err
	}

	return container, issuerURL, nil
}

func waitForHTTP200(ctx context.Context, endpoint string) error {
	client := &http.Client{Timeout: 5 * time.Second}
	deadline := time.Now().Add(httpReady)

	for time.Now().Before(deadline) {
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

		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("timed out waiting for %s", endpoint)
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

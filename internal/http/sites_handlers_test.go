package http

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Flarenzy/simple-k8s-app/internal/domain"
	"github.com/google/uuid"
)

type siteServiceStub struct {
	sites       []domain.Site
	statistics  []domain.SiteStatistics
	deleted     bool
	updated     domain.Site
	created     domain.Site
	find        domain.Site
	createInput domain.CreateSiteInput
	updateInput domain.UpdateSiteInput
	createCalls int
	updateCalls int
	err         error
	deleteCalls []uuid.UUID
}

func (s *siteServiceStub) List(context.Context) ([]domain.Site, error) { return s.sites, s.err }

func (s *siteServiceStub) FindByID(context.Context, uuid.UUID) (domain.Site, error) {
	return s.find, s.err
}

func (s *siteServiceStub) Create(_ context.Context, input domain.CreateSiteInput) (domain.Site, error) {
	s.createCalls++
	s.createInput = input
	return s.created, s.err
}

func (s *siteServiceStub) Update(_ context.Context, input domain.UpdateSiteInput) (domain.Site, error) {
	s.updateCalls++
	s.updateInput = input
	return s.updated, s.err
}

func (s *siteServiceStub) Delete(_ context.Context, id uuid.UUID) (bool, error) {
	s.deleteCalls = append(s.deleteCalls, id)
	return s.deleted, s.err
}

func (s *siteServiceStub) Statistics(context.Context) ([]domain.SiteStatistics, error) {
	return s.statistics, s.err
}

func newSiteHandlerTestAPI(service domain.SitesService) *API {
	return NewAPI(slog.New(slog.NewTextHandler(io.Discard, nil)), stubHealthChecker{}, nil, service, nil)
}

func TestSiteRoutesSupportCRUDAndStatistics(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	service := &siteServiceStub{
		sites:      []domain.Site{{ID: id, Name: "Belgrade", Description: "HQ", CreatedAt: now, UpdatedAt: now}},
		find:       domain.Site{ID: id, Name: "Belgrade", Description: "HQ", CreatedAt: now, UpdatedAt: now},
		created:    domain.Site{ID: id, Name: "Belgrade", Description: "HQ", CreatedAt: now, UpdatedAt: now},
		updated:    domain.Site{ID: id, Name: "Belgrade", Description: "Updated", CreatedAt: now, UpdatedAt: now},
		deleted:    true,
		statistics: []domain.SiteStatistics{{ID: id, Name: "Belgrade", Description: "HQ", CreatedAt: now, UpdatedAt: now, SubnetCount: 2, UsedIPCount: 3, TotalIPCount: 503, FreeIPCount: 500}},
	}
	api := newSiteHandlerTestAPI(service)

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		status     int
		decodeInto any
	}{
		{name: "list", method: http.MethodGet, path: "/api/v1/sites", status: http.StatusOK, decodeInto: new([]SiteResponse)},
		{name: "create", method: http.MethodPost, path: "/api/v1/sites", body: `{"name":" Belgrade ","description":"HQ"}`, status: http.StatusCreated, decodeInto: new(SiteResponse)},
		{name: "statistics", method: http.MethodGet, path: "/api/v1/sites/statistics", status: http.StatusOK, decodeInto: new([]SiteStatisticsResponse)},
		{name: "find", method: http.MethodGet, path: "/api/v1/sites/11111111-1111-1111-1111-111111111111", status: http.StatusOK, decodeInto: new(SiteResponse)},
		{name: "update", method: http.MethodPatch, path: "/api/v1/sites/11111111-1111-1111-1111-111111111111", body: `{"name":" Belgrade ","description":"Updated"}`, status: http.StatusOK, decodeInto: new(SiteResponse)},
		{name: "delete", method: http.MethodDelete, path: "/api/v1/sites/11111111-1111-1111-1111-111111111111", status: http.StatusNoContent},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			rec := httptest.NewRecorder()
			api.Router().ServeHTTP(rec, req)
			if rec.Code != test.status {
				t.Fatalf("expected %d, got %d: %s", test.status, rec.Code, rec.Body.String())
			}
			if test.decodeInto != nil && json.NewDecoder(rec.Body).Decode(test.decodeInto) != nil {
				t.Fatal("expected valid JSON response")
			}
		})
	}

	if len(service.deleteCalls) != 1 || service.deleteCalls[0] != id {
		t.Fatalf("expected delete for %s, got %v", id, service.deleteCalls)
	}
	if service.createInput != (domain.CreateSiteInput{Name: "Belgrade", Description: "HQ"}) {
		t.Fatalf("unexpected create input: %+v", service.createInput)
	}
	if service.updateInput != (domain.UpdateSiteInput{ID: id, Name: "Belgrade", Description: "Updated"}) {
		t.Fatalf("unexpected update input: %+v", service.updateInput)
	}
}

func TestSiteRoutesRejectInvalidIDAndMalformedBody(t *testing.T) {
	api := newSiteHandlerTestAPI(&siteServiceStub{})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/sites/not-a-uuid", nil)
	recorder := httptest.NewRecorder()
	api.Router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid id, got %d", recorder.Code)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/v1/sites", strings.NewReader("{"))
	recorder = httptest.NewRecorder()
	api.Router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for malformed body, got %d", recorder.Code)
	}
}

func TestSiteRoutesRejectBlankNames(t *testing.T) {
	service := &siteServiceStub{}
	api := newSiteHandlerTestAPI(service)
	id := "11111111-1111-1111-1111-111111111111"

	for _, test := range []struct {
		method string
		path   string
	}{
		{method: http.MethodPost, path: "/api/v1/sites"},
		{method: http.MethodPatch, path: "/api/v1/sites/" + id},
	} {
		t.Run(test.method, func(t *testing.T) {
			req := httptest.NewRequest(test.method, test.path, strings.NewReader(`{"name":" \t ","description":"HQ"}`))
			rec := httptest.NewRecorder()
			api.Router().ServeHTTP(rec, req)
			assertJSONError(t, rec, http.StatusBadRequest, "bad request")
		})
	}

	if service.createCalls != 0 || service.updateCalls != 0 {
		t.Fatalf("expected blank-name requests to stop before service calls: create=%d update=%d", service.createCalls, service.updateCalls)
	}
}

func TestSiteRoutesMapServiceErrorsToAPIContract(t *testing.T) {
	tests := []struct {
		name       string
		serviceErr error
		method     string
		path       string
		wantStatus int
		wantError  string
	}{
		{name: "list", method: http.MethodGet, path: "/api/v1/sites", serviceErr: context.Canceled, wantStatus: http.StatusInternalServerError, wantError: "internal server error"},
		{name: "find not found", method: http.MethodGet, path: "/api/v1/sites/11111111-1111-1111-1111-111111111111", serviceErr: domain.ErrNotFound, wantStatus: http.StatusNotFound, wantError: "site not found"},
		{name: "update not found", method: http.MethodPatch, path: "/api/v1/sites/11111111-1111-1111-1111-111111111111", serviceErr: domain.ErrNotFound, wantStatus: http.StatusNotFound, wantError: "site not found"},
		{name: "delete not found", method: http.MethodDelete, path: "/api/v1/sites/11111111-1111-1111-1111-111111111111", wantStatus: http.StatusNotFound, wantError: "site not found"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &siteServiceStub{err: test.serviceErr}
			if test.name == "delete not found" {
				service.deleted = false
			}
			api := newSiteHandlerTestAPI(service)
			body := strings.NewReader(`{"name":"Belgrade","description":"HQ"}`)
			req := httptest.NewRequest(test.method, test.path, body)
			rec := httptest.NewRecorder()
			api.Router().ServeHTTP(rec, req)

			assertJSONError(t, rec, test.wantStatus, test.wantError)
		})
	}
}

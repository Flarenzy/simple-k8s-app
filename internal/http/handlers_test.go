package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/Flarenzy/simple-k8s-app/internal/domain"
)

type stubHealthChecker struct {
	err error
}

func (s stubHealthChecker) Ping(context.Context) error {
	return s.err
}

type stubService struct {
	listSubnetsFn      func(context.Context) ([]domain.Subnet, error)
	createSubnetFn     func(context.Context, domain.CreateSubnetInput) (domain.Subnet, error)
	getSubnetFn        func(context.Context, int64) (domain.Subnet, error)
	deleteSubnetFn     func(context.Context, int64) error
	listIPsFn          func(context.Context, int64) ([]domain.IPAddress, error)
	createIPFn         func(context.Context, int64, domain.CreateIPInput) (domain.IPAddress, error)
	updateIPHostnameFn func(context.Context, int64, domain.IPAddressID, domain.UpdateIPInput) (domain.IPAddress, error)
	deleteIPFn         func(context.Context, int64, domain.IPAddressID) error
}

func (s stubService) ListSubnets(ctx context.Context) ([]domain.Subnet, error) {
	if s.listSubnetsFn == nil {
		return nil, nil
	}
	return s.listSubnetsFn(ctx)
}

func (s stubService) CreateSubnet(ctx context.Context, input domain.CreateSubnetInput) (domain.Subnet, error) {
	if s.createSubnetFn == nil {
		return domain.Subnet{}, nil
	}
	return s.createSubnetFn(ctx, input)
}

func (s stubService) GetSubnet(ctx context.Context, id int64) (domain.Subnet, error) {
	if s.getSubnetFn == nil {
		return domain.Subnet{}, nil
	}
	return s.getSubnetFn(ctx, id)
}

func (s stubService) DeleteSubnet(ctx context.Context, id int64) error {
	if s.deleteSubnetFn == nil {
		return nil
	}
	return s.deleteSubnetFn(ctx, id)
}

func (s stubService) ListIPs(ctx context.Context, subnetID int64) ([]domain.IPAddress, error) {
	if s.listIPsFn == nil {
		return nil, nil
	}
	return s.listIPsFn(ctx, subnetID)
}

func (s stubService) CreateIP(ctx context.Context, subnetID int64, input domain.CreateIPInput) (domain.IPAddress, error) {
	if s.createIPFn == nil {
		return domain.IPAddress{}, nil
	}
	return s.createIPFn(ctx, subnetID, input)
}

func (s stubService) UpdateIPHostname(ctx context.Context, subnetID int64, id domain.IPAddressID, input domain.UpdateIPInput) (domain.IPAddress, error) {
	if s.updateIPHostnameFn == nil {
		return domain.IPAddress{}, nil
	}
	return s.updateIPHostnameFn(ctx, subnetID, id, input)
}

func (s stubService) DeleteIP(ctx context.Context, subnetID int64, id domain.IPAddressID) error {
	if s.deleteIPFn == nil {
		return nil
	}
	return s.deleteIPFn(ctx, subnetID, id)
}

func newHandlerTestAPI(service domain.NetworkService, healthErr error) *API {
	return NewAPI(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		stubHealthChecker{err: healthErr},
		service,
		nil,
	)
}

func mustPrefix(t *testing.T, cidr string) netip.Prefix {
	t.Helper()

	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		t.Fatalf("parse prefix %q: %v", cidr, err)
	}

	return prefix
}

func mustAddr(t *testing.T, addr string) netip.Addr {
	t.Helper()

	ip, err := netip.ParseAddr(addr)
	if err != nil {
		t.Fatalf("parse addr %q: %v", addr, err)
	}

	return ip
}

func assertJSONError(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantErr string) {
	t.Helper()

	if rec.Code != wantStatus {
		t.Fatalf("expected %d, got %d", wantStatus, rec.Code)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}

	if resp.Error != wantErr {
		t.Fatalf("expected error %q, got %q", wantErr, resp.Error)
	}
}

func TestGetAllSubnetsReturnsJSONPayload(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	api := newHandlerTestAPI(stubService{
		listSubnetsFn: func(context.Context) ([]domain.Subnet, error) {
			return []domain.Subnet{
				{
					ID:          7,
					CIDR:        mustPrefix(t, "10.0.0.0/24"),
					Description: "office",
					CreatedAt:   now,
					UpdatedAt:   now,
				},
			}, nil
		},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/subnets", nil)
	rec := httptest.NewRecorder()
	api.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}

	var resp []SubnetResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode subnet list: %v", err)
	}

	if len(resp) != 1 {
		t.Fatalf("expected 1 subnet, got %d", len(resp))
	}
	if resp[0].ID != 7 || resp[0].CIDR != "10.0.0.0/24" || resp[0].Description != "office" {
		t.Fatalf("unexpected subnet payload: %+v", resp[0])
	}
}

func TestGetAllSubnetsReturnsInternalServerError(t *testing.T) {
	api := newHandlerTestAPI(stubService{
		listSubnetsFn: func(context.Context) ([]domain.Subnet, error) {
			return nil, errors.New("boom")
		},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/subnets", nil)
	rec := httptest.NewRecorder()
	api.Router().ServeHTTP(rec, req)

	assertJSONError(t, rec, http.StatusInternalServerError, "internal server error")
}

func TestReadyzReturnsServiceUnavailableWhenHealthCheckFails(t *testing.T) {
	api := newHandlerTestAPI(stubService{}, context.Canceled)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	api.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}
}

func TestGetSubnetByIDReturnsNotFound(t *testing.T) {
	api := newHandlerTestAPI(stubService{
		getSubnetFn: func(context.Context, int64) (domain.Subnet, error) {
			return domain.Subnet{}, domain.ErrNotFound
		},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/subnets/42", nil)
	rec := httptest.NewRecorder()
	api.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected %d, got %d", http.StatusNotFound, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "subnet not found") {
		t.Fatalf("expected subnet not found body, got %q", rec.Body.String())
	}
}

func TestGetSubnetByIDCoversRequestAndErrorMapping(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		serviceErr error
		wantStatus int
		wantErr    string
	}{
		{
			name:       "invalid subnet id",
			target:     "/api/v1/subnets/nope",
			wantStatus: http.StatusBadRequest,
			wantErr:    "bad request",
		},
		{
			name:       "internal error",
			target:     "/api/v1/subnets/42",
			serviceErr: errors.New("boom"),
			wantStatus: http.StatusInternalServerError,
			wantErr:    "internal server error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			api := newHandlerTestAPI(stubService{
				getSubnetFn: func(context.Context, int64) (domain.Subnet, error) {
					return domain.Subnet{}, tc.serviceErr
				},
			}, nil)

			req := httptest.NewRequest(http.MethodGet, tc.target, nil)
			rec := httptest.NewRecorder()
			api.Router().ServeHTTP(rec, req)

			assertJSONError(t, rec, tc.wantStatus, tc.wantErr)
		})
	}
}

func TestGetSubnetByIDReturnsJSONPayload(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	api := newHandlerTestAPI(stubService{
		getSubnetFn: func(context.Context, int64) (domain.Subnet, error) {
			return domain.Subnet{
				ID:          42,
				CIDR:        mustPrefix(t, "10.0.0.0/24"),
				Description: "office",
				CreatedAt:   now,
				UpdatedAt:   now,
			}, nil
		},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/subnets/42", nil)
	rec := httptest.NewRecorder()
	api.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}

	var resp SubnetResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode subnet response: %v", err)
	}

	if resp.ID != 42 || resp.CIDR != "10.0.0.0/24" || resp.Description != "office" {
		t.Fatalf("unexpected subnet payload: %+v", resp)
	}
}

func TestCreateSubnetReturnsBadRequestOnInvalidInput(t *testing.T) {
	api := newHandlerTestAPI(stubService{
		createSubnetFn: func(context.Context, domain.CreateSubnetInput) (domain.Subnet, error) {
			return domain.Subnet{}, domain.ErrInvalidInput
		},
	}, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/subnets", strings.NewReader(`{"cidr":"bad"}`))
	rec := httptest.NewRecorder()
	api.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "invalid cidr") {
		t.Fatalf("expected invalid cidr body, got %q", rec.Body.String())
	}
}

func TestCreateSubnetCoversRequestAndErrorMapping(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		serviceErr error
		wantStatus int
		wantErr    string
	}{
		{
			name:       "bad json",
			body:       `{"cidr":`,
			wantStatus: http.StatusBadRequest,
			wantErr:    "bad request",
		},
		{
			name:       "internal error",
			body:       `{"cidr":"10.0.0.0/24","description":"office"}`,
			serviceErr: errors.New("boom"),
			wantStatus: http.StatusInternalServerError,
			wantErr:    "internal server error while saving subnet to db",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			api := newHandlerTestAPI(stubService{
				createSubnetFn: func(context.Context, domain.CreateSubnetInput) (domain.Subnet, error) {
					return domain.Subnet{}, tc.serviceErr
				},
			}, nil)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/subnets", strings.NewReader(tc.body))
			rec := httptest.NewRecorder()
			api.Router().ServeHTTP(rec, req)

			assertJSONError(t, rec, tc.wantStatus, tc.wantErr)
		})
	}
}

func TestCreateSubnetReturnsCreatedPayload(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	api := newHandlerTestAPI(stubService{
		createSubnetFn: func(_ context.Context, input domain.CreateSubnetInput) (domain.Subnet, error) {
			return domain.Subnet{
				ID:          42,
				CIDR:        mustPrefix(t, input.CIDR),
				Description: input.Description,
				CreatedAt:   now,
				UpdatedAt:   now,
			}, nil
		},
	}, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/subnets", strings.NewReader(`{"cidr":"10.0.0.0/24","description":"office"}`))
	rec := httptest.NewRecorder()
	api.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected %d, got %d", http.StatusCreated, rec.Code)
	}

	var resp SubnetResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode create subnet response: %v", err)
	}

	if resp.ID != 42 || resp.CIDR != "10.0.0.0/24" || resp.Description != "office" {
		t.Fatalf("unexpected create subnet payload: %+v", resp)
	}
}

func TestCreateIPReturnsConflictAsBadRequest(t *testing.T) {
	api := newHandlerTestAPI(stubService{
		createIPFn: func(context.Context, int64, domain.CreateIPInput) (domain.IPAddress, error) {
			return domain.IPAddress{}, domain.ErrConflict
		},
	}, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/subnets/42/ips", strings.NewReader(`{"ip":"10.0.0.10","hostname":"h"}`))
	rec := httptest.NewRecorder()
	api.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "ip exists") {
		t.Fatalf("expected ip exists body, got %q", rec.Body.String())
	}
}

func TestUpdateIPReturnsSubnetSpecificNotFound(t *testing.T) {
	api := newHandlerTestAPI(stubService{
		updateIPHostnameFn: func(context.Context, int64, domain.IPAddressID, domain.UpdateIPInput) (domain.IPAddress, error) {
			return domain.IPAddress{}, domain.ErrSubnetNotFound
		},
	}, nil)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/subnets/42/ips/550e8400-e29b-41d4-a716-446655440000", strings.NewReader(`{"hostname":"new-host"}`))
	rec := httptest.NewRecorder()
	api.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected %d, got %d", http.StatusNotFound, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "subnet not found") {
		t.Fatalf("expected subnet not found body, got %q", rec.Body.String())
	}
}

func TestGetIPsBySubnetIDCoversRequestAndErrorMapping(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		serviceErr error
		wantStatus int
		wantErr    string
	}{
		{
			name:       "invalid subnet id",
			target:     "/api/v1/subnets/nope/ips",
			wantStatus: http.StatusBadRequest,
			wantErr:    "bad request",
		},
		{
			name:       "subnet not found",
			target:     "/api/v1/subnets/42/ips",
			serviceErr: domain.ErrNotFound,
			wantStatus: http.StatusNotFound,
			wantErr:    "subnet not found",
		},
		{
			name:       "internal error",
			target:     "/api/v1/subnets/42/ips",
			serviceErr: errors.New("boom"),
			wantStatus: http.StatusInternalServerError,
			wantErr:    "internal server error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			api := newHandlerTestAPI(stubService{
				listIPsFn: func(context.Context, int64) ([]domain.IPAddress, error) {
					return nil, tc.serviceErr
				},
			}, nil)

			req := httptest.NewRequest(http.MethodGet, tc.target, nil)
			rec := httptest.NewRecorder()
			api.Router().ServeHTTP(rec, req)

			assertJSONError(t, rec, tc.wantStatus, tc.wantErr)
		})
	}
}

func TestGetIPsBySubnetIDReturnsJSONPayload(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	api := newHandlerTestAPI(stubService{
		listIPsFn: func(context.Context, int64) ([]domain.IPAddress, error) {
			return []domain.IPAddress{
				{
					ID:        domain.IPAddressID("550e8400-e29b-41d4-a716-446655440000"),
					IP:        mustAddr(t, "10.0.0.10"),
					Hostname:  "printer-1",
					SubnetID:  42,
					CreatedAt: now,
					UpdatedAt: now,
				},
			}, nil
		},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/subnets/42/ips", nil)
	rec := httptest.NewRecorder()
	api.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}

	var resp []IPResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode ip list: %v", err)
	}

	if len(resp) != 1 {
		t.Fatalf("expected 1 ip, got %d", len(resp))
	}
	if resp[0].IP != "10.0.0.10" || resp[0].Hostname != "printer-1" || resp[0].SubnetID != 42 {
		t.Fatalf("unexpected ip payload: %+v", resp[0])
	}
}

func TestCreateIPBySubnetIDCoversRequestAndErrorMapping(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		body       string
		serviceErr error
		wantStatus int
		wantErr    string
	}{
		{
			name:       "invalid subnet id",
			target:     "/api/v1/subnets/nope/ips",
			body:       `{"ip":"10.0.0.10","hostname":"h"}`,
			wantStatus: http.StatusBadRequest,
			wantErr:    "bad request",
		},
		{
			name:       "bad json",
			target:     "/api/v1/subnets/42/ips",
			body:       `{"ip":`,
			wantStatus: http.StatusBadRequest,
			wantErr:    "bad request",
		},
		{
			name:       "subnet not found",
			target:     "/api/v1/subnets/42/ips",
			body:       `{"ip":"10.0.0.10","hostname":"h"}`,
			serviceErr: domain.ErrNotFound,
			wantStatus: http.StatusNotFound,
			wantErr:    "subnet not found",
		},
		{
			name:       "invalid input",
			target:     "/api/v1/subnets/42/ips",
			body:       `{"ip":"10.0.0.10","hostname":"h"}`,
			serviceErr: domain.ErrInvalidInput,
			wantStatus: http.StatusBadRequest,
			wantErr:    "bad request",
		},
		{
			name:       "internal error",
			target:     "/api/v1/subnets/42/ips",
			body:       `{"ip":"10.0.0.10","hostname":"h"}`,
			serviceErr: errors.New("boom"),
			wantStatus: http.StatusInternalServerError,
			wantErr:    "internal server error while creating ip",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			api := newHandlerTestAPI(stubService{
				createIPFn: func(context.Context, int64, domain.CreateIPInput) (domain.IPAddress, error) {
					return domain.IPAddress{}, tc.serviceErr
				},
			}, nil)

			req := httptest.NewRequest(http.MethodPost, tc.target, strings.NewReader(tc.body))
			rec := httptest.NewRecorder()
			api.Router().ServeHTTP(rec, req)

			assertJSONError(t, rec, tc.wantStatus, tc.wantErr)
		})
	}
}

func TestCreateIPBySubnetIDReturnsCreatedPayload(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	api := newHandlerTestAPI(stubService{
		createIPFn: func(_ context.Context, subnetID int64, input domain.CreateIPInput) (domain.IPAddress, error) {
			return domain.IPAddress{
				ID:        domain.IPAddressID("550e8400-e29b-41d4-a716-446655440000"),
				IP:        mustAddr(t, input.IP),
				Hostname:  input.Hostname,
				SubnetID:  subnetID,
				CreatedAt: now,
				UpdatedAt: now,
			}, nil
		},
	}, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/subnets/42/ips", strings.NewReader(`{"ip":"10.0.0.10","hostname":"h"}`))
	rec := httptest.NewRecorder()
	api.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected %d, got %d", http.StatusCreated, rec.Code)
	}

	var resp IPResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode create ip response: %v", err)
	}

	if resp.IP != "10.0.0.10" || resp.Hostname != "h" || resp.SubnetID != 42 {
		t.Fatalf("unexpected create payload: %+v", resp)
	}
}

func TestUpdateIPByUUIDCoversRequestAndErrorMapping(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		body       string
		serviceErr error
		wantStatus int
		wantErr    string
	}{
		{
			name:       "invalid subnet id",
			target:     "/api/v1/subnets/nope/ips/550e8400-e29b-41d4-a716-446655440000",
			body:       `{"hostname":"h"}`,
			wantStatus: http.StatusBadRequest,
			wantErr:    "bad request",
		},
		{
			name:       "malformed uuid",
			target:     "/api/v1/subnets/42/ips/not-a-uuid",
			body:       `{"hostname":"h"}`,
			wantStatus: http.StatusBadRequest,
			wantErr:    "bad request",
		},
		{
			name:       "bad json",
			target:     "/api/v1/subnets/42/ips/550e8400-e29b-41d4-a716-446655440000",
			body:       `{"hostname":`,
			wantStatus: http.StatusBadRequest,
			wantErr:    "bad request",
		},
		{
			name:       "ip not found",
			target:     "/api/v1/subnets/42/ips/550e8400-e29b-41d4-a716-446655440000",
			body:       `{"hostname":"h"}`,
			serviceErr: domain.ErrNotFound,
			wantStatus: http.StatusNotFound,
			wantErr:    "ip not found",
		},
		{
			name:       "invalid input",
			target:     "/api/v1/subnets/42/ips/550e8400-e29b-41d4-a716-446655440000",
			body:       `{"hostname":"h"}`,
			serviceErr: domain.ErrInvalidInput,
			wantStatus: http.StatusBadRequest,
			wantErr:    "bad request",
		},
		{
			name:       "internal error",
			target:     "/api/v1/subnets/42/ips/550e8400-e29b-41d4-a716-446655440000",
			body:       `{"hostname":"h"}`,
			serviceErr: errors.New("boom"),
			wantStatus: http.StatusInternalServerError,
			wantErr:    "internal server error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			api := newHandlerTestAPI(stubService{
				updateIPHostnameFn: func(context.Context, int64, domain.IPAddressID, domain.UpdateIPInput) (domain.IPAddress, error) {
					return domain.IPAddress{}, tc.serviceErr
				},
			}, nil)

			req := httptest.NewRequest(http.MethodPatch, tc.target, strings.NewReader(tc.body))
			rec := httptest.NewRecorder()
			api.Router().ServeHTTP(rec, req)

			assertJSONError(t, rec, tc.wantStatus, tc.wantErr)
		})
	}
}

func TestUpdateIPByUUIDReturnsUpdatedPayload(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	api := newHandlerTestAPI(stubService{
		updateIPHostnameFn: func(_ context.Context, subnetID int64, id domain.IPAddressID, input domain.UpdateIPInput) (domain.IPAddress, error) {
			return domain.IPAddress{
				ID:        id,
				IP:        mustAddr(t, "10.0.0.10"),
				Hostname:  input.Hostname,
				SubnetID:  subnetID,
				CreatedAt: now,
				UpdatedAt: now,
			}, nil
		},
	}, nil)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/subnets/42/ips/550e8400-e29b-41d4-a716-446655440000", strings.NewReader(`{"hostname":"new-host"}`))
	rec := httptest.NewRecorder()
	api.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}

	var resp IPResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode update response: %v", err)
	}

	if resp.ID != "550e8400-e29b-41d4-a716-446655440000" || resp.Hostname != "new-host" {
		t.Fatalf("unexpected update payload: %+v", resp)
	}
}

func TestDeleteIPByUUIDAndSubnetIDCoversRequestAndErrorMapping(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		serviceErr error
		wantStatus int
		wantErr    string
	}{
		{
			name:       "invalid subnet id",
			target:     "/api/v1/subnets/nope/ips/550e8400-e29b-41d4-a716-446655440000",
			wantStatus: http.StatusBadRequest,
			wantErr:    "bad request",
		},
		{
			name:       "malformed uuid",
			target:     "/api/v1/subnets/42/ips/not-a-uuid",
			wantStatus: http.StatusBadRequest,
			wantErr:    "bad request",
		},
		{
			name:       "not found",
			target:     "/api/v1/subnets/42/ips/550e8400-e29b-41d4-a716-446655440000",
			serviceErr: domain.ErrNotFound,
			wantStatus: http.StatusNotFound,
			wantErr:    "subnet or ip not found",
		},
		{
			name:       "invalid input",
			target:     "/api/v1/subnets/42/ips/550e8400-e29b-41d4-a716-446655440000",
			serviceErr: domain.ErrInvalidInput,
			wantStatus: http.StatusBadRequest,
			wantErr:    "bad request",
		},
		{
			name:       "internal error",
			target:     "/api/v1/subnets/42/ips/550e8400-e29b-41d4-a716-446655440000",
			serviceErr: errors.New("boom"),
			wantStatus: http.StatusInternalServerError,
			wantErr:    "internal server error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			api := newHandlerTestAPI(stubService{
				deleteIPFn: func(context.Context, int64, domain.IPAddressID) error {
					return tc.serviceErr
				},
			}, nil)

			req := httptest.NewRequest(http.MethodDelete, tc.target, nil)
			rec := httptest.NewRecorder()
			api.Router().ServeHTTP(rec, req)

			assertJSONError(t, rec, tc.wantStatus, tc.wantErr)
		})
	}
}

func TestDeleteIPByUUIDAndSubnetIDReturnsNoContent(t *testing.T) {
	api := newHandlerTestAPI(stubService{
		deleteIPFn: func(context.Context, int64, domain.IPAddressID) error {
			return nil
		},
	}, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/subnets/42/ips/550e8400-e29b-41d4-a716-446655440000", nil)
	rec := httptest.NewRecorder()
	api.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected %d, got %d", http.StatusNoContent, rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty body, got %q", rec.Body.String())
	}
}

func TestDeleteSubnetByIDCoversRequestAndErrorMapping(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		serviceErr error
		wantStatus int
		wantErr    string
	}{
		{
			name:       "invalid subnet id",
			target:     "/api/v1/subnets/nope",
			wantStatus: http.StatusBadRequest,
			wantErr:    "bad request",
		},
		{
			name:       "not found",
			target:     "/api/v1/subnets/42",
			serviceErr: domain.ErrNotFound,
			wantStatus: http.StatusNotFound,
			wantErr:    "subnet not found",
		},
		{
			name:       "internal error",
			target:     "/api/v1/subnets/42",
			serviceErr: errors.New("boom"),
			wantStatus: http.StatusInternalServerError,
			wantErr:    "internal server error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			api := newHandlerTestAPI(stubService{
				deleteSubnetFn: func(context.Context, int64) error {
					return tc.serviceErr
				},
			}, nil)

			req := httptest.NewRequest(http.MethodDelete, tc.target, nil)
			rec := httptest.NewRecorder()
			api.Router().ServeHTTP(rec, req)

			assertJSONError(t, rec, tc.wantStatus, tc.wantErr)
		})
	}
}

func TestDeleteSubnetByIDReturnsNoContent(t *testing.T) {
	api := newHandlerTestAPI(stubService{
		deleteSubnetFn: func(context.Context, int64) error {
			return nil
		},
	}, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/subnets/42", nil)
	rec := httptest.NewRecorder()
	api.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected %d, got %d", http.StatusNoContent, rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty body, got %q", rec.Body.String())
	}
}

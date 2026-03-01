package http

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

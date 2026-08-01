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
)

type stubReportingService struct {
	getSettingsFn func(context.Context) (domain.ReportingSettings, error)
	updateFn      func(context.Context, domain.UpdateReportingSettingsInput) (domain.ReportingSettings, error)
	historyFn     func(context.Context, int64, string) (domain.SubnetUsageHistory, error)
}

func (s stubReportingService) GetSettings(ctx context.Context) (domain.ReportingSettings, error) {
	return s.getSettingsFn(ctx)
}

func (s stubReportingService) UpdateSettings(ctx context.Context, input domain.UpdateReportingSettingsInput) (domain.ReportingSettings, error) {
	return s.updateFn(ctx, input)
}

func (s stubReportingService) GetSubnetUsageHistory(ctx context.Context, subnetID int64, usageRange string) (domain.SubnetUsageHistory, error) {
	return s.historyFn(ctx, subnetID, usageRange)
}

func (s stubReportingService) RunSnapshotCycle(context.Context) (domain.SnapshotCycleResult, error) {
	return domain.SnapshotCycleResult{}, nil
}

func newReportingTestAPI(service domain.ReportingService) *API {
	api := NewAPI(slog.New(slog.NewTextHandler(io.Discard, nil)), stubHealthChecker{}, nil, nil, nil)
	api.ReportingService = service
	return api
}

func TestGetReportingSettingsReturnsDefaults(t *testing.T) {
	api := newReportingTestAPI(stubReportingService{getSettingsFn: func(context.Context) (domain.ReportingSettings, error) {
		return domain.ReportingSettings{Cadence: domain.ReportingCadenceHourly, RetentionDays: 30}, nil
	}})
	recorder := httptest.NewRecorder()
	api.Router().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/reporting/settings", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response ReportingSettingsResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Cadence != "hourly" || response.RetentionDays != 30 {
		t.Fatalf("unexpected settings: %+v", response)
	}
}

func TestUpdateReportingSettingsReturnsValidationError(t *testing.T) {
	api := newReportingTestAPI(stubReportingService{updateFn: func(_ context.Context, input domain.UpdateReportingSettingsInput) (domain.ReportingSettings, error) {
		if input.RetentionDays != 181 {
			t.Fatalf("unexpected input: %+v", input)
		}
		return domain.ReportingSettings{}, domain.ErrInvalidInput
	}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPatch, "/api/v1/reporting/settings", strings.NewReader(`{"cadence":"hourly","retention_days":181}`))
	api.Router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
}

func TestGetSubnetUsageHistoryDefaultsToSevenDaysAndReturnsEmptyPoints(t *testing.T) {
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	api := newReportingTestAPI(stubReportingService{historyFn: func(_ context.Context, subnetID int64, usageRange string) (domain.SubnetUsageHistory, error) {
		if subnetID != 42 || usageRange != "7d" {
			t.Fatalf("unexpected request: subnet=%d range=%s", subnetID, usageRange)
		}
		return domain.SubnetUsageHistory{SubnetID: subnetID, Range: usageRange, From: now.Add(-7 * 24 * time.Hour), To: now, Cadence: domain.ReportingCadenceHourly}, nil
	}})
	recorder := httptest.NewRecorder()
	api.Router().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/subnets/42/usage-history", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response SubnetUsageHistoryResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Range != "7d" || response.Points == nil || len(response.Points) != 0 {
		t.Fatalf("unexpected history response: %+v", response)
	}
}

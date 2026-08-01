package domain

import (
	"context"
	"errors"
	"net/netip"
	"testing"
	"time"
)

type stubReportingRepository struct {
	settings        ReportingSettings
	updateFn        func(context.Context, UpdateReportingSettingsInput) (ReportingSettings, error)
	listSnapshotsFn func(context.Context, int64, time.Time, time.Time) ([]SubnetUsageSnapshot, error)
	captureDueFn    func(context.Context) (int64, error)
	deleteExpiredFn func(context.Context) (int64, error)
}

func (s stubReportingRepository) GetSettings(context.Context) (ReportingSettings, error) {
	return s.settings, nil
}

func (s stubReportingRepository) UpdateSettings(ctx context.Context, input UpdateReportingSettingsInput) (ReportingSettings, error) {
	if s.updateFn != nil {
		return s.updateFn(ctx, input)
	}
	return ReportingSettings{Cadence: input.Cadence, RetentionDays: input.RetentionDays}, nil
}

func (s stubReportingRepository) ListSnapshots(ctx context.Context, subnetID int64, from, to time.Time) ([]SubnetUsageSnapshot, error) {
	if s.listSnapshotsFn != nil {
		return s.listSnapshotsFn(ctx, subnetID, from, to)
	}
	return nil, nil
}

func (s stubReportingRepository) CaptureDueSnapshots(ctx context.Context) (int64, error) {
	if s.captureDueFn != nil {
		return s.captureDueFn(ctx)
	}
	return 0, nil
}

func (s stubReportingRepository) DeleteExpiredSnapshots(ctx context.Context) (int64, error) {
	if s.deleteExpiredFn != nil {
		return s.deleteExpiredFn(ctx)
	}
	return 0, nil
}

func TestReportingSettingsValidation(t *testing.T) {
	service := NewReportingService(stubReportingRepository{}, stubSubnetRepository{})
	for _, input := range []UpdateReportingSettingsInput{
		{Cadence: "monthly", RetentionDays: 30},
		{Cadence: ReportingCadenceHourly, RetentionDays: 0},
		{Cadence: ReportingCadenceWeekly, RetentionDays: 181},
	} {
		if _, err := service.UpdateSettings(context.Background(), input); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("expected invalid input for %+v, got %v", input, err)
		}
	}
	settings, err := service.UpdateSettings(context.Background(), UpdateReportingSettingsInput{Cadence: ReportingCadenceDaily, RetentionDays: 180})
	if err != nil || settings.Cadence != ReportingCadenceDaily || settings.RetentionDays != 180 {
		t.Fatalf("unexpected valid settings result: settings=%+v err=%v", settings, err)
	}
}

func TestGetSubnetUsageHistoryUsesBoundedWindowAndStoredSnapshots(t *testing.T) {
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	wantPoint := SubnetUsageSnapshot{SubnetID: 42, CapturedAt: now.Add(-time.Hour), UsedIPs: 8, TotalIPs: 256}
	reports := stubReportingRepository{
		settings: ReportingSettings{Cadence: ReportingCadenceHourly, RetentionDays: 30},
		listSnapshotsFn: func(_ context.Context, subnetID int64, from, to time.Time) ([]SubnetUsageSnapshot, error) {
			if subnetID != 42 || !from.Equal(now.Add(-7*24*time.Hour)) || !to.Equal(now) {
				t.Fatalf("unexpected snapshot query: subnet=%d from=%s to=%s", subnetID, from, to)
			}
			return []SubnetUsageSnapshot{wantPoint}, nil
		},
	}
	service := NewReportingService(reports, stubSubnetRepository{findFn: func(context.Context, int64) (Subnet, error) {
		return Subnet{ID: 42, CIDR: netip.MustParsePrefix("10.0.0.0/24")}, nil
	}}).(*reportingService)
	service.now = func() time.Time { return now }

	history, err := service.GetSubnetUsageHistory(context.Background(), 42, "7d")
	if err != nil {
		t.Fatalf("get history: %v", err)
	}
	if history.Range != "7d" || history.Cadence != ReportingCadenceHourly || len(history.Points) != 1 || history.Points[0] != wantPoint {
		t.Fatalf("unexpected history: %+v", history)
	}
}

func TestGetSubnetUsageHistoryRejectsUnboundedRangeAndIPv6(t *testing.T) {
	service := NewReportingService(stubReportingRepository{}, stubSubnetRepository{findFn: func(context.Context, int64) (Subnet, error) {
		return Subnet{CIDR: netip.MustParsePrefix("2001:db8::/64")}, nil
	}})
	if _, err := service.GetSubnetUsageHistory(context.Background(), 1, "all"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected invalid range, got %v", err)
	}
	if _, err := service.GetSubnetUsageHistory(context.Background(), 1, "24h"); !errors.Is(err, ErrIPv6Unsupported) {
		t.Fatalf("expected IPv6 unsupported, got %v", err)
	}
}

func TestRunSnapshotCycleCapturesBeforeCleanup(t *testing.T) {
	called := false
	service := NewReportingService(stubReportingRepository{
		captureDueFn: func(context.Context) (int64, error) { called = true; return 3, nil },
		deleteExpiredFn: func(context.Context) (int64, error) {
			if !called {
				t.Fatal("cleanup ran before capture")
			}
			return 2, nil
		},
	}, stubSubnetRepository{})
	result, err := service.RunSnapshotCycle(context.Background())
	if err != nil || result.Captured != 3 || result.Deleted != 2 {
		t.Fatalf("unexpected cycle result: result=%+v err=%v", result, err)
	}
}

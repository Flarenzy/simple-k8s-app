package domain

import (
	"context"
	"fmt"
	"time"
)

const (
	MinReportingRetentionDays int32 = 1
	MaxReportingRetentionDays int32 = 180
)

var usageRangeDurations = map[string]time.Duration{
	"24h":  24 * time.Hour,
	"7d":   7 * 24 * time.Hour,
	"30d":  30 * 24 * time.Hour,
	"90d":  90 * 24 * time.Hour,
	"180d": 180 * 24 * time.Hour,
}

type reportingService struct {
	reports ReportingRepository
	subnets SubnetRepository
	now     func() time.Time
}

func NewReportingService(reports ReportingRepository, subnets SubnetRepository) ReportingService {
	return &reportingService{reports: reports, subnets: subnets, now: time.Now}
}

func (s *reportingService) GetSettings(ctx context.Context) (ReportingSettings, error) {
	return s.reports.GetSettings(ctx)
}

func (s *reportingService) UpdateSettings(ctx context.Context, input UpdateReportingSettingsInput) (ReportingSettings, error) {
	if !validReportingCadence(input.Cadence) {
		return ReportingSettings{}, fmt.Errorf("%w: cadence must be hourly, daily, or weekly", ErrInvalidInput)
	}
	if input.RetentionDays < MinReportingRetentionDays || input.RetentionDays > MaxReportingRetentionDays {
		return ReportingSettings{}, fmt.Errorf("%w: retention_days must be between %d and %d", ErrInvalidInput, MinReportingRetentionDays, MaxReportingRetentionDays)
	}
	return s.reports.UpdateSettings(ctx, input)
}

func (s *reportingService) GetSubnetUsageHistory(ctx context.Context, subnetID int64, usageRange string) (SubnetUsageHistory, error) {
	duration, ok := usageRangeDurations[usageRange]
	if !ok {
		return SubnetUsageHistory{}, fmt.Errorf("%w: range must be 24h, 7d, 30d, 90d, or 180d", ErrInvalidInput)
	}
	subnet, err := s.subnets.FindByID(ctx, subnetID)
	if err != nil {
		return SubnetUsageHistory{}, err
	}
	if !subnet.CIDR.Addr().Is4() {
		return SubnetUsageHistory{}, ErrIPv6Unsupported
	}
	settings, err := s.reports.GetSettings(ctx)
	if err != nil {
		return SubnetUsageHistory{}, err
	}
	to := s.now().UTC()
	from := to.Add(-duration)
	points, err := s.reports.ListSnapshots(ctx, subnetID, from, to)
	if err != nil {
		return SubnetUsageHistory{}, err
	}
	if points == nil {
		points = []SubnetUsageSnapshot{}
	}
	return SubnetUsageHistory{
		SubnetID: subnetID,
		Range:    usageRange,
		From:     from,
		To:       to,
		Cadence:  settings.Cadence,
		Points:   points,
	}, nil
}

func (s *reportingService) RunSnapshotCycle(ctx context.Context) (SnapshotCycleResult, error) {
	captured, err := s.reports.CaptureDueSnapshots(ctx)
	if err != nil {
		return SnapshotCycleResult{}, err
	}
	deleted, err := s.reports.DeleteExpiredSnapshots(ctx)
	return SnapshotCycleResult{Captured: captured, Deleted: deleted}, err
}

func validReportingCadence(cadence ReportingCadence) bool {
	switch cadence {
	case ReportingCadenceHourly, ReportingCadenceDaily, ReportingCadenceWeekly:
		return true
	default:
		return false
	}
}

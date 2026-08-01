package db

import (
	"context"
	"time"

	sqlc "github.com/Flarenzy/simple-k8s-app/internal/db/sqlc"
	"github.com/Flarenzy/simple-k8s-app/internal/domain"
	"github.com/jackc/pgx/v5/pgtype"
)

type ReportingRepository struct {
	queries *sqlc.Queries
}

func NewReportingRepository(queries *sqlc.Queries) *ReportingRepository {
	return &ReportingRepository{queries: queries}
}

func (r *ReportingRepository) GetSettings(ctx context.Context) (domain.ReportingSettings, error) {
	settings, err := r.queries.GetReportingSettings(ctx)
	if err != nil {
		return domain.ReportingSettings{}, err
	}
	return reportingSettings(settings.Cadence, settings.RetentionDays, settings.LastSnapshotAt), nil
}

func (r *ReportingRepository) UpdateSettings(ctx context.Context, input domain.UpdateReportingSettingsInput) (domain.ReportingSettings, error) {
	settings, err := r.queries.UpdateReportingSettings(ctx, sqlc.UpdateReportingSettingsParams{
		Cadence:       string(input.Cadence),
		RetentionDays: input.RetentionDays,
	})
	if err != nil {
		return domain.ReportingSettings{}, err
	}
	return reportingSettings(settings.Cadence, settings.RetentionDays, settings.LastSnapshotAt), nil
}

func (r *ReportingRepository) ListSnapshots(ctx context.Context, subnetID int64, from, to time.Time) ([]domain.SubnetUsageSnapshot, error) {
	snapshots, err := r.queries.ListSubnetUsageSnapshots(ctx, sqlc.ListSubnetUsageSnapshotsParams{
		SubnetID:     subnetID,
		CapturedAt:   pgtype.Timestamptz{Time: from, Valid: true},
		CapturedAt_2: pgtype.Timestamptz{Time: to, Valid: true},
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.SubnetUsageSnapshot, 0, len(snapshots))
	for _, snapshot := range snapshots {
		out = append(out, domain.SubnetUsageSnapshot{
			SubnetID:   snapshot.SubnetID,
			CapturedAt: snapshot.CapturedAt.Time,
			UsedIPs:    snapshot.UsedIps,
			TotalIPs:   snapshot.TotalIps,
		})
	}
	return out, nil
}

func (r *ReportingRepository) CaptureDueSnapshots(ctx context.Context) (int64, error) {
	return r.queries.CaptureDueSubnetUsageSnapshots(ctx)
}

func (r *ReportingRepository) DeleteExpiredSnapshots(ctx context.Context) (int64, error) {
	return r.queries.DeleteExpiredSubnetUsageSnapshots(ctx)
}

func reportingSettings(cadence string, retentionDays int32, lastSnapshot pgtype.Timestamptz) domain.ReportingSettings {
	settings := domain.ReportingSettings{Cadence: domain.ReportingCadence(cadence), RetentionDays: retentionDays}
	if lastSnapshot.Valid {
		capturedAt := lastSnapshot.Time
		settings.LastSnapshotAt = &capturedAt
	}
	return settings
}

package db

import (
	"context"
	"testing"
	"time"

	sqlc "github.com/Flarenzy/simple-k8s-app/internal/db/sqlc"
	"github.com/Flarenzy/simple-k8s-app/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestReportingRepositoryMapsSettingsAndLastSnapshot(t *testing.T) {
	capturedAt := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	repository := NewReportingRepository(sqlc.New(stubDBTX{queryRowFn: func(context.Context, string, ...any) pgx.Row {
		return stubRow{values: []any{"hourly", int32(30), pgtype.Timestamptz{Time: capturedAt, Valid: true}}}
	}}))
	settings, err := repository.GetSettings(context.Background())
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	if settings.Cadence != domain.ReportingCadenceHourly || settings.RetentionDays != 30 || settings.LastSnapshotAt == nil || !settings.LastSnapshotAt.Equal(capturedAt) {
		t.Fatalf("unexpected settings: %+v", settings)
	}
}

func TestReportingRepositoryMapsUsageSnapshots(t *testing.T) {
	capturedAt := pgtype.Timestamptz{Time: time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC), Valid: true}
	repository := NewReportingRepository(sqlc.New(stubDBTX{queryFn: func(context.Context, string, ...any) (pgx.Rows, error) {
		return &stubRows{rows: [][]any{{int64(42), capturedAt, int64(8), int64(256)}}}, nil
	}}))
	snapshots, err := repository.ListSnapshots(context.Background(), 42, capturedAt.Time.Add(-time.Hour), capturedAt.Time.Add(time.Hour))
	if err != nil {
		t.Fatalf("list snapshots: %v", err)
	}
	if len(snapshots) != 1 || snapshots[0].SubnetID != 42 || snapshots[0].UsedIPs != 8 || snapshots[0].TotalIPs != 256 || !snapshots[0].CapturedAt.Equal(capturedAt.Time) {
		t.Fatalf("unexpected snapshots: %+v", snapshots)
	}
}

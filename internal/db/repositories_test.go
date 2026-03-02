package db

import (
	"context"
	"errors"
	"net/netip"
	"reflect"
	"testing"
	"time"

	sqlc "github.com/Flarenzy/simple-k8s-app/internal/db/sqlc"
	"github.com/Flarenzy/simple-k8s-app/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type stubDBTX struct {
	queryFn    func(context.Context, string, ...any) (pgx.Rows, error)
	queryRowFn func(context.Context, string, ...any) pgx.Row
}

func (s stubDBTX) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (s stubDBTX) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if s.queryFn == nil {
		return &stubRows{}, nil
	}
	return s.queryFn(ctx, sql, args...)
}

func (s stubDBTX) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if s.queryRowFn == nil {
		return stubRow{err: pgx.ErrNoRows}
	}
	return s.queryRowFn(ctx, sql, args...)
}

type stubRow struct {
	values []any
	err    error
}

func (r stubRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return assignScannedValues(dest, r.values)
}

type stubRows struct {
	rows   [][]any
	idx    int
	closed bool
	err    error
}

func (r *stubRows) Close() {
	r.closed = true
}

func (r stubRows) Err() error {
	return r.err
}

func (r stubRows) CommandTag() pgconn.CommandTag {
	return pgconn.CommandTag{}
}

func (r stubRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}

func (r *stubRows) Next() bool {
	if r.idx >= len(r.rows) {
		r.closed = true
		return false
	}
	r.idx++
	return true
}

func (r *stubRows) Scan(dest ...any) error {
	if r.idx == 0 || r.idx > len(r.rows) {
		return errors.New("scan called without current row")
	}
	return assignScannedValues(dest, r.rows[r.idx-1])
}

func (r *stubRows) Values() ([]any, error) {
	if r.idx == 0 || r.idx > len(r.rows) {
		return nil, errors.New("values called without current row")
	}
	return r.rows[r.idx-1], nil
}

func (r *stubRows) RawValues() [][]byte {
	return nil
}

func (r *stubRows) Conn() *pgx.Conn {
	return nil
}

func assignScannedValues(dest []any, values []any) error {
	if len(dest) != len(values) {
		return errors.New("scan argument mismatch")
	}

	for i := range dest {
		target := reflect.ValueOf(dest[i])
		if target.Kind() != reflect.Ptr || target.IsNil() {
			return errors.New("destination must be a non-nil pointer")
		}

		value := reflect.ValueOf(values[i])
		if !value.Type().AssignableTo(target.Elem().Type()) {
			return errors.New("value not assignable to destination")
		}
		target.Elem().Set(value)
	}

	return nil
}

func mustPrefix(t *testing.T, cidr string) netip.Prefix {
	t.Helper()

	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		t.Fatalf("parse prefix %q: %v", cidr, err)
	}

	return prefix
}

func mustAddr(t *testing.T, ip string) netip.Addr {
	t.Helper()

	addr, err := netip.ParseAddr(ip)
	if err != nil {
		t.Fatalf("parse addr %q: %v", ip, err)
	}

	return addr
}

func mustUUID(t *testing.T, id string) pgtype.UUID {
	t.Helper()

	parsed, err := parseDomainIPID(domain.IPAddressID(id))
	if err != nil {
		t.Fatalf("parse uuid %q: %v", id, err)
	}

	return parsed
}

func testTimestamptz() pgtype.Timestamptz {
	return pgtype.Timestamptz{
		Time:  time.Unix(1700000000, 0).UTC(),
		Valid: true,
	}
}

func TestSubnetRepositoryFindByIDMapsNoRowsToNotFound(t *testing.T) {
	repo := NewSubnetRepository(sqlc.New(stubDBTX{
		queryRowFn: func(context.Context, string, ...any) pgx.Row {
			return stubRow{err: pgx.ErrNoRows}
		},
	}))

	_, err := repo.FindByID(context.Background(), 42)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestSubnetRepositoryDeleteReturnsFalseWhenNothingDeleted(t *testing.T) {
	repo := NewSubnetRepository(sqlc.New(stubDBTX{
		queryRowFn: func(context.Context, string, ...any) pgx.Row {
			return stubRow{values: []any{int64(0)}}
		},
	}))

	deleted, err := repo.Delete(context.Background(), 42)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if deleted {
		t.Fatal("expected deleted to be false")
	}
}

func TestSubnetRepositoryListMapsRowsToDomain(t *testing.T) {
	now := testTimestamptz()
	repo := NewSubnetRepository(sqlc.New(stubDBTX{
		queryFn: func(context.Context, string, ...any) (pgx.Rows, error) {
			return &stubRows{
				rows: [][]any{
					{int64(7), mustPrefix(t, "10.0.0.0/24"), "office", now, now},
				},
			}, nil
		},
	}))

	subnets, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(subnets) != 1 {
		t.Fatalf("expected 1 subnet, got %d", len(subnets))
	}
	if subnets[0].ID != 7 || subnets[0].CIDR.String() != "10.0.0.0/24" || subnets[0].Description != "office" {
		t.Fatalf("unexpected subnet: %+v", subnets[0])
	}
}

func TestIPRepositoryFindByIDAndSubnetRejectsInvalidUUID(t *testing.T) {
	repo := NewIPRepository(sqlc.New(stubDBTX{}))

	_, err := repo.FindByIDAndSubnet(context.Background(), domain.IPAddressID("not-a-uuid"), 42)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestIPRepositoryFindByIDAndSubnetMapsNoRowsToNotFound(t *testing.T) {
	repo := NewIPRepository(sqlc.New(stubDBTX{
		queryRowFn: func(context.Context, string, ...any) pgx.Row {
			return stubRow{err: pgx.ErrNoRows}
		},
	}))

	_, err := repo.FindByIDAndSubnet(context.Background(), domain.IPAddressID("550e8400-e29b-41d4-a716-446655440000"), 42)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestIPRepositoryCreateMapsUniqueViolationToConflict(t *testing.T) {
	repo := NewIPRepository(sqlc.New(stubDBTX{
		queryRowFn: func(context.Context, string, ...any) pgx.Row {
			return stubRow{err: &pgconn.PgError{ConstraintName: "unique_ip"}}
		},
	}))

	_, err := repo.Create(context.Background(), domain.CreateIPRecord{
		IP:       mustAddr(t, "10.0.0.10"),
		Hostname: "printer",
	}, 42)
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func TestIPRepositoryUpdateHostnameMapsNoRowsToNotFound(t *testing.T) {
	repo := NewIPRepository(sqlc.New(stubDBTX{
		queryRowFn: func(context.Context, string, ...any) pgx.Row {
			return stubRow{err: pgx.ErrNoRows}
		},
	}))

	_, err := repo.UpdateHostname(context.Background(), domain.IPAddressID("550e8400-e29b-41d4-a716-446655440000"), domain.UpdateIPInput{Hostname: "new-host"})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestIPRepositoryDeleteByIDAndSubnetRejectsInvalidUUID(t *testing.T) {
	repo := NewIPRepository(sqlc.New(stubDBTX{}))

	deleted, err := repo.DeleteByIDAndSubnet(context.Background(), domain.IPAddressID("not-a-uuid"), 42)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
	if deleted {
		t.Fatal("expected deleted to be false")
	}
}

func TestIPRepositoryDeleteByIDAndSubnetReturnsFalseOnNoRows(t *testing.T) {
	repo := NewIPRepository(sqlc.New(stubDBTX{
		queryRowFn: func(context.Context, string, ...any) pgx.Row {
			return stubRow{err: pgx.ErrNoRows}
		},
	}))

	deleted, err := repo.DeleteByIDAndSubnet(context.Background(), domain.IPAddressID("550e8400-e29b-41d4-a716-446655440000"), 42)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if deleted {
		t.Fatal("expected deleted to be false")
	}
}

func TestIPRepositoryListBySubnetIDMapsRowsToDomain(t *testing.T) {
	now := testTimestamptz()
	repo := NewIPRepository(sqlc.New(stubDBTX{
		queryFn: func(context.Context, string, ...any) (pgx.Rows, error) {
			return &stubRows{
				rows: [][]any{
					{mustUUID(t, "550e8400-e29b-41d4-a716-446655440000"), mustAddr(t, "10.0.0.10"), "printer", now, now, int64(42)},
				},
			}, nil
		},
	}))

	ips, err := repo.ListBySubnetID(context.Background(), 42)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(ips) != 1 {
		t.Fatalf("expected 1 ip, got %d", len(ips))
	}
	if ips[0].ID != domain.IPAddressID(uuid.MustParse("550e8400-e29b-41d4-a716-446655440000").String()) || ips[0].IP.String() != "10.0.0.10" || ips[0].Hostname != "printer" {
		t.Fatalf("unexpected ip: %+v", ips[0])
	}
}

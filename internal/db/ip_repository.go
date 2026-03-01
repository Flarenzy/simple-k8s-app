package db

import (
	"context"
	"errors"
	"fmt"
	"net/netip"

	sqlc "github.com/Flarenzy/simple-k8s-app/internal/db/sqlc"
	"github.com/Flarenzy/simple-k8s-app/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type IPRepository struct {
	queries *sqlc.Queries
}

func NewIPRepository(queries *sqlc.Queries) *IPRepository {
	return &IPRepository{queries: queries}
}

func (r *IPRepository) ListBySubnetID(ctx context.Context, subnetID int64) ([]domain.IPAddress, error) {
	ips, err := r.queries.ListIPsBySubnetID(ctx, subnetID)
	if err != nil {
		return nil, err
	}

	out := make([]domain.IPAddress, 0, len(ips))
	for _, ip := range ips {
		out = append(out, toDomainIP(ip))
	}

	return out, nil
}

func (r *IPRepository) FindByIDAndSubnet(ctx context.Context, id domain.IPAddressID, subnetID int64) (domain.IPAddress, error) {
	parsedID, err := parseDomainIPID(id)
	if err != nil {
		return domain.IPAddress{}, fmt.Errorf("%w: invalid ip id", domain.ErrInvalidInput)
	}

	ip, err := r.queries.GetIPByUUIDandSubnetID(ctx, sqlc.GetIPByUUIDandSubnetIDParams{
		ID:       parsedID,
		SubnetID: subnetID,
	})
	if err != nil {
		if isNoRows(err) {
			return domain.IPAddress{}, domain.ErrNotFound
		}
		return domain.IPAddress{}, err
	}

	return toDomainIP(ip), nil
}

func (r *IPRepository) Create(ctx context.Context, input domain.CreateIPInput, subnetID int64) (domain.IPAddress, error) {
	ipAddr, err := netip.ParseAddr(input.IP)
	if err != nil {
		return domain.IPAddress{}, fmt.Errorf("%w: invalid ip", domain.ErrInvalidInput)
	}

	ip, err := r.queries.CreateIPAddress(ctx, sqlc.CreateIPAddressParams{
		Ip:       ipAddr,
		Hostname: input.Hostname,
		SubnetID: subnetID,
	})
	if err != nil {
		if isUniqueIPViolation(err) {
			return domain.IPAddress{}, domain.ErrConflict
		}
		return domain.IPAddress{}, err
	}

	return toDomainIP(ip), nil
}

func (r *IPRepository) UpdateHostname(ctx context.Context, id domain.IPAddressID, input domain.UpdateIPInput) (domain.IPAddress, error) {
	parsedID, err := parseDomainIPID(id)
	if err != nil {
		return domain.IPAddress{}, fmt.Errorf("%w: invalid ip id", domain.ErrInvalidInput)
	}

	ip, err := r.queries.UpdateIPByUUID(ctx, sqlc.UpdateIPByUUIDParams{
		Hostname: input.Hostname,
		ID:       parsedID,
	})
	if err != nil {
		if isNoRows(err) {
			return domain.IPAddress{}, domain.ErrNotFound
		}
		return domain.IPAddress{}, err
	}

	return toDomainIP(ip), nil
}

func (r *IPRepository) DeleteByIDAndSubnet(ctx context.Context, id domain.IPAddressID, subnetID int64) (bool, error) {
	parsedID, err := parseDomainIPID(id)
	if err != nil {
		return false, fmt.Errorf("%w: invalid ip id", domain.ErrInvalidInput)
	}

	_, err = r.queries.DeleteIPByUUIDandSubnetID(ctx, sqlc.DeleteIPByUUIDandSubnetIDParams{
		ID:       parsedID,
		SubnetID: subnetID,
	})
	if err != nil {
		if isNoRows(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func toDomainIP(ip sqlc.IpAddress) domain.IPAddress {
	return domain.IPAddress{
		ID:        domain.IPAddressID(ip.ID.String()),
		IP:        ip.Ip,
		Hostname:  ip.Hostname,
		SubnetID:  ip.SubnetID,
		CreatedAt: ip.CreatedAt.Time,
		UpdatedAt: ip.UpdatedAt.Time,
	}
}

func parseDomainIPID(id domain.IPAddressID) (pgtype.UUID, error) {
	u, err := uuid.Parse(string(id))
	if err != nil {
		return pgtype.UUID{}, err
	}

	var parsed pgtype.UUID
	copy(parsed.Bytes[:], u[:])
	parsed.Valid = true

	return parsed, nil
}

func isUniqueIPViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.ConstraintName == "unique_ip"
}

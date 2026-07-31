package db

import (
	"context"
	"errors"

	sqlc "github.com/Flarenzy/simple-k8s-app/internal/db/sqlc"
	"github.com/Flarenzy/simple-k8s-app/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type SubnetRepository struct {
	queries *sqlc.Queries
}

func NewSubnetRepository(queries *sqlc.Queries) *SubnetRepository {
	return &SubnetRepository{queries: queries}
}

func (r *SubnetRepository) List(ctx context.Context) ([]domain.Subnet, error) {
	subnets, err := r.queries.ListSubnets(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]domain.Subnet, 0, len(subnets))
	for _, subnet := range subnets {
		out = append(out, toDomainSubnet(subnet))
	}

	return out, nil
}

func (r *SubnetRepository) FindByID(ctx context.Context, id int64) (domain.Subnet, error) {
	subnet, err := r.queries.GetSubnetByID(ctx, id)
	if err != nil {
		if isNoRows(err) {
			return domain.Subnet{}, domain.ErrNotFound
		}
		return domain.Subnet{}, err
	}

	return toDomainSubnet(subnet), nil
}

func (r *SubnetRepository) Create(ctx context.Context, input domain.CreateSubnetRecord) (domain.Subnet, error) {
	subnet, err := r.queries.CreateSubnet(ctx, sqlc.CreateSubnetParams{
		Cidr:        input.CIDR,
		SiteID:      siteIDParam(input.SiteID),
		Description: input.Description,
	})
	if err != nil {
		return domain.Subnet{}, err
	}

	return toDomainSubnet(subnet), nil
}

func (r *SubnetRepository) Update(ctx context.Context, input domain.UpdateSubnetRecord) (domain.Subnet, error) {
	subnet, err := r.queries.UpdateSubnet(ctx, sqlc.UpdateSubnetParams{
		ID:          input.ID,
		Cidr:        input.CIDR,
		SiteID:      siteIDParam(input.SiteID),
		Description: input.Description,
	})
	if err != nil {
		if isNoRows(err) {
			return domain.Subnet{}, domain.ErrNotFound
		}
		return domain.Subnet{}, err
	}
	return toDomainSubnet(subnet), nil
}

func (r *SubnetRepository) Delete(ctx context.Context, id int64) (bool, error) {
	deleted, err := r.queries.DeleteSubnetByID(ctx, id)
	if err != nil {
		return false, err
	}

	return deleted > 0, nil
}

func toDomainSubnet(subnet sqlc.Subnet) domain.Subnet {
	return domain.Subnet{
		ID:          subnet.ID,
		CIDR:        subnet.Cidr,
		SiteID:      subnet.SiteID.Bytes,
		Description: subnet.Description,
		CreatedAt:   subnet.CreatedAt.Time,
		UpdatedAt:   subnet.UpdatedAt.Time,
	}
}

func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

func siteIDParam(id *uuid.UUID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *id, Valid: true}
}

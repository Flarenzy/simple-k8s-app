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

type SitesRepository struct {
	queries *sqlc.Queries
}

func NewSitesRepository(queries *sqlc.Queries) *SitesRepository {
	return &SitesRepository{queries: queries}
}

func (r *SitesRepository) List(ctx context.Context) ([]domain.Site, error) {
	sites, err := r.queries.ListSites(ctx)
	if err != nil {
		return nil, err
	}
	list := make([]domain.Site, 0)
	for _, site := range sites {
		list = append(list, toDomainSite(site))
	}
	return list, nil
}

func (r *SitesRepository) FindByID(ctx context.Context, id uuid.UUID) (domain.Site, error) {
	site, err := r.queries.GetSiteByID(ctx, uUIDtoPgUUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Site{}, domain.ErrNotFound
		}
		return domain.Site{}, err
	}
	return toDomainSite(site), nil
}

func (r *SitesRepository) Create(ctx context.Context, input domain.CreateSiteRecord) (domain.Site, error) {
	var createSitesParams sqlc.CreateSiteParams
	createSitesParams.ID.Bytes = [16]byte(uuid.New())
	createSitesParams.ID.Valid = true
	createSitesParams.Name = input.Name
	createSitesParams.Description = input.Description
	site, err := r.queries.CreateSite(ctx, createSitesParams)
	if err != nil {
		return domain.Site{}, err
	}
	return toDomainSite(site), nil
}

func (r *SitesRepository) Delete(ctx context.Context, id uuid.UUID) (bool, error) {
	count, err := r.queries.DeleteSiteByID(ctx, uUIDtoPgUUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return count > 0, nil
}

func (r *SitesRepository) Update(ctx context.Context, input domain.UpdateSiteInput) (domain.Site, error) {
	var updateSitesParams sqlc.UpdateSiteParams
	updateSitesParams.ID.Valid = true
	updateSitesParams.ID.Bytes = input.ID
	updateSitesParams.Name = input.Name
	updateSitesParams.Description = input.Description
	site, err := r.queries.UpdateSite(ctx, updateSitesParams)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Site{}, domain.ErrNotFound
		}
		return domain.Site{}, err
	}
	return toDomainSite(site), nil
}

func (r *SitesRepository) PerSubnetStatistics(ctx context.Context) ([]domain.SubnetStatistics, error) {
	statistics, err := r.queries.PerSubnetStatistics(ctx)
	if err != nil {
		return []domain.SubnetStatistics{}, err
	}
	list := make([]domain.SubnetStatistics, 0)
	for _, statistic := range statistics {
		if statistic.Cidr == nil {
			continue
		}
		list = append(list, domain.SubnetStatistics{
			SiteID:      pgUUIDToUUID(statistic.ID),
			SubnetID:    statistic.SubID.Int64,
			CIDR:        *statistic.Cidr,
			UsedIPCount: statistic.UsedIps,
		})
	}
	return list, nil
}

func toDomainSite(site sqlc.Site) domain.Site {
	id := pgUUIDToUUID(site.ID)
	if id == uuid.Nil {
		return domain.Site{}
	}
	return domain.Site{
		ID:          id,
		Name:        site.Name,
		Description: site.Description,
		CreatedAt:   site.CreatedAt.Time,
		UpdatedAt:   site.UpdatedAt.Time,
	}
}

func pgUUIDToUUID(id pgtype.UUID) uuid.UUID {
	if !id.Valid {
		return uuid.Nil
	}
	return id.Bytes
}

func uUIDtoPgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{
		Bytes: id,
		Valid: true,
	}
}

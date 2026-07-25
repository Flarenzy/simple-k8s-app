package domain

import (
	"context"
	"math"

	"github.com/google/uuid"
)

type sitesService struct {
	sites SiteRepository
}

func NewSitesService(sites SiteRepository) SitesService {
	return &sitesService{
		sites: sites,
	}
}

func (s *sitesService) List(ctx context.Context) ([]Site, error) {
	return s.sites.List(ctx)
}

func (s *sitesService) FindByID(ctx context.Context, id uuid.UUID) (Site, error) {
	return s.sites.FindByID(ctx, id)
}

func (s *sitesService) Delete(ctx context.Context, id uuid.UUID) (bool, error) {
	_, err := s.sites.Delete(ctx, id)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *sitesService) Create(ctx context.Context, input CreateSiteInput) (Site, error) {
	site, err := s.sites.Create(ctx, CreateSiteRecord{
		Name:        input.Name,
		Description: input.Description,
	})
	if err != nil {
		return Site{}, err
	}
	return site, nil
}

func (s *sitesService) Update(ctx context.Context, input UpdateSiteInput) (Site, error) {
	site, err := s.sites.Update(ctx, input)
	if err != nil {
		return Site{}, err
	}
	return site, nil
}

func (s *sitesService) perSubnetStatistics(ctx context.Context) ([]SubnetStatistics, error) {
	statistics, err := s.sites.PerSubnetStatistics(ctx)
	if err != nil {
		return nil, err
	}
	return statistics, nil
}

func (s *sitesService) Statistics(ctx context.Context) ([]SiteStatistics, error) {
	sites, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	perSubnetStatistics, err := s.perSubnetStatistics(ctx)
	if err != nil {
		return nil, err
	}
	statistics := make([]SiteStatistics, len(sites))
	for i, site := range sites {
		siteStat := SiteStatistics{
			ID:          site.ID,
			Name:        site.Name,
			Description: site.Description,
			CreatedAt:   site.CreatedAt,
			UpdatedAt:   site.UpdatedAt,
		}
		for _, subnetStatistic := range perSubnetStatistics {
			if subnetStatistic.SiteID == site.ID {
				siteStat = addSubnetStatistic(siteStat, subnetStatistic)
			}
		}
		statistics[i] = siteStat
	}
	return statistics, nil
}

// TODO: refactor func
func addSubnetStatistic(siteStat SiteStatistics, subStat SubnetStatistics) SiteStatistics {
	if !subStat.CIDR.IsValid() {
		return siteStat
	}
	siteStat.SubnetCount++
	siteStat.UsedIPCount += subStat.UsedIPCount
	if subStat.CIDR.Bits() == 31 {
		if 2-subStat.UsedIPCount >= 0 {
			siteStat.FreeIPCount += 2 - subStat.UsedIPCount
		}
	} else if subStat.CIDR.Bits() == 32 {
		if 1-subStat.UsedIPCount >= 0 {
			siteStat.FreeIPCount += 1 - subStat.UsedIPCount
		}
	} else {
		availAddr := math.Pow(2, float64(32-subStat.CIDR.Bits())) - 2
		if subStat.UsedIPCount <= int64(availAddr) {
			siteStat.FreeIPCount += int64(availAddr) - subStat.UsedIPCount
		}
	}
	siteStat.TotalIPCount = siteStat.FreeIPCount + siteStat.UsedIPCount
	return siteStat
}

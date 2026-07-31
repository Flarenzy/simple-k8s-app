package domain

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
)

type siteRepositoryStub struct {
	sites      []Site
	statistics []SubnetStatistics
}

func (s siteRepositoryStub) List(context.Context) ([]Site, error) {
	return s.sites, nil
}

func (siteRepositoryStub) FindByID(context.Context, uuid.UUID) (Site, error) {
	return Site{}, nil
}

func (siteRepositoryStub) Create(context.Context, CreateSiteRecord) (Site, error) {
	return Site{}, nil
}

func (siteRepositoryStub) Update(context.Context, UpdateSiteInput) (Site, error) {
	return Site{}, nil
}

func (siteRepositoryStub) Delete(context.Context, uuid.UUID) (bool, error) {
	return false, nil
}

func (s siteRepositoryStub) PerSubnetStatistics(context.Context) ([]SubnetStatistics, error) {
	return s.statistics, nil
}

func TestSitesServiceStatisticsAggregatesAssociatedSubnets(t *testing.T) {
	firstSite := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	secondSite := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	now := time.Unix(1700000000, 0).UTC()
	service := NewSitesService(siteRepositoryStub{
		sites: []Site{
			{ID: firstSite, Name: "First", CreatedAt: now, UpdatedAt: now},
			{ID: secondSite, Name: "Second", CreatedAt: now, UpdatedAt: now},
		},
		statistics: []SubnetStatistics{
			{SiteID: firstSite, SubnetID: 1, CIDR: netip.MustParsePrefix("10.0.0.0/24"), UsedIPCount: 3},
			{SiteID: firstSite, SubnetID: 2, CIDR: netip.MustParsePrefix("2001:db8::/126"), UsedIPCount: 1},
			{SiteID: firstSite, SubnetID: 3, CIDR: netip.MustParsePrefix("2001:db8:1::/64"), UsedIPCount: 2},
			{SiteID: secondSite, SubnetID: 4, CIDR: netip.MustParsePrefix("10.0.1.0/31"), UsedIPCount: 2},
		},
	})

	statistics, err := service.Statistics(context.Background())
	if err != nil {
		t.Fatalf("get statistics: %v", err)
	}

	if got := statistics[0]; got.SubnetCount != 3 || got.UsedIPCount != 6 || got.FreeIPCount != 254 || got.TotalIPCount != 260 {
		t.Fatalf("unexpected first site statistics: %+v", got)
	}
	if got := statistics[1]; got.SubnetCount != 1 || got.UsedIPCount != 2 || got.FreeIPCount != 0 || got.TotalIPCount != 2 {
		t.Fatalf("unexpected second site statistics: %+v", got)
	}
}

func TestSubnetCapacityHandlesAddressFamilies(t *testing.T) {
	tests := []struct {
		cidr string
		want int64
	}{
		{cidr: "10.0.0.0/24", want: 254},
		{cidr: "10.0.0.0/31", want: 2},
		{cidr: "10.0.0.1/32", want: 1},
		{cidr: "2001:db8::/126", want: 4},
		{cidr: "2001:db8::/128", want: 1},
		{cidr: "2001:db8::/64", want: 0},
	}

	for _, test := range tests {
		t.Run(test.cidr, func(t *testing.T) {
			if got := subnetCapacity(netip.MustParsePrefix(test.cidr)); got != test.want {
				t.Fatalf("expected capacity %d, got %d", test.want, got)
			}
		})
	}
}

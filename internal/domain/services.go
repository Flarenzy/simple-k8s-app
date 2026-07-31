package domain

import (
	"context"

	"github.com/google/uuid"
)

type NetworkService interface {
	ListSubnets(ctx context.Context) ([]Subnet, error)
	CreateSubnet(ctx context.Context, input CreateSubnetInput) (Subnet, error)
	UpdateSubnet(ctx context.Context, input UpdateSubnetInput) (Subnet, error)
	GetSubnet(ctx context.Context, id int64) (Subnet, error)
	DeleteSubnet(ctx context.Context, id int64) error
	ListIPs(ctx context.Context, subnetID int64) ([]IPAddress, error)
	CreateIP(ctx context.Context, subnetID int64, input CreateIPInput) (IPAddress, error)
	UpdateIPHostname(ctx context.Context, subnetID int64, id IPAddressID, input UpdateIPInput) (IPAddress, error)
	DeleteIP(ctx context.Context, subnetID int64, id IPAddressID) error
}

type SitesService interface {
	List(ctx context.Context) ([]Site, error)
	FindByID(ctx context.Context, id uuid.UUID) (Site, error)
	Create(ctx context.Context, input CreateSiteInput) (Site, error)
	Update(ctx context.Context, input UpdateSiteInput) (Site, error)
	Delete(ctx context.Context, id uuid.UUID) (bool, error)
	Statistics(ctx context.Context) ([]SiteStatistics, error)
}

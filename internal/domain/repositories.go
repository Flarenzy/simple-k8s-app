package domain

import (
	"context"

	"github.com/google/uuid"
)

type SubnetRepository interface {
	List(ctx context.Context) ([]Subnet, error)
	FindByID(ctx context.Context, id int64) (Subnet, error)
	Create(ctx context.Context, input CreateSubnetRecord) (Subnet, error)
	Delete(ctx context.Context, id int64) (bool, error)
}

type IPRepository interface {
	ListBySubnetID(ctx context.Context, subnetID int64) ([]IPAddress, error)
	FindByIDAndSubnet(ctx context.Context, id IPAddressID, subnetID int64) (IPAddress, error)
	Create(ctx context.Context, input CreateIPRecord, subnetID int64) (IPAddress, error)
	UpdateHostname(ctx context.Context, id IPAddressID, input UpdateIPInput) (IPAddress, error)
	DeleteByIDAndSubnet(ctx context.Context, id IPAddressID, subnetID int64) (bool, error)
}

type SiteRepository interface {
	List(ctx context.Context) ([]Site, error)
	FindByID(ctx context.Context, id uuid.UUID) (Site, error)
	Create(ctx context.Context, input CreateSiteRecord) (Site, error)
	Update(ctx context.Context, input UpdateSiteInput) (Site, error)
	Delete(ctx context.Context, id uuid.UUID) (bool, error)
	PerSubnetStatistics(ctx context.Context) ([]SubnetStatistics, error)
}

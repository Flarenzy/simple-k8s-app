package domain

import "context"

type SubnetRepository interface {
	List(ctx context.Context) ([]Subnet, error)
	FindByID(ctx context.Context, id int64) (Subnet, error)
	Create(ctx context.Context, input CreateSubnetInput) (Subnet, error)
	Delete(ctx context.Context, id int64) (bool, error)
}

type IPRepository interface {
	ListBySubnetID(ctx context.Context, subnetID int64) ([]IPAddress, error)
	FindByIDAndSubnet(ctx context.Context, id IPAddressID, subnetID int64) (IPAddress, error)
	Create(ctx context.Context, input CreateIPInput, subnetID int64) (IPAddress, error)
	UpdateHostname(ctx context.Context, id IPAddressID, input UpdateIPInput) (IPAddress, error)
	DeleteByIDAndSubnet(ctx context.Context, id IPAddressID, subnetID int64) (bool, error)
}

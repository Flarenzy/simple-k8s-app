package domain

import "context"

type NetworkService interface {
	ListSubnets(ctx context.Context) ([]Subnet, error)
	CreateSubnet(ctx context.Context, input CreateSubnetInput) (Subnet, error)
	GetSubnet(ctx context.Context, id int64) (Subnet, error)
	DeleteSubnet(ctx context.Context, id int64) error
	ListIPs(ctx context.Context, subnetID int64) ([]IPAddress, error)
	CreateIP(ctx context.Context, subnetID int64, input CreateIPInput) (IPAddress, error)
	UpdateIPHostname(ctx context.Context, subnetID int64, id IPAddressID, input UpdateIPInput) (IPAddress, error)
	DeleteIP(ctx context.Context, subnetID int64, id IPAddressID) error
}

package domain

import (
	"net/netip"

	"github.com/google/uuid"
)

type CreateSubnetInput struct {
	CIDR        string
	Description string
}

type CreateIPInput struct {
	IP       string
	Hostname string
}

type CreateSiteInput struct {
	Name        string
	Description string
}

type UpdateIPInput struct {
	Hostname string
}

type UpdateSiteInput struct {
	ID          uuid.UUID
	Name        string
	Description string
}

type CreateSubnetRecord struct {
	CIDR        netip.Prefix
	Description string
}

type CreateIPRecord struct {
	IP       netip.Addr
	Hostname string
}

type CreateSiteRecord struct {
	Name        string
	Description string
}

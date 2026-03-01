package domain

import "net/netip"

type CreateSubnetInput struct {
	CIDR        string
	Description string
}

type CreateIPInput struct {
	IP       string
	Hostname string
}

type UpdateIPInput struct {
	Hostname string
}

type CreateSubnetRecord struct {
	CIDR        netip.Prefix
	Description string
}

type CreateIPRecord struct {
	IP       netip.Addr
	Hostname string
}

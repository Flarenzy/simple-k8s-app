package domain

import (
	"net/netip"
	"time"
)

type Subnet struct {
	ID          int64
	CIDR        netip.Prefix
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type IPAddress struct {
	ID        string
	IP        netip.Addr
	Hostname  string
	SubnetID  int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

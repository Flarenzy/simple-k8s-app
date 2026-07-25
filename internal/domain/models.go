package domain

import (
	"net/netip"
	"time"

	"github.com/google/uuid"
)

type IPAddressID string

type Subnet struct {
	ID          int64
	CIDR        netip.Prefix
	SiteID      uuid.UUID
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type IPAddress struct {
	ID        IPAddressID
	IP        netip.Addr
	Hostname  string
	SubnetID  int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Site struct {
	ID          uuid.UUID
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Statistics struct {
	SubnetCount  int64
	UsedIPCount  int64
	TotalIPCount int64
	FreeIPCount  int64
}

type SubnetStatistics struct {
	SiteID      uuid.UUID
	SubnetID    int64
	CIDR        netip.Prefix
	UsedIPCount int64
}

type SiteStatistics struct {
	ID           uuid.UUID
	Name         string
	Description  string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	SubnetCount  int64
	UsedIPCount  int64
	TotalIPCount int64
	FreeIPCount  int64
}

package domain

import (
	"net/netip"
	"time"

	"github.com/google/uuid"
)

type IPAddressID string

type Subnet struct {
	ID           int64
	CIDR         netip.Prefix
	SiteID       uuid.UUID
	UsedIPCount  int64
	TotalIPCount int64
	Description  string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type IPAddress struct {
	ID                 IPAddressID
	IP                 netip.Addr
	Hostname           string
	SubnetID           int64
	CreatedAt          time.Time
	UpdatedAt          time.Time
	KubernetesServices []KubernetesServiceEnrichment
}

type KubernetesSource struct {
	Key  string
	Name string
}

type KubernetesServicePort struct {
	Name        string
	Protocol    string
	Port        int32
	TargetPort  string
	AppProtocol string
	NodePort    *int32
}

type KubernetesMatchedAddress struct {
	IP   netip.Addr
	Kind string
}

type KubernetesServiceEnrichment struct {
	Source           KubernetesSource
	UID              string
	Name             string
	Namespace        string
	Type             string
	DNSName          string
	MatchedAddresses []KubernetesMatchedAddress
	Ports            []KubernetesServicePort
	ObservedAt       time.Time
}

type KubernetesServiceAddress struct {
	Kind    string
	Address netip.Addr
	IPMode  string
}

type KubernetesServiceHostname struct {
	Kind     string
	Hostname string
}

type KubernetesServiceSnapshot struct {
	UID             string
	Namespace       string
	Name            string
	Type            string
	ResourceVersion string
	ExternalName    string
	DNSName         string
	Ports           []KubernetesServicePort
	Addresses       []KubernetesServiceAddress
	Hostnames       []KubernetesServiceHostname
}

type KubernetesSourceConfig struct {
	Key            string
	Name           string
	SiteID         uuid.UUID
	ClusterDomain  string
	Namespaces     []string
	StaleRetention time.Duration
}

type KubernetesReconcileResult struct {
	Services  int
	Matched   int
	Unmatched int
	Ambiguous int
}

type KubernetesSourceStatus struct {
	Source        KubernetesSource
	SiteID        uuid.UUID
	ClusterDomain string
	Namespaces    []string
	State         string
	LastAttemptAt *time.Time
	LastSuccessAt *time.Time
	LastError     string
	Services      int
	Matched       int
	Unmatched     int
	Ambiguous     int
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

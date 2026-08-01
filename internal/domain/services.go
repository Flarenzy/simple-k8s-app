package domain

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
)

type ImportService interface {
	ImportCSV(ctx context.Context, input io.Reader) (ImportResult, error)
}

type NetworkService interface {
	ListSubnets(ctx context.Context) ([]Subnet, error)
	CreateSubnet(ctx context.Context, input CreateSubnetInput) (Subnet, error)
	UpdateSubnet(ctx context.Context, input UpdateSubnetInput) (Subnet, error)
	AssignSubnetSite(ctx context.Context, input AssignSubnetSiteInput) (Subnet, error)
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

type KubernetesDiscoveryService interface {
	Reconcile(ctx context.Context, source KubernetesSourceConfig, services []KubernetesServiceSnapshot, observedAt time.Time) (KubernetesReconcileResult, error)
	RecordFailure(ctx context.Context, source KubernetesSourceConfig, attemptedAt time.Time, err error) error
	ListSourceStatuses(ctx context.Context) ([]KubernetesSourceStatus, error)
	ListServicesBySubnetID(ctx context.Context, subnetID int64) ([]KubernetesServiceObservation, error)
}

type ReportingService interface {
	GetSettings(ctx context.Context) (ReportingSettings, error)
	UpdateSettings(ctx context.Context, input UpdateReportingSettingsInput) (ReportingSettings, error)
	GetSubnetUsageHistory(ctx context.Context, subnetID int64, usageRange string) (SubnetUsageHistory, error)
	RunSnapshotCycle(ctx context.Context) (SnapshotCycleResult, error)
}

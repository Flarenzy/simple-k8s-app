package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type SubnetRepository interface {
	List(ctx context.Context) ([]Subnet, error)
	FindByID(ctx context.Context, id int64) (Subnet, error)
	Create(ctx context.Context, input CreateSubnetRecord) (Subnet, error)
	Update(ctx context.Context, input UpdateSubnetRecord) (Subnet, error)
	AssignSite(ctx context.Context, id int64, siteID uuid.UUID) (Subnet, error)
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

type KubernetesDiscoveryRepository interface {
	Reconcile(ctx context.Context, source KubernetesSourceConfig, services []KubernetesServiceSnapshot, observedAt time.Time) (KubernetesReconcileResult, error)
	RecordFailure(ctx context.Context, source KubernetesSourceConfig, attemptedAt time.Time, message string) error
	ListSourceStatuses(ctx context.Context) ([]KubernetesSourceStatus, error)
	ListServicesBySubnetID(ctx context.Context, subnetID int64) (map[IPAddressID][]KubernetesServiceEnrichment, error)
	ListAllServicesBySubnetID(ctx context.Context, subnetID int64) ([]KubernetesServiceObservation, error)
}

type ReportingRepository interface {
	GetSettings(ctx context.Context) (ReportingSettings, error)
	UpdateSettings(ctx context.Context, input UpdateReportingSettingsInput) (ReportingSettings, error)
	ListSnapshots(ctx context.Context, subnetID int64, from, to time.Time) ([]SubnetUsageSnapshot, error)
	CaptureDueSnapshots(ctx context.Context) (int64, error)
	DeleteExpiredSnapshots(ctx context.Context) (int64, error)
}

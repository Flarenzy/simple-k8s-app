package http

import (
	"time"

	"github.com/Flarenzy/simple-k8s-app/internal/domain"
	"github.com/google/uuid"
)

// SubnetResponse is a simplified view returned to clients and used in Swagger.
type SubnetResponse struct {
	ID          int64      `json:"id" example:"1"`
	CIDR        string     `json:"cidr" example:"10.0.0.0/24"`
	SiteID      *uuid.UUID `json:"site_id,omitempty" example:"50e8400-e29b-41d4-a716-446655440000"`
	UsedIPs     int64      `json:"used_ips"`
	TotalIPs    int64      `json:"total_ips"`
	Description string     `json:"description" example:"Office network"`
	CreatedAt   time.Time  `json:"created_at" example:"2024-05-10T15:04:05Z"`
	UpdatedAt   time.Time  `json:"updated_at" example:"2024-05-10T15:04:05Z"`
}

// CreateSubnetRequest is the payload accepted when creating a subnet.
type CreateSubnetRequest struct {
	CIDR        string     `json:"cidr" example:"10.0.0.0/24" validate:"required"`
	SiteID      *uuid.UUID `json:"site_id" example:"50e8400-e29b-41d4-a716-446655440000"`
	Description string     `json:"description" example:"Office network"`
}

type SiteRequest struct {
	Name        string `json:"name" example:"Belgrade" validate:"required"`
	Description string `json:"description" example:"Primary office"`
}

type SiteResponse struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	SubnetCount int64     `json:"subnet_count"`
	UsedIPs     int64     `json:"used_ips"`
	TotalIPs    int64     `json:"total_ips"`
	FreeIPs     int64     `json:"free_ips"`
}

type AssignSubnetSiteRequest struct {
	SiteID *uuid.UUID `json:"site_id"`
}

type SiteStatisticsResponse struct {
	SiteResponse
	SubnetCount  int64 `json:"subnet_count"`
	UsedIPCount  int64 `json:"used_ip_count"`
	TotalIPCount int64 `json:"total_ip_count"`
	FreeIPCount  int64 `json:"free_ip_count"`
}

// ErrorResponse is a simple envelope for error messages.
type ErrorResponse struct {
	Error string `json:"error" example:"subnet not found"`
}

type ReportingSettingsRequest struct {
	Cadence       string `json:"cadence" example:"hourly" enums:"hourly,daily,weekly"`
	RetentionDays int32  `json:"retention_days" example:"30" minimum:"1" maximum:"180"`
}

type ReportingSettingsResponse struct {
	Cadence        string     `json:"cadence" example:"hourly" enums:"hourly,daily,weekly"`
	RetentionDays  int32      `json:"retention_days" example:"30"`
	LastSnapshotAt *time.Time `json:"last_snapshot_at,omitempty" example:"2026-08-01T10:00:00Z"`
}

type SubnetUsageSnapshotResponse struct {
	CapturedAt time.Time `json:"captured_at" example:"2026-08-01T10:00:00Z"`
	UsedIPs    int64     `json:"used_ips" example:"42"`
	TotalIPs   int64     `json:"total_ips" example:"256"`
}

type SubnetUsageHistoryResponse struct {
	SubnetID int64                         `json:"subnet_id" example:"4"`
	Range    string                        `json:"range" example:"7d"`
	From     time.Time                     `json:"from" example:"2026-07-25T10:00:00Z"`
	To       time.Time                     `json:"to" example:"2026-08-01T10:00:00Z"`
	Cadence  string                        `json:"cadence" example:"hourly"`
	Points   []SubnetUsageSnapshotResponse `json:"points"`
}

type ImportResponse struct {
	Processed int        `json:"processed"`
	Created   int        `json:"created"`
	Updated   int        `json:"updated"`
	Failed    int        `json:"failed"`
	Errors    []RowError `json:"errors"`
}

type RowError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}

// IPResponse is a simplified view returned to clients and used in Swagger.
type IPResponse struct {
	ID                 string                      `json:"id" example:"50e8400-e29b-41d4-a716-446655440000"`
	IP                 string                      `json:"ip" example:"10.0.0.1"`
	Hostname           string                      `json:"hostname" example:"printer-1"`
	SubnetID           int64                       `json:"subnet_id" example:"4"`
	CreatedAt          time.Time                   `json:"created_at" example:"2024-05-10T15:04:05Z"`
	UpdatedAt          time.Time                   `json:"updated_at" example:"2024-05-10T15:04:05Z"`
	KubernetesServices []KubernetesServiceResponse `json:"kubernetes_services"`
}

type KubernetesSourceResponse struct {
	Key  string `json:"key" example:"prod-cluster"`
	Name string `json:"name" example:"Production"`
}

type KubernetesMatchedAddressResponse struct {
	IP   string `json:"ip" example:"10.96.12.4"`
	Kind string `json:"kind" example:"cluster_ip"`
}

type KubernetesServicePortResponse struct {
	Name        string `json:"name" example:"https"`
	Protocol    string `json:"protocol" example:"TCP"`
	Port        int32  `json:"port" example:"443"`
	TargetPort  string `json:"target_port" example:"8443"`
	AppProtocol string `json:"app_protocol,omitempty" example:"https"`
	NodePort    *int32 `json:"node_port,omitempty" example:"30443"`
}

type KubernetesServiceResponse struct {
	Source           KubernetesSourceResponse           `json:"source"`
	UID              string                             `json:"uid" example:"02e12c93-1234-5678-90ab-abcdefabcdef"`
	Name             string                             `json:"name" example:"orders"`
	Namespace        string                             `json:"namespace" example:"commerce"`
	Type             string                             `json:"type" example:"LoadBalancer"`
	DNSName          string                             `json:"dns_name" example:"orders.commerce.svc.cluster.local"`
	MatchedAddresses []KubernetesMatchedAddressResponse `json:"matched_addresses"`
	Ports            []KubernetesServicePortResponse    `json:"ports"`
	ObservedAt       time.Time                          `json:"observed_at" example:"2026-08-01T10:00:00Z"`
}

type KubernetesAddressObservationResponse struct {
	IP                 string  `json:"ip" example:"10.96.12.4"`
	Kind               string  `json:"kind" example:"cluster_ip"`
	IPMode             string  `json:"ip_mode,omitempty" example:"VIP"`
	MatchStatus        string  `json:"match_status" example:"matched" enums:"matched,unmatched,ambiguous"`
	MatchCount         int     `json:"match_count" example:"1"`
	MatchedIPAddressID *string `json:"matched_ip_address_id,omitempty" example:"50e8400-e29b-41d4-a716-446655440000"`
	MatchedSubnetID    *int64  `json:"matched_subnet_id,omitempty" example:"4"`
}

type KubernetesHostnameObservationResponse struct {
	Kind     string `json:"kind" example:"load_balancer"`
	Hostname string `json:"hostname" example:"orders.example.com"`
}

type KubernetesServiceObservationResponse struct {
	Source       KubernetesSourceResponse                `json:"source"`
	UID          string                                  `json:"uid" example:"02e12c93-1234-5678-90ab-abcdefabcdef"`
	Name         string                                  `json:"name" example:"orders"`
	Namespace    string                                  `json:"namespace" example:"commerce"`
	Type         string                                  `json:"type" example:"LoadBalancer"`
	ExternalName string                                  `json:"external_name,omitempty" example:"orders.example.com"`
	DNSName      string                                  `json:"dns_name" example:"orders.commerce.svc.cluster.local"`
	MatchStatus  string                                  `json:"match_status" example:"matched" enums:"matched,unmatched,ambiguous,no_usable_ip"`
	Addresses    []KubernetesAddressObservationResponse  `json:"addresses"`
	Hostnames    []KubernetesHostnameObservationResponse `json:"hostnames"`
	Ports        []KubernetesServicePortResponse         `json:"ports"`
	ObservedAt   time.Time                               `json:"observed_at" example:"2026-08-01T10:00:00Z"`
}

type KubernetesDiscoveryStatusResponse struct {
	Source        KubernetesSourceResponse `json:"source"`
	SiteID        uuid.UUID                `json:"site_id"`
	ClusterDomain string                   `json:"cluster_domain"`
	Namespaces    []string                 `json:"namespaces"`
	State         string                   `json:"state" example:"healthy"`
	LastAttemptAt *time.Time               `json:"last_attempt_at"`
	LastSuccessAt *time.Time               `json:"last_success_at"`
	LastError     string                   `json:"last_error"`
	Services      int                      `json:"services"`
	Matched       int                      `json:"matched"`
	Unmatched     int                      `json:"unmatched"`
	Ambiguous     int                      `json:"ambiguous"`
	NoUsableIP    int                      `json:"no_usable_ip"`
}

// CreateIPRequest is the payload accepted when creating a ip.
type CreateIPRequest struct {
	IP       string `json:"ip" example:"10.0.0.1"`
	Hostname string `json:"hostname" example:"printer-1"`
}

// UpdateIPRequest is the payload accepted when updating an ip.
type UpdateIPRequest struct {
	Hostname string `json:"hostname" example:"pc-1"`
}

func subnetToResponse(s domain.Subnet) SubnetResponse {
	var siteID *uuid.UUID
	if s.SiteID != uuid.Nil {
		siteID = &s.SiteID
	}
	return SubnetResponse{
		ID:          s.ID,
		CIDR:        s.CIDR.String(),
		SiteID:      siteID,
		UsedIPs:     s.UsedIPCount,
		TotalIPs:    s.TotalIPCount,
		Description: s.Description,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
	}
}

func reportingSettingsToResponse(settings domain.ReportingSettings) ReportingSettingsResponse {
	return ReportingSettingsResponse{
		Cadence: string(settings.Cadence), RetentionDays: settings.RetentionDays,
		LastSnapshotAt: settings.LastSnapshotAt,
	}
}

func subnetUsageHistoryToResponse(history domain.SubnetUsageHistory) SubnetUsageHistoryResponse {
	points := make([]SubnetUsageSnapshotResponse, 0, len(history.Points))
	for _, point := range history.Points {
		points = append(points, SubnetUsageSnapshotResponse{
			CapturedAt: point.CapturedAt, UsedIPs: point.UsedIPs, TotalIPs: point.TotalIPs,
		})
	}
	return SubnetUsageHistoryResponse{
		SubnetID: history.SubnetID, Range: history.Range, From: history.From, To: history.To,
		Cadence: string(history.Cadence), Points: points,
	}
}

func subnetsToResponse(subnets []domain.Subnet) []SubnetResponse {
	out := make([]SubnetResponse, 0, len(subnets))
	for _, s := range subnets {
		out = append(out, subnetToResponse(s))
	}
	return out
}

func ipToResponse(i domain.IPAddress) IPResponse {
	response := IPResponse{
		ID:                 string(i.ID),
		IP:                 i.IP.String(),
		Hostname:           i.Hostname,
		SubnetID:           i.SubnetID,
		CreatedAt:          i.CreatedAt,
		UpdatedAt:          i.UpdatedAt,
		KubernetesServices: make([]KubernetesServiceResponse, 0, len(i.KubernetesServices)),
	}
	for _, service := range i.KubernetesServices {
		response.KubernetesServices = append(response.KubernetesServices, kubernetesServiceToResponse(service))
	}
	return response
}

func ipsToResponse(ips []domain.IPAddress) []IPResponse {
	out := make([]IPResponse, 0, len(ips))
	for _, ip := range ips {
		out = append(out, ipToResponse(ip))
	}
	return out
}

func kubernetesServiceToResponse(service domain.KubernetesServiceEnrichment) KubernetesServiceResponse {
	response := KubernetesServiceResponse{
		Source: KubernetesSourceResponse{Key: service.Source.Key, Name: service.Source.Name},
		UID:    service.UID, Name: service.Name, Namespace: service.Namespace, Type: service.Type,
		DNSName: service.DNSName, ObservedAt: service.ObservedAt,
		MatchedAddresses: make([]KubernetesMatchedAddressResponse, 0, len(service.MatchedAddresses)),
		Ports:            make([]KubernetesServicePortResponse, 0, len(service.Ports)),
	}
	for _, address := range service.MatchedAddresses {
		response.MatchedAddresses = append(response.MatchedAddresses, KubernetesMatchedAddressResponse{IP: address.IP.String(), Kind: address.Kind})
	}
	for _, port := range service.Ports {
		response.Ports = append(response.Ports, KubernetesServicePortResponse{
			Name: port.Name, Protocol: port.Protocol, Port: port.Port, TargetPort: port.TargetPort,
			AppProtocol: port.AppProtocol, NodePort: port.NodePort,
		})
	}
	return response
}

func kubernetesServiceObservationsToResponse(services []domain.KubernetesServiceObservation) []KubernetesServiceObservationResponse {
	responses := make([]KubernetesServiceObservationResponse, 0, len(services))
	for _, service := range services {
		response := KubernetesServiceObservationResponse{
			Source:       KubernetesSourceResponse{Key: service.Source.Key, Name: service.Source.Name},
			UID:          service.UID,
			Name:         service.Name,
			Namespace:    service.Namespace,
			Type:         service.Type,
			ExternalName: service.ExternalName,
			DNSName:      service.DNSName,
			MatchStatus:  string(service.MatchStatus),
			Addresses:    make([]KubernetesAddressObservationResponse, 0, len(service.Addresses)),
			Hostnames:    make([]KubernetesHostnameObservationResponse, 0, len(service.Hostnames)),
			Ports:        make([]KubernetesServicePortResponse, 0, len(service.Ports)),
			ObservedAt:   service.ObservedAt,
		}
		for _, address := range service.Addresses {
			var matchedIPAddressID *string
			if address.MatchedIPAddressID != nil {
				id := string(*address.MatchedIPAddressID)
				matchedIPAddressID = &id
			}
			response.Addresses = append(response.Addresses, KubernetesAddressObservationResponse{
				IP: address.IP.String(), Kind: address.Kind, IPMode: address.IPMode,
				MatchStatus: string(address.MatchStatus), MatchCount: address.MatchCount,
				MatchedIPAddressID: matchedIPAddressID, MatchedSubnetID: address.MatchedSubnetID,
			})
		}
		for _, hostname := range service.Hostnames {
			response.Hostnames = append(response.Hostnames, KubernetesHostnameObservationResponse{
				Kind: hostname.Kind, Hostname: hostname.Hostname,
			})
		}
		for _, port := range service.Ports {
			response.Ports = append(response.Ports, KubernetesServicePortResponse{
				Name: port.Name, Protocol: port.Protocol, Port: port.Port, TargetPort: port.TargetPort,
				AppProtocol: port.AppProtocol, NodePort: port.NodePort,
			})
		}
		responses = append(responses, response)
	}
	return responses
}

func kubernetesStatusesToResponse(statuses []domain.KubernetesSourceStatus) []KubernetesDiscoveryStatusResponse {
	responses := make([]KubernetesDiscoveryStatusResponse, 0, len(statuses))
	for _, status := range statuses {
		responses = append(responses, KubernetesDiscoveryStatusResponse{
			Source: KubernetesSourceResponse{Key: status.Source.Key, Name: status.Source.Name},
			SiteID: status.SiteID, ClusterDomain: status.ClusterDomain,
			Namespaces: append([]string(nil), status.Namespaces...), State: status.State,
			LastAttemptAt: status.LastAttemptAt, LastSuccessAt: status.LastSuccessAt, LastError: status.LastError,
			Services: status.Services, Matched: status.Matched, Unmatched: status.Unmatched, Ambiguous: status.Ambiguous,
			NoUsableIP: status.NoUsableIP,
		})
	}
	return responses
}

func (r CreateSubnetRequest) toInput() domain.CreateSubnetInput {
	return domain.CreateSubnetInput{
		CIDR:        r.CIDR,
		SiteID:      r.SiteID,
		Description: r.Description,
	}
}

func (r SiteRequest) createInput() domain.CreateSiteInput {
	return domain.CreateSiteInput{Name: r.Name, Description: r.Description}
}

func (r SiteRequest) updateInput(id uuid.UUID) domain.UpdateSiteInput {
	return domain.UpdateSiteInput{ID: id, Name: r.Name, Description: r.Description}
}

func siteToResponse(site domain.Site) SiteResponse {
	return SiteResponse{ID: site.ID, Name: site.Name, Description: site.Description, CreatedAt: site.CreatedAt, UpdatedAt: site.UpdatedAt}
}

func sitesToResponse(sites []domain.Site) []SiteResponse {
	responses := make([]SiteResponse, 0, len(sites))
	for _, site := range sites {
		responses = append(responses, siteToResponse(site))
	}
	return responses
}

func siteStatisticsToSiteResponses(statistics []domain.SiteStatistics) []SiteResponse {
	responses := make([]SiteResponse, 0, len(statistics))
	for _, statistic := range statistics {
		responses = append(responses, SiteResponse{
			ID: statistic.ID, Name: statistic.Name, Description: statistic.Description,
			CreatedAt: statistic.CreatedAt, UpdatedAt: statistic.UpdatedAt,
			SubnetCount: statistic.SubnetCount, UsedIPs: statistic.UsedIPCount,
			TotalIPs: statistic.TotalIPCount, FreeIPs: statistic.FreeIPCount,
		})
	}
	return responses
}

func siteStatisticsToResponse(statistics []domain.SiteStatistics) []SiteStatisticsResponse {
	responses := make([]SiteStatisticsResponse, 0, len(statistics))
	for _, statistic := range statistics {
		responses = append(responses, SiteStatisticsResponse{
			SiteResponse: SiteResponse{
				ID: statistic.ID, Name: statistic.Name, Description: statistic.Description,
				CreatedAt: statistic.CreatedAt, UpdatedAt: statistic.UpdatedAt,
				SubnetCount: statistic.SubnetCount, UsedIPs: statistic.UsedIPCount,
				TotalIPs: statistic.TotalIPCount, FreeIPs: statistic.FreeIPCount,
			},
			SubnetCount:  statistic.SubnetCount,
			UsedIPCount:  statistic.UsedIPCount,
			TotalIPCount: statistic.TotalIPCount,
			FreeIPCount:  statistic.FreeIPCount,
		})
	}
	return responses
}

func (i CreateIPRequest) toInput() domain.CreateIPInput {
	return domain.CreateIPInput{
		IP:       i.IP,
		Hostname: i.Hostname,
	}
}

func (r UpdateIPRequest) toInput() domain.UpdateIPInput {
	return domain.UpdateIPInput{
		Hostname: r.Hostname,
	}
}

func importResultToResponse(result domain.ImportResult) ImportResponse {
	errors := make([]RowError, 0, len(result.Errors))
	for _, rowError := range result.Errors {
		errors = append(errors, RowError{Row: rowError.Row, Message: rowError.Message})
	}
	return ImportResponse{Processed: result.Processed, Created: result.Created, Updated: result.Updated, Failed: result.Failed, Errors: errors}
}

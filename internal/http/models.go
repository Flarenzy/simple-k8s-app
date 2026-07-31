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
	Description string     `json:"description" example:"Office network"`
	CreatedAt   time.Time  `json:"created_at" example:"2024-05-10T15:04:05Z"`
	UpdatedAt   time.Time  `json:"updated_at" example:"2024-05-10T15:04:05Z"`
}

// CreateSubnetRequest is the payload accepted when creating a subnet.
type CreateSubnetRequest struct {
	CIDR        string     `json:"cidr" example:"10.0.0.0/24" validate:"required"`
	SiteID      *uuid.UUID `json:"site_id,omitempty" example:"50e8400-e29b-41d4-a716-446655440000"`
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

// IPResponse is a simplified view returned to clients and used in Swagger.
type IPResponse struct {
	ID        string    `json:"id" example:"50e8400-e29b-41d4-a716-446655440000"`
	IP        string    `json:"ip" example:"10.0.0.1"`
	Hostname  string    `json:"hostname" example:"printer-1"`
	SubnetID  int64     `json:"subnet_id" example:"4"`
	CreatedAt time.Time `json:"created_at" example:"2024-05-10T15:04:05Z"`
	UpdatedAt time.Time `json:"updated_at" example:"2024-05-10T15:04:05Z"`
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
		Description: s.Description,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
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
	return IPResponse{
		ID:        string(i.ID),
		IP:        i.IP.String(),
		Hostname:  i.Hostname,
		SubnetID:  i.SubnetID,
		CreatedAt: i.CreatedAt,
		UpdatedAt: i.UpdatedAt,
	}
}

func ipsToResponse(ips []domain.IPAddress) []IPResponse {
	out := make([]IPResponse, 0, len(ips))
	for _, ip := range ips {
		out = append(out, ipToResponse(ip))
	}
	return out
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

func siteStatisticsToResponse(statistics []domain.SiteStatistics) []SiteStatisticsResponse {
	responses := make([]SiteStatisticsResponse, 0, len(statistics))
	for _, statistic := range statistics {
		responses = append(responses, SiteStatisticsResponse{
			SiteResponse: siteToResponse(domain.Site{ID: statistic.ID, Name: statistic.Name, Description: statistic.Description, CreatedAt: statistic.CreatedAt, UpdatedAt: statistic.UpdatedAt}),
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

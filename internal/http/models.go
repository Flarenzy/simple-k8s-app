package http

import (
	"time"

	"github.com/Flarenzy/simple-k8s-app/internal/domain"
)

// SubnetResponse is a simplified view returned to clients and used in Swagger.
type SubnetResponse struct {
	ID          int64     `json:"id" example:"1"`
	CIDR        string    `json:"cidr" example:"10.0.0.0/24"`
	Description string    `json:"description" example:"Office network"`
	CreatedAt   time.Time `json:"created_at" example:"2024-05-10T15:04:05Z"`
	UpdatedAt   time.Time `json:"updated_at" example:"2024-05-10T15:04:05Z"`
}

// CreateSubnetRequest is the payload accepted when creating a subnet.
type CreateSubnetRequest struct {
	CIDR        string `json:"cidr" example:"10.0.0.0/24" validate:"required"`
	Description string `json:"description" example:"Office network"`
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
	return SubnetResponse{
		ID:          s.ID,
		CIDR:        s.CIDR.String(),
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
		Description: r.Description,
	}
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

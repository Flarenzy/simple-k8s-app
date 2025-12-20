package http

import (
	"fmt"
	"net/netip"
	"time"

	sqlc "github.com/Flarenzy/simple-k8s-app/internal/db/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
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

func subnetToResponse(s sqlc.Subnet) SubnetResponse {
	return SubnetResponse{
		ID:          s.ID,
		CIDR:        s.Cidr.String(),
		Description: s.Description,
		CreatedAt:   s.CreatedAt.Time,
		UpdatedAt:   s.UpdatedAt.Time,
	}
}

func subnetsToResponse(subnets []sqlc.Subnet) []SubnetResponse {
	out := make([]SubnetResponse, 0, len(subnets))
	for _, s := range subnets {
		out = append(out, subnetToResponse(s))
	}
	return out
}

func ipToResponse(i sqlc.IpAddress) IPResponse {
	return IPResponse{
		ID:        i.ID.String(),
		IP:        i.Ip.String(),
		Hostname:  i.Hostname,
		SubnetID:  i.SubnetID,
		CreatedAt: i.CreatedAt.Time,
		UpdatedAt: i.UpdatedAt.Time,
	}
}

func ipsToreponse(ips []sqlc.IpAddress) []IPResponse {
	out := make([]IPResponse, 0, len(ips))
	for _, ip := range ips {
		out = append(out, ipToResponse(ip))
	}
	return out
}

func (r CreateSubnetRequest) toParams() (sqlc.CreateSubnetParams, error) {
	cidr, err := netip.ParsePrefix(r.CIDR)
	if err != nil {
		return sqlc.CreateSubnetParams{}, fmt.Errorf("parse cidr: %w", err)
	}
	return sqlc.CreateSubnetParams{
		Cidr:        cidr,
		Description: r.Description,
	}, nil
}

func (i CreateIPRequest) toParams(id int64) (sqlc.CreateIPAddressParams, error) {
	ip, err := netip.ParseAddr(i.IP)
	if err != nil {
		return sqlc.CreateIPAddressParams{}, err
	}
	return sqlc.CreateIPAddressParams{
		Ip:       ip,
		Hostname: i.Hostname,
		SubnetID: id,
	}, nil

}

func (r UpdateIPRequest) toParams(id pgtype.UUID) sqlc.UpdateIPByUUIDParams {
	return sqlc.UpdateIPByUUIDParams{
		Hostname: r.Hostname,
		ID:       id,
	}
}

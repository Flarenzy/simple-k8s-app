package http

import (
	"fmt"
	"net/netip"
	"time"

	sqlc "github.com/Flarenzy/simple-k8s-app/internal/db/sqlc"
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

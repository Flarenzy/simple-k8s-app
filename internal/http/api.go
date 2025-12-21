package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/netip"

	sqlc "github.com/Flarenzy/simple-k8s-app/internal/db/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/swaggo/http-swagger" // http-swagger middleware
)

type API struct {
	Logger  *slog.Logger
	DB      *pgxpool.Pool
	Queries *sqlc.Queries
}

func NewAPI(logger *slog.Logger, db *pgxpool.Pool, queries *sqlc.Queries) *API {
	return &API{
		Logger:  logger,
		DB:      db,
		Queries: queries,
	}
}

func (a *API) Router() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", a.handleHealthz)
	mux.HandleFunc("/readyz", a.handleReadyz)
	mux.Handle("/swagger/", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))

	mux.HandleFunc("GET /api/v1/subnets", a.handleGetAllSubnets)
	mux.HandleFunc("POST /api/v1/subnets", a.handleCreateSubnet)
	mux.HandleFunc("GET /api/v1/subnets/{id}", a.handleGetSubnetByID)
	mux.HandleFunc("POST /api/v1/subnets/{id}/ips", a.handleCreateIPBySubnetID)
	mux.HandleFunc("GET /api/v1/subnets/{id}/ips", a.handleGetIPsBySubnetID)
	mux.HandleFunc("PATCH /api/v1/subnets/{id}/ips/{uuid}", a.handleUpdateIPByUUID)
	mux.HandleFunc("DELETE /api/v1/subnets/{id}/ips/{uuid}", a.handleDeleteIPByUUIDandSubnetID)

	return mux
}

func (a *API) subnetExists(ctx context.Context, id int64) (bool, netip.Prefix, error) {
	subnet, err := a.Queries.GetSubnetByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		a.Logger.DebugContext(ctx, "not found", "id", id, "err", err.Error())
		return false, netip.Prefix{}, nil
	}
	if err != nil {
		a.Logger.ErrorContext(ctx, "unexpected error", "err", err.Error())
		return false, netip.Prefix{}, err
	}
	return true, subnet.Cidr, nil
}

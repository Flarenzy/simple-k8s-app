package http

import (
	"log/slog"
	"net/http"

	sqlc "github.com/Flarenzy/simple-k8s-app/internal/db/sqlc"
	"github.com/jackc/pgx/v5/pgxpool"
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
	mux.HandleFunc("GET /api/v1/subnets", a.handleGetAllSubnets)
	mux.HandleFunc("POST /api/v1/subnets", a.handleCreateSubnet)
	mux.HandleFunc("GET /api/v1/subnets/{id}", a.handleGetSubnetByID)
	// mux.HandleFunc("/api/v1/ips", a.handleIPs)

	return mux
}

package http

import (
	"log/slog"
	"net/http"

	"github.com/Flarenzy/simple-k8s-app/internal/domain"
	"github.com/MicahParks/keyfunc/v3"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/swaggo/http-swagger" // http-swagger middleware
)

type API struct {
	Logger       *slog.Logger
	DB           *pgxpool.Pool
	Service      domain.NetworkService
	authEnabled  bool
	authIssuer   string
	authAudience string
	jwks         keyfunc.Keyfunc
}

func NewAPI(logger *slog.Logger, db *pgxpool.Pool, svc domain.NetworkService, authCfg AuthConfig) *API {
	a := &API{
		Logger:  logger,
		DB:      db,
		Service: svc,
	}
	a.initAuth(authCfg)
	return a
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
	mux.HandleFunc("DELETE /api/v1/subnets/{id}", a.handleDeleteSubnetByID)
	mux.HandleFunc("POST /api/v1/subnets/{id}/ips", a.handleCreateIPBySubnetID)
	mux.HandleFunc("GET /api/v1/subnets/{id}/ips", a.handleGetIPsBySubnetID)
	mux.HandleFunc("PATCH /api/v1/subnets/{id}/ips/{uuid}", a.handleUpdateIPByUUID)
	mux.HandleFunc("DELETE /api/v1/subnets/{id}/ips/{uuid}", a.handleDeleteIPByUUIDandSubnetID)

	return a.authMiddleware(mux)
}

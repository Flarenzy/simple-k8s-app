package http

import (
	"context"
	"log/slog"
	"net/http"

	apiauth "github.com/Flarenzy/simple-k8s-app/internal/auth"
	"github.com/Flarenzy/simple-k8s-app/internal/domain"
	"github.com/swaggo/http-swagger" // http-swagger middleware
)

type HealthChecker interface {
	Ping(ctx context.Context) error
}

type API struct {
	Logger        *slog.Logger
	Health        HealthChecker
	Service       domain.NetworkService
	Authenticator apiauth.Authenticator
}

func NewAPI(logger *slog.Logger, health HealthChecker, svc domain.NetworkService, authenticator apiauth.Authenticator) *API {
	return &API{
		Logger:        logger,
		Health:        health,
		Service:       svc,
		Authenticator: authenticator,
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
	mux.HandleFunc("DELETE /api/v1/subnets/{id}", a.handleDeleteSubnetByID)
	mux.HandleFunc("POST /api/v1/subnets/{id}/ips", a.handleCreateIPBySubnetID)
	mux.HandleFunc("GET /api/v1/subnets/{id}/ips", a.handleGetIPsBySubnetID)
	mux.HandleFunc("PATCH /api/v1/subnets/{id}/ips/{uuid}", a.handleUpdateIPByUUID)
	mux.HandleFunc("DELETE /api/v1/subnets/{id}/ips/{uuid}", a.handleDeleteIPByUUIDandSubnetID)

	return a.authMiddleware(mux)
}

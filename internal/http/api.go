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
	Logger             *slog.Logger
	Health             HealthChecker
	NetService         domain.NetworkService
	SitesService       domain.SitesService
	ImportService      domain.ImportService
	Authenticator      apiauth.Authenticator
	CORSAllowedOrigins []string
}

func NewAPI(logger *slog.Logger,
	health HealthChecker,
	netSvc domain.NetworkService,
	sitesSvc domain.SitesService,
	authenticator apiauth.Authenticator) *API {
	return NewAPIWithCORS(logger, health, netSvc, sitesSvc, authenticator, nil)
}

func NewAPIWithCORS(logger *slog.Logger,
	health HealthChecker,
	netSvc domain.NetworkService,
	sitesSvc domain.SitesService,
	authenticator apiauth.Authenticator,
	corsAllowedOrigins []string) *API {
	return &API{
		Logger:             logger,
		Health:             health,
		NetService:         netSvc,
		SitesService:       sitesSvc,
		Authenticator:      authenticator,
		CORSAllowedOrigins: corsAllowedOrigins,
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
	mux.HandleFunc("PATCH /api/v1/subnets/{id}", a.handleUpdateSubnet)
	mux.HandleFunc("PATCH /api/v1/subnets/{id}/site", a.handleAssignSubnetSite)
	mux.HandleFunc("DELETE /api/v1/subnets/{id}", a.handleDeleteSubnetByID)
	mux.HandleFunc("GET /api/v1/sites", a.handleGetAllSites)
	mux.HandleFunc("POST /api/v1/sites", a.handleCreateSite)
	mux.HandleFunc("GET /api/v1/sites/statistics", a.handleGetSiteStatistics)
	mux.HandleFunc("GET /api/v1/sites/{id}", a.handleGetSiteByID)
	mux.HandleFunc("PATCH /api/v1/sites/{id}", a.handleUpdateSite)
	mux.HandleFunc("DELETE /api/v1/sites/{id}", a.handleDeleteSiteByID)
	mux.HandleFunc("POST /api/v1/import/csv", a.handleImportCSV)
	mux.HandleFunc("POST /api/v1/subnets/{id}/ips", a.handleCreateIPBySubnetID)
	mux.HandleFunc("GET /api/v1/subnets/{id}/ips", a.handleGetIPsBySubnetID)
	mux.HandleFunc("PATCH /api/v1/subnets/{id}/ips/{uuid}", a.handleUpdateIPByUUID)
	mux.HandleFunc("DELETE /api/v1/subnets/{id}/ips/{uuid}", a.handleDeleteIPByUUIDandSubnetID)

	return a.corsMiddleware(a.authMiddleware(mux))
}

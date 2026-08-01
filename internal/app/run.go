package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	apiauth "github.com/Flarenzy/simple-k8s-app/internal/auth"
	appdb "github.com/Flarenzy/simple-k8s-app/internal/db"
	sqlcdb "github.com/Flarenzy/simple-k8s-app/internal/db/sqlc"
	"github.com/Flarenzy/simple-k8s-app/internal/domain"
	apihttp "github.com/Flarenzy/simple-k8s-app/internal/http"
	kubediscovery "github.com/Flarenzy/simple-k8s-app/internal/kubernetes"
	reportingrunner "github.com/Flarenzy/simple-k8s-app/internal/reporting"
)

type Config struct {
	Port                string
	DSN                 string
	ReadTimeout         time.Duration
	WriteTimeout        time.Duration
	AuthEnabled         bool
	Issuer              string
	Audience            string
	JWKSURL             string
	CORSAllowedOrigins  []string
	KubernetesDiscovery kubediscovery.Config
}

func parseCSV(value string) []string {
	var entries []string
	for _, entry := range strings.Split(value, ",") {
		if entry = strings.TrimSpace(entry); entry != "" {
			entries = append(entries, entry)
		}
	}
	return entries
}

var (
	listenFn = net.Listen
	serveFn  = Serve
)

func LoadConfig() (Config, error) {
	discoveryConfig, err := kubediscovery.ConfigFromEnv(os.Getenv)
	if err != nil {
		return Config{}, fmt.Errorf("load kubernetes discovery config: %w", err)
	}
	cfg := Config{
		DSN:                 os.Getenv("DB_CONN"),
		Port:                os.Getenv("PORT"),
		ReadTimeout:         3 * time.Second,
		WriteTimeout:        3 * time.Second,
		AuthEnabled:         os.Getenv("AUTH_ENABLED") == "true",
		Issuer:              os.Getenv("KEYCLOAK_ISSUER"),
		Audience:            os.Getenv("KEYCLOAK_AUDIENCE"),
		JWKSURL:             os.Getenv("KEYCLOAK_JWKS_URL"),
		CORSAllowedOrigins:  parseCSV(os.Getenv("CORS_ALLOWED_ORIGINS")),
		KubernetesDiscovery: discoveryConfig,
	}

	if cfg.DSN == "" {
		return Config{}, fmt.Errorf("missing required environment variable: DB_CONN")
	}
	if cfg.Port == "" {
		cfg.Port = "4040"
	}
	return cfg, nil
}

func Run(ctx context.Context, cfg Config) error {
	listener, err := listenFn("tcp", fmt.Sprintf(":%s", cfg.Port))
	if err != nil {
		return fmt.Errorf("listen on %q: %w", cfg.Port, err)
	}
	defer func() {
		if closeErr := listener.Close(); closeErr != nil {
			_, printerErr := fmt.Fprintf(os.Stderr, "error closing listener: %v\n", closeErr)
			if printerErr != nil {
				return
			}
		}
	}()

	return serveFn(ctx, cfg, listener)
}

func Serve(ctx context.Context, cfg Config, listener net.Listener) error {
	logger := slog.Default()
	pool, err := appdb.NewPool(ctx, cfg.DSN)
	if err != nil {
		return err
	}
	defer pool.Close()

	queries := sqlcdb.New(pool)
	subnetRepo := appdb.NewSubnetRepository(queries)
	ipRepo := appdb.NewIPRepository(queries)
	sitesRepo := appdb.NewSitesRepository(queries)
	discoveryRepo := appdb.NewKubernetesDiscoveryRepository(pool)
	reportingRepo := appdb.NewReportingRepository(queries)
	networkService := domain.NewLoggingNetworkService(logger, domain.NewNetworkServiceWithDiscovery(subnetRepo, ipRepo, sitesRepo, discoveryRepo))
	sitesService := domain.NewSitesService(sitesRepo)
	discoveryService := domain.NewKubernetesDiscoveryService(discoveryRepo)
	reportingService := domain.NewReportingService(reportingRepo, subnetRepo)
	authenticator, err := newAuthenticator(ctx, cfg)
	if err != nil {
		return fmt.Errorf("initialize authenticator: %w", err)
	}

	api := apihttp.NewAPIWithCORS(logger, pool, networkService, sitesService, authenticator, cfg.CORSAllowedOrigins)
	api.ImportService = domain.NewCSVImportService(sitesService, networkService)
	api.DiscoveryService = discoveryService
	api.ReportingService = reportingService
	go reportingrunner.NewRunner(reportingService, logger).Run(ctx)

	if cfg.KubernetesDiscovery.Enabled {
		client, clientErr := kubediscovery.NewClient(cfg.KubernetesDiscovery)
		if clientErr != nil {
			return fmt.Errorf("initialize kubernetes discovery client: %w", clientErr)
		}
		runner := kubediscovery.NewRunner(cfg.KubernetesDiscovery, client, discoveryService, logger)
		go runner.Run(ctx)
	}

	server := &http.Server{
		Addr:         listener.Addr().String(),
		Handler:      api.Router(),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		fmt.Printf("Serving server on %s\n", listener.Addr())
		if serveErr := server.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			errCh <- serveErr
		}
		close(errCh)
	}()

	select {
	case serveErr, ok := <-errCh:
		if ok && serveErr != nil {
			return serveErr
		}
		return nil
	case <-ctx.Done():
	}

	fmt.Println("\nShutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err = server.Shutdown(shutdownCtx); err != nil {
		return err
	}

	for serveErr := range errCh {
		if serveErr != nil {
			return serveErr
		}
	}

	return nil
}

func newAuthenticator(ctx context.Context, cfg Config) (apiauth.Authenticator, error) {
	return apiauth.NewKeycloakAuthenticator(ctx, apiauth.Config{
		Enabled:  cfg.AuthEnabled,
		Issuer:   cfg.Issuer,
		Audience: cfg.Audience,
		JWKSURL:  cfg.JWKSURL,
	})
}

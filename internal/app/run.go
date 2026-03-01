package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	apiauth "github.com/Flarenzy/simple-k8s-app/internal/auth"
	appdb "github.com/Flarenzy/simple-k8s-app/internal/db"
	sqlcdb "github.com/Flarenzy/simple-k8s-app/internal/db/sqlc"
	"github.com/Flarenzy/simple-k8s-app/internal/domain"
	apihttp "github.com/Flarenzy/simple-k8s-app/internal/http"
)

type Config struct {
	Port         string
	DSN          string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	AuthEnabled  bool
	Issuer       string
	Audience     string
	JWKSURL      string
}

func LoadConfig() Config {
	cfg := Config{
		DSN:          os.Getenv("DB_CONN"),
		Port:         os.Getenv("PORT"),
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		AuthEnabled:  os.Getenv("AUTH_ENABLED") == "true",
		Issuer:       os.Getenv("KEYCLOAK_ISSUER"),
		Audience:     os.Getenv("KEYCLOAK_AUDIENCE"),
		JWKSURL:      os.Getenv("KEYCLOAK_JWKS_URL"),
	}

	if cfg.DSN == "" {
		log.Fatal("missing required environment variable: DB_CONN")
	}
	if cfg.Port == "" {
		cfg.Port = "4040"
	}
	return cfg
}

func Run(ctx context.Context, cfg Config) error {

	logger := slog.Default()
	pool, err := appdb.NewPool(ctx, cfg.DSN)
	if err != nil {
		return err
	}
	defer pool.Close()

	queries := sqlcdb.New(pool)
	subnetRepo := appdb.NewSubnetRepository(queries)
	ipRepo := appdb.NewIPRepository(queries)
	networkService := domain.NewLoggingNetworkService(logger, domain.NewNetworkService(subnetRepo, ipRepo))
	authenticator, err := newAuthenticator(ctx, cfg)
	if err != nil {
		return fmt.Errorf("initialize authenticator: %w", err)
	}

	api := apihttp.NewAPI(logger, pool, networkService, authenticator)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      api.Router(),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}

	go func() {
		fmt.Printf("Serving server on port %s\n", cfg.Port)
		if err = server.ListenAndServe(); err != nil && !errors.Is(http.ErrServerClosed, err) {
			fmt.Printf("ListenAndServe error: %s\n", err)
		}
	}()

	<-ctx.Done()

	fmt.Println("\nShutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return server.Shutdown(shutdownCtx)
}

func newAuthenticator(ctx context.Context, cfg Config) (apiauth.Authenticator, error) {
	return apiauth.NewKeycloakAuthenticator(ctx, apiauth.Config{
		Enabled:  cfg.AuthEnabled,
		Issuer:   cfg.Issuer,
		Audience: cfg.Audience,
		JWKSURL:  cfg.JWKSURL,
	})
}

package api

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	appdb "github.com/Flarenzy/simple-k8s-app/internal/db"
	sqlcdb "github.com/Flarenzy/simple-k8s-app/internal/db/sqlc"
	apihttp "github.com/Flarenzy/simple-k8s-app/internal/http"
)

type Config struct {
	Port         string
	DSN          string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

func LoadConfig() Config {
	cfg := Config{
		DSN:          os.Getenv("DB_CONN"),
		Port:         os.Getenv("PORT"),
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
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
	dsn := os.Getenv("DB_CONN")
	if dsn == "" {
		dsn = "postgres://ipam:ipam@host:5432/ipam?sslmode=disable"
	}
	pool, err := appdb.NewPool(ctx, cfg.DSN)
	if err != nil {
		return err
	}
	defer pool.Close()

	queries := sqlcdb.New(pool)

	api := apihttp.NewAPI(logger, pool, queries)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      api.Router(),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}

	go func() {
		fmt.Printf("Serving server on port %s\n", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("ListenAndServe error: %s\n", err)
		}
	}()

	<-ctx.Done()

	fmt.Println("\nShutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return server.Shutdown(shutdownCtx)
}

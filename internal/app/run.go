package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	appdb "github.com/Flarenzy/simple-k8s-app/internal/db"
	sqlcdb "github.com/Flarenzy/simple-k8s-app/internal/db/sqlc"
	apihttp "github.com/Flarenzy/simple-k8s-app/internal/http"
)

type Config struct {
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

func LoadConfig() Config {
	return Config{
		Port:         4040,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}
}

func Run(ctx context.Context, cfg Config) error {

	logger := slog.Default()
	dsn := ""

	pool, err := appdb.NewPool(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()

	queries := sqlcdb.New(pool)

	api := apihttp.NewAPI(logger, pool, queries)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      api.Router(),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}

	go func() {
		fmt.Printf("Serving server on port %d\n", cfg.Port)
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

package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/Flarenzy/simple-k8s-app/internal/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	cfg := api.LoadConfig()

	if err := api.Run(ctx, cfg); err != nil {
		log.Fatalf("server exited: %v", err)
	}
}

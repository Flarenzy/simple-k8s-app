package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/Flarenzy/simple-k8s-app/docs"
	"github.com/Flarenzy/simple-k8s-app/internal/app"
)

//	@title			Simple IPAM API
//	@version		1.0
//	@description	This is a simple ipam server.
//	@termsOfService	http://swagger.io/terms/

//	@contact.name	API Support
//	@contact.url	http://www.swagger.io/support
//	@contact.email	support@swagger.io

//	@license.name	Apache 2.0
//	@license.url	http://www.apache.org/licenses/LICENSE-2.0.html

//	@host		localhost:4040
//	@BasePath	/

//	@securityDefinitions.basic	BasicAuth

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Swagger served at /swagger; use same host as the request origin to avoid CORS/host mismatches.
	docs.SwaggerInfo.Host = ""
	docs.SwaggerInfo.BasePath = "/"
	docs.SwaggerInfo.Schemes = []string{"http"}

	cfg := api.LoadConfig()

	if err := api.Run(ctx, cfg); err != nil {
		log.Fatalf("server exited: %v", err)
	}
}

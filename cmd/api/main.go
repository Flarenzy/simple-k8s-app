package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	
	"github.com/Flarenzy/simple-k8s-app/internal/app"
	_ "github.com/Flarenzy/simple-k8s-app/docs"
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
//	@BasePath	/api/v1

//	@securityDefinitions.basic	BasicAuth

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	cfg := api.LoadConfig()

	if err := api.Run(ctx, cfg); err != nil {
		log.Fatalf("server exited: %v", err)
	}
}

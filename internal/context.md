# Backend Context

The backend is organized as a small layered Go service:

- `internal/app` loads configuration, opens PostgreSQL, constructs repositories and services, and serves HTTP.
- `internal/domain` owns models, validation, repository contracts, and network/site business services.
- `internal/db` adapts SQLC queries to domain repository interfaces.
- `internal/http` exposes health, readiness, authentication, CORS, Swagger, subnet, and IP endpoints.
- `internal/auth` contains the optional Keycloak/JWT boundary.

The application is started by `cmd/api/main.go`. Use CodeGraph to trace symbols such as `Serve`, `NewAPI`, `NewNetworkService`, or `NewSitesService` before changing cross-layer wiring.

# API Entrypoint Context

`main.go` is the production process entrypoint. It creates the application context, loads environment-based configuration, and delegates lifecycle management to `internal/app`.

Keep process concerns here minimal. Runtime wiring, database initialization, authentication setup, HTTP server construction, and graceful shutdown belong in `internal/app/run.go`.

`internal/app` also starts the database-coordinated subnet usage snapshot runner; it is independent of Kubernetes discovery.

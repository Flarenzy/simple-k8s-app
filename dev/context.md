# Local Development Context

`dev/docker-compose.yaml` supplies local infrastructure, primarily PostgreSQL and Keycloak, with realm fixtures in this directory. The Makefile starts the backend on port 4040 and the Vite frontend on port 5173, using the local Keycloak realm when authentication is enabled.

Use `make dev-up` for infrastructure, `make db-migrate` for schema setup, and `make run` for the application. Keep local realm client settings synchronized with frontend and API environment variables.

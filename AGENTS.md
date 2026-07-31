# Repository Guide

This repository contains IPAM, a small network inventory application. The Go backend exposes a JSON HTTP API for subnets and IP addresses, persists data in PostgreSQL through SQLC-generated queries, and optionally authenticates requests with Keycloak. The React/Vite frontend consumes the API and provides the browser UI for subnet and IP management. Sites are represented in the database, repository, domain service, and application wiring; site API and UI integration is part of the current feature work.

## Working with the code

- Use CodeGraph before searching broadly or reading unfamiliar code. From the repository root, run `codegraph explore "<symbol or architecture question>"`. The `.codegraph/` index is part of the local development context.
- Read the nearest `context.md` before changing code in a documented area. The context files describe boundaries, data flow, and validation commands.
- Preserve generated SQLC and Swagger output conventions when changing their sources. Use the Makefile targets for generation and verification.
- Keep behavior self-explanatory; do not add explanatory comments to production code when clear names and structure are sufficient.
- Use `make test` for the Go unit suite, `make test-integration` for container-backed integration tests, and `npm run build` in `frontend` for the frontend build.

## Context map

- [Backend overview](internal/context.md)
- [Application entrypoint](cmd/api/context.md)
- [Domain layer](internal/domain/context.md)
- [HTTP API](internal/http/context.md)
- [Persistence and SQLC](internal/db/context.md)
- [Database schema and queries](db/context.md)
- [Frontend](frontend/context.md)
- [Deployment assets](deploy/context.md)
- [Helm chart](deploy/helm/ipam/context.md)
- [Local development stack](dev/context.md)
- [Integration tests](integration/context.md)

README.md and the Makefile remain the authoritative references for the complete local setup and command details.

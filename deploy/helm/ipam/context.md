# IPAM Helm Chart Context

This chart deploys three application workloads: API, frontend, and migrations. It also supports the PostgreSQL and optional Keycloak dependencies. `values.yaml` is the main configuration surface; templates derive names and labels through `_helpers.tpl`.

The API is exposed on port 4040 and the frontend on port 80. Ingress and HTTPRoute templates define how `/api` and browser traffic reach those services; when Keycloak is enabled, Gateway API also renders its browser route. Keycloak master bootstrap and IPAM application-admin credentials must come from separate Secrets. Render the chart before committing template changes and verify probes, secrets, and environment variable names against the Go and frontend configuration.

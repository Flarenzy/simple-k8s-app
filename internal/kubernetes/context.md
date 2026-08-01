# Kubernetes Discovery Context

This package owns outbound Kubernetes configuration, the official client-go adapter, Service-to-snapshot transformation, and the optional periodic runner. It does not persist observations directly: complete snapshots cross the domain contract into `internal/db`, where source locking, site-scoped matching, and atomic publication occur.

Discovery is not part of API health or readiness. Keep authentication explicit (`in_cluster` or a named kubeconfig path/context), never resolve observed hostnames, and never add IPAM write behavior to this package. Validate changes with `go test ./internal/kubernetes` and the PostgreSQL-backed discovery journey in `make test-integration`.

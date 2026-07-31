# Domain Context

This package defines the application vocabulary and contracts used by adapters. `network_service.go` handles subnet/IP behavior, while `sites_service.go` handles site CRUD and aggregated site statistics. Repository interfaces in `repositories.go` keep the domain independent of PostgreSQL and SQLC.

Changes here should preserve validation and domain error semantics consumed by HTTP handlers and tests. Trace interfaces and implementations with CodeGraph before changing method signatures.

# Database Context

PostgreSQL schema migrations are applied with Goose from `db/migrations`. SQLC input queries are stored in `db/queries` and generate Go code under `internal/db/sqlc` according to `sqlc.yaml`.

Sites have their own table and queries, and subnets can reference a site. Migration order matters; new schema changes should be additive and tested against the integration database path.

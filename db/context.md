# Database Context

PostgreSQL schema migrations are applied with Goose from `db/migrations`. SQLC input queries are stored in `db/queries` and generate Go code under `internal/db/sqlc` according to `sqlc.yaml`.

Sites have their own table and queries, and subnets can reference a site. Migration order matters; new schema changes should be additive and tested against the integration database path.

Subnet usage reporting stores a singleton cadence/retention policy and immutable IPv4 usage snapshots. `CaptureDueSubnetUsageSnapshots` advances the singleton timestamp and inserts one complete due run atomically, preventing duplicate runs across API replicas; Kubernetes observation tables are not reporting inputs.

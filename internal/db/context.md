# Persistence Context

`internal/db` is the PostgreSQL adapter. `db.go` creates the connection pool, repository files map SQLC results to domain models, and `sqlc/` contains generated query code and database models.

SQL statements live under `db/queries`; schema changes live under `db/migrations`. Regenerate SQLC through the Makefile after query changes and keep repository behavior covered by the existing database tests where practical.

`reporting_repository.go` maps the singleton reporting policy and periodic subnet usage snapshots. Snapshot capture and retention cleanup are SQLC queries; history is based only on stored snapshots, never reconstructed from current IP rows.

// Package database provides the persistence layer for the Kilnx
// runtime: connection management, schema migration, and dialect-aware
// query execution against SQLite or PostgreSQL.
//
// The package is organized around three concerns:
//
//   - Driver (driver.go, driver_postgres.go): opens connections from a
//     DATABASE_URL and exposes a *sql.DB.
//   - Dialect (dialect.go, dialect_sqlite.go, dialect_postgres.go):
//     emits dialect-specific SQL for CREATE TABLE, ALTER, parameter
//     binding, and identifier quoting.
//   - Migration (database.go): derives the target schema from the
//     parser AST, diffs it against the live schema, and applies the
//     necessary CREATE/ALTER statements transactionally. Migration
//     history is recorded in a kilnx-managed metadata table.
//     Read-only drift detection (DetectColumnDrift) reports columns
//     whose live schema diverges from the model along orphan, type,
//     NOT NULL, UNIQUE, and DEFAULT-presence axes. Drift is advisory:
//     PlanMigration is intentionally additive and never emits ALTER
//     or DROP COLUMN to reconcile.
//
// Query execution helpers live in query.go. Public types in
// interfaces.go define what the runtime depends on so tests can
// substitute fakes.
package database

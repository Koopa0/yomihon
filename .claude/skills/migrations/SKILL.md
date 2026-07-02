---
name: migrations
description: >-
  Database migration patterns for golang-migrate with PostgreSQL — numbered
  up/down file structure, CLI and in-Go/testcontainers execution, safe vs
  dangerous DDL, zero-downtime expand-contract patterns, NOT NULL and index
  strategies, data backfill scripts, and team coordination (numbering,
  review checklist, rollback strategy).
when_to_use: >-
  Use when creating or reviewing files in migrations/, adding or renaming a
  table, column, or index, running or rolling back migrations, fixing a dirty
  migration state, backfilling data, or planning a zero-downtime schema change.
  Trigger keywords: "migration", "golang-migrate", "migrate up/down", "schema
  change", "ALTER TABLE", "CREATE INDEX CONCURRENTLY", "backfill", "rollback".
user_invocable: true
metadata:
  author: koopa
  version: "1.0"
  lang: go
---

# /migrations — golang-migrate Patterns for PostgreSQL

This project uses [golang-migrate](https://github.com/golang-migrate/migrate) for database schema migrations.

## Safe vs Dangerous DDL — Decide First

Before writing any `ALTER TABLE`, classify the operation. Anything in the DANGEROUS column requires a multi-step zero-downtime plan (see `references/zero-downtime.md`).

| Operation | Safe? | Why / What to do instead |
|-----------|-------|--------------------------|
| `CREATE TABLE` | SAFE | New table, no existing rows to lock |
| `ADD COLUMN` (nullable) | SAFE | No lock, no rewrite |
| `ADD COLUMN ... NOT NULL DEFAULT x` | SAFE | Default avoids full row rewrite |
| `CREATE INDEX CONCURRENTLY` | SAFE | No table lock (cannot run in a transaction — see below) |
| `RENAME COLUMN` | CAREFUL | Breaks sqlc — run `sqlc generate` + fix Go code after |
| `ALTER COLUMN ... SET NOT NULL` | DANGEROUS | Locks table, scans all rows → use `CHECK ... NOT VALID` then `VALIDATE CONSTRAINT` |
| `ALTER COLUMN ... TYPE ...` | DANGEROUS | Full table scan + rewrite → use expand-contract |
| `DROP COLUMN` | DANGEROUS | Data loss → use expand-contract, drop only after code migrated |
| `CREATE INDEX` (non-concurrent) | DANGEROUS | Locks table during build → use `CONCURRENTLY` |

IMPORTANT: `CREATE INDEX CONCURRENTLY` cannot run inside a transaction. golang-migrate runs each file in a transaction by default. Split index creation into a separate migration file run manually (see `references/safe-ddl.md` and `references/zero-downtime.md`).

## MUST / NEVER

- MUST: every `.up.sql` has a corresponding `.down.sql`
- MUST: sequential numbering `NNN_description.{up,down}.sql`, lowercase underscore-separated description
- MUST: one logical change per migration (don't mix table creation with data changes)
- MUST: NOT NULL columns have a `DEFAULT`
- MUST: indexes on foreign-key columns
- MUST: run `sqlc generate` + `go build ./...` after any column/table rename
- NEVER: large data backfills inside a migration — migrations must be fast and atomic; use a separate `cmd/backfill` script
- NEVER: `DROP TABLE` / `DROP COLUMN` in a down migration for non-reversible changes — make the down a no-op or raise instead
- NEVER: modify an already-applied migration — create a new one

## Migration File Structure

```
migrations/
  001_create_users.up.sql
  001_create_users.down.sql
  002_create_orders.up.sql
  002_create_orders.down.sql
  003_add_order_status.up.sql
  003_add_order_status.down.sql
```

Rules:
- Sequential numbering: `NNN_description.{up,down}.sql`
- Every `.up.sql` MUST have a corresponding `.down.sql`
- Description is lowercase, underscore-separated, describes the change
- One logical change per migration (don't mix table creation with data changes)

## Creating a New Migration

```bash
migrate create -ext sql -dir migrations -seq <description>
```

This creates the pair:
```
migrations/NNN_<description>.up.sql
migrations/NNN_<description>.down.sql
```

Install the CLI:
```bash
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

## Running Migrations (CLI)

```bash
# Apply all pending migrations
migrate -database "${DATABASE_URL}" -path migrations up

# Apply N migrations
migrate -database "${DATABASE_URL}" -path migrations up N

# Rollback last migration
migrate -database "${DATABASE_URL}" -path migrations down 1

# Rollback all migrations
migrate -database "${DATABASE_URL}" -path migrations down

# Go to specific version
migrate -database "${DATABASE_URL}" -path migrations goto V

# Check current version
migrate -database "${DATABASE_URL}" -path migrations version

# Force a version (for fixing dirty state)
migrate -database "${DATABASE_URL}" -path migrations force V
```

## Creating a Table (most-used DDL)

```sql
-- up
CREATE TABLE orders (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id),
    total       BIGINT NOT NULL CHECK (total > 0),
    status      TEXT NOT NULL DEFAULT 'pending',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_orders_user_id ON orders(user_id);
CREATE INDEX idx_orders_status ON orders(status) WHERE status != 'completed';

-- down
DROP TABLE IF EXISTS orders;
```

## Common Pitfalls

| Pitfall | Solution |
|---------|----------|
| "Dirty database version" | Migration failed halfway. Fix the SQL, then `migrate force V` to the last clean version |
| `CREATE INDEX CONCURRENTLY` fails in transaction | Use a separate migration or run manually outside of migrate |
| Down migration drops data | Only use `DROP TABLE` / `DROP COLUMN` in down migrations for truly reversible changes. For data-loss operations, the down migration should be a no-op or raise an error |
| Migration path wrong in tests | Use relative path from test file location to `migrations/` directory |
| Conflicting migration numbers | Coordinate with team. Sequential numbering means merge conflicts on numbers. Consider timestamp-based naming for teams |

## Migration Review Checklist

Before merging a migration:

- [ ] Has corresponding `.down.sql`
- [ ] Down migration is reversible (or documented as irreversible)
- [ ] No table locks on large tables (use CONCURRENTLY for indexes)
- [ ] NOT NULL columns have DEFAULT values
- [ ] Foreign keys reference existing tables
- [ ] Indexes are created for foreign key columns
- [ ] No data backfill in migration (use separate script)
- [ ] Tested locally with `migrate up` and `migrate down`

## Navigation

| Topic | File |
|-------|------|
| Safe DDL recipes (add column, NOT NULL column, concurrent index, rename, dangerous ops) | `references/safe-ddl.md` |
| Zero-downtime expand-contract, safe column rename, NOT NULL on existing column, large-table index, data backfill (ON CONFLICT, batch script, progress tracking) | `references/zero-downtime.md` |
| Execution (Go startup, testcontainers, Makefile), team coordination (naming, review checklist, rollback), local + CI testing | `references/execution-and-rollback.md` |

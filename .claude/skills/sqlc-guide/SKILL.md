---
name: sqlc-guide
description: >-
  sqlc configuration and usage for Go with pgx/v5 — sqlc.yaml setup
  (per-feature generation, type overrides), query annotations (:one,
  :many, :exec, :execrows, :copyfrom, batch), sqlc.arg/narg/embed/slice,
  the DBTX interface, nullable column handling (pointers vs pgtype),
  JOIN patterns, CTEs, COALESCE conditional updates, testing generated
  queries, and common pitfalls.
when_to_use: >-
  Use when writing or editing query.sql files, configuring sqlc.yaml,
  running sqlc generate, choosing a query annotation, adding named or
  nullable parameters, dynamic IN lists with sqlc.slice, bulk inserts
  with :copyfrom, batch operations, mapping JOIN results to structs,
  handling nullable columns, or debugging sqlc generation errors and
  schema drift between migrations and queries.
user_invocable: true
metadata:
  author: koopa
  version: "1.0"
  lang: go
---

# sqlc Guide

## Common Pitfalls

1. **Missing schema**: sqlc needs the schema to validate queries. Point `schema:` to your migrations directory.
2. **Column ambiguity**: Use table aliases in JOINs to avoid ambiguous column references.
3. **NULL handling**: Use `sqlc.narg()` for nullable parameters, not `sqlc.arg()`.
4. **Array types**: Use `pgtype.Array[T]` for PostgreSQL array columns.
5. **RETURNING ***: Returns all columns — ensure your struct matches.
6. **Enum types**: Define PostgreSQL enums in schema and sqlc will generate Go constants.
7. **`:exec` loses rows affected for Delete**: `:exec` returns only `error` — callers cannot distinguish "row found and deleted" from "no matching row". If the HTTP handler must emit 404 for a missing target, use `:execrows` instead and map `rows == 0` to `ErrNotFound` in the store. Using `:exec` for Delete leaks a silent 204-on-missing-id bug that lint and build do not catch.

Detailed pitfalls with code examples: see "Common Pitfalls — Detailed" below.

## Query Annotations

### :one — Returns exactly one row

```sql
-- name: OrderByID :one
SELECT id, status, total, created_at, updated_at
FROM orders
WHERE id = $1;
```

Generated Go:

```go
func (q *Queries) OrderByID(ctx context.Context, id string) (Order, error)
```

### :many — Returns multiple rows

```sql
-- name: OrdersByUserID :many
SELECT id, status, total, created_at
FROM orders
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;
```

Generated Go:

```go
func (q *Queries) OrdersByUserID(ctx context.Context, arg OrdersByUserIDParams) ([]Order, error)
```

### :exec — No return value

```sql
-- name: DeleteOrder :exec
DELETE FROM orders WHERE id = $1;
```

Generated Go:

```go
func (q *Queries) DeleteOrder(ctx context.Context, id string) error
```

### :execrows — Returns affected row count

```sql
-- name: SoftDeleteExpiredOrders :execrows
UPDATE orders
SET deleted_at = now()
WHERE status = 'expired' AND deleted_at IS NULL;
```

Generated Go:

```go
func (q *Queries) SoftDeleteExpiredOrders(ctx context.Context) (int64, error)
```

### :execresult — Returns pgconn.CommandTag

```sql
-- name: UpdateOrderStatus :execresult
UPDATE orders SET status = $1, updated_at = now() WHERE id = $2;
```

### :copyfrom — Bulk insert

```sql
-- name: BulkInsertOrders :copyfrom
INSERT INTO orders (id, user_id, status, total, created_at)
VALUES ($1, $2, $3, $4, $5);
```

Batch annotations (`:batchone`, `:batchmany`, `:batchexec`): see `references/advanced-queries.md`.

## Named Parameters

### sqlc.arg() — Required parameter

```sql
-- name: CreateOrder :one
INSERT INTO orders (id, user_id, status, total, created_at)
VALUES (
    gen_random_uuid(),
    sqlc.arg(user_id),
    sqlc.arg(status),
    sqlc.arg(total),
    now()
)
RETURNING *;
```

### sqlc.narg() — Nullable parameter

```sql
-- name: UpdateOrder :one
UPDATE orders
SET
    status = COALESCE(sqlc.narg(status), status),
    total = COALESCE(sqlc.narg(total), total),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;
```

## Configuration (sqlc.yaml)

```yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "internal/**/query.sql"  # recursive — queries live in internal/<feature>/query.sql
    schema: "migrations/"
    gen:
      go:
        package: "db"
        out: "internal/db"
        sql_package: "pgx/v5"
        emit_json_tags: true
        emit_empty_slices: true
        emit_result_struct_pointers: false
        emit_params_struct_pointers: false
        overrides:
          - db_type: "uuid"
            go_type: "string"
          - db_type: "timestamptz"
            go_type: "time.Time"
          - db_type: "jsonb"
            go_type:
              import: "encoding/json"
              type: "RawMessage"
            nullable: true
```

Per-feature generation, type overrides, nullable column handling, and query file organization: see `references/configuration.md`.

## Using with pgxpool.Pool

sqlc generates a `Queries` type that accepts `DBTX` interface:

```go
pool, _ := pgxpool.New(ctx, databaseURL)
queries := db.New(pool) // pass pool directly

// For transactions
tx, _ := pool.Begin(ctx)
defer tx.Rollback(ctx)
qtx := queries.WithTx(tx)
// use qtx for transactional queries
tx.Commit(ctx)
```

## DBTX Interface

sqlc generates a `DBTX` interface that both `*pgxpool.Pool` and `pgx.Tx` satisfy:

```go
// Generated by sqlc
type DBTX interface {
    Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error)
    Query(context.Context, string, ...interface{}) (pgx.Rows, error)
    QueryRow(context.Context, string, ...interface{}) pgx.Row
}

type Queries struct {
    db DBTX
}

func New(db DBTX) *Queries {
    return &Queries{db: db}
}
```

This enables the Store pattern in our project:

```go
type Store struct {
    q *db.Queries
}

func NewStore(dbtx db.DBTX) *Store {
    return &Store{q: db.New(dbtx)}
}

func (s *Store) WithTx(tx pgx.Tx) *Store {
    return &Store{q: db.New(tx)}
}
```

## Running sqlc

```bash
# Generate code
sqlc generate

# Verify queries without generating
sqlc vet

# Check for differences
sqlc diff
```

## Common Pitfalls — Detailed

### 1. Schema Out of Sync

```bash
# After migration changes
sqlc generate  # MUST regenerate
go build ./... # verify compilation
```

### 2. Column Name Mismatch in JOINs

```sql
-- WRONG: ambiguous 'id'
SELECT id, name FROM users u JOIN orders o ON u.id = o.user_id;

-- CORRECT: use aliases
SELECT u.id as user_id, u.name, o.id as order_id FROM users u JOIN orders o ON u.id = o.user_id;

-- OR use sqlc.embed()
SELECT sqlc.embed(u), sqlc.embed(o) FROM users u JOIN orders o ON u.id = o.user_id;
```

### 3. Forgetting COALESCE for Aggregates

```sql
-- WRONG: returns NULL if no rows
SELECT SUM(total_cents) FROM orders WHERE user_id = $1;

-- CORRECT: returns 0 if no rows
SELECT COALESCE(SUM(total_cents), 0) FROM orders WHERE user_id = $1;
```

### 4. Missing Index for Query Patterns

```sql
-- This query pattern:
-- name: PendingOrdersByUser :many
SELECT * FROM orders WHERE user_id = $1 AND status = 'pending';

-- Needs this index:
CREATE INDEX idx_orders_user_status ON orders(user_id, status);
```

### 5. Using * with RETURNING

```sql
-- Be careful: RETURNING * returns all columns
-- If table schema changes, generated struct changes
-- Prefer explicit column list for stability:
INSERT INTO orders (user_id, status) VALUES ($1, $2)
RETURNING id, user_id, status, created_at, updated_at;
```

### 6. N+1 Queries

```go
// WRONG: N+1 queries
orders, _ := queries.OrdersByUser(ctx, userID)
for _, o := range orders {
    items, _ := queries.OrderItems(ctx, o.ID) // N queries!
}

// CORRECT: Use batch or single query with JOIN
items, _ := queries.OrderItemsByUser(ctx, userID) // 1 query
// Or use batch:
br := queries.OrderItemsBatch(ctx, orderIDs)
```

### 7. Missing emit_empty_slices

```yaml
# Without this, empty results return nil slice
gen:
  go:
    emit_empty_slices: true  # returns [] instead of nil
```

This matters for JSON: `[]` vs `null`.

## Navigation

Deep material lives in `references/` — read the file matching your task:

| Topic | File | When to read |
|-------|------|--------------|
| Per-feature generation, type overrides, nullable columns (pointers vs pgtype), query file organization | `references/configuration.md` | Setting up or changing sqlc.yaml beyond the base config |
| sqlc.embed(), sqlc.slice(), batch operations (:batchone/:batchmany/:batchexec), CTEs, subqueries, COALESCE conditional updates, JOIN patterns | `references/advanced-queries.md` | Writing complex queries: JOINs, dynamic IN lists, bulk/batch, partial updates |
| sqlc vet, integration tests with testcontainers-go | `references/testing.md` | Validating queries and testing generated code |

---
name: postgres-patterns
description: >-
  PostgreSQL reference for Go services — preferred data types (UUID,
  TIMESTAMPTZ, BIGINT cents, JSONB), schema templates and soft delete,
  B-tree/partial/GIN indexing, migration file conventions, UPSERT, CTEs,
  cursor pagination, advisory locks, EXPLAIN ANALYZE, 3NF normalization
  and denormalization criteria, OLTP vs OLAP design, trigger/view/JSONB
  caution rules, and Go-SQL type mapping.
when_to_use: >-
  Use when designing a PostgreSQL schema or table, choosing column types,
  writing CREATE TABLE or CREATE INDEX, adding indexes (partial, GIN,
  concurrent), writing migration SQL, doing UPSERT / INSERT ON CONFLICT,
  implementing cursor-based pagination, analyzing slow queries with
  EXPLAIN ANALYZE, deciding JSONB vs proper columns, evaluating triggers,
  views, or materialized views, or answering normalization questions.
user_invocable: true
metadata:
  author: koopa
  version: "1.0"
  lang: go
---

# PostgreSQL Patterns Reference

## Data Types

### Preferred Types

| Use Case | PostgreSQL Type | Go Type |
|----------|----------------|---------|
| Primary key | `UUID` + `gen_random_uuid()` | `string` |
| Timestamps | `TIMESTAMPTZ` | `time.Time` |
| Money/cents | `BIGINT` (store as cents) | `int64` |
| Status/enum | `TEXT` + CHECK constraint | `string` |
| JSON data | `JSONB` | `json.RawMessage` |
| Boolean | `BOOLEAN` | `bool` |
| Short text | `TEXT` (no VARCHAR limit) | `string` |
| Integer | `INTEGER` or `BIGINT` | `int32` / `int64` |

### Avoid

- `TIMESTAMP` without time zone — always use `TIMESTAMPTZ`
- `VARCHAR(n)` — just use `TEXT` (PostgreSQL treats them the same internally)
- `SERIAL` — use `gen_random_uuid()` or `GENERATED ALWAYS AS IDENTITY`
- `MONEY` type — use `BIGINT` in cents
- `ENUM` type — use `TEXT` with CHECK constraint (easier to migrate)

## PostgreSQL Features — Use Judiciously

PostgreSQL has many advanced features. Use them when appropriate, not just because they exist.

### Use Freely

- **CHECK constraints**: Enforce data validity
- **Foreign keys**: Enforce referential integrity
- **Partial indexes**: Optimize specific query patterns
- **UUID generation**: `gen_random_uuid()` built-in

### Use When Needed

- **Advisory locks**: Distributed coordination
- **CTEs**: Complex query composition (but watch performance)
- **Window functions**: Analytics queries
- **LATERAL joins**: Row-by-row subqueries

### Use with Caution

- **Triggers**: Only for audit/infrastructure (see references/schema-design.md)
- **Rules**: Almost never — use triggers if you must
- **Inheritance**: Prefer partitioning instead
- **Custom types**: `CREATE TYPE` adds complexity — use sparingly

### Evaluate Carefully Before Using

- **Extensions** (pg_trgm, ltree, PostGIS): Add only when genuinely needed
- **Generated columns**: Nice but adds implicit behavior
- **Row-level security**: Powerful but complex — ensure you need it
- **Table partitioning**: Only for tables with millions of rows

### Avoid

- **LISTEN/NOTIFY** for critical workflows — use proper message queues
- **Stored procedures** for business logic — keep logic in Go
- **PL/pgSQL functions** beyond simple helpers — complexity hides in the DB

## Anti-Patterns

- Don't use `OFFSET` for deep pagination — use cursor-based pagination
- Don't create indexes on every column — only on columns used in WHERE/JOIN/ORDER BY
- Don't use `SELECT *` in production queries — specify columns
- Don't forget to ANALYZE after bulk data changes
- Don't use `TEXT` arrays for relationships — normalize into a junction table
- Don't use recursive CTEs for simple hierarchies — use `ltree` extension
- Don't use triggers for business logic — keep it in Go
- Don't use JSONB to avoid schema design — normalize first
- Don't add extensions "just in case" — add when needed
- Don't denormalize without documented justification

## Schema Patterns

### Table Template

```sql
CREATE TABLE orders (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id),
    status      TEXT NOT NULL DEFAULT 'pending'
                CHECK (status IN ('pending', 'confirmed', 'shipped', 'delivered', 'cancelled')),
    total_cents BIGINT NOT NULL CHECK (total_cents >= 0),
    metadata    JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### Soft Delete

```sql
ALTER TABLE orders ADD COLUMN deleted_at TIMESTAMPTZ;

-- Partial index for active records only
CREATE INDEX idx_orders_active ON orders(created_at) WHERE deleted_at IS NULL;
```

## Indexing

### B-tree (Default)

```sql
-- Single column
CREATE INDEX idx_orders_user_id ON orders(user_id);

-- Composite (column order matters — most selective first for equality, range last)
CREATE INDEX idx_orders_user_status ON orders(user_id, status);

-- Unique
CREATE UNIQUE INDEX idx_users_email ON users(email);
```

### Partial Index

```sql
-- Only index active orders
CREATE INDEX idx_orders_pending ON orders(created_at)
WHERE status = 'pending';
```

### GIN (for JSONB)

```sql
CREATE INDEX idx_orders_metadata ON orders USING gin(metadata);

-- Query: WHERE metadata @> '{"priority": "high"}'
```

### Concurrent Index Creation (for production)

```sql
CREATE INDEX CONCURRENTLY idx_orders_user_id ON orders(user_id);
```

## Query Patterns

For CTEs, advisory locks, and EXPLAIN ANALYZE, read references/queries-and-performance.md.

### UPSERT (INSERT ON CONFLICT)

```sql
INSERT INTO users (id, email, name, updated_at)
VALUES ($1, $2, $3, now())
ON CONFLICT (email)
DO UPDATE SET
    name = EXCLUDED.name,
    updated_at = now()
RETURNING *;
```

### Cursor-Based Pagination

```sql
-- First page
SELECT id, created_at, status
FROM orders
WHERE user_id = $1
ORDER BY created_at DESC, id DESC
LIMIT $2;

-- Next page (pass last row's created_at and id)
SELECT id, created_at, status
FROM orders
WHERE user_id = $1
  AND (created_at, id) < ($3, $4)
ORDER BY created_at DESC, id DESC
LIMIT $2;
```

## Go-SQL Type Mapping

| PostgreSQL | Go | sqlc Config |
|------------|-----|-------------|
| `UUID` | `string` | default |
| `TEXT` | `string` | default |
| `INTEGER` | `int32` | default |
| `BIGINT` | `int64` | default |
| `BOOLEAN` | `bool` | default |
| `TIMESTAMPTZ` | `time.Time` | default |
| `JSONB` | `json.RawMessage` | `emit_json_tags: true` |
| `NUMERIC` | `pgtype.Numeric` | requires handling |
| `UUID[]` | `[]string` | configure in sqlc.yaml |
| `BYTEA` | `[]byte` | default |

### Nullable Columns

```yaml
# sqlc.yaml
sql:
  - schema: "migrations/*.sql"
    queries: "internal/**/query.sql"
    engine: "postgresql"
    gen:
      go:
        package: "db"
        out: "internal/db"
        sql_package: "pgx/v5"
        emit_json_tags: true
        emit_pointers_for_null_types: true  # NULL -> *T
```

## Navigation

Deep material lives in references/. Read the file matching your task:

| Topic | File | What's Inside |
|-------|------|---------------|
| Schema design doctrine | references/schema-design.md | 3NF normalization, when denormalization is acceptable, OLTP vs OLAP design, trigger caution rules, view usage rules, JSONB doctrine and efficient JSONB querying |
| Query analysis & coordination | references/queries-and-performance.md | CTE composition, advisory locks (session/transaction/try), EXPLAIN ANALYZE and how to read its output |
| Migration files | references/migrations.md | Migration file naming convention, up/down migration templates |

- Designing or reviewing a schema, deciding JSONB vs columns, evaluating a trigger or view → read references/schema-design.md
- Analyzing a slow query or coordinating across processes → read references/queries-and-performance.md
- Writing migration SQL files → read references/migrations.md

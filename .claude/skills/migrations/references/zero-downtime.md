# Zero-Downtime Migration Patterns

## Expand-Contract Pattern

For schema changes that would break running code, use a multi-phase approach:

**Phase 1: Expand** (add new, keep old)
```sql
-- Migration N: Add new column, keep old
ALTER TABLE orders ADD COLUMN status_v2 TEXT;
```

**Phase 2: Migrate** (code writes to both, reads from new)
```go
// Application code: write to both columns
_, err := tx.Exec(ctx, `UPDATE orders SET status = $1, status_v2 = $1 WHERE id = $2`, status, id)
```

**Phase 3: Backfill** (separate script, not migration)
```sql
-- Run as batch job, not migration
UPDATE orders SET status_v2 = status WHERE status_v2 IS NULL LIMIT 10000;
```

**Phase 4: Contract** (remove old, after all code updated)
```sql
-- Migration N+1: Only after all code uses status_v2
ALTER TABLE orders DROP COLUMN status;
ALTER TABLE orders RENAME COLUMN status_v2 TO status;
```

## Column Rename (Safe)

```sql
-- Migration 1: Add new column
ALTER TABLE orders ADD COLUMN total_cents BIGINT;

-- Backfill (script, not migration)
UPDATE orders SET total_cents = total * 100 WHERE total_cents IS NULL;

-- Migration 2: Set NOT NULL after backfill
ALTER TABLE orders ALTER COLUMN total_cents SET NOT NULL;

-- Migration 3: Drop old column (after code fully migrated)
ALTER TABLE orders DROP COLUMN total;
```

## Adding NOT NULL to Existing Column

```sql
-- WRONG: Locks table, scans all rows
ALTER TABLE orders ALTER COLUMN user_id SET NOT NULL;

-- CORRECT: Add constraint NOT VALID, then validate separately
ALTER TABLE orders ADD CONSTRAINT orders_user_id_not_null
    CHECK (user_id IS NOT NULL) NOT VALID;

-- In a separate migration (or manually):
ALTER TABLE orders VALIDATE CONSTRAINT orders_user_id_not_null;
```

## Large Table Index Creation

```sql
-- WRONG: Locks table during build
CREATE INDEX idx_orders_created_at ON orders(created_at);

-- CORRECT: Build concurrently (cannot be in transaction)
-- This migration must be run manually outside of golang-migrate
CREATE INDEX CONCURRENTLY idx_orders_created_at ON orders(created_at);
```

For golang-migrate, create a separate "manual" migration file and document it:
```
migrations/
  005_add_orders_index.up.sql      # Contains comment: "Run manually with psql"
  005_add_orders_index.down.sql
  MANUAL_005_README.md             # Instructions for manual execution
```

# Data Backfill Patterns

NEVER do large data changes in migrations. Migrations should be fast and atomic.

## `ON CONFLICT DO NOTHING` — Prefer Unnamed Target

When the target table has multiple `UNIQUE` constraints, **prefer `ON CONFLICT DO NOTHING` (no explicit target) over `ON CONFLICT (col)`**. The unnamed form covers every unique constraint at once — a collision on any of them falls through as a skip instead of aborting the whole statement.

```sql
-- WRONG: named target only protects email. A username collision aborts the INSERT.
INSERT INTO user_profiles (user_id, email, username, ...)
SELECT u.id, u.email, u.username, ...
FROM users u
WHERE u.profile_id IS NULL
ON CONFLICT (email) DO NOTHING;

-- CORRECT: unnamed target covers email, username, and any other unique constraint.
INSERT INTO user_profiles (user_id, email, username, ...)
SELECT u.id, u.email, u.username, ...
FROM users u
WHERE u.profile_id IS NULL
ON CONFLICT DO NOTHING;
```

Naming a specific `ON CONFLICT` target is an **optimization**, not the default. Only name a column when you need `DO UPDATE SET` (upsert) semantics bound to a specific index. For plain idempotent backfills, unnamed is strictly safer with zero downside.

## Batch Backfill Script

```go
// cmd/backfill/main.go — separate command, not migration
func backfillStatusV2(ctx context.Context, pool *pgxpool.Pool) error {
    const batchSize = 1000

    for {
        result, err := pool.Exec(ctx, `
            UPDATE orders
            SET status_v2 = status
            WHERE status_v2 IS NULL
            LIMIT $1
        `, batchSize)
        if err != nil {
            return fmt.Errorf("backfill batch: %w", err)
        }

        if result.RowsAffected() == 0 {
            break // done
        }

        slog.Info("backfill progress", "rows", result.RowsAffected())
        time.Sleep(100 * time.Millisecond) // avoid overwhelming DB
    }
    return nil
}
```

## Backfill with Progress Tracking

```sql
-- Create progress tracking table
CREATE TABLE backfill_progress (
    task_name TEXT PRIMARY KEY,
    last_processed_id UUID,
    updated_at TIMESTAMPTZ DEFAULT now()
);

-- Backfill script uses cursor-based pagination
UPDATE backfill_progress SET last_processed_id = $1 WHERE task_name = 'status_v2';
```

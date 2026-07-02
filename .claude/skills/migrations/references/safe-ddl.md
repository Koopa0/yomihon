# Safe Migration Patterns

## Creating a Table

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

## Adding a Column (safe)

```sql
-- up: nullable column, no lock issues
ALTER TABLE orders ADD COLUMN notes TEXT;

-- down
ALTER TABLE orders DROP COLUMN IF EXISTS notes;
```

## Adding a NOT NULL Column (safe)

```sql
-- up: add with default to avoid rewriting all rows
ALTER TABLE orders ADD COLUMN priority INT NOT NULL DEFAULT 0;

-- down
ALTER TABLE orders DROP COLUMN IF EXISTS priority;
```

## Adding an Index (safe, concurrent)

```sql
-- up: CONCURRENTLY avoids table lock
CREATE INDEX CONCURRENTLY idx_orders_created_at ON orders(created_at);

-- down
DROP INDEX CONCURRENTLY IF EXISTS idx_orders_created_at;
```

IMPORTANT: `CREATE INDEX CONCURRENTLY` cannot run inside a transaction. golang-migrate runs each file in a transaction by default. To use CONCURRENTLY, you must disable the transaction for that migration. There are two approaches:

1. Use `migrate`'s `x-no-transaction` parameter in the migration filename (not supported by default; depends on driver)
2. Split the index creation into a separate migration file that you run manually

## Renaming a Column (careful)

```sql
-- up
ALTER TABLE orders RENAME COLUMN total TO total_cents;

-- down
ALTER TABLE orders RENAME COLUMN total_cents TO total;
```

WARNING: Renaming columns breaks sqlc-generated code. After this migration:
1. Run `sqlc generate` to regenerate
2. Update all Go code referencing the old column name

## Dangerous Operations

NEVER do these in a migration without careful planning:

```sql
-- DANGEROUS: locks the entire table while rewriting
ALTER TABLE orders ALTER COLUMN status SET NOT NULL;

-- DANGEROUS: full table scan + rewrite
ALTER TABLE orders ALTER COLUMN status TYPE VARCHAR(50);

-- DANGEROUS: data loss
ALTER TABLE orders DROP COLUMN notes;
```

For large tables, use a multi-step migration:
1. Add new column (nullable)
2. Backfill data in batches (separate script, not migration)
3. Add NOT NULL constraint with `NOT VALID`
4. Validate constraint separately: `ALTER TABLE orders VALIDATE CONSTRAINT ...`

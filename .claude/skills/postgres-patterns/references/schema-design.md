# Schema Design Doctrine

Normalization rules, OLTP vs OLAP design, and caution rules for triggers, views, and JSONB.

## Normalization

### Third Normal Form (3NF) — The Baseline

All tables MUST achieve 3NF. This prevents update anomalies and ensures data integrity.

```sql
-- VIOLATION: 2NF — partial dependency on composite key
CREATE TABLE order_items (
    order_id UUID,
    product_id UUID,
    product_name TEXT,  -- depends only on product_id, not the full key
    quantity INTEGER,
    PRIMARY KEY (order_id, product_id)
);

-- CORRECT: Normalize product_name to products table
CREATE TABLE order_items (
    order_id UUID REFERENCES orders(id),
    product_id UUID REFERENCES products(id),
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    unit_price_cents BIGINT NOT NULL,  -- snapshot at order time (intentional denorm)
    PRIMARY KEY (order_id, product_id)
);
-- product_name retrieved via JOIN
```

### When Denormalization is Acceptable

Document the reason in migration comments:

1. **Audit/History tables**: Preserve point-in-time snapshots
   ```sql
   -- Intentional denorm: order_history captures state at order time
   CREATE TABLE order_history (
       id UUID PRIMARY KEY,
       order_id UUID NOT NULL,
       user_email TEXT NOT NULL,  -- snapshot, user may change email later
       total_cents BIGINT NOT NULL,
       recorded_at TIMESTAMPTZ NOT NULL DEFAULT now()
   );
   ```

2. **Read-heavy aggregates**: Materialized for performance
   ```sql
   -- Intentional denorm: pre-computed for dashboard queries
   CREATE MATERIALIZED VIEW user_order_stats AS
   SELECT user_id, COUNT(*) as order_count, SUM(total_cents) as lifetime_value
   FROM orders
   WHERE status = 'delivered'
   GROUP BY user_id;
   ```

## OLTP vs OLAP Design

### OLTP (This Project's Focus)

Optimized for transactions:
- Normalized schema (3NF)
- Row-oriented storage
- B-tree indexes on foreign keys and lookup columns
- Small, frequent transactions

```sql
-- OLTP: Normalized, indexed for point queries
CREATE TABLE orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    status TEXT NOT NULL DEFAULT 'pending',
    total_cents BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_orders_user_id ON orders(user_id);
CREATE INDEX idx_orders_status_created ON orders(status, created_at)
    WHERE status IN ('pending', 'confirmed');
```

### When Analytics Needs Emerge

DO NOT denormalize the OLTP schema. Instead:

1. **Materialized Views** for simple aggregates:
   ```sql
   CREATE MATERIALIZED VIEW daily_revenue AS
   SELECT date_trunc('day', created_at) as day, SUM(total_cents) as revenue
   FROM orders WHERE status = 'delivered'
   GROUP BY 1;

   -- Refresh during off-peak hours
   REFRESH MATERIALIZED VIEW CONCURRENTLY daily_revenue;
   ```

2. **Read Replicas** for heavy reporting queries

3. **Separate OLAP Storage** (ClickHouse, DuckDB) for true analytics workloads

## Triggers — Use with Extreme Caution

Triggers hide logic from the application. AVOID them for business logic.

### Acceptable Uses

1. **Audit logging** (infrastructure, not business):
   ```sql
   CREATE OR REPLACE FUNCTION audit_log() RETURNS TRIGGER AS $$
   BEGIN
       INSERT INTO audit_log (table_name, operation, row_id, changed_at)
       VALUES (TG_TABLE_NAME, TG_OP, NEW.id, now());
       RETURN NEW;
   END;
   $$ LANGUAGE plpgsql;
   ```

2. **Enforcing constraints** that CHECK cannot express (rare):
   ```sql
   -- Ensure order total matches sum of items (complex constraint)
   CREATE TRIGGER verify_order_total
   BEFORE UPDATE ON orders
   FOR EACH ROW EXECUTE FUNCTION verify_order_total_matches_items();
   ```

### NEVER Use Triggers For

- `updated_at` timestamps — set explicitly in UPDATE queries
- Business logic (notifications, state transitions)
- Cross-table cascading updates that should be transactions
- Anything the application should know about and control

### Why Explicit is Better

```sql
-- WRONG: Trigger updates timestamp invisibly
CREATE TRIGGER set_updated_at BEFORE UPDATE ON orders
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- CORRECT: Explicit in every UPDATE query
-- name: UpdateOrderStatus :one
UPDATE orders SET status = $2, updated_at = now()
WHERE id = $1 RETURNING *;
```

The Go code sees exactly what's happening. No hidden side effects.

## Views — Use for Read Optimization Only

Views are useful for encapsulating complex JOINs. They are NOT a substitute for proper schema design.

### Acceptable Uses

```sql
-- Simplify common multi-table queries
CREATE VIEW order_details AS
SELECT o.id, o.status, o.total_cents, o.created_at,
       u.email as user_email, u.name as user_name
FROM orders o
JOIN users u ON o.user_id = u.id;

-- Use in sqlc
-- name: OrderDetails :many
SELECT * FROM order_details WHERE id = $1;
```

### Avoid

- Views that hide complexity you should understand
- Deeply nested views (view referencing view referencing view)
- Views as a substitute for proper JOINs in queries
- Updatable views — too much magic, use explicit INSERT/UPDATE

### Materialized Views for Performance

```sql
-- Pre-compute expensive aggregates
CREATE MATERIALIZED VIEW user_stats AS
SELECT user_id, COUNT(*) as total_orders, MAX(created_at) as last_order_at
FROM orders
GROUP BY user_id;

CREATE UNIQUE INDEX idx_user_stats_user_id ON user_stats(user_id);

-- Refresh periodically (not real-time)
REFRESH MATERIALIZED VIEW CONCURRENTLY user_stats;
```

## JSONB — Structured Data, Not Schema Avoidance

JSONB is for **semi-structured data** that genuinely varies per row. It is NOT a way to avoid schema design.

### Acceptable Uses

1. **External API responses** (structure varies):
   ```sql
   CREATE TABLE webhook_events (
       id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
       event_type TEXT NOT NULL,
       payload JSONB NOT NULL,  -- external system, schema varies
       received_at TIMESTAMPTZ NOT NULL DEFAULT now()
   );
   ```

2. **User-defined attributes** (truly dynamic):
   ```sql
   CREATE TABLE products (
       id UUID PRIMARY KEY,
       name TEXT NOT NULL,
       price_cents BIGINT NOT NULL,
       attributes JSONB  -- color, size, custom fields per category
   );
   ```

3. **Metadata/settings** (rarely queried):
   ```sql
   ALTER TABLE users ADD COLUMN preferences JSONB DEFAULT '{}';
   ```

### NEVER Use JSONB For

- Core business attributes (normalize them!)
- Fields you frequently filter/sort on (add proper columns)
- Avoiding schema migrations (proper columns are faster and safer)
- Storing arrays of IDs (use junction tables)

```sql
-- WRONG: Hiding relational data in JSONB
CREATE TABLE orders (
    id UUID PRIMARY KEY,
    items JSONB  -- [{product_id, qty, price}, ...]  NO!
);

-- CORRECT: Proper junction table
CREATE TABLE order_items (
    order_id UUID REFERENCES orders(id),
    product_id UUID REFERENCES products(id),
    quantity INTEGER NOT NULL,
    unit_price_cents BIGINT NOT NULL,
    PRIMARY KEY (order_id, product_id)
);
```

### Querying JSONB Efficiently

```sql
-- GIN index for containment queries
CREATE INDEX idx_products_attributes ON products USING gin(attributes);

-- Query specific keys
SELECT * FROM products WHERE attributes->>'color' = 'red';

-- Containment (uses GIN index)
SELECT * FROM products WHERE attributes @> '{"size": "large"}';
```

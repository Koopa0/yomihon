# sqlc Advanced Query Patterns

Advanced query material for the sqlc-guide skill: sqlc.embed, sqlc.slice, batch operations, CTEs, subqueries, COALESCE conditional updates, and JOIN patterns.

## Advanced Features

### sqlc.embed() — Embedded Structs for JOINs

When JOINing tables, use `sqlc.embed()` to generate nested structs:

```sql
-- name: OrderWithUser :one
SELECT
    sqlc.embed(orders),
    sqlc.embed(users)
FROM orders
JOIN users ON orders.user_id = users.id
WHERE orders.id = $1;
```

Generated Go:

```go
type OrderWithUserRow struct {
    Order Order
    User  User
}

func (q *Queries) OrderWithUser(ctx context.Context, id string) (OrderWithUserRow, error)
```

### sqlc.slice() — Dynamic IN Queries

For variable-length IN clauses:

```sql
-- name: OrdersByIDs :many
SELECT * FROM orders
WHERE id = ANY(sqlc.slice(ids));
```

Generated Go:

```go
func (q *Queries) OrdersByIDs(ctx context.Context, ids []string) ([]Order, error)
```

**Note**: `sqlc.slice()` generates `= ANY($1)` which works with pgx arrays. For `IN` syntax, use:

```sql
-- name: OrdersByIDs :many
SELECT * FROM orders
WHERE id = ANY(@ids::uuid[]);
```

### Batch Operations

#### :batchone — Batch queries returning one row each

```sql
-- name: GetOrderBatch :batchone
SELECT * FROM orders WHERE id = $1;
```

Generated Go:

```go
func (q *Queries) GetOrderBatch(ctx context.Context, ids []string) *GetOrderBatchBatchResults

// Usage
br := queries.GetOrderBatch(ctx, []string{"id1", "id2", "id3"})
defer br.Close()

br.QueryRow(func(i int, order Order, err error) {
    // process each result
})
```

#### :batchmany — Batch queries returning multiple rows each

```sql
-- name: OrderItemsBatch :batchmany
SELECT * FROM order_items WHERE order_id = $1;
```

#### :batchexec — Batch exec operations

```sql
-- name: UpdateOrderStatusBatch :batchexec
UPDATE orders SET status = $2, updated_at = now() WHERE id = $1;
```

### CTEs (Common Table Expressions)

```sql
-- name: UserOrderStats :many
WITH order_totals AS (
    SELECT user_id, SUM(total_cents) as total, COUNT(*) as count
    FROM orders
    WHERE status = 'delivered'
    GROUP BY user_id
)
SELECT
    u.id,
    u.email,
    COALESCE(ot.total, 0) as lifetime_value,
    COALESCE(ot.count, 0) as order_count
FROM users u
LEFT JOIN order_totals ot ON u.id = ot.user_id
WHERE u.created_at > $1;
```

### Subqueries

```sql
-- name: UsersWithRecentOrders :many
SELECT * FROM users
WHERE id IN (
    SELECT DISTINCT user_id FROM orders
    WHERE created_at > now() - interval '30 days'
);

-- Or with EXISTS (often faster)
-- name: UsersWithRecentOrders :many
SELECT u.* FROM users u
WHERE EXISTS (
    SELECT 1 FROM orders o
    WHERE o.user_id = u.id
    AND o.created_at > now() - interval '30 days'
);
```

### Conditional Updates with COALESCE

```sql
-- name: PatchOrder :one
UPDATE orders SET
    status = COALESCE(sqlc.narg(status), status),
    total_cents = COALESCE(sqlc.narg(total_cents), total_cents),
    metadata = COALESCE(sqlc.narg(metadata), metadata),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;
```

Generated Go allows passing `nil` for fields you don't want to update:

```go
order, err := queries.PatchOrder(ctx, PatchOrderParams{
    ID:     orderID,
    Status: ptr("shipped"), // update this
    // TotalCents: nil — keeps existing value
    // Metadata: nil — keeps existing value
})
```

## JOIN Patterns

### One-to-One JOIN

```sql
-- name: OrderWithUser :one
SELECT
    o.id, o.status, o.total_cents, o.created_at,
    u.id as user_id, u.email as user_email, u.name as user_name
FROM orders o
JOIN users u ON o.user_id = u.id
WHERE o.id = $1;
```

**Problem**: Flat struct with all fields. Better approach:

```sql
-- name: OrderWithUser :one
SELECT
    sqlc.embed(o),
    sqlc.embed(u)
FROM orders o
JOIN users u ON o.user_id = u.id
WHERE o.id = $1;
```

### One-to-Many: Separate Queries (Recommended)

```sql
-- name: Order :one
SELECT * FROM orders WHERE id = $1;

-- name: OrderItems :many
SELECT * FROM order_items WHERE order_id = $1;
```

In Go, make two queries. This is clearer than complex JOINs with array aggregation.

### One-to-Many: JSON Aggregation (Advanced)

```sql
-- name: OrderWithItems :one
SELECT
    o.*,
    COALESCE(
        json_agg(json_build_object(
            'id', oi.id,
            'product_id', oi.product_id,
            'quantity', oi.quantity
        )) FILTER (WHERE oi.id IS NOT NULL),
        '[]'
    ) as items
FROM orders o
LEFT JOIN order_items oi ON o.id = oi.order_id
WHERE o.id = $1
GROUP BY o.id;
```

Requires custom type override:

```yaml
overrides:
  - column: "*.items"
    go_type:
      import: "encoding/json"
      type: "RawMessage"
```

# Bulk Operations and Listen/Notify (pgx/v5)

Multi-query round-trips (batches), bulk insert (CopyFrom), and PostgreSQL notifications.

## Batch Operations

Send multiple queries in a single round-trip:

```go
batch := &pgx.Batch{}
for _, id := range orderIDs {
    batch.Queue(`SELECT id, status, total FROM orders WHERE id = $1`, id)
}

br := pool.SendBatch(ctx, batch)
defer br.Close()

var orders []Order
for range orderIDs {
    var o Order
    if err := br.QueryRow().Scan(&o.ID, &o.Status, &o.Total); err != nil {
        return nil, fmt.Errorf("scanning batch result: %w", err)
    }
    orders = append(orders, o)
}
```

## CopyFrom (Bulk Insert)

```go
_, err := pool.CopyFrom(
    ctx,
    pgx.Identifier{"orders"},
    []string{"id", "status", "total", "created_at"},
    pgx.CopyFromSlice(len(orders), func(i int) ([]any, error) {
        return []any{
            orders[i].ID,
            orders[i].Status,
            orders[i].Total,
            orders[i].CreatedAt,
        }, nil
    }),
)
```

## Listen/Notify

```go
conn, err := pool.Acquire(ctx)
if err != nil {
    return fmt.Errorf("acquiring connection: %w", err)
}
defer conn.Release()

_, err = conn.Exec(ctx, "LISTEN order_changes")
if err != nil {
    return fmt.Errorf("listening: %w", err)
}

for {
    notification, err := conn.Conn().WaitForNotification(ctx)
    if err != nil {
        return fmt.Errorf("waiting for notification: %w", err)
    }
    // process notification.Payload
}
```

# Advanced Transactions (pgx/v5)

Savepoints and retry logic for transient failures (serialization failures, deadlocks).

## Savepoints (Nested Transaction Behavior)

pgx doesn't have automatic savepoints, but you can use them manually:

```go
tx, _ := pool.Begin(ctx)
defer tx.Rollback(ctx)

// Main operation
_, err = tx.Exec(ctx, `INSERT INTO orders (id, status) VALUES ($1, $2)`, orderID, "pending")
if err != nil {
    return err
}

// Nested operation with savepoint
_, err = tx.Exec(ctx, "SAVEPOINT sp1")
if err != nil {
    return err
}

_, err = tx.Exec(ctx, `INSERT INTO order_items (order_id, product_id) VALUES ($1, $2)`, orderID, productID)
if err != nil {
    // Rollback only the savepoint, not the entire transaction
    tx.Exec(ctx, "ROLLBACK TO SAVEPOINT sp1")
    // Continue with other operations...
} else {
    tx.Exec(ctx, "RELEASE SAVEPOINT sp1")
}

return tx.Commit(ctx)
```

## Retry Logic for Transient Failures

### Serialization Failure Retry

```go
func withSerializableRetry(ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) error {
    const maxRetries = 3

    for attempt := 0; attempt < maxRetries; attempt++ {
        tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
        if err != nil {
            return fmt.Errorf("beginning tx: %w", err)
        }

        err = fn(tx)
        if err != nil {
            tx.Rollback(ctx)

            if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == "40001" {
                // Serialization failure — retry
                backoff := time.Duration(attempt*100) * time.Millisecond
                time.Sleep(backoff)
                continue
            }
            return err
        }

        if err := tx.Commit(ctx); err != nil {
            if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == "40001" {
                backoff := time.Duration(attempt*100) * time.Millisecond
                time.Sleep(backoff)
                continue
            }
            return fmt.Errorf("committing: %w", err)
        }
        return nil
    }
    return fmt.Errorf("max retries exceeded for serializable transaction")
}
```

### Deadlock Retry

```go
func isDeadlock(err error) bool {
    var pgErr *pgconn.PgError
    return errors.As(err, &pgErr) && pgErr.Code == "40P01"
}

// Retry on deadlock with exponential backoff
```

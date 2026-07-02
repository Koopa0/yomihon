---
name: pgx-patterns
description: >-
  pgx/v5 best-practice reference for PostgreSQL in Go — pgxpool setup and
  tuning, QueryRow/CollectRows/RowToStructByName scanning, transactions with
  defer Rollback, batches, CopyFrom bulk insert, pgconn.PgError code handling,
  pgtype custom types, Listen/Notify, timeouts and context propagation, pool
  health callbacks and stats, query tracing, savepoints, retry logic for
  serialization failures and deadlocks, SSL/TLS, and prepared statement
  caching.
when_to_use: >-
  Use when writing or reviewing store.go database code with pgx/v5 —
  configuring pgxpool, scanning rows into structs, handling pgx.ErrNoRows or
  unique-violation errors, wrapping multi-step writes in transactions, bulk
  inserting, setting query timeouts, or debugging pool exhaustion. Trigger
  keywords: "pgx", "pgxpool", "QueryRow", "CollectRows", "transaction",
  "BeginTx", "CopyFrom", "PgError", "pgtype", "connection pool".
user_invocable: true
metadata:
  author: koopa
  version: "1.0"
  lang: go
---

# pgx/v5 Patterns Reference

Core patterns are below. Deep material lives in `references/` — see Navigation at the end.

## Common Pitfalls

### 1. Forgetting to Close Rows

```go
// WRONG — rows never closed on error
rows, err := pool.Query(ctx, sql)
if err != nil {
    return err
}
// if processing fails, rows leak

// CORRECT — use CollectRows (auto-closes)
items, err := pgx.CollectRows(rows, pgx.RowToStructByName[Item])

// Or close explicitly
rows, err := pool.Query(ctx, sql)
if err != nil {
    return err
}
defer rows.Close() // always close
```

### 2. Scanning into Wrong Types

```go
// PostgreSQL BIGINT is int64, not int
var count int // WRONG on 32-bit systems
var count int64 // CORRECT
```

### 3. Using Pool After Close

```go
pool.Close()
pool.Query(ctx, sql) // panics or returns error
```

### 4. Ignoring Context Cancellation

```go
// WRONG — query continues even if client disconnects
rows, _ := pool.Query(context.Background(), longRunningQuery)

// CORRECT — use request context
rows, _ := pool.Query(r.Context(), longRunningQuery)
```

### 5. Transaction Isolation Misunderstanding

```go
// Read Committed (default): sees committed changes from other transactions
// Repeatable Read: sees snapshot from start of transaction
// Serializable: full isolation, may fail with serialization error

// For financial operations, use Serializable with retry logic
tx, _ := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
```

For the retry logic itself, read `references/advanced-transactions.md`.

## Error Handling

### pgx.ErrNoRows

```go
if errors.Is(err, pgx.ErrNoRows) {
    return nil, ErrNotFound
}
```

### PostgreSQL Error Codes (pgconn.PgError)

```go
import "github.com/jackc/pgx/v5/pgconn"

// Go 1.26+: errors.AsType replaces errors.As (rules/go-version.md)
if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
    switch pgErr.Code {
    case "23505": // unique_violation
        return nil, ErrConflict
    case "23503": // foreign_key_violation
        return nil, fmt.Errorf("referenced record not found: %w", err)
    case "23514": // check_violation
        return nil, fmt.Errorf("constraint violated: %s: %w", pgErr.ConstraintName, err)
    }
}
```

### Common PG Error Codes

| Code  | Name                | Meaning                  |
|-------|---------------------|--------------------------|
| 23505 | unique_violation    | Duplicate key            |
| 23503 | foreign_key_violation | Referenced row missing |
| 23514 | check_violation     | CHECK constraint failed  |
| 23502 | not_null_violation  | NULL in NOT NULL column  |
| 40001 | serialization_failure | Serializable conflict  |
| 40P01 | deadlock_detected   | Deadlock                 |

## Connection Pool (pgxpool)

### Setup

```go
import "github.com/jackc/pgx/v5/pgxpool"

config, err := pgxpool.ParseConfig(databaseURL)
if err != nil {
    return fmt.Errorf("parsing database config: %w", err)
}

// Optional: tune pool settings
config.MaxConns = 25
config.MinConns = 5
config.MaxConnLifetime = time.Hour
config.MaxConnIdleTime = 30 * time.Minute

pool, err := pgxpool.NewWithConfig(ctx, config)
if err != nil {
    return fmt.Errorf("creating connection pool: %w", err)
}
defer pool.Close()

// Verify connectivity
if err := pool.Ping(ctx); err != nil {
    return fmt.Errorf("pinging database: %w", err)
}
```

### Simple Setup

```go
pool, err := pgxpool.New(ctx, databaseURL)
if err != nil {
    return fmt.Errorf("creating pool: %w", err)
}
defer pool.Close()
```

## Querying

### Single Row

```go
var order Order
err := pool.QueryRow(ctx,
    `SELECT id, status, total, created_at FROM orders WHERE id = $1`,
    orderID,
).Scan(&order.ID, &order.Status, &order.Total, &order.CreatedAt)

if errors.Is(err, pgx.ErrNoRows) {
    return nil, ErrNotFound
}
if err != nil {
    return nil, fmt.Errorf("querying order %s: %w", orderID, err)
}
```

### Multiple Rows with CollectRows

```go
rows, err := pool.Query(ctx,
    `SELECT id, status, total, created_at FROM orders WHERE user_id = $1 ORDER BY created_at DESC`,
    userID,
)
if err != nil {
    return nil, fmt.Errorf("querying orders: %w", err)
}

orders, err := pgx.CollectRows(rows, pgx.RowToStructByName[Order])
if err != nil {
    return nil, fmt.Errorf("collecting orders: %w", err)
}
```

### RowToStructByName Requirements

Struct fields must have `db` tags matching column names:

```go
type Order struct {
    ID        string    `db:"id" json:"id"`
    Status    string    `db:"status" json:"status"`
    Total     int64     `db:"total" json:"total"`
    CreatedAt time.Time `db:"created_at" json:"created_at"`
}
```

### Custom Row Scanner

```go
orders, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Order, error) {
    var o Order
    err := row.Scan(&o.ID, &o.Status, &o.Total, &o.CreatedAt)
    return o, err
})
```

## Transactions

### Basic Pattern

```go
tx, err := pool.Begin(ctx)
if err != nil {
    return fmt.Errorf("beginning transaction: %w", err)
}
defer tx.Rollback(ctx) // no-op after commit

_, err = tx.Exec(ctx,
    `UPDATE orders SET status = $1, updated_at = now() WHERE id = $2`,
    newStatus, orderID,
)
if err != nil {
    return fmt.Errorf("updating order: %w", err)
}

_, err = tx.Exec(ctx,
    `INSERT INTO order_history (order_id, status, created_at) VALUES ($1, $2, now())`,
    orderID, newStatus,
)
if err != nil {
    return fmt.Errorf("inserting history: %w", err)
}

if err := tx.Commit(ctx); err != nil {
    return fmt.Errorf("committing: %w", err)
}
return nil
```

### Using pool.BeginTx for Isolation

```go
tx, err := pool.BeginTx(ctx, pgx.TxOptions{
    IsoLevel: pgx.Serializable,
})
```

## Context, Timeouts, and Cancellation

### Query Timeout

```go
// Per-query timeout
ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
defer cancel()

rows, err := pool.Query(ctx, `SELECT * FROM orders WHERE status = $1`, status)
// If query takes >5s, context is cancelled, query is aborted
```

### Graceful Shutdown

```go
// In main.go
ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
defer stop()

// When signal received, ctx is cancelled
// All in-flight queries using this ctx will be aborted
// pool.Close() will wait for active connections to finish

go func() {
    <-ctx.Done()
    // Give in-flight requests time to complete
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    pool.Close() // blocks until all connections released or timeout
}()
```

### Context Propagation in HTTP Handlers

```go
func getOrder(store *Store) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // r.Context() is cancelled when client disconnects
        // This propagates to database queries
        order, err := store.Order(r.Context(), r.PathValue("id"))
        // ...
    }
}
```

## Navigation

Deep material moved to `references/`. Read the file matching your task:

| Topic | File | When to read |
|-------|------|--------------|
| Pool health callbacks, pool stats, query tracing (QueryTracer + OTel), SSL/TLS, prepared statement caching | `references/pool-production-setup.md` | Tuning the pool for production, observability, or debugging pool exhaustion |
| Savepoints, serialization-failure retry, deadlock retry | `references/advanced-transactions.md` | Serializable transactions, nested rollback, transient-failure retry loops |
| Batch operations, CopyFrom bulk insert, Listen/Notify | `references/bulk-and-notify.md` | High-throughput writes, multi-query round-trips, Postgres notifications |
| pgtype nullable types, arrays, JSONB, composite types | `references/scanning-and-types.md` | Scanning non-trivial column types into Go structs |

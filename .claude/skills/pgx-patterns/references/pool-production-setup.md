# Pool Production Setup (pgx/v5)

Production-grade pool configuration: health callbacks, statistics, query tracing, TLS, and prepared statement caching.

## Connection Pool Health and Callbacks

### Pool Configuration with Callbacks

```go
config, _ := pgxpool.ParseConfig(databaseURL)

// Called before returning connection from pool
config.BeforeAcquire = func(ctx context.Context, conn *pgx.Conn) bool {
    // Return false to discard this connection and try another
    // Useful for checking connection health
    return conn.Ping(ctx) == nil
}

// Called after connection returned to pool
config.AfterRelease = func(conn *pgx.Conn) bool {
    // Return false to close connection instead of returning to pool
    // Useful for cleaning up session state
    _, err := conn.Exec(context.Background(), "DISCARD ALL")
    return err == nil
}

// Called when new connection is established
config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
    // Set session-level settings
    _, err := conn.Exec(ctx, "SET statement_timeout = '30s'")
    return err
}

pool, _ := pgxpool.NewWithConfig(ctx, config)
```

### Pool Statistics (Observability)

```go
stats := pool.Stat()
slog.Info("pool stats",
    "total_conns", stats.TotalConns(),
    "acquired_conns", stats.AcquiredConns(),
    "idle_conns", stats.IdleConns(),
    "max_conns", stats.MaxConns(),
    "constructing_conns", stats.ConstructingConns(),
    "new_conns_count", stats.NewConnsCount(),
    "max_lifetime_destroy_count", stats.MaxLifetimeDestroyCount(),
    "max_idle_destroy_count", stats.MaxIdleDestroyCount(),
)

// Expose as Prometheus metrics
poolTotalConns.Set(float64(stats.TotalConns()))
poolAcquiredConns.Set(float64(stats.AcquiredConns()))
poolIdleConns.Set(float64(stats.IdleConns()))
```

## Query Tracing (Logging/Observability)

### Implement pgx.QueryTracer

```go
type queryTracer struct {
    logger *slog.Logger
}

type ctxKey struct{}

func (t *queryTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
    return context.WithValue(ctx, ctxKey{}, time.Now())
}

func (t *queryTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
    startTime := ctx.Value(ctxKey{}).(time.Time)
    duration := time.Since(startTime)

    if data.Err != nil {
        t.logger.Error("query failed",
            "sql", data.SQL,
            "args", data.Args,
            "duration", duration,
            "error", data.Err,
        )
        return
    }

    // Log slow queries
    if duration > 100*time.Millisecond {
        t.logger.Warn("slow query",
            "sql", data.SQL,
            "duration", duration,
            "rows", data.CommandTag.RowsAffected(),
        )
    }
}

// Attach to pool config
config.ConnConfig.Tracer = &queryTracer{logger: logger}
```

### OpenTelemetry Integration

```go
import "github.com/exaring/otelpgx"

config, _ := pgxpool.ParseConfig(databaseURL)
config.ConnConfig.Tracer = otelpgx.NewTracer()
pool, _ := pgxpool.NewWithConfig(ctx, config)
```

## SSL/TLS Configuration

### Required for Production

```go
config, _ := pgxpool.ParseConfig(databaseURL)

// sslmode is part of the connection string, but you can override:
// databaseURL := "postgres://user:pass@host:5432/db?sslmode=verify-full&sslrootcert=/path/to/ca.crt"

// Or configure TLS manually:
config.ConnConfig.TLSConfig = &tls.Config{
    ServerName: "db.example.com",
    MinVersion: tls.VersionTLS12,
    // For mutual TLS:
    // Certificates: []tls.Certificate{cert},
    // RootCAs: caCertPool,
}
```

### SSL Modes

| Mode | Description | Use Case |
|------|-------------|----------|
| `disable` | No SSL | Local development only |
| `require` | SSL required, no verification | Encrypted but vulnerable to MITM |
| `verify-ca` | Verify server cert signed by CA | Most common production setting |
| `verify-full` | Verify CA + hostname | Highest security |

## Prepared Statement Caching

pgx automatically prepares and caches statements. Control this:

```go
config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeCacheStatement // default
// or
config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol // no prepare
// or
config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeDescribeExec // always describe
```

### When to Use Simple Protocol

```go
// For dynamic SQL or one-off queries
rows, err := pool.Query(ctx, sql, pgx.QueryExecModeSimpleProtocol, args...)
```

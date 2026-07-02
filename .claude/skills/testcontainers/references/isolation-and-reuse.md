# Isolation Strategies & Container Reuse

Transaction-based isolation (the fastest strategy) lives in `SKILL.md` § Transaction-Based Isolation. This file covers the remaining isolation strategies and container reuse patterns.

## Per-Test Isolation

Use transactions for isolation when sharing a container:

```go
func truncateTables(t *testing.T, pool *pgxpool.Pool) {
    t.Helper()
    _, err := pool.Exec(t.Context(), "TRUNCATE orders CASCADE")
    if err != nil {
        t.Fatalf("truncating: %v", err)
    }
}
```

Or use a separate schema per test:

```go
func withSchema(t *testing.T, pool *pgxpool.Pool) *pgxpool.Pool {
    t.Helper()
    ctx := t.Context()
    schema := "test_" + strings.ReplaceAll(t.Name(), "/", "_")

    _, err := pool.Exec(ctx, fmt.Sprintf("CREATE SCHEMA %s", pgx.Identifier{schema}.Sanitize()))
    if err != nil {
        t.Fatalf("creating schema: %v", err)
    }
    t.Cleanup(func() {
        pool.Exec(ctx, fmt.Sprintf("DROP SCHEMA %s CASCADE", pgx.Identifier{schema}.Sanitize()))
    })

    // Set search_path for this test's connections
    // ...
    return pool
}
```

## Parallel Test Isolation

### Schema-Based Isolation

Each test gets its own schema:

```go
func withTestSchema(t *testing.T, pool *pgxpool.Pool) *pgxpool.Pool {
    t.Helper()
    ctx := t.Context()

    // Create unique schema
    schema := fmt.Sprintf("test_%s_%d", strings.ReplaceAll(t.Name(), "/", "_"), time.Now().UnixNano())
    schema = strings.ToLower(schema)

    _, err := pool.Exec(ctx, fmt.Sprintf("CREATE SCHEMA %s", pgx.Identifier{schema}.Sanitize()))
    if err != nil {
        t.Fatalf("creating schema: %v", err)
    }
    t.Cleanup(func() {
        pool.Exec(context.Background(), fmt.Sprintf("DROP SCHEMA %s CASCADE", pgx.Identifier{schema}.Sanitize()))
    })

    // Create schema-scoped pool
    config, _ := pgxpool.ParseConfig(pool.Config().ConnString())
    config.ConnConfig.RuntimeParams["search_path"] = schema
    schemaPool, err := pgxpool.NewWithConfig(ctx, config)
    if err != nil {
        t.Fatalf("creating schema pool: %v", err)
    }
    t.Cleanup(func() { schemaPool.Close() })

    // Apply migrations to schema
    applyMigrations(t, schemaPool)

    return schemaPool
}
```

### Container Per Test (Slowest, Most Isolated)

```go
func TestWithOwnContainer(t *testing.T) {
    t.Parallel()
    pool := setupPostgres(t) // each test gets its own container
    // ...
}
```

## Container Reuse

### Reuse Between Test Runs (Development)

```go
pg, err := postgres.Run(ctx,
    "postgres:17-alpine",
    testcontainers.WithReuse(testcontainers.WithReuseByName("go-spec-test-db")),
    postgres.WithDatabase("testdb"),
)
```

**Warning**: Reuse can cause test pollution. Only use during local development.

### Singleton Pattern for Package

```go
//go:build integration

package order_test

import (
    "sync"
    "testing"
)

var (
    sharedPool     *pgxpool.Pool
    sharedPoolOnce sync.Once
)

func getSharedPool(t *testing.T) *pgxpool.Pool {
    t.Helper()

    sharedPoolOnce.Do(func() {
        // Deliberately NOT t.Context(): the pool is shared across tests and
        // must outlive the first caller's test. Same for TestMain (no *testing.T).
        ctx := context.Background()
        pg, err := postgres.Run(ctx, "postgres:17-alpine",
            postgres.WithDatabase("testdb"),
            postgres.WithUsername("test"),
            postgres.WithPassword("test"),
        )
        if err != nil {
            t.Fatalf("starting postgres: %v", err)
        }
        // Note: cannot use t.Cleanup here because t is from first caller

        connStr, _ := pg.ConnectionString(ctx, "sslmode=disable")
        sharedPool, err = pgxpool.New(ctx, connStr)
        if err != nil {
            t.Fatalf("creating pool: %v", err)
        }

        applyMigrations(ctx, sharedPool)
    })

    return sharedPool
}
```

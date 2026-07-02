---
name: testcontainers
description: >-
  testcontainers-go patterns for PostgreSQL integration testing in Go —
  container setup with the postgres module, golang-migrate wiring, shared
  container via TestMain, isolation strategies (transaction, schema,
  container-per-test), container reuse, wait strategies, test data fixtures,
  testing HTTP handlers against a real database, and fixes for common Docker
  issues.
when_to_use: >-
  Use when writing or debugging integration tests against a real PostgreSQL —
  creating integration_test.go with //go:build integration, testing store.go
  database operations, running make test-integration, choosing a test isolation
  strategy, or fixing container startup timeouts, port conflicts, or
  Docker-not-running errors. Trigger keywords: testcontainers, integration
  test, postgres container, real database in tests.
user_invocable: true
metadata:
  author: koopa
  version: "1.0"
  lang: go
---

# testcontainers-go PostgreSQL Patterns

## Performance Tips

1. **Use TestMain** for shared container across package
2. **Use transactions** for test isolation instead of truncating tables
3. **Reuse connections** — don't create new pool per test
4. **Parallel tests** with proper isolation
5. **Slim images** — use `postgres:17-alpine` instead of full image
6. **Skip in CI** if container overhead is too high (use separate integration job)

## Basic Setup

**File Convention**: exactly ONE `integration_test.go` per feature package. First line: `//go:build integration`. Package: `package <feature>_test` (black-box, through the public surface). See `.claude/rules/testing.md` § Integration Tests.

```go
//go:build integration

package order_test

import (
    "testing"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/modules/postgres"
    "github.com/testcontainers/testcontainers-go/wait"
)

func setupPostgres(t *testing.T) *pgxpool.Pool {
    t.Helper()
    ctx := t.Context()

    pg, err := postgres.Run(ctx,
        "postgres:17-alpine",
        postgres.WithDatabase("testdb"),
        postgres.WithUsername("test"),
        postgres.WithPassword("test"),
        testcontainers.WithWaitStrategy(
            wait.ForLog("database system is ready to accept connections").
                WithOccurrence(2),
        ),
    )
    if err != nil {
        t.Fatalf("starting postgres container: %v", err)
    }
    t.Cleanup(func() {
        if err := pg.Terminate(ctx); err != nil {
            t.Logf("terminating postgres: %v", err)
        }
    })

    connStr, err := pg.ConnectionString(ctx, "sslmode=disable")
    if err != nil {
        t.Fatalf("getting connection string: %v", err)
    }

    pool, err := pgxpool.New(ctx, connStr)
    if err != nil {
        t.Fatalf("creating pool: %v", err)
    }
    t.Cleanup(func() { pool.Close() })

    return pool
}
```

## With Migrations

```go
func setupPostgresWithMigrations(t *testing.T) *pgxpool.Pool {
    t.Helper()
    pool := setupPostgres(t)
    ctx := t.Context()

    // Apply migrations
    migrations := []string{
        `CREATE TABLE IF NOT EXISTS orders (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            user_id UUID NOT NULL,
            status TEXT NOT NULL DEFAULT 'pending',
            total BIGINT NOT NULL DEFAULT 0,
            created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
        )`,
        `CREATE INDEX idx_orders_user_id ON orders(user_id)`,
        `CREATE INDEX idx_orders_status ON orders(status)`,
    }

    for _, m := range migrations {
        if _, err := pool.Exec(ctx, m); err != nil {
            t.Fatalf("applying migration: %v", err)
        }
    }

    return pool
}
```

For wiring real migration files via golang-migrate (path resolution from project root), read `references/advanced-setup.md`.

## Shared Container in TestMain

Share a single container across all tests in a package:

```go
//go:build integration

package order_test

import (
    "context"
    "log"
    "os"
    "testing"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/testcontainers/testcontainers-go/modules/postgres"
)

var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
    ctx := context.Background()

    pg, err := postgres.Run(ctx,
        "postgres:17-alpine",
        postgres.WithDatabase("testdb"),
        postgres.WithUsername("test"),
        postgres.WithPassword("test"),
    )
    if err != nil {
        log.Fatalf("starting postgres: %v", err)
    }

    connStr, err := pg.ConnectionString(ctx, "sslmode=disable")
    if err != nil {
        log.Fatalf("getting connection string: %v", err)
    }

    testPool, err = pgxpool.New(ctx, connStr)
    if err != nil {
        log.Fatalf("creating pool: %v", err)
    }

    // Apply migrations
    applyMigrations(ctx, testPool)

    code := m.Run()

    testPool.Close()
    pg.Terminate(ctx)
    os.Exit(code)
}
```

## Transaction-Based Isolation (Fastest)

Each test runs in a transaction that's rolled back:

```go
func TestWithTx(t *testing.T) {
    pool := getSharedPool(t) // shared container
    ctx := t.Context()

    tx, err := pool.Begin(ctx)
    if err != nil {
        t.Fatalf("beginning tx: %v", err)
    }
    t.Cleanup(func() { tx.Rollback(ctx) }) // rollback, not commit

    store := order.NewStore(tx) // store uses transaction

    // Test operations...
    // All changes are rolled back after test
}
```

For the other isolation strategies (truncate, schema-per-test, container-per-test) and container reuse patterns, read `references/isolation-and-reuse.md`.

## Complete Integration Test Example

```go
//go:build integration

package order_test

import (
    "context"
    "errors"
    "testing"

    "github.com/google/go-cmp/cmp"
    "github.com/google/go-cmp/cmp/cmpopts"
    "github.com/koopa0/go-spec/internal/order"
)

func TestStore_CreateAndRetrieve(t *testing.T) {
    pool := setupPostgresWithMigrations(t)
    store := order.NewStore(pool)
    ctx := t.Context()

    // Create
    created, err := store.Create(ctx, order.CreateParams{
        UserID: "user-1",
        Total:  1500,
    })
    if err != nil {
        t.Fatalf("creating order: %v", err)
    }

    // Retrieve
    got, err := store.ByID(ctx, created.ID)
    if err != nil {
        t.Fatalf("getting order: %v", err)
    }

    if diff := cmp.Diff(created, got, cmpopts.IgnoreFields(order.Order{}, "CreatedAt", "UpdatedAt")); diff != "" {
        t.Errorf("order mismatch (-want +got):\n%s", diff)
    }
}

func TestStore_NotFound(t *testing.T) {
    pool := setupPostgresWithMigrations(t)
    store := order.NewStore(pool)
    ctx := t.Context()

    _, err := store.ByID(ctx, "nonexistent")
    if err == nil {
        t.Fatal("expected error, got nil")
    }
    if !errors.Is(err, order.ErrNotFound) {
        t.Errorf("expected ErrNotFound, got %v", err)
    }
}
```

## Running Integration Tests

```bash
# Run only integration tests
go test -tags=integration ./...

# Run with verbose output
go test -tags=integration -v ./...

# Run specific integration test
go test -tags=integration -run TestStore_CreateAndRetrieve ./internal/order/
```

## Navigation

Deep-dive material lives in `references/`. Read on demand:

| Topic | File | When to read |
|-------|------|--------------|
| Isolation strategies & container reuse | `references/isolation-and-reuse.md` | Choosing truncate vs schema-per-test vs container-per-test, parallel test isolation, reuse between runs, singleton shared pool |
| Advanced setup | `references/advanced-setup.md` | Init scripts, custom/health-check wait strategies, golang-migrate path resolution and wiring |
| Fixtures & handler tests | `references/fixtures-and-handlers.md` | Factory functions, bulk test data generation, testing HTTP handlers against a real database |
| Troubleshooting | `references/troubleshooting.md` | Container startup timeouts, port conflicts, Docker not running, cleanup failures |

# Advanced Setup — Init Scripts, Wait Strategies, golang-migrate

## With Init Scripts

```go
pg, err := postgres.Run(ctx,
    "postgres:17-alpine",
    postgres.WithDatabase("testdb"),
    postgres.WithUsername("test"),
    postgres.WithPassword("test"),
    postgres.WithInitScripts("../../migrations/001_initial.up.sql"),
)
```

## Advanced Wait Strategies

### Custom Wait Strategy

```go
pg, err := postgres.Run(ctx,
    "postgres:17-alpine",
    testcontainers.WithWaitStrategy(
        wait.ForAll(
            wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
            wait.ForListeningPort("5432/tcp"),
        ).WithDeadline(60*time.Second),
    ),
)
```

### Health Check Wait

```go
testcontainers.WithWaitStrategy(
    wait.ForSQL("5432/tcp", "pgx", func(host string, port nat.Port) string {
        return fmt.Sprintf("postgres://test:test@%s:%s/testdb?sslmode=disable", host, port.Port())
    }).WithQuery("SELECT 1"),
)
```

## Testing with golang-migrate

### Path Resolution

```go
func getMigrationsPath() string {
    // Find project root by looking for go.mod
    dir, _ := os.Getwd()
    for {
        if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
            return filepath.Join(dir, "migrations")
        }
        parent := filepath.Dir(dir)
        if parent == dir {
            panic("could not find project root")
        }
        dir = parent
    }
}

func applyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
    migrationsPath := getMigrationsPath()
    m, err := migrate.New("file://"+migrationsPath, pool.Config().ConnString())
    if err != nil {
        return fmt.Errorf("creating migrator: %w", err)
    }
    defer m.Close()

    if err := m.Up(); err != nil && err != migrate.ErrNoChange {
        return fmt.Errorf("running migrations: %w", err)
    }
    return nil
}
```

# Troubleshooting — Common Issues

## Container Startup Timeout

```go
// Increase timeout for slow CI environments
pg, err := postgres.Run(ctx,
    "postgres:17-alpine",
    testcontainers.WithWaitStrategy(
        wait.ForLog("database system is ready").
            WithStartupTimeout(120*time.Second),
    ),
)
```

## Port Conflicts

```go
// Let testcontainers assign random port (default behavior)
connStr, _ := pg.ConnectionString(ctx)
// connStr will have the assigned port
```

## Docker Not Running

```go
func TestMain(m *testing.M) {
    // Check Docker availability before running tests
    ctx := context.Background()
    _, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
        ContainerRequest: testcontainers.ContainerRequest{
            Image: "alpine:latest",
        },
        Started: false,
    })
    if err != nil {
        log.Println("Docker not available, skipping integration tests")
        os.Exit(0)
    }

    os.Exit(m.Run())
}
```

## Cleanup Failures

```go
t.Cleanup(func() {
    // Use background context for cleanup
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    if err := pg.Terminate(ctx); err != nil {
        t.Logf("warning: failed to terminate container: %v", err)
    }
})
```

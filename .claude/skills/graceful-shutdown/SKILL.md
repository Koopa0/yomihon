---
name: graceful-shutdown
description: >-
  Go server graceful shutdown patterns — OS signal handling with
  signal.NotifyContext, http.Server.Shutdown connection draining, background
  goroutine and worker lifecycle, SSE broker and NATS connection draining,
  per-subsystem timeout strategy, health checks during shutdown, and resource
  cleanup ordering in main.go.
when_to_use: >-
  Use when implementing server startup/shutdown, wiring main.go with multiple
  subsystems that need coordinated teardown, managing long-lived connections
  (SSE, WebSocket, NATS), running background pipelines or workers, handling
  SIGINT/SIGTERM, or when shutdown order and cleanup timeouts matter.
metadata:
  author: koopa
  version: "1.0"
  lang: go
---

# Graceful Shutdown Patterns

## Complete main.go Shutdown Orchestration

```go
func main() {
    // 1. Setup root context from OS signals
    ctx, stop := signal.NotifyContext(context.Background(),
        syscall.SIGINT, syscall.SIGTERM,
    )
    defer stop()

    cfg := loadConfig()
    logger := setupLogger(cfg.LogLevel)

    // 2. Create resources (order matters for teardown)
    pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
    if err != nil {
        log.Fatalf("creating pool: %v", err)
    }

    // 3. Wire application
    mux := http.NewServeMux()
    // ... register routes ...

    handler := chain(mux,
        recoverPanic(logger),
        logRequest(logger),
    )

    // 4. Run
    if err := run(ctx, logger, handler, pool, cfg.Port); err != nil {
        logger.Error("server exited with error", "error", err)
        os.Exit(1)
    }
}
```

## Shutdown Sequence

```
Signal received (SIGINT/SIGTERM)
  │
  ▼
1. Stop accepting new HTTP requests
  │  └─ srv.Shutdown(ctx) stops listener
  │
  ▼
2. Drain in-flight HTTP requests
  │  └─ srv.Shutdown waits for active handlers to return
  │  └─ SSE/WebSocket: context cancellation triggers cleanup
  │
  ▼
3. Stop background workers
  │  └─ Cancel worker contexts, wg.Wait()
  │
  ▼
4. Close external connections
  │  └─ pool.Close() — waits for active queries to finish
  │  └─ NATS conn.Drain()
  │
  ▼
5. Exit
```

## Standard run() Function

```go
func run(ctx context.Context, logger *slog.Logger, handler http.Handler, pool *pgxpool.Pool, port string) error {
    srv := &http.Server{
        Addr:         ":" + port,
        Handler:      handler,
        ReadTimeout:  5 * time.Second,
        WriteTimeout: 10 * time.Second,
        IdleTimeout:  120 * time.Second,
    }

    // Start server
    errCh := make(chan error, 1)
    go func() {
        logger.Info("server starting", "port", port)
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            errCh <- err
        }
    }()

    // Wait for signal or server error
    select {
    case err := <-errCh:
        return fmt.Errorf("server error: %w", err)
    case <-ctx.Done():
        logger.Info("shutdown signal received")
    }

    // Graceful shutdown with timeout
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    // Phase 1: Stop accepting, drain HTTP requests
    if err := srv.Shutdown(shutdownCtx); err != nil {
        return fmt.Errorf("server shutdown: %w", err)
    }
    logger.Info("http server drained")

    // Phase 2: Close database pool
    pool.Close()
    logger.Info("database pool closed")

    return nil
}
```

## With Background Workers

```go
func run(ctx context.Context, logger *slog.Logger, handler http.Handler, pool *pgxpool.Pool, port string) error {
    srv := &http.Server{
        Addr:    ":" + port,
        Handler: handler,
    }

    // Track background goroutines
    var wg sync.WaitGroup

    // Start background worker (Go 1.25+: wg.Go handles Add/Done)
    workerCtx, cancelWorker := context.WithCancel(ctx)
    wg.Go(func() {
        runWorker(workerCtx, logger)
    })

    // Start server
    errCh := make(chan error, 1)
    go func() {
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            errCh <- err
        }
    }()

    select {
    case err := <-errCh:
        cancelWorker()
        return fmt.Errorf("server error: %w", err)
    case <-ctx.Done():
    }

    shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    // Phase 1: HTTP drain
    if err := srv.Shutdown(shutdownCtx); err != nil {
        logger.Error("http shutdown error", "error", err)
    }

    // Phase 2: Stop background workers
    cancelWorker()

    // Wait for workers with timeout
    done := make(chan struct{})
    go func() {
        wg.Wait()
        close(done)
    }()

    select {
    case <-done:
        logger.Info("background workers stopped")
    case <-shutdownCtx.Done():
        logger.Warn("timed out waiting for workers")
    }

    // Phase 3: Close resources
    pool.Close()

    return nil
}
```

## SSE Connection Draining

SSE handlers hold long-lived connections. They MUST respect context cancellation:

```go
func sseHandler(broker *Broker, logger *slog.Logger) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        flusher, ok := w.(http.Flusher)
        if !ok {
            http.Error(w, "streaming not supported", http.StatusInternalServerError)
            return
        }

        w.Header().Set("Content-Type", "text/event-stream")
        w.Header().Set("Cache-Control", "no-cache")
        w.Header().Set("Connection", "keep-alive")

        // Subscribe to events
        ch := broker.Subscribe()
        defer broker.Unsubscribe(ch)

        ctx := r.Context() // cancelled on shutdown OR client disconnect

        for {
            select {
            case <-ctx.Done():
                // Server shutdown or client disconnected
                return
            case event := <-ch:
                fmt.Fprintf(w, "data: %s\n\n", event)
                flusher.Flush()
            }
        }
    }
}
```

When `srv.Shutdown()` is called:
1. Listener stops accepting new connections
2. `r.Context()` is cancelled for in-flight requests
3. SSE handler's `select` hits `ctx.Done()` and returns
4. `srv.Shutdown()` returns once all handlers exit

### SSE Broker Shutdown

```go
type Broker struct {
    mu      sync.RWMutex
    clients map[chan string]struct{}
    closed  bool
}

// Publish with non-blocking send — never block on slow clients
func (b *Broker) Publish(event string) {
    b.mu.RLock()
    defer b.mu.RUnlock()
    if b.closed {
        return
    }
    for ch := range b.clients {
        select {
        case ch <- event:
        default:
            // Client too slow, drop event
        }
    }
}

// Close unblocks all subscribers
func (b *Broker) Close() {
    b.mu.Lock()
    defer b.mu.Unlock()
    b.closed = true
    for ch := range b.clients {
        close(ch)
    }
}
```

## Background Pipeline Pattern

For long-running processing pipelines (e.g., story generation):

```go
type Pipeline struct {
    wg     sync.WaitGroup
    cancel context.CancelFunc
}

func NewPipeline(ctx context.Context, logger *slog.Logger) *Pipeline {
    ctx, cancel := context.WithCancel(ctx)
    p := &Pipeline{cancel: cancel}

    p.wg.Go(func() { // Go 1.25+
        p.run(ctx, logger)
    })

    return p
}

func (p *Pipeline) run(ctx context.Context, logger *slog.Logger) {
    for {
        select {
        case <-ctx.Done():
            logger.Info("pipeline shutting down")
            return
        default:
            // Process next item — pass ctx so individual operations cancel too
            if err := p.processNext(ctx); err != nil {
                if ctx.Err() != nil {
                    return // shutdown requested
                }
                logger.Error("pipeline error", "error", err)
            }
        }
    }
}

// Wait blocks until the pipeline finishes all in-progress work
func (p *Pipeline) Wait() {
    p.cancel()
    p.wg.Wait()
}
```

### Integration in main.go

```go
pipeline := NewPipeline(ctx, logger)

// In shutdown sequence (after HTTP drain, before pool close):
pipeline.Wait()
logger.Info("pipeline drained")
```

## Timeout Strategy

```go
// Development: short timeout, fast feedback
shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

// Production: long enough for SSE clients + in-flight AI generation
shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
```

### Per-Subsystem Timeouts

```go
// HTTP: 10s for normal requests
httpCtx, httpCancel := context.WithTimeout(context.Background(), 10*time.Second)
defer httpCancel()
srv.Shutdown(httpCtx)

// Workers: 20s for in-progress tasks
workerCtx, workerCancel := context.WithTimeout(context.Background(), 20*time.Second)
defer workerCancel()
cancelWorker()
waitWithTimeout(workerCtx, &wg)

// Pool: close last (active queries use worker timeout)
pool.Close()
```

## Health Check During Shutdown

```go
var shutdownStarted atomic.Bool

func healthCheck() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if shutdownStarted.Load() {
            http.Error(w, "shutting down", http.StatusServiceUnavailable)
            return
        }
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("ok"))
    }
}

// Set before calling srv.Shutdown — load balancer stops routing
shutdownStarted.Store(true)
```

## NATS Connection Draining

```go
// NATS Drain waits for in-flight messages to be processed
nc, _ := nats.Connect(natsURL)

// In shutdown:
if err := nc.Drain(); err != nil {
    logger.Error("nats drain", "error", err)
}
// Drain blocks until all subscriptions process buffered messages
```

See: nats skill for full NATS patterns.

## Common Mistakes

```go
// WRONG: os.Exit in signal handler — skips defers and cleanup
signal.Notify(sigCh, syscall.SIGINT)
go func() {
    <-sigCh
    os.Exit(0) // resources leak!
}()

// WRONG: context.Background() for shutdown — no timeout
srv.Shutdown(context.Background()) // blocks forever if handler hangs

// WRONG: Closing pool before HTTP drain
pool.Close()           // kills in-flight queries!
srv.Shutdown(ctx)      // handlers fail

// WRONG: Not waiting for goroutines
cancelWorker()
// pool.Close()        // worker might still be running
pool.Close()           // MUST wg.Wait() first
```

---
name: go-concurrency
description: >-
  Go concurrency patterns — goroutine lifecycle, errgroup for parallel I/O,
  worker pools, channel patterns (fan-out/fan-in, pipeline, done channel),
  sync primitives, race condition prevention, and timeout strategies.
when_to_use: >-
  Use when implementing concurrent code, worker pools, parallel I/O, channel
  pipelines, SSE brokers, or background processing; when working with shared
  state, goroutines, mutexes, sync.WaitGroup, errgroup, or select; or when
  the user mentions data races, race detector output, deadlocks, or timeouts
  in concurrent code.
metadata:
  author: koopa
  version: "2.0"
  lang: go
---

# Go Concurrency Patterns

## Decision: Do You Need Concurrency?

```
Is the operation I/O bound (network, disk, DB)?
├─ No  → Stay synchronous. Concurrency adds complexity for no gain.
├─ Yes → Are there MULTIPLE independent I/O operations?
│   ├─ No  → Stay synchronous. One goroutine per request is enough.
│   └─ Yes → Is latency critical?
│       ├─ No  → Sequential is simpler. Do them one by one.
│       └─ Yes → Use errgroup for parallel I/O.
```

See: concurrency.md rule for the full decision framework and primitive selection table.

## Decision: Mutex vs Atomic

Atomics are ONLY for single-word independent state — counters, flags,
`atomic.Pointer[T]` lazy publication. ANY compound invariant (2+ fields change
together, or a read decides a write) takes a mutex — atomics cannot protect
multi-field invariants (see concurrency.md rule).

```
Single counter/flag/pointer?
├─ Yes → atomic (lock-free, no contention)
├─ No  → Multiple related fields need consistent update?
│   └─ Yes → Mutex (atomic can't protect multi-field invariants)
├─ Read-heavy config object, rare writes?
│   └─ atomic.Pointer[T] or atomic.Value (swap entire object on write)
└─ Profiled mutex contention on hot path?
    └─ Consider replacing with atomic
```

Full atomic patterns (atomic.Pointer[T], CAS rules, memory model,
double-checked locking, anti-patterns): references/sync-and-atomics.md.

## Common Mistakes

```go
// WRONG: t.Fatal from goroutine
go func() {
    t.Fatal("failed") // undefined behavior
}()
// CORRECT: t.Error from goroutine
go func() {
    t.Error("failed")
}()

// WRONG: unbounded goroutines
for _, item := range items {
    go process(item) // 1M items = 1M goroutines
}
// CORRECT: bounded with errgroup
g.SetLimit(10)
for _, item := range items {
    g.Go(func() error { return process(ctx, item) })
}

// WRONG: mutex held during I/O
mu.Lock()
resp, _ := http.Get(url) // blocks all other goroutines waiting for lock
mu.Unlock()
// CORRECT: copy, release, then I/O
mu.Lock()
urlCopy := url
mu.Unlock()
resp, _ := http.Get(urlCopy)

// WRONG: defer in loop
for _, item := range items {
    mu.Lock()
    defer mu.Unlock() // only unlocks when function returns!
}
// CORRECT: extract to function
for _, item := range items {
    func() {
        mu.Lock()
        defer mu.Unlock()
        process(item)
    }()
}
```

## errgroup: Parallel Tasks with Error Handling

```go
import "golang.org/x/sync/errgroup"

func enrichOrder(ctx context.Context, id string) (*EnrichedOrder, error) {
    var (
        order    *Order
        user     *User
        payments []Payment
    )

    g, ctx := errgroup.WithContext(ctx)

    g.Go(func() error {
        var err error
        order, err = orderStore.Order(ctx, id)
        if err != nil {
            return fmt.Errorf("fetching order: %w", err)
        }
        return nil
    })

    g.Go(func() error {
        var err error
        user, err = userStore.User(ctx, id)
        if err != nil {
            return fmt.Errorf("fetching user: %w", err)
        }
        return nil
    })

    g.Go(func() error {
        var err error
        payments, err = paymentStore.Payments(ctx, id)
        if err != nil {
            return fmt.Errorf("fetching payments: %w", err)
        }
        return nil
    })

    if err := g.Wait(); err != nil {
        return nil, err // first error cancels ctx, other goroutines see ctx.Done()
    }

    return &EnrichedOrder{Order: order, User: user, Payments: payments}, nil
}
```

### errgroup with Bounded Concurrency

```go
func processItems(ctx context.Context, items []Item) error {
    g, ctx := errgroup.WithContext(ctx)
    g.SetLimit(10) // max 10 concurrent goroutines

    for _, item := range items {
        g.Go(func() error {
            return process(ctx, item)
        })
    }

    return g.Wait()
}
```

## WaitGroup.Go (Go 1.25+) — Preferred

MUST USE `wg.Go()` on Go 1.25+. The old `Add`/`Done` pattern is legacy.

```go
// LEGACY: manual Add/Done (pre-Go 1.25)
var wg sync.WaitGroup
wg.Add(1)
go func() {
    defer wg.Done()
    // work
}()
wg.Wait()

// PREFERRED (Go 1.25+): wg.Go — handles Add/Done automatically
var wg sync.WaitGroup
wg.Go(func() {
    // work
})
wg.Wait()
```

`wg.Go(f)` calls `wg.Add(1)`, launches `go f()`, and defers `wg.Done()` internally.
No risk of mismatched Add/Done counts. Use this everywhere you would have used the
manual pattern. Go 1.25+ also adds the `waitgroup` vet analyzer that detects misplaced
`wg.Add()` calls in legacy code (e.g., `Add` inside the goroutine instead of before it).

## Timeout and Deadline

```go
// context.WithTimeout — cancels after duration
func fetchWithTimeout(ctx context.Context, url string) ([]byte, error) {
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, fmt.Errorf("creating request: %w", err)
    }

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("fetching %s: %w", url, err)
    }
    defer resp.Body.Close()

    return io.ReadAll(resp.Body)
}

// select with time.After — timeout on channel operation
select {
case result := <-resultCh:
    return result, nil
case <-time.After(5 * time.Second):
    return nil, fmt.Errorf("operation timed out")
case <-ctx.Done():
    return nil, ctx.Err()
}
```

For cancellation cleanup, prefer `context.AfterFunc(ctx, cleanup)` (Go 1.21+) over a dedicated goroutine blocked on `<-ctx.Done()` — no extra goroutine to leak, and the returned `stop` function deregisters the callback when cleanup is no longer needed.

## Race Condition Prevention

### Common Data Race: Loop Variable Capture

```go
// Go 1.22+ — loop variables are per-iteration, this is safe
for _, item := range items {
    go func() {
        process(item) // safe in Go 1.22+
    }()
}
```

### Shared Map Access

```go
// WRONG: concurrent map read/write panics
var cache = make(map[string]string)
go func() { cache["key"] = "value" }()
go func() { _ = cache["key"] }()

// CORRECT: protect with mutex
var (
    mu    sync.RWMutex
    cache = make(map[string]string)
)
```

### Detect Races

```bash
go test -race ./...
go run -race ./cmd/app
```

Always run tests with `-race` in CI. See: verify skill Step 6 (Race Detection).

## Goroutine Leak Prevention

Every goroutine must have a documented exit condition:

```go
// WRONG: goroutine runs forever
go func() {
    for {
        process()
    }
}()

// CORRECT: context cancellation stops it
go func() {
    for {
        select {
        case <-ctx.Done():
            return
        default:
            process(ctx)
        }
    }
}()
```

Leak detection in tests (synctest bubble, goleak policy):
references/testing-concurrent-code.md.

## Navigation

Deep material lives in references/. Read the matching file on demand:

| Topic | File |
|-------|------|
| Worker pools, fan-out/fan-in, pipelines, SSE broker (in-memory pub/sub, non-blocking publish) | references/channels-and-workers.md |
| Mutex vs RWMutex, sync.OnceValues, sync.Pool, atomic deep dive (Int64/Bool, Pointer[T], Value, CAS, memory model, happens-before, double-checked locking, atomic anti-patterns) | references/sync-and-atomics.md |
| Testing concurrent code: synctest bubbles, fake clock, goroutine leak detection in tests | references/testing-concurrent-code.md |

For channel-based pipelines or worker pools, read references/channels-and-workers.md.
For shared-state protection or atomics beyond simple counters, read references/sync-and-atomics.md.
For testing concurrent or time-dependent code, read references/testing-concurrent-code.md.

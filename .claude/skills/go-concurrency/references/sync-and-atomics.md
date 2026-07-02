# sync Primitives and Atomics

Implementation reference for shared-state protection. The mutex-vs-atomic
decision tree and the single-word rule live in SKILL.md.

## sync Primitives

### Mutex vs RWMutex

```go
// Mutex: write-heavy or balanced read/write
type Counter struct {
    mu    sync.Mutex
    count int64
}

func (c *Counter) Increment() {
    c.mu.Lock()
    c.count++
    c.mu.Unlock()
}

func (c *Counter) Value() int64 {
    c.mu.Lock()
    defer c.mu.Unlock()
    return c.count
}

// RWMutex: read-heavy (>10:1 read/write ratio)
type Cache struct {
    mu    sync.RWMutex
    items map[string]Item
}

func (c *Cache) Get(key string) (Item, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    item, ok := c.items[key]
    return item, ok
}

func (c *Cache) Set(key string, item Item) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.items[key] = item
}
```

### For Simple Counters: Use atomic

```go
var requestCount atomic.Int64

func handler(w http.ResponseWriter, r *http.Request) {
    requestCount.Add(1)
    // ...
}

func metrics(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "requests: %d", requestCount.Load())
}
```

### sync.OnceValues: One-Time Init That Can Fail

NEVER `log.Fatalf` inside a `once.Do` closure — it hides the error from the
caller and violates error-handling.md (`log.Fatal` only in `main()` startup
helpers). Use `sync.OnceValues` (Go 1.21+) and propagate the error:

```go
var getPool = sync.OnceValues(func() (*pgxpool.Pool, error) {
    pool, err := pgxpool.New(context.Background(), databaseURL)
    if err != nil {
        return nil, fmt.Errorf("creating pool: %w", err)
    }
    return pool, nil
})

func handler(w http.ResponseWriter, r *http.Request) {
    pool, err := getPool()
    if err != nil {
        http.Error(w, "service unavailable", http.StatusServiceUnavailable)
        return
    }
    // use pool
}
```

Caveats:
- **A failed result is cached too.** The function runs exactly once — if init
  fails, every later call returns the same error forever. Fine for fail-fast
  startup; WRONG for retryable resources (those need a mutex with explicit
  retry state, not a Once).
- **Struct-field `sync.Once` remains correct for per-instance init.** The
  package-level `OnceValues` pattern is for process-wide singletons; a
  `sync.Once` field lazily initializing its own struct instance is fine — just
  return the error to the caller instead of calling `log.Fatalf`.

### sync.Pool: Reuse Allocations

```go
var bufPool = sync.Pool{
    New: func() any {
        return new(bytes.Buffer)
    },
}

func processRequest(data []byte) string {
    buf := bufPool.Get().(*bytes.Buffer)
    defer func() {
        buf.Reset()
        bufPool.Put(buf)
    }()

    buf.Write(data)
    // process...
    return buf.String()
}
```

## Atomic Deep Dive

### atomic.Int64 / atomic.Bool (Go 1.19+)

```go
// Counter — lock-free, zero contention
var requestCount atomic.Int64

func handler(w http.ResponseWriter, r *http.Request) {
    requestCount.Add(1)
    // ...
}

// Flag — graceful shutdown signal
var shuttingDown atomic.Bool

func shutdown() {
    shuttingDown.Store(true)
}

func handler(w http.ResponseWriter, r *http.Request) {
    if shuttingDown.Load() {
        http.Error(w, "shutting down", http.StatusServiceUnavailable)
        return
    }
    // ...
}
```

### atomic.Pointer[T] (Go 1.19+) — Config Hot-Reload

```go
// Type-safe atomic pointer — swap entire config on reload
type Config struct {
    MaxConns int
    Timeout  time.Duration
    Features map[string]bool
}

var currentConfig atomic.Pointer[Config]

func init() {
    currentConfig.Store(&Config{MaxConns: 10, Timeout: 5 * time.Second})
}

// Reader — zero contention, no lock
func getConfig() *Config {
    return currentConfig.Load()
}

// Writer — swap entire config atomically
func reloadConfig(newCfg *Config) {
    currentConfig.Store(newCfg)
    // Readers see old config until they call Load() again.
    // No partial reads — either old config or new config, never a mix.
}
```

**Why atomic.Pointer over sync.RWMutex for config**:
- Readers never block, even during a write
- No lock contention under high read volume
- Trade-off: each write allocates a new Config (acceptable for rare config reloads)

### atomic.Value — Untyped Alternative

```go
// When you need to store different types (rare)
var cache atomic.Value // stores map[string]string

func initCache() {
    cache.Store(make(map[string]string))
}

func getCache() map[string]string {
    return cache.Load().(map[string]string)
}

// ✅ Prefer atomic.Pointer[T] over atomic.Value for type safety
```

### CompareAndSwap

A CAS retry loop is justified ONLY when ALL three hold:
(a) the state is a single word,
(b) the retry body is side-effect-free (safe to re-run after a lost race),
(c) a benchmark proves mutex contention on this path.

The default is a mutex. Any CAS loop MUST carry a comment justifying all three.

```go
// CAS loop justified: peak is a single int64, the body is a pure max
// comparison, and BenchmarkRecordPeak shows mutex contention at 16 goroutines.
func (m *Metrics) RecordPeak(v int64) {
    for {
        cur := m.peak.Load()
        if v <= cur || m.peak.CompareAndSwap(cur, v) {
            return
        }
    }
}
```

### Go Memory Model: What You Must Know

Go atomics provide **sequential consistency** — unlike C/C++/Rust, there is no
`Relaxed`, `Acquire`, `Release` ordering to choose. Every atomic operation in Go
is sequentially consistent. This means:

- You do NOT need to think about memory ordering
- All goroutines agree on the order of atomic operations
- This is simpler but slightly slower than relaxed atomics in other languages

### Key Happens-Before Guarantees

| Operation | Guarantee |
|-----------|-----------|
| `ch <- v` happens-before `<-ch` | Channel send before receive |
| `mu.Unlock()` happens-before next `mu.Lock()` | Mutex unlock before lock |
| `once.Do(f)` — `f()` happens-before any `Do` returns | Once init before use |
| `atomic.Store` happens-before `atomic.Load` (of same variable) | Atomic write before read |
| `go f()` — the `go` statement happens-before `f()` starts | Goroutine creation |

### Double-Checked Locking: The Classic Bug

```go
// ❌ BROKEN — data race on 'instance'
var instance *Config

func getConfig() *Config {
    if instance == nil {           // unsynchronized read
        mu.Lock()
        if instance == nil {
            instance = &Config{...} // another goroutine may see partial init
        }
        mu.Unlock()
    }
    return instance
}
// The first read of 'instance' has no synchronization.
// Another goroutine might see a partially initialized *Config.

// ✅ CORRECT — sync.Once (simplest, most common)
var (
    once     sync.Once
    instance *Config
)

func getConfig() *Config {
    once.Do(func() {
        instance = &Config{...}
    })
    return instance
}

// ✅ CORRECT — atomic.Pointer (when you need to reset/reload)
var configPtr atomic.Pointer[Config]

func getConfig() *Config {
    cfg := configPtr.Load()
    if cfg != nil {
        return cfg
    }
    mu.Lock()
    defer mu.Unlock()
    if cfg := configPtr.Load(); cfg != nil {
        return cfg // another goroutine initialized while we waited
    }
    cfg = &Config{...}
    configPtr.Store(cfg)
    return cfg
}
```

### Anti-Patterns

```go
// ❌ WRONG — using deprecated function-style API
atomic.AddInt64(&count, 1)  // pre-Go 1.19
atomic.LoadInt64(&count)

// ✅ CORRECT — method-style API (Go 1.19+)
var count atomic.Int64
count.Add(1)
count.Load()

// ❌ WRONG — atomic for multi-field update
var balance atomic.Int64
var owner atomic.Pointer[string]
// These are independent — no way to atomically update both

// ✅ CORRECT — Mutex for multi-field invariants
type Account struct {
    mu      sync.Mutex
    balance int64
    owner   string
}

// ❌ WRONG — mixing atomic and non-atomic access to same variable
var flag int32
go func() { atomic.StoreInt32(&flag, 1) }()
go func() { if flag == 1 { ... } }() // non-atomic read = data race

// ✅ CORRECT — all access must be atomic
var flag atomic.Int32
go func() { flag.Store(1) }()
go func() { if flag.Load() == 1 { ... } }()
```

See: go-performance skill for when atomic reduces allocation pressure.
See: go-types skill for the sync.Mutex copy rule.

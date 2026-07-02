---
name: go-performance
description: >-
  Go performance patterns — pre-allocation with benchmarks, strings.Builder,
  escape analysis, sync.Pool, pprof workflow, and common allocation traps.
  Complements the perf-reviewer agent (detection commands, review output
  format) with deep how-to patterns for writing fast code.
when_to_use: >-
  Use when optimizing hot paths, reducing allocations, profiling CPU or
  memory with pprof, writing benchmarks, or when escape analysis behavior
  is unclear. Trigger phrases: "optimize this", "slow", "too many
  allocations", "profile", "why does this escape to the heap", "sync.Pool",
  "benchmark".
metadata:
  author: koopa
  version: "1.0"
  lang: go
---

# Go Performance Patterns

## Optimization Decision Tree

```
Is this code on a hot path (called per-request or in a tight loop)?
├─ No  → Don't optimize. Clarity > speed.
├─ Yes → Have you MEASURED the bottleneck?
│   ├─ No  → Benchmark first (see Benchmarking section)
│   └─ Yes → What is the bottleneck?
│       ├─ CPU → Profile with pprof -cpuprofile
│       │   Common: regex in loop, reflection, sorting
│       ├─ Memory (allocs) → Profile with pprof -memprofile
│       │   Common: string concat, missing pre-alloc, interface boxing
│       ├─ I/O (database) → EXPLAIN ANALYZE the query
│       │   Common: N+1, missing index, unbounded SELECT
│       └─ Latency → Trace with OpenTelemetry
│           Common: sequential I/O that could be concurrent
```

**Rule**: NEVER optimize without measuring first. Premature optimization adds
complexity without proven benefit.

## Pre-Allocation

### Slice Pre-Allocation

```go
// ❌ WRONG — grows dynamically, ~log2(n) allocations
var result []Order
for _, row := range rows {
    result = append(result, mapOrder(row))
}

// ✅ CORRECT — single allocation
result := make([]Order, 0, len(rows))
for _, row := range rows {
    result = append(result, mapOrder(row))
}
```

**Benchmark comparison**:
```
BenchmarkAppendNoPrealloc-8    1000    1523 ns/op    4080 B/op    8 allocs/op
BenchmarkAppendPrealloc-8      1000     892 ns/op    1024 B/op    1 allocs/op
```

### Map Pre-Allocation

```go
// ❌ WRONG — rehashes as map grows
m := make(map[string]Order)
for _, o := range orders {
    m[o.ID] = o
}

// ✅ CORRECT — sized hint prevents rehashing
m := make(map[string]Order, len(orders))
for _, o := range orders {
    m[o.ID] = o
}
```

### When to Pre-Allocate

| Situation | Pre-allocate? |
|-----------|--------------|
| Length known at call site | Yes, always |
| Length bounded (e.g., LIMIT in SQL) | Yes, use the bound |
| Length unknown, likely small (<10) | No, append is fine |
| Length unknown, could be large | Yes, use reasonable estimate |

## strings.Builder

```go
// ❌ WRONG — O(n^2), new allocation per concatenation
func buildCSV(records []Record) string {
    var s string
    for _, r := range records {
        s += r.Name + "," + strconv.Itoa(r.Value) + "\n"
    }
    return s
}

// ✅ CORRECT — O(n), single buffer
func buildCSV(records []Record) string {
    var b strings.Builder
    b.Grow(len(records) * 32) // estimate avg line length
    for _, r := range records {
        b.WriteString(r.Name)
        b.WriteByte(',')
        b.WriteString(strconv.Itoa(r.Value))
        b.WriteByte('\n')
    }
    return b.String()
}
```

**Rule**: any string concatenation in a loop MUST use `strings.Builder`.
For 2-3 fixed parts outside a loop, `+` is fine.

## Escape Analysis

### What It Is

The Go compiler decides whether a variable lives on the stack (cheap) or
escapes to the heap (expensive — needs GC). Understanding escape helps
reduce allocations.

### Checking Escape Decisions

```bash
# Show escape analysis decisions
go build -gcflags='-m' ./internal/order/ 2>&1 | grep -E 'escape|heap'

# More verbose (includes inlining decisions)
go build -gcflags='-m -m' ./internal/order/ 2>&1 | head -50
```

### Common Escape Causes

| Pattern | Why It Escapes | Fix |
|---------|---------------|-----|
| Returning pointer to local | Outlives stack frame | Return value type if small |
| Assigning to interface | Compiler can't track concrete type | Avoid interface on hot path |
| Closure capturing variable | Variable outlives function | Pass as parameter instead |
| Slice append beyond cap | New backing array on heap | Pre-allocate with known cap |
| `fmt.Sprintf` | Arguments boxed into `[]any` | `strconv` functions on hot path |

```go
// ❌ Escapes — pointer returned, allocated on heap
func newOrder(id string) *Order {
    o := Order{ID: id} // escapes to heap
    return &o
}

// ✅ No escape — value returned, stays on stack (if small)
func newOrder(id string) Order {
    return Order{ID: id} // stack allocated
}

// ❌ Escapes — interface boxing
func logValue(v any) { // v escapes
    slog.Info("value", "v", v)
}

// ✅ No escape — concrete type
func logValue(v string) {
    slog.Info("value", "v", v)
}
```

### When to Care About Escape

- Hot path functions called thousands of times per second
- Benchmark shows high `allocs/op`
- pprof memory profile shows the function

**Do NOT care** for startup code, error paths, or functions called <100 times/second.

## sync.Pool

### When to Use

```
Is the object expensive to create (>1KB, complex init)?
├─ No  → Don't pool. GC handles small objects fine.
├─ Yes → Is it used in a hot path (per-request or tighter)?
│   ├─ No  → Don't pool. Complexity not worth it.
│   └─ Yes → sync.Pool. BUT measure before and after.
```

### Pattern

```go
var bufPool = sync.Pool{
    New: func() any {
        return new(bytes.Buffer)
    },
}

func processRequest(data []byte) string {
    buf := bufPool.Get().(*bytes.Buffer)
    defer func() {
        buf.Reset() // MUST reset before returning to pool
        bufPool.Put(buf)
    }()

    buf.Write(data)
    // ... use buf ...
    return buf.String()
}
```

### sync.Pool Rules

- ALWAYS reset state before returning to pool (`buf.Reset()`, zero fields)
- NEVER store pointers to pooled objects — they may be reused
- NEVER assume the pool retains objects (GC can clear the pool)
- Pool is per-type — don't pool `any`

### Anti-Pattern: Pooling Small Objects

```go
// ❌ WRONG — strings are small, pooling adds overhead
var stringPool = sync.Pool{
    New: func() any { return new(string) },
}

// ❌ WRONG — small structs, GC handles fine
var pointPool = sync.Pool{
    New: func() any { return new(Point) }, // 16 bytes
}
```

## Benchmarking Workflow

### Writing Benchmarks (Go 1.24+)

```go
func BenchmarkProcessOrder(b *testing.B) {
    order := createTestOrder()
    for b.Loop() {
        processOrder(order)
    }
}

// With setup that shouldn't be measured
func BenchmarkStoreOrder(b *testing.B) {
    store := setupStore(b)
    order := createTestOrder()
    b.ResetTimer() // exclude setup time

    for b.Loop() {
        store.Create(b.Context(), order)
    }
}

// Sub-benchmarks for different sizes
func BenchmarkMarshal(b *testing.B) {
    for _, size := range []int{1, 10, 100, 1000} {
        b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
            data := generateData(size)
            for b.Loop() {
                json.Marshal(data)
            }
        })
    }
}
```

### Running and Comparing

```bash
# Quick exploratory run with memory stats (NOT comparable evidence)
go test -bench=. -benchmem ./internal/order/

# Comparable runs go through the make targets (benchstat over -count=10):
make bench-baseline BENCH_PKG=./internal/order/   # on OLD code → tmp/bench/old.txt
# ... apply your change ...
make bench-compare BENCH_PKG=./internal/order/    # NEW code → benchstat old vs new
```

See § Statistical Discipline below for why single runs and manual one-off
comparisons are not acceptable evidence.

### Reading Results

```
BenchmarkProcessOrder-8    52384    22847 ns/op    4096 B/op    12 allocs/op
```

| Field | Meaning | Target |
|-------|---------|--------|
| `ns/op` | Time per operation | Lower is better |
| `B/op` | Heap bytes per operation | 0 ideal for hot paths |
| `allocs/op` | Heap allocations per operation | 0 ideal for hot paths |

## Statistical Discipline

### The Local-Noise Problem

A development machine is a noisy benchmark environment: browsers, the IDE,
background indexers, and laptop thermal throttling all skew timings. Two
single runs of the SAME code can differ by more than the improvement you're
trying to prove. Single runs are never comparable.

### Workflow

1. On OLD code: `make bench-baseline` — runs `-count=10`, saves `tmp/bench/old.txt`
2. Apply the change
3. On NEW code: `make bench-compare` — runs `-count=10`, saves `tmp/bench/new.txt`,
   then runs `benchstat` over both

Scope either target with `BENCH_PKG=./internal/order/` to avoid benchmarking
the whole module.

### Reading benchstat Output

```
              │  old.txt   │             new.txt             │
              │   sec/op   │   sec/op    vs base             │
ProcessOrder-8  22.85µ ± 2%   18.12µ ± 3%  -20.70% (p=0.000 n=10)
BuildCSV-8      1.523µ ± 5%   1.498µ ± 6%  ~       (p=0.436 n=10)
```

- A performance claim requires **p < 0.05**.
- `~` means benchstat could NOT prove a difference. NEVER report `~` or
  p ≥ 0.05 as a win — "no proven difference" is the only honest summary.
- Cite the benchstat delta (`-20.70% (p=0.000)`), not raw `ns/op` from a
  single run.

### Hard Rules

- NEVER compare numbers across machines, Go versions, sessions, or thermal
  states. Baseline and comparison must come from the same machine in the
  same session under the same conditions.
- Shared CI runners: benchmarks are smoke tests only — they verify the code
  runs, never gate on absolute numbers.

See `.claude/rules/testing.md` § Benchmark Discipline for the canonical
MUST/NEVER rules this section expands.

## pprof Quick Reference

```bash
# CPU profile
go test -bench=BenchmarkX -cpuprofile=cpu.prof ./internal/order/
go tool pprof -top cpu.prof          # top consumers
go tool pprof -http=:8080 cpu.prof   # web UI (flame graph)

# Memory profile
go test -bench=BenchmarkX -memprofile=mem.prof ./internal/order/
go tool pprof -top -cum mem.prof     # top allocators

# Live profiling (add to main.go)
import _ "net/http/pprof"
go http.ListenAndServe("localhost:6060", nil)

# Then:
go tool pprof http://localhost:6060/debug/pprof/heap
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30
```

## Common Performance Traps

### regexp.MustCompile in Function Body

```go
// ❌ WRONG — compiles regex every call (expensive)
func validate(s string) bool {
    re := regexp.MustCompile(`^[a-z]+$`)
    return re.MatchString(s)
}

// ✅ CORRECT — compile once at package level
var validPattern = regexp.MustCompile(`^[a-z]+$`)

func validate(s string) bool {
    return validPattern.MatchString(s)
}
```

### fmt.Sprintf on Hot Path

```go
// ❌ WRONG — reflection + allocation
key := fmt.Sprintf("order:%s", id)

// ✅ CORRECT — direct concatenation (no reflection)
key := "order:" + id

// ❌ WRONG — Sprintf for int conversion
s := fmt.Sprintf("%d", n)

// ✅ CORRECT — strconv (no reflection)
s := strconv.Itoa(n)
```

### Large Struct Copy in Range

```go
// ❌ WRONG — copies entire LargeStruct each iteration
type LargeStruct struct {
    Data [1024]byte
    // ... many fields
}

for _, item := range largeSlice {
    process(item) // item is a copy
}

// ✅ CORRECT — use index to avoid copy
for i := range largeSlice {
    process(&largeSlice[i])
}
```

### Creating HTTP Client Per Request

```go
// ❌ WRONG — new client (and transport) every request
func fetchOrder(url string) (*Order, error) {
    client := &http.Client{} // new connection pool each time
    resp, err := client.Get(url)
    // ...
}

// ✅ CORRECT — package-level client, reuses connections
var httpClient = &http.Client{
    Timeout: 10 * time.Second,
}

func fetchOrder(url string) (*Order, error) {
    resp, err := httpClient.Get(url)
    // ...
}
```

### Interface Conversion on Hot Path

```go
// ❌ WRONG — interface boxing causes escape to heap
func sum(values []int) int {
    var total any = 0 // escapes to heap
    for _, v := range values {
        total = total.(int) + v // assertion + boxing each iteration
    }
    return total.(int)
}

// ✅ CORRECT — concrete type, stays on stack
func sum(values []int) int {
    var total int
    for _, v := range values {
        total += v
    }
    return total
}
```

## Green Tea GC (Go 1.26 Default)

The Green Tea garbage collector is enabled by default in Go 1.26, providing 10-40% reduction in GC overhead for most workloads.

- Works at page level: scans entire pages globally, tracks objects locally within pages
- No code changes needed — it's transparent
- Opt-out: `GOEXPERIMENT=nogreenteagc` (will be removed in Go 1.27)
- If profiling shows GC regression, file a bug with `runtime/pprof` data

### Impact on Profiling

Green Tea changes GC trace output format. When using `GODEBUG=gctrace=1`:
- Overall GC time should decrease
- Mark phase may show different patterns
- Use `go tool pprof` as before — the interface is unchanged

## Stack-Allocated Slices (Go 1.25+)

Starting in Go 1.25, the compiler allocates more slice backing stores on the
stack instead of the heap, reducing heap allocations automatically. This means:

- Small slices with known capacity may no longer show up in `allocs/op`
- Escape analysis output (`-gcflags='-m'`) may show fewer escapes for slices
- No code changes needed — the improvement is transparent

See: perf-reviewer agent for detection commands and review workflow.
See: go-stdlib-patterns skill for io.ReadAll dangers and strings.Builder usage.
See: go-concurrency skill for lock contention and concurrent I/O patterns.

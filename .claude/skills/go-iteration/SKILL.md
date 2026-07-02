---
name: go-iteration
description: >-
  Go iteration patterns — traditional range, range-over-func (Go 1.23+),
  push vs pull iterators, stdlib iterators (iter, slices, maps packages),
  and the channel vs iterator decision.
when_to_use: >-
  Use when choosing an iteration strategy, implementing custom iterators,
  using range-over-func, or when iter.Seq/iter.Seq2 usage is unclear.
  Triggers: "iterator", "range over a function", "iter.Seq", "yield",
  "lazy sequence", "channel vs iterator", "slices.Values/maps.Keys".
metadata:
  author: koopa
  version: "1.0"
  lang: go
---

# Go Iteration Patterns

## Iterator Decision Tree

```
What are you iterating over?
├─ Slice, map, string, channel → range (built-in)
├─ Custom data source → Do you need to PRODUCE values lazily?
│   ├─ No  → Return a slice (simplest)
│   ├─ Yes → Is the consumer in a different goroutine?
│       ├─ Yes → Channel (concurrent producer/consumer)
│       └─ No  → iter.Seq / iter.Seq2 (Go 1.23+)
│           ├─ Single value → iter.Seq[V]
│           └─ Key-value pair → iter.Seq2[K, V]
```

## Traditional Range (Always Valid)

```go
// Slice — index + value
for i, order := range orders {
    process(i, order)
}

// Map — key + value (random order)
for key, value := range config {
    fmt.Println(key, value)
}

// String — index + rune (NOT byte)
for i, r := range "héllo" {
    fmt.Printf("%d: %c\n", i, r) // byte index, rune value
}

// Channel — blocks until closed
for msg := range ch {
    handle(msg)
}

// Integer (Go 1.22+)
for i := range 10 {
    fmt.Println(i) // 0..9
}
```

## Range-Over-Func (Go 1.23+)

### Core Concept

A function that accepts a `yield` callback. When used with `range`, the
loop body becomes the yield callback:

```go
// iter.Seq[V] — single value iterator
func Fibonacci() iter.Seq[int] {
    return func(yield func(int) bool) {
        a, b := 0, 1
        for yield(a) {
            a, b = b, a+b
        }
    }
}

// Usage — range calls yield for each iteration
for n := range Fibonacci() {
    if n > 100 {
        break // yield returns false, iterator stops
    }
    fmt.Println(n)
}
```

### iter.Seq vs iter.Seq2

```go
// iter.Seq[V] — single value
type Seq[V any] func(yield func(V) bool)

// iter.Seq2[K, V] — key-value pair
type Seq2[K, V any] func(yield func(K, V) bool)
```

| Use | Type | Example |
|-----|------|---------|
| Values only | `iter.Seq[V]` | `Fibonacci()`, `Lines(reader)` |
| Index + value | `iter.Seq2[int, V]` | `Enumerate(seq)` |
| Key + value | `iter.Seq2[K, V]` | `maps.All(m)` |
| Error iteration | `iter.Seq2[V, error]` | `Rows(query)` (see below) |

### Error Handling Pattern

```go
// Return errors as the second value in Seq2
func QueryRows(ctx context.Context, q *db.Queries) iter.Seq2[Order, error] {
    return func(yield func(Order, error) bool) {
        rows, err := q.ListOrders(ctx)
        if err != nil {
            yield(Order{}, fmt.Errorf("listing orders: %w", err))
            return
        }
        for _, row := range rows {
            order, err := mapOrder(row)
            if !yield(order, err) {
                return
            }
        }
    }
}

// Consumer checks error on each iteration
for order, err := range QueryRows(ctx, queries) {
    if err != nil {
        return fmt.Errorf("iterating orders: %w", err)
    }
    process(order)
}
```

### Writing Custom Iterators

```go
// Filter — returns new iterator that skips non-matching elements
func Filter[V any](seq iter.Seq[V], fn func(V) bool) iter.Seq[V] {
    return func(yield func(V) bool) {
        for v := range seq {
            if fn(v) {
                if !yield(v) {
                    return
                }
            }
        }
    }
}

// Map — transforms each element
func Map[V, U any](seq iter.Seq[V], fn func(V) U) iter.Seq[U] {
    return func(yield func(U) bool) {
        for v := range seq {
            if !yield(fn(v)) {
                return
            }
        }
    }
}

// Take — returns first n elements
func Take[V any](seq iter.Seq[V], n int) iter.Seq[V] {
    return func(yield func(V) bool) {
        i := 0
        for v := range seq {
            if i >= n {
                return
            }
            if !yield(v) {
                return
            }
            i++
        }
    }
}
```

## Stdlib Iterator Functions (Go 1.23+)

### slices Package

```go
import "slices"

// slices.All — index + value iterator
for i, v := range slices.All(orders) {
    fmt.Println(i, v)
}

// slices.Values — value-only iterator
for v := range slices.Values(orders) {
    process(v)
}

// slices.Backward — reverse iteration
for i, v := range slices.Backward(orders) {
    fmt.Println(i, v)
}

// slices.Collect — materialize iterator into slice
result := slices.Collect(Filter(slices.Values(orders), isActive))

// slices.Sorted — sort an iterator into a new slice
sorted := slices.Sorted(maps.Keys(m))
```

### maps Package

```go
import "maps"

// maps.Keys — iterator over keys
for k := range maps.Keys(m) {
    fmt.Println(k)
}

// maps.Values — iterator over values
for v := range maps.Values(m) {
    process(v)
}

// maps.All — key-value iterator
for k, v := range maps.All(m) {
    fmt.Println(k, v)
}

// Deterministic key iteration
for _, k := range slices.Sorted(maps.Keys(m)) {
    fmt.Println(k, m[k])
}
```

## Push vs Pull Iterators

### Push Iterator (iter.Seq — Standard)

The iterator drives the loop. It calls `yield` for each value:

```go
// Push: iterator controls flow
func Lines(r io.Reader) iter.Seq[string] {
    return func(yield func(string) bool) {
        scanner := bufio.NewScanner(r)
        for scanner.Scan() {
            if !yield(scanner.Text()) {
                return // consumer broke out of range
            }
        }
    }
}
```

### Pull Iterator (iter.Pull — When Needed)

Convert push to pull when you need to interleave two iterators or
consume values one at a time outside of `range`:

```go
// Pull: consumer controls flow
next, stop := iter.Pull(Lines(file))
defer stop() // MUST call stop to release resources

// Read exactly 3 lines
for i := 0; i < 3; i++ {
    line, ok := next()
    if !ok {
        break
    }
    process(line)
}
```

### Decision: Push vs Pull

```
Can you consume values in a simple range loop?
├─ Yes → Push (iter.Seq) — simpler, no cleanup needed
└─ No  → Do you need to interleave multiple iterators?
    ├─ Yes → Pull (iter.Pull) — MUST defer stop()
    └─ No  → Do you need to consume N values then stop?
        ├─ Yes → Pull, or use Take(seq, n) with push
        └─ No  → Push (default choice)
```

## Channel vs Iterator

```
Is the producer in a separate goroutine from the consumer?
├─ Yes → Channel
│   - Producer and consumer run concurrently
│   - Backpressure via buffered channel
│   - Example: reading from network stream in background
├─ No  → Is this a one-shot sequential scan?
│   ├─ Yes → iter.Seq (Go 1.23+) or return []T
│   │   - No goroutine overhead
│   │   - Composable (Filter, Map, Take)
│   │   - break works naturally
│   └─ No  → Is the data unbounded / real-time?
│       ├─ Yes → Channel (goroutine produces indefinitely)
│       └─ No  → iter.Seq or []T
```

```go
// ❌ WRONG — channel for sequential scan (unnecessary goroutine)
func allOrders(orders []Order) <-chan Order {
    ch := make(chan Order)
    go func() {
        defer close(ch)
        for _, o := range orders {
            ch <- o
        }
    }()
    return ch
}

// ✅ CORRECT — iterator, no goroutine overhead
func allOrders(orders []Order) iter.Seq[Order] {
    return slices.Values(orders)
}
```

## Anti-Patterns

### Forgetting to Check yield Return

```go
// ❌ WRONG — ignores yield return, can't break out of range
func Numbers() iter.Seq[int] {
    return func(yield func(int) bool) {
        for i := 0; ; i++ {
            yield(i) // if consumer breaks, this keeps running!
        }
    }
}

// ✅ CORRECT — check yield, stop when false
func Numbers() iter.Seq[int] {
    return func(yield func(int) bool) {
        for i := 0; ; i++ {
            if !yield(i) {
                return
            }
        }
    }
}
```

### Not Calling stop() on Pull Iterator

```go
// ❌ WRONG — resource leak
next, stop := iter.Pull(seq)
v, _ := next()
// stop never called — goroutine leak!

// ✅ CORRECT — always defer stop
next, stop := iter.Pull(seq)
defer stop()
```

### Using Channel When Iterator Suffices

```go
// ❌ WRONG — goroutine + channel for simple transformation
func doubled(nums []int) <-chan int {
    ch := make(chan int)
    go func() {
        defer close(ch)
        for _, n := range nums {
            ch <- n * 2
        }
    }()
    return ch
}

// ✅ CORRECT — no goroutine needed
func doubled(nums []int) iter.Seq[int] {
    return func(yield func(int) bool) {
        for _, n := range nums {
            if !yield(n * 2) {
                return
            }
        }
    }
}
```

### Materializing When Streaming Would Work

```go
// ❌ WRONG — collects all results into slice, then iterates
results := slices.Collect(expensiveIterator())
for _, r := range results {
    process(r)
}

// ✅ CORRECT — stream directly if you only need one pass
for r := range expensiveIterator() {
    process(r)
}
```

See: go-concurrency skill for channel patterns and goroutine lifecycle.
See: go-types skill for slice behavior and range semantics.
See: go-generics skill for generic iterator utility functions.

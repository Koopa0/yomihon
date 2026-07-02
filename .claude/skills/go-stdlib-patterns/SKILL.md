---
name: go-stdlib-patterns
description: >-
  Go standard library patterns — io.Reader/Writer composition, encoding/json
  pitfalls, time.Time handling, modern sort/slices API, strings/bytes
  builders, and advanced context usage.
when_to_use: >-
  Use when working with io pipelines, JSON encoding/decoding, time
  comparisons, sorting, string building, or context value design. Also use
  when io.ReadAll vs streaming or json.Decoder vs json.Unmarshal decisions
  are unclear. Trigger keywords: io.Copy, io.TeeReader, io.LimitReader,
  json struct tags, time.Time, slices.Sort, strings.Builder,
  context.WithValue.
metadata:
  author: koopa
  version: "1.0"
  lang: go
---

# Go Standard Library Patterns

## Decision Trees

### io Pipeline

```
Do you need to TRANSFORM data as it flows?
├─ Yes → io.TeeReader, io.Pipe, or wrap with custom Reader
│   Example: hash while copying, decompress while reading
├─ No  → Do you need to COPY from Reader to Writer?
│   ├─ Yes → io.Copy (buffered, streaming)
│   │   NEVER io.ReadAll + Write (loads entire content into memory)
│   └─ No  → Do you need to LIMIT how much you read?
│       ├─ Yes → io.LimitReader (prevents OOM from untrusted input)
│       └─ No  → Pass the io.Reader directly to the consumer
```

### json Decoder vs Unmarshal

```
Is the JSON coming from an io.Reader (http body, file, network)?
├─ Yes → json.NewDecoder(r).Decode(&v)
│   - Streams, constant memory
│   - Handles single JSON value from a stream
├─ No  → Is the JSON already a []byte in memory?
│   ├─ Yes → json.Unmarshal(data, &v)
│   └─ No  → Get it as io.Reader if possible, then Decoder
```

## MUST / NEVER Rules

- **NEVER `io.ReadAll` an unbounded reader** (http body, network) — stream with
  `json.NewDecoder(r).Decode(&v)` or `io.Copy`. `io.ReadAll` is acceptable only for
  small, bounded files; test code; or when bytes are needed for multiple operations.
  (Go 1.26+ makes `io.ReadAll` ~2x faster with ~50% less memory transparently — but
  the streaming rule still holds for unbounded input.)
- **NEVER use `omitempty` on `int`, `bool`, or any type whose zero value is a valid
  business value** — `0` and `false` get silently dropped. Use a pointer (`*T`) or
  drop `omitempty`. Only use it on `string`, `*T`, `[]T`, `map` where empty/nil means
  "not provided".
- **NEVER compare `time.Time` with `==`** — use `t1.Equal(t2)`. `==` also compares the
  monotonic clock and `*Location`; the same instant in different zones is `Equal` but
  not `==`. Use `Before` / `After` for ordering.
- **NEVER use `sort.Slice` / `sort.Sort`** — use the `slices` package.
- **NEVER use `string` or `int` as a context key** — collision risk across packages.
  Define an unexported struct type and provide `With<Name>` / `<Name>From` functions.
- **Context values are for cross-cutting concerns only** (request ID, trace span, auth
  user identity). Business data flows through function parameters — never the logger,
  db pool, or values the store layer reads.

## Most-Used Snippets

### Stream a request body (don't ReadAll)

```go
// ❌ WRONG — loads entire body into memory, OOM risk
body, err := io.ReadAll(r.Body)
if err != nil { return err }
var order Order
if err := json.Unmarshal(body, &order); err != nil { return err }

// ✅ CORRECT — streams directly, constant memory
var order Order
if err := json.NewDecoder(r.Body).Decode(&order); err != nil {
    return fmt.Errorf("decoding request: %w", err)
}
```

### The omitempty trap

```go
type Config struct {
    Port    int  `json:"port,omitempty"`    // ❌ port=0 is omitted!
    Verbose bool `json:"verbose,omitempty"` // ❌ verbose=false is omitted!
}

cfg := Config{Port: 0, Verbose: false}
data, _ := json.Marshal(cfg)
// Output: {} — both fields silently dropped!

// ✅ CORRECT — use pointer for truly optional fields
type Config struct {
    Port    int     `json:"port"`            // always included
    Verbose bool    `json:"verbose"`         // always included
    Notes   *string `json:"notes,omitempty"` // only omit when nil (truly absent)
}
```

### slices for sorting and search (Go 1.21+)

```go
import "slices"

// Sort by single field
slices.SortFunc(orders, func(a, b Order) int {
    return cmp.Compare(a.Total, b.Total)
})

// Sort by multiple fields
slices.SortFunc(orders, func(a, b Order) int {
    if c := cmp.Compare(a.Status, b.Status); c != 0 {
        return c
    }
    return cmp.Compare(a.CreatedAt.Unix(), b.CreatedAt.Unix())
})

// Binary search
i, found := slices.BinarySearchFunc(orders, target, func(o Order, t string) int {
    return cmp.Compare(o.ID, t)
})

// Deterministic map iteration (Go 1.23+)
keys := slices.Sorted(maps.Keys(m))
```

### strings.Builder for concatenation

```go
// ❌ WRONG — O(n^2) string concatenation
var result string
for _, s := range items {
    result += s + ", " // allocates new string each iteration
}

// ✅ CORRECT — O(n) with Builder
var b strings.Builder
for i, s := range items {
    if i > 0 {
        b.WriteString(", ")
    }
    b.WriteString(s)
}
result := b.String()
```

For known sizes, pre-allocate: `b.Grow(estimatedSize)`.

### time comparison

```go
// ❌ WRONG — == compares monotonic clock + location, fails across zones
if t1 == t2 { ... }

// ✅ CORRECT — Equal compares wall clock only
if t1.Equal(t2) { ... }

// For sorting/ordering
if t1.Before(t2) { ... }
if t1.After(t2) { ... }
```

### context custom key type (mandatory)

```go
// ❌ WRONG — string key, collision risk across packages
ctx = context.WithValue(ctx, "userID", userID)

// ✅ CORRECT — unexported struct type, zero-size, collision-proof
type userIDKey struct{}

func WithUserID(ctx context.Context, id string) context.Context {
    return context.WithValue(ctx, userIDKey{}, id)
}

func UserIDFrom(ctx context.Context) (string, bool) {
    id, ok := ctx.Value(userIDKey{}).(string)
    return id, ok
}
```

## Navigation — Deep Reference

| Topic | File |
|-------|------|
| io composition (TeeReader, LimitReader, MultiWriter), io anti-patterns, json.Number, json.RawMessage, json anti-patterns | references/io-json-streaming.md |
| time storage/UTC, time.Tick leaks, Duration construction, time anti-patterns, bytes.Buffer.Peek, strings.Cut, strings.EqualFold, strings anti-patterns | references/time-and-strings.md |
| context.WithoutCancel, context.AfterFunc, context anti-patterns | references/context-patterns.md |

See: go-concurrency skill for context cancellation and timeout patterns.
See: go-types skill for type assertion patterns used with context values.

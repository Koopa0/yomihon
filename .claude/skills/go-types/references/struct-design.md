# Struct Design, Immutable Catalogs, and Embedding

Deep reference for designing struct types: field ordering, zero value,
constructors, read-only catalog data, and the embedding decision.

## Struct Design

### Field Ordering: Largest First (Memory Alignment)

```go
// ❌ WRONG — padding wastes 7 bytes per struct
type Bad struct {
    active bool   // 1 byte + 7 padding
    count  int64  // 8 bytes
    name   string // 16 bytes
}
// sizeof: 32 bytes

// ✅ CORRECT — no padding
type Good struct {
    name   string // 16 bytes
    count  int64  // 8 bytes
    active bool   // 1 byte + 0 padding (end of struct)
}
// sizeof: 25 bytes (rounded to 32 for alignment, but fewer wasted bytes internally)
```

For hot-path structs only. Don't reorder fields in domain types for micro-optimization.

### Zero Value Design

Design structs so the zero value is useful:

```go
// ✅ Good zero value — sync.Mutex, bytes.Buffer work at zero value
type Server struct {
    mu       sync.Mutex
    handlers map[string]http.Handler // nil is ok if you init before first write
}

// ✅ Explicit init when zero value is dangerous
func NewServer() *Server {
    return &Server{
        handlers: make(map[string]http.Handler),
    }
}
```

### Constructor Pattern

```go
// Required dependencies → constructor parameters
func NewStore(dbtx db.DBTX) *Store {
    return &Store{q: db.New(dbtx)}
}

// Optional configuration → functional options (only if 3+ options)
// For 0-2 options, use zero value defaults
```

See: go-philosophy.md rule for functional options threshold.

## Immutable Catalog Data — Accessor Functions, Not Package Vars

Go has no `const` for struct values. `var X = Foo{...}` at package level is **externally mutable** — any caller can write `pkg.X.Field = "..."` at runtime and corrupt the value for the whole process.

```go
// ❌ WRONG — exported var is mutable from outside the package.
// A caller can do `errcat.NotFound.Message = "pwned"` and the change
// persists for every subsequent reader.
var NotFound = ErrorInfo{
    Code:    "NOT_FOUND",
    Message: "Resource does not exist",
    Status:  http.StatusNotFound,
}
```

The idiomatic fix for "read-only catalog" is an **accessor function returning by value**:

```go
// ✅ CORRECT — accessor function. Caller gets a copy, cannot mutate
// the source. Compile-time name checking still works (errcat.NotFound
// is a type error if misspelled).
func NotFound() ErrorInfo {
    return ErrorInfo{
        Code:    "NOT_FOUND",
        Message: "Resource does not exist",
        Status:  http.StatusNotFound,
    }
}
```

Allocation cost is irrelevant for catalog data consumed at startup or on admin endpoints — the returned value is usually short-lived. For catalogs measured in hundreds of entries, this pattern allocates a few KB per read; for tens of entries it allocates a few hundred bytes. Not a concern.

Trade-off: callers lose `&pkg.NotFound` (taking the address). If a pointer is genuinely needed, use an unexported var + getter returning `*ErrorInfo` — but prefer copy semantics anyway, since shared mutable references defeat the read-only intent.

## Struct Embedding

### When to Embed

```
Is the outer type a "kind of" the inner type? (IS-A relationship)
├─ No  → don't embed, use a named field
├─ Yes → Does the outer type need ALL methods of the inner type promoted?
│   ├─ No  → don't embed, use a named field + delegate specific methods
│   └─ Yes → embed
```

### Embedding for Method Promotion

```go
// ✅ CORRECT — Logger wraps slog.Logger, IS-A logger
type Logger struct {
    *slog.Logger
}

// All slog.Logger methods promoted: l.Info(), l.Error(), etc.
```

### Embedding Pitfalls

```go
// ❌ WRONG — embedding interface in struct (zero value panics)
type CountingReader struct {
    io.Reader // embedded interface
    count int
}
cr := CountingReader{} // Reader is nil
cr.Read(buf)           // PANIC

// ✅ CORRECT — named field
type CountingReader struct {
    r     io.Reader
    count int
}
```

```go
// ❌ WRONG — embedding exposes methods you don't want
type UserStore struct {
    *pgxpool.Pool // exposes Pool.Exec, Pool.Query, etc. to callers
}

// ✅ CORRECT — named field, expose nothing
type UserStore struct {
    q *db.Queries
}
```

**Rule**: NEVER embed `*pgxpool.Pool`, `*sql.DB`, or similar infrastructure types.
Use named fields.

See: interfaces.md rule for interface embedding pitfall.
See: go-interfaces skill for full interface design patterns and testing with interfaces.

---
name: go-generics
description: >-
  Go generics patterns -- decision tree for generics vs interfaces vs
  concrete types, constraint design, type parameter naming, generic
  utility patterns, and common pitfalls including comparable behavior.
when_to_use: >-
  Use when deciding whether generics are appropriate, designing type
  constraints, writing generic functions or types, when comparable
  behavior is unclear, or when generics interact with interfaces.
  Triggers: "should this be generic", "type parameter", "constraint",
  "[T any]", "comparable", "generic function/type".
metadata:
  author: koopa
  version: "1.0"
  lang: go
---

# Go Generics Patterns

## Decision Tree: Should You Use Generics?

```
Is the logic IDENTICAL across multiple concrete types?
├─ No  → concrete types. Each type has its own logic.
│   Example: Store[T] — WRONG, each store has different queries
├─ Yes → Can an interface express the requirement?
│   ├─ Yes → Does the function need to RETURN the same type it received?
│   │   ├─ No  → use interface (simpler)
│   │   │   Example: func Process(r io.Reader) — Reader is enough
│   │   └─ Yes → generics (interface can't express "return T")
│   │       Example: func Clone[T any](s []T) []T
│   └─ No  → generics
│       Example: func Map[T, U any](s []T, fn func(T) U) []U
│
└─ Does it eliminate >3 truly identical implementations?
    ├─ Yes → generics justified
    │   Example: func Contains[T comparable](s []T, v T) bool
    └─ No  → concrete types. Duplication is cheaper than abstraction.
```

**The bar**: generics must eliminate 3+ implementations that are truly identical.
2 similar functions is not enough — just write both.

## Constraint Design

### Built-In Constraints

| Constraint | Meaning | Use When |
|------------|---------|----------|
| `any` | No constraint | Container types, utility functions |
| `comparable` | Supports `==` and `!=` | Map keys, equality checks |
| `cmp.Ordered` | Supports `<`, `>`, `<=`, `>=` | Sorting, min/max |

### Custom Constraints

```go
// Union constraint — specific types only
type Number interface {
    ~int | ~int32 | ~int64 | ~float32 | ~float64
}

func Sum[N Number](nums []N) N {
    var total N
    for _, n := range nums {
        total += n
    }
    return total
}
```

### The `~` Operator

```go
// Without ~: only exact type matches
type Exact interface { int | string }

// With ~: matches named types with underlying type
type Approx interface { ~int | ~string }

type UserID int       // underlying type is int
type Username string  // underlying type is string

// Sum[Exact] — UserID does NOT satisfy (it's not int, it's UserID)
// Sum[Approx] — UserID satisfies (~int matches any type with underlying int)
```

**Rule**: almost always use `~` for numeric/string constraints.

### Constraint Best Practices

```go
// ✅ Use stdlib constraints when possible
import "cmp"

func Max[T cmp.Ordered](a, b T) T {
    if a > b {
        return a
    }
    return b
}

// ❌ WRONG — reinventing what cmp.Ordered provides
type Ordered interface {
    ~int | ~int8 | ~int16 | ~int32 | ~int64 |
    ~float32 | ~float64 | ~string
}
```

## The `comparable` Pitfall

`comparable` means the type supports `==` at compile time. But some types that
satisfy `comparable` will **panic at runtime**.

```go
// This struct satisfies comparable at compile time
type Key struct {
    Name  string
    Tags  []string // slices are NOT comparable at runtime
}

func Index[K comparable, V any](m map[K]V, key K) (V, bool) {
    v, ok := m[key]
    return v, ok
}

// Compiles fine, but PANICS at runtime:
m := make(map[Key]int)
m[Key{Name: "a", Tags: []string{"x"}}] = 1 // runtime panic: unhashable type
```

**Rule**: only use `comparable` with types you KNOW are safely comparable
(primitives, strings, structs with only primitive fields). If a struct contains
`[]byte`, `map`, `func`, or `any` fields, it will panic as a map key.

## Approved Patterns in This Project

### Generic JSON Helpers

```go
package order

import (
    "encoding/json"
    "fmt"
    "net/http"
)

func decode[T any](r *http.Request) (T, error) {
    var v T
    if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
        return v, fmt.Errorf("decoding request body: %w", err)
    }
    return v, nil
}

func encode[T any](w http.ResponseWriter, status int, v T) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    if err := json.NewEncoder(w).Encode(v); err != nil {
        // best-effort: headers already sent
        slog.Error("encoding response", "error", err)
    }
}
```

### Pointer Helper

```go
// ptr[T] helper — DEPRECATED by new(expr) in Go 1.26
// OLD:
func ptr[T any](v T) *T { return &v }
age := ptr(25)

// NEW (Go 1.26+): use built-in new with initial value
age := new(25)
name := new("hello")

// Usage (Go 1.26+)
params := db.CreateOrderParams{
    Notes: new("special instructions"), // *string
}
```

### Collection Utilities

```go
// Contains checks if a value exists in a slice.
func Contains[T comparable](s []T, v T) bool {
    for _, item := range s {
        if item == v {
            return true
        }
    }
    return false
}

// Map transforms a slice using a function.
func Map[T, U any](s []T, fn func(T) U) []U {
    result := make([]U, len(s))
    for i, v := range s {
        result[i] = fn(v)
    }
    return result
}

// Filter returns elements that satisfy the predicate.
func Filter[T any](s []T, fn func(T) bool) []T {
    var result []T
    for _, v := range s {
        if fn(v) {
            result = append(result, v)
        }
    }
    return result
}
```

**Note**: Go 1.21+ has `slices.Contains`. Prefer stdlib over project utilities
when available. Only write custom generics when stdlib doesn't cover the case.

## Generics + Interfaces Interaction

### Generic Function Accepting Interface

```go
// ✅ Interface for behavior, generic for type safety
func Process[T fmt.Stringer](items []T) []string {
    result := make([]string, len(items))
    for i, item := range items {
        result[i] = item.String()
    }
    return result
}
```

### When to Use Interface vs Generic

```
Does the function need to STORE or RETURN the type parameter?
├─ Yes → generics (interface loses the type)
│   func Clone[T any](s []T) []T
│   func Cache[K comparable, V any]
├─ No  → Does the function only CALL methods on the type?
│   ├─ Yes → interface (simpler, more flexible)
│   │   func Process(r io.Reader) error
│   └─ No  → depends on the operation
│       Equality check → comparable constraint
│       Ordering → cmp.Ordered constraint
│       Arithmetic → custom Number constraint
```

### Generic Type with Interface Method

```go
// Result wraps a value with an error — generic container
type Result[T any] struct {
    Value T
    Err   error
}

func NewResult[T any](v T, err error) Result[T] {
    return Result[T]{Value: v, Err: err}
}

func (r Result[T]) Unwrap() (T, error) {
    return r.Value, r.Err
}
```

## Type Parameter Naming

| Count | Convention | Example |
|-------|-----------|---------|
| Single | `T` | `func Clone[T any](s []T) []T` |
| Two (input/output) | `T`, `U` | `func Map[T, U any](s []T, fn func(T) U) []U` |
| Key/Value | `K`, `V` | `func Keys[K comparable, V any](m map[K]V) []K` |
| Constrained | short descriptive | `func Sum[N Number](nums []N) N` |

**NEVER** use long names like `Element`, `Item`, `Value` — keep them 1-2 characters.

## Anti-Patterns

### Single-Type Generic

```go
// ❌ WRONG — only ever instantiated with Order
func Process[T Order](t T) error {
    // ... logic specific to Order ...
}

// ✅ CORRECT — just use the concrete type
func Process(o Order) error {
    // ...
}
```

### Generic Store / Repository

```go
// ❌ WRONG — each store has completely different queries
type Store[T any] struct {
    db *pgxpool.Pool
}

func (s *Store[T]) Create(ctx context.Context, item T) error { ... }
func (s *Store[T]) FindByID(ctx context.Context, id string) (T, error) { ... }

// ✅ CORRECT — each feature has its own store with specific queries
type OrderStore struct { q *db.Queries }
type UserStore struct { q *db.Queries }
```

This is a Java/C# pattern. In Go, each store has different SQL, different
mapping, different error handling. Generic CRUD doesn't work.

### Over-Constrained

```go
// ❌ WRONG — constraint with 15 types
type AllNumbers interface {
    ~int | ~int8 | ~int16 | ~int32 | ~int64 |
    ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
    ~float32 | ~float64
}

// ✅ CORRECT — use stdlib constraint
import "cmp"
func Max[T cmp.Ordered](a, b T) T { ... }
```

### Generic for 2 Types

```go
// ❌ WRONG — only Order and User, just write both
func Validate[T interface{ Order | User }](t T) error { ... }

// ✅ CORRECT — two concrete functions
func ValidateOrder(o Order) error { ... }
func ValidateUser(u User) error { ... }
```

### Premature Generic Abstraction

```go
// ❌ WRONG — building a generic "framework" for future use
type Pipeline[In, Out any] struct {
    stages []Stage[In, Out]
}

// ✅ CORRECT — write concrete code for the current need
func processOrders(ctx context.Context, orders []Order) ([]Result, error) {
    // ...
}
```

## Self-Referential Generics (Go 1.26+)

```go
// Self-referential generics (Go 1.26+)
type Adder[A Adder[A]] interface {
    Add(A) A
}
// Use sparingly — this is advanced. Most code doesn't need this.
```

See: go-philosophy.md rule — "Is this abstraction needed TODAY?"
See: go-types skill for receiver decision tree and value vs pointer semantics.
See: go-interfaces skill for when to use interfaces instead of generics.

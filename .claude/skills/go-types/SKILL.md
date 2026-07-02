---
name: go-types
description: >-
  Go type system patterns reference — receiver selection decision tree, value
  vs pointer semantics, nil pitfalls (typed nil in interface), slice
  append/backing-array behavior, map reference semantics, type assertions and
  switches, struct design (field ordering, zero value, constructors), and
  embedding.
when_to_use: >-
  Use when designing Go types or structs, choosing between value and pointer
  receivers, deciding pass-by-value vs pointer, debugging nil interface or
  typed-nil surprises, when slice or map mutation/append behavior is unclear,
  when writing type assertions or type switches, or when struct embedding
  decisions are needed.
metadata:
  author: koopa
  version: "1.0"
  lang: go
---

# Go Type System Patterns

## Receiver Decision Tree

This is the most referenced decision in the project. All reviewer checks trace back here.

```
Does the method MODIFY the receiver?
├─ Yes → pointer receiver (*T)
├─ No  → Is the receiver LARGE (>= 3 fields or contains slice/map)?
│   ├─ Yes → pointer receiver (*T) for performance
│   └─ No  → Does any OTHER method on this type use pointer receiver?
│       ├─ Yes → pointer receiver (*T) for CONSISTENCY
│       └─ No  → value receiver (T)
```

### Consistency Rule

**All methods on a type MUST use the same receiver kind.** Mixing causes subtle bugs
with interface satisfaction and confusing semantics.

```go
// ❌ WRONG — mixed receivers on same type
func (o Order) Total() int          { return o.total }
func (o *Order) SetStatus(s Status) { o.status = s }

// ✅ CORRECT — all pointer (because SetStatus mutates)
func (o *Order) Total() int          { return o.total }
func (o *Order) SetStatus(s Status)  { o.status = s }
```

### When Value Receiver Is Correct

```go
// Small, immutable types — value receiver is fine
type Point struct{ X, Y float64 }

func (p Point) Distance(other Point) float64 {
    dx := p.X - other.X
    dy := p.Y - other.Y
    return math.Sqrt(dx*dx + dy*dy)
}

// String type wrappers
type Status string

func (s Status) IsValid() bool {
    switch s {
    case StatusPending, StatusActive, StatusDone:
        return true
    }
    return false
}
```

### Method Set Implications

```
Type T  → method set: value receivers only
Type *T → method set: value + pointer receivers
```

This means:
```go
type Saver interface { Save() }

type Doc struct{}
func (d *Doc) Save() {} // pointer receiver

var s Saver = Doc{}   // ❌ compile error: Doc does not implement Saver
var s Saver = &Doc{}  // ✅ *Doc implements Saver
```

## Value vs Pointer Semantics

### Decision Table

| Type | Default | Why |
|------|---------|-----|
| `int`, `string`, `bool`, `float64` | Value | Cheap to copy, immutable |
| Small struct (1-2 fields, no slice/map) | Value | Cheap, clear semantics |
| Large struct (3+ fields) | Pointer | Avoid copy overhead |
| Struct with `sync.Mutex` | Pointer | MUST NOT copy mutex |
| Struct with slice/map field | Pointer | Shares underlying data — pointer makes intent explicit |
| Interface | Value | Already a pointer internally |

### The Mutex Rule

NEVER copy a struct containing `sync.Mutex`, `sync.RWMutex`, `sync.WaitGroup`,
`sync.Once`, or `bytes.Buffer`. Always use pointer.

```go
// ❌ WRONG — copying mutex
type Cache struct {
    mu    sync.Mutex
    items map[string]string
}
c1 := Cache{items: map[string]string{}}
c2 := c1 // copies mutex — undefined behavior

// ✅ CORRECT — pointer, or don't copy
c2 := &c1 // shares the same mutex
```

`go vet` catches this with `copylocks` check.

## Nil Pitfalls

### Typed Nil in Interface — The Classic Trap

```go
// ❌ BUG — returns typed nil, but caller sees non-nil error
func validate(s string) error {
    var err *ValidationError // typed nil pointer
    if s == "" {
        err = &ValidationError{Field: "name"}
    }
    return err // even when err is nil (*ValidationError), interface is non-nil!
}

func main() {
    err := validate("hello")
    if err != nil {
        fmt.Println("error!") // THIS PRINTS — err is (*ValidationError)(nil), not nil
    }
}
```

```go
// ✅ CORRECT — return explicit nil for the interface
func validate(s string) error {
    if s == "" {
        return &ValidationError{Field: "name"}
    }
    return nil // untyped nil — interface is nil
}
```

**Rule**: NEVER assign a typed nil to an interface variable and return it.
Always return `nil` directly for the "no error" path.

### Nil Slice vs Nil Map

```go
// Nil slice is safe — append, len, cap, range all work
var s []string
s = append(s, "ok") // works
len(s)              // 0
for range s {}      // no iterations

// Nil map is NOT safe for writes
var m map[string]int
_ = m["key"]       // safe — returns zero value
m["key"] = 1       // ❌ PANIC: assignment to nil map

// ✅ Always initialize maps before writing
m = make(map[string]int)
m["key"] = 1
```

### Nil Interface Methods

```go
var r io.Reader // nil interface
r.Read(buf)     // ❌ PANIC: nil pointer dereference

// Always check interface for nil before calling methods
if r != nil {
    r.Read(buf)
}
```

## Type Assertions and Switches

### Decision: When to Use Which

```
Do you know the exact concrete type?
├─ Yes → type assertion: v, ok := x.(ConcreteType)
├─ No  → Do you need to handle MULTIPLE possible types?
│   ├─ Yes → type switch
│   └─ No  → errors.As for error chains, type assertion otherwise
```

### Type Assertion (Always Use Comma-Ok)

```go
// ❌ WRONG — panics if wrong type
s := val.(string)

// ✅ CORRECT — comma-ok pattern
s, ok := val.(string)
if !ok {
    return fmt.Errorf("expected string, got %T", val)
}
```

### Type Switch

```go
func describe(v any) string {
    switch x := v.(type) {
    case string:
        return fmt.Sprintf("string of length %d", len(x))
    case int:
        return fmt.Sprintf("integer %d", x)
    case nil:
        return "nil"
    default:
        return fmt.Sprintf("unknown type %T", x)
    }
}
```

**Rule**: Type switches on `error` should use `errors.Is`/`errors.As` instead.
See: error-patterns skill.

## Anti-Patterns

### Returning Typed Nil

```go
// ❌ Every function returning an interface must return untyped nil
func findUser(id string) error {
    var err *NotFoundError
    // ... logic that might set err ...
    return err // BUG: typed nil
}
```

### Ignoring Append Return

```go
// ❌ Result of append is never used — items unchanged
func collect(items []string, new string) {
    append(items, new) // staticcheck: SA4010
}
```

Full append/backing-array semantics: `references/slices-and-maps.md`.

### Copying Sync Types

```go
// ❌ go vet: copylocks
func process(c Cache) { // Cache contains sync.Mutex — copied on call
    c.mu.Lock()
}

// ✅ Pass by pointer
func process(c *Cache) {
    c.mu.Lock()
}
```

### String Conversion Assumptions

```go
// ❌ Assumes UTF-8 byte indexing = character indexing
first := name[0] // byte, not rune

// ✅ Use range for rune iteration
for i, r := range name {
    if i == 0 {
        first = r
    }
}

// ✅ Or convert to rune slice (allocates)
runes := []rune(name)
first := runes[0]
```

### Map Without Initialization

```go
// ❌ Nil map write panics
type Registry struct {
    items map[string]Item
}
r := Registry{}
r.items["a"] = Item{} // PANIC

// ✅ Constructor initializes
func NewRegistry() *Registry {
    return &Registry{items: make(map[string]Item)}
}
```

## Navigation — Deep References

Read these on demand; they are not loaded with the skill.

| Topic | File | When to read |
|-------|------|--------------|
| Slice behavior (append/backing array, slice as function parameter, pre-allocation) and map behavior (reference semantics, random iteration order, comma-ok) | `references/slices-and-maps.md` | When slice or map mutation, append, or iteration behavior is unclear |
| Struct design (field ordering/alignment, zero value, constructor pattern), immutable catalog data (accessor functions vs package vars), and struct embedding (when to embed, method promotion, pitfalls) | `references/struct-design.md` | When designing structs or constructors, exposing read-only catalog data, or deciding whether to embed |

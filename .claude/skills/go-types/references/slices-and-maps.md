# Slice and Map Behavior

Deep reference for slice backing-array semantics and map reference semantics.
The companion MUST-NEVER rules (ignoring append return, nil map writes) live in
SKILL.md § Anti-Patterns.

## Slice Behavior

### Append May Return a New Backing Array

```go
a := make([]int, 0, 2)
a = append(a, 1, 2) // a: [1, 2], cap: 2

b := append(a, 3)   // cap exceeded → NEW backing array
// a is still [1, 2] — NOT affected by b's append
// b is [1, 2, 3] with new backing array

c := a[:1]           // c shares a's backing array
c[0] = 99            // a[0] is also 99!
```

**Rule**: ALWAYS reassign the result of `append`:
```go
// ❌ WRONG — result of append discarded
append(items, newItem)

// ✅ CORRECT
items = append(items, newItem)
```

### Slice as Function Parameter

```go
// Modifications to ELEMENTS are visible to caller (shared backing array)
func double(s []int) {
    for i := range s {
        s[i] *= 2 // caller sees this
    }
}

// Append is NOT visible to caller (may create new backing array)
func addItem(s []int, item int) {
    s = append(s, item) // caller does NOT see this
}

// ✅ CORRECT — return the new slice
func addItem(s []int, item int) []int {
    return append(s, item)
}
```

### Pre-Allocate When Length Is Known

```go
// ❌ WRONG — grows dynamically, multiple allocations
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

## Map Behavior

### Maps Are Reference Types

```go
func populate(m map[string]int) {
    m["key"] = 42 // caller sees this — maps are reference types
}

m := make(map[string]int)
populate(m)
fmt.Println(m["key"]) // 42
```

### Iteration Order Is Random

```go
// ❌ WRONG — test depends on map iteration order
for k, v := range m {
    fmt.Println(k, v) // order changes between runs
}

// ✅ CORRECT — sort keys first for deterministic output
keys := slices.Sorted(maps.Keys(m)) // Go 1.23+
for _, k := range keys {
    fmt.Println(k, m[k])
}
```

### Comma-Ok Pattern

```go
// Distinguish "key not found" from "key exists with zero value"
v, ok := m["key"]
if !ok {
    // key does not exist
}
```

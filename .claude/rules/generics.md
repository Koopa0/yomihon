---
paths:
  - "**/*.go"
---

# Generics Rules

## Decision Tree

Same logic for multiple types? → Interface sufficient? → Yes: use interface. No: generics justified only if it eliminates >3 identical implementations.

## Use For

- Container helpers: `decode[T]`, `encode[T]`
- Type-safe wrappers where `any` loses info
- Functions identical across types: `min`, `max`, `contains`, `filter`

## NEVER

- NEVER generics to avoid 2-3 concrete functions. Duplication is cheaper.
- NEVER `Store[T]` — each store has different queries.
- NEVER `Repository[T, ID]` — Java pattern.
- NEVER single-type instantiation: `Process[T Order]` → just use `Order`.

## Constraints

- `cmp.Ordered` (Go 1.21+), not `constraints.Ordered`
- `comparable` for map keys — but structs with `[]byte`/`map`/`func`/`any` fields panic at runtime
- `new(expr)` for pointer creation (Go 1.26+), not `ptr[T]` helper

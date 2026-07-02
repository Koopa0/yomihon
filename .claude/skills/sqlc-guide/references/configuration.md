# sqlc Configuration Reference

Extended configuration material for the sqlc-guide skill: per-feature generation, type overrides, nullable column handling, and query file organization. The base sqlc.yaml block lives in SKILL.md.

## Per-Feature Generation

For package-by-feature, configure per feature:

```yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "internal/order/query.sql"
    schema: "migrations/"
    gen:
      go:
        package: "order"
        out: "internal/order"
        sql_package: "pgx/v5"
        emit_json_tags: true
        emit_empty_slices: true
  - engine: "postgresql"
    queries: "internal/user/query.sql"
    schema: "migrations/"
    gen:
      go:
        package: "user"
        out: "internal/user"
        sql_package: "pgx/v5"
        emit_json_tags: true
        emit_empty_slices: true
```

## Type Overrides

### In sqlc.yaml

```yaml
overrides:
  - db_type: "uuid"
    go_type: "string"
  - db_type: "text"
    go_type: "string"
  - db_type: "timestamptz"
    go_type: "time.Time"
  - db_type: "jsonb"
    go_type:
      import: "encoding/json"
      type: "RawMessage"
  - db_type: "pg_catalog.int8"
    go_type: "int64"
```

### Column-specific override

```yaml
overrides:
  - column: "orders.metadata"
    go_type:
      import: "encoding/json"
      type: "RawMessage"
```

## Nullable Column Handling

### Option 1: Pointers (recommended)

```yaml
# sqlc.yaml
gen:
  go:
    emit_pointers_for_null_types: true
```

Generated:

```go
type Order struct {
    ID        string
    DeletedAt *time.Time // nullable → pointer
}
```

### Option 2: pgtype Types

```yaml
# sqlc.yaml (default behavior)
gen:
  go:
    emit_pointers_for_null_types: false
```

Generated:

```go
type Order struct {
    ID        string
    DeletedAt pgtype.Timestamptz // has Valid field
}
```

### Option 3: sql.Null* Types (for database/sql, not pgx)

Not recommended for pgx — use pgtype or pointers.

## Query File Organization

### Per-Feature (Recommended for Package-by-Feature)

```
internal/
├── order/
│   ├── query.sql      ← order queries only
│   ├── store.go
│   └── handler.go
├── user/
│   ├── query.sql      ← user queries only
│   ├── store.go
│   └── handler.go
```

```yaml
# sqlc.yaml
sql:
  - queries: "internal/order/query.sql"
    schema: "migrations/"
    gen:
      go:
        package: "order"
        out: "internal/order"
  - queries: "internal/user/query.sql"
    # ...
```

### Shared Queries (for cross-cutting concerns)

```
internal/
├── db/
│   ├── query.sql      ← shared queries (health check, etc.)
│   └── generated files
```

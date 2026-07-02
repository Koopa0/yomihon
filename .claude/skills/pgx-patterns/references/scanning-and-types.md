# Scanning and Custom Types (pgx/v5)

pgtype nullable types and scanning non-trivial column types: arrays, JSONB, composite types.

## Custom Types (pgtype)

```go
import "github.com/jackc/pgx/v5/pgtype"

// For nullable fields
type Order struct {
    ID          string          `db:"id"`
    Description pgtype.Text     `db:"description"` // nullable text
    Metadata    map[string]any  `db:"metadata"`     // nullable jsonb (or use json.RawMessage)
    DeletedAt   pgtype.Timestamptz `db:"deleted_at"` // nullable timestamptz
}
```

## Scanning Complex Types

### Arrays

```go
import "github.com/jackc/pgx/v5/pgtype"

type User struct {
    ID    string
    Tags  pgtype.Array[string] `db:"tags"`
    Roles []string             // Also works if emit_db_tags is set
}

// Query
var tags pgtype.Array[string]
err := pool.QueryRow(ctx, `SELECT tags FROM users WHERE id = $1`, id).Scan(&tags)

// Insert
_, err = pool.Exec(ctx, `INSERT INTO users (id, tags) VALUES ($1, $2)`,
    id, pgtype.Array[string]{Elements: []string{"admin", "user"}, Valid: true})
```

### JSONB

```go
// Using json.RawMessage (recommended)
type Order struct {
    ID       string
    Metadata json.RawMessage `db:"metadata"`
}

// Or using map
type Order struct {
    ID       string
    Metadata map[string]any `db:"metadata"`
}

// Insert JSONB
metadata := map[string]any{"source": "web", "version": 2}
_, err = pool.Exec(ctx, `INSERT INTO orders (id, metadata) VALUES ($1, $2)`, id, metadata)
```

### Composite Types

```go
// PostgreSQL: CREATE TYPE address AS (street TEXT, city TEXT, zip TEXT);

type Address struct {
    Street string
    City   string
    Zip    string
}

// Register the type
config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
    dt, err := conn.LoadType(ctx, "address")
    if err != nil {
        return err
    }
    conn.TypeMap().RegisterType(dt)
    return nil
}
```

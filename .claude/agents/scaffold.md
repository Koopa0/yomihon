---
name: scaffold
description: Scaffolds new feature packages in internal/ with types, handlers, store, queries, and tests. PREREQUISITE — planner must have produced an approved plan. Use after plan is approved, "scaffold".
model: sonnet
tools: Read, Grep, Glob, Write, Edit, Bash
maxTurns: 15
effort: medium
permissionMode: acceptEdits
skills:
  - pgx-patterns
  - sqlc-guide
  - http-server
  - error-patterns
  - migrations
---

# Feature Scaffolder

You scaffold new feature packages in `internal/` following the project's package-by-feature structure.

## Prerequisite

This agent runs AFTER the `planner` agent has produced an approved plan. The plan provides:
- Feature name and responsibility
- File list and type definitions
- API endpoints and database schema

If no plan exists, DO NOT proceed. Ask for it to be created first.

## Process

1. **Read the comprehension report** (Phase 0-1 output) to understand context and confirmed direction
2. **Read the approved plan** (Phase 2 output) for feature name, files, types, and endpoints
3. **Check existing features** in `internal/` to understand patterns
4. **Create the feature package** with all necessary files, matching the plan exactly
5. **Register routes** in `cmd/app/main.go` if HTTP handlers were created
6. **Verify** the code compiles: `go build ./...`

Do not deviate from the plan. If the plan is missing details, ask — do not guess.

## Files to Create

For a feature named `<feature>`:

### `internal/<feature>/<feature>.go`

Types, constants, and sentinel errors:

```go
package <feature>

import "errors"

// Sentinel errors
var (
    ErrNotFound = errors.New("not found")
    ErrConflict = errors.New("conflict")
)

// Core type
type <Feature> struct {
    ID        string    `json:"id"`
    // ... fields
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

### `internal/<feature>/handler.go`

HTTP handlers using closure pattern, with local helpers:

```go
package <feature>

import (
    "encoding/json"
    "fmt"
    "log/slog"
    "net/http"
)

// --- handler helpers (unexported, each feature owns its copy) ---

func decode[T any](w http.ResponseWriter, r *http.Request) (T, error) {
    var v T
    r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
    if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
        return v, fmt.Errorf("decoding request: %w", err)
    }
    return v, nil
}

func encode[T any](w http.ResponseWriter, status int, v T) {
    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("X-Content-Type-Options", "nosniff")
    w.WriteHeader(status)
    if err := json.NewEncoder(w).Encode(v); err != nil {
        slog.Error("encoding response", "error", err)
    }
}

type errorResponse struct {
    Error string `json:"error"`
}

func respondError(w http.ResponseWriter, status int, msg string) {
    encode(w, status, errorResponse{Error: msg})
}

// --- handlers ---

func list<Feature>s(store *Store, logger *slog.Logger) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // implementation
    }
}

func create<Feature>(store *Store, logger *slog.Logger) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // implementation
    }
}

func get<Feature>(store *Store, logger *slog.Logger) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        id := r.PathValue("id")
        // implementation
    }
}
```

### `internal/<feature>/store.go`

Database operations wrapping sqlc-generated queries:

```go
package <feature>

import (
    "context"
    "errors"
    "fmt"

    "github.com/jackc/pgx/v5"

    "github.com/koopa0/go-spec/internal/db"
)

type Store struct {
    q *db.Queries
}

func NewStore(dbtx db.DBTX) *Store {
    return &Store{q: db.New(dbtx)}
}

func (s *Store) WithTx(tx pgx.Tx) *Store {
    return &Store{q: db.New(tx)}
}

func (s *Store) <Feature>(ctx context.Context, id string) (*<Feature>, error) {
    row, err := s.q.<Feature>ByID(ctx, id)
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return nil, ErrNotFound
        }
        return nil, fmt.Errorf("querying <feature> %s: %w", id, err)
    }
    return <feature>FromRow(row), nil
}

// unexported type conversion
func <feature>FromRow(row db.<Feature>) *<Feature> {
    return &<Feature>{
        ID:        row.ID,
        CreatedAt: row.CreatedAt,
        UpdatedAt: row.UpdatedAt,
    }
}
```

### `internal/<feature>/query.sql`

sqlc query file (skeleton):

```sql
-- name: <Feature>ByID :one
SELECT id, created_at, updated_at FROM <feature>s WHERE id = $1;

-- name: <Feature>s :many
SELECT id, created_at, updated_at FROM <feature>s ORDER BY created_at DESC;

-- name: Create<Feature> :one
INSERT INTO <feature>s (id, created_at, updated_at)
VALUES ($1, now(), now())
RETURNING id, created_at, updated_at;
```

### `internal/<feature>/<feature>_test.go`

Test file with basic structure:

```go
package <feature>

import "testing"

func TestExample(t *testing.T) {
    t.Skip("TODO: implement tests")
}
```

## Rules

- Feature name is always singular, lowercase: `order`, not `orders` or `Order`
- All files go in `internal/<feature>/`
- Handler pattern: closure or struct based on complexity (see http-server.md)
- Handler helpers (`decode`, `encode`, `respondError`) are local unexported functions in each feature
- Store constructor accepts `db.DBTX`, NEVER `*pgxpool.Pool`
- Store provides `WithTx(tx pgx.Tx) *Store` for transaction support
- Use concrete `*Store` type, not an interface (interfaces are discovered when a consumer boundary or second impl appears — rules/interfaces.md)
- Do NOT create `integration_test.go` speculatively — when integration tests are needed they live in exactly one `integration_test.go` per feature (`//go:build integration`, package `<feature>_test`)
- Include sentinel errors for common cases (NotFound, Conflict)
- SQL uses `<feature>s` as table name (plural)
- `updated_at` set explicitly in UPDATE queries, not via triggers
- Run `sqlc generate` after creating query.sql
- Run `go build ./...` after scaffolding to verify compilation

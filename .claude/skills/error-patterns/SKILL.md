---
name: error-patterns
description: >-
  Go error handling patterns — sentinel errors vs custom error types, error
  wrapping strategy with %w, domain error to HTTP status mapping, an errors.Is
  vs errors.As decision tree, and cross-package error design.
when_to_use: >-
  Use when implementing error handling, defining domain or sentinel errors
  (ErrNotFound, ErrConflict), wrapping errors with fmt.Errorf and %w, mapping
  domain errors to HTTP status codes in handlers or middleware, choosing
  between errors.Is and errors.As, translating pgx errors in a store, or
  handling errors in Genkit flows.
metadata:
  author: koopa
  version: "1.0"
  lang: go
---

# Go Error Patterns

## Decision Tree: Sentinel vs Custom Type

```
Does the caller need to BRANCH on this error?
├─ No  → return fmt.Errorf("context: %w", err)
├─ Yes → Does the caller only need to IDENTIFY the error?
│   ├─ Yes → Sentinel: var ErrNotFound = errors.New("not found")
│   └─ No  → Does the caller need to EXTRACT data from the error?
│       └─ Yes → Custom type with errors.As
```

### Sentinel Errors (Most Common)

```go
// internal/order/order.go — define where the type lives
var (
    ErrNotFound     = errors.New("not found")
    ErrConflict     = errors.New("conflict")
    ErrForbidden    = errors.New("forbidden")
    ErrInvalidInput = errors.New("invalid input")
)
```

Only define sentinels the handler actually branches on. See: error-handling.md rule.

### Custom Error Types (Rare)

Use ONLY when the caller needs to extract structured data:

```go
// Validation with multiple field errors — caller needs field names
type ValidationError struct {
    Fields []FieldError
}

type FieldError struct {
    Field   string `json:"field"`
    Message string `json:"message"`
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation failed: %d field(s)", len(e.Fields))
}

// In handler — extract field details for API response (Go 1.26+)
if ve, ok := errors.AsType[*ValidationError](err); ok {
    encode(w, http.StatusUnprocessableEntity, map[string]any{
        "error":  "validation failed",
        "fields": ve.Fields,
    })
    return
}
```

### When NOT to Use Custom Types

- Single-condition check → sentinel
- Infrastructure errors (timeout, connection) → wrap with `%w`, never custom type
- Errors that cross package boundaries and you don't control → wrap, don't cast

## errors.Is vs errors.As

```
Do you need to CHECK IDENTITY? → errors.Is(err, ErrNotFound)
Do you need to EXTRACT DATA?   → errors.AsType[*PgError](err)  (Go 1.26+)
```

```go
// errors.Is — checks the chain for a matching value
if errors.Is(err, ErrNotFound) {
    respondError(w, http.StatusNotFound, "not found")
    return
}

// errors.AsType — checks the chain for a matching TYPE and extracts it (Go 1.26+)
if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
    switch pgErr.Code {
    case "23505":
        return ErrConflict
    }
}
```

**Never use `err == ErrNotFound`** — it doesn't traverse the wrap chain.

### errors.AsType (Go 1.26+)

`errors.AsType[T]()` is the generic, type-safe replacement for `errors.As`:

```go
// OLD — errors.As (Go 1.25 and earlier)
// Requires pre-declaring a pointer variable; easy to forget pointer-to-pointer
var pgErr *pgconn.PgError
if errors.As(err, &pgErr) {
    switch pgErr.Code {
    case "23505":
        return ErrConflict
    }
}

// NEW — errors.AsType (Go 1.26+)
// Type-safe, no variable declaration needed, faster
if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
    switch pgErr.Code {
    case "23505":
        return ErrConflict
    }
}
```

Always prefer `errors.AsType` on Go 1.26+. Use `errors.As` only when targeting older Go versions.

## Error Wrapping Strategy

### Format

```go
// PATTERN: "<doing what>: %w"
// lowercase, no punctuation, adds context the caller doesn't have
return fmt.Errorf("querying order %s: %w", id, err)
return fmt.Errorf("parsing request body: %w", err)
return fmt.Errorf("inserting order: %w", err)
```

### `%w` vs `%v`

```go
// %w — preserves error chain (internal code)
return fmt.Errorf("querying order: %w", err)

// %v — breaks chain (system boundaries — prevents leaking internal types)
return fmt.Errorf("database error: %v", err)
```

Use `%v` at API boundaries where you don't want callers depending on internal error types.

### Wrapping Depth

```
handler → store → pgx
  ↑         ↑      ↑
  log     wrap    originate
```

```go
// store.go — WRAP with context
func (s *Store) Order(ctx context.Context, id string) (*Order, error) {
    // ...
    if errors.Is(err, pgx.ErrNoRows) {
        return nil, ErrNotFound // translate, don't wrap
    }
    return nil, fmt.Errorf("querying order %s: %w", id, err)
}

// handler.go — HANDLE (log or respond, never both)
order, err := store.Order(ctx, id)
if errors.Is(err, order.ErrNotFound) {
    respondError(w, http.StatusNotFound, "order not found")
    return
}
if err != nil {
    logger.Error("getting order", "id", id, "error", err)
    respondError(w, http.StatusInternalServerError, "internal error")
    return
}
```

## Domain Error → HTTP Status Mapping

### Error Response Middleware

```go
// Central mapping — use in every handler or as middleware
func handleError(w http.ResponseWriter, logger *slog.Logger, err error) {
    switch {
    case errors.Is(err, ErrNotFound):
        respondError(w, http.StatusNotFound, "not found")
    case errors.Is(err, ErrConflict):
        respondError(w, http.StatusConflict, "already exists")
    case errors.Is(err, ErrForbidden):
        respondError(w, http.StatusForbidden, "forbidden")
    case errors.Is(err, ErrInvalidInput):
        respondError(w, http.StatusUnprocessableEntity, err.Error())
    default:
        // Unknown errors — log full detail, return generic message
        logger.Error("unhandled error", "error", err)
        respondError(w, http.StatusInternalServerError, "internal error")
    }
}

// With ValidationError extraction
func handleError(w http.ResponseWriter, logger *slog.Logger, err error) {
    ve, isValidation := errors.AsType[*ValidationError](err) // Go 1.26+
    switch {
    case isValidation:
        encode(w, http.StatusUnprocessableEntity, map[string]any{
            "error":  "validation failed",
            "fields": ve.Fields,
        })
    case errors.Is(err, ErrNotFound):
        respondError(w, http.StatusNotFound, "not found")
    case errors.Is(err, ErrConflict):
        respondError(w, http.StatusConflict, "already exists")
    default:
        logger.Error("unhandled error", "error", err)
        respondError(w, http.StatusInternalServerError, "internal error")
    }
}
```

### Usage in Handler

```go
func getOrder(store *order.Store, logger *slog.Logger) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        o, err := store.Order(r.Context(), r.PathValue("id"))
        if err != nil {
            handleError(w, logger, err)
            return
        }
        encode(w, http.StatusOK, o)
    }
}
```

### Mapping Table

| Domain Error     | HTTP Status | Response Message |
|------------------|-------------|------------------|
| `ErrNotFound`    | 404         | "not found"      |
| `ErrConflict`    | 409         | "already exists"  |
| `ErrForbidden`   | 403         | "forbidden"      |
| `ErrInvalidInput`| 422         | err.Error()      |
| `ValidationError`| 422         | structured fields |
| unknown          | 500         | "internal error" |

## Cross-Package Error Design

### Export vs Unexport

```go
// EXPORT: errors that consumers branch on
var ErrNotFound = errors.New("not found")     // handler checks this
var ErrConflict = errors.New("conflict")       // handler checks this

// DO NOT EXPORT: internal implementation errors
// If only used inside the package, keep unexported
var errPoolExhausted = errors.New("pool exhausted")
```

### Cross-Feature Error Checking

```go
// internal/notification/handler.go
// Consumer imports the producer's sentinel
import "github.com/koopa0/go-spec/internal/order"

o, err := orderReader.Order(ctx, id)
if errors.Is(err, order.ErrNotFound) {
    // handle missing order
}
```

### Never Import Another Feature's Store

Define consumer-side interface. See: interfaces.md rule.

## Store Layer: pgx Error Translation

```go
// internal/order/store.go
func (s *Store) Order(ctx context.Context, id string) (*Order, error) {
    row, err := s.q.Order(ctx, id)
    if errors.Is(err, pgx.ErrNoRows) {
        return nil, ErrNotFound
    }
    if err != nil {
        return nil, fmt.Errorf("querying order %s: %w", id, err)
    }
    return mapOrder(row), nil
}

func (s *Store) CreateOrder(ctx context.Context, p CreateParams) (*Order, error) {
    row, err := s.q.CreateOrder(ctx, mapCreateParams(p))
    if err != nil {
        if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == "23505" {
            return nil, ErrConflict
        }
        return nil, fmt.Errorf("inserting order: %w", err)
    }
    return mapOrder(row), nil
}
```

See: pgx-patterns skill for full pgconn.PgError code table.

## Genkit Flow Error Handling

```go
genkit.DefineFlow(g, "generateRecipe",
    func(ctx context.Context, input *GenerateInput) (*Recipe, error) {
        // Validation errors — return ErrInvalidInput
        if input.Ingredient == "" {
            return nil, fmt.Errorf("%w: ingredient is required", ErrInvalidInput)
        }

        // AI generation errors — wrap with context
        recipe, _, err := genkit.GenerateData[Recipe](ctx, g,
            ai.WithPrompt("..."),
        )
        if err != nil {
            return nil, fmt.Errorf("generating recipe for %q: %w", input.Ingredient, err)
        }

        // Check finish reason
        // See: genkit-go skill for FinishReason handling

        // Store errors — propagate domain errors
        if err := store.Create(ctx, recipe); err != nil {
            return nil, fmt.Errorf("saving recipe: %w", err)
        }

        return recipe, nil
    })
```

### Flow Error → HTTP Response

Flows exposed via `genkit.Handler()` return errors as 500 by default. For custom mapping, wrap:

```go
func flowHandler(flow *core.Flow[*Input, *Output], logger *slog.Logger) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var input Input
        if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
            respondError(w, http.StatusBadRequest, "invalid request body")
            return
        }

        result, err := flow.Run(r.Context(), &input)
        if err != nil {
            handleError(w, logger, err) // reuse domain error mapping
            return
        }
        encode(w, http.StatusOK, result)
    }
}
```

## Anti-Patterns

```go
// NEVER: log AND return
logger.Error("failed", "error", err)
return fmt.Errorf("operation: %w", err)  // error logged twice

// NEVER: string matching
if strings.Contains(err.Error(), "not found") { ... }

// NEVER: naked errors without context
return err  // loses call site context

// NEVER: sentinel for infrastructure errors
var ErrTimeout = errors.New("timeout")  // just wrap the real error

// NEVER: panic for recoverable errors
if err != nil { panic(err) }  // use return
```

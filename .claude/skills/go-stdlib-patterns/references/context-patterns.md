# context — Advanced Patterns

Deep material for context usage beyond cancellation.
The mandatory custom key type rule lives in SKILL.md.

## context.WithoutCancel (Go 1.21+)

```go
// Need to do cleanup work after the parent context is cancelled
func handler(w http.ResponseWriter, r *http.Request) {
    // Request context cancels when client disconnects
    ctx := r.Context()

    order, err := createOrder(ctx, params)
    if err != nil { ... }

    // Audit log must succeed even if client disconnected
    auditCtx := context.WithoutCancel(ctx) // inherits values, ignores cancel
    go func() {
        logAudit(auditCtx, "order.created", order.ID)
    }()
}
```

**When to use**: fire-and-forget operations that need context values (tracing, user ID)
but must not be cancelled when the parent is cancelled (audit logs, cleanup).

## context.AfterFunc (Go 1.21+)

```go
// Run cleanup when context is cancelled
stop := context.AfterFunc(ctx, func() {
    slog.Info("context cancelled, releasing resources")
    resource.Release()
})
// If we finish normally, prevent the cleanup func from running
defer stop()
```

## context Anti-Patterns

```go
// ❌ WRONG — storing logger in context
ctx = context.WithValue(ctx, "logger", logger)

// ✅ CORRECT — pass logger as function parameter or struct field

// ❌ WRONG — storing database connection in context
ctx = context.WithValue(ctx, "db", pool)

// ✅ CORRECT — pass as function parameter

// ❌ WRONG — reading auth user in store layer from context
func (s *Store) CreateOrder(ctx context.Context, p Params) error {
    userID := UserIDFrom(ctx) // store shouldn't know about auth
}

// ✅ CORRECT — handler extracts, passes as parameter
func (s *Store) CreateOrder(ctx context.Context, userID string, p Params) error {
}
```

**Context values are for cross-cutting concerns only**: request ID, trace span,
authenticated user identity. Business data flows through function parameters.

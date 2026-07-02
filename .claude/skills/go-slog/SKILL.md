---
name: go-slog
description: >-
  Go log/slog patterns — handler setup (JSON vs Text vs test handlers),
  logger injection via middleware, structured key naming, log group design,
  custom slog.Value types, and OpenTelemetry trace correlation. Complements
  go-philosophy.md § Observability (MUST/NEVER rules) with deep how-to
  patterns.
when_to_use: >-
  Use when setting up logging, injecting loggers into HTTP handlers,
  choosing structured log keys, designing log groups, or correlating logs
  with OTel traces. Trigger phrases: "slog", "structured logging", "logging
  setup", "logger middleware", "log keys", "trace id in logs".
metadata:
  author: koopa
  version: "1.0"
  lang: go
---

# Go slog Patterns

## Handler Setup

### Decision Tree: Which Handler?

```
What environment is this for?
├─ Production → slog.NewJSONHandler (machine-readable)
├─ Development → slog.NewTextHandler (human-readable)
└─ Testing → Do you need to assert log output?
    ├─ Yes → custom handler writing to *bytes.Buffer
    └─ No  → slog.NewTextHandler(io.Discard, nil)
```

### Production Setup

```go
func setupLogger(level string) *slog.Logger {
    var lvl slog.Level
    switch level {
    case "debug":
        lvl = slog.LevelDebug
    case "warn":
        lvl = slog.LevelWarn
    case "error":
        lvl = slog.LevelError
    default:
        lvl = slog.LevelInfo
    }

    handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: lvl,
    })
    return slog.New(handler)
}
```

### Multi-Handler Setup (Go 1.26+)

```go
// slog.NewMultiHandler (Go 1.26+) — fan-out to multiple handlers
handler := slog.NewMultiHandler(
    slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}),
    slog.NewJSONHandler(errorFile, &slog.HandlerOptions{Level: slog.LevelError}),
)
logger := slog.New(handler)
```

`Enabled()` returns true if ANY sub-handler is enabled at the given level.
This replaces custom multi-handler implementations.

### Setting Default Logger

```go
func main() {
    logger := setupLogger(cfg.LogLevel)
    slog.SetDefault(logger) // sets package-level slog functions

    // Pass logger explicitly to handlers — don't rely on default
}
```

**Rule**: set default for stdlib/third-party code that uses `slog.Info()` etc.,
but always pass `*slog.Logger` explicitly to your own code.

## Logger Injection

### Middleware Pattern (Preferred)

```go
// Middleware adds request-scoped fields to logger
func withLogger(logger *slog.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            reqID := r.Context().Value(requestIDKey{}).(string)
            log := logger.With(
                "request_id", reqID,
                "method", r.Method,
                "path", r.URL.Path,
            )
            ctx := context.WithValue(r.Context(), loggerKey{}, log)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

type loggerKey struct{}

func loggerFrom(ctx context.Context) *slog.Logger {
    if l, ok := ctx.Value(loggerKey{}).(*slog.Logger); ok {
        return l
    }
    return slog.Default() // fallback, never nil
}
```

### Handler Closure Pattern (Alternative)

```go
// Logger passed directly to handler closure — no context lookup
func createOrder(store OrderWriter, logger *slog.Logger) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        reqID := r.Context().Value(requestIDKey{}).(string)
        log := logger.With("request_id", reqID)

        log.Info("creating order")
        // ...
        if err != nil {
            log.Error("creating order", "error", err)
            http.Error(w, "internal error", http.StatusInternalServerError)
            return
        }
        log.Info("order created", "order_id", order.ID)
    }
}
```

**Decision**: use middleware pattern when many handlers need the same logger setup.
Use closure pattern for simple cases or when handlers need different logger configs.

## Key Naming

### Conventions

| Rule | Example | Why |
|------|---------|-----|
| `snake_case` always | `request_id`, `user_id` | Consistent with JSON field names |
| Short, specific | `order_id` not `the_order_identifier` | Grep-friendly |
| No redundant prefix | `"error", err` not `"error_message", err.Error()` | slog formats error automatically |
| Domain identifiers first | `"order_id", id, "status", s` | Most useful fields for filtering |

### Standard Key Names (Project-Wide)

| Key | Type | Where Used |
|-----|------|-----------|
| `request_id` | string | All request-scoped logs |
| `method` | string | HTTP method |
| `path` | string | URL path |
| `status` | int | HTTP response status |
| `duration_ms` | float64 | Request duration |
| `error` | error | Error value |
| `order_id` | string | Order feature |
| `user_id` | string | User feature |

### What to Log at Each Level

```go
// Debug — development-only details
slog.Debug("query executed", "sql", query, "duration_ms", d.Milliseconds())

// Info — business events, state changes
slog.Info("order created", "order_id", id, "user_id", uid, "total", total)

// Warn — recoverable issues, degraded state
slog.Warn("cache miss, falling back to db", "key", key)

// Error — unrecoverable, needs investigation
slog.Error("creating order", "error", err, "user_id", uid)
```

## Log Groups

```go
// Group related attributes
logger.Info("request completed",
    slog.Group("request",
        slog.String("method", r.Method),
        slog.String("path", r.URL.Path),
    ),
    slog.Group("response",
        slog.Int("status", status),
        slog.Duration("duration", duration),
    ),
)
// JSON output: {"request":{"method":"GET","path":"/orders"},"response":{"status":200,"duration":"12ms"}}
```

### GroupAttrs Helper (Go 1.25+)

```go
// slog.GroupAttrs (Go 1.25+) — create group from slice
attrs := []slog.Attr{
    slog.String("method", r.Method),
    slog.String("path", r.URL.Path),
}
logger.LogAttrs(ctx, slog.LevelInfo, "request", slog.GroupAttrs("http", attrs...)...)
```

Useful when attributes are built dynamically or conditionally before logging.

**Rule**: use groups to namespace when you have 5+ attributes or when attributes
from different domains (request, response, database) appear in the same log line.

## Custom slog.LogValuer

Implement `slog.LogValuer` to control how types appear in logs:

```go
// Redact sensitive fields automatically
type User struct {
    ID       string
    Email    string
    Password string // must never appear in logs
}

func (u User) LogValue() slog.Value {
    return slog.GroupValue(
        slog.String("id", u.ID),
        slog.String("email", u.Email),
        // Password intentionally omitted
    )
}

// Usage — safe to log directly
slog.Info("user authenticated", "user", user)
// Output: {"user":{"id":"u1","email":"a@b.c"}} — no password
```

**When to implement LogValuer**:
- Types with sensitive fields (users, credentials, config)
- Types with large fields you want to summarize (structs with []byte)
- Domain types where default formatting is unclear

## OpenTelemetry Correlation

### Adding Trace Context to Logs

```go
import "go.opentelemetry.io/otel/trace"

// Custom handler that adds trace/span IDs to every log line
type traceHandler struct {
    inner slog.Handler
}

func (h *traceHandler) Handle(ctx context.Context, r slog.Record) error {
    span := trace.SpanFromContext(ctx)
    if span.SpanContext().IsValid() {
        r.AddAttrs(
            slog.String("trace_id", span.SpanContext().TraceID().String()),
            slog.String("span_id", span.SpanContext().SpanID().String()),
        )
    }
    return h.inner.Handle(ctx, r)
}

func (h *traceHandler) Enabled(ctx context.Context, level slog.Level) bool {
    return h.inner.Enabled(ctx, level)
}

func (h *traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
    return &traceHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *traceHandler) WithGroup(name string) slog.Handler {
    return &traceHandler{inner: h.inner.WithGroup(name)}
}
```

**Rule**: only add trace correlation when OTel is already set up (Phase 2+).
Don't add trace IDs to logs if you're not running a trace collector.

### Record.Source() (Go 1.25+)

`slog.Record.Source()` returns the source location (`*slog.Source`) directly,
replacing manual `runtime.Caller` extraction in custom handlers. Use this
instead of parsing `AddSource` output when you need file/line in handler logic.

## Anti-Patterns

### Log AND Return Error

```go
// ❌ WRONG — error logged twice (here + caller)
func (s *Store) Order(ctx context.Context, id string) (*Order, error) {
    order, err := s.q.GetOrder(ctx, id)
    if err != nil {
        slog.Error("getting order", "error", err) // logged here
        return nil, fmt.Errorf("getting order: %w", err) // AND returned
    }
    return order, nil
}

// ✅ CORRECT — return only, let handler decide
func (s *Store) Order(ctx context.Context, id string) (*Order, error) {
    order, err := s.q.GetOrder(ctx, id)
    if err != nil {
        return nil, fmt.Errorf("getting order %s: %w", id, err)
    }
    return order, nil
}
```

### String Formatting in Log Messages

```go
// ❌ WRONG — fmt.Sprintf defeats structured logging
slog.Info(fmt.Sprintf("order %s created by %s", id, userID))

// ✅ CORRECT — structured key-value pairs
slog.Info("order created", "order_id", id, "user_id", userID)
```

### Logging Request Bodies

```go
// ❌ WRONG — may contain passwords, tokens, PII
slog.Debug("request received", "body", string(body))

// ✅ CORRECT — log only safe, specific fields
slog.Debug("request received", "content_type", r.Header.Get("Content-Type"),
    "content_length", r.ContentLength)
```

### Inconsistent Key Names

```go
// ❌ WRONG — same concept, different keys across codebase
slog.Info("...", "req_id", id)     // in middleware
slog.Info("...", "requestId", id)  // in handler
slog.Info("...", "request_id", id) // in store

// ✅ CORRECT — one canonical name used everywhere
slog.Info("...", "request_id", id)
```

### Using slog.Default() Everywhere

```go
// ❌ WRONG — no request context, can't correlate logs
func processOrder(order Order) {
    slog.Info("processing", "order_id", order.ID) // which request?
}

// ✅ CORRECT — pass logger with request context
func processOrder(log *slog.Logger, order Order) {
    log.Info("processing", "order_id", order.ID) // has request_id from middleware
}
```

See: go-philosophy.md § Observability rule for MUST/NEVER constraints on logging.
See: otel-guide skill for full OpenTelemetry setup including trace handler wiring.
See: go-stdlib-patterns skill for context key design patterns.

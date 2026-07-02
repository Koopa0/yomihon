---
name: go-middleware
description: >-
  Go HTTP middleware patterns — the func(http.Handler) http.Handler
  signature, ordering with WHY explanations, common middleware catalog,
  chain composition, and anti-patterns. Complements the http-server
  skill (middleware implementations) with ordering rationale and
  composition patterns.
when_to_use: >-
  Use when adding HTTP middleware, deciding middleware order, composing
  middleware chains, or when CORS/auth ordering is unclear. Triggers:
  "middleware", "middleware order", "CORS before auth", "wrap handler",
  "logging/recovery/auth chain".
metadata:
  author: koopa
  version: "1.0"
  lang: go
---

# Go HTTP Middleware Patterns

## Middleware Signature

```go
// The standard middleware signature in Go
type Middleware func(http.Handler) http.Handler
```

Every middleware:
1. Accepts an `http.Handler` (the next handler in the chain)
2. Returns an `http.Handler` (wrapping the next handler)
3. Can run code before and/or after calling `next.ServeHTTP`

```go
func example(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // BEFORE: runs before the handler
        start := time.Now()

        next.ServeHTTP(w, r) // call next handler

        // AFTER: runs after the handler
        slog.Info("request", "duration", time.Since(start))
    })
}
```

## Middleware Ordering

### The Correct Order (and WHY)

```
Request → Recovery → RequestID → Logger → CORS → RateLimit → Auth → Handler
                                                                        │
Response ← Recovery ← RequestID ← Logger ← CORS ← RateLimit ← Auth ←──┘
```

| Position | Middleware | WHY This Position |
|----------|-----------|-------------------|
| 1 | **Recovery** | Must catch panics from ALL subsequent middleware and handlers. If placed later, a panic in an earlier middleware crashes the server. |
| 2 | **RequestID** | Must be set before Logger so every log line has the ID. Must be before Auth so auth failure logs include the request ID. |
| 3 | **Logger** | Needs RequestID (set in step 2). Must be before CORS/Auth so it logs ALL requests, including rejected ones. Logs duration by wrapping response. |
| 4 | **CORS** | Must be BEFORE Auth. Browsers send preflight OPTIONS requests without auth headers. If Auth runs first, preflight is rejected with 401, and the browser blocks the real request. |
| 5 | **RateLimit** | Before Auth to prevent brute-force attacks. Rate limiting unauthenticated requests is cheaper than authenticating first. |
| 6 | **Auth** | Last "infrastructure" middleware. Only business handlers run after this. Placed after CORS so preflight passes. Placed after RateLimit so attackers are throttled before hitting auth logic. |

### The CORS-Before-Auth Rule (Most Common Mistake)

```go
// ❌ WRONG — browser preflight blocked by auth
mux := http.NewServeMux()
handler := auth(cors(mux))      // auth runs BEFORE cors
// Browser: OPTIONS /api/orders → 401 Unauthorized (no token on preflight)
// Result: CORS error in browser console, nothing works

// ✅ CORRECT — CORS handles preflight before auth checks
handler := cors(auth(mux))      // cors runs BEFORE auth
// Browser: OPTIONS /api/orders → 200 (CORS headers returned)
// Browser: GET /api/orders + token → auth checks token → 200
```

### Per-Route vs Global Middleware

```
Is this middleware needed for EVERY route?
├─ Yes → Global (wrap the mux)
│   Examples: Recovery, RequestID, Logger, CORS
├─ No  → Is it needed for a GROUP of routes?
│   ├─ Yes → Route group (wrap a sub-mux or use a chain)
│   │   Examples: Auth (only protected routes), Admin (only admin routes)
│   └─ No  → Per-handler (wrap individual handler)
│       Examples: RateLimit on login only, Cache on specific GET
```

```go
// Global middleware — wraps entire mux
mux := http.NewServeMux()
mux.HandleFunc("GET /health", healthCheck())
mux.HandleFunc("GET /orders/{id}", getOrder(store))
mux.HandleFunc("POST /orders", createOrder(store))

handler := chain(recovery, requestID, logger, cors)(mux)

// Per-route auth — only on protected endpoints
mux.HandleFunc("GET /orders/{id}", authMiddleware(getOrder(store)))
mux.HandleFunc("POST /orders", authMiddleware(createOrder(store)))
mux.HandleFunc("GET /health", healthCheck()) // no auth
```

## Chain Composition

### Simple Chain Function

```go
// chain applies middleware in order: first middleware is outermost
func chain(middlewares ...Middleware) Middleware {
    return func(handler http.Handler) http.Handler {
        for i := len(middlewares) - 1; i >= 0; i-- {
            handler = middlewares[i](handler)
        }
        return handler
    }
}

// Usage — reads left-to-right, executes left-to-right
handler := chain(
    recovery,    // 1st: outermost, catches panics from everything
    requestID,   // 2nd: sets request ID
    withLogger(logger), // 3rd: logs with request ID
    corsHandler, // 4th: handles preflight
    rateLimiter, // 5th: throttles before auth
)(mux)
```

### Why Reverse Loop?

The chain function loops in reverse because middleware wraps inside-out:
`recovery(requestID(logger(mux)))`. The last applied middleware is the
outermost. Reversing the loop makes the API read in execution order.

## Common Middleware Catalog

| Middleware | Responsibility | Scope |
|-----------|---------------|-------|
| Recovery | Catch panics, return 500, log stack trace | Global |
| RequestID | Set/propagate X-Request-ID header + context | Global |
| Logger | Log request method, path, status, duration | Global |
| CORS | Set Access-Control headers, handle preflight | Global |
| RateLimit | Throttle by IP or token | Global or per-route |
| Auth | Validate JWT/session, set user in context | Route group |
| Timeout | `http.TimeoutHandler` wrapper | Per-route |
| Compress | gzip response (use stdlib `compress/gzip`) | Global |

**Note**: for implementation code of each middleware, see the http-server skill.

## Response Capture

To log response status or measure response time, you need a response writer
wrapper that captures the status code:

```go
type responseWriter struct {
    http.ResponseWriter
    status int
    wrote  bool
}

func (rw *responseWriter) WriteHeader(code int) {
    if !rw.wrote {
        rw.status = code
        rw.wrote = true
    }
    rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
    if !rw.wrote {
        rw.status = http.StatusOK
        rw.wrote = true
    }
    return rw.ResponseWriter.Write(b)
}

// Usage in logger middleware
func withRequestLog(logger *slog.Logger) Middleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
            start := time.Now()
            next.ServeHTTP(rw, r)
            logger.Info("request",
                "method", r.Method,
                "path", r.URL.Path,
                "status", rw.status,
                "duration_ms", time.Since(start).Milliseconds(),
            )
        })
    }
}
```

## Anti-Patterns

### Middleware That Modifies Request Body

```go
// ❌ WRONG — reading body in middleware prevents handler from reading it
func logBody(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        body, _ := io.ReadAll(r.Body) // body consumed!
        slog.Debug("body", "content", string(body))
        next.ServeHTTP(w, r) // handler gets empty body
    })
}
```

### Middleware With Business Logic

```go
// ❌ WRONG — business validation in middleware
func validateOrder(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        var order Order
        json.NewDecoder(r.Body).Decode(&order)
        if order.Total <= 0 {
            http.Error(w, "invalid total", http.StatusBadRequest)
            return
        }
        next.ServeHTTP(w, r)
    })
}

// ✅ CORRECT — validation belongs in the handler
```

### Too Many Middleware

```go
// ❌ WRONG — 10+ middleware layers, impossible to debug
handler := chain(
    recovery, requestID, logger, cors, rateLimit,
    auth, rbac, audit, metrics, tracing, compress,
)(mux)

// ✅ CORRECT — keep to 5-7 max
// Combine related concerns (e.g., otelhttp handles tracing+metrics)
```

### Middleware That Calls next.ServeHTTP Twice

```go
// ❌ WRONG — handler runs twice
func bad(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        next.ServeHTTP(w, r) // first call
        if someCondition {
            next.ServeHTTP(w, r) // second call — double execution!
        }
    })
}
```

See: http-server skill for middleware implementation code (recovery, CORS, auth, logging).
See: auth-patterns skill for JWT validation middleware details.
See: go-slog skill for logger injection middleware patterns.

---
name: http-server
description: >-
  net/http server patterns for Go 1.22+ std lib routing, updated through Go
  1.26 — method+path ServeMux routing, path parameters and wildcards,
  closure-based dependency injection, JSON and error response helpers,
  middleware (logging, recovery, auth, CORS, request ID, compression,
  per-handler timeout, CrossOriginProtection), health/readiness endpoints,
  file upload, streaming responses, and reverse proxy.
when_to_use: >-
  Use when building or modifying HTTP servers or handlers with net/http,
  registering routes on ServeMux, extracting path parameters, writing
  handler.go files, adding or ordering middleware, returning JSON or error
  responses, validating requests, handling query parameters, file uploads,
  streaming, CORS, bearer token auth, or reverse proxies — any std-lib HTTP
  work without frameworks (chi, gin, echo).
user_invocable: true
metadata:
  author: koopa
  version: "1.0"
  lang: go
---

# net/http Server Patterns Reference

Core routing, dependency injection, response helpers, and pitfalls live here. Deep implementations (middleware catalog, server lifecycle, request handling) live in `references/` — see Navigation at the bottom.

## Common Pitfalls

### Response Already Sent

```go
// WRONG — writes header twice
func handler(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    if someCondition {
        w.WriteHeader(http.StatusBadRequest) // silently ignored!
        return
    }
}

// CORRECT — return after each response
func handler(w http.ResponseWriter, r *http.Request) {
    if someCondition {
        respondError(w, http.StatusBadRequest, "error")
        return
    }
    encode(w, http.StatusOK, data)
}
```

### Context Deadline in Handlers

```go
// Always respect context cancellation
func handler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    result, err := expensiveOperation(ctx) // pass ctx
    if err != nil {
        if ctx.Err() != nil {
            // Client disconnected, don't bother responding
            return
        }
        respondError(w, http.StatusInternalServerError, "error")
        return
    }
    encode(w, http.StatusOK, result)
}
```

### Middleware Order

```
Outermost → Innermost:
  Recovery → RequestID → CORS → Logging → Auth → Handler

Recovery catches panics from everything
RequestID available for all subsequent middleware
CORS handles preflight before auth
Logging captures all request/response info
Auth applied only where needed
```

## Routing (Go 1.22+)

### Method + Path Routing

```go
mux := http.NewServeMux()

// Method routing
mux.HandleFunc("GET /orders", listOrders(store))
mux.HandleFunc("POST /orders", createOrder(store))
mux.HandleFunc("GET /orders/{id}", getOrder(store))
mux.HandleFunc("PUT /orders/{id}", updateOrder(store))
mux.HandleFunc("DELETE /orders/{id}", deleteOrder(store))

// Exact match (no trailing slash wildcard)
mux.HandleFunc("GET /health", healthCheck())

// Subtree match (trailing slash)
mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

// NOTE (Go 1.26): ServeMux trailing slash redirects now use 307 (was 301).
// This preserves the HTTP method (POST stays POST after redirect).
```

### Path Parameters

```go
func getOrder(store *order.Store) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        id := r.PathValue("id")
        if id == "" {
            http.Error(w, "missing order id", http.StatusBadRequest)
            return
        }
        // use id
    }
}
```

### Wildcard Catch-All

```go
// {path...} captures the rest of the path
mux.HandleFunc("GET /files/{path...}", serveFile(root))

func serveFile(root string) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        path := r.PathValue("path")
        // ...
    }
}
```

## Closure-Based Dependency Injection

Every handler is a function that returns `http.HandlerFunc`:

```go
func listOrders(store *order.Store, logger *slog.Logger) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        ctx := r.Context()

        orders, err := store.List(ctx)
        if err != nil {
            logger.Error("listing orders", "error", err)
            http.Error(w, "internal error", http.StatusInternalServerError)
            return
        }

        encode(w, http.StatusOK, orders)
    }
}
```

## JSON Helpers

```go
func encode[T any](w http.ResponseWriter, status int, v T) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    if err := json.NewEncoder(w).Encode(v); err != nil {
        slog.Error("encoding response", "error", err)
    }
}

func decode[T any](r *http.Request) (T, error) {
    var v T
    dec := json.NewDecoder(r.Body)
    dec.DisallowUnknownFields()
    if err := dec.Decode(&v); err != nil {
        return v, fmt.Errorf("decoding request body: %w", err)
    }
    return v, nil
}
```

## Error Response Helper

```go
type errorResponse struct {
    Error string `json:"error"`
}

func respondError(w http.ResponseWriter, status int, msg string) {
    encode(w, status, errorResponse{Error: msg})
}
```

## Middleware

### Signature

```go
type middleware func(http.Handler) http.Handler
```

### Chaining Middleware

```go
func chain(handler http.Handler, middlewares ...middleware) http.Handler {
    for i := len(middlewares) - 1; i >= 0; i-- {
        handler = middlewares[i](handler)
    }
    return handler
}

// Usage
handler := chain(mux,
    recoverPanic(logger),
    logRequest(logger),
    requestID(),
)
```

Implementations of `recoverPanic`, `logRequest`, `requestID`, and every other middleware live in `references/middleware.md`.

## Navigation

| Topic | File | When to read |
|-------|------|--------------|
| Logging, recovery, auth (bearer token, route-level), CORS, CrossOriginProtection, request ID, per-handler timeout, compression, response writer wrapper | `references/middleware.md` | For any middleware implementation, read references/middleware.md |
| Graceful shutdown, health and readiness endpoints | `references/server-lifecycle.md` | For starting, health-checking, or shutting down the server, read references/server-lifecycle.md |
| Query parameters, request validation, file upload, streaming responses, reverse proxy | `references/request-handling.md` | For parsing/validating requests or serving uploads, streams, and proxies, read references/request-handling.md |

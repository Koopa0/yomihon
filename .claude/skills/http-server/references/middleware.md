# Middleware Implementations

Implementations for the `func(http.Handler) http.Handler` middleware signature defined in SKILL.md. For ordering rules, see "Middleware Order" under Common Pitfalls in SKILL.md.

## Logging Middleware

```go
func logRequest(logger *slog.Logger) middleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()
            sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}

            next.ServeHTTP(sw, r)

            logger.Info("request",
                "method", r.Method,
                "path", r.URL.Path,
                "status", sw.status,
                "duration_ms", time.Since(start).Milliseconds(),
            )
        })
    }
}

type statusWriter struct {
    http.ResponseWriter
    status int
}

func (w *statusWriter) WriteHeader(status int) {
    w.status = status
    w.ResponseWriter.WriteHeader(status)
}
```

## Recovery Middleware

```go
func recoverPanic(logger *slog.Logger) middleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            defer func() {
                if rec := recover(); rec != nil {
                    logger.Error("panic recovered",
                        "panic", rec,
                        "method", r.Method,
                        "path", r.URL.Path,
                    )
                    http.Error(w, "internal server error", http.StatusInternalServerError)
                }
            }()
            next.ServeHTTP(w, r)
        })
    }
}
```

## Authentication Middleware

### Bearer Token Auth

```go
type ctxKey string
const userIDKey ctxKey = "user_id"

func authMiddleware(validateToken func(string) (string, error)) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            auth := r.Header.Get("Authorization")
            if auth == "" {
                respondError(w, http.StatusUnauthorized, "missing authorization header")
                return
            }

            if !strings.HasPrefix(auth, "Bearer ") {
                respondError(w, http.StatusUnauthorized, "invalid authorization format")
                return
            }

            token := strings.TrimPrefix(auth, "Bearer ")
            userID, err := validateToken(token)
            if err != nil {
                respondError(w, http.StatusUnauthorized, "invalid token")
                return
            }

            ctx := context.WithValue(r.Context(), userIDKey, userID)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

// In handlers:
func protectedHandler() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        userID := r.Context().Value(userIDKey).(string)
        // use userID
    }
}
```

### Route-Level Auth

```go
func setupRoutes(mux *http.ServeMux, auth func(http.Handler) http.Handler) {
    // Public routes
    mux.HandleFunc("GET /health", healthCheck())
    mux.HandleFunc("POST /login", login())

    // Protected routes - wrap individual handlers
    mux.Handle("GET /orders", auth(http.HandlerFunc(listOrders(store))))
    mux.Handle("POST /orders", auth(http.HandlerFunc(createOrder(store))))
}
```

## CORS Middleware

```go
func cors(allowedOrigins []string) func(http.Handler) http.Handler {
    allowed := make(map[string]bool)
    for _, o := range allowedOrigins {
        allowed[o] = true
    }

    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            origin := r.Header.Get("Origin")
            if allowed[origin] {
                w.Header().Set("Access-Control-Allow-Origin", origin)
                w.Header().Set("Access-Control-Allow-Credentials", "true")
            }
            w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
            w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
            w.Header().Set("Access-Control-Max-Age", "86400")

            if r.Method == http.MethodOptions {
                w.WriteHeader(http.StatusNoContent)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

## CrossOriginProtection (Go 1.25+)

```go
// CrossOriginProtection — CSRF protection via Fetch metadata headers.
// Replaces custom CSRF token middleware for modern browsers.
mux := http.NewServeMux()
mux.HandleFunc("POST /api/orders", createOrder)

// Wrap with CrossOriginProtection for state-changing endpoints
csrf := http.NewCrossOriginProtection()
handler := csrf.Handler(mux)
```

## Request ID Middleware

```go
type requestIDKey struct{}

func requestID() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            id := r.Header.Get("X-Request-ID")
            if id == "" {
                id = generateID() // crypto/rand based
            }
            ctx := context.WithValue(r.Context(), requestIDKey{}, id)
            w.Header().Set("X-Request-ID", id)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

func requestIDFrom(ctx context.Context) string {
    id, _ := ctx.Value(requestIDKey{}).(string)
    return id
}

func generateID() string {
    b := make([]byte, 16)
    rand.Read(b)
    return hex.EncodeToString(b)
}
```

## Per-Handler Timeout

```go
func withTimeout(timeout time.Duration, h http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        ctx, cancel := context.WithTimeout(r.Context(), timeout)
        defer cancel()
        h(w, r.WithContext(ctx))
    }
}

// Usage
mux.HandleFunc("GET /slow", withTimeout(5*time.Second, slowHandler()))
```

## Compression Middleware

```go
import "compress/gzip"

func gzipMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
            next.ServeHTTP(w, r)
            return
        }

        w.Header().Set("Content-Encoding", "gzip")
        gz := gzip.NewWriter(w)
        defer gz.Close()

        next.ServeHTTP(&gzipWriter{ResponseWriter: w, Writer: gz}, r)
    })
}

type gzipWriter struct {
    http.ResponseWriter
    io.Writer
}

func (w *gzipWriter) Write(b []byte) (int, error) {
    return w.Writer.Write(b)
}
```

## Response Writer Wrapper (for status capture)

```go
type responseWriter struct {
    http.ResponseWriter
    status      int
    wroteHeader bool
}

func (w *responseWriter) WriteHeader(status int) {
    if w.wroteHeader {
        return
    }
    w.status = status
    w.wroteHeader = true
    w.ResponseWriter.WriteHeader(status)
}

func (w *responseWriter) Write(b []byte) (int, error) {
    if !w.wroteHeader {
        w.WriteHeader(http.StatusOK)
    }
    return w.ResponseWriter.Write(b)
}

// Use in logging middleware to capture status
func logRequest(logger *slog.Logger) func(http.Handler) http.Handler {
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

---
name: config-management
description: >-
  Go configuration management patterns — environment variables with the
  standard library, type-safe config structs, nested config design, fail-fast
  validation at startup, godotenv for development, and sensitive value
  redaction.
when_to_use: >-
  Use when setting up application config, adding or reading environment
  variables, designing a config struct in cmd/app/main.go, writing
  loadConfig/getEnv/getDuration helpers, validating config at startup, using
  godotenv or .env files in development, or redacting secrets and API keys
  from logs.
metadata:
  author: koopa
  version: "1.0"
  lang: go
---

# Config Management Patterns

## Config Struct Design

```go
// cmd/app/main.go — config lives here, nowhere else

type config struct {
    Server   serverConfig
    Database databaseConfig
    AI       aiConfig
    CORS     corsConfig
}

type serverConfig struct {
    Port         string
    ReadTimeout  time.Duration
    WriteTimeout time.Duration
}

type databaseConfig struct {
    URL             string
    MaxConns        int32
    MinConns        int32
    MaxConnLifetime time.Duration
}

type aiConfig struct {
    DefaultModel string
    APIKey       string // sensitive — never log
}

type corsConfig struct {
    AllowedOrigins []string
}
```

## Loading: Pure Std Lib

```go
func loadConfig() config {
    return config{
        Server: serverConfig{
            Port:         getEnv("PORT", "8080"),
            ReadTimeout:  getDuration("READ_TIMEOUT", 5*time.Second),
            WriteTimeout: getDuration("WRITE_TIMEOUT", 10*time.Second),
        },
        Database: databaseConfig{
            URL:             requireEnv("DATABASE_URL"),
            MaxConns:        int32(getInt("DB_MAX_CONNS", 25)),
            MinConns:        int32(getInt("DB_MIN_CONNS", 5)),
            MaxConnLifetime: getDuration("DB_MAX_CONN_LIFETIME", time.Hour),
        },
        AI: aiConfig{
            DefaultModel: getEnv("DEFAULT_MODEL", "googleai/gemini-2.5-flash"),
            APIKey:       requireEnv("GEMINI_API_KEY"),
        },
        CORS: corsConfig{
            AllowedOrigins: getEnvSlice("CORS_ORIGINS", []string{"http://localhost:3000"}),
        },
    }
}
```

## Helper Functions

### Required vs Optional vs Default

```go
// Required — fail fast if missing
func requireEnv(key string) string {
    v := os.Getenv(key)
    if v == "" {
        log.Fatalf("required environment variable %s is not set", key)
    }
    return v
}

// Optional with default
func getEnv(key, fallback string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return fallback
}

// Typed helpers
func getInt(key string, fallback int) int {
    v := os.Getenv(key)
    if v == "" {
        return fallback
    }
    n, err := strconv.Atoi(v)
    if err != nil {
        log.Fatalf("environment variable %s must be an integer: %s", key, v)
    }
    return n
}

func getBool(key string, fallback bool) bool {
    v := os.Getenv(key)
    if v == "" {
        return fallback
    }
    b, err := strconv.ParseBool(v)
    if err != nil {
        log.Fatalf("environment variable %s must be a boolean: %s", key, v)
    }
    return b
}

func getDuration(key string, fallback time.Duration) time.Duration {
    v := os.Getenv(key)
    if v == "" {
        return fallback
    }
    d, err := time.ParseDuration(v)
    if err != nil {
        log.Fatalf("environment variable %s must be a duration: %s", key, v)
    }
    return d
}

func getEnvSlice(key string, fallback []string) []string {
    v := os.Getenv(key)
    if v == "" {
        return fallback
    }
    return strings.Split(v, ",")
}
```

## Validation: Fail Fast at Startup

```go
func (c config) validate() {
    if c.Server.Port == "" {
        log.Fatal("PORT must not be empty")
    }
    if c.Database.URL == "" {
        log.Fatal("DATABASE_URL must not be empty")
    }
    if c.Database.MaxConns < 1 {
        log.Fatal("DB_MAX_CONNS must be >= 1")
    }
    if c.Database.MinConns > c.Database.MaxConns {
        log.Fatalf("DB_MIN_CONNS (%d) must not exceed DB_MAX_CONNS (%d)",
            c.Database.MinConns, c.Database.MaxConns)
    }
    if c.Server.ReadTimeout <= 0 {
        log.Fatal("READ_TIMEOUT must be positive")
    }
}

// In main():
func main() {
    cfg := loadConfig()
    cfg.validate() // panics here, not at first request
    // ...
}
```

## .env Loading for Development

```go
import "github.com/joho/godotenv"

func main() {
    // Load .env in development — silently skip if missing (production)
    _ = godotenv.Load() // best-effort: .env doesn't exist in production

    cfg := loadConfig()
    cfg.validate()
    // ...
}
```

### .env File

```bash
# .env — NEVER commit this file (.gitignore enforces)
DATABASE_URL=postgres://dev:dev@localhost:5432/app?sslmode=disable
GEMINI_API_KEY=your-dev-key-here
PORT=8080
LOG_LEVEL=debug
CORS_ORIGINS=http://localhost:3000,http://localhost:5173
```

### .env.example

```bash
# .env.example — commit this, document all variables
DATABASE_URL=postgres://user:pass@localhost:5432/dbname?sslmode=disable
GEMINI_API_KEY=
PORT=8080
LOG_LEVEL=info
CORS_ORIGINS=http://localhost:3000
DEFAULT_MODEL=googleai/gemini-2.5-flash
DB_MAX_CONNS=25
DB_MIN_CONNS=5
```

## Wiring Config to Features

NEVER pass the entire config struct to feature packages. Pass only needed values:

```go
func main() {
    cfg := loadConfig()
    cfg.validate()

    logger := setupLogger(cfg.LogLevel)

    // Pool gets database config
    poolCfg, _ := pgxpool.ParseConfig(cfg.Database.URL)
    poolCfg.MaxConns = cfg.Database.MaxConns
    poolCfg.MinConns = cfg.Database.MinConns
    poolCfg.MaxConnLifetime = cfg.Database.MaxConnLifetime
    pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
    // ...

    // Server gets server config
    srv := &http.Server{
        Addr:         ":" + cfg.Server.Port,
        Handler:      handler,
        ReadTimeout:  cfg.Server.ReadTimeout,
        WriteTimeout: cfg.Server.WriteTimeout,
    }

    // CORS middleware gets origin list
    corsMiddleware := cors(cfg.CORS.AllowedOrigins)

    // Auth middleware gets secret
    authMiddleware := requireAuth(cfg.JWTSecret)
}
```

## Sensitive Value Redaction

```go
// For slog structured logging
func (c config) LogValue() slog.Value {
    return slog.GroupValue(
        slog.String("port", c.Server.Port),
        slog.String("database_url", redactURL(c.Database.URL)),
        slog.String("ai_api_key", "[REDACTED]"),
        slog.String("default_model", c.AI.DefaultModel),
    )
}

func redactURL(rawURL string) string {
    u, err := url.Parse(rawURL)
    if err != nil {
        return "[INVALID_URL]"
    }
    if u.User != nil {
        u.User = url.UserPassword("***", "***")
    }
    return u.String()
}

// Log at startup
logger.Info("config loaded", "config", cfg)
// Output: config loaded config.port=8080 config.database_url=postgres://***:***@localhost:5432/app config.ai_api_key=[REDACTED]
```

## Testing Config Override

```go
// In tests — set env vars for the test, use t.Setenv for auto-cleanup
func TestLoadConfig(t *testing.T) {
    t.Setenv("DATABASE_URL", "postgres://test:test@localhost:5432/test")
    t.Setenv("GEMINI_API_KEY", "test-key")
    t.Setenv("PORT", "9999")

    cfg := loadConfig()

    if cfg.Server.Port != "9999" {
        t.Errorf("loadConfig().Server.Port = %q, want %q", cfg.Server.Port, "9999")
    }
}
```

### Feature-Level Config in Tests

```go
// Feature packages accept individual values, not config struct
// So tests just pass values directly — no config loading needed
func TestOrderStore(t *testing.T) {
    pool := setupTestDB(t) // testcontainers — see testcontainers skill
    store := order.NewStore(db.New(pool))
    // test store methods directly
}
```

## Logger Setup from Config

```go
func setupLogger(level string) *slog.Logger {
    var logLevel slog.Level
    switch strings.ToLower(level) {
    case "debug":
        logLevel = slog.LevelDebug
    case "warn":
        logLevel = slog.LevelWarn
    case "error":
        logLevel = slog.LevelError
    default:
        logLevel = slog.LevelInfo
    }

    return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: logLevel,
    }))
}
```

## Rules

- Config struct lives in `cmd/app/main.go` only
- `requireEnv` + `log.Fatalf` for required values — fail at startup
- `getEnv` with defaults for optional values
- NEVER read `os.Getenv` inside feature packages
- NEVER pass entire config struct to features — pass individual values
- NEVER commit `.env` files — commit `.env.example` instead
- NEVER log API keys, passwords, or tokens — implement `slog.LogValuer`

## Dependency Wiring

### Wiring Order

```
config → logger → pool → stores → handlers → middleware → mux → server
```

Each step depends on the previous. This order is not arbitrary — it reflects
dependency flow. If you wire out of order, you'll have nil pointer panics.

### Pattern

```go
// main.go — explicit wiring, no framework
func main() {
    cfg := loadConfig()
    cfg.validate()

    logger := setupLogger(cfg.LogLevel)
    slog.SetDefault(logger)

    ctx := context.Background()
    pool, err := pgxpool.NewWithConfig(ctx, poolConfig(cfg.Database))
    if err != nil {
        log.Fatalf("creating pool: %v", err)
    }
    defer pool.Close()

    queries := db.New(pool)

    // Feature wiring — each feature gets only what it needs
    orderStore := order.NewStore(queries)
    userStore := user.NewStore(queries)

    mux := http.NewServeMux()
    order.RegisterRoutes(mux, orderStore, logger)
    user.RegisterRoutes(mux, userStore, logger)

    handler := chain(recovery, requestID, withLogger(logger), corsHandler)(mux)

    srv := &http.Server{
        Addr:         ":" + cfg.Server.Port,
        Handler:      handler,
        ReadTimeout:  cfg.Server.ReadTimeout,
        WriteTimeout: cfg.Server.WriteTimeout,
    }
    // ... graceful shutdown ...
}
```

### Growth Strategy

```
Feature count   Wiring pattern
─────────────   ──────────────────────────────────────────
< 5 features    Flat wiring in main()
5-10 features   Group into wireOrder(mux, q, log), wireUser(mux, q, log) helpers
> 10 features   Wire functions return cleanup func for graceful shutdown:
                  cleanup := wireOrder(mux, q, log) → defer cleanup()
```

### Anti-Patterns

```go
// ❌ WRONG — global variable for dependency
var db *pgxpool.Pool // set in main, used everywhere

func handler(w http.ResponseWriter, r *http.Request) {
    db.Query(r.Context(), ...) // implicit dependency, untestable
}

// ✅ CORRECT — explicit dependency injection via closure
func getOrder(store *order.Store, logger *slog.Logger) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // store is explicit, testable
    }
}

// ❌ WRONG — passing entire config to feature
func NewStore(cfg config) *Store { ... } // feature sees DB password, API keys

// ✅ CORRECT — pass only needed values
func NewStore(queries *db.Queries) *Store { ... }

// ❌ WRONG — init() for dependency setup
func init() {
    pool, _ = pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
}
// Untestable, implicit ordering, fails silently

// ❌ WRONG — DI framework
func main() {
    container := dig.New()
    container.Provide(pgxpool.New)
    container.Provide(order.NewStore)
    container.Invoke(func(s *order.Store) { ... })
}
// Magic, implicit, hard to debug — just write the 5 lines of explicit wiring
```

See: graceful-shutdown skill for shutdown ordering that mirrors wiring order.
See: go-middleware skill for middleware chain composition.

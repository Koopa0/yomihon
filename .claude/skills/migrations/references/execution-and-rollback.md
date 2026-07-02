# Running Migrations in Go Code

For `cmd/app/main.go` startup:

```go
import (
    "github.com/golang-migrate/migrate/v4"
    _ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
    _ "github.com/golang-migrate/migrate/v4/source/file"
)

func runMigrations(databaseURL string) error {
    m, err := migrate.New("file://migrations", databaseURL)
    if err != nil {
        return fmt.Errorf("creating migrator: %w", err)
    }
    defer m.Close()

    if err := m.Up(); err != nil && err != migrate.ErrNoChange {
        return fmt.Errorf("running migrations: %w", err)
    }
    return nil
}
```

# Running Migrations in Tests (testcontainers)

Apply migrations to the test database before running store tests:

```go
//go:build integration

func setupTestDB(t *testing.T) *pgxpool.Pool {
    t.Helper()
    ctx := t.Context()

    pg, err := postgres.Run(ctx, "postgres:17-alpine")
    if err != nil {
        t.Fatalf("starting postgres: %v", err)
    }
    t.Cleanup(func() { pg.Terminate(ctx) })

    connStr, err := pg.ConnectionString(ctx, "sslmode=disable")
    if err != nil {
        t.Fatalf("getting connection string: %v", err)
    }

    // Run migrations
    m, err := migrate.New("file://../../migrations", connStr)
    if err != nil {
        t.Fatalf("creating migrator: %v", err)
    }
    if err := m.Up(); err != nil && err != migrate.ErrNoChange {
        t.Fatalf("running migrations: %v", err)
    }
    srcErr, dbErr := m.Close()
    if srcErr != nil {
        t.Fatalf("closing migration source: %v", srcErr)
    }
    if dbErr != nil {
        t.Fatalf("closing migration db: %v", dbErr)
    }

    pool, err := pgxpool.New(ctx, connStr)
    if err != nil {
        t.Fatalf("creating pool: %v", err)
    }
    t.Cleanup(func() { pool.Close() })

    return pool
}
```

Note on relative path: `"file://../../migrations"` assumes the test file is in `internal/<feature>/`. Adjust the depth to match your package location relative to the project root.

# Makefile Integration

```makefile
MIGRATE_URL ?= $(DATABASE_URL)

migrate-up:
	migrate -database "$(MIGRATE_URL)" -path migrations up

migrate-down:
	migrate -database "$(MIGRATE_URL)" -path migrations down 1

migrate-create:
	@read -p "Migration name: " name; \
	migrate create -ext sql -dir migrations -seq $$name

migrate-version:
	migrate -database "$(MIGRATE_URL)" -path migrations version

migrate-force:
	@read -p "Force version: " v; \
	migrate -database "$(MIGRATE_URL)" -path migrations force $$v
```

# Team Coordination

## Sequential vs Timestamp Naming

| Approach | Pros | Cons |
|----------|------|------|
| Sequential (001, 002) | Easy to read order | Merge conflicts on numbers |
| Timestamp (20240115120000) | No conflicts | Hard to read order |

For small teams: sequential is fine. For larger teams: consider timestamps.

```bash
# Timestamp-based creation
migrate create -ext sql -dir migrations -format "20060102150405" add_user_email
# Creates: migrations/20240115120000_add_user_email.up.sql
```

## Migration Review Checklist

Before merging a migration:

- [ ] Has corresponding `.down.sql`
- [ ] Down migration is reversible (or documented as irreversible)
- [ ] No table locks on large tables (use CONCURRENTLY for indexes)
- [ ] NOT NULL columns have DEFAULT values
- [ ] Foreign keys reference existing tables
- [ ] Indexes are created for foreign key columns
- [ ] No data backfill in migration (use separate script)
- [ ] Tested locally with `migrate up` and `migrate down`

## Rollback Strategy

```bash
# Check current version
migrate -database "$DB" -path migrations version

# Rollback last migration
migrate -database "$DB" -path migrations down 1

# If migration is dirty (failed halfway)
migrate -database "$DB" -path migrations force <last_good_version>
```

# Migration Testing

## Local Testing Workflow

```bash
# 1. Apply all migrations
migrate -database "postgres://localhost/testdb?sslmode=disable" -path migrations up

# 2. Test rollback
migrate -database "postgres://localhost/testdb?sslmode=disable" -path migrations down 1

# 3. Re-apply
migrate -database "postgres://localhost/testdb?sslmode=disable" -path migrations up 1

# 4. Generate sqlc to verify schema matches queries
sqlc generate

# 5. Build to verify Go code compiles
go build ./...
```

## CI Integration

```yaml
# .github/workflows/test.yml
- name: Test migrations
  run: |
    docker run -d --name postgres -e POSTGRES_PASSWORD=test -p 5432:5432 postgres:17-alpine
    sleep 5
    migrate -database "postgres://postgres:test@localhost/postgres?sslmode=disable" -path migrations up
    migrate -database "postgres://postgres:test@localhost/postgres?sslmode=disable" -path migrations down
    migrate -database "postgres://postgres:test@localhost/postgres?sslmode=disable" -path migrations up
```

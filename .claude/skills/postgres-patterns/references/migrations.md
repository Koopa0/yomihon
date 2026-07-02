# Migration File Patterns

File naming convention and up/down migration templates.

## File Naming

```
migrations/
  001_create_users.up.sql
  001_create_users.down.sql
  002_create_orders.up.sql
  002_create_orders.down.sql
  003_add_order_metadata.up.sql
  003_add_order_metadata.down.sql
```

## Up Migration

```sql
-- 001_create_users.up.sql
BEGIN;

CREATE TABLE users (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email      TEXT NOT NULL,
    name       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_users_email ON users(email);

COMMIT;
```

## Down Migration

```sql
-- 001_create_users.down.sql
DROP TABLE IF EXISTS users;
```

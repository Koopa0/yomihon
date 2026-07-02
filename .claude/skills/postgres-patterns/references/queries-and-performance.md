# Query Analysis and Coordination

CTE composition, advisory locks, and reading EXPLAIN ANALYZE output.

## CTE (Common Table Expression)

```sql
WITH active_orders AS (
    SELECT id, user_id, total_cents
    FROM orders
    WHERE status = 'confirmed'
      AND created_at > now() - interval '30 days'
)
SELECT u.id, u.email, COALESCE(SUM(ao.total_cents), 0) as total
FROM users u
LEFT JOIN active_orders ao ON ao.user_id = u.id
GROUP BY u.id, u.email;
```

## Advisory Locks

```sql
-- Session-level lock (released when connection closes)
SELECT pg_advisory_lock($1);
SELECT pg_advisory_unlock($1);

-- Transaction-level lock (released at end of transaction)
SELECT pg_advisory_xact_lock($1);

-- Try lock (non-blocking)
SELECT pg_try_advisory_lock($1);
```

## EXPLAIN ANALYZE

```sql
EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT)
SELECT * FROM orders WHERE user_id = $1 AND status = 'pending';
```

Key things to look for:
- **Seq Scan** on large tables → add an index
- **Nested Loop** with large outer set → consider Hash Join
- **Buffers: shared hit** → cache hits (good)
- **Buffers: shared read** → disk reads (may need more shared_buffers)
- **Rows Removed by Filter** >> actual rows → index not selective enough

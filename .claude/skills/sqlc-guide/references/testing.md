# Testing with sqlc

Testing material for the sqlc-guide skill: validating queries with sqlc vet and integration-testing generated code against real PostgreSQL.

### Verify Queries Compile

```bash
sqlc vet  # Validates SQL syntax and type checking
```

### Integration Tests (with testcontainers-go)

```go
func TestOrderQueries(t *testing.T) {
    ctx := context.Background()
    pool := setupTestDB(t) // testcontainers-go
    queries := db.New(pool)

    // Test CreateOrder
    order, err := queries.CreateOrder(ctx, db.CreateOrderParams{
        UserID: testUserID,
        Status: "pending",
        Total:  1000,
    })
    if err != nil {
        t.Fatalf("CreateOrder() error = %v", err)
    }

    // Test OrderByID
    got, err := queries.OrderByID(ctx, order.ID)
    if err != nil {
        t.Fatalf("OrderByID() error = %v", err)
    }
    if diff := cmp.Diff(order, got); diff != "" {
        t.Errorf("OrderByID() mismatch (-want +got):\n%s", diff)
    }
}
```

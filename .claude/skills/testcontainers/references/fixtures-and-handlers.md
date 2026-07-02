# Test Data Fixtures & HTTP Handler Integration Tests

## Test Data Fixtures

### Factory Functions

```go
func createTestOrder(t *testing.T, store *order.Store, overrides ...func(*order.CreateParams)) *order.Order {
    t.Helper()
    ctx := t.Context()

    params := order.CreateParams{
        UserID: "test-user-" + uuid.NewString()[:8],
        Status: "pending",
        Total:  1000,
    }
    for _, override := range overrides {
        override(&params)
    }

    created, err := store.Create(ctx, params)
    if err != nil {
        t.Fatalf("creating test order: %v", err)
    }
    return created
}

// Usage
order := createTestOrder(t, store, func(p *order.CreateParams) {
    p.Status = "shipped"
    p.Total = 5000
})
```

### Bulk Data Generation

```go
func createTestOrders(t *testing.T, store *order.Store, count int) []*order.Order {
    t.Helper()
    ctx := t.Context()

    orders := make([]*order.Order, count)
    for i := 0; i < count; i++ {
        orders[i] = createTestOrder(t, store, func(p *order.CreateParams) {
            p.Total = int64(100 * (i + 1))
        })
    }
    return orders
}
```

## Testing HTTP Handlers with Real Database

```go
func TestOrderHandler_Integration(t *testing.T) {
    pool := setupPostgresWithMigrations(t)
    store := order.NewStore(pool)
    logger := slog.New(slog.NewTextHandler(io.Discard, nil))

    // Create test order
    testOrder := createTestOrder(t, store)

    // Test handler
    handler := order.GetOrder(store, logger)

    req := httptest.NewRequest("GET", "/orders/"+testOrder.ID, nil)
    req.SetPathValue("id", testOrder.ID)
    w := httptest.NewRecorder()

    handler.ServeHTTP(w, req)

    if w.Code != http.StatusOK {
        t.Fatalf("GetOrder(%q) status = %d, want %d", testOrder.ID, w.Code, http.StatusOK)
    }

    var got order.Order
    if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
        t.Fatalf("decoding response: %v", err)
    }

    if diff := cmp.Diff(testOrder.ID, got.ID); diff != "" {
        t.Errorf("GetOrder(%q) ID mismatch (-want +got):\n%s", testOrder.ID, diff)
    }
}
```

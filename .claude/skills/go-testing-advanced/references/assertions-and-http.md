# go-cmp Assertions and HTTP Handler Tests

Advanced cmp options and HTTP handler test patterns. The most-used option,
`cmpopts.IgnoreFields`, lives in SKILL.md.

## go-cmp Advanced Usage

### cmpopts.SortSlices

```go
// Compare slices regardless of order
if diff := cmp.Diff(want, got,
    cmpopts.SortSlices(func(a, b Order) bool {
        return a.ID < b.ID
    }),
); diff != "" {
    t.Errorf("Orders() mismatch (-want +got):\n%s", diff)
}
```

### cmpopts.EquateApprox

```go
// Float comparison with tolerance
if diff := cmp.Diff(want, got,
    cmpopts.EquateApprox(0.001, 0), // relative tolerance 0.1%
); diff != "" {
    t.Errorf("Calculate() mismatch (-want +got):\n%s", diff)
}
```

### cmpopts.EquateEmpty

```go
// Treat nil and empty slices/maps as equal
if diff := cmp.Diff(want, got,
    cmpopts.EquateEmpty(),
); diff != "" {
    t.Errorf("mismatch (-want +got):\n%s", diff)
}
```

### Custom Comparer

```go
// Compare time.Time with tolerance
timeCmp := cmp.Comparer(func(a, b time.Time) bool {
    return a.Sub(b).Abs() < time.Second
})

if diff := cmp.Diff(want, got, timeCmp); diff != "" {
    t.Errorf("mismatch (-want +got):\n%s", diff)
}
```

### Combining Options

```go
opts := cmp.Options{
    cmpopts.IgnoreFields(Order{}, "ID", "CreatedAt"),
    cmpopts.SortSlices(func(a, b Item) bool { return a.Name < b.Name }),
    cmpopts.EquateEmpty(),
}

if diff := cmp.Diff(want, got, opts...); diff != "" {
    t.Errorf("mismatch (-want +got):\n%s", diff)
}
```

## HTTP Handler Test Patterns

### Testing with Context Values

```go
func TestProtectedEndpoint(t *testing.T) {
    handler := getProfile(store, slog.Default())

    req := httptest.NewRequest("GET", "/profile", nil)
    // Simulate authenticated user (normally set by auth middleware)
    ctx := context.WithValue(req.Context(), userIDKey{}, "user-123")
    req = req.WithContext(ctx)

    w := httptest.NewRecorder()
    handler.ServeHTTP(w, req)

    if w.Code != http.StatusOK {
        t.Fatalf("getProfile() status = %d, want %d", w.Code, http.StatusOK)
    }
}
```

### Response Body Assertions

```go
func assertJSONResponse[T any](t *testing.T, w *httptest.ResponseRecorder, want T, opts ...cmp.Option) {
    t.Helper()

    if ct := w.Header().Get("Content-Type"); ct != "application/json" {
        t.Errorf("Content-Type = %q, want %q", ct, "application/json")
    }

    var got T
    if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
        t.Fatalf("decoding response body: %v", err)
    }

    if diff := cmp.Diff(want, got, opts...); diff != "" {
        t.Errorf("response body mismatch (-want +got):\n%s", diff)
    }
}
```

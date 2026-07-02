# Golden Files and Test Fixtures

How to set up golden file comparisons and reusable test data. The golden file
RULES (review policy, naming, commit policy) live in SKILL.md.

## Golden File Testing

Golden files store expected output and auto-update with `-update` flag.
Ideal for testing serialization, template rendering, API responses, and
prompt output.

### Pattern

```go
package order_test

import (
    "flag"
    "os"
    "path/filepath"
    "testing"

    "github.com/google/go-cmp/cmp"
)

var update = flag.Bool("update", false, "update golden files")

func TestOrderJSON(t *testing.T) {
    order := Order{
        ID:     "abc-123",
        Status: StatusActive,
        Total:  4999,
    }

    got, err := json.MarshalIndent(order, "", "  ")
    if err != nil {
        t.Fatalf("marshaling order: %v", err)
    }

    golden := filepath.Join("testdata", t.Name()+".golden")

    if *update {
        // Go 1.22+: testdata dir must exist
        if err := os.MkdirAll("testdata", 0o755); err != nil {
            t.Fatalf("creating testdata dir: %v", err)
        }
        if err := os.WriteFile(golden, got, 0o644); err != nil {
            t.Fatalf("updating golden file: %v", err)
        }
        return
    }

    want, err := os.ReadFile(golden)
    if err != nil {
        t.Fatalf("reading golden file %s: %v (run with -update to create)", golden, err)
    }

    if diff := cmp.Diff(string(want), string(got)); diff != "" {
        t.Errorf("OrderJSON() mismatch (-want +got):\n%s\nRun with -update to accept changes.", diff)
    }
}
```

### Usage

```bash
# Run tests normally (compare against golden files)
go test ./internal/order/...

# Update golden files after intentional changes
go test ./internal/order/... -update

# Always review the diff before committing updated golden files
git diff testdata/
```

## Test Fixtures

### Shared Setup with t.Cleanup

```go
func setupStore(t *testing.T) *Store {
    t.Helper()
    ctx := t.Context() // Go 1.24+

    pool := setupPostgres(t) // from testcontainers
    runMigrations(t, pool)

    store := NewStore(pool)

    // Seed test data
    _, err := store.CreateOrder(ctx, CreateParams{
        Total:  1000,
        Status: StatusPending,
    })
    if err != nil {
        t.Fatalf("seeding test data: %v", err)
    }

    // No need for explicit cleanup — t.Cleanup in setupPostgres handles it
    return store
}
```

### Test Data Builder

```go
// For complex types with many fields, use a builder function
func testOrder(t *testing.T, mods ...func(*Order)) *Order {
    t.Helper()
    o := &Order{
        ID:        "test-" + t.Name(),
        Status:    StatusPending,
        Total:     1000,
        CreatedAt: time.Now(),
    }
    for _, mod := range mods {
        mod(o)
    }
    return o
}

// Usage
func TestOrderTotal(t *testing.T) {
    o := testOrder(t, func(o *Order) {
        o.Total = 5000
    })
    // ...
}
```

### File-Based Fixtures

```go
func loadFixture(t *testing.T, name string) []byte {
    t.Helper()
    data, err := os.ReadFile(filepath.Join("testdata", name))
    if err != nil {
        t.Fatalf("loading fixture %s: %v", name, err)
    }
    return data
}

func TestParseOrder(t *testing.T) {
    input := loadFixture(t, "order_valid.json")
    got, err := ParseOrder(input)
    if err != nil {
        t.Fatalf("ParseOrder() error: %v", err)
    }
    // ...
}
```

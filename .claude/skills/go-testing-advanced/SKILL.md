---
name: go-testing-advanced
description: >-
  Advanced Go testing patterns — golden file testing, test fixtures,
  coverage workflow, go-cmp advanced usage (cmpopts), subtests with
  t.Parallel, test naming conventions, and test helper design. Complements
  the testing.md rule (MUST/NEVER) with deep how-to patterns.
when_to_use: >-
  Use when writing complex tests, setting up golden file comparisons,
  designing test fixtures or helpers, measuring coverage, or when
  t.Parallel / t.Setenv interaction is unclear. Trigger keywords: golden
  files, testdata, -update flag, cmp.Diff, cmpopts, subtests, t.Helper,
  coverage report.
metadata:
  author: koopa
  version: "1.0"
  lang: go
---

# Go Advanced Testing Patterns

## Boundary: What Goes Where

```
testing.md rule     = MUST/NEVER rules (format, naming, t.Fatal vs t.Error)
testcontainers skill = how to set up real PostgreSQL containers
go-testing-advanced  = how to write BETTER tests (this file)
```

## Test Naming Conventions

- `TestXxx` mirrors the function under test: `ParseStatus` → `TestParseStatus`.
- Scenario variants use `TestXxx_scenario` — underscore separates the function
  name from the scenario: `TestParseStatus_invalidInput`.
- Subtest names in `t.Run` are short lowercase phrases. Spaces become
  underscores in `-run` patterns (`-run TestParseStatus/empty_input`) — keep
  them shell-friendly.
- `BenchmarkXxx` and `FuzzXxx` mirror the function name the same way:
  `BenchmarkParseStatus`, `FuzzParseStatus`.
- Helpers NEVER start with `Test` — `setupStore`, `loadFixture`, not
  `TestSetupStore` (the test runner would try to run them).
- Failure messages follow got-before-want: `FuncName(input) = got, want expected`
  (testing.md).

Package policy: unit/handler/fuzz/bench tests are white-box (`package <feature>`);
`integration_test.go` is black-box (`package <feature>_test`) — see
testing.md § Test Packages.

## When to Use t.Parallel

```
Is the test independent of other tests?
├─ No (shared state, env vars, global config) → don't parallelize
├─ Yes → Does it use t.Setenv?
│   ├─ Yes → don't parallelize
│   └─ No  → Does the parent test use t.Parallel?
│       ├─ Yes → add t.Parallel to subtest too
│       └─ No  → add t.Parallel to both parent and subtest
```

Full parallel subtest example: references/parallel-and-synctest.md.

## t.Setenv Panics in Parallel Tests

```go
// ❌ WRONG — t.Setenv panics if test or parent is parallel
func TestConfig(t *testing.T) {
    t.Parallel()
    t.Setenv("PORT", "9090") // PANIC: cannot use t.Setenv in parallel test
}

// ✅ CORRECT — don't use t.Parallel with t.Setenv
func TestConfig(t *testing.T) {
    t.Setenv("PORT", "9090") // fine without t.Parallel
    cfg := loadConfig()
    if cfg.Port != "9090" {
        t.Errorf("Port = %q, want %q", cfg.Port, "9090")
    }
}

// ✅ ALSO CORRECT — use a non-parallel subtest for env manipulation
func TestConfig(t *testing.T) {
    t.Run("port override", func(t *testing.T) {
        // NOT parallel — intentional
        t.Setenv("PORT", "9090")
        // ...
    })
}
```

## Helper vs Subtest

```
Is this reusable setup code?
├─ Yes → helper function (t.Helper + t.Cleanup)
├─ No  → Is this a distinct test scenario?
│   ├─ Yes → t.Run subtest
│   └─ No  → inline code
```

## Rules for Helpers

```go
// 1. Always call t.Helper()
// 2. Accept *testing.T as first parameter
// 3. Use t.Fatal for setup failures (caller can't continue)
// 4. Use t.Cleanup for teardown (not defer)
// 5. Return values, don't use output params

func mustParseJSON[T any](t *testing.T, data []byte) T {
    t.Helper()
    var v T
    if err := json.Unmarshal(data, &v); err != nil {
        t.Fatalf("parsing JSON: %v", err)
    }
    return v
}
```

## Golden File Rules

- Store in `testdata/` directory (ignored by `go build`)
- Name after test: `TestName.golden` or `TestName/subtest.golden`
- ALWAYS review `git diff testdata/` before committing updates AND state why
  the new output is correct — NEVER regenerate just to go green
  (testing.md § Low-Value Tests #8)
- Golden files are committed to git (they ARE the expected output)

Full golden-file pattern and `-update` workflow:
references/golden-files-and-fixtures.md.

## What to Cover

| Priority | What | Target |
|----------|------|--------|
| HIGH | Store methods | 80%+ (real DB via testcontainers) |
| HIGH | Parsing/validation | 90%+ (table-driven + fuzz) |
| MEDIUM | HTTP handlers | 70%+ (success + error paths) |
| LOW | Wiring (main.go) | 0% (tested via integration tests) |
| SKIP | Generated code (internal/db/) | 0% (excluded) |

Coverage commands (profiles, -coverpkg, CI threshold):
references/coverage-and-test-output.md.

## go-cmp: cmpopts.IgnoreFields (most-used option)

```go
// Ignore auto-generated fields
if diff := cmp.Diff(want, got,
    cmpopts.IgnoreFields(Order{}, "ID", "CreatedAt", "UpdatedAt"),
); diff != "" {
    t.Errorf("CreateOrder() mismatch (-want +got):\n%s", diff)
}
```

More cmp options (SortSlices, EquateApprox, EquateEmpty, custom comparers,
combining options): references/assertions-and-http.md.

## Anti-Patterns

### Testing Implementation, Not Behavior

```go
// ❌ WRONG — tests internal state
func TestStore(t *testing.T) {
    s := NewStore(pool)
    if s.q == nil {
        t.Fatal("queries is nil") // who cares? test the behavior
    }
}

// ✅ CORRECT — tests behavior
func TestCreateOrder(t *testing.T) {
    s := NewStore(pool)
    order, err := s.CreateOrder(t.Context(), params)
    if err != nil {
        t.Fatalf("CreateOrder() error: %v", err)
    }
    if order.Status != StatusPending {
        t.Errorf("CreateOrder().Status = %v, want %v", order.Status, StatusPending)
    }
}
```

### Over-Mocking

```go
// ❌ WRONG — mocking the database for store tests
type mockDB struct { ... }
func TestStoreOrder(t *testing.T) {
    store := NewStore(&mockDB{...})
    // Tests the mock, not the actual SQL
}

// ✅ CORRECT — real database via testcontainers
func TestStoreOrder(t *testing.T) {
    pool := setupPostgres(t)
    store := NewStore(pool)
    // Tests actual SQL execution
}
```

See: testcontainers skill for database test setup.

### Shared State Between Tests

```go
// ❌ WRONG — tests depend on each other
var globalStore *Store

func TestCreate(t *testing.T) {
    globalStore.CreateOrder(...)
}
func TestRead(t *testing.T) {
    // depends on TestCreate having run first
    globalStore.Order(...)
}

// ✅ CORRECT — each test sets up its own state
func TestRead(t *testing.T) {
    store := setupStore(t) // fresh store with known state
    // seed the data this test needs
    store.CreateOrder(t.Context(), ...)
    // now test the read
    got, err := store.Order(t.Context(), id)
    // ...
}
```

### Ignoring Errors in Setup

```go
// ❌ WRONG — silent failure in setup
func setupStore(t *testing.T) *Store {
    pool, _ := pgxpool.New(ctx, connStr) // error ignored!
    return NewStore(pool)
}

// ✅ CORRECT — fail fast with t.Fatal
func setupStore(t *testing.T) *Store {
    t.Helper()
    pool, err := pgxpool.New(t.Context(), connStr)
    if err != nil {
        t.Fatalf("creating pool: %v", err)
    }
    return NewStore(pool)
}
```

## Navigation

| Topic | File |
|-------|------|
| Golden file pattern (`-update` flag), fixture setup with t.Cleanup, test data builders, file-based fixtures | references/golden-files-and-fixtures.md |
| Parallel subtest example code; testing/synctest fake-clock concurrent tests (Go 1.25+) | references/parallel-and-synctest.md |
| go-cmp options beyond IgnoreFields (SortSlices, EquateApprox, EquateEmpty, custom comparers, combining); HTTP handler tests (context values, JSON body assertions) | references/assertions-and-http.md |
| Coverage commands (profiles, -coverpkg, CI threshold script); t.Attr structured attributes (Go 1.25+); t.ArtifactDir persistent output (Go 1.26+) | references/coverage-and-test-output.md |

For golden file consumers, read references/golden-files-and-fixtures.md. For
concurrent or parallel test questions, read references/parallel-and-synctest.md.
For assertion or handler-test questions, read references/assertions-and-http.md.
For coverage or test-output questions, read references/coverage-and-test-output.md.

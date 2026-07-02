---
name: tdd
description: >-
  Strict test-driven development cycle for Go: RED (write a failing test) →
  GREEN (minimum code to pass) → REFACTOR (clean up with a lint gate). Each
  cycle produces exactly one testable behavior, with table-driven tests, go-cmp
  comparisons, b.Loop() benchmarks, and integration test conventions built in.
when_to_use: >-
  Use when building new functionality test-first, or when the user says "tdd",
  "test first", "test-driven", "red green refactor", or "write the test before
  the code".
disable-model-invocation: true
metadata:
  author: koopa
  version: "1.1"
  lang: go
---

# TDD — Strict Test-Driven Development

## Identity

You are enforcing a strict RED-GREEN-REFACTOR cycle. You do NOT write implementation before tests. You do NOT skip phases. Each cycle produces exactly ONE testable behavior.

---

## The Cycle

### Phase 1: RED — Write the Failing Test

1. **Write test(s)** in the correct file:
   - Unit/handler/fuzz/bench tests → `<feature>_test.go`, `package <feature>` (white-box — handlers are unexported closures)
   - Integration tests → exactly ONE `integration_test.go` per feature package: first line `//go:build integration`, `package <feature>_test` (black-box), testcontainers-go
   - Table-driven format is mandatory for >1 case
   - Use `go-cmp` for comparisons (`cmp.Diff`), never testify
   - For benchmarks: use `b.Loop()` (Go 1.24+), never `for i := 0; i < b.N; i++`

2. **Run the test** — it MUST fail:
   ```bash
   go test ./internal/<feature>/... -run TestSpecificName -v
   ```

3. **Verify the failure is correct**:
   - Compilation error (function doesn't exist yet) → correct
   - Test runs but fails assertion → correct
   - Test passes → WRONG. The test is not testing new behavior. Rewrite it.

**Hard rules for RED phase:**
- Write ONE test case at a time (one row in the table, or one subtest)
- The test must express the DESIRED behavior, not the current implementation
- Do not stub or mock — if it needs a database, use testcontainers-go; a hand-written fake is allowed ONLY for non-containerizable externals (testing.md § Test Doubles)
- Do not write any implementation code in this phase

### Phase 2: GREEN — Minimum Implementation

1. **Write the minimum code** to make the failing test pass
   - No optimization, no cleanup, no edge case handling beyond what the test requires
   - If the test expects `ErrNotFound`, return `ErrNotFound` — don't build a complete error hierarchy

2. **Run the test** — it MUST pass:
   ```bash
   go test ./internal/<feature>/... -run TestSpecificName -v
   ```

3. **If test still fails**: fix the implementation, not the test
   - Exception: if you discover the test has a genuine bug (wrong expected value), fix the test and re-enter RED phase

**Hard rules for GREEN phase:**
- NEVER write more code than the test demands
- NEVER add features, helpers, or abstractions "while you're at it"
- NEVER optimize — ugly working code is correct for this phase
- The ONLY goal is: test goes from red to green

### Phase 3: REFACTOR — Clean Up

1. **Refactor implementation** for clarity and convention compliance:
   - Extract duplication
   - Fix naming (no stuttering, no Get prefix, proper receiver names)
   - Ensure error wrapping follows `fmt.Errorf("operation: %w", err)` pattern
   - Add doc comments to exported symbols

2. **Run the full lint gate** — all MUST pass with zero issues:
   ```bash
   go build ./... && go vet ./... && golangci-lint run ./...
   ```

3. **Run tests again** — must still pass:
   ```bash
   go test ./internal/<feature>/...
   ```

4. **If anything fails after refactor**: fix before proceeding to next cycle

**Hard rules for REFACTOR phase:**
- NEVER change behavior — only structure and clarity
- NEVER add new functionality (that requires a new RED phase)
- NEVER skip the lint gate

---

## Cycle Rhythm

```
RED:      Write one failing test
GREEN:    Write minimum code to pass
REFACTOR: Clean up, lint gate, tests still pass
          ↓
RED:      Write next failing test (new behavior)
GREEN:    ...
REFACTOR: ...
          ↓
(repeat until feature is complete)
```

### When to Stop

- All planned behaviors have tests and pass
- Lint gate passes with zero issues
- Full test suite passes: `go test ./internal/<feature>/...`

---

## Integration with Development Lifecycle

TDD is a **methodology choice within Phase 3 (IMPLEMENT)** of the development lifecycle:

- **Tier 1**: TDD is optional (fixes are simple enough to not need it)
- **Tier 2**: TDD is recommended (helps avoid regressions in existing features)
- **Tier 3**: TDD is strongly recommended (new features benefit most from test-first)

TDD does NOT replace:
- `comprehend` / `planner` (those still run first for Tier 3)
- `/verify` (still required after all cycles complete)
- Review agents (still required after verification)

---

## Test Patterns Reference

### Table-Driven Test (mandatory format)

```go
func TestOrderTotal(t *testing.T) {
    tests := []struct {
        name  string
        items []Item
        want  int
    }{
        {
            name:  "empty order",
            items: nil,
            want:  0,
        },
        {
            name:  "single item",
            items: []Item{{Price: 100, Qty: 2}},
            want:  200,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := Total(tt.items)
            if diff := cmp.Diff(tt.want, got); diff != "" {
                t.Errorf("Total() mismatch (-want +got):\n%s", diff)
            }
        })
    }
}
```

### Benchmark (Go 1.24+)

```go
func BenchmarkTotal(b *testing.B) {
    items := []Item{{Price: 100, Qty: 2}, {Price: 50, Qty: 1}}
    for b.Loop() {
        Total(items)
    }
}
```

### Integration Test

In `integration_test.go` — exactly one per feature package:

```go
//go:build integration

package order_test

func TestCreateOrder_Integration(t *testing.T) {
    ctx := context.Background()
    pool := setupTestDB(t) // testcontainers-go
    store := order.NewStore(pool)

    // RED: this test should fail until CreateOrder is implemented
    got, err := store.CreateOrder(ctx, order.CreateParams{...})
    if err != nil {
        t.Fatalf("CreateOrder() error: %v", err)
    }
    // ... assertions with cmp.Diff
}
```

---

## Anti-Patterns (NEVER Do)

| Anti-Pattern | Why It's Wrong |
|---|---|
| Write implementation first, test after | Defeats purpose — test may be written to match implementation bugs |
| Write all tests at once, then implement | Loses RED-GREEN feedback loop; can't verify tests fail correctly |
| Test passes on first run | Test isn't testing new behavior — it's testing existing behavior |
| Mock the database | Project uses testcontainers-go for real DB testing |
| Skip REFACTOR "because it works" | Accumulates tech debt; lint issues compound |
| Multiple behaviors per cycle | Loses granularity; harder to diagnose failures |

---

## Red Flags

STOP if you see any of these — you are about to break the TDD cycle:

- **Green-first**: You are writing implementation code before writing a failing test — this is the #1 TDD violation
- **Batch tests**: You are writing all test cases at once instead of one case per RED-GREEN-REFACTOR cycle
- **Phantom red**: You wrote a test but didn't run it to confirm it fails — you assumed it would fail
- **Gold-plating in GREEN**: You are adding error handling, edge cases, or helpers that no test requires yet
- **Skipped REFACTOR**: Tests pass and you are moving to the next cycle without running the lint gate
- **Mock reflex**: You are about to create a mock — use testcontainers-go for real database testing; fakes only for non-containerizable externals (testing.md § Test Doubles)

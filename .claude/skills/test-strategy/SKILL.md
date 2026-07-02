---
name: test-strategy
description: >-
  Test strategy decision tree (Q0-Q6) that determines WHICH test types each
  function needs based on characteristics like external I/O, trust boundaries,
  and stated performance claims, producing a Pre-Test Analysis test plan table.
  Complements testing.md, which governs HOW each test type is written.
when_to_use: >-
  Use before writing tests for any new function, package, or feature — when
  deciding which test types to write, during the test-writer agent's Pre-Test
  Analysis step, or via /test-strategy. Trigger phrases: "what tests should
  this have", "test plan", "which tests to write", "do we need a benchmark /
  fuzz / integration test".
metadata:
  author: koopa
  version: "1.0"
  lang: go
  sunset: >-
    If 5 consecutive Pre-Test Analysis results match what testing.md static
    rules would produce (decision tree adds no value), downgrade to on-demand.
---

# Test Strategy — Decision Tree

## Priority Declaration

This skill decides **which test types each function needs**.
The concrete **how to write** each test type is governed by `testing.md` and `go-testing-advanced`.

When this decision tree conflicts with testing.md static rules:
- **Whether to write a test type** → this skill takes precedence
- **How to write that test** → testing.md takes precedence

Example: the decision tree (Q4) decides WHETHER a function gets a benchmark —
only a stated performance claim or a measured hot path qualifies. Once a
benchmark exists, testing.md governs HOW: `b.Loop()`, and every claim backed by
`benchstat` over `-count=10` runs of both old and new code
(`make bench-baseline` / `make bench-compare`).

---

## Decision Tree

Complete all applicable questions for each public function/method.

### Q0: Pure business logic?
(State machine transitions, permission checks, calculation rules, data transforms — no I/O, HTTP, or concurrency)

**YES →**
- Table-driven unit test (mandatory)
- Focus: edge cases, boundary values, error paths
- Coverage target: >= 85%
- Continue to Q2 (may need fuzz) and Q4 (may have hot path)

### Q1: Involves external I/O?
(Database, HTTP calls, filesystem, message queue)

**YES (database / HTTP / NATS) →**
- Integration test mandatory:
  - PostgreSQL → testcontainers-go (PostgreSQL module)
  - NATS → testcontainers-go (NATS module) — preferred path. An embedded
    nats-server is a NEW dependency: it may only appear as an explicitly
    flagged option, never as the default route.
  - HTTP dependencies → `httptest.Server`
- All integration tests for a feature live in exactly ONE `integration_test.go`
  per feature package — first line `//go:build integration`,
  `package <feature>_test`
- Unit test covers pure logic portions only
- Do not mock the database or the broker — use testcontainers-go

**YES (filesystem) →** `t.TempDir()` in a unit test — NOT an integration test

**NO →** Unit test is sufficient

### Q2: Takes untrusted bytes/strings across a trust boundary?
(HTTP body parsing, decoders, custom format parsers, validators of raw input)

**YES →**
- Fuzz test mandatory — the fuzz target is the function that accepts the raw
  `[]byte`/`string`
- Table-driven unit test for known edge cases

**NO →** No fuzz test needed. Validation of an already-decoded struct gets
table-driven unit tests, not fuzz.

### Q3: HTTP handler?

**YES →**
- Handler test mandatory (httptest.NewRequest + SetPathValue)
- Cover: status codes, response body, error responses, auth middleware

**NO →** No handler test needed

### Q4: Stated performance claim or measured hot path?
(A performance claim you intend to make, or a path pprof/production data shows is hot)

**YES →**
- Benchmark mandatory (b.Loop)
- Every performance claim MUST be verified with `benchstat` over `-count=10`
  runs of BOTH old and new code: `make bench-baseline` → apply change →
  `make bench-compare`. `~` or p ≥ 0.05 = no proven difference — never a win.
- Consider pre-allocation and escape analysis impact

**NO →** No benchmark. "Looks hot" is not a measurement; write one only when a
claim or a profile appears.

### Q5: Complex structured output?
(JSON response, generated files, reports)

**YES →**
- Consider golden file test (testdata/ + -update flag)

**NO →** No golden file test needed

### Q6: Concurrent access?
(Shared state, goroutine interaction, channels)

**YES →**
- testing/synctest mandatory (Go 1.25+)
- Race detector: `go test -race`

**NO →** Standard testing is sufficient

---

## Pre-Test Analysis Output Format

Before writing any test, produce this table:

```
| Function     | Unit | Integration | Fuzz | Benchmark | Handler | Golden | synctest |
|--------------|------|-------------|------|-----------|---------|--------|----------|
| CreateOrder  | ✓    | ✓ (DB)      |      |           |         |        |          |
| ParseRequest | ✓    |             | ✓    |           |         |        |          |
| CalcDiscount | ✓(Q0)|             |      |           |         |        |          |
| HandleGet    | ✓    | ✓ (DB)      |      |           | ✓       |        |          |
| SyncState    | ✓    |             |      |           |         |        | ✓ (map)  |
```

Parenthetical notes explain why (e.g., "Q0" = pure logic, "DB" = database I/O, "map" = shared map).

User confirms the table before test writing begins.

---

## Coverage Thresholds (Soft Enforcement)

Integrated into the verify skill's checklist. Not a hard gate — reported to user, not blocking commit.

1. Run `go test -coverprofile=coverage.out ./...`
2. Check with `go tool cover -func=coverage.out`
3. Reference thresholds:
   - store/ packages: >= 80%
   - parsing/validation: >= 90%
   - pure business logic: >= 85%
   - handlers: >= 70%
   - overall: >= 75%
4. Below threshold → report to user, recommend adding tests
   (Do not block commit — avoids disrupting TDD/prototype flow)

---

## Integration Points

- **test-writer agent**: runs Pre-Test Analysis as first step (before writing any test)
- **verify skill**: includes coverage threshold check after running tests
- **tdd skill**: decision tree runs once at the start of the first RED phase to set the test plan, not repeated per cycle
- **testing.md**: governs all test writing mechanics — this skill only governs test type selection

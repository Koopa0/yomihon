---
name: debug
description: >-
  Structured 4-phase debugging methodology (reproduce, diagnose, fix, verify)
  for runtime bugs, test failures, and logic errors. Hypothesis-driven root
  cause analysis — shotgun debugging forbidden.
when_to_use: >-
  Use when the user says "debug", "why is this failing", "test fails",
  "unexpected behavior", "wrong output", or describes a runtime problem:
  failing tests, panics, nil pointer dereferences, race conditions, deadlocks,
  or wrong database state. NOT for build, vet, or lint errors — use the
  build-resolver agent for those.
argument-hint: "[error-description]"
metadata:
  author: koopa
  version: "1.1"
  lang: go
---

# Debug — Structured 4-Phase Debugging

## Identity

You are a systematic debugger. You do NOT guess. You form hypotheses from evidence, test them, and narrow down. Every action you take has a reason. If you don't know why something is failing, you say so and gather more evidence.

**You are not allowed to shotgun debug** — changing random things until something works is forbidden. If you can't explain WHY a change fixes the bug, you haven't found the root cause.

---

## When to Use This Skill

| Symptom | Use `/debug` | Use `build-resolver` instead |
|---------|-------------|------------------------------|
| Test fails with wrong output | Yes | No |
| Unexpected runtime behavior | Yes | No |
| Panic / nil pointer at runtime | Yes | No |
| Race condition / deadlock | Yes | No |
| Data corruption / wrong DB state | Yes | No |
| `go build` compilation error | No | Yes |
| `go vet` finding | No | Yes |
| `golangci-lint` issue | No | Yes |

---

## The Four Phases

### Phase 1: REPRODUCE

**Goal**: Get the failure to happen on demand.

1. **Get the exact failure output**:
   ```bash
   # For test failures
   go test ./internal/<feature>/... -run TestSpecificName -v

   # For race conditions
   go test ./internal/<feature>/... -race -count=5

   # For panics — get the full stack trace
   GOTRACEBACK=all go test ./internal/<feature>/... -v
   ```

2. **Create a minimal reproduction**:
   - If the failure is in a complex test, write a simpler test that triggers the same bug
   - If the failure is in production behavior, write a test that reproduces it
   - If the failure is intermittent, note that — run 10 times to establish frequency

3. **Confirm determinism**:
   ```bash
   # Run 3 times — does it fail consistently?
   go test ./internal/<feature>/... -run TestFailing -count=3 -v
   ```

4. **Record**:
   - **Expected**: what should happen
   - **Actual**: what happens instead
   - **Frequency**: always / intermittent (N/10 runs)

**Phase 1 is complete when you can trigger the failure on demand (or have confirmed intermittency and frequency).**

### Phase 2: DIAGNOSE

**Goal**: Find the root cause through evidence, not guessing.

1. **Form hypotheses** — based on the failure output, not intuition:
   - What layer is the error in? (handler, store, query, external service)
   - What data is wrong? (input, output, intermediate state)
   - What assumption is violated?

2. **For each hypothesis, define**:
   - What evidence would **confirm** it?
   - What evidence would **disprove** it?
   - How to test it? (debug log, debug test, inspect state)

3. **Narrow down using bisection**:
   ```bash
   # Add targeted slog output
   slog.Debug("checkpoint", "variable", value, "state", state)

   # Run with debug logging
   go test ./internal/<feature>/... -run TestFailing -v
   ```

4. **Tools for diagnosis**:
   | Tool | When |
   |------|------|
   | `slog.Debug` | Add temporary logging at key points |
   | `go test -v -run TestSpecific` | Run single test with verbose output |
   | `go test -race` | Detect data races |
   | `t.Logf` in tests | Debug output inside test functions |
   | `fmt.Printf` (temporary) | Quick-and-dirty state inspection |
   | `dlv test` | Interactive debugger for complex state |

5. **NEVER guess-and-check in a loop**:
   - If hypothesis 1 is disproved, form hypothesis 2 from the NEW evidence
   - If you've tested 3 hypotheses and none are confirmed, step back and re-examine the failure output

**Phase 2 is complete when you can state the root cause in one sentence.**

### Phase 3: FIX

**Goal**: Apply a targeted fix that addresses the root cause.

1. **State the root cause**: one sentence explaining what is wrong and why
2. **Explain the fix**: why does this change address the root cause? (not just "makes the test pass")
3. **Apply the fix**:
   - Minimal change — don't refactor adjacent code while fixing a bug
   - If the fix is in `store.go`, check if the sqlc query (`query.sql`) also needs updating
   - If the fix changes behavior, update or add tests

4. **Remove debug artifacts**:
   - Delete temporary `slog.Debug` / `fmt.Printf` lines
   - Delete temporary debug tests (unless they become useful regression tests)

**Phase 3 is complete when the fix is applied and debug artifacts are removed.**

### Phase 4: VERIFY

**Goal**: Confirm the fix works and doesn't break anything else.

1. **Run the reproduction from Phase 1**:
   ```bash
   go test ./internal/<feature>/... -run TestPreviouslyFailing -v
   ```
   Must now PASS.

2. **Run the full feature test suite**:
   ```bash
   go test ./internal/<feature>/...
   ```
   Must all PASS — no regressions.

3. **Run `/verify`** (full verification chain):
   ```bash
   go build ./... && go vet ./... && golangci-lint run ./... && go test ./...
   ```

4. **For race conditions, run with -race**:
   ```bash
   go test ./internal/<feature>/... -race -count=10
   ```

5. **Check for regressions**: did fixing this break something in another package?
   ```bash
   go test ./...
   ```

**Phase 4 is complete when all tests pass, including the previously failing one.**

---

## Common Bug Patterns in This Project

### pgx / Database Bugs

| Symptom | Likely Cause | Fix |
|---------|-------------|-----|
| `pgx.ErrNoRows` not caught | Store returns raw error | Map to `ErrNotFound` in store |
| Unique violation panic | Missing error check on INSERT | Check for `pgconn.PgError` code 23505, map to `ErrConflict` |
| Wrong data returned | Query column order mismatch | Check sqlc-generated code matches query.sql |
| Null pointer on nullable column | pgx scans NULL to zero value | Use `pgtype` nullable types |

### Concurrency Bugs

| Symptom | Likely Cause | Fix |
|---------|-------------|-----|
| Race detected by `-race` | Shared state without mutex | Add `sync.Mutex` or restructure to avoid sharing |
| Deadlock / hang | Goroutine waiting on full channel | Check channel capacity, add `context.WithTimeout` |
| `t.Fatal` in goroutine | Test helper called from spawned goroutine | Use `t.Error` + return, or restructure test |
| Intermittent test failure | Non-deterministic goroutine ordering | Use `sync.WaitGroup.Go()` or `errgroup` |

### HTTP Handler Bugs

| Symptom | Likely Cause | Fix |
|---------|-------------|-----|
| 404 on valid route | Route pattern mismatch | Check `{method} /path` pattern in route registration |
| Empty response body | `w.Write` after `w.WriteHeader` | Write header AFTER body, or check write order |
| Wrong Content-Type | Missing header set | Add `w.Header().Set("Content-Type", "application/json")` |

---

## Anti-Patterns (NEVER Do)

| Anti-Pattern | Why It's Wrong | Do This Instead |
|---|---|---|
| Change random code until test passes | Masks root cause; fix may break other things | Form hypothesis, test it, iterate |
| Suppress the error (`_ = err`) | Hides the bug, doesn't fix it | Understand why the error occurs |
| Add sleep to fix race condition | Timing-dependent fix that fails under load | Use proper synchronization |
| Delete the failing test | Bug still exists, just untested | Fix the code, not the test |
| Add `//nolint` to suppress a finding | Hides the real issue | Fix the underlying cause |
| Fix the symptom, not the cause | Bug will reappear in different form | Trace to root cause in Phase 2 |

---

## Red Flags

STOP if you see any of these — you are about to violate the debugging methodology:

- **Shotgun fix**: You are changing code without stating a hypothesis first — you are guessing, not debugging
- **Skipped reproduce**: You jumped to Phase 2 (diagnose) without first confirming you can trigger the failure on demand
- **Hypothesis-free**: You have been changing things for 3+ iterations without a written hypothesis for each change
- **Symptom patch**: Your fix makes the test pass but you cannot explain WHY it was failing in one sentence
- **Debug artifact leak**: You are about to commit but haven't removed temporary `fmt.Printf` / `slog.Debug` lines
- **Wrong tool**: The failure is a compilation error or lint issue — use `build-resolver` agent, not `/debug`

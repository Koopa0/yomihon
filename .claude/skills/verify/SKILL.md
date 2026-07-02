---
name: verify
description: >-
  Full Go verification chain for this project: build → vet → modernize (Go
  1.26+) → golangci-lint full suite → unit tests → race detection → integration
  tests when Docker is available, run as a 5-step cognitive process (IDENTIFY,
  RUN, READ, VERIFY, CLAIM) with a required output format, failure handling,
  and //nolint usage rules.
when_to_use: >-
  Use before any commit, PR, or "done" claim, after implementing or modifying
  Go code, or when the user says "/verify", "run verification", "check the
  build", "run lint", or "do the tests pass". Required completion gate for
  every lifecycle tier.
disable-model-invocation: true
metadata:
  author: koopa
  version: "1.2"
  lang: go
---

# /verify — Go Verification Loop

## Current Changes
!`git diff --stat 2>/dev/null || echo "No git changes detected"`

Run the full verification chain for this Go project. **Every step must pass with zero issues** before the code is considered ready.

## Linter Suite

This project uses a strict `golangci-lint` configuration with these critical linters:

| Linter | Purpose | Category |
|--------|---------|----------|
| `staticcheck` | Extended static analysis (SA/S/ST/QF) | Correctness |
| `gosec` | Security vulnerability scan | Security |
| `errcheck` | Unchecked errors | Error handling |
| `errorlint` | Error wrapping and comparison | Error handling |
| `govet` | Official go vet | Correctness |
| `gocritic` | Code quality | Style |
| `gocyclo` | Cyclomatic complexity (max 15) | Complexity |
| `gocognit` | Cognitive complexity (max 20) | Complexity |

## Verification is a Cognitive Process, Not a Command

Follow these 5 steps. Do NOT skip any step.

### IDENTIFY — What needs verification?

Before running anything, list:
- Files you changed (from `git diff --stat`)
- Types you added or modified
- Endpoints you created or changed
- Tests you expect to exercise your changes

### RUN → READ → VERIFY → CLAIM

For each step below: run the command, READ the actual output (not just the exit code),
VERIFY it covers your changes, ONLY THEN move on.

## Execution Order

Run these commands SEQUENTIALLY. Stop at the first failure and report it.

### Step 1: Build
```bash
go build ./...
```
If this fails, the code has compilation errors. Fix them before continuing.

### Step 2: Vet
```bash
go vet ./...
```
If this fails, there are suspicious constructs. Fix them before continuing.

### Step 3: Modernize (Go 1.26+)
```bash
go fix ./...
git status --porcelain
```
`go fix` applies the modernizer suite (`new(expr)`, `min`/`max`, range-over-int,
`slices` helpers). If it rewrote files, those rewrites belong in your change —
review them, then stage them with the rest. Never discard a modernizer rewrite;
if one looks wrong, that is a finding to report, not to silently revert.

### Step 4: Lint (Full Suite)
```bash
golangci-lint run ./...
```
This runs ALL configured linters including staticcheck, gosec, errcheck.
If this fails, fix ALL issues — zero tolerance.

### Step 5: Unit Tests
```bash
go test ./...
```
If this fails, tests are broken. Fix the code or tests before continuing.

### Step 6: Race Detection
```bash
go test -race ./...
```
If this fails, there are data races. Fix concurrency issues.

### Step 7: Integration Tests (if Docker available)
```bash
if docker info >/dev/null 2>&1; then
  echo "Docker available — running integration tests..."
  go test -tags integration -race -timeout 300s ./...
else
  echo "Docker not available — skipping integration tests"
fi
```
Integration tests catch schema drift and real database behavior that unit tests miss.

## Output Format

Report results as a table:

```
| Step          | Status | Details              |
|---------------|--------|----------------------|
| go build      | PASS   |                      |
| go vet        | PASS   |                      |
| go fix        | PASS   | no rewrites          |
| golangci-lint | FAIL   | 2 errcheck issues    |
| go test       | SKIP   | blocked by lint fail |
| go test -race | SKIP   | blocked by lint fail |
| integration   | SKIP   | Docker not available  |
```

## On Failure

When a step fails:
1. Show the full error output
2. Identify the root cause
3. Fix the issue (if simple and safe)
4. Re-run **from Step 1** (not just the failed step)
5. If the fix is non-trivial or would change behavior, stop and explain

## Rules

- NEVER skip a step
- NEVER suppress lint issues with `//nolint` unless it's a verified false positive AND has a comment explaining why
- NEVER change `.golangci.yml` to make errors disappear
- Run ALL steps even if user only asks to "check the build"
- If tests were passing before your changes, they must still pass after
- `internal/db/` (sqlc generated code) is excluded from linting — this is configured in `.golangci.yml`

## //nolint Usage

If a `//nolint` directive is genuinely needed:

```go
//nolint:errcheck // Write to ResponseWriter rarely fails and we can't recover anyway
_, _ = w.Write(data)
```

Requirements:
1. MUST have a comment explaining why
2. MUST be specific (e.g., `//nolint:errcheck` not `//nolint`)
3. MUST be approved by the user

## Red Flags

STOP if you see any of these — you are about to claim verification passed without actually verifying:

- **Phantom pass**: You are about to say "all checks passed" but you haven't quoted specific output from each step
- **Ghost tests**: Test count didn't change after you added new code — your new code has no tests
- **Invisible file**: Your new/modified `.go` file doesn't appear in the linter output — it wasn't checked
- **Speed mirage**: You are claiming "done" within seconds of running `/verify` — you didn't read the output
- **Exit-code-only**: You checked `exit 0` but didn't read WHAT passed — the test binary might test zero packages
- **Stale pass**: You are citing output from a PREVIOUS run, not the run you just executed

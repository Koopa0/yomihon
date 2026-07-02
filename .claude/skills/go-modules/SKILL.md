---
name: go-modules
description: >-
  Go module management patterns — go.mod maintenance, Minimum Version
  Selection, vendoring, workspace mode (go.work), build tags, and module
  design decisions. Complements go-philosophy.md § Dependencies rule
  (approved/forbidden deps) with how-to patterns.
when_to_use: >-
  Use when managing or upgrading dependencies, using build tags for
  integration tests, setting up go.work or vendoring, or when
  go.mod/go.sum behavior is unclear. Triggers: "go.mod", "go.sum",
  "go mod tidy", "upgrade/add dependency", "build tag",
  "//go:build integration", "vendor", "go.work", "version selection".
metadata:
  author: koopa
  version: "1.0"
  lang: go
---

# Go Module Management

## go.mod Management

### What `go mod tidy` Does

```bash
go mod tidy
```

1. Adds missing module requirements (imports in code but not in go.mod)
2. Removes unused module requirements (in go.mod but not imported)
3. Updates go.sum with checksums for all dependencies

**Rule**: run `go mod tidy` after every dependency change. CI should verify
`go mod tidy` produces no diff.

### Minimum Version Selection (MVS)

Go uses MVS — unlike npm/cargo/pip, Go picks the **minimum version** that
satisfies all requirements, not the latest.

```
Module A requires pkg >= 1.2.0
Module B requires pkg >= 1.3.0
Go selects: pkg 1.3.0 (minimum that satisfies both)
npm would select: pkg 1.9.0 (latest compatible)
```

**Why this matters**:
- Builds are reproducible without a lockfile — go.mod IS the lockfile
- `go get -u` is the explicit opt-in to upgrade
- You won't get surprise breaking changes from transitive deps

### Upgrading Dependencies

```bash
# Upgrade single dependency to latest
go get github.com/jackc/pgx/v5@latest

# Upgrade to specific version
go get github.com/jackc/pgx/v5@v5.7.4

# Upgrade all direct dependencies (patch only)
go get -u=patch ./...

# Upgrade all direct dependencies (minor + patch)
go get -u ./...

# Check for available upgrades
go list -m -u all
```

**Decision**:

```
What kind of upgrade?
├─ Security fix → go get pkg@latest (upgrade immediately)
├─ Routine maintenance → go get -u=patch ./... (patch only, safer)
├─ New feature needed → go get pkg@v1.X.0 (specific version)
└─ Major version upgrade → go get pkg/v2@latest (new import path)
```

### go.sum: Checksum Database

- `go.sum` contains cryptographic checksums for all module versions
- ALWAYS commit `go.sum` — it ensures integrity
- NEVER edit `go.sum` manually — `go mod tidy` manages it
- If `go.sum` has conflicts in merge, resolve by running `go mod tidy`

## Vendoring

### Decision: When to Vendor

```
Do you need vendoring?
├─ Air-gapped CI (no internet during build)? → Yes
├─ Regulatory requirement for reproducible builds? → Yes
├─ Need to patch a dependency locally? → Yes (temporarily)
├─ Normal development with internet? → No (go.sum ensures integrity)
└─ Library (published module)? → NEVER vendor
```

### Vendor Workflow

```bash
# Create vendor directory
go mod vendor

# Build using vendor
go build -mod=vendor ./...

# Verify vendor matches go.sum
go mod verify

# CI: verify vendor is up-to-date
go mod vendor && git diff --exit-code vendor/
```

**Rules**:
- If you vendor, commit `vendor/` to version control
- Always run `go mod verify` in CI
- NEVER manually edit files in `vendor/`

## Workspace Mode (go.work)

### Decision Tree

```
Do you need go.work?
├─ Developing multiple related modules locally? → Yes
│   Example: main app + shared library, both in active development
├─ Monorepo with multiple go.mod files? → Yes
│   Example: services/api/go.mod + services/worker/go.mod
├─ Single module project? → No
├─ Library published to proxy? → NEVER commit go.work
└─ Temporary local development? → go.work + .gitignore
```

### Setup

```bash
# Create workspace
go work init ./app ./lib

# Add another module
go work use ./services/worker
```

```
# go.work
go 1.24

use (
    ./app
    ./lib
)
```

### Rules

- `go.work` overrides `go.mod` `require` directives for local modules
- NEVER commit `go.work` for published libraries — it breaks consumers
- For applications: commit `go.work` if your team uses monorepo structure
- Add `go.work.sum` to `.gitignore` (or commit it for reproducibility)

## Build Tags

### Integration Tests

```go
//go:build integration

package order_test

func TestStore_Integration(t *testing.T) {
    // requires Docker, database, etc.
}
```

```bash
# Run only unit tests (default)
go test ./...

# Run integration tests
go test -tags=integration ./...

# Run both
go test -tags=integration ./...
```

### Platform-Specific Code

```go
//go:build linux

package server

func setSocketOptions(fd int) error {
    // Linux-specific socket options
}
```

```go
//go:build !windows

package server

func signalHandler() {
    // Unix signal handling (not available on Windows)
}
```

### Build Tag Syntax (Go 1.17+)

```go
// ✅ CORRECT — Go 1.17+ syntax
//go:build integration
//go:build linux && amd64
//go:build !windows

// ❌ WRONG — old syntax (pre-1.17)
// +build integration
```

**Rule**: always use `//go:build` syntax. The old `// +build` syntax is
deprecated and will be removed.

### Custom Build Tags

```go
//go:build wireinject

// Used for code generation tools (wire, etc.)
// Only compiled when explicitly requested
```

## Module Design

### internal/ Directory Restriction

```
mymodule/
├── internal/     ← only importable by mymodule and its subpackages
│   └── auth/     ← cannot be imported by external modules
├── order/        ← importable by anyone
└── go.mod
```

The `internal/` directory is enforced by the Go toolchain, not convention.
Code in `internal/` cannot be imported by other modules.

### When to Split Into Multiple Modules

```
Are you building a library (published for others to use)?
├─ Yes → Consider splitting if:
│   - Different parts have different release cadences
│   - Users only need a subset of functionality
│   - Example: github.com/user/lib + github.com/user/lib/testing
├─ No (application) → Usually DON'T split
│   - Single go.mod for the entire application
│   - Multiple packages within the module is fine
│   - Split only if you extract a truly reusable library
└─ Monorepo with multiple services?
    → One go.mod per service + go.work for local development
```

### Major Version Modules

```
v0.x.x / v1.x.x → github.com/user/pkg
v2.x.x           → github.com/user/pkg/v2  (new import path!)
v3.x.x           → github.com/user/pkg/v3
```

```go
// go.mod
module github.com/user/pkg/v2

// Callers:
import "github.com/user/pkg/v2"
```

**Rule**: major version bump = new import path. This allows v1 and v2 to
coexist in the same binary (diamond dependency resolution).

## Anti-Patterns

### `replace` in Committed go.mod

```go
// ❌ WRONG — replace in go.mod of a published library
// breaks all consumers
replace github.com/some/dep => ../local-dep

// ✅ Acceptable — replace in application go.mod (not published)
// For temporary local development or forked dependency
replace github.com/some/dep => github.com/myorg/dep v1.2.3-patched
```

**Rule**: NEVER commit `replace` directives in library modules. For
applications, `replace` is acceptable but should be documented and temporary.

### go get Without Understanding

```bash
# ❌ WRONG — blindly upgrading everything
go get -u ./...  # upgrades ALL deps to latest minor — may break things

# ✅ CORRECT — upgrade deliberately
go get -u=patch ./...           # patches only, safest
go get github.com/specific/dep # upgrade one dep you understand
```

### Vendoring Without Verification

```bash
# ❌ WRONG — vendor without CI check
go mod vendor
git add vendor/
# vendor could be stale if someone forgot to re-vendor

# ✅ CORRECT — CI verifies vendor is fresh
go mod vendor && git diff --exit-code vendor/
```

### Importing test-only Dependencies in Main Code

```go
// ❌ WRONG — go-cmp imported in production code
import "github.com/google/go-cmp/cmp"

func compareOrders(a, b Order) bool {
    return cmp.Equal(a, b) // test utility in production
}

// ✅ CORRECT — use go-cmp only in _test.go files
// In production, implement comparison logic directly
```

See: go-philosophy.md § Dependencies rule for approved/forbidden dependency lists.
See: go-version.md rule for Go version and build tag requirements.

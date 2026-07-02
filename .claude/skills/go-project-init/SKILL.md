---
name: go-project-init
description: >-
  Initialize a Go project with Claude Code configuration. Two modes —
  "feature" scaffolds a new feature package under internal/ (types, handler,
  store, query.sql, tests); "bootstrap" creates a complete .claude/ config
  for a NEW project based on go-spec patterns.
when_to_use: >-
  Use when starting a new Go project, bootstrapping a .claude/ configuration
  for another repo, or scaffolding a new feature package in this project.
  Trigger phrases: "new project", "init project", "bootstrap", "set up
  claude config", "add a feature package", "/go-project-init feature",
  "/go-project-init bootstrap".
metadata:
  author: koopa
  version: "2.0"
  lang: go
---

# Go Project Initialization

Two modes:

## Mode 1: `/go-project-init feature` — Add a Feature (Existing Project)

Use this to scaffold a new feature package in the go-spec project.

### Step 1: Define the Feature

Ask the user:
- What is the feature name? (singular, lowercase: `order`, `user`, `auth`)
- What are the core types?
- Does it need HTTP handlers?
- Does it need database storage?
- Does it need Genkit AI flows?
- Does it need integration tests?

### Step 2: Create Package Structure

```bash
mkdir -p internal/<feature>
```

### Step 3: Create Files

Create these files based on answers:

| File | When | Contains |
|------|------|----------|
| `internal/<feature>/<feature>.go` | Always | Types, sentinel errors |
| `internal/<feature>/handler.go` | If HTTP | Handler closures |
| `internal/<feature>/store.go` | If database | Store with `db.DBTX` |
| `internal/<feature>/query.sql` | If database | sqlc queries |
| `internal/<feature>/flow.go` | If AI | Genkit flow definitions |
| `internal/<feature>/tool.go` | If AI | Genkit tool definitions |
| `internal/<feature>/<feature>_test.go` | Always | Tests |
| `migrations/NNN_create_<feature>s.up.sql` | If database | CREATE TABLE |
| `migrations/NNN_create_<feature>s.down.sql` | If database | DROP TABLE |

### Step 4: Types File Template

```go
package <feature>

import (
    "errors"
    "time"
)

// Sentinel errors — only define what the handler branches on.
var (
    ErrNotFound = errors.New("not found")
    ErrConflict = errors.New("conflict")
)

// <Feature> represents a <description>.
type <Feature> struct {
    ID        string    `json:"id"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

### Step 5: Store Template

```go
package <feature>

import (
    "context"
    "errors"
    "fmt"

    "github.com/jackc/pgx/v5"

    "<module>/internal/db"
)

// Store handles <feature> database operations.
type Store struct {
    dbtx db.DBTX
}

// NewStore returns a Store backed by the given database connection.
func NewStore(dbtx db.DBTX) *Store {
    return &Store{dbtx: dbtx}
}

// <Feature> returns a single <feature> by ID.
func (s *Store) <Feature>(ctx context.Context, id string) (*<Feature>, error) {
    // TODO: implement with sqlc-generated query
    return nil, fmt.Errorf("not implemented")
}
```

### Step 6: Handler Template

```go
package <feature>

import (
    "log/slog"
    "net/http"
)

func handle<Feature>s(store *Store, log *slog.Logger) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // TODO: implement
    }
}

func handle<Feature>(store *Store, log *slog.Logger) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        id := r.PathValue("id")
        _ = id
        // TODO: implement
    }
}

func handleCreate<Feature>(store *Store, log *slog.Logger) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // TODO: implement
    }
}
```

### Step 7: Register Routes and Verify

Add to `cmd/app/main.go`:

```go
<feature>Store := <feature>.NewStore(pool)
mux.HandleFunc("GET /<feature>s", <feature>.handle<Feature>s(<feature>Store, logger))
mux.HandleFunc("GET /<feature>s/{id}", <feature>.handle<Feature>(<feature>Store, logger))
mux.HandleFunc("POST /<feature>s", <feature>.handleCreate<Feature>(<feature>Store, logger))
```

Then verify:

```bash
go build ./... && go vet ./... && golangci-lint run ./...
```

---

## Mode 2: `/go-project-init bootstrap` — New Project Setup

Use this to create a complete Claude Code configuration for a **new Go project**, based on go-spec patterns. The output is a `.claude/` directory structure that the new project's Claude Code can use immediately.

### Interactive Questionnaire

Ask the user these questions in order. Each answer shapes the generated configuration.

#### Q1: Project Identity

```
What is the project name? (e.g., "order-api", "payment-service")
What does it do in one sentence?
What is the Go module path? (e.g., "github.com/org/project")
```

#### Q2: Golden Rules

Present the go-spec defaults and ask what to keep/add/remove:

```
Default golden rules (from go-spec):
1. Package-by-feature, not by layer
2. Standard library first
3. Simplicity over cleverness
4. No DDD layers
5. Errors are values

Which do you want to KEEP? (default: all)
Any rules to ADD? (e.g., "All API responses must be JSON:API format")
Any rules to REMOVE? (number)
```

#### Q3: Tech Stack

Present the go-spec defaults and ask for overrides:

```
Default stack:
- HTTP: net/http (Go 1.22+ routing)
- Database: PostgreSQL via pgx/v5
- Query gen: sqlc
- Logging: log/slog
- Testing: std testing + go-cmp
- Linting: golangci-lint v2

Override any? (e.g., "database: none", "add: Redis via go-redis")
```

#### Q4: Agent Configuration

```
Which agents do you need?
- [x] comprehend (understand before coding) — recommended
- [x] planner (design before implementing) — recommended
- [x] go-reviewer (code review) — recommended
- [x] review-code (deep paranoid review) — recommended
- [ ] db-reviewer (SQL review) — enable if using database
- [ ] security-reviewer (security review) — enable if handling auth/user data
- [ ] perf-reviewer (performance review) — enable on demand
- [x] scaffold (create feature packages) — recommended
- [x] test-writer (generate tests) — recommended
- [x] build-resolver (fix build errors) — recommended
- [ ] refactor (simplify code) — enable on demand
```

#### Q5: Acceptance Criteria

```
What are the acceptance criteria for this project?
(These will be added to CLAUDE.md so Claude Code knows when the project is "done")

Examples:
- "All endpoints return proper error responses"
- "100% of store methods have integration tests"
- "Zero golangci-lint warnings"
```

#### Q6: Hooks

```
Which hooks do you want?
- [x] check-anti-patterns (block forbidden directories) — recommended
- [x] check-generated-code (block edits to generated code) — recommended if using sqlc
- [x] format-go (auto-format on save) — recommended
- [x] verify-commit-message (validate commit format) — recommended
```

### Generated Output

The harness is NOT just `.claude/` — its mechanical gates live at the repo
root (`.golangci.yml`, `Makefile`, `tests/`, `.github/`). Copy ALL of these,
or the consumer gets the docs without the enforcement.

```
.claude/
├── settings.json          ← permissions + hooks registration + skillOverrides
│                             (name-only for deep refs; see go-spec settings.json)
├── QUICKSTART.md          ← decision tree (adapted from go-spec)
├── rules/                 ← MUST/NEVER rules (path-scoped via paths: frontmatter)
│   └── ... (go-philosophy, naming, error-handling, interfaces, testing,
│           concurrency, go-version, package-organization, project-structure,
│           development-lifecycle, git-workflow, agents, ...)
├── agents/                ← only agents selected in Q4 (carry skills: preload)
├── skills/                ← skills relevant to Q3 tech stack
├── hooks/                 ← hooks selected in Q6, incl.:
│   ├── parse-hook-input.sh
│   ├── session-start.sh           ← warns on non-v2 lint / missing benchstat,govulncheck / go<1.25
│   ├── log-instructions-loaded.sh ← InstructionsLoaded audit → .claude/rule-load.log
│   └── ...
└── agent-memory/          ← directories for memory-enabled agents

# Repo root — REQUIRED for the mechanical gates (do not skip):
.golangci.yml              ← v2: depguard/forbidigo, audited linters, formatters
                             (swap goimports local-prefixes to {{MODULE_PATH}})
Makefile                   ← test -race -shuffle, lint version-assert + config verify,
                             vuln, bench/bench-baseline/bench-compare, verify-spec
tests/
├── test-hooks.sh          ← mutation-proof hook tests
├── test-skill-format.sh   ← frontmatter, router sync, citations, listing budget
├── test-consistency.sh    ← rule consistency
├── test-skill-triggering.sh ← advisory skill-trigger eval (not in CI)
└── test-rule-compliance.sh  ← advisory rule-compliance probe (not in CI)
.github/workflows/verify-spec.yml ← runs make verify-spec on PR/push
.gitignore                 ← incl. .claude/rule-load.log, .claude/session-learnings.log

CLAUDE.md                  ← project root, includes Q1 identity + Q5 acceptance criteria
```

### Template Markers

Files that need project-specific content use these markers:

| Marker | Replaced With |
|--------|---------------|
| `{{PROJECT_NAME}}` | Project name from Q1 |
| `{{PROJECT_DESCRIPTION}}` | One-sentence description from Q1 |
| `{{MODULE_PATH}}` | Go module path from Q1 |
| `{{GOLDEN_RULES}}` | Formatted rules from Q2 |
| `{{TECH_STACK}}` | Formatted stack from Q3 |
| `{{ACCEPTANCE_CRITERIA}}` | Formatted criteria from Q5 |

### CLAUDE.md Template

```markdown
# {{PROJECT_NAME}} — Project Memory

## Purpose

{{PROJECT_DESCRIPTION}}

## Tech Stack

{{TECH_STACK}}

## Core Principles

{{GOLDEN_RULES}}

## Project Layout

```
cmd/app/          → Entry point, wiring only
internal/         → All application code, organized by feature
  <feature>/      → <feature>.go, handler.go, store.go, query.sql, <feature>_test.go
migrations/       → Numbered SQL (if database)
sqlc.yaml         → sqlc configuration (if database)
```

## Acceptance Criteria

{{ACCEPTANCE_CRITERIA}}

## Available Agents

| Agent | Model | Memory | Purpose |
|-------|-------|--------|---------|
{{AGENT_TABLE}}

## Available Skills

| Skill | Command | Purpose |
|-------|---------|---------|
{{SKILL_TABLE}}

## Verification Workflow

Before any commit or PR, run `/verify`, or `make verify-spec`, or:
```bash
go build ./... && go vet ./... && golangci-lint run ./... && go test ./... && govulncheck ./...
```
```

### Post-Bootstrap Checklist

After generating, remind the user:

1. Review and adjust `CLAUDE.md` — it's the single source of truth
2. Review `settings.json` — adjust permissions; the `skillOverrides` block
   collapses deep-reference skills to name-only to fit the listing budget
3. **Swap the module path**: in `.golangci.yml` set `formatters.settings.goimports.local-prefixes`
   to your `{{MODULE_PATH}}` (it ships as `github.com/koopa0/go-spec`)
4. **Install the gate tools**: `golangci-lint` v2 (a stale v1 on PATH silently
   breaks the gate), `go install golang.org/x/vuln/cmd/govulncheck@latest`,
   `go install golang.org/x/perf/cmd/benchstat@latest`
5. Run `make verify-spec` to confirm the gates pass, then `/manage-spec validate`
6. Create first feature with `/go-project-init feature`

### What Bootstrap Does NOT Do

- Does NOT initialize `go.mod` (user does `go mod init`)
- Does NOT create `cmd/app/main.go` (too project-specific)
- Does NOT set up Docker (kurodo is local-only; no Docker skill in this harness)
- Copies `.github/workflows/verify-spec.yml` (generic — reads go.mod), but does
  NOT configure deploy/release CI (out of scope)

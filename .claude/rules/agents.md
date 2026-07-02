# Agent Orchestration Rules

## Auto-Delegation Triggers

Claude MUST delegate without waiting for user to ask:

| Trigger | Agent |
|---------|-------|
| New feature / non-trivial change | `comprehend` (FIRST) |
| After comprehend, user confirms | `planner` |
| After plan approved, new package | `scaffold` |
| Write/Edit to `.go` file | `go-reviewer` |
| Write/Edit to `.sql` or `*store*.go` | `db-reviewer` |
| Auth/crypto code, "security review" | `security-reviewer` |
| "performance", "slow", "optimize" | `perf-reviewer` |
| Test failure, "write tests" | `test-writer` |
| Build/vet/lint error | `build-resolver` |
| "simplify", "flatten", DDD detected | `refactor` |
| `/verify` passes, "deep review", pre-PR | `review-code` |

## Priority (When Multiple Apply)

| Scenario | Priority |
|----------|----------|
| Build error + code written | `build-resolver` first |
| Security concern | `security-reviewer` first |
| Performance concern | `go-reviewer` → `perf-reviewer` |
| SQL + Go store | `db-reviewer` (covers both) |
| New feature | `comprehend` always first |

## Review Ordering

**L1** (fast): go-reviewer, security-reviewer, db-reviewer. Parallel Agent Team when 2+ triggered.
**L2** (deep): review-code (opus). See `development-lifecycle.md` for per-tier L2 rules.

Write agents MUST run `go build ./...` after changes. See CLAUDE.md for full agent list.

# Git Workflow Rules

## Commit Message Format

```
<type>: <description>

[optional body]
```

**NEVER include `Co-Authored-By` in commit messages.** No attribution lines of any kind.

### Types

| Type | When |
|------|------|
| `feat` | New feature or capability |
| `fix` | Bug fix |
| `refactor` | Code change that neither fixes a bug nor adds a feature |
| `test` | Adding or updating tests |
| `docs` | Documentation only |
| `chore` | Build, CI, tooling changes |
| `perf` | Performance improvement |
| `checkpoint` | Auto-generated safety point from `/checkpoint` skill — not for manual use |

### Rules

- Description is lowercase, imperative mood, no period: `feat: add order creation endpoint`
- Body explains WHY, not WHAT (the diff shows what)
- One logical change per commit
- NEVER commit `.env`, credentials, or secrets
- NEVER use `--no-verify` unless explicitly asked
- NEVER force push to main/master

## Before Committing

Run verification in this order:

1. `go build ./...` — must compile
2. `go vet ./...` — must pass
3. `golangci-lint run ./...` — must pass with 0 issues
4. `go test ./...` — must pass

If any step fails, fix before committing.

## PR Workflow

1. Create branch from main: `feat/order-creation` or `fix/query-timeout`
2. Make commits following the format above
3. Run full verification (`/verify` skill)
4. Push and create PR with `gh pr create`
5. PR description must include Summary (bullet points) and Test Plan

## NEVER

- NEVER commit generated code without running `sqlc generate` first
- NEVER amend a commit after a hook failure — create a NEW commit
- NEVER `git add .` or `git add -A` — stage specific files by name
- NEVER push directly to main without PR

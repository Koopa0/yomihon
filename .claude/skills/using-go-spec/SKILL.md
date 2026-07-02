---
name: using-go-spec
description: >-
  Active skill routing table for go-spec — maps the task at hand (lifecycle
  phase, Go pattern area, project operation) to the skill or rule to read
  first, plus the Augment Context Engine vs grep search strategy. A router, not
  a reference: it tells you WHERE to look, not WHAT to do.
when_to_use: >-
  Use at session start and before writing any code — when starting an
  implementation task, planning a Tier 1/2/3 change, or unsure which go-spec
  skill, rule, or reference applies. Trigger phrases: "which skill applies",
  "where is the pattern for X", beginning any coding task in this repo.
metadata:
  author: koopa
  version: "1.0"
  lang: go
---

# Using go-spec — Skill Router

Before starting any implementation task, scan this table and read any matching skill.
Multiple skills can apply simultaneously. Read them BEFORE writing code.

## Task → Skill Routing

### Planning & Lifecycle
| If you are... | Read |
|---|---|
| Starting a non-trivial change (Tier 2/3) | `development-lifecycle.md` (always-loaded rule) |
| Executing a multi-task plan | `/execute-plan` |
| Deciding WHICH tests to write | `/test-strategy` |
| Doing test-driven development | `/tdd` |
| Debugging a runtime failure | `/debug` |
| Questioning an existing design | `/devil-advocate` |
| Reviewing package design quality | `/design-review` |

### Go Patterns (read when writing code in that area)
| If you are touching... | Read |
|---|---|
| HTTP handlers, routing, middleware | `/http-server` + `/go-middleware` |
| Error types, sentinel errors, wrapping | `/error-patterns` |
| Database queries, pgx, transactions | `/pgx-patterns` + `/sqlc-guide` |
| SQL migrations | `/migrations` + `/postgres-patterns` |
| Tests (table-driven, fixtures, golden) | `/go-testing-advanced` |
| Integration tests (PostgreSQL/NATS containers) | `/testcontainers` |
| API endpoints, pagination, error format, versioning | `/api-design` |
| Concurrent code, goroutines, channels | `/go-concurrency` |
| Interfaces, consumer-side design | `/go-interfaces` |
| Generics, type constraints | `/go-generics` |
| Server startup, shutdown, signals | `/graceful-shutdown` |
| Auth, JWT, RBAC, passwords | `/auth-patterns` |
| Config, env vars, validation | `/config-management` |
| Logging, slog, structured keys | `/go-slog` |
| Caching, Ristretto | `/ristretto` |
| NATS messaging | `/nats` |
| Genkit AI flows, tools, prompts | `/genkit-go` |
| Iterators, range-over-func | `/go-iteration` |
| Performance, allocations, pprof | `/go-performance` |
| Type design, receivers, embedding | `/go-types` |
| Doc comments | `/go-doc` |
| Modules, go.mod, build tags | `/go-modules` |
| stdlib (io, json, time, sort) | `/go-stdlib-patterns` |

### Project Operations
| If you are... | Read |
|---|---|
| Scaffolding a new feature package | `/go-project-init` |
| Adding/validating skills, rules, hooks | `/manage-spec` |
| Researching external libraries/APIs | `/research` |
| Reviewing session learnings | `/reflect` |

## Rules (always loaded, no action needed)
These are rules, not skills — they load automatically for `.go` files:
- `development-lifecycle.md` — tier selection, hypothesis, phases
- `package-organization.md` — package-by-feature, forbidden names
- `naming.md` — Go naming conventions
- `error-handling.md` — error wrapping, sentinels
- `interfaces.md` — consumer-side, no 1-impl interfaces

## Search Strategy — Augment Context Engine + Grep

You have `codebase-retrieval` (Augment Context Engine MCP). Use it proactively.

### When to use which

| Question type | Tool | Why |
|---|---|---|
| "How does feature X work?" | `codebase-retrieval` | Semantic cross-file understanding |
| "What patterns are used for Y?" | `codebase-retrieval` | Concept search across packages |
| "What would break if I change Z?" | `codebase-retrieval` | Impact analysis |
| "Where is the function that handles W?" | `codebase-retrieval` | Discovery when you don't know the file |
| "Find all uses of `pgxpool.Pool`" | `Grep` | Exact symbol, exhaustive match |
| "Find definition of `Order` struct" | `Grep` | Known identifier lookup |
| "List all files matching `*_test.go`" | `Glob` | File pattern matching |

### Two-pass protocol (for non-trivial tasks)

1. **Discover** with `codebase-retrieval` — get the semantic map, relationships, key files
2. **Verify** with `Grep/Read` — confirm symbols exist, check exact signatures, find all call sites

If Context Engine and grep disagree, **trust grep** (Context Engine has a short stale window after file changes).

### Red Flags
- You are about to explore a feature area using only grep without first asking Context Engine what's there
- You are reading 5+ files trying to understand how something works — Context Engine could have given you the map in one query
- You are guessing file paths — Context Engine knows the index

## How to Use This
1. Read the user's request
2. Scan the skill tables above for matches
3. For understanding tasks, query `codebase-retrieval` first
4. Read matching skill(s) BEFORE writing any code
5. If no skill matches, proceed with rules only

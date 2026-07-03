# kurodo

This file exists because the mistakes an LLM makes in this repo are **predictable** — not random errors, but the same errors, made again and again. What follows is not advice; it is rules. General engineering discipline (read-before-write, minimal diff, test-before-fix, one change at a time) is enforced mechanically by go-spec's rules and this repo's hooks; the full version lives in `.claude/rules/`. This file records only what is specific to this repo.

## Orientation (30-second version)

A local reading and adjudication interface for a personal knowledge vault; the shared successor to yomihon and kura. Local-only, single-user, never exposed. Reads all of `~/obsidian`, writes exactly one field (`status`). yomihon (`~/go/src/github.com/koopa0/yomihon`) and kura (`~/rust/github.com/koopa0/kura`) are frozen in service until their respective retirement gates — they are reference implementations, not dependencies.

## The four walls (violating one means stop and find Koopa, not route around it)

1. **The write face = the single frontmatter field `status`.** A transition must pass the vault-schema.toml state machine (from + owner) validation; each transition is one git commit (authored with Koopa's own git identity). Writing any other field → a new decision.
2. **Always 127.0.0.1.** The listener hardcodes loopback; only the port is configurable. The search index and all derived data never leave the machine.
3. **The single source of schema understanding = `~/obsidian/System/schemas/vault-schema.toml`.** `internal/schema` is the only package that reads it; no hardcoded second copy of an enum or state machine may appear anywhere in the repo.
4. **The renderer never "fixes" a note.** It reads fault-tolerantly and surfaces diagnostics (bad YAML, broken links, name collisions); the judge only reports — a human edits the file.

## Predictable mistakes for this repo

1. **Touching renderer / graph / search without reading `docs/vault-model.md`.** You will fall back on generic Obsidian knowledge and write the wrong wikilink resolution — this vault's dialect has a spec (kura's `graph.rs`: NFC, title is never a key, never guess on a collision). Read that document first.
2. **Copying an enum or state machine into code.** When you feel the urge to write a list such as `if status == "ready"`, the answer is always in `internal/schema`, and the source is always the toml (wall 3).
3. **Casually "fixing" a piece of bad frontmatter.** The diagnostic types are read-only. kurodo reports, kura adjudicates, a human edits the file (wall 4).
4. **Porting code from yomihon.** Correctness transfers via fixtures; the implementation is written fresh (decision D04). What you may carry over is test assertions and screenshot baselines, not the parser.
5. **Re-serializing YAML.** The status write is surgical: exactly one line changes inside the frontmatter block, and everything else stays byte-identical. Any yaml marshal round-trip will destroy the file's formatting.
6. **Inventing a second way to be correct.** JSONL fields, the fingerprint (FNV-1a + `0x1f`), ordering, exit codes, scan boundaries — all target byte-compatibility with kura (`docs/spec.md` §5). "Improving" it = breaking four pipelines.
7. **Adding a dependency or a framework.** Alpine was already removed once in yomihon M2; htmx waits until a real partial-update need appears; vector search goes through the three kura-field-log gates (D05). The "do not introduce" list in `docs/design.md` §2 is the law of the yard. The pgx/sqlc/testcontainers clauses in `.claude/rules/` are go-spec shared text and do not apply — kurodo has no database (D24).
8. **Erecting your own milestone fences.** There is no M1 / M2 (D15). The spec and acceptance criteria live in `docs/spec.md`; implementation order is decided by the pain felt in use.

## Facts

- Stack: Go 1.26 / templ / Tailwind v4 (standalone CLI, no Node) / goldmark. **No database** — the search index is in-memory (D24); do not reintroduce PostgreSQL/pgx/sqlc.
- Module: `github.com/koopa0/kurodo`; binary `kurodo`; serve defaults to `127.0.0.1:9610`
- All derived state (the graph, the nav model, the search index) is in-memory, rebuilt from the vault by one scanner behind an `atomic.Pointer` snapshot (D25); the truth is always the vault files + git
- Generated code (`*_templ.go`) is never edited by hand

## Go standards

All Go conventions follow go-spec: `~/go/src/github.com/koopa0/go-spec`. Highlights: package-by-feature (no layered directory names like services/repository/models/handlers), testing with stdlib + go-cmp (no testify, no mock frameworks — and no testcontainers, since there is no database), golangci-lint v2 zero tolerance.

## Required reading (in order)

1. `docs/vault-model.md` — required before touching renderer / graph / search
2. `docs/spec.md` — goals, final feature spec, acceptance criteria
3. `docs/design.md` — architecture, data flow, retirement gates
4. `docs/decisions.md` — the decision log (why it is the way it is)

## Reference implementations (reference, don't port code; correctness transfers via fixtures, see D04)

| Scope                                                   | Location                                                                                                           |
| ------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------ |
| Reference implementation for Obsidian dialect rendering | `~/go/src/github.com/koopa0/yomihon/internal/markdown/parser.go`                                                   |
| Reference spec for wikilink resolution semantics        | `~/rust/github.com/koopa0/kura/src/graph.rs`, `src/wikilink.rs`                                                    |
| An existing markdown component                          | `~/koopa0.dev/frontend/src/app/core/services/markdown.service.ts` (assumes an untrusted body; a different context) |
| templ UI blocks                                         | `~/go/src/github.com/koopa0/goilerplate/blocks/` (take only the UI blocks, not its layered structure)              |
| Reading-interface style reference                       | `~/Downloads/tailwind-plus-syntax/syntax-ts/`                                                                      |

## Harness (synced from the go-spec bootstrap on 2026-07-02)

Rules live in `.claude/rules/` (path-scoped); the decision tree is in `.claude/QUICKSTART.md`; hooks are registered in `.claude/settings.json` (layered-directory blocking, generated-code blocking, auto-formatting, commit-message validation, and so on). Verification gates: `make verify` (fmt → vet → lint → test → build) and `make verify-spec` (harness self-check).

## Available Agents

| Agent            | Purpose                                      |
| ---------------- | -------------------------------------------- |
| `comprehend`     | Understand the codebase before starting work |
| `planner`        | Design a plan before implementing            |
| `scaffold`       | Build a feature package skeleton             |
| `go-reviewer`    | Go code review                               |
| `review-code`    | Deep, paranoid review                        |
| `db-reviewer`    | SQL / schema review                          |
| `test-writer`    | Test generation                              |
| `build-resolver` | Fix build / lint errors                      |

## Available Skills

Skills synced from go-spec (see `.claude/skills/` for the list): http-server / testing / debug / lifecycle / verify / checkpoint, and others. Some don't apply (genkit, nats, auth, docker, otel, ristretto, api-design were dropped; the pgx/sqlc/postgres/testcontainers skills remain on disk as shared reference but kurodo has no database — D24).

One skill is **project-local**, not from the go-spec sync: `native-web-first` (HTML-first / CSS-first / Baseline-Web-API discipline for the server-rendered templ + vanilla-JS UI; authored for the 2026-07-03 reading-page redesign, D26–D28). A genericized copy is upstreamed to go-spec, but this repo's copy carries the kurodo-specific pins (the four walls, D27 zero-JS write path, D28 English chrome, the design bundle as sole UI reference).

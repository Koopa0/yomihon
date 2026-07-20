# yomihon

This file exists because the mistakes an LLM makes in this repo are
**predictable** — not random errors, but the same errors, made again and again.
What follows is not advice; it is rules. The tracked
`ENGINEERING_STANDARD.md` and `PROJECT_PROFILE.md` own the general engineering
and acceptance contract. Optional maintainer-local go-spec rules and hooks may
accelerate that contract, but are not clean-clone authority or evidence. This
file records only what is specific to this repo.

## Orientation (30-second version)

A local reading and adjudication interface for a personal knowledge vault. Local-only, single-user, never exposed. Reads all of `~/obsidian`, writes exactly one field (`status`).

## The four walls (violating one means stop and find Koopa, not route around it)

1. **The write face = the single frontmatter field `status`.** A transition must pass the vault-schema.toml state machine (from + owner) validation; each transition is one git commit (authored with Koopa's own git identity). Writing any other field → a new decision.
2. **Always 127.0.0.1.** The listener hardcodes loopback; only the port is configurable. yomihon never serves or exposes the vault or any derived data beyond the machine. The three authorized outbound exceptions are: (D32, Koopa 2026-07-05) instance note content allowed by the contract privacy policy — never a `[privacy].never_egress_dirs` path (D18), never non-instance artifacts (D47) — sent to the embedding API to compute search vectors, which are stored locally; (D50.1, Koopa 2026-07-12) the query text of an explicitly requested semantic search, sent at most once per explicit action when semantic retrieval is applicable, which never enters yomihon's logs, caches, errors, metrics, or traces; and (D57, 2026-07-16 under Koopa's delegated product authority) an explicit developer certification action may send only the fixed, repo-owned synthetic protocol probes and synthetic eval corpus/queries, never arbitrary input or vault bytes. Any other egress is a new decision.
3. **The single source of schema understanding = `~/obsidian/System/schemas/vault-schema.toml`.** `internal/schema` is the only package that reads it; no hardcoded second copy of an enum or state machine may appear anywhere in the repo.
4. **The renderer never "fixes" a note.** It reads fault-tolerantly and surfaces diagnostics (bad YAML, broken links, name collisions); the judge only reports — a human edits the file.

## Review your own work before you ask anyone to review it

A bot review, a CI run, and Koopa's acceptance are **backstops**. They are not your quality process. Handing them unfinished work and letting them find your bugs is the failure this section exists to stop — and it announces itself the same way every time: the same twenty lines come back with a new finding, round after round, and each fix opens the next hole.

**Read `.claude/skills/self-review/SKILL.md` before you push, before you open a PR, before you ask for a bot review, before you say "done" — and again after every fix you make in response to a review finding.** It carries the three passes (enumerate the failure space, make every check prove it can fail, reconcile every artifact with the code), the hard gates, and a register of the mistakes actually made in this repo, each with the signature that reveals it. Add a row when you make a new one.

The four that recur most:

1. **Enumerate before you patch.** Name the complete set of ways the thing can be entered, bypassed, or left early. A `ResponseWriter` wrapper has one such set (`WriteHeader`, `Write`, `ReadFrom`, `Flush`, `Hijack`, and the handler that writes nothing). If you cannot name the whole set you are guessing, and a reviewer will enumerate it for you, one item per round.
2. **A test you have not watched fail is not a lock.** A mutation that only breaks the build proves nothing; a mutation that never applied proves nothing; a test that passes for the wrong reason is worse than none.
3. **A fix that closes one hole must be checked for the hole it opens.** Ask what your change makes newly reachable.
4. **Never push on a red gate.** `make verify && git commit`, never `;`.

When a reviewer does find something real, that is a gift — and also a receipt showing what you skipped.

## Predictable mistakes for this repo

1. **Touching renderer / graph / search without reading `docs/vault-model.md`.** You will fall back on generic Obsidian knowledge and write the wrong wikilink resolution — this vault's dialect has a spec (NFC, title is never a key, never guess on a collision), pinned by this repo's golden files and fixtures. Read that document first.
2. **Copying an enum or state machine into code.** When you feel the urge to write a list such as `if status == "ready"`, the answer is always in `internal/schema`, and the source is always the toml (wall 3).
3. **Casually "fixing" a piece of bad frontmatter.** The diagnostic types are read-only. yomihon reports, a human edits the file (wall 4).
4. **Re-serializing YAML.** The status write is surgical: exactly one line changes inside the frontmatter block, and everything else stays byte-identical. Any yaml marshal round-trip will destroy the file's formatting.
5. **Inventing a second way to be correct.** JSONL fields, the fingerprint (FNV-1a + `0x1f`), ordering, exit codes, scan boundaries — all target the frozen byte format this repo's golden files pin (`docs/spec.md` §5). "Improving" it = breaking four pipelines.
6. **Adding a dependency or a framework.** htmx waits until a real partial-update need appears. The "do not introduce" list in `docs/design.md` §2 is the law of the yard; databases and vector stores are per-feature engineering calls with escalation ladders in `docs/roadmap.md` §4 (D31/D32). D32's one embedded SQLite generation store is the current exception: it uses feature-local sqlc plus `database/sql`/modernc and real temporary SQLite files in tests. The shared pgx/testcontainers/migration clauses do not apply to that disposable embedded store; they apply if the PostgreSQL rung later opens.
7. **Erecting your own milestone fences.** There is no M1 / M2 (D15). The spec and acceptance criteria live in `docs/spec.md`; every remaining face ships (Koopa, 2026-07-05) and the sequencing view is `docs/roadmap.md` — order by dependency and leverage, still no fences.

## Facts

- Stack: Go 1.26 / templ / Tailwind v4 standalone CLI / goldmark. Node is development-only for locked frontend lint and browser probes; it is not a product-build or runtime dependency. Daily reading/UI state is in-memory (D24/D25); optional CLI semantic search owns one local SQLite generation store plus a per-command RAM vector index (D32/D50). Further database adoption remains a per-feature call (D31), with measured ladders in `docs/roadmap.md` §4.
- Module: `github.com/koopa0/yomihon`; binary `yomihon`; serve defaults to `127.0.0.1:9610`
- The server's graph, nav model, and lexical search index are in-memory, rebuilt from the vault by one scanner behind an `atomic.Pointer` snapshot (D25). The separate semantic CLI generation is disposable local SQLite derived state; the truth remains vault files + git.
- Generated code (`*_templ.go`) is never edited by hand

## Go standards

Go follows `ENGINEERING_STANDARD.md` §11 and the exact authority order in §3.
An optional checkout at `~/go/src/github.com/koopa0/go-spec` provides reusable
skills and stricter maintainer workflow, not a second normative standard.
Highlights: package-by-feature (no layered directory names such as
services/repository/models/handlers), testing with stdlib + go-cmp (no testify
or mock frameworks; embedded SQLite tests use real temporary files rather than
testcontainers), and golangci-lint v2 with zero tolerated findings.

## Required reading (in order)

1. `ENGINEERING_STANDARD.md` + `PROJECT_PROFILE.md` — normative standard and yomihon's resolved applicability, gates, commands, owners, and budgets
2. `docs/program.md` — delivery program, roles, remaining PR units, and where every product document lives
3. `docs/standards.md` — stricter repo-local execution protocol; it cannot weaken the standard/profile
4. `docs/product.md` — the product lens: positioning, modes, the constitutional queue, the aesthetic charter
5. `docs/vault-model.md` — required before touching renderer / graph / search
6. `docs/spec.md` — goals, final feature spec, acceptance criteria
7. `docs/design.md` — architecture, data flow, retirement gates
8. `docs/decisions.md` — the decision log (why it is the way it is)
9. `docs/roadmap.md` — the sequencing and design blueprint for everything not yet built
10. Per-face plan docs before building that face: `docs/judge-plan.md`, `docs/search-plan.md`, `docs/ux-plan.md`, and the B/H/D plan docs when they exist

## Reference implementations (reference, don't port code; correctness transfers via fixtures, see D04)

| Scope                                                   | Location                                                                                                           |
| ------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------ |
| Wikilink resolution + judge wire-format semantics       | this repo's golden files and fixtures (`internal/judge/testdata/`, `internal/graph` tests) — the goldens are the contract |
| An existing markdown component                          | `~/koopa0.dev/frontend/src/app/core/services/markdown.service.ts` (assumes an untrusted body; a different context) |
| templ UI blocks                                         | `~/go/src/github.com/koopa0/goilerplate/blocks/` (take only the UI blocks, not its layered structure)              |
| Reading-interface style reference                       | `~/Downloads/tailwind-plus-syntax/syntax-ts/`                                                                      |

## Optional maintainer harness

The ignored `.claude/`, `.agents/`, and `.codex/` trees may provide local
path-scoped rules, skills, and hooks. They are not present in a public clean
clone and therefore never support a formal PASS. The tracked canonical gate is
`make verify`; the profile classifies every additional conditional or
environment-specific stage. `make verify-spec` checks a local harness when one
is installed, but is not a product/repository conformance certificate.

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

Skills synced from go-spec (see `.claude/skills/` for the list): http-server / testing / debug / lifecycle / verify / checkpoint, and others. Some don't apply (genkit, nats, auth, docker, otel, ristretto, api-design were dropped). sqlc applies to the semantic generation store; pgx/postgres/testcontainers remain reference material only until the measured PostgreSQL rung opens.

One skill is **project-local**, not from the go-spec sync: `native-web-first` (HTML-first / CSS-first / Baseline-Web-API discipline for the server-rendered templ + vanilla-JS UI; authored for the 2026-07-03 reading-page redesign, D26–D28). A genericized copy is upstreamed to go-spec, but this repo's copy carries the yomihon-specific pins (the four walls, D27 zero-JS write path, D28 Traditional Chinese browser chrome, the design bundle as sole UI reference).

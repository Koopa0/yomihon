# Rename migration plan (kurodo → yomihon, D44)

A one-shot coordinated migration, not a global find-and-replace. This is the
executable form of D44; a guide dispatches it from here and verifies against
§Acceptance. Once executed it is history — kept as the record of how the
rename was done.

## Why this is operations, not a sed sweep

Three seams carry live consumers or security locks; each is called out below
and re-listed in §Landmines so nobody trips.

## Preconditions (all must hold before starting)

- The experience batch is merged (done, PR #27) — the rename touches every
  import path, so no other feature branch may be open while it runs. It is a
  **solo sweep on a clean tree**, and the cleanest slot is *next*, before the
  next feature branch is born as `kurodo` and widens the sweep.
- Working tree clean; `main` synced.
- Koopa has chosen the moment (the directory and GitHub moves are his hand and
  must be coordinated with the code PR).

## Koopa's hand (external — coordinated with the code PR, not builder work)

1. **GitHub repo renames.** `Koopa0/yomihon` → `Koopa0/yomihon-dev` (the frozen
   predecessor vacates the name); `Koopa0/kurodo` → `Koopa0/yomihon`. GitHub
   keeps redirects; the module-path change in the code PR makes it
   authoritative.
2. **Local working-directory move** (between sessions, once the code PR is
   merged): `~/go/src/github.com/koopa0/kurodo` → `.../yomihon`. A directory
   move under a live session breaks tooling — do it with no session open on the
   old path.
3. **Claude Code memory-directory move** (the same moment — the hidden hole):
   `~/.claude/projects/-Users-koopa-go-src-github-com-koopa0-kurodo` →
   `~/.claude/projects/-Users-koopa-go-src-github-com-koopa0-yomihon`. The
   memory is keyed by the slugified absolute path; without this move the entire
   cross-session handoff memory and session history are orphaned (still on
   disk under the old slug, but never auto-loaded). The harness itself
   (`.claude/` hooks and settings) uses relative paths and rides the
   working-directory move for free — verified 2026-07-08; nothing inside it is
   hardcoded to the old path.
4. **Cron cutover** (operations, the kura→kurodo discipline): the four
   `~/.hermes/scripts/cron-*-wrapper.sh` switch their invocation from
   `~/go/bin/kurodo` to `~/go/bin/yomihon`, each with a `.pre-yomihon-rename`
   backup; verify each runs; delete the old `~/go/bin/kurodo` binary only after
   all four are verified.

## Builder's mechanical scope (one PR on a clean tree)

The module-path change is the spine; everything else follows from it.

1. **Module path.** `go mod edit -module github.com/koopa0/yomihon`; rewrite
   every import of `github.com/koopa0/kurodo` → `github.com/koopa0/yomihon`
   across all `*.go`; `go mod tidy`.
2. **Command directory.** `git mv cmd/kurodo cmd/yomihon` (the binary name is
   the directory's); update the Makefile — `MODULE`, `bin/kurodo` → `bin/yomihon`,
   `./cmd/kurodo` → `./cmd/yomihon`, the `run` target.
3. **Regenerate.** `make gen` — the `*_templ.go` files carry import paths and
   regenerate; commit them. `make css` if anything CSS-adjacent moved.
4. **Environment variables** (landmine 2). `KURODO_ROOT` / `KURODO_PORT` →
   `YOMIHON_ROOT` / `YOMIHON_PORT` in the serve config (`cmd/*/main.go`).
   **Update the whitelist lock in the same commit** — the loopback-adjacent
   test that permits only those two variables (`cmd/*/main_test.go`: the
   `allowed` map and its assertion message) moves with the names, or the lock
   is asserting the wrong contract. Update every `realvault_test.go` that sets
   them, and `.github/e2e/smoke.sh` (`KURODO_ROOT` / `KURODO_PORT` /
   `KURODO_BIN` / `KURODO_SMOKE_PORT`).
5. **stderr prefix and usage.** The `"kurodo:"` error prefix and the usage
   strings in `main.go` → `"yomihon:"`.
6. **The markdown report's tool name** (landmine 1). `internal/judge/report.go`
   emits `tool: kurodo` and `# kurodo check`; change both to `yomihon` and
   regenerate `internal/judge/testdata/golden/report-md.golden` to match. This
   is a deliberate contract change — the format is ours now (D43) — but it is a
   byte-output change a live consumer reads, so the guide confirms the
   md-consuming pipeline (the vault-qa cron and any agent that greps the tool
   name) is cut over before merge. Only this one golden changes; the human and
   JSONL goldens carry no tool name.
7. **UI wordmark.** The topbar wordmark string `kurodo` → `yomihon`.
8. **Living documents.** Rewrite the product name in `spec.md`, `design.md`,
   `roadmap.md`, `standards.md`, `product.md`, `program.md`, `ux-plan.md`,
   `README.md`, and `CLAUDE.md` (its identity line, module, binary, env, and
   command references) → yomihon. **The decision log and `judge-plan.md` §12
   keep "kurodo" as historical fact** — D01–D44 are not rewritten, the same way
   kura stays named in its own history. A future reader must be able to see
   that the project was once kurodo and why it changed.

## Landmines, restated

1. **md tool name** — a live consumer reads it; regenerate the golden *and*
   confirm the consumer cutover, do not just flip the string.
2. **env whitelist** — a loopback security lock; the assertion moves with the
   variable names or it guards the wrong thing.
3. **cron binary path** — the kura-switch discipline: backup, verify each of
   the four, delete the old binary only after.

## Acceptance (the guide re-verifies independently)

- `make verify` green under the new module and binary; `make gen && make css`
  leaves the tree clean.
- `grep -rn kurodo --include='*.go'` = 0 outside historical fixtures;
  `grep -rn kurodo docs/` hits only `decisions.md` and `judge-plan.md` §12
  (history) — the living docs are clean.
- The env-whitelist test passes, now naming `YOMIHON_ROOT` / `YOMIHON_PORT`,
  and a planted stray `os.Getenv` still turns it red (the lock still locks).
- `report-md.golden` regenerated and byte-correct; the md consumer confirmed
  not to match the old tool name.
- The four crons run on `~/go/bin/yomihon` (operations, verified as the kura
  switch was).
- The memory directory is moved (a fresh session finds its handoff, not an
  empty project).

## Sequencing summary

Code PR (builder, on a clean tree) → merge → Koopa moves the working directory
and the memory directory, renames the GitHub repos, cuts the four crons over
and deletes the old binary. Feature work (Home v0.5, the content-driven
sidebar, the hover layer, B, H) resumes on the renamed base and is born as
yomihon.

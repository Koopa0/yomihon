# Rename migration plan (kurodo → yomihon, D44)

A one-shot coordinated migration, not a global find-and-replace. This is the
executable form of D44; a guide dispatches it from here and verifies against
§Acceptance. Once executed it is history — kept as the record of how the
rename was done.

## Why this is operations, not a sed sweep

Four seams carry live consumers, security locks, or two-sided contracts; each
is called out below and re-listed in §Landmines so nobody trips.

## Preconditions (all must hold before starting)

- The experience batch is merged (done, PR #27) — the rename touches every
  import path, so no other feature branch may be open while it runs. It is a
  **solo sweep on a clean tree**, and the cleanest slot is *next*, before the
  next feature branch is born as `kurodo` and widens the sweep.
- Working tree clean; `main` synced.
- Koopa has chosen the moment (the directory and GitHub moves are his hand and
  must be coordinated with the code PR).

**Position at dispatch (verified 2026-07-08, guide, adversarial round).** Koopa
moved ahead of the written sequencing: the working directory already sits at
`~/go/src/github.com/koopa0/yomihon` (the old `kurodo/` path remains as an
empty directory awaiting deletion; the frozen predecessor sits at
`yomihon-dev/`, local only). `main` is `1b5b733`, synced with origin. The tree
carries two deliberate uncommitted edits — the `go.mod` module line is
pre-flipped to `yomihon`, and `.gitignore` newly ignores the on-disk
`.agents/` and `.codex/` harness directories. The branch absorbs both: its
first commit is the `.gitignore` chore, and the module-line flip rides the
module-rename commit.

## Koopa's hand (external — coordinated with the code PR, not builder work)

1. **GitHub repo rename — one, not two.** `Koopa0/yomihon` resolves to nothing
   (verified 2026-07-08: no repo, no redirect — the frozen predecessor never
   holds the GitHub name, and `Koopa0/yomihon-dev` does not exist either), so
   the only move is `Koopa0/kurodo` → `Koopa0/yomihon`. GitHub keeps a
   redirect; the module-path change in the code PR makes it authoritative.
   Afterwards the local clone's remote should tell the truth:
   `git remote set-url origin git@github.com:Koopa0/yomihon.git`.
2. **Local working-directory move — done** (2026-07-08, ahead of the written
   sequencing; the sweep simply runs in the new directory). What remains is
   deleting the emptied `~/go/src/github.com/koopa0/kurodo/` once nothing else
   points at it.
3. **Claude Code memory-directory move** (the hidden hole, now overdue — the
   working directory has already moved):
   `~/.claude/projects/-Users-koopa-go-src-github-com-koopa0-kurodo` →
   `~/.claude/projects/-Users-koopa-go-src-github-com-koopa0-yomihon`. The
   memory is keyed by the slugified absolute path; without this move the entire
   cross-session handoff memory and session history are orphaned (still on
   disk under the old slug, but never auto-loaded). The harness itself
   (`.claude/` hooks and settings) uses relative paths and rode the
   working-directory move for free — verified 2026-07-08; nothing inside it is
   hardcoded to the old path.
   **Collision, found at dispatch:** the target slug already exists and holds
   the frozen predecessor's memory (one index entry, `yomihon-build-and-m1`),
   which sessions in this project's directory now auto-load as stale context.
   Reconcile in the same motion: move the predecessor's memory to
   `-Users-koopa-go-src-github-com-koopa0-yomihon-dev` (its project's new
   path), then move the kurodo slug's contents into the vacated yomihon slug.
4. **Cron cutover** (operations, the kura→kurodo discipline): the four
   `~/.hermes/scripts/cron-*-wrapper.sh` switch their invocation from
   `~/go/bin/kurodo` to `~/go/bin/yomihon`, each with a `.pre-yomihon-rename`
   backup; verify each runs; delete the old `~/go/bin/kurodo` binary only after
   all four are verified. (Verified 2026-07-08: exactly four wrappers name the
   binary — vault, vault-qa, translator, grinder.)
5. **The wider operations sweep** (same moment, lower stakes): the hermes
   memory (`~/.hermes/claude-memory/`) and the vault's agent guides
   (`System/agent-guides/`, `Vault-Index.md`, `Note-Schema.md`, and
   `vault-schema.toml`'s prose) name kurodo and go stale together. None of
   them matches the md report's tool name (verified 2026-07-08: nothing under
   `~/.hermes` or the vault greps for `tool: kurodo`), so landmine 1's
   consumer cutover reduces to the binary path in item 4. The legacy-named
   report file `System/reports/kura-vault-check.md` keeps its name; renaming
   it is separate vault-side work, not part of this migration.

## Builder's mechanical scope (one PR on a clean tree)

The module-path change is the spine; everything else follows from it. The
sweep includes comment prose — the acceptance grep reads comments too.

1. **Module path.** `go mod edit -module github.com/koopa0/yomihon` (already
   done in the tree); rewrite every import of `github.com/koopa0/kurodo` →
   `github.com/koopa0/yomihon` across all `*.go` and `*.templ`; `go mod tidy`.
   `.golangci.yml`'s `local-prefixes` pins the module path too — flip it in
   the same commit or the import-grouping lint asserts the wrong home.
2. **Command directory.** `git mv cmd/kurodo cmd/yomihon` (the binary name is
   the directory's); update the Makefile — `MODULE`, `bin/kurodo` → `bin/yomihon`,
   `./cmd/kurodo` → `./cmd/yomihon`, the `run` target — and the CI workflow
   (`.github/workflows/ci.yml`): the e2e step builds `./cmd/kurodo` into a
   runner-temp binary, and the header comment names the product.
3. **The client script** (found in the adversarial round). `git mv
   assets/js/kurodo.js assets/js/yomihon.js`, then every point that names it:
   the `go:embed` list and package comment (`assets/assets.go`), the asset
   registry's served name (`internal/asset/asset.go` — the URL becomes
   `/static/yomihon.js`), the asset tests, the shell's `<script src>`
   (`internal/ui/layouts/base.templ`), and the CI biome lint path.
4. **The CSS scope class** (found in the adversarial round). `.kurodo` →
   `.yomihon` across `assets/css/` (components, fonts, input, tokens) and the
   shell's `class="kurodo"` in `base.templ`. The `k-` component prefix stays:
   it is a neutral prefix, not the product name, and churning every class
   buys nothing.
5. **Client-state keys** (landmine 4). The cookie prefix is a two-sided
   contract: the client script writes `kurodo_${name}` and
   `internal/ui/pages/href.go` reads `kurodo_theme` / `kurodo_ruby` — flip
   writer and readers to `yomihon_` in the same commit. Likewise the
   sessionStorage key `kurodo.nav` → `yomihon.nav` (written by the client
   script and the sidebar's inline script, asserted in the sidebar test). The
   single user loses one cookie's worth of theme/ruby/sidebar state, once.
6. **Regenerate.** `make gen` — the `*_templ.go` files carry import paths, the
   class, the script src, and the wordmark; commit them. `make css` — the
   scope class lives in generated `output.css` too.
7. **Environment variables** (landmine 2). `KURODO_ROOT` / `KURODO_PORT` →
   `YOMIHON_ROOT` / `YOMIHON_PORT` in the serve config (`cmd/*/main.go`).
   **Update the whitelist lock in the same commit** — the loopback-adjacent
   test that permits only those two variables (`cmd/*/main_test.go`: the
   `allowed` map and its assertion message) moves with the names, or the lock
   is asserting the wrong contract. Update every test that sets them — the
   four `realvault_test.go` files (lesson, nav, render, search) plus
   `internal/schema/schema_test.go` — and `.github/e2e/smoke.sh`
   (`KURODO_ROOT` / `KURODO_PORT` / `KURODO_BIN` / `KURODO_SMOKE_PORT`).
8. **stderr prefix and usage.** The `"kurodo:"` error prefix and the usage
   strings in `main.go` → `"yomihon:"`.
9. **The markdown report's tool name** (landmine 1). `internal/judge/report.go`
   emits `tool: kurodo` and `# kurodo check`; change both to `yomihon` and
   regenerate `internal/judge/testdata/golden/report-md.golden` to match. This
   is a deliberate contract change — the format is ours now (D43). The guide
   verified at dispatch (2026-07-08) that no consumer matches the tool name —
   the vault-qa cron overwrites the report file wholesale and takes its ledger
   line from the human format — so the cutover concern is the binary path
   (Koopa's hand, item 4); re-check at merge. Only this one golden changes;
   the human and JSONL goldens carry no tool name.
10. **UI wordmark.** The topbar wordmark string and the document-title suffix
    (`— kurodo`) in `base.templ` → `yomihon`.
11. **Living documents.** Rewrite the product name in `spec.md`, `design.md`,
    `roadmap.md`, `standards.md`, `product.md`, `program.md`, `ux-plan.md`,
    `vault-model.md`, `search-plan.md`, `judge-plan.md` (outside §12),
    `README.md` (including its 蔵人 etymology block, which becomes a past-name
    note or moves to the decision log's telling), `.gitignore`'s comment line,
    and `CLAUDE.md` (its identity line, module, binary, env, and command
    references) → yomihon. **History keeps its name**: the decision log,
    `judge-plan.md` §12, this plan, the two exchange records
    (`claude-design-brief.md`, `obsidian-cc-questions.md`), and the frozen
    design bundle (`docs/design/`) are not rewritten — D01–D44 stay as
    written, the same way kura stays named in its own history. A future reader
    must be able to see that the project was once kurodo and why it changed.

## Landmines, restated

1. **md tool name** — a live pipeline overwrites the report file; regenerate
   the golden *and* re-confirm at merge that nothing matches the old tool name
   (at dispatch, nothing did), do not just flip the string.
2. **env whitelist** — a loopback security lock; the assertion moves with the
   variable names or it guards the wrong thing.
3. **cron binary path** — the kura-switch discipline: backup, verify each of
   the four, delete the old binary only after.
4. **client-state keys** — the cookie prefix and the sessionStorage key are a
   contract between the client script and the Go server; flip both sides in
   one commit or theme and ruby state silently stop round-tripping.

## Acceptance (the guide re-verifies independently)

- `make verify` green under the new module and binary; `make gen && make css`
  leaves the tree clean.
- `git grep -i kurodo` = 0 across tracked files outside the history carve-out —
  `docs/decisions.md`, `docs/judge-plan.md` §12, this plan, the two exchange
  records (`docs/claude-design-brief.md`, `docs/obsidian-cc-questions.md`),
  and the frozen design bundle (`docs/design/`). Everything else — Go, templ,
  generated files, assets, CI, Makefile, `.gitignore`, README, the living
  docs — is clean. (The original criterion greped only `*.go` and `docs/`,
  which would have let the client script, the CSS scope class, and the CI
  paths ship half-renamed, and named a carve-out list this plan itself
  violates.)
- The env-whitelist test passes, now naming `YOMIHON_ROOT` / `YOMIHON_PORT`,
  and a planted stray `os.Getenv` still turns it red (the lock still locks).
- `report-md.golden` regenerated and byte-correct; nothing hermes- or
  vault-side matches the old tool name (verified at dispatch, re-checked at
  merge).
- The served surface still works against the real vault: a page renders, the
  renamed client script loads at `/static/yomihon.js`, and the theme cookie
  round-trips under its new name.
- The four crons run on `~/go/bin/yomihon` (operations, verified as the kura
  switch was).
- The memory directory is moved and the predecessor's memory reconciled to
  its own slug (a fresh session in this directory finds this project's
  handoff, not the frozen predecessor's notes).

## Sequencing summary

The working directory has already moved (2026-07-08). What remains: code PR
(builder, in the moved directory) → independent acceptance → merge → Koopa
renames the GitHub repo and points the local remote at it, reconciles and
moves the memory directories, cuts the four crons over, deletes the old
binary and the emptied old directory. Feature work (Home v0.5, the
content-driven sidebar, the hover layer, B, H) resumes on the renamed base
and is born as yomihon.

# Decision Log

> Each entry records "the decision + why." Overturning any one is fine, but do it by opening a new decision with Koopa, not by routing around it in code.

## D01 Naming: kurodo (蔵人)

Naming took three steps: fuzukue (文机, discarded) → reuse yomihon (folder provisionally named yomihon-v2) → **settle on kurodo** (2026-07-02, Koopa ruled). What triggered the last step: Koopa pointed out that the agent on the obsidian side also uses this tool as a CLI (kura's check/exists/coverage and later extensions) — this project is the **shared successor to two tools, yomihon and kura**, and the name yomihon (読本, "the reading book") named only the reading half.

kurodo (蔵人, _kurōdo_): the Kurōdo-dokoro was the emperor's secretariat — it kept the document store, read documents for the sovereign, relayed rulings, and controlled what came and went. The authority to rule rested with the sovereign, the execution with the kurodo — exactly the shape of "only Koopa can press ready." Literally "the person of the store" (蔵の人), it continues the kura (蔵, "store") lineage directly. Binary `kurodo`, module `github.com/koopa0/kurodo`. yomihon keeps its name, frozen in service until its retirement gate — no module name collision, no v1/v2 disambiguation.

## D02 The four walls (see CLAUDE.md; the rationale is recorded here)

The dangerous axis is never "how widely it reads," but "how deeply it writes" and "how open it is to the outside." So the walls stand at: the write face (single status field + state machine + git commit), the network face (loopback hard-wired), the contract face (toml as the single source), and the honesty face (the renderer never edits notes). Inside the walls the feature space is wide open.

## D03 Scope: the whole vault + search, no feature fences

The original narrow v0 (render only Writing/Concepts, no search) was rejected by Koopa, and the reasoning holds: read-only rendering pointed at ten folders versus two is the same pipeline with a different input list; search is a real need for a 400+ and still-growing corpus. The defense against the second-system effect becomes D02's walls plus a vertical-slice shipping gate ("read through one piece, certify one lesson"), not a feature-list fence.

## D04 Don't import yomihon packages; transfer correctness via fixtures

Koopa's architectural decision: implement everything fresh, using the existing components as reference. What really needs preventing is not rewriting but the **silent drift of two implementations with the same semantics** — so the transfer medium changes from code to tests: yomihon's `testdata/lesson.md` assertion patterns, the m1-review screenshots, and kura's conformance snapshots are brought over as acceptance specs. The definition of correctness may not be reinvented. The three reference implementations and their respective roles are in the CLAUDE.md table.

## D05 Search: determinism first, upgrades go through the three gates

v0 = deterministic substring + structured filtering. The conditions for upgrading to a persistent or semantic index all go through the kura-field-log three gates (a real case + recurrence + deterministically decidable, `System/kura-field-log.md:22`). On 2026-06-30 Koopa lifted the hard ban — the door is open, but evidence is required. (The v0 mechanism is now an **in-memory** index, D24 — the original pg_trgm plan was dropped with PostgreSQL; the three-gate rule is unchanged.)

## D06 Derived data is disposable (originally "PG holds only derived data")

Source of truth = vault files + git; all derived data can be rebuilt at any time. No status history table — git log is the history; don't double-book. **Superseded in mechanism by D24**: the principle (derived data is disposable, the truth is vault + git) stands, but there is no PostgreSQL — derived state is in-memory (D25). The original text read: "The DB opens a new database on the same local PG instance as koopa0.dev, introduced only when the search face comes online."

## D07 status write = surgical single-line rewrite + shell git + Koopa's identity

Change only the single `status:` line inside the frontmatter, and never re-serialize the YAML (that would destroy formatting and comments). git goes through `os/exec`, not go-git: the audit layer must share exactly the same semantics as hand-run git, and the dependency justifies itself. A content_hash before the write guards against races. **The commit author uses Koopa's own git identity** (the vault's git config), and the message notes `(via kurodo)` — the audit meaning is "Koopa pressed it" (2026-07-02, Koopa ruled).

## D08 State-machine enforcement is kurodo's contribution, not a duplication of kura

vault-schema.toml already contains `[[lifecycle]]` (from + owner) and the slug pattern — the groundwork for "contract-first" was smaller than expected. The toml admits that a file-scan cannot validate from→to (it cannot see the prior state); kurodo is an interactive writer that can read the current state, so it naturally fills this in. **What the contract still lacks**: a renderability requirement is not in the toml — v0 does not need one (wall 4 already requires fault-tolerant rendering); if a decidable renderability contract appears later, add it to the toml, don't write it into code.

## D09 harness: from thin to full (updated 2026-07-02)

The original plan was a thin harness (a one-page CLAUDE.md + pointer). Koopa reversed it: kurodo syncs the full go-spec Claude Code configuration (bootstrap: rules / agents / hooks / skills / tests / verify-spec) — it is already a production-grade repo, the successor to two tools and a reader used daily. Drop the pieces that don't apply (genkit, nats, auth, docker, otel, ristretto, api-design; keep 8 agents). AGENTS.md stays a pointer; don't build .codex/.agents mirrors (kura's mirror carrying bad strings is the cautionary precedent). `.golangci.yml` and `.lsp.json` sync from go-spec (only the module path changes; `sqlc.yaml` was synced too but later removed with PostgreSQL — D24). goilerplate serves only as a source of UI blocks — its boilerplate is service/repository layering, contrary to go-spec doctrine, so the structure is not taken.

## D10 v0 shipping bar

"Koopa reads through a long piece inside kurodo and certifies a lesson in place." That one keypress attacks the system's real current bottleneck (adjudication friction). The growth order of the other features is decided by the pain felt in use, not scheduled in advance.

## D11 Retirement is an evidence-based gate, not a date

yomihon: five interactions + fixtures + screenshot acceptance + two weeks of real study → retire, and the SSG folds into `kurodo export`. kura: JSONL byte-for-byte golden comparison + snapshots + scan-boundary replication + four-pipeline switchover → retire. Until met: yomihon is frozen in service (bug fixes only, already tagged `v1.0.0`), and kura stands as the gate without a line changed. yomihon SPEC §13's `yomihon check` plan is retired; the lint responsibility moves to `kurodo check`.

## D12 The configuration surface is minimal

`KURODO_ROOT` (vault path, default `~/obsidian`) / `KURODO_PORT` (default 9610, a goroawase pun on ku-ro-do, no deeper meaning). There is no bind-address setting (wall 2), and — as of D24 — no database setting: the index is in-memory. A reserved-but-unused `KURODO_DB` would be expansionary; a persistence setting (e.g. `KURODO_INDEX`) is introduced only if SQLite ever actually lands.

## D13 The UI write face allows all legal transitions

Koopa ruled (2026-07-02): any from→to the toml lifecycle allows can be pressed in the UI (the same write path, the same validation + commit), with the `ready` button visually highlighted. Reason: adjudication friction drops across the board, while the write-face risk does not grow with the kind of transition — it is always the single status field (wall 1 unchanged).

## D14 kurodo is also the agents' CLI

The Claude Code on the obsidian side (and the hermes pipeline) is a direct consumer of `check` / `exists` / `coverage` — so the output formats (the JSONL contract, `--format`, exit codes) are a **public interface**, not an internal detail, and aligning with kura is a hard requirement (the retirement gate). Later extensions (backlinks, frontmatter query, MCP server…) are proposed per real vault-side usage and scheduled through the yard.

## D15 No milestone fences

Koopa ruled (2026-07-02): the spec is expressed as "goals + final feature spec + evidence gates" (`spec.md`), with no M1/M2 sequence — constraining the order in advance only ties our hands. Implementation order is decided by the pain felt in use; the retirement gates remain evidence-based (D11), independent of any schedule. The only ordering suggestion kept (not a fence): first wire up the single "finish reading → certify" keypress (D10).

## D16 A flip does not touch the `updated` field

Koopa ruled (2026-07-02, rejecting the spec's original recommendation to "sync updated"): the meaning of `updated` is **content freshness** (when the understanding was last revised), and certifying does not revise the understanding — a note finished three weeks ago and pressed ready today has a content freshness of three weeks ago, and that is the truth. Syncing updated would pollute the semantic truth-value with UI convenience, whereas stale/superseded-type views rely precisely on freshness. The home of flip visibility: git log + a pipeline.base grouped by status. Wall 1 stays annotation-free.

## D17 Dependency boundary and audit boundary (four things the 2026-07-02 spec review pinned down)

1. **The reading face does not depend on the DB**: files are the truth, PG only speeds up search; PG absent → reading works as usual, ⌘K degrades explicitly. "Reading every day" must not inherit a daemon dependency.
2. **The judge face (check/exists/coverage) = a stateless file scan**, and does not touch the DB — what the four pipelines consume must be a kura-shaped, zero-dependency binary, or the retirement gate is tightened by the back door.
3. **Wall 3's fault tolerance is asymmetric**: reads fail-open (render anyway + diagnostics), writes fail-closed (schema unavailable → no transition buttons, POST rejected).
4. **The audit boundary, stated explicitly**: `author=Koopa` is bounded by the local trust boundary; same-account local processes are cryptographically indistinguishable, so don't over-engineer with tokens — fix it through governance (agents never call the write endpoint — this goes into the vault-side agent-guides).

## D18 Privacy boundary: Diary may be rendered, but is unconditionally excluded from egress

On the vault side, `Privacy-Boundary.md` was drafted on 2026-07-02 (pending Koopa's final review): the line = the top-level `Diary/` folder (fail-closed). What this means for kurodo: local-only rendering for Koopa himself is legitimate; `export`, `check` findings (which land in reports read by agents), and every snapshot and egress path unconditionally exclude `Diary/` — even `--all` does not include it, mirroring kura. Mechanical source: once toml `[privacy] never_egress_dirs` lands, read it from the toml (wall 3), don't hardcode.

## D19 All-English project text, Google open-source standard (2026-07-02)

Koopa ruled: all of kurodo's own text is English (docs, README, UI, errors, callout default titles, commit messages), at Google open-source engineering standard; the repo stays private for now (may be open-sourced later) so no per-file license headers or contributor flow are added yet. The name kurodo and its 蔵人 etymology/soul are kept, told in English. The tool still renders the vault's Japanese/Chinese content unchanged — this ruling governs kurodo's own chrome, not the material it displays. English-only now; a future en/zh-TW/ja i18n is possible but no i18n framework is built now (convergent). Consequence: callout default titles become English (Note/Question/Example/Warning/Danger per the reading face), a deliberate divergence from yomihon (frozen, original titles) that does not affect the retirement gate (interaction fidelity, not title language). CI moves from the go-spec harness self-test (verify-spec) to a standard Go gate (build/vet/lint/test/govulncheck) — see D20 if present.

## D20 CI is a standard Go gate, not the harness self-test (2026-07-02)

The PR gate is build + vet + golangci-lint + test (+ govulncheck), gating kurodo's actual code, which is the Google-standard shape. The .claude/ harness self-test (make verify-spec) remains available for local dev but does not gate PRs — it tests the dev harness, not the product, and was failing in CI for an environment reason unrelated to the code.

## D21 Incremental indexing is a periodic mtime scan, not fsnotify (2026-07-03)

Koopa's convergence challenge, accepted. The reconciliation scan the search plan already needed (kqueue silently drops events) run at a ~2-second cadence _is_ the incremental indexer. A full mtime `stat` over ~420 files is millisecond-scale, it satisfies spec §3's freshness bound (≤3s worst case — one ~2s cadence plus the ~100 ms rebuild — stated with margin so it is stably decidable), and it handles create/delete/rename uniformly — where fsnotify on macOS is non-recursive (walk-and-watch, re-watch every new directory) and loses events. Dropping fsnotify removes a dependency and two bug classes (lost events, directory tracking) for the cost of running one scan loop a little more often. Change detection is by mtime alone (no content hash — a full rebuild is ~100 ms and hashing would force reading every file on every scan). This overrides the certified spec §3 wording "fsnotify does incremental updates" — spec §3 is updated to match.

## D22 No golang-migrate (2026-07-03; simplified by D24)

Koopa's challenge, accepted. golang-migrate exists for versioned, incremental _data_ migration, but kurodo's derived data is disposable (D06) and its "migration" semantics is drop-and-rebuild-from-the-vault, so a migration ladder is the wrong tool. This was originally resolved as "embed `001_initial.sql` + compare a schema hash." **D24 makes it fully moot**: with an in-memory index there is no schema, no SQL, no `migrations/` at all. Recorded for the future: even a persistent SQLite index (D24's upgrade path) rebuilds rather than migrates, so golang-migrate stays out.

## D23 The index holds only what search reads (2026-07-03; re-expressed for D24)

Koopa's challenge, accepted (YAGNI). None of the six filters or the substring match reads a link structure (that serves a future backlinks feature, unscheduled in design §10) or raw frontmatter/errors (results show only path + title + status + snippet). Since the index rebuilds from the vault at zero cost, those are added when a real consumer arrives, not speculatively. Under D24 (in-memory) the index per note is exactly: `rel_path`, `title`, `note_type`, `domain`, `status`, `slug`, `topics`, and the NFC-folded `plain_text` — nothing more.

## D24 Search index = in-memory, not PostgreSQL (2026-07-03)

Three engines were evaluated: **in-memory**, **SQLite**, **PostgreSQL**. v0 chooses in-memory. Reasoning: kurodo is local and single-user, and its derived state (graph, nav) is already built in memory from the vault (D25); a ~419-file substring index is a few MB and microsecond-scale to query, held as a read-only structure rebuilt from the always-present vault. PostgreSQL was dropped — a client-server RDBMS is over-reach for a local single-user tool, it ties the tool to a daemon, and its one real edge (pgvector for a future embedder) sits behind D05's evidence gate, where sqlite-vec would serve equally. This reverses the earlier PG choice (D06/spec §3/design §6); consequences: no pgx / sqlc / PostgreSQL / testcontainers / DSN, no `migrations/`, and `KURODO_DB` is removed from the config surface (D12) rather than left reserved.

**SQLite is the recorded upgrade path, not for v0, with a mechanical trigger** (so a future session does not re-litigate on feel): if a `kurodo search` CLI becomes a _frequently-invoked_ command in kura's shape — no server, a full vault scan on every invocation — the per-invocation rescan cost is the pain that opens the SQLite gate (embedded, serverless, disposable single file, `modernc.org/sqlite` pure-Go; FTS5 trigram / sqlite-vec available). Not PostgreSQL, then or ever, for this tool.

## D25 One vault Snapshot feeds graph, nav, and search (2026-07-03)

Koopa's correction, accepted — and it fixes an existing gap. Verified: today `graph.Build` and `nav.Build` run once at startup and never refresh, so editing a note leaves the sidebar and wikilink resolution stale until a restart. Rather than give search a _separate_ freshness mechanism (which would create torn states — a fresh graph against a stale nav), one scanner owns a `Snapshot{Graph, Nav, Search}` behind an `atomic.Pointer`: about every 2 seconds it `stat`-walks the vault, and on any mtime / file-set change it rebuilds all three and swaps once. Handlers read the pointer once per request. A full rebuild over ~419 files is ~100 ms and happens only on change; per-note incremental updates are unneeded complexity at this scale (reconsider past ~10k files). This is also the incremental mechanism of D21, and it closes the edit-goes-stale gap — search's infrastructure upgrades the reading face for free.

## D26 The navigation face is status-first (2026-07-03)

Koopa ruled: the left sidebar leads with a **status axis**, not the folder tree — the bottleneck kurodo attacks is adjudication friction (D10), and a `draft (2)` line puts the pending-judgment queue in the eye where the folder tree buried it. The lifecycle folder tree (spec §2 — nav order, top level ≤9) is not deleted; it drops to a **collapsed `<details>` Folders section**, so the mechanical acceptance "any vault file reachable in ≤3 clicks" still holds. The syllabus tree keeps its place. **"Reports" is returned to its spec §2 meaning — the daily-briefing HTML** (`nav.Model.Report.Briefing`); the diagnostic-count panel the design sketched under that name needs a diagnostics-index landing page that does not exist yet, so it is parked in design §10, not wired to a 404.

Grouping and labels are **schema-sourced, never hardcoded** (wall 3). One correction to the design's premise, surfaced during implementation: the toml's `status_group` is a note-**type** → group map (`note`/`system`/`lesson`), **not** the phase partition (Inbox / Growth / Authoring) the mock drew — that partition has no source in the contract. v0 therefore renders the **`note` group's ordered statuses** (the toml array order: `captured … archived`) as one flat Lifecycle list; each row carries a live count from the snapshot search index and links to the pure-filter browse page `GET /search?q=status:<name>`. The mock's phase labels are dropped as illustrative, per Koopa's own instruction. A future phase axis, if wanted, is a new `status_phase` grouping added to `vault-schema.toml` (Koopa's domain, wall 3) that the same renderer reads — it is not invented in Go. New read-only primitives only: `search.Index.CountByStatus`, and the note handler assembling the list from the status contract's ordered statuses + those counts + the current note's status (is-active). `internal/nav` stays folder/syllabus/report-only and never reads the contract (its §0.1 boundary).

## D27 The seal is progressive enhancement; the write path has zero JS dependency (2026-07-03)

Koopa ruled: pressing `ready` is 落款／鈐印 — a deliberate, koopa-only, one-way seal — and the design's press-and-hold (~430 ms) ritual is legitimate, but it may not become a JS dependency. The mechanism is unchanged to the byte: every legal transition is one `<form method="post" action="/status">`, PRG (spec §4 step 8), the error vocabulary as-is; JS only intercepts pointer / `R` / Enter to run the hold and, on completion, calls `form.requestSubmit()` — **not `fetch`** — so the server sees exactly what a no-JS submit sends. With JS off the button is a one-press submit, fully functional. Spec §4's `one form per key, no JS` is amended to **"one form per key; the write path has zero JS dependency — JS may only add ceremony on top of a working plain form"** (consistent with spec §1's vanilla-JS-only principle). This overrides the phrasing, not the algorithm.

Two consequences. (1) D13 still governs the panel: `ready` is the only primary — the seal button; **every other legal transition renders as its own quiet secondary form** (the Archive style), one press, no hold — the ritual is reserved for the sealing moment, not spread across all transitions. (2) The post-seal ink-bloom / hairline / 済 animation is driven by a **one-time signal on the PRG redirect**, implementer's choice, recorded: a `?sealed=1` query param on the `303 → /notes/<path>` target that `kurodo.js` strips via `history.replaceState` once the animation plays, so a manual refresh never replays it (no cookie, no server flash state). The `git · commit <hash>` provenance line is a **read-only** `git log -1 --format=%h -- <path>` in `internal/status` — the only package that touches git; read-only is no exception.

## D28 A bounded bilingual exception to D19: identity markers, not functional text (2026-07-03)

Koopa ruled: D19 (all-English chrome) stands for functional text, but the design's bilingual voice is a load-bearing identity element, so a bounded exception opens. The line runs between **function and ritual**. Allowed bilingual: single-character seals (済 印 振), treated as graphics — logo-level, not text; and **CJK · English paired terms** in ritual vocabulary (鈐印 · sealed by koopa) and callout labels (補足 · note) — the English must always be present and stand alone, the CJK is the identity layer. Everything functional stays pure English: full sentences (the seal poem becomes an English line, e.g. "The last step is koopa's alone. Hold — once, not withdrawn."), diagnostic and error text ("target not found"; `expected integer, got "N4"`), metrics and meta ("1,240 chars"), and every `aria-label`. This **explicitly overrides D19's callout clause** (English-only default titles Note / Question / …): callout default titles may now carry the paired CJK · English form — an override, not a route-around. Vault material — the note's own content, including a lesson's 訂正記録 H2 — is untouched; it was never chrome, and sits outside both D19 and D28.

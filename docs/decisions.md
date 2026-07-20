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

## D05 — removed (2026-07-05)

The evidence gates on search upgrades were retired by D31; semantic search is committed scope (D32). The number is kept only so older references resolve.

## D06 Derived data is disposable (originally "PG holds only derived data")

Source of truth = vault files + git; all derived data can be rebuilt at any time. No status history table — git log is the history; don't double-book. **Superseded in mechanism by D24**: the principle (derived data is disposable, the truth is vault + git) stands, but there is no PostgreSQL — derived state is in-memory (D25). The original text read: "The DB opens a new database on the same local PG instance as koopa0.dev, introduced only when the search face comes online."

## D07 status write = surgical single-line rewrite + shell git + Koopa's identity

Change only the single `status:` line inside the frontmatter, and never re-serialize the YAML (that would destroy formatting and comments). git goes through `os/exec`, not go-git: the audit layer must share exactly the same semantics as hand-run git, and the dependency justifies itself. A content_hash before the write guards against races. **The commit author uses Koopa's own git identity** (the vault's git config), and the message notes `(via kurodo)` — the audit meaning is "Koopa pressed it" (2026-07-02, Koopa ruled).

## D08 State-machine enforcement is kurodo's contribution, not a duplication of kura

vault-schema.toml already contains `[[lifecycle]]` (from + owner) and the slug pattern — the groundwork for "contract-first" was smaller than expected. The toml admits that a file-scan cannot validate from→to (it cannot see the prior state); kurodo is an interactive writer that can read the current state, so it naturally fills this in. **What the contract still lacks**: a renderability requirement is not in the toml — v0 does not need one (wall 4 already requires fault-tolerant rendering); if a decidable renderability contract appears later, add it to the toml, don't write it into code.

## D09 harness: from thin to full (updated 2026-07-02)

The original plan was a thin harness (a one-page CLAUDE.md + pointer). Koopa reversed it: kurodo syncs the full go-spec Claude Code configuration (bootstrap: rules / agents / hooks / skills / tests / verify-spec) — it is already a production-grade repo, the successor to two tools and a reader used daily. Drop the pieces that don't apply (genkit, nats, auth, docker, otel, ristretto, api-design; keep 8 agents). AGENTS.md stays a pointer; don't build .codex/.agents mirrors (kura's mirror carrying bad strings is the cautionary precedent). `.golangci.yml` and `.lsp.json` sync from go-spec (only the module path changes; `sqlc.yaml` was synced too but later removed with PostgreSQL — D24). goilerplate serves only as a source of UI blocks — its boilerplate is service/repository layering, contrary to go-spec doctrine, so the structure is not taken.

## D10 v0 shipping bar

"Koopa reads through a long piece inside kurodo and certifies a lesson in place." That one keypress attacks the system's real current bottleneck (adjudication friction). This bar was met; the remaining work's scope and ordering are governed by D37 and `roadmap.md`.

## D11 Retirement is an evidence-based gate, not a date

yomihon: five interactions + fixtures + screenshot acceptance + two weeks of real study → retire, and the SSG folds into `kurodo export`. kura: JSONL byte-for-byte golden comparison + snapshots + scan-boundary replication + four-pipeline switchover → retire. Until met: yomihon is frozen in service (bug fixes only, already tagged `v1.0.0`), and kura stands as the gate without a line changed. yomihon SPEC §13's `yomihon check` plan is retired; the lint responsibility moves to `kurodo check`. (The gate contents were later refined: D38 narrowed yomihon's gate, D40 closed it outright and added the differential campaign — judge-plan §13 — as kura's final prerequisite.)

## D12 The configuration surface is minimal (corrected 2026-07-14)

`YOMIHON_ROOT` (vault path, default `~/obsidian`) / `YOMIHON_PORT` (default
9610). There is no bind-address setting (wall 2). The daily reading and lexical
search index remains in memory under D24, so there is no database setting. A
reserved-but-unused database variable would be expansionary; the optional D32
semantic CLI owns its derived SQLite generation store without making its path
configurable. That face adds exactly one environment variable when production
dispatch lands: `YOMIHON_EMBED_KEY`, read lazily for explicit provider use and
never stored in the repo. The retired `KURODO_*` spellings were stale project-
rename residue, not alternate supported names.

## D13 The UI write face allows all legal transitions

Koopa ruled (2026-07-02): any from→to the toml lifecycle allows can be pressed in the UI (the same write path, the same validation + commit), with the `ready` button visually highlighted. Reason: adjudication friction drops across the board, while the write-face risk does not grow with the kind of transition — it is always the single status field (wall 1 unchanged).

## D14 kurodo is also the agents' CLI

The Claude Code on the obsidian side (and the hermes pipeline) is a direct consumer of `check` / `exists` / `coverage` — so the output formats (the JSONL contract, `--format`, exit codes) are a **public interface**, not an internal detail, and aligning with kura is a hard requirement (the retirement gate). Later extensions (backlinks, frontmatter query, MCP server…) are proposed per real vault-side usage and scheduled through the yard.

## D15 No milestone fences

Koopa ruled (2026-07-02): the spec is expressed as "goals + final feature spec + evidence gates" (`spec.md`), with no M1/M2 sequence — constraining the order in advance only ties our hands. The retirement gates remain evidence-based (D11), independent of any schedule. What survives of this decision is the *no fences* half: nothing is date-boxed and no sequence is a gate. The *scope* half (build only what pain demands) was superseded by D37; ordering is now by dependency and leverage (`roadmap.md` §1), still as suggestion, not fence.

## D16 A flip does not touch the `updated` field

Koopa ruled (2026-07-02, rejecting the spec's original recommendation to "sync updated"): the meaning of `updated` is **content freshness** (when the understanding was last revised), and certifying does not revise the understanding — a note finished three weeks ago and pressed ready today has a content freshness of three weeks ago, and that is the truth. Syncing updated would pollute the semantic truth-value with UI convenience, whereas stale/superseded-type views rely precisely on freshness. The home of flip visibility: git log + a pipeline.base grouped by status. Wall 1 stays annotation-free.

## D17 Dependency boundary and audit boundary (four things the 2026-07-02 spec review pinned down)

1. **The reading face does not depend on the DB**: files are the truth, PG only speeds up search; PG absent → reading works as usual, ⌘K degrades explicitly. "Reading every day" must not inherit a daemon dependency.
2. **The judge face (check/exists/coverage) = a stateless file scan**, and does not touch the DB — what the four pipelines consume must be a kura-shaped, zero-dependency binary, or the retirement gate is tightened by the back door.
3. **Wall 3's fault tolerance is asymmetric**: reads fail-open (render anyway + diagnostics), writes fail-closed (schema unavailable → no transition buttons, POST rejected).
4. **The audit boundary, stated explicitly**: `author=Koopa` is bounded by the local trust boundary; same-account local processes are cryptographically indistinguishable, so don't over-engineer with tokens — fix it through governance (agents never call the write endpoint — this goes into the vault-side agent-guides).

## D18 Privacy boundary: Diary may be rendered, but is unconditionally excluded from egress

On the vault side, `Privacy-Boundary.md` was drafted on 2026-07-02 (pending Koopa's final review): the initial line = the top-level `Diary/` folder (fail-closed). What this means for kurodo: local-only rendering for Koopa himself is legitimate; `export`, `check` findings (which land in reports read by agents), and every snapshot and egress path unconditionally exclude the contract-declared private paths — even `--all` does not include them. The toml `[privacy] never_egress_dirs` capability has since landed; `internal/schema` is the mechanical source (wall 3), and consumers do not hardcode its current directory value.

## D19 All-English project text, Google open-source standard (2026-07-02)

Koopa ruled: all of kurodo's own text is English (docs, README, UI, errors, callout default titles, commit messages), at Google open-source engineering standard; the repo stays private for now (may be open-sourced later) so no per-file license headers or contributor flow are added yet. The name kurodo and its 蔵人 etymology/soul are kept, told in English. The tool still renders the vault's Japanese/Chinese content unchanged — this ruling governs kurodo's own chrome, not the material it displays. English-only now; a future en/zh-TW/ja i18n is possible but no i18n framework is built now (convergent). Consequence: callout default titles become English (Note/Question/Example/Warning/Danger per the reading face), a deliberate divergence from yomihon (frozen, original titles) that does not affect the retirement gate (interaction fidelity, not title language). CI moves from the go-spec harness self-test (verify-spec) to a standard Go gate (build/vet/lint/test/govulncheck) — see D20 if present. **D28 now supersedes this decision for browser-facing interface text; repository prose, CLI and wire output, source identifiers, and commit messages remain English.**

*Amended 2026-07-14: Koopa committed to a future MIT-licensed public source
release. The repository remains private until that publication action occurs,
but contributor guidance, security reporting, dependency provenance, and
reproducible CI are now present-tense obligations. This supersedes only D19's
"no contributor flow yet" consequence; it does not claim that publication has
already happened or turn the personal application into a hosted service.*

*Amended 2026-07-17: the first public line is source-only `v0.x`, built with Go
1.26.5 or newer under the MIT license. Pre-1.0 Go package APIs carry no general
compatibility promise; the separately frozen agent-facing CLI and JSON wire
contracts remain compatible according to their own decisions and golden locks.
This is a release policy, not a declaration that `v0.1.0` is ready: public
publication still waits for the recorded release gates.*

*Amended again 2026-07-17: the public identity is a design artifact, not a
generated illustration or an arbitrary engineering icon. Logo and favicon work
starts from a written, product-grounded brand brief and human comparison of
directions; the selected direction is drawn as deterministic vector geometry,
with the favicon reduced from the same system and checked in monochrome, light
and dark, and at 16/24/32 CSS pixels. Image-generation output is not an
authorized source for either asset. A banner is optional and does not satisfy
the identity or real-product-screenshot release gates.*

## D20 CI is a standard Go gate, not the harness self-test (2026-07-02)

The PR gate is build + vet + golangci-lint + test (+ govulncheck), gating kurodo's actual code, which is the Google-standard shape. The .claude/ harness self-test (make verify-spec) remains available for local dev but does not gate PRs — it tests the dev harness, not the product, and was failing in CI for an environment reason unrelated to the code.

## D21 Incremental indexing is a periodic mtime scan, not fsnotify (2026-07-03)

Koopa's convergence challenge, accepted. The reconciliation scan the search plan already needed (kqueue silently drops events) run at a ~2-second cadence _is_ the incremental indexer. A full mtime `stat` over ~420 files is millisecond-scale, it satisfies spec §3's freshness bound (≤3s worst case — one ~2s cadence plus the ~100 ms rebuild — stated with margin so it is stably decidable), and it handles create/delete/rename uniformly — where fsnotify on macOS is non-recursive (walk-and-watch, re-watch every new directory) and loses events. Dropping fsnotify removes a dependency and two bug classes (lost events, directory tracking) for the cost of running one scan loop a little more often. Change detection is by mtime alone (no content hash — a full rebuild is ~100 ms and hashing would force reading every file on every scan). This overrides the certified spec §3 wording "fsnotify does incremental updates" — spec §3 is updated to match.

## D22 No golang-migrate (2026-07-03; simplified by D24; amended 2026-07-14)

Koopa's challenge, accepted. golang-migrate exists for a long-lived ladder of
in-place data migrations. Yomihon's indexes are rebuildable derived data, so
that ladder remains the wrong ownership model. D24 keeps the lexical index
entirely in memory. The later semantic generation store does use SQLite, but it
does not reverse this decision: its container schema version and its numerical
vector-format version are separate. An unknown schema or an incompatible
vector format is replaced only by an explicit build, never by an ordinary
search. Before the first *known-compatible* schema bump, that build must ship a
version-specific old-schema reader that copies still-compatible vectors into a
new file, validates the complete generation, and atomically replaces the old
file; it must not mutate the old schema in place. Version 1 has no predecessor,
so a generic copy-forward framework now would be speculative. There is no
`migrations/` ladder and golang-migrate stays out, while paid vectors are not
needlessly discarded when a future schema-only change can prove them compatible.

## D23 The index holds only what search reads (2026-07-03; re-expressed for D24)

Koopa's challenge, accepted (YAGNI). None of the six filters or the substring match reads a link structure (that serves the backlinks feature, since scheduled as the H face — D33) or raw frontmatter/errors (results show only path + title + status + snippet). Since the index rebuilds from the vault at zero cost, those are added when a real consumer arrives, not speculatively. Under D24 (in-memory) the index per note is exactly: `rel_path`, `title`, `note_type`, `domain`, `status`, `slug`, `topics`, and the NFC-folded `plain_text` — nothing more.

## D24 The lexical search index is in-memory (2026-07-03)

Three engines were evaluated: **in-memory**, **SQLite**, **PostgreSQL**. v0 chooses in-memory. Reasoning: kurodo is local and single-user, and its derived state (graph, nav) is already built in memory from the vault (D25); a ~419-file substring index is a few MB and microsecond-scale to query, held as a read-only structure rebuilt from the always-present vault. A client-server RDBMS buys nothing at this scale and ties the tool to a daemon. This reverses the earlier PG choice (D06/spec §3/design §6); consequences: no pgx / sqlc / DSN, no `migrations/`, and `KURODO_DB` is removed from the config surface (D12) rather than left reserved.

**SQLite is the recorded upgrade rung for lexical-search persistence, with a mechanical trigger** (so a future session does not re-litigate on feel): if a `kurodo search` CLI becomes a _frequently-invoked_ command with no server and a full vault scan per invocation, the per-invocation rescan cost opens the SQLite rung (embedded, serverless, disposable single file, `modernc.org/sqlite` pure-Go; FTS5 trigram available). Database adoption in general is a per-feature engineering call (D31); the escalation ladders live in `roadmap.md` §4.

## D25 One vault Snapshot feeds graph, nav, and search (2026-07-03)

Koopa's correction, accepted — and it fixes an existing gap. Verified: today `graph.Build` and `nav.Build` run once at startup and never refresh, so editing a note leaves the sidebar and wikilink resolution stale until a restart. Rather than give search a _separate_ freshness mechanism (which would create torn states — a fresh graph against a stale nav), one scanner owns a `snapshot.View{Graph, Nav, Search}` behind an `atomic.Pointer`: about every 2 seconds it `stat`-walks the vault, and on any mtime / file-set change it rebuilds all three and swaps once. Handlers read the pointer once per request. A full rebuild over ~419 files is ~100 ms and happens only on change; per-note incremental updates are unneeded complexity at this scale (reconsider past ~10k files). This is also the incremental mechanism of D21, and it closes the edit-goes-stale gap — search's infrastructure upgrades the reading face for free.

*Implementation amendment, 2026-07-18:* the load-bearing D25 decision is one
published generation, not three filesystem walks. The scanner now performs one
descriptor-rooted enumeration, captures each Markdown note and owned sidecar at
most once, builds every projection from those captured inputs, and atomically
publishes an opaque `snapshot.View`. Its projections are available only through
read-only methods; a handler calls `View.Capture` once at request entry to bind
generation data and its revocable artifact authority to one response. This
supersedes the original public-field notation and independent-walk mechanism,
which could observe three filesystem moments inside one nominal generation.

## D26 The navigation face is status-first (2026-07-03)

Koopa ruled: the left sidebar leads with a **status axis**, not the folder tree — the bottleneck kurodo attacks is adjudication friction (D10), and a `draft (2)` line puts the pending-judgment queue in the eye where the folder tree buried it. The lifecycle folder tree (spec §2 — nav order, top level ≤9) is not deleted; it drops to a **collapsed `<details>` Folders section**, so the mechanical acceptance "any vault file reachable in ≤3 clicks" still holds. The syllabus tree keeps its place. **"Reports" is returned to its spec §2 meaning — the daily-briefing HTML** (`nav.Model.Report.Briefing`); the diagnostic-count panel the design sketched under that name needs a diagnostics-index landing page that does not exist yet, so it is parked in design §10, not wired to a 404.

Grouping and labels are **schema-sourced, never hardcoded** (wall 3). One correction to the design's premise, surfaced during implementation: the toml's `status_group` is a note-**type** → group map (`note`/`system`/`lesson`), **not** the phase partition (Inbox / Growth / Authoring) the mock drew — that partition has no source in the contract. v0 therefore renders the **`note` group's ordered statuses** (the toml array order: `captured … archived`) as one flat Lifecycle list; each row carries a live count from the snapshot search index and links to the pure-filter browse page `GET /search?q=status:<name>`. The mock's phase labels are dropped as illustrative, per Koopa's own instruction. A future phase axis, if wanted, is a new `status_phase` grouping added to `vault-schema.toml` (Koopa's domain, wall 3) that the same renderer reads — it is not invented in Go. The flat v0 list is a deliberate simplification, not deferred work: it faithfully renders what `status_group` provides (type-groups, not phases), and the mock's phase grouping is an optional future enhancement gated on that toml axis — owed to no one. New read-only primitives only: `search.Index.CountByStatus`, and the note handler assembling the list from the status contract's ordered statuses + those counts + the current note's status (is-active). `internal/nav` stays folder/syllabus/report-only and never reads the contract (its §0.1 boundary).

## D27 The seal is progressive enhancement; the write path has zero JS dependency (2026-07-03)

Koopa ruled: pressing `ready` is 落款／鈐印 — a deliberate, koopa-only, one-way seal — and the design's press-and-hold (~430 ms) ritual is legitimate, but it may not become a JS dependency. The mechanism is unchanged to the byte: every legal transition is one `<form method="post" action="/status">`, PRG (spec §4 step 10), the error vocabulary as-is; JS only intercepts pointer / `R` / Enter to run the hold and, on completion, calls `form.requestSubmit()` — **not `fetch`** — so the server sees exactly what a no-JS submit sends. With JS off the button is a one-press submit, fully functional. Spec §4's `one form per key, no JS` is amended to **"one form per key; the write path has zero JS dependency — JS may only add ceremony on top of a working plain form"**. This overrides the phrasing, not the algorithm.

Two consequences. (1) D13 still governs the panel: `ready` is the only primary — the seal button; **every other legal transition renders as its own quiet secondary form** (the Archive style), one press, no hold — the ritual is reserved for the sealing moment, not spread across all transitions. (2) The post-seal ink-bloom / hairline / 済 animation is driven by a **one-time signal on the PRG redirect**, implementer's choice, recorded: a `?sealed=1` query param on the `303 → /notes/<path>` target that `kurodo.js` strips via `history.replaceState` once the animation plays, so a manual refresh never replays it (no cookie, no server flash state). The `git · commit <hash>` provenance line is a **read-only** `git log -1 --format=%h -- <path>` in `internal/status` — the only package that touches git; read-only is no exception.

## D28 Traditional Chinese is the browser interface language; content declares its own language (2026-07-03; amended 2026-07-12)

Koopa reversed D19's browser-language clause after real-page accessibility verification. Yomihon's primary browser chrome, instructional copy, browser-facing diagnostics and status messages, counts, and accessible names use Traditional Chinese; the document root therefore declares `lang="zh-Hant"`. Vault material remains authored content rather than chrome and keeps its own language. Any span or region whose language differs from its surrounding document declares that switch locally — in particular, Japanese read-aloud passages keep `lang="ja"` even though their controls and teaching guidance are Traditional Chinese. Proper names, paths, schema keys, code, and other technical tokens may remain English; use a local `lang` attribute when a natural-language switch would otherwise be ambiguous. Single-glyph seals (済 印 振) and paired ritual terms remain allowed, but English is no longer required to carry the meaning. Existing English browser strings are migration debt: do not expand them, and migrate them as a bounded interface-language unit rather than introducing an i18n framework. Repository prose, CLI and wire contracts, source identifiers, and commit messages remain English under D19.

*Amended 2026-07-17: authored note language comes only from the optional
universal frontmatter field `lang` when `vault-schema.toml` declares that key in
`fields.known`. Its value is a BCP 47 language tag. A missing declaration,
missing value, wrong type, or invalid tag renders the article as `lang="und"`;
yomihon never guesses from folder, domain, filename, or note text. Valid tags
are canonicalized for HTML, invalid declared values are check diagnostics, and
known Japanese subregions may still carry their narrower local `lang="ja"`.*

## D29 Slot-machine sidecar data lives at `System/slots/`, joined by slug (2026-07-03)

The slot machine's pattern/fill data (the hand-authored sentence frames behind feature F's slot interaction) needs a home in the vault. Three homes were weighed. **(a) Co-located** with each lesson (`Writing/lessons/japanese/L01.slots.yaml` beside `L01 〜は〜です.md`): rejected — folder-coupling to signal relatedness is exactly what the Vault-Architecture rule rejects (the slug is the link, not the directory), and it scatters a machine-owned data format through the human-authored lessons tree. **(b) A new top-level `slots/`**: rejected — the vault sits at its ≤9-top-level-folder ceiling, and a new top folder for one feature's sidecar spends the scarcest budget the vault has. **(c) `System/slots/`** (chosen): `System/` is already the machine's shelf (schemas, reports), it sits **outside the checker's scan boundary** (`scan.knowledge_dirs`) so a `.yaml` there can never be mistaken for a note and indexed, and it adds no top-level folder. The 20 files `L01–L20.yaml` are a **byte-identical copy** from yomihon's `slots/` (vault commit `f82eac6`, authored as Koopa); yomihon's copy stays frozen until its retirement gate, and the vault is now the single source of truth for slot data.

Two structural consequences. (1) **The join is by slug, never filename.** A lesson note's slug (`jp-minna-l01`) matches the `slug:` field *inside* the slot file, not the slot filename (`L01.yaml`) — lesson filenames carry the Japanese title (`L01 〜は〜です.md`) and slot filenames are `LNN.yaml`; neither is stable nor derivable from the other, so the loader indexes sidecars by their internal slug (mirroring the concept index's basename key). (2) **The slot format is kurodo-owned, not schema-governed.** Wall 3 (the single source of schema understanding is `vault-schema.toml`) governs the **note** frontmatter schema; the slot sidecar's shape (patterns / slots / fills / the closed 8-token color set) is a Go struct in `internal/lesson`, validated at load by the loader itself — it is not, and must not become, a second copy of anything in the toml. The slot loader is a **separate read path** from the note scanner: slots are never indexed as notes, never enter the D25 snapshot, and are loaded once into a slug-keyed index. Owed by this decision (landing in the same PR): repath the two doc references to the slot home — `design.md` §8 and `spec.md` §6 — from `slots/` to `System/slots/`.

## D30 The report sandbox is a three-mechanism guarantee with one decision home (2026-07-05)

The E face shipped "sandboxed report HTML" (PR #8), but the guarantee spans **three mechanisms that must all hold**: (1) the `/reports/<name>` page embeds the report in an `<iframe sandbox="allow-scripts">` (no `allow-same-origin` — the script runs in an opaque origin); (2) `/reports/<name>/raw` serves the bytes with `Content-Security-Policy: sandbox allow-scripts; frame-ancestors 'self'` plus `Cache-Control: no-store`, and only from `System/reports/`; (3) `/notes/` serves **only `.md`** — any non-note resource is 404, which closes the side door of loading a report through the notes route and executing script inside kurodo's own origin. Remove any one mechanism and the ledger line "reports are sandboxed" silently becomes false; this entry exists so a future session has one decision to consult instead of rediscovering the invariant from three scattered handlers. Authority: spec §1's report clause and D26's Reports meaning — **not** D18, which governs only Diary egress. The I-face wall sweep owes a test lock per mechanism, and the E lesson generalizes: any "sandbox this resource" feature must sweep **every route that can serve the same bytes**.

*Amended by D59 on 2026-07-18: the historical script allowance above is
superseded. The current three mechanisms are: (1) the report iframe carries a
bare `sandbox` with no capability token; (2) the byte-verbatim `/raw` response
carries a bare CSP sandbox, `script-src 'none'`, closed automatic-resource
directives, `frame-ancestors 'self'`, and `Cache-Control: no-store`; (3) every
alternative raw-file route either escapes the bytes as source or applies its
own scriptless sandbox, so no route executes vault HTML with first-party
authority. Static HTML, inline CSS/SVG, and data media remain visible; authored
scripts, event handlers, automatic refresh/navigation, forms, remote resource
loads, and WebRTC do not run. A user deliberately following a link is an
explicit navigation, not an automatic-egress guarantee. Future report
interaction requires a separate ruling and an audited first-party declarative
renderer; executable report-authored JavaScript is not a deferred toggle.*

## D31 Databases are not categorically excluded (2026-07-05; amends D24/D22, retires D05's gates)

Koopa overruled the categorical posture: "no database, do not reintroduce PostgreSQL" is not his decision and does not stand as a wall; capability is not to be artificially constrained ("不要限縮我們開發的能力"). What this changes: adopting a database is now an **engineering call made per feature**, with no evidence gate in front of it — D05's three kura-field-log gates are retired, and semantic search moves from "gated upgrade" to committed scope (D32). What stands, on engineering merit rather than dogma: at the current scale (~437 notes, ~10⁴ chunks) in-process derived state behind the D25 snapshot remains the *default shape*, because it is faster than any out-of-process store, keeps the vault as the single truth (D06), and keeps the CLI faces stateless. The recorded escalation ladders (D24's SQLite note; D32's vector rungs; D33's graph triggers) are upgrade paths with explicit triggers, so future sessions escalate by measurement, not by re-litigating a ban that no longer exists.

## D32 Semantic search is committed scope: Gemini embeddings, hybrid retrieval, in-process vectors first (2026-07-05)

The B face ships hybrid search, not just lexical. Shape:

- **Embedder = `gemini-embedding-2` over the API** (amended by D50.9; the
  superseded `gemini-embedding-001` history remains in the amendment note
  below, not as a second plan of record). The key is injected via env var and
  never stored in the repo; Koopa supplies his own paid-project key only for
  live provider use, not at builder dispatch. Compilation and offline
  verification require no key. Distribution is BYOK-only: yomihon bundles no
  credential and operates no shared proxy. Dimension is **1,536**, selected by
  the ruled paired 1,536-vs-3,072 evaluation recorded in D50's 2026-07-16
  amendment. Rationale: the vault is mixed
  繁中/日文/code-term text and this is the current multilingual embedder selected
  by D50; a local fallback (Ollama bge-m3 class) stays legal but is not the plan
  of record.
- **The egress ruling, explicit**: sending note *content* to the embedding API is authorized. This is a bounded reading of wall 2 — the wall's operative meaning is that kurodo never *serves or exposes* the vault or its derived data (loopback-only listener, no derived data leaving as an artifact); an outbound embedding call that Koopa authorizes is not a wall breach. Contract-declared private paths are unconditionally excluded from the embedding pipeline (D18, fail-closed, mechanically through `[privacy]`). Any future widening of what leaves the machine is a new decision, not an extrapolation of this one.
- **Storage**: one owner-only SQLite generation store persists reusable vectors;
  each explicit CLI action hydrates one complete active generation into an
  immutable in-memory `[]float32` exact-search index. The store retains active
  and previous immutable generations plus at most one resumable staging
  generation; activation is a catalog transaction, not a mutable-row cache
  update. It is disposable derived data, but compatible paid vectors are reused
  by exact submitted bytes. `serve` never opens either layer. At today's scale
  (~10⁴ chunks or fewer × 1536–3072 dimensions), exact cosine remains the
  baseline with 100% retrieval recall; the escalation trigger below opens a
  benchmark, not an automatic backend swap.
- **Retrieval**: heading-based chunking (reusing the judge face's goldmark extraction layer, inputs bounded to the model's token limit) + chunk vectors; exact cosine top-k fused with the lexical index via **RRF**. RRF's real knobs (the k constant, per-channel list depths, chunk→note aggregation) are pinned in the B plan doc — "no tuned weights" describes RRF's formula, not an absence of decisions. Surface: `yomihon search` is lexical by default and `--semantic` explicitly opts the CLI/agent call into hybrid retrieval, with loud failure on cold cache or an unavailable query API (`roadmap.md` §4a). The ordinary ⌘K panel, `/search` page, and live fragment are permanently lexical-only; a future human exploration feature must be a separately ruled Related/Find-related surface, never an implicit mode of ordinary search.
- **The scale trigger opens measurement, not a preselected rung**: before
  hydration, rung 1 admits fewer than 100,000 chunks and at most 1 GiB of raw
  vector payload (`chunks × dimension × 4`); crossing either deterministic
  limit, or observing p95 exact-scan above about 100 ms, benchmarks the current
  SQLite-generation/RAM-exact design against the then-current embedded-vector
  candidates and PostgreSQL exact search. PostgreSQL is adopted only for a real
  server-owned capability (shared remote access, independently operating
  writers, or database backup/replication ownership) or when the embedded
  design misses a predeclared SLO and PostgreSQL passes the complete
  correctness, privacy, resource, and latency gate. pgvector ANN is a separate
  decision, reached only if PostgreSQL exact search also misses the SLO and ANN
  passes held-out recall and filter-completeness gates. Neon is the preferred
  managed PostgreSQL candidate to measure if that evaluation opens; selecting
  a remote service still requires an explicit egress ruling for **any**
  real-vault corpus- or query-derived transfer, including vectors, query
  vectors, filters, and identifiers whether or not they are persisted.
  Synthetic-only comparison remains permissible before that ruling. Fusion and
  the agent CLI do not change across
  candidates; ordinary UI search remains lexical and outside this ladder. A
  model, dimension, or numerical protocol change is an incompatible generation
  cutover, not a store-schema migration.

*(Amended 2026-07-12 by D50: the embedder of record is now `gemini-embedding-2` — the 001 generation's retirement window opened before this plan could build, and embedding generations are incompatible, so the corpus re-embeds in full. Corrected 2026-07-13: the swap is not "a re-embed and nothing else" — the successor's request protocol differs (no task-type field, API-side normalization at truncated dimensions, different multi-input semantics) and is re-pinned from its own documentation, entering the cache identity. The representation is chunk-only: note-level vectors are dropped, returning only if the eval set shows broad-topic recall failing. Corrected again 2026-07-14 under Koopa's delegated product authority: one process owns one complete generation identity and does not keep a registry of retired embedders. An incompatible active generation is unavailable to that process until an explicit build publishes a compatible replacement.)*

*(Amended 2026-07-13 by Koopa's delegated product ruling: `serve` owns no
semantic capability at all — no provider client, key, vector store, document
builder, or query path. A cold cache or incompatible identity is built only by
the explicit `yomihon search-index build` command, so an ordinary search can
never surprise the operator with a whole-vault upload or an unbounded first
build. Once a compatible active generation exists, an explicit
`yomihon search --semantic` action automatically reconciles current vault
drift before querying: unchanged rows are reused, changed/new eligible chunks
are embedded, deletions and newly ineligible paths are removed locally, and
the query is sent only after a complete current generation is active. This is
the product convenience boundary: routine semantic search self-refreshes,
while expensive cold/identity rebuilds remain explicit. The durable cache is
one SQLite file containing isolated immutable generations; a transaction
activates a completed generation and retains the prior active generation.
No stale vector or partial semantic ranking is served.)*

## D33 Relation queries are answered by the in-process graph: H grows query verbs and a whole-graph export; no graph database now (2026-07-05)

The real requirement (Koopa, 2026-07-05): agents — hermes, Claude, the obsidian-side agent — must be able to ask *relationship* questions ("what connects to X", "how do X and Y relate", "what co-cites both") without reading the vault file by file; some of those answers are structural (graph) and unreachable by semantic similarity alone. The graph already exists in memory (D25), rebuilt every ~2s; what is missing is a **query surface**, not storage. So the H face grows:

- **Relation verbs**: `kurodo graph backlinks|neighbors|path|related <note> --json` — O(1)–O(edges) walks over the in-memory index, microseconds at this scale.
- **Whole-graph export**: `kurodo graph export --json` — the entire graph today (~450 nodes, ~2×10³ edges) is low hundreds of KB in path-keyed JSON; an agent loads it in one call and traverses arbitrarily in its own context. This is the answer to "surely they can't grep note by note" at the current scale — but it stops fitting in an LLM context around ~10⁴ notes, so the **verbs are the contract-bearing agent interface** and the export is a convenience; their JSON contracts (fields, ordering, exit codes) are pinned in the H plan doc and golden-tested (the D37 contract rule).
- **UI counterparts** (kurodo is not CLI-only): a backlinks / structural-neighbors panel in the reading page's right column (backlinks was already parked in design §10) and, later, the graph view. D32's semantic nearest results remain agent-only. A human semantic or mixed Related/Find-related surface requires its own product ruling and is not authorized by D33.

**Graph-database triggers** (any one reopens the question, and the ladder is embedded-first, server-last): the graph outgrows in-process rebuild (~10⁵ notes); multiple independent writers need transactional graph mutation (today the graph is a read-only derivative and the only writer is the vault itself); graph algorithms at a scale where in-process is infeasible; or relation queries must span corpora beyond the vault. Why not now: at 437 nodes a Neo4j/dgraph adds an ETL sync pipeline, a second stateful truth that can drift, server ops, and a network hop on every query — while adding no query capability that the export path does not already give an LLM consumer.

## D34 The agent surface is CLI-first; no MCP server (2026-07-05)

Koopa ruled CLI-only, and the assessment agrees: the cron ecosystem consumes exit codes + JSONL; agent sessions shell out; `kurodo serve` already covers the long-lived human surface. MCP would duplicate all three. The one recorded blind spot: a non-shell AI surface (claude.ai web/mobile) cannot exec a CLI — if that need materializes, the first move is a thin tool on the existing knowledge-MCP that shells to kurodo, not an MCP server inside kurodo. Fully reversible; deferred at zero cost.

## D35 Dreaming (background organizing) is an agent-side consumer; kurodo's write face does not grow (2026-07-05)

The dreaming capability — nightly analysis proposing merges, missing links, orphan adoptions, syllabus gaps, digests — lives **outside** the kurodo binary, as cron/agent pipelines composing kurodo's read-only primitives (check / coverage / exists / D32 semantic search / D33 graph verbs) plus an LLM. Proposals land as report files written by the *agent* under its own identity; kurodo renders them (E face) and continues to write exactly one field (wall 1) and fix nothing (wall 4). This is spec §0's end-state 4 realized, not a new face. A future **adjudication-inbox UI** (render proposals, record accept/reject) is legal as a reading surface; the moment its "accept" needs kurodo to persist anything beyond `status`, that is a wall-1 question to bring to Koopa explicitly — recorded here so it cannot slide in as an implementation detail.

## D36 Quality rails are committed work items, not advice (2026-07-05)

Koopa: "沒有其他我們不把事情和設計做好的理由" — the quality tooling ships as work, scheduled right after the A face (alongside the I sweep). The package: **CI** (GitHub Actions, jobs classified by code area — lint-go, test-go with `-race`, fuzz-smoke, assets-drift for templ/css regeneration diff, lint-frontend, build, e2e-smoke booting the server on a fixture vault); **linters** beyond Go — Biome for repository-owned JavaScript (the original enhancement file and the later browser probes), Stylelint for the hand-written stylesheet sources, regeneration-drift checks for every generated artifact (the output.css drift and the hand-edit hazard are both closed mechanically), a rendered-page axe-core/Nu job as a later non-blocking addition; **fuzzing** aimed where the judge build actually bled (U+2028 escaping, the empty-link panic, trailing-backslash targets, YAML coercion — every one was fuzz-reachable): `splitFrontmatter`, `parseNote`, wikilink/pathref extraction, `WriteJSONL` invariants, `stripTarget`; **synctest** for the D25 rescanner's timing logic; **benchmarks** (scan/rebuild, search query, full `kurodo check`) under benchstat discipline, smoke-only in CI; and a one-off **differential fuzz campaign** (random vaults through both engines), sequenced strictly *before* the conformance scaffolding is deleted — the sandwich generalized, run while the reference binary still exists; its honest cost is the schema-aware vault generator (near-valid frontmatter + cross-note link topology), not the runner. CI never runs the reference-binary conformance tests (no kura, no real vault in CI — by design); golden fixtures carry that weight there. CI-only linters (Biome/Stylelint/axe-core) may use Node on the runner — the Node-free stack fact is about the product build, not the CI environment.

## D37 Scope ruling: every remaining face ships; ordering by dependency and leverage; agent-facing output is a frozen contract (2026-07-05)

Koopa ruled: A/I/B/D/G/H all ship — the pain-driven rule's *scope* half (build only what pain demands) is superseded; its *no fences* half stands (D15, as rewritten). The sequencing view is `roadmap.md` §1 — a suggestion ordered by dependency and leverage, never a date or a gate.

Two riders, both born from this project's own history. (1) **The contract rule**: every agent-facing CLI output (the D33 graph verbs, `kurodo search`, frontmatter query — everything an external consumer can parse) is a public interface: its JSON field set, ordering, and exit codes are specified in the owning face's plan doc *before* build and pinned by golden tests, exactly as the judge face's bytes were. A cron already greps `"severity":"warn"` out of check's JSONL; the next consumer will grep whatever ships next. (2) **The plan-doc pattern generalizes**: a face without a spec.md section (hybrid search, the agent toolbox, the cockpit) gets a plan document before build — the judge-plan pattern — and that plan carries the face's acceptance criteria; `roadmap.md` §5a lists each plan doc's obligated contents.

## D38 The yomihon retirement gate excludes the export face (2026-07-05)

The acceptance block's placement under spec §6 (the export face) made the gate ambiguous: read literally, retiring yomihon waited for `kurodo export`, scheduled near-last. Koopa ruled on the deciding fact — **nothing consumes yomihon's SSG output today** — so the gate is exactly the three reading-face items (the five interactions + fixtures, already merged with F; `m1-review/` screenshot parity; two weeks of real studying, clock running since ~2026-07-03), and the export face ships on its own position in the sequence without blocking the retirement declaration. The temporary SSG capability gap between yomihon's retirement and G's landing is accepted, because a capability with zero consumers cannot gate anything. spec §6 now separates the gate from export's own acceptance. (Two of the three items were later waived outright — D40.)

## D39 `check` output drops every finding that touches `Diary/` (2026-07-05)

The wall-lock sweep surfaced a latent privacy leak, probe-verified on both engines: the reference scanner walks `Diary/` like any directory and emits its findings — path, line, and **content fragments** (a broken link's target text, a schema value) — and kurodo, byte-compatible, reproduced this. But the vault-qa cron writes the full check report to `System/reports/`, which agents and LLM pipelines read, so the day the diary carries its first broken link, private text flows into agent contexts. spec §5's egress-exclusion clause had asserted this could not happen ("mirroring kura") — its premise about the reference's behavior was factually wrong; both engines leaked by design.

Koopa ruled (option a): **the check output layer drops every finding any of whose touched paths (citing path or collision member) begins with `Diary/`** — unconditionally, `--all` included, fail-closed per D18. What is lost is nothing he wants: the reading page's per-note diagnostics panel still shows a diary note's broken links when he opens it himself (local rendering is not egress), and the vault currently has zero diary findings, so no live consumer sees a change. This is a deliberate divergence from the reference — the reference's behavior is the defect — recorded in the divergence register (judge-plan §12, entry 8, landing with the implementation) with a dedicated fixture pinning: a diary broken link produces a finding on the reference and none on kurodo. From this ruling on, spec §5's exclusion clause states an enforced truth instead of an unverified assumption.

## D40 Retirement is discard, and yomihon's remaining gate items are waived (2026-07-06)

Koopa ruled the end state plainly: when this project completes, both reference implementations are discarded outright — archived, unmaintained, untracked. Two consequences:

**yomihon**: of the gate D38 had narrowed to three reading-face items, the first (the five interactions independently reproduced, fixtures passing) is merged; the remaining two — screenshot parity against `m1-review/` and the two-week studying clock — are **waived** (ruled 2026-07-05, confirmed 2026-07-06). Koopa moved his daily reading to kurodo outright and does not track parity; reading-surface problems found in daily use are ordinary UX work (roadmap §5b), not gate evidence. The retirement is therefore effective on Koopa's declaration alone.

**kura**: the gate is unchanged — the differential fuzz campaign (judge-plan §13) remains the declaration's final engineering prerequisite, because four live pipelines consume the output bytes. After the declaration: the conformance, sandwich, and differential scaffolding is deleted; every golden and pinning fixture stays (they are the frozen contract); the reference-binary shell export is removed; the `.pre-kurodo-switch` wrapper backups (written beside each of the four cron wrapper scripts in `~/.hermes/scripts/` on switchover day, as the rollback path) are cleaned; the reference binaries are deleted.

After both declarations, kurodo owns every format it emits and is free to improve them: byte-compatibility was migration discipline while a second engine existed, not deference to it.

## D41 The interaction ladder: native first, justified JavaScript, mature libraries admissible on need (2026-07-06; amended 2026-07-11)

Koopa ruled the UI posture in two halves. First, the ladder stands: semantic
HTML, then CSS, then Baseline Web APIs, then a small vanilla-JS progressive
enhancement only when a concrete need remains or the script is materially
clearer than the native alternative. JavaScript is allowed, not the default;
the working no-JS core path remains. Motion and loading polish — view
transitions, deliberate animation, busy states — are **in-scope quality, not
gold-plating**; `ux-plan.md` owns that inventory. Second, the ladder's top rung
is opened: **a mature, useful library may be introduced when it genuinely earns
its place** — over-constraining and wheel-reinventing are both defects. Admission
criteria, all required: (1) a real need a reasonable vanilla implementation
cannot meet at proportionate cost; (2) maturity — stable API, maintained, widely
deployed; (3) vendorable, served from this repo (the loopback promise admits no
CDN); (4) size proportionate to the need; (5) it must not take over rendering —
the app stays a server-rendered MPA with no client-framework runtime; htmx,
Alpine, and any other client abstraction require a concrete unmet need and this
same admission discussion; (6) each admission is recorded as a decision here,
need named. mermaid is the standing precedent. This supersedes the harsher "any
library request is a stop-and-surface" phrasing and the earlier Chromium-only
reading of this decision.

## D42 The journal influences no egress verdict; a public note's own words remain its own (2026-07-06)

An external review found that D39 closed the journal's *output* but not its *influence*: a journal note still counted as a source — its outgoing edges could flip a public concept from orphan to referenced/mounted in coverage, and a planned-name list kept in the journal downgraded a public broken link from warn to info. An agent reading those reports learns nothing of the journal's text but can infer bits of it from public verdicts. Koopa delegated the ruling; decided fail-closed, consistent with D18/D39: **journal content — its links, its planned lists — exerts no influence on any egress surface's verdict about public notes.** Coverage's mount/reference edges and the planned-name set now exclude journal sources.

The boundary D39 drew stands unchanged on the other side: a public note that names a journal title wrote those words itself, so the link and its resolution remain visible properties of the public note (suppressing resolution would only invert the signal — a broken-link finding would then announce "this name lives only in the journal"). The reading page is untouched: local rendering is not egress, and the shared graph keeps the journal whole.

Consequences: a new deliberate divergence (judge-plan §12, entry 10) with fixtures pinning both exclusions; the differential harness grows the constructs and manifest-driven normalization; and the fix lands **before** the retirement campaign runs, so campaign evidence is collected against the final behavior.

## D43 kura is retired (2026-07-07, declared by Koopa)

The declaration per judge-plan §13, all five bar items met: 9,000 generated vaults across three independent runs (2026-07-06: 5,000 over bases 20260706/61803398/577215664; 2026-07-07: 2,000 at base 20260707, then 2,000 at base 314159265 on the main that had absorbed the hardening work), zero unexplained byte differences after §12 normalization, rule reach complete in every run, every register construct exercised live, nothing left unadjudicated — the campaign found no unknown divergence at all. The four pipelines have run on kurodo since 2026-07-05 without incident.

kura is discarded per D40. The scaffolding dies and the contract stays: the conformance, sandwich, and differential tests are deleted; every golden and pinning fixture remains; the reference-binary shell export is removed; the wrapper backups are cleaned; the reference binaries are deleted. The §12 register stands as the historical record of why the bytes look the way they do. From this declaration on, kurodo owns every format it emits — byte compatibility was migration discipline while a second engine existed, and the second engine no longer exists.

## D44 The project is renamed kurodo → yomihon (2026-07-08, reverses D01's name)

D01 chose kurodo (蔵人, the sovereign's secretariat) for the keep-the-store, read-for-the-sovereign, relay-rulings metaphor. Koopa reversed it: the name over-signifies and does not carry the read-book concept that is the product's felt center; the 蔵人 framing is not apt for what the thing is to its daily user. The name it takes — yomihon (読本, "the reading book") — names the surface the owner lives in. The objection that killed this name in D01 (it names only the reading half of a reader-plus-judge system) was reconsidered and set aside by the owner: reading is the lived surface, the judge and agent faces are machinery behind it, and naming is the owner's taste to rule (product.md §6).

The name is currently held by the frozen predecessor at `~/go/src/github.com/koopa0/yomihon` (gate-closed, D40). It vacates: the predecessor's local directory and GitHub repo become `yomihon-dev`; this project takes `github.com/koopa0/yomihon`. The decision log keeps "kurodo" as historical fact — D01–D43 are not rewritten, the same way kura stays named in its own history; living docs and code take the new name.

**This executes as one coordinated migration, not a global replace** — three live-consumer landmines make it operations, not a mechanical sweep:

1. The judge's markdown report emits `tool: kurodo` and `# kurodo check` (`internal/judge/report.go`), which the vault-qa cron and reading agents consume, and a golden pins it. Renaming the emitted name is a byte-output change with a live consumer — flip it in lockstep with anything that matches on it (the switchover discipline of the divergence register's preamble entry, generalized now that the format is ours, D43).
2. The environment-variable whitelist is a loopback-adjacent test lock (`cmd/kurodo/main_test.go`: only `KURODO_ROOT` / `KURODO_PORT` may be read). Renaming the variables updates a security lock and its assertion together.
3. The four cron consumers call `~/go/bin/kurodo`; the binary name changes on install, so they cut over to the `yomihon` binary with the same backup-and-verify discipline the kura→kurodo switchover used — the old binary stays until the cutover is verified, then is deleted.

Sequencing: after the experience batch merges and before the next feature branch opens — a rename touches every import path, so it must be a solo sweep on a clean tree. The GitHub repo renames and the local directory move are Koopa's hand (a directory move under a live session breaks tooling).

## D45 Every vault file is readable in yomihon (Koopa, 2026-07-09)

The sidebar lists the whole vault, but the reading route served only `.md`
and 404'd the rest — surfaced when the vault root's Makefile appeared in the
tree as a dead link. Koopa ruled: the terminal does not pick which files
deserve eyes — every file the vault holds opens, and the presentation fits
the type. Markdown keeps the note page. Text renders as a read-only source
view, chroma-highlighted by extension and plain where unknown. Images
display; PDFs hand to the browser's viewer; anything binary (or any text
file too large to render comfortably) gets an honest information page
pointing at its raw bytes. Raw bytes serve under the same CSP-sandbox
discipline the report briefings established, because a same-origin SVG or
HTML document would otherwise run scripts against the app's origin. The
write face is untouched — source views carry no status face and no seal —
and search and the graph stay markdown-only: reading widened; adjudication
and indexing did not. (Correction, 2026-07-09: "the graph stays
markdown-only" understated the standing behavior — the graph already
indexes every non-markdown file as a wikilink resolution target, extensions
kept, pinned by test. It is the graph's *note set* that stays
markdown-only; this ruling makes those already-resolving links open instead
of 404.)

## D46 The seal status has one source (guide, 2026-07-10, under Koopa's same-day delegation)

`sealStatus = "ready"` is written three times — `internal/note/handler.go`,
`internal/status/handler.go`, `internal/ui/pages/note.templ` — each with a
comment explaining that it is the one true copy. Three true copies are a
hardcoded enum wearing an apology, and wall 3 admits none: the schema toml is
the single source of schema understanding, and `internal/schema` is the only
package that reads it.

The literal moves to `internal/schema` and the three packages consume it from
there. Derivation from the state machine was the preferred form — the seal is
the koopa-only transition — but the owner does not identify it: `ready` and
`published` both carry `owner = ["koopa"]`, so "the koopa-only status" names
two. Deriving from a predicate that is not unique would encode a coincidence.
It is therefore one pinned constant in `internal/schema`, named for the seal
and not for its owner, with the toml as its citation. Should the schema ever
mark the seal explicitly, the constant becomes a derivation and the consumers
do not move.

## D47 The instance contract separates paths, maps, and artifacts (Koopa, 2026-07-11; amended after review the same day)

The vault contract now declares navigation roles and non-instance locality.
Study paths retain unresolved, ambiguous, and uniquely resolved non-instance
targets in source order as warning, non-link rows; general maps expose only
uniquely resolved governed targets in navigation, while their reading pages keep
every row. A non-instance study-path warning carries its source text and target
only: no governed path, status, placement, readiness, or write identity. It
counts as a path row but never as ready. `[navigation]` accepts only disjoint,
duplicate-free `path_types` and `map_types` drawn from `[enums].type`; one bad
value invalidates the whole role set.

`[navigation]` and `[artifacts]` validate and degrade independently. Missing,
invalid, or incomplete navigation roles disable Paths and Maps with a named
diagnostic, without closing aggregates or writes. Missing, invalid, or incomplete
artifact policy disables Paths/Maps, Home recent/lifecycle/advanceable
projections, metadata-filtered search, and the write face identically, with no
all-files fallback; Folders, direct and raw reading, reports, and text search
remain available. Each declared section has required keys: `path_types` and
`map_types` for `[navigation]`, `non_instance_dirs` for `[artifacts]`. An explicit
empty list is a valid declaration of none; an omitted key makes that capability
invalid rather than defaulting it to empty. Diagnostics distinguish the states.
These byte-stable English strings are internal capability detail; D28's later
browser-language ruling requires each browser surface to lead with a
Traditional Chinese explanation and to mark any retained exact detail as
English technical text:

- missing navigation: `contract declares no navigation roles; Paths and Maps disabled until it does`;
- missing artifacts: `contract declares no artifact policy; instance projections disabled until it does`;
- incomplete navigation names missing `path_types`, `map_types`, or both and ends `Paths and Maps disabled`;
- incomplete artifacts: `invalid artifact policy: missing required key "non_instance_dirs"`;
- an invalid value names that value.

Core TOML or lifecycle failure keeps the standing whole-contract degradation.

Artifact locality is frozen as NFC-normalized, vault-relative component-prefix
matching. Empty, `.`, `..`, absolute, and backslash paths are rejected; a path
matches directory `D` only when it equals `D` or begins with `D + "/"`, so
`System/templates-old` does not match `System/templates`.

A non-instance file has no Paths/Maps destination or reverse placement and is
absent from Home recent, structured-search results, lifecycle and advanceable
counts, and lesson reading enhancements. The sole navigation exception is the
source-order warning representation above when a study path names it. Its status
UI is a quiet non-governable state with no forms, and `/status` rejects it. It
remains present in Folders, direct note/file/raw reading, bare-text and folder
search, and unchanged wikilink resolution and diagnostics; judge keeps its own
scan policy. Metadata filters (`type`, `status`, `domain`, `topic`, `slug`)
require valid artifact policy. Without it, any query containing one — including
a mixed metadata-and-text query — is explicitly unavailable, never treated as an
ignored filter or zero results; bare-text and folder queries continue.

Status writes parse the form, normalize and validate the path, then reject a
non-instance before stat/read/git: `ErrNonInstance` is HTTP 422, including for a
nonexistent target. Its internal sentinel is
`status: target is not a governable artifact`; its D28 browser presentation is
`不屬於生命週期治理範圍`. Missing
or invalid or incomplete artifact policy is a distinct HTTP 503 from core-contract closure.
Zero-entry maps keep the current `<details>` plus “Open map” presentation in this
round. 4M remains prohibited; only a concrete user task blocked by the current
presentation may reopen it.

**Amendment record.** The original ruling did not enumerate the
role × resolution × target-instance-state row for a uniquely resolved
non-instance study-path target, nor the section-present × required-key-omitted
configuration state. Implementation and tests chose answers, then the first D47
wording repeated them; that was conformance to an assumption, not authority.
The amended rows above are Koopa's correction after review. `standards.md` now
requires the full predicate cross-product with authority labels before dispatch
and forbids tests, mutations, campaigns, or later prose from manufacturing a
ruling.

## D48 Personalization is bounded: presentation, never interpretation (Koopa, 2026-07-11)

The owner may tune how the page reads; nothing the owner tunes changes what
the vault means. Candidate dimensions — not build commitments — are the
classic reading controls: theme (light/dark/system), a curated set of
reading font presets, size, line height, measure, paragraph spacing, ruby
visibility/size/contrast, reading density, plus the system accessibility
preferences already honored. Constraints, all load-bearing:

- The default remains the complete, quiet scholar's desk; preferences serve
  taste, never repair a broken default.
- Preferences are user-initiated, reversible, and can always return to the
  default. "Typography" means article reading typography — the application
  shell is not user-rearrangeable.
- No preference changes schema truth, note membership, navigation
  predicates, mode existence, route behavior, semantic ranking, privacy or
  egress, the status lifecycle, or any vault file.
- AI never auto-restyles: no theme, font, layout, or mode changes from
  inferred content, persona, or mood.
- Fonts stay self-hosted or system-stack (the shipped woff2 discipline);
  no appearance setting may introduce a third-party network request.
- Keyboard, focus, zoom, contrast, ruby readability, and reduced-motion
  are never sacrificed to a preference.
- Preferences are local and per-device (the theme/furigana cookie is the
  shipped precedent); an app-side settings file, cross-device sync, or any
  vault write is a separate storage ruling.
- Out of scope unless separately ruled: theme marketplaces, arbitrary
  custom CSS, remote font URLs, drag-and-drop panel layouts, per-mode
  workspace layouts, layout DSLs, and any setting that hides authoritative
  content.

## D49 Single-key shortcuts are an accepted deviation from WCAG 2.1.4, narrowly (Koopa, 2026-07-12)

The global printable shortcuts (`/` filter, `[` drawer, held `R` seal) stay,
without a disable or remap mechanism. This deviates from WCAG 2.1.4, which
wants printable-character shortcuts turn-off-able, remappable, or
focus-scoped — the deviation is recorded, not waved away, and it is narrow:

- It holds only for the current product form: one operator, one machine,
  shortcuts chosen by that operator for himself.
- The suppression contexts are part of the deviation's terms: text entry,
  select, contenteditable, and open dialogs always disarm single-key
  bindings. A regression there is a defect, not a taste call.
- Held `R` keeps its hold guard, and no shortcut path may ever bypass the
  state machine's legality check — the shortcut is an entrance to the same
  write face, never a side door.
- Reopen conditions: the moment the product serves more than one user, a
  remote surface, voice control, or alternative input methods, this
  decision must be re-ruled, not extrapolated.
- If disable/remap ever ships, its natural home is the D48 preferences
  lane — but D48 as written forbids sacrificing keyboard invariants to a
  preference and therefore does not pre-authorize it; that future needs its
  own explicit exception here, so the canon never contradicts itself
  silently.

*(Amended 2026-07-20 by Koopa, superseding only the no-disable conclusion
above. One local, per-device, persisted control named for single-key shortcuts
ships default-on and turns off exactly `/`, `[`, and held `R` together. It does
not remap them. ⌘K, Escape, and every other keyboard behavior remain
unchanged. While enabled, the existing typing/select/contenteditable/dialog
suppression, held-R guard, and lifecycle-legality checks remain mandatory. The
turn-off mechanism closes the WCAG 2.1.4 deviation; the historical rationale
and reopen conditions above remain the record of the earlier ruling.)*

## D50 The hybrid-search ruling sheet (Koopa, 2026-07-12)

Eleven rulings closing the B plan's adversarial round; search-plan.md Part II
is their working-out, and every degraded-matrix cell traces to one of them.

1. **Query egress**: only an explicit CLI `--semantic` action embeds a query,
   at most one send per process action. Ordinary UI search (⌘K, `/search`, and
   the live fragment) is lexical always; raw query text never enters logs,
   caches, errors, metrics, or traces.
2. **Generation cutover** (amended 2026-07-14 under Koopa's delegated product
   authority): one process owns exactly one complete generation identity. It
   queries only a compatible active generation and never keeps a registry of
   retired model/protocol clients. An explicit incompatible build may stage a
   replacement while the old active generation remains transactionally intact,
   but that old generation is not queryable by the new-identity process. Search
   exits 3 with `cache-mismatch` until activation. A query vector never scores
   against another numerical identity; `previous` is a publication-retention
   slot, not an automatic fallback or a user-visible rollback command.
3. **Representation**: chunk-only; D32's note-level vectors are dropped
   (amended there). Reopens only if the eval set shows broad-topic recall
   failing — not on token-limit grounds.
4. **source_kind moves to the H face**: it is not a hybrid dependency; B
   leaves the grammar untouched, and the canon sync (spec §3, Part I §3)
   precedes H's implementation.
5. **Strict CLI never serves a stale/partial semantic generation.** Compatible
   bounded drift is reconciled to a complete generation before query; every
   failure exits 3 with a typed reason. A partial answer never wears exit 0 —
   anything else smuggles the killed best-effort mode back in.
6. **Rate limiting**: query and interactive-reconcile paths fail fast (429 =
   unavailable now); only the explicit `search-index build` command backs off,
   boundedly and in its persisted staging retry ledger.
7. **`--semantic=best-effort` is dead**, including roadmap §4a's "may
   exist".
8. **Eval artifacts**: the repo commits only synthetic fixtures; real-vault
   evaluation stays local with its per-query paired diffs; only
   content-free aggregates may be committed or quoted.
9. **Embedder of record: `gemini-embedding-2`** — the 001 generation's
   retirement window opened before build (the listed date is the earliest
   possible retirement, which already disqualifies it as a plan of
   record), and embedding generations are incompatible, so the change
   costs one full re-embed. Dimension and chunking are evaluated within
   this model; an on-device comparison is not B's obligation and reopens
   only on cloud-quality failure, an egress re-ruling, or an offline need.
10. **The privacy capability is the dispatch prerequisite for all of
    Part II** — no cloud document embedding, no agent-facing
    ranking/fusion/output ships before the vault contract declares it and
    `internal/schema` derives it, fail-closed.
11. **Provider account and distribution posture** (Koopa, 2026-07-13):
    yomihon does not operate a hosted AI service. Semantic search is optional
    and BYOK-only: Koopa's live deployment uses his own paid Gemini project;
    each downstream user supplies their own provider account and key and is
    responsible for that provider's terms. Yomihon bundles no credential,
    shares no account, and operates no embedding proxy. A key is required only
    for live provider use, never for compilation or offline verification;
    provider-specific terms are external to this implementation plan rather
    than copied into canon.

*(Amended 2026-07-13, closing the delta round's open cells: the four-walls
text now names both egress exceptions, ending the conflict with item 1;
`lexical/not-applicable` joins the CLI contract as the fourth legal pair —
`--semantic` on a pure-filter or empty-text query answers lexically, exit
0, zero embedding; with the privacy capability missing or invalid the
strict CLI emits no result payload at all (exit 3, fixed privacy stderr) —
agent output is fail-closed, and local UI lexical continues; every request
walks the precedence gate privacy → artifact → semantic applicability →
**semantic-request fork** → cache → configuration → query API, the first
failure names the reason: the fork answers any request that did not
explicitly ask for semantic (plain lexical) as `lexical/off`, exit 0,
right there — so **the cache, configuration, and query-API stages are
reached only by a text-bearing request that explicitly asked for
semantic**, and a plain lexical query is never turned away by a cold
cache, a missing key, or any network state. The query text never leaves
the machine before the final stage; item 9's "one full
re-embed and nothing else" is withdrawn — the successor's request protocol
differs and re-pins from its own documentation into the cache identity;
and unmanaged epoch-identity mismatch (cold) was initially distinguished from
managed old-epoch serving. The latter was superseded by D50.2's 2026-07-14
single-identity ruling: staging remains an explicit publication state, but a
new-identity process does not serve the incompatible active generation.)*

*(Amended 2026-07-13, closing the last two open families. **The agent
wire has three discriminated envelopes and no query-echo field.** They
are told apart by which top-level key is present, not by exit code alone
(exit 3 can be either of the first two): a request the command can answer
(even only lexically) carries the `{mode, semantic, coverage?, results}`
shape (exit 0 or 3); a request it cannot answer at all — the privacy
capability invalid, or a metadata filter that cannot be evaluated, with or
without `--semantic` — carries only the `error` envelope shape (exit 3);
and a request yomihon can confirm it formed wrongly carries only the
`internal_error` envelope shape (exit 1). H7 freezes the byte-exact JSON
bodies and stderr lines for those shapes. Non-JSON mode prints silent
stdout with the frozen stderr line for the second and third. Copying the
query into a diagnostic would put raw query text into an error surface, which
item 1 forbids. Answer results may naturally contain matching note bytes; no
dedicated field repeats the caller's input. Item 10's boundary is
therefore precise: a capability-only diagnostic may leave; a result or
ranking payload may not. **Embedding-credential failure splits in two**: a
missing key is a configuration-preflight state before the network —
`embedder-unconfigured`, no client constructed, no query embedded,
reading never blocked, reported only by the explicit semantic CLI action
— and it is checked only for a text-bearing CLI request that explicitly asked
for semantic, after the index state is known, so it can never mask the
strict CLI's earlier incomplete-index reason. Ordinary UI search never enters
this gate and therefore has no embedding-configuration state. A provider refusal of the credential
is `embedder-rejected`: at most one call per explicit action, no in-place
retry, the provider's response body never forwarded, and no cross-request
auth latch is authorized — the status-code classification is pinned from
the successor's own error taxonomy at the protocol step, not hardcoded
here. The full provider error taxonomy splits by **fault ownership** (ruled by
Koopa 2026-07-13 — an EXPLICIT-RULING, not a derivation; the earlier draft
that called it CANON-DERIVED from D50.5 overreached, since D50.5 rules only
stale-partial): **only a response we can confirm is a yomihon-formed
malformed request is an internal error** — CLI exit 1, no result or ranking
payload. **Every known provider fault, and any unknown or unclassifiable
provider response, is treated as semantic `unavailable` and never claimed
as a yomihon fault** — CLI exit 3 with the lexical results preserved. The
ordinary UI remains lexical and has no provider diagnostic. The default for uncertainty
is therefore "the provider's problem, semantic off," never "our internal
error." The concrete provider-error-class → reason mapping is completed at
the protocol step from the successor's own documentation. The single
outbound request per action is counted at the HTTP boundary, not a client
method, with automatic retry disabled, so a transient failure cannot
amplify the query egress.)*

*(Amended 2026-07-13 by Koopa: ordinary human search is permanently
lexical-only. Semantic/hybrid retrieval is an explicit CLI/agent capability;
the former UI submit/toggle semantic path, its query egress, credential state,
provider diagnostics, stale-partial serving rule, and UI columns in the
degraded cross-product are retired. A future human fuzzy-exploration need may
be ruled as a clearly separate Related/Find-related surface, but this amendment
does not authorize it and it may not be mixed into ordinary search.)*

*(Amended again 2026-07-13 under Koopa's delegated product authority,
superseding the serve-owned background and immediate stale-partial terminal
above. `serve` is structurally outside semantic search. A cold or incompatible
index requires the explicit `yomihon search-index build` action; a compatible
active generation that merely differs from current vault content is
automatically reconciled by the explicit `search --semantic` action before its
single query send. Reconciliation may send only changed/new eligible document
chunks and may write the cache; it never sends the query first. Failure leaves
the old active generation byte-identical and returns the lexical answer with
exit 3. A concurrent writer yields the typed `index-refreshing` unavailable
state. Successful reconciliation creates a complete immutable generation and
activates it in one SQLite transaction. Stale vectors and partial hybrid
answers remain forbidden. This product choice favors a search that normally
just works without allowing one query to trigger an implicit first/full
corpus build.)*

*(Implementation clarification, 2026-07-13, under the same delegated product
authority. "Compatible drift" is intentionally bounded: before document
egress, `search --semantic` computes the exact missing-vector work and will
reconcile at most **128 chunks and 100,000 submitted proxy tokens** in one
interactive action. Crossing either bound returns typed `rebuild-required`
without sending a document or query and directs the operator to the explicit
build command. Interactive reconciliation performs one attempt per document
and fails fast on 429 or any provider failure; D50.6's bounded retry/backoff is
owned only by `search-index build`. Before activation, the candidate's complete
sorted content-hash corpus manifest and both policy sources are re-read and
must still match; drift leaves the prior active generation untouched, sends no
query, and returns typed `vault-changed`. A build may persist **one** staging
generation so paid successful rows survive interruption, but it is resumable
only when the full vector identity, target content-hash corpus manifest,
policy-source freshness, expected chunk count, and recorded retry state still
match; otherwise the next writer discards it locally before egress. Active and
previous are immutable, staging alone is mutable, and activation transactionally
performs `previous=active; active=staging; staging=NULL` before pruning every
unreferenced generation. No automatic fallback ranks from previous. Finally,
structured filters are local constraints and are never embedded: the query
provider receives the original-case, original-Unicode bare tokens in input
order, joined by one ASCII space; recognized filter tokens are removed. This
projection is part of the protocol identity.)*

*(Amended 2026-07-14 under Koopa's delegated product authority. The semantic
generation store's first implementation is supported only on **Darwin and Linux** and is
deliberately unsupported on Windows: Go's
synthetic Windows mode bits cannot prove the owner-only `0700`/`0600` privacy
boundary, and an honest DACL design needs its own ruling and Windows-runtime
evidence. The three store entry points therefore return a typed error before
stat, mkdir, SQLite, key access, or provider construction. Plain lexical CLI,
ordinary UI search, `serve`, and judge remain supported and store-free. A
text-bearing `search --semantic` request returns exit 3 with its lexical answer,
`semantic=unavailable`, and reason `unsupported-platform`; `search-index build`
returns the exit-3 error envelope with the same reason. Pure-filter semantic and
plain lexical requests terminate at their earlier gates and never observe this
state. H7 freezes the stderr bytes and matrix row; a real `windows-latest` job
proves that no directory, database, lock, WAL, SHM, or journal is created.)*
*No runtime/compile support claim is made here for other targets. In
particular, a Unix-like `GOOS` name is not support evidence: the selected
SQLite dependency does not currently compile for every such target.*

*(Implementation clarification, 2026-07-14, under the same delegated product
authority. A production generation may activate only with H9's matching
40-query recorded-vector workload; its 120-sample top-k p95 is stored at a
minimum of one microsecond. `top_k_p95_us=0` means only an internal synthetic
fixture-capture/bootstrap generation and is not a dispatchable production
state. Rung 1's deterministic exact-index envelope is `chunks < 100,000` and
raw vector payload `<= 1 GiB`; crossing either condition returns `capacity`
before allocation and opens the rung-2 comparison. Thus 3,072 dimensions admit
at most 87,381 chunks, while 1,536 dimensions reach the count gate first. This
is a preflight guarantee, not a promise that arbitrary Go/OS allocation failure
can be recovered.)*

*(Amended 2026-07-16 under Koopa's delegated product authority. The H9
retrieval oracle distinguishes relevance from prohibition. Every required
positive must be in the top 5; rank 1 must belong to the required or explicitly
acceptable positive set. A designated related sibling is contrastive rather
than forbidden: if present, it must rank below every required positive. Hard
filter violations remain forbidden across the complete fused result list.
The live paired `gemini-embedding-2` run gave both 1,536 and 3,072 dimensions
the same 40/40 required-positive rank-1 result and the same 40/40
contrast-below-positive result. Because 3,072 showed no retrieval benefit while
doubling vector payload and exact-scan work, 1,536 is the production dimension
and enters the cache identity and committed recording.)*

*(Amended 2026-07-20 by Koopa through D60. Explicit build retry authority is
now a durable five-send-slot batch per pending chunk and storage generation,
not five guaranteed HTTP requests or a renewable ordinary-build loop. Only 429
retries automatically inside one build action. Exhaustion is
`attempt-budget-exhausted`; a separate `--renew-attempt-budget` action may
authorize one replacement batch only under D60's exact staging preconditions.
The build error wire now carries active/staging recovery state, immediate retry
safety, and the next action. The search answer envelope is otherwise unchanged.)*

## D51 `published` is selection for publication, not an external-success receipt (2026-07-16, under Koopa's delegated product authority)

`status: published` means that Koopa has selected the note for membership in
the public collection. It is desired state and an input to a future publisher;
it does not assert that an external deployment succeeded, that a particular
revision was deployed, or that the note is currently live.

The lifecycle entry in `vault-schema.toml` remains the only authority that can
offer or refuse this transition. Yomihon must not add a `published` string
special case in Go. A future publisher reads the selected set without writing
publication receipts back into the vault. Its deployment revision, success,
retry, and liveness records belong to the publisher's own log or repository.
Before an automated publisher first consumes this state, its plan must include
a reviewed historical backfill, its own privacy exclusions, and an explicit
review of the selection affordance, mistaken-selection recovery, and withdrawal
path. Those are publisher obligations, not meanings smuggled into the status
field. D13's current quiet-transition treatment does not by itself authorize a
future publisher to make one click immediately public.

The rejected alternative was receipt-gated status. Yomihon cannot verify every
manual publication or historical backfill within its local privacy boundary,
so that interpretation would make one field mean verified receipt on one path
and human attestation on another.

## D52 Lifecycle transitions always change state (2026-07-16, under Koopa's delegated product authority)

A lifecycle transition from a status to the same status is illegal. For
`from = ["*"]`, the wildcard means the initial state plus every *other*
declared status legal for that note type; it does not authorize arbitrary
unknown status bytes or a no-op self-transition.

This keeps the UI from offering the current status as an action and preserves
the write face's audit contract: one accepted action produces one actual status
change and one matching commit, rather than a no-op rewrite followed by a
commit failure.

## D53 Schema coordination and supersession vocabulary have bounded owners (2026-07-16, under Koopa's delegated product authority)

The schema-v1 coordination keys do not all create yomihon behavior:

- `aligned_with` names the human doctrine that reviewers keep semantically
  aligned with the machine contract. Yomihon retains and validates the key's
  shape but does not claim it can prove prose-level alignment.
- `generated_at_must_match` belongs to the vault's generated-fileclass check.
  Yomihon decodes it so strict loading accepts the current vocabulary; it does
  not reimplement that generator or its CI.
- `[supersession]` is machine authority for the existing replacement ledger.
  Lessons name the configured predecessor and successor fields. Other note
  types use the configured general-link field. The configured archived status
  is the common structural exit for superseded notes.

When `[supersession]` is present, the archived status must be legal for every
declared type, have one non-overlapping lifecycle authority row for every type,
and accept every *other* status in that type's effective status group. This is
a structural representability check, not an authorization check: `owner = []`
remains valid and intentionally pauses interactive execution without making
the supersession vocabulary incoherent.

The judge consumes these configured field names in three bounded ways:

1. The existing `provenance.unresolved` rule reads the configured lesson
   predecessor/successor fields and the configured general-link field instead
   of hardcoding `evolution_*`. Findings for those configured fields cite
   `vault-schema.toml#supersession`; the fixed `based_on` and `related` fields
   retain their existing provenance citation.
2. `supersession.predecessor_not_archived` is a warning on a governable lesson
   whose configured successor field contains at least one genuine string while
   its status is a declared non-archived lesson status. A missing or invalid
   status is left to the existing schema findings rather than producing a
   second claim.
3. `supersession.archived_navigation_target` is a warning on each body
   wikilink in a live, governable type declared by `[navigation]` as a path or
   map when that link resolves uniquely to another governable note whose status
   is the configured archive status. Unresolved and ambiguous links remain
   owned by their existing rules.

The whole judge command requires valid privacy authority (D54). Within that
authorized scan, both new rules require a valid artifact policy, exclude
privacy-denied and non-instance source and target files, and the
navigation-target rule additionally requires valid navigation roles. If
`[supersession]` is absent, neither new rule runs and `provenance.unresolved`
does not guess replacement-field names. If a rule-specific capability is
unavailable, the affected rule emits nothing rather than classifying files
through a fallback. Each new rule is `warn`; its JSONL shape, diagnostic
strings, ordering, and fingerprint are frozen by the judge golden contract
before release.

The judge must not infer reciprocal-link requirements, a fuzzy-duplicate
threshold, or an automatic delete/write path. Similarity and reciprocity remain
human review questions until separately ruled.

## D54 Agent judge requires explicit privacy authority (2026-07-16, under Koopa's delegated product authority)

`check`, `coverage`, and `exists` are agent-facing egress surfaces, not merely
local readers. Each command must load one valid `[privacy]` capability before
reading vault notes and retain that same authority through emission. A missing,
invalid, or source-stale privacy capability is a tool failure: every format
emits empty stdout, exits 2, and writes exactly
`yomihon: privacy authority unavailable; agent-facing output disabled\n` to
stderr. An explicitly present `never_egress_dirs = []` remains valid and means
that the contract intentionally declares no private directory.

This is an explicit availability and wire ruling, not a derivation from
D18/D39/D42: those decisions require exclusion but did not choose what the
three commands do when the exclusion authority itself is unavailable. Exit 2
uses the judge's existing tool-error class; exit 0 would falsely certify a
clean/empty result, exit 1 has command-specific meanings, and semantic
search's exit 3 does not belong to this CLI.

The privacy policy, never a Go path literal, owns every output and influence
filter: finding paths/collision members/resolved targets and title evidence;
planned-name sources; coverage candidates, mount sources, and unrouted rows;
exists matches; and D53 source/target eligibility. The full graph may still
contain a private note so a public author's own link text and resolution retain
D42's semantics; private notes are removed at the rule/output boundaries, not
pre-deleted from the graph.

Privacy directory prefixes compare NFC-normalized path components with Unicode
case folding on every platform. This is CANON-DERIVED from D18's fail-closed
boundary plus the platform-honesty rule: a case-insensitive filesystem can name
one directory through multiple case spellings, and such an alias cannot create
egress authority. On a case-sensitive filesystem the conservative consequence
is only that a case-variant sibling is also withheld.

Error output is also egress. Contract/privacy failures use the fixed line above,
and note-walk/read failures use the content- and path-free `vault scan failed`
tool error. The same loaded policy is revalidated after payload construction
and immediately before `stdout.Write`; a filesystem and an arbitrary
`io.Writer` cannot form one atomic transaction, so this adjacent final check
is the honest publication boundary.

## D55 Judge scan, grammar, and wire ambiguities are closed (2026-07-16, under Koopa's delegated product authority)

The agent judge strictly walks the complete non-hidden vault. Hidden path
segments remain excluded and `.gitignore` remains irrelevant, but an unreadable
nested entry is a tool failure rather than a partial clean result. Default
`check` scope removes only findings whose touched paths are all under
`System/`; `--all` disables that presentation filter. `Diagrams/` and `Views/`
have no separate exclusion. Contract-declared privacy exclusions remain
absolute and are never relaxed by `--all`.

`--root` and `--format` are shared flags. `--all`, `--deny`, `--baseline`, and
positional path filters belong only to `check`; `coverage` accepts no
positionals, and `exists` accepts exactly one name. A flag or positional owned
by another command is an exit-2 usage error before any vault scan.

In JSON/machine format, `coverage` and `exists` emit compact JSON followed by
one newline. Their explicit `human` format remains readable text, and `md`
falls back to that human view because markdown is check-only. Historical pretty
JSON snapshots define value shape, not wire bytes. Current tool errors use the
honest `yomihon:` prefix; predecessor names in the migration record are
historical, not live output contracts.

## D56 Reading projections are request snapshots; commit provenance is byte-bound (2026-07-16, under Koopa's delegated product authority)

A reading request captures one immutable lifecycle view and one immutable vault
snapshot, then derives every projection in that response from those values. A
contract edit after capture affects the next request; the UI does not pretend a
filesystem-backed response can be globally atomic with an external editor.
This is safe because the local reading surface is not an egress boundary and
every status POST still revalidates current authority under the lifecycle
publication lock before writing.

The sealed-note commit line is stricter: the note reader passes the SHA-256 of
the exact bytes it rendered to `internal/status`. A hash is shown only when the
current file is clean and matches those bytes both before and after the git-log
query. A concurrent flip or external edit therefore suppresses provenance
instead of pairing an older reading snapshot with a newer commit.

## D57 Fixed synthetic provider certification is a bounded egress exception (2026-07-16, under Koopa's delegated product authority)

Provider protocol and dimension claims cannot be certified offline, but routing
their verification through the ordinary user search command would create a
bootstrap cycle and misdescribe who owns the request. An explicit developer
certification action may therefore send only two fixed, repo-owned classes:
the hard-coded synthetic protocol probes and the committed synthetic eval
corpus/queries.

This is the third outbound exception to wall 2, separate from D32's eligible
note-content egress and D50.1's explicit user-query egress. Its boundary is
structural:

- no argument or environment value can supply text, a vault root, or an
  operator-selected input file; input bytes come only from the fixed repo
  fixtures, while the local output path and ruled 1,536/3,072 candidate
  dimension may vary;
- every corpus path and query carries the repo's synthetic marker and is
  validated before use;
- each row is one direct request with automatic retry, redirect following, and
  environment proxies disabled; provider bodies and submitted text are never
  logged or forwarded;
- the action is opt-in, test-only, and absent from the product binary; it uses
  the same `YOMIHON_EMBED_KEY` credential name rather than adding a second
  product configuration surface;
- its output remains local, owner-only, structurally validated, and contains
  vectors plus content hashes rather than raw query or corpus text.

This does not authorize an arbitrary recording tool, a real-vault capture
driver, or a new user-facing query surface. Real-vault evaluation may consume
only vectors already produced by prior explicit `search --semantic` actions.
Changing any input ownership above is a new egress decision, not a test refactor.

## D58 Status publication is supported only where its durability claim is proved (2026-07-17, under Koopa's delegated product authority)

The ordinary reader, navigation, judge, and lexical search remain supported on
macOS, Linux, and Windows. The status write face is supported on macOS and
Linux only. A successful status action promises more than an atomic rename: it
promises that the replacement file and its containing directory entry were
synchronized before the matching git commit and 303 response. The first
implementation has real-kernel evidence for that sequence only on Darwin and
Linux.

On Windows and every other unproved target, contract-derived read projections
remain available, including lifecycle groups and advanceable counts. The
platform limitation removes transition forms, gives the reading page a stable
write diagnostic, and makes a direct `POST /status` return 503 with the
unchanged recovery state. After form and vault-relative path-shape validation,
the refusal occurs before target filesystem, git, or contract-source access.
It must never perform the rename first and report uncertainty afterward.

Go exposing a directory handle is not sufficient evidence. Microsoft documents
`FlushFileBuffers` for a handle with `GENERIC_WRITE`, while the documented list
of directory-handle consumers does not establish it as a directory-entry
durability barrier. A future Windows write implementation therefore requires a
documented confinement-preserving primitive, a watched-red crash/durability
contract, and a real Windows CI runner before this ruling is widened. A
path-based call that weakens `os.Root` confinement is not an acceptable
portability shortcut.

## D59 Authored note HTML is display input, not browser authority (2026-07-18, under Koopa's delegated product authority)

The vault remains the byte authority: reading never rewrites a note to make it
safe. The HTML projection, however, grants authored markup only the inert
Japanese-reading subset the vault demonstrably needs: `ruby`, `rt`, `rp`,
`br`, and a conservatively validated quoted `lang` attribute on the first three
elements. The exact inert `<!-- read-aloud: ja -->` lesson marker also survives
long enough for the server to consume it into a TTS control. Other authored
tags, attributes, and comments are rendered visibly as text.
In particular, a note cannot contribute scripts, event handlers, refresh or
navigation directives, styles, frames, forms, media, or automatically loaded
remote subresources to yomihon's first-party page. A remote Markdown image is
an explicit external link rather than an automatic request; ordinary external
Markdown links remain user-activated navigation.

This is a renderer boundary backed by a response boundary. Every executable
application script carries the unpredictable nonce of that response. The
reading CSP accepts that nonce through `strict-dynamic`, does not grant
`script-src 'self'`, and refuses script attributes, objects, workers, foreign
frames, and foreign automatic subresources. CSP is defense in depth, not the
mechanism that neutralizes authored `meta` refresh: that element is already
inert before the browser parses the page.

Reports retain their distinct D30 contract, but not its historical script
allowance. The allowlisted bytes stay unchanged and render as static HTML,
inline CSS/SVG, and data media in a bare sandboxed opaque origin. Report-authored
JavaScript never executes; automatic refresh/navigation, forms, remote resource
loads, and WebRTC remain inert. The resource-level CSP independently refuses
scripts and automatic network mechanisms. This is intentionally narrower than
claiming that an opaque origin or CSP can firewall arbitrary executable
JavaScript. If reports later need interaction, a separately ruled first-party
renderer consumes a declarative data format; raw authored JavaScript is not
enabled. Every alternative raw-file route either renders markup as escaped
source or uses its own scriptless sandbox. This also amends D30's historical
third mechanism: `/notes/` now has read-only views for non-Markdown files, so
the invariant is no longer "non-note means 404"; it is "no alternative route
executes those bytes with first-party authority."

## D60 Semantic retry authorization, recovery wire, and first-use gaps are closed (Koopa, 2026-07-20)

This ruling amends D49 and D50 without erasing their history.

**Semantic send authorization and retry.** Before invoking the chunk-send
capability, the build ledger commits one durable send-slot reservation. Any
abort after that commit and before the transaction that stores the returned
vector and clears the attempt row consumes the slot, even when no HTTP request
was made. Provider configuration or construction failure happens before
reservation and consumes no slot. Each chunk-send or query-send client method
invocation still performs at most one HTTP request, follows no redirect, and
uses no ambient proxy; a full-build action may invoke the chunk-send method
again only through the bounded 429 loop below.

Each storage generation grants at most five send slots to each pending chunk.
Only a 429 automatically retries inside one `search-index build` action. A
valid `Retry-After` overrides the 1s/4s/9s/16s fallback; a wait over 30 seconds
is persisted and the action exits. Every other provider or local terminal ends
that action after its reserved slot. Exhaustion does not erase fault ownership:
a confirmed malformed request remains exit 1; credential rejection remains
`embedder-rejected`; and privacy, artifact, local-input, or vault-change
prerequisites retain their own reason even when the fifth slot was consumed.
Their recovery envelope reports `staging_generation=requires-authorization`,
so repair is followed by renewal rather than an ordinary sixth attempt. Only a
provider availability terminal with no higher-priority repair — 429,
unreachable, or unknown/provider-failed — becomes
`attempt-budget-exhausted` when its reservation consumes the last slot. A
build that begins with exhausted pending work also reports that reason. Slots
upper-bound actual HTTP requests; a reserved slot that aborts before transport
makes the bound conservative, and an ordinary build never mints more authority.

`yomihon search-index build --renew-attempt-budget` is the only renewal. It is
admitted only when one staging generation carries the complete exact target
manifest for the current build and at least one pending chunk has exhausted
its five slots. Under the writer lease it
revalidates privacy authority, artifact authority, corpus, and both policy
sources, then one transaction creates a new matching staging generation,
copies completed vectors, points the staging role at the replacement, and
deletes the old generation and ledger. The active role is unchanged by that
authorization commit. The same action then continues the ordinary build using
the new batch; if interrupted, the commit permanently authorizes the remaining
slots for a later ordinary build. One invocation renews at most one batch,
including when the flag is repeated. Missing, mismatched, corrupt, or
not-exhausted staging returns exit 3 `attempt-budget-not-renewable` with zero
domain mutation and zero provider send; it never falls back to ordinary build
or to a corruption reset. A missing store creates no file. For an existing
store, SQLite may recover committed WAL state and maintain WAL/SHM bookkeeping
while the existing writer lease is held, but schema, catalog roles,
generations, chunks, and send-slot rows remain logically unchanged.

**Recovery wire.** `search-index build --json` exit-3 failure has a dedicated
envelope whose `error` fields, in order, are `reason`, `active_generation`,
`staging_generation`, `retry_safe`, and `next_action`. Internal build errors
put the same four recovery fields after `detail` inside `internal_error`.
That internal envelope is only the confirmed exit-1 yomihon-fault surface.
Usage, local filesystem/SQLite failure, and interruption before emission remain
D37 exit 2 with empty stdout and one sanitized stderr line; they do not gain a
JSON recovery envelope. A stdout failure is also exit 2, but bytes already
accepted by the writer form an invalid partial envelope and must be discarded;
no contract can make those accepted bytes empty retroactively.
`active_generation` is one of `not-inspected`, `absent`, `preserved-usable`,
or `preserved-unusable`; `staging_generation` is one of `not-inspected`,
`absent`, `incompatible`, `resumable`, or `requires-authorization`.
`incompatible` means a physical staging role remains but is not admissible for
the current target; renewal leaves it untouched. `retry_safe` means the failed
action may be repeated automatically and immediately without repair, waiting,
new provider-budget consent, or consuming a new provider-send slot. Every
currently frozen build exit-3 reason is false; even
`next_action=retry-build` names a new operator action, not an automatic loop.
`next_action` is one of `retry-build`,
`wait-and-retry`, `renew-attempt-budget`, `repair-configuration`,
`repair-vault-contract`, `repair-input`, `use-supported-platform`,
`review-capacity`, or `repair-yomihon`. No `retry_after` field is promised.
Search keeps its answer envelope and lexical results; exhaustion appears only
as `coverage.reason=attempt-budget-exhausted`.

The recovery fields describe current observable state, not historical
provenance. In particular, an explicit ordinary build may reset corrupt
derived storage before later failing, in which case `active_generation` may be
`absent`; yomihon stores no cross-restart "corruption was reset" marker.
Failures preserve an active generation only when it was valid, and
`preserved-usable` is reserved for an active generation the current process can
actually serve. An incompatible, stale, retired, or otherwise nondispatchable
active is `preserved-unusable` rather than a false preservation success.

**Home, shortcuts, and first use.** With a valid vault contract, a missing
root `README.md` does not turn Home into a redirect or 404. `/` returns 200,
renders the complete snapshot-backed dashboard, and replaces only the README
body with an explicit read-only recovery state telling the operator to create
`README.md` at the vault root with an external editor or file tool, then reload.
Direct `/notes/README.md` remains an honest 404. Yomihon never creates that
file. The D49 amendment above adds the one default-on persisted single-key
control and no remapping. First use is documented with one tracked
`examples/vault-schema.toml` whose repository parser gate both loads the file
and asserts every example lifecycle owner is exactly `["koopa"]`. That value
is required by the shipped fixed status actor; it is product policy, not user
identity configuration. Yomihon may diagnose a missing or invalid contract and
point to that example, but it gains no `init` command and never creates or edits
the vault contract.

# kurodo functional spec (final form)

> **No milestone fences** (decisions D15): this document defines "what done looks like," not "what to build first." Each feature face has its own spec and acceptance criteria; every remaining face ships (D37), ordered by dependency and leverage — the sequencing view and the plan-doc obligations for faces not yet specified here live in `roadmap.md`.
> The four walls (`CLAUDE.md`) override everything in this document. Status: **certified (2026-07-02, including review revisions: §0.1 dependency boundary, §4 fail-closed and the audit boundary, D16=(a)); amended 2026-07-05 (D31/D32/D37: semantic search committed, scope ruling, dead-rule cleanup); amended 2026-07-06 (D40: the yomihon gate closed, retirement recorded as discard, the kura gate gains the differential-campaign prerequisite — judge-plan §13)**.

## 0. Goals

Let Koopa read the whole vault in one place, and complete adjudication right where he finishes reading; at the same time take over kura's judge and agent-toolbox responsibilities, becoming the vault ecosystem's human terminal + CLI interface.

**What the end state looks like (the definition of system success)**:

1. Koopa reads in kurodo every day — yomihon retires by its retirement gate.
2. The draft queue is flipped the moment reading finishes — adjudication friction disappears.
3. kura's four pipelines run on `kurodo check` — kura retires by its retirement gate.
4. Vault-side agents use kurodo as a toolbox (`check` / `exists` / `coverage` and later extensions).

### 0.1 Dependency boundary (global invariants)

- **Files are the truth; a derived index only accelerates. No face may depend on a persistent store**: v0 has no database at all — the graph, nav, and search indexes are in-memory, rebuilt from the vault (D24, D25), so search is as available as reading. The principle stands for any future persistent index (SQLite, D24): it may only accelerate, and the "read every day" habit MUST NOT inherit a daemon dependency — otherwise the tool-absorption law would kill it.
- **`check` / `exists` / `coverage` are stateless file scans that never touch the DB**: what the four pipelines and vault-side agents consume must be a zero-dependency binary (kura's deployment shape). If the judge face required PG to be present = a deployability regression = a covert tightening of the retirement gate — forbidden.
- **The single source of schema truth = `~/obsidian/System/schemas/vault-schema.toml` (wall 3), and fault tolerance is asymmetric by direction**: the reading face is fail-open on schema problems (render anyway + diagnostic — under-reporting a lint is harmless); **the write face is fail-closed** (if the schema can't be read or is broken → show no transition keys, reject every POST — under-reporting on read is fine, but a bad write destroys a file).
- **Privacy boundary** (`System/agent-guides/Privacy-Boundary.md`, draft pending Koopa's final review): the top-level `Diary/` never egresses. kurodo is local-only, so **rendering it for Koopa himself is legitimate**; but every egress face — `export`, `check` (findings written to a report will be read by agents), any snapshot — unconditionally excludes `Diary/`, `--all` included (D39: an enforced drop at the output layer, stricter than the reference engine ever was). The exclusion covers **influence, not only output** (D42): journal content never changes an egress surface's verdict about a public note — its edges do not mount public concepts, its planned lists do not soften public broken links. What stays visible is what a public note wrote itself: a link naming a journal title, and that link's resolution, are the author's own words. Mechanical source: once toml `[privacy] never_egress_dirs` lands, read it from the toml, not hardcoded (wall 3).

---

## 1. The reading face

**Spec**:

- Full-vault rendering: the complete dialect table = `vault-model.md` layer 1 (the four wikilink states, embed, callouts (two buckets + English default titles), `==highlight==`, tasks, raw-HTML ruby passed through verbatim, mermaid; including the highlight and embed that yomihon lacks).
- The fault-tolerance rule: broken YAML / unknown callout / broken link → render anyway + diagnostic, never crash, never fix (wall 4).
- TOC: CJK-safe slug (following yomihon's `Slug()` semantics: keep `\p{L}\p{N}`, `-2` suffix on collision, `section` fallback).
- Diagnostics pane: broken YAML, unresolved / ambiguous link, corrections ledger — display only.
- Reports: `System/reports/` contains daily-briefing HTML, presented verbatim in a sandboxed iframe. Sandbox policy: **no `allow-same-origin`** — even with scripts allowed it lands in an opaque origin, forming a double layer of defense with the same-origin protection.
- `.base` / `.canvas` link back to open in Obsidian; D2 is not rendered (already decided for koopa0.dev).
- Code highlighting is server-side (chroma). JS principle: **zero client frameworks; interactivity climbs the D41 ladder** — Chromium-native APIs, then hand-written vanilla (the five interactions are the precedent), then a mature vendored library when a D41 admission is recorded (mermaid is the standing one).

**Body leading-H1 policy**: the page title comes from frontmatter `title`; a leading H1 in the body is removed so it isn't shown twice (following yomihon's already-verified behavior).

**Acceptance**:

- All dialect conformance tests pass (the structural-assertion pattern inherited from yomihon's `testdata/lesson.md`). Fixtures cover at least: ruby / `<br>` passed through verbatim; all callout types + `[!x]-` / `[!x]+` folding + unknown types degraded to blockquote; the four wikilink states + alias display + ambiguous marking; `![[embed]]`; `==highlight==`; task list; GFM tables (including escaped `\|`); dialect not processed inside a fence + a one-time warning; a broken-YAML single diagnostic that does not cascade; body leading-H1 removal; CJK slug aligned with the TOC anchor (including the `-2` collision suffix).
- Every `.md` in the real vault opens: zero 500s, zero blank pages (the mechanical definition of fault tolerance).
- Japanese lessons (L01–L20 + the P series) render with all five interactions. (The original parity criterion — the 14 `m1-review/` screenshots as baseline — was waived by D40 along with the rest of the observation gate; the screenshots remain available in the yomihon repo as a design reference only.)

## 2. The navigation face

**Spec** (status-first, D26): the sidebar leads with a **Lifecycle** list — the schema's `note`-group statuses in toml array order, each with a live snapshot count and a link to the pure-filter browse page (`/search?q=status:<name>`) — then the syllabus tree, then Reports (daily-briefing HTML), then a **collapsed** Folders tree (the lifecycle folder browse, vault order, top level ≤9). Grouping, labels, and counts trace to the toml and the snapshot, never hardcoded (wall 3). The ≤3-click acceptance below is preserved by the Folders tree.

**Home-page spec** (finalized by the vault side's 2026-07-02 reply; **layout superseded 2026-07-05**: the D face lands the adjudication cockpit as the landing surface — status queues lead, and the four blocks below survive as cockpit content; the reconciliation is the D plan doc's job, `roadmap.md` §3): four stable blocks — five domain MOC entry points, four cross-domain boards (reusing the **questions** the boards answer, not hardcoding campaign-flavored labels), the mechanical-gate list, and pointers to the governing documents. **The two-layer IA is not flattened**: cross-domain boards live on the home page; domain workspace views (e.g. `日本語課程.base`) hang under that domain's MOC block. The facts of the status vocabulary always trace back to the toml (that section of Vault-Index is a human copy). The v0 division of labor for `.base` (linking back to Obsidian) is confirmed on the vault side — reimplementing it = a second query engine that also inherits the drift of the hardcoded schema inside `.base`; if a frontmatter query lands later, the boards could derive a native rendering from the toml, to be revisited then.

**Syllabus-tree parsing spec** (mechanically decidable):

- A syllabus is identified by frontmatter `type: study-path` (a toml enum), **not by hardcoding filenames**.
- Go syllabus shape: H2 = part, H3 = module, both in the pipe format `slug | English | Chinese` (split into three, trimmed); a list item = a lesson.
- 大家的日本語 shape: only the "課程序列" H2 has a navigation tree beneath it (H3 = learning stage); the other H2s (每日 loop, 學習階段, 缺口) are not navigation and are not parsed.
- A lesson's link = the **first wikilink** in the list item, resolved by graph semantics (vault-model layer 1); an unresolved / ambiguous lesson is **still listed + given the diagnostic styling**, never dropped (reading face is fail-open).
- Lesson nodes carry a status badge.

**Acceptance**: any vault file reachable in ≤3 clicks; syllabus-page order = the in-file listing order; the two real syllabi each render into a tree with a lesson count matching the number of list items in the file; deliberately write a broken-link lesson → it is still listed + marked.

## 3. The search face

**Spec**:

- Deterministic substring over an **in-memory index** (no database, D24) + structured filters; a ⌘K panel.
- **Query semantics** (deterministic, reproducible): whitespace tokenization; a bare word matches by substring after `fold(s) = strings.ToLower(nfc(s))` — the single match definer, applied to both the stored text and the query; multiple words = AND. Six fixed filter keys: `type:` `status:` `domain:` `topic:` (single-value containment on `topics`) `folder:` (rel_path prefix, `/`-boundary — `folder:Writing` does not match `Writing-old/`) `slug:`, values compared by literal equality, no enum validation. A pure-filter query (no bare word) is legal — it is structured browsing; an empty query (no word, no filter) returns nothing. No magic syntax, no quoted phrases (a whitespace-free CJK run is inherently one token, i.e. a contiguous substring).
- **Results and ordering** (deterministic): title hits (all tokens match the title) rank before body hits; within a group, rel_path lexicographic order — guaranteed by keeping the index entries sorted by rel_path, not by a sort call. Each entry = path + title + status badge + a context snippet around the earliest matched-token offset. No result limit in v0 (the corpus is small; any truncation is the panel's concern).
- The index is fully derived and in-memory (see `design.md` §6): it is rebuilt from the vault, and a ~2s mtime scan applies incremental updates (D21) — no fsnotify, no persistent store. Change detection is by mtime: the scan compares the current `{path → mtime}` set to the previous one and, on any change, rebuilds; it handles create/delete/rename uniformly. There is no content hash — at this scale a full rebuild is ~100 ms, and mtime avoids reading every file on every scan (reconsider past ~10k files).
- Semantic / vector: committed scope (D32) — Gemini embeddings + RRF fusion with the deterministic layer above; it composes with, never replaces, the lexical semantics of this section. Design, storage shape, and escalation ladders in `roadmap.md`.

**Acceptance**: a 2-character CJK query returns correctly; spot checks match `rg`'s results _on the note body_ (one-directional: what `rg` finds in the body text, kurodo finds too — `rg` also matching raw markup that `plain_text` strips is by design, not a bug); a literal `%` in a query matches a literal `%` (substring is literal, no wildcards); NFD-form content is found by an NFC-typed query; rebuild the index twice → byte-identical results; a concurrent read during an index swap is race-free (`-race`); a vault file change is reflected within one scan cycle — ≤3s worst case (a ~2s cadence plus the ~100 ms rebuild), so the bound is stably decidable.

## 4. The adjudication face (the only write)

**Premise (wall 3)**: the single source of the state machine = `internal/schema`, loaded at runtime from vault-schema.toml, with zero hardcoding in the repo. If the schema fails to load → the write face is **fail-closed**: show no transition keys, reject every `POST /status`; the reading face works as usual (the asymmetric fault tolerance of §0.1).

**The audit boundary (stated plainly)**: the `author=Koopa` audit claim is bounded by the **local trust boundary** — local processes under the same account (the browser, curl, an agent) are cryptographically indistinguishable, and kurodo does not over-engineer this with tokens. CrossOriginProtection blocks a browser's cross-site form POST, not local processes. Governance reinforcement: the vault-side agent-guides set a hard rule that "an agent never calls kurodo's write endpoint" (already added to `obsidian-cc-questions.md` §5).

**Ruled**: a flip **does not touch the `updated` field** (D16=(a)) — the semantics of `updated` are content freshness, and certifying does not revise understanding.

**Spec — the formal write-path algorithm**:

```
POST /status (path, from, to)
 1. Same-origin check (Go 1.26 http.CrossOriginProtection — any website can send
    a form POST to 127.0.0.1 without triggering a CORS preflight; this protection
    is the necessary deepening of wall 2)
 2. Read the file → split off the frontmatter → current status = from_actual
    · from_actual ≠ form.from → 409 "page out of date", no write
 3. schema.Transition(type, from_actual, to, "koopa") → on failure 422 with the reason, no write
 4. Pre-flight: the vault is a git repo and the file is clean
    · `git status --porcelain -- <file>` non-empty → abort with "this file has
      uncommitted changes; a flip would pollute the audit" (better to press once
      more than to pollute git history)
 5. Surgical rewrite: only within the frontmatter block, matching the `status:`
    line at line start
    · exactly one line → replace the whole line with `status: <to>`; zero or
      multiple lines → abort (a schema violation, left to kura / a human — kurodo
      does not fix files)
    · byte-identical outside that line; never re-serialize the YAML
 6. Atomic write-back (temp + rename, preserving permissions); before writing,
    stat and compare against step 2's mtime, abort on any change
 7. git add <file> && git commit -m "status(<relpath>): <from> → <to> (via kurodo)"
    · run at the vault root; set no author, so it = the vault git config (= Koopa's identity, D07)
    · commit failure → explicitly show "file changed, commit failed" + the raw text; no automatic rollback
 8. 302 → the reading page (PRG)
```

**Spec — UI**: the status panel lists the **currently legal** transitions (`schema` computes them with actor=`koopa`; show only the legal keys, never a disabled one); all legal transitions are open, and `ready` is the only primary style (D13); one form per key; the write path has zero JS dependency (D27 — JS may only add the seal's ~430 ms hold ceremony on top of a working plain form, and calls `form.requestSubmit()`, never `fetch`); no frontmatter (drills) → "No frontmatter (valid)" with no keys; broken YAML → show only a diagnostic and no keys (if the read is unreliable, don't write); schema load failure → show a "Contract unavailable" diagnostic and no keys (fail-closed).

**Known trade-off**: the dirty-file abort blocks the real flow of "halfway through reading, fix a typo in Obsidian → flip." v0 ships first; the two-step "commit the manual edit first, then flip" is decided by pain per D15.

**Spec — error vocabulary**:

| Scenario                             | HTTP | Presentation                                                      |
| ------------------------------------ | ---- | ----------------------------------------------------------------- |
| form.from ≠ current                  | 409  | Page out of date — reload and press again                         |
| illegal transition / owner-forbidden | 422  | The schema's rejection reason, verbatim                           |
| file dirty                           | 409  | Has uncommitted changes; a flip would pollute the audit           |
| status line zero or multiple         | 422  | Schema violation, left to kura / a human                          |
| mtime changed                        | 409  | The file was modified between read and write                      |
| git commit failed                    | 500  | File changed + the raw git text + manual-remediation instructions |

**Acceptance (automated)**:

1. Surgical precision: frontmatter with quoted values, comments, trailing whitespace, and `based_on: "[[...]]"` is, after rewriting, **byte-identical** except for the status line (golden comparison).
2. State-machine table-driven: full coverage of legal / illegal from→to / owner.
3. dirty file → abort and no write.
4. stale form → 409 and no write.
5. broken YAML → no keys; a direct POST is rejected.
6. Real git verification (temp git repo): the commit exists, the message format is correct, the author is taken from the repo git config, and the diff is exactly one line.
7. A cross-origin POST (`Sec-Fetch-Site: cross-site`) is rejected.

**Acceptance (manual, = the v0 shipping gate D10)**: 8. Koopa finishes a real long piece in `Writing/` and presses a legal transition; `git -C ~/obsidian log -1 --stat` shows that commit (author = Koopa, one file, one line); Obsidian confirms only the status changed. 9. When the current status has no legal transitions the panel shows no keys (the "No legal transitions" branch). Note: this contract keeps `archived` reachable from every status (`from = ["*"]`), so a valid note always offers at least that key — the no-keys branch is the panel's fail-safe, not a state a live vault file reaches; verify it against the panel logic (or a note whose contract yields an empty set), not against a `ready` file. Drills show "No frontmatter (valid)".

## 5. The judge and agent toolbox (kura inheritance face)

**Spec**: `kurodo check` (15 rules) / `exists` (dedup oracle) / `coverage` (MOC coverage). **Deployment shape = kura: a single binary, stateless file scanning, never touches the DB, no daemon dependency (§0.1)**.

**Command semantics follow kura, not reinvented**: `check [PATHS...]`'s positional arguments only filter output, the graph is always built for the whole tree; `--root` defaults to cwd; `exists`'s matching surface is deliberately wider than the resolver's (resolver keys + `title` / `title_en`), exit 0/1; `coverage`'s output is a single pretty JSON object (not JSONL), whose shape is defined by `conformance__coverage_report.snap`. The external interface is byte-compatible: JSONL field shape, `path→line→rule_id` ordering, the fingerprint (FNV-1a, `0x1f` separator, 16-digit lowercase hex), exit codes 0/1/2, `--deny <severity|rule>`, `--format json|human|md`, and the scan boundary (System/Diagrams/Views excluded by default, `--all`). **Load-bearing points (real consumers, verified on the vault side 2026-07-02)**: the grinder cron consumes JSONL directly with `grep '"severity":"warn"'` — the field names and the severity string values are an external contract; `--deny error`'s exit code is treated as a BAD flag by two crons; `--format md`'s report body is written by the vault QA cron, overwriting `System/reports/kura-vault-check.md`; `exists`'s exit 0/1 is the dedup answer. `--baseline` and `--all` currently have **zero real consumers** (verified absent) — kept byte-compatible anyway, but not a hot path.

**Egress exclusion**: `check` unconditionally excludes `Diary/` (not even `--all` includes it — findings written to a report will be read by agents; §0.1 privacy boundary, read from toml `[privacy]` once it lands).

**First batch of extensions (verified on the vault side; to be done after the byte-compat gate is met — the H face, expanded with graph relation verbs and whole-graph export per D33 / `roadmap.md`)**:

1. **frontmatter query** — structured queries (e.g. `type=lesson status=imported domain=golang`). Today this relies on `rg '^status:'` by hand plus manual intersection, with multiple vault skills each hand-rolling the same thing.
2. **`backlinks <note>`** — backlinks / blast radius. rg is blind to alias-mediated links (`[[Title|display]]`), and the resolver's alias table is exactly where the value is; a recurring scenario before retiring / renaming.

**Do-not-build list** (kura's field log has ruled these WATCH; kurodo does not take them on): the ruby-pairing check (zero real failures), the stray-tag check (root cause is in the hermes generation pipeline, fix it at the source). orphans are not missing — `coverage`'s three-tier classification (mounted / pending_mount / orphan) already covers it, no separate command.

**Acceptance (= the kura retirement gate; all six items are met — kura was declared retired 2026-07-07, D43)**:

1. kura conformance snapshots are byte-exact (`conformance__jsonl_output.snap`, `conformance__coverage_report.snap`). **Met** — the golden fixtures pin these bytes, and their comparison tests remain the enforced contract.
2. Against the real vault: `kurodo check` and `kura check` produce byte-for-byte identical JSONL. **Met** historically by the real-vault sandwich, which was deleted with the scaffolding once kura was retired; the golden fixtures (item 1) carry the byte contract forward, with no reference engine left to diff against.
3. schema.* rules follow vault-guard-spec §8 granularity (a vault-side document, under `~/obsidian/System/`): the (path, rule-class, field/value) sets are equivalent. **Met.**
4. **All real consumers switched over** — executed 2026-07-05: the four cron wrappers now invoke `kurodo`, each with a rollback backup written beside it. **Met.**
5. The judge's three commands run in an environment without PG (the CI environment is the proof). **Met.**
6. **The differential campaign reaches its completion bar** (`judge-plan.md` §13; added by D40): generated-vault differential fuzzing across both engines with zero unexplained byte differences. **Met** — the campaign ran across three independent runs to zero unexplained divergence, and the declaration cited §13.

With item 6 met, all six acceptance items hold; kura was declared retired on 2026-07-07 (D43), and the conformance scaffolding was then deleted while the goldens keep the contract.

## 6. The export face (yomihon inheritance face)

**Spec**: `kurodo export` = SSG static output (`dist/`), covering the Japanese lessons + the syllabus index + the five interactions (furigana visibility toggle, native details folding, TTS `data-tts` stripping `<rt>/<rp>` at build time, slot sidecar, concept `<dialog>`). Egress face: unconditionally excludes `Diary/` (§0.1 privacy boundary). PWA / Service Worker: **cut, not inherited** — yomihon's SW, being HTTP-only, never actually registered and is verified dead weight. export output = pure static files.

**The yomihon retirement gate — closed (D38 narrowed it, D40 closed it)**: the engineering item (the five interactions independently reproduced, all fixtures passing, direct consumption of `System/slots/L01–L20.yaml`) is merged; the two observation items (`m1-review/` screenshot parity, the two-week studying clock) are waived — Koopa moved daily reading to kurodo outright and does not track parity. Retirement is effective on his declaration alone; until then yomihon merely sits frozen (tag `v1.0.0`). Reading-surface problems found in daily use are ordinary UX work (`roadmap.md` §5b), not gate evidence.

**Acceptance of the export face itself** (own schedule, `roadmap.md` §1): the five interactions function in the static output, and `Diary/` is absent from `dist/`.

## 7. Global quality gates

- `make verify` (fmt→vet→lint→test→build) all pass; lint 0 issues; `go test -race -shuffle` all green. The fuller pre-push protocol (regeneration no-op, kill-tests, hygiene greps) is `standards.md` §5.
- The four walls have test locks: loopback-only, path-escape rejection, the write face touches only the status line, and the renderer never fixes a file (diagnostic types are read-only).

## 8. Rulings

**D16 = (a): a flip does not touch `updated`** (Koopa ruled, 2026-07-02, overruling the originally recommended (b)). Rationale: in a vault with provenance discipline, `updated` means "when this note's understanding was last revised" — and certifying precisely does not revise understanding; (b) would pollute the real signal of "content freshness" into "any touch," whereas the stale/superseded views rely on exactly that freshness. A flip's visibility already has a home: the git log, and the pipeline.base grouped by status. Wall 1 keeps zero annotations.

# kurodo functional spec (final form)

> **No milestone fences** (decisions D15): this document defines "what done looks like," not "what to build first." Each feature face has its own spec and acceptance criteria; implementation order is decided by the pain of use. The only ordering suggestion (not a fence): first wire up the single "finish reading → certify" keystroke (D10).
> The four walls (`CLAUDE.md`) override everything in this document. Status: **certified (2026-07-02, including review revisions: §0.1 dependency boundary, §4 fail-closed and the audit boundary, D16=(a))**.

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
- **Privacy boundary** (`System/agent-guides/Privacy-Boundary.md`, draft pending Koopa's final review): the top-level `Diary/` never egresses. kurodo is local-only, so **rendering it for Koopa himself is legitimate**; but every egress face — `export`, `check` (findings written to a report will be read by agents), any snapshot — unconditionally excludes `Diary/` (not even `--all` includes it, mirroring kura). Mechanical source: once toml `[privacy] never_egress_dirs` lands, read it from the toml, not hardcoded (wall 3).

---

## 1. The reading face

**Spec**:
- Full-vault rendering: the complete dialect table = `vault-model.md` layer 1 (the four wikilink states, embed, callouts (two buckets + English default titles), `==highlight==`, tasks, raw-HTML ruby passed through verbatim, mermaid; including the highlight and embed that yomihon lacks).
- The fault-tolerance rule: broken YAML / unknown callout / broken link → render anyway + diagnostic, never crash, never fix (wall 4).
- TOC: CJK-safe slug (following yomihon's `Slug()` semantics: keep `\p{L}\p{N}`, `-2` suffix on collision, `section` fallback).
- Diagnostics pane: broken YAML, unresolved / ambiguous link, corrections ledger — display only.
- Reports: `System/reports/` contains daily-briefing HTML, presented verbatim in a sandboxed iframe. Sandbox policy: **no `allow-same-origin`** — even with scripts allowed it lands in an opaque origin, forming a double layer of defense with the same-origin protection.
- `.base` / `.canvas` link back to open in Obsidian; D2 is not rendered (already decided for koopa0.dev).
- Code highlighting is server-side (chroma). JS principle: **zero frameworks, zero external JS dependencies; hand-written vanilla is allowed** (yomihon's five interactions, ~207 lines of hand-written JS, are the precedent).

**Body leading-H1 policy**: the page title comes from frontmatter `title`; a leading H1 in the body is removed so it isn't shown twice (following yomihon's already-verified behavior).

**Acceptance**:
- All dialect conformance tests pass (the structural-assertion pattern inherited from yomihon's `testdata/lesson.md`). Fixtures cover at least: ruby / `<br>` passed through verbatim; all callout types + `[!x]-` / `[!x]+` folding + unknown types degraded to blockquote; the four wikilink states + alias display + ambiguous marking; `![[embed]]`; `==highlight==`; task list; GFM tables (including escaped `\|`); dialect not processed inside a fence + a one-time warning; a broken-YAML single diagnostic that does not cascade; body leading-H1 removal; CJK slug aligned with the TOC anchor (including the `-2` collision suffix).
- Every `.md` in the real vault opens: zero 500s, zero blank pages (the mechanical definition of fault tolerance).
- Japanese lessons (L01–L20 + the P series) at visual parity with yomihon: the 14 screenshots in `m1-review/` are the baseline.

## 2. The navigation face

**Spec**: sidebar = lifecycle folders (vault order, top level ≤9) + the syllabus tree + a Reports section.

**Home-page spec** (finalized by the vault side's 2026-07-02 reply): four stable blocks — five domain MOC entry points, four cross-domain boards (reusing the **questions** the boards answer, not hardcoding campaign-flavored labels), the mechanical-gate list, and pointers to the governing documents. **The two-layer IA is not flattened**: cross-domain boards live on the home page; domain workspace views (e.g. `日本語課程.base`) hang under that domain's MOC block. The facts of the status vocabulary always trace back to the toml (that section of Vault-Index is a human copy). The v0 division of labor for `.base` (linking back to Obsidian) is confirmed on the vault side — reimplementing it = a second query engine that also inherits the drift of the hardcoded schema inside `.base`; if a frontmatter query lands later, the boards could derive a native rendering from the toml, to be revisited then.

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
- Semantic / vector: not scheduled; upgrading goes through the three gates (D05), and would be SQLite / sqlite-vec, not PostgreSQL (D24).

**Acceptance**: a 2-character CJK query returns correctly; spot checks match `rg`'s results *on the note body* (one-directional: what `rg` finds in the body text, kurodo finds too — `rg` also matching raw markup that `plain_text` strips is by design, not a bug); a literal `%` in a query matches a literal `%` (substring is literal, no wildcards); NFD-form content is found by an NFC-typed query; rebuild the index twice → byte-identical results; a concurrent read during an index swap is race-free (`-race`); a vault file change is reflected within one scan cycle — ≤3s worst case (a ~2s cadence plus the ~100 ms rebuild), so the bound is stably decidable.

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

**Spec — UI**: the status panel lists the **currently legal** transitions (`schema` computes them with actor=`koopa`; show only the legal keys, never a disabled one); all legal transitions are open, and `ready` is the only primary style (D13); one form per key, no JS; no frontmatter (drills) → "No frontmatter (valid)" with no keys; broken YAML → show only a diagnostic and no keys (if the read is unreliable, don't write); schema load failure → show a "Contract unavailable" diagnostic and no keys (fail-closed).

**Known trade-off**: the dirty-file abort blocks the real flow of "halfway through reading, fix a typo in Obsidian → flip." v0 ships first; the two-step "commit the manual edit first, then flip" is decided by pain per D15.

**Spec — error vocabulary**:

| Scenario | HTTP | Presentation |
|---|---|---|
| form.from ≠ current | 409 | Page out of date — reload and press again |
| illegal transition / owner-forbidden | 422 | The schema's rejection reason, verbatim |
| file dirty | 409 | Has uncommitted changes; a flip would pollute the audit |
| status line zero or multiple | 422 | Schema violation, left to kura / a human |
| mtime changed | 409 | The file was modified between read and write |
| git commit failed | 500 | File changed + the raw git text + manual-remediation instructions |

**Acceptance (automated)**:
1. Surgical precision: frontmatter with quoted values, comments, trailing whitespace, and `based_on: "[[...]]"` is, after rewriting, **byte-identical** except for the status line (golden comparison).
2. State-machine table-driven: full coverage of legal / illegal from→to / owner.
3. dirty file → abort and no write.
4. stale form → 409 and no write.
5. broken YAML → no keys; a direct POST is rejected.
6. Real git verification (temp git repo): the commit exists, the message format is correct, the author is taken from the repo git config, and the diff is exactly one line.
7. A cross-origin POST (`Sec-Fetch-Site: cross-site`) is rejected.

**Acceptance (manual, = the v0 shipping gate D10)**:
8. Koopa finishes a real long piece in `Writing/` and presses a legal transition; `git -C ~/obsidian log -1 --stat` shows that commit (author = Koopa, one file, one line); Obsidian confirms only the status changed.
9. A `ready` file's panel has no keys (no-legal-transition presentation is correct); drills show "No frontmatter (valid)".

## 5. The judge and agent toolbox (kura inheritance face)

**Spec**: `kurodo check` (15 rules) / `exists` (dedup oracle) / `coverage` (MOC coverage). **Deployment shape = kura: a single binary, stateless file scanning, never touches the DB, no daemon dependency (§0.1)**.

**Command semantics follow kura, not reinvented**: `check [PATHS...]`'s positional arguments only filter output, the graph is always built for the whole tree; `--root` defaults to cwd; `exists`'s matching surface is deliberately wider than the resolver's (resolver keys + `title` / `title_en`), exit 0/1; `coverage`'s output is a single pretty JSON object (not JSONL), whose shape is defined by `conformance__coverage_report.snap`. The external interface is byte-compatible: JSONL field shape, `path→line→rule_id` ordering, the fingerprint (FNV-1a, `0x1f` separator, 16-digit lowercase hex), exit codes 0/1/2, `--deny <severity|rule>`, `--format json|human|md`, and the scan boundary (System/Diagrams/Views excluded by default, `--all`). **Load-bearing points (real consumers, verified on the vault side 2026-07-02)**: the grinder cron consumes JSONL directly with `grep '"severity":"warn"'` — the field names and the severity string values are an external contract; `--deny error`'s exit code is treated as a BAD flag by two crons; `--format md`'s report body is written by the vault QA cron, overwriting `System/reports/kura-vault-check.md`; `exists`'s exit 0/1 is the dedup answer. `--baseline` and `--all` currently have **zero real consumers** (verified absent) — kept byte-compatible anyway, but not a hot path.

**Egress exclusion**: `check` unconditionally excludes `Diary/` (not even `--all` includes it — findings written to a report will be read by agents; §0.1 privacy boundary, read from toml `[privacy]` once it lands).

**First batch of extensions (verified on the vault side, the three gates passed; to be done after the byte-compat gate is met)**:
1. **frontmatter query** — structured queries (e.g. `type=lesson status=imported domain=golang`). Today this relies on `rg '^status:'` by hand plus manual intersection, with multiple vault skills each hand-rolling the same thing.
2. **`backlinks <note>`** — backlinks / blast radius. rg is blind to alias-mediated links (`[[Title|display]]`), and the resolver's alias table is exactly where the value is; a recurring scenario before retiring / renaming.

**Do-not-build list** (kura's field log has ruled these WATCH; kurodo does not take them on): the ruby-pairing check (zero real failures), the stray-tag check (root cause is in the hermes generation pipeline, fix it at the source), vector / semantic search (D05's three gates not passed). orphans are not missing — `coverage`'s three-tier classification (mounted / pending_mount / orphan) already covers it, no separate command.

**Acceptance (= the kura retirement gate)**:
1. kura conformance snapshots are byte-exact (`conformance__jsonl_output.snap`, `conformance__coverage_report.snap`).
2. Against the real vault: `kurodo check` and `kura check` produce byte-for-byte identical JSONL.
3. schema.* rules follow vault-guard-spec §8 granularity: the (path, rule-class, field/value) sets are equivalent.
4. **All real consumers switched over** — current state verified (2026-07-02): 4 cron wrappers (`cron-vault-wrapper.sh:132`, `cron-translator-wrapper.sh:91`, `cron-grinder-wrapper.sh:47`, `cron-vault-qa-wrapper.sh`) + the manual gates (QA-Gate tier 0, capture-source, share-rewrite, the file-editing quick gate recorded in kura-field-log) + obsidian CC usage. On the switchover day, a re-inventory is authoritative. Until then, not one line of kura changes.
5. The judge's three commands run in an environment without PG (the CI environment is the proof).

## 6. The export face (yomihon inheritance face)

**Spec**: `kurodo export` = SSG static output (`dist/`), covering the Japanese lessons + the syllabus index + the five interactions (furigana visibility toggle, native details folding, TTS `data-tts` stripping `<rt>/<rp>` at build time, slot sidecar, concept `<dialog>`). Egress face: unconditionally excludes `Diary/` (§0.1 privacy boundary). PWA / Service Worker: **cut, not inherited** — yomihon's SW, being HTTP-only, never actually registered and is verified dead weight. export output = pure static files.

**Acceptance (= the yomihon retirement gate)**:
1. The five interactions are independently reproduced and all fixtures pass (yomihon's testdata assertion pattern + direct consumption of `slots/L01–L20.yaml`).
2. `m1-review/` screenshots at visual parity.
3. Koopa actually studies with kurodo for two weeks. Until then, yomihon is frozen in service (tag `v1.0.0`).

## 7. Global quality gates

- `make verify` (fmt→vet→lint→test→build) all pass; lint 0 issues; `go test -race -shuffle` all green; `make verify-spec` all green.
- The four walls have test locks: loopback-only, path-escape rejection, the write face touches only the status line, and the renderer never fixes a file (diagnostic types are read-only).

## 8. Rulings

**D16 = (a): a flip does not touch `updated`** (Koopa ruled, 2026-07-02, overruling the originally recommended (b)). Rationale: in a vault with provenance discipline, `updated` means "when this note's understanding was last revised" — and certifying precisely does not revise understanding; (b) would pollute the real signal of "content freshness" into "any touch," whereas the stale/superseded views rely on exactly that freshness. A flip's visibility already has a home: the git log, and the pipeline.base grouped by status. Wall 1 keeps zero annotations.

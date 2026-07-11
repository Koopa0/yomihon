# Vault Model — the builder's guide to this Obsidian vault

> Required reading before you touch the renderer / graph / search. This document describes the dialect, semantics, and authority structure of one **specific vault** (`~/obsidian`) — it is not a general Obsidian tutorial.
> In this document, "yomihon-dev" = the old reader, frozen in service (`~/go/src/github.com/koopa0/yomihon-dev`, a reference implementation); "yomihon" = this project.
>
> Facts anchored on 2026-07-02, all verified against real files. The vault is alive: the scale numbers will change, but the contract layers (layers 2 and 3) evolve only through vault-schema.toml.

---

## Layer 1: The Obsidian dialect (what no Markdown parser gives you for free)

### Wikilinks (the most important section)

Four syntactic forms: `[[name]]`, `[[name|display text]]`, `[[name#heading]]`, `[[name^block]]`.

The reference spec for resolution semantics is kura's `src/graph.rs` + `src/wikilink.rs`; yomihon must copy it rule by rule:

- **Normalize**: `trim → Unicode NFC → lowercase`. NFC is mandatory for CJK (macOS filenames use NFD; kura also NFC-normalizes paths as it walks — see `vault.rs`).
- **Key set**: every note is keyed under four forms — the filename stem, the full name with `.md`, the vault-relative path stem, and the full relative path — **plus its frontmatter `aliases`**.
- **`title` is never a key.** This is the reason kura's killer rule `link.title_not_alias` exists: a link written against a note's title breaks silently in Obsidian.
- **Stripping order**: first `|` (display text), then `#` (heading), then `^` (block). Escaped `\|` inside tables must be handled (strip the trailing `\` at the end of the pre-pipe segment). A `[[#heading]]` that strips down to empty → a same-file jump; skip it outright.
- **Anchors are never validated**: `[[X#h]]` counts as resolved as long as X exists; whether the heading/block exists is not checked.
- **Name collisions → Ambiguous; no guessing**: when one normalized name maps to multiple paths, list them all (sorted); never pick one. Same-named files in different folders collide.
- **Non-Markdown resources** (canvas / pdf / images) are keyed by filename and path *with* the extension — Obsidian requires the extension when linking to non-note files.
- **Scan boundaries**: `[[...]]` inside a code span / fenced block or an `%%…%%` Obsidian comment does not count as a link; a wikilink never spans lines.

### Embeds: `![[...]]`

Transclusion embeds; may appear for images, notes, or PDFs. Note: yomihon-dev's parser **does not handle** embeds (the leading `!` survives as literal text) — yomihon must add this; it is one of the known gaps in the reference implementation.

### Callouts: `> [!type]`

yomihon-dev has a complete conversion honed on real lessons; copy its semantics:

- Fold markers: `[!type]-` → a `<details>` **collapsed** by default; `[!type]+` → an expanded `<details>`; no suffix → a static tinted alert.
- Type mapping (two color buckets: `note` = sky, `warning` = amber) plus default English titles:

| Callout type | Bucket | Default title |
|---|---|---|
| info / note / tip / hint / abstract / summary / todo | note | Note |
| question / help / faq | note | Question |
| example / quote / cite | note | Example |
| warning / caution / attention | warning | Warning |
| danger / error / bug / fail / failure / missing | warning | Danger |

- **Unknown type → degrade to a plain `<blockquote>` plus a log warning; never crash** (the fault-tolerance rule).
- Translation and answer folds use native `<details>`: zero JS, offline-safe, good accessibility.

### Highlights: `==text==`

yomihon-dev **does not handle** this — yomihon must, since you'll hit it when rendering the whole vault.

### Tasks: `- [ ]` / `- [x]`

GFM task list; covered by goldmark `extension.GFM`.

### YAML frontmatter

A structural layer, not decoration (see layer 2). Fault-tolerance requirement: bad YAML emits exactly **one** diagnostic (aligned with kura's `schema.frontmatter`, no cascade); one note must never wreck the whole render. Split out the frontmatter before running any body preprocessing (yomihon-dev's `splitFrontmatter` lesson: otherwise a value like `based_on: [[...]]` gets mangled by the wikilink pass).

### Raw HTML in the body

Japanese lessons hand-write whole passages of `<ruby>…<rt>…</rt></ruby>` and `<br>`, which **must pass through verbatim, unsanitized** (goldmark `WithUnsafe`). This holds only under a trusted corpus plus local-only — one of the reasons wall 2 exists. Do not set automatic hard line breaks; let explicit `<br>` take effect.

Example: `Writing/lessons/japanese/L20 普通形と常体.md:141` (katakana gets ruby too); particles are annotated with their actual reading — `<ruby>は<rt>わ</rt></ruby>`, `<ruby>を<rt>お</rt></ruby>`.

### Code-fence safety

Line-oriented preprocessing (the callout / wikilink / table passes) doesn't understand fences. The policy inherited from yomihon-dev: when `[[...]]`, `> [!…]`, or a pipe-table is detected inside a fence, emit a single build warning rather than silently corrupting the output.

### Non-Markdown files

| Kind | Reality | Handling |
|---|---|---|
| `.base` (Obsidian Bases) | `Views/` has 5 (knowledge-overview, needs-attention, pipeline, share-rewrite, 日本語課程) | v0 links back to open in Obsidian; **do not implement a query engine** |
| `.canvas` | Exactly 1: `Diagrams/canvas/DDIA-Ch1-Overview.canvas` (JSON) | Same as above — link back or thumbnail |
| `.d2` | 2 under `Diagrams/d2/` | **Do not render** — koopa0.dev already decided the 7.8 MB WASM isn't worth it; don't overturn that |
| mermaid code fence | Used in Writing/ | Render (client-side; already aligned with koopa0.dev) |
| `System/reports/daily-briefing/*.html` | hermes daily briefings | Serve as-is (trusted corpus) |

---

## Layer 2: This vault's semantic model (more important than the dialect)

### Scale (2026-07-02 snapshot)

419 `.md` files across the vault: Writing 163, Concepts 120, Sources 88, System 36, Maps 11, Synthesis 1, Inbox 0. Search and indexing are designed on the assumption of a low-thousands corpus that agents keep growing.

### Folders = lifecycle, not classification

The main flow: `Inbox → Sources → Concepts → Maps / Synthesis → Writing`; `System/` = governance layer, `Views/` = dashboards, `Diagrams/` = diagrams. Hard rule (Vault-Architecture.md): **≤ 9 top-level folders, domains one level deep, no subfolders** — navigation runs through MOCs and `topics`, not a tree. yomihon's sidebar should reflect this model rather than invent a tree of its own.

### The four hard rules (Note-Schema.md)

1. Properties carry structured data (type / status / domain / topics / provenance).
2. Tags are for cross-domain retrieval only; they never encode type / status / domain.
3. Folders express artifact lifecycle, not a full topical taxonomy.
4. `type` = kind of artifact, `status` = lifecycle stage — the same fact is never double-encoded.

### The frontmatter data model

- Required (knowledge notes): `title, aliases, type, domain, topics, status, created, updated`; Inbox is exempt from `domain`; the four types `system / guide / template / research-brief` are exempt from domain.
- `type` has 20 values, `domain` has 13 (golang, rust, japanese, meta, …) — **enums are always governed by the toml; never copy them into code** (wall 3).
- Source-type notes additionally have five provenance fields (`source_kind / source_provider / source_work / source_locator` plus `based_on`); a concept must have either `based_on` or `source_locator` (kura `schema.provenance`).
- Lesson-specific fields: `slug, title_en, version_sensitive, objectives_count, assessment_item_count, corrections, evolution_predecessor, evolution_successors`.
- The `domain` value must equal the name of the folder the note lives in (kura `schema.domain_folder`).
- The schema is **closed**: any key outside `fields.known` is an error.

### Instance authority is separate from frontmatter

The same toml declares which note types become navigation structures and which
vault directories hold readable artifacts rather than governed note instances:

```toml
[navigation]
path_types = ["study-path"]
map_types = ["moc", "source-map", "topic-map"]

[artifacts]
non_instance_dirs = ["System/templates"]
```

`path_types` and `map_types` are disjoint, duplicate-free subsets of
`[enums].type`. `non_instance_dirs` uses NFC-normalized, vault-relative component
prefixes: `System/templates` matches itself and descendants, never
`System/templates-old`. All three keys are required when their section exists;
an explicit `=[]` is valid authority declaring an empty set, while an omitted
key makes that capability unavailable. Navigation and artifact validation are
independent.

A non-instance file is still readable through direct/raw routes, Folders,
bare-text search, and `folder:` search. It has no instance metadata, lifecycle,
status-write, lesson, Home-recent, or governed navigation identity. If a study
path names one uniquely, its source-order row survives only as a non-link
`non-instance` warning with no status, placement, or ready credit; general maps
omit that row.

### status is a grouped state machine (not one flat enum)

The toml's `[fields.status_group]` maps types into three groups:

- **note group**: captured → cleaned (sources); seedling → growing → evergreen (concepts); draft → ready (writing)
- **lesson group**: imported → draft → ready
- **system group**: active / archived
- `archived` can be entered from any state (`from = ["*"]`).

The toml's `[[lifecycle]]` table declares, for each status, its `from` (legal prior states) and `owner` — **the owner of `ready` is `koopa`; any agent writing ready is a violation**. yomihon is a single-user local app whose operator *is* Koopa, so a ready button in the UI is legal; but illegal from→to transitions must be blocked.

Key fact: the toml comments admit that a file-scan tool (kura) cannot see the *previous* state, so it validates only enum + owner, and from→to enforcement is deferred. **yomihon is an interactive writer that reads the current state before writing, so it can naturally enforce the full from→to + owner check** — this is yomihon's first substantive contribution to the contract, not a repeat of kura's work.

### slug

Only lessons need one. Pattern `^[a-z0-9]+(-[a-z0-9]+)*$` (built into the toml); namespace prefixes: Japanese `jp-minna-lNN` / `jp-kana-pNN`, Go lessons plain, one rust lesson `rust-ownership`. **Once a slug is finalized it never changes; the filename may.** Policy note: `System/schemas/Slug-Policy.md` — the prefixes are a minting convention living at the doc layer; the toml only validates the pattern. If yomihon later needs to consume the prefixes mechanically, propose adding them to the toml then. Today all 147 lessons carry a slug and all are compliant (2026-07-02 verified).

### The syllabus (study-path) is machine-parseable

- `Maps/Go 課綱.md`: H2 = part, H3 = module, both in the pipe format `slug | English | Chinese`; list items = lessons (wikilinks); row order = sequence.
- `Maps/大家的日本語 初級I 學習路徑.md`: **the structure differs slightly** — under the H2 「課程序列」, the H3s are learning stages (解碼期, 動詞入門, …) and list items are lessons. The two invariants — list item = lesson, row order = sequence — hold for both, but do not assume the two syllabi are isomorphic.
- The current parser also preserves direct wikilink list rows under an H2 before
  any H3 subgroup; task checkboxes and rows without wikilinks are not path
  entries. Do not narrow the second path back to a single named H2.
- When rendering a syllabus page, this structure *is* the navigation tree.

### Japanese-material specifics

- Two series: `L01`–`L20` (the main grammar lessons) plus `P01`–`P07` (the kana prerequisite lessons), all `type: lesson, domain: japanese`.
- **The drills (`Writing/lessons/japanese/drills/`, 8 files) having no frontmatter is intentional and legal** (toml `no_frontmatter_is_legal`; both kura and Bases exclude them) — treat them as attachment-level content; don't require a schema.
- The division of labor: the vault owns **understanding** (the P series, the grammar lessons), the Kotonoha app owns **reflex** (kana/kanji drills) — drill-style interaction never grows into yomihon.
- Orthography rules (Japanese-Companion-Guide.md): furigana may fade, **katakana always gets ruby and never fades**, particles are annotated with their actual reading, strict level-gating.

### golang-lesson specifics

Revised lessons carry a **corrections ledger**: a frontmatter `corrections:` list, each item `{claim / fix / source}` (e.g. `Writing/lessons/golang/Garbage Collection Guide.md`). Worth surfacing when rendering — it is the audit face of "what this material has had corrected."

---

## Layer 3: Authority and governance (yomihon's place in the ecosystem)

### vault-schema.toml is the machine source of truth

`System/schemas/vault-schema.toml` (schema_version 1) declares itself the SoT; its consumers are kura (the schema.* rules), `gen_fileclasses.py`, Note-Schema.md (the human doctrine, "change the toml before you change an enum"), and yomihon. yomihon only reads it, never hard-codes (wall 3).

### kura is the corpus judge (15 rules)

7 link/graph rules (`link.title_not_alias`, `link.broken`, `link.broken.path`, `collision.alias`, `provenance.unresolved`, `map.disk_mismatch`, `map.disk_unlisted`) plus 8 schema.* rules (enum / required / unknown_key / slug / domain_folder / legacy_tag / provenance / frontmatter, all at error level). Gate semantics: `--deny error`; info never gates.

When yomihon sees something broken: **render it anyway and flag a diagnostic; don't fix, don't block, don't overstep** (wall 4).

### The pipelines (the real consumers of `check`, verified vault-side 2026-07-02)

- `cron-vault-wrapper.sh:132` — `kura check --root "$WT" --deny error` (worktree self-check)
- `cron-translator-wrapper.sh:91` — `kura check "${FILES[@]}" --root "$VAULT" --deny error` (explicit file arguments)
- `cron-grinder-wrapper.sh:47` — `--format json` piped into `grep '"severity":"warn"'` (**the JSONL field names and severity string values are therefore an external contract**)
- `cron-vault-qa-wrapper.sh` — `--format md` written out, overwriting `System/reports/kura-vault-check.md`
- Manual gates: QA-Gate layer 0, step 6 of the capture-source skill, all-green before share-rewrite's final review, and a quick gate after every lesson edit (kura-field-log:32)
- `kura exists` = the dedup oracle before creating a concept (exit 0/1 is the answer); `kura coverage` = the orphan/routing watchdog (`Maps/研究 Brief 索引.md:15` depends on it explicitly)
- `--baseline` and `--all`: **zero real consumers** (verified absent) — carried over for byte-compat, not a hot path

The JSONL contract (the retirement gate's golden comparison target, field shape): `rule_id, severity, path, line?, field?, message, evidence, suggested_action, source_rule, target?, resolved_to?, collision_members?, fingerprint`. Sorted `path → line → rule_id`; the fingerprint = FNV-1a over (rule_id, path, target), each segment followed by a `0x1f` separator byte, 16-digit lowercase hex; exit codes 0 / 1 / 2. Byte-exact targets: `kura/tests/snapshots/conformance__jsonl_output.snap` and `conformance__coverage_report.snap`.

### Scan boundary ≠ render boundary

By default kura **does not scan** `System/`, `Diagrams/`, `Views/` (only `--all` does). yomihon's render surface is larger than kura's scan surface (it must read reports and briefings); but when aligning a `yomihon check`, it must replicate kura's scan boundary, or the JSONL won't line up.

### git is the audit layer

The vault is a git repo. yomihon makes one commit per status transition (wall 1) — this keeps everything reversible and is the precondition that lets the yard be opened wide. **Do not build a separate status-history table: `git log` is the history.**

### The write pipeline at a glance

hermes goes through a worktree branch → the three QA-Gate layers (kura → Codex → Claude) → only Claude merges → **only Koopa presses ready**. yomihon's status flip is this chain's human-terminal interface, not a bypass.

### The privacy line (a dedicated doc is drafted: `System/agent-guides/Privacy-Boundary.md`, pending Koopa's final review)

- The line is a **folder**: the top-level `Diary/` never egresses (a folder is fail-closed, a frontmatter flag is fail-open, so use the folder).
- For yomihon: local-only rendering for Koopa himself is **legal**; every egress surface (export, check findings, snapshots, feeding an agent) unconditionally excludes `Diary/` — not even `--all` includes it, because findings written into a report would be read by agents.
- The mechanical source: the toml will add `[privacy] never_egress_dirs = ["Diary"]` — kura, yomihon, and hermes all read the same one (wall 3); no hard-coding.
- The `type: diary` draft: lives in `Diary/`, domain-exempt, does not require a status. The boundary test is a single sentence: **if you want an agent to look at it, it isn't a diary** (a Japanese diary-writing exercise is not a diary).

---

## Layer 4: The builder's reading order (real files, not paraphrase)

1. **Data model**: `System/schemas/vault-schema.toml` → `System/schemas/Note-Schema.md` → `System/schemas/Vault-Architecture.md`
2. **System philosophy and the judge's spec**: `System/Vault-Index.md` → `System/Koopa-Knowledge-Compiler.md` → `System/vault-guard-spec.md` (note: the spec's filename is still the old name; the tool has been renamed kura) plus `System/kura-field-log.md`
3. **The people, the division of labor, the gates**: `System/agent-guides/about-koopa.md` → `collaboration-charter.md` → `QA-Gate.md` → `Japanese-Companion-Guide.md` → `Privacy-Boundary.md` (draft) plus `System/schemas/Slug-Policy.md`
4. **The three reference implementations**: `~/go/src/github.com/koopa0/yomihon-dev/internal/markdown/parser.go` (dialect handling) plus `kura/src/graph.rs`, `src/wikilink.rs` (the link-resolution spec) plus `~/koopa0.dev/frontend/src/app/core/services/markdown.service.ts` (an existing component; untrusted premise, a different context)
5. **A sampling of real content (read the real files before writing your first line of rendering code)**:
   - `Writing/lessons/japanese/L20 普通形と常体.md` (an HTML-ruby lesson)
   - `Writing/lessons/golang/Garbage Collection Guide.md` (a corrections ledger)
   - `Maps/Go 課綱.md` plus `Maps/大家的日本語 初級I 學習路徑.md` (the two syllabus structures)
   - `System/reports/kura-vault-check.md` plus `System/reports/daily-briefing/latest.html` (the report surface)

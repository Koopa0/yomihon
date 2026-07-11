# Search face — implementation plan (spec §3)

> Status: **built and merged** (the lexical engine, /search page, and ⌘K shell are on main); kept as the record of the plan. The hybrid extension is D32 / roadmap.md, not this document.
> This refines `spec.md` §3 and `design.md` §6–7 into a concrete plan. The
> engine is **in-memory, no database** (D24); the index is one of three models
> in a shared vault Snapshot (D25); incremental freshness is a ~2s mtime scan
> (D21). The four walls (`CLAUDE.md`) and the dependency boundary (`spec.md`
> §0.1) govern everything below.

## 1. Shape, in one line

An in-memory index of `~419` notes (a few MB), rebuilt from the vault inside
the shared Snapshot, queried by a deterministic NFC-folded substring match plus
six structured filters. No database, no new dependency, no daemon, no Docker in
the tests.

## 2. Where it lives (D25 — the shared Snapshot)

`internal/search` owns the index and query; it does **not** own freshness.
`internal/snapshot` holds `Snapshot{Graph *graph.Index, Nav *nav.Model, Search
*search.Index, ArtifactPolicy schema.ArtifactPolicy}` behind an
`atomic.Pointer[Snapshot]` and runs one scanner that, about every 2 seconds,
`stat`-walks the vault and — on any mtime or file-set change — rebuilds all three
and swaps the pointer once. Navigation roles and artifact policy are derived once
from the startup schema and reused on every rebuild; a rescan never re-reads the
contract. The search handler reads the pointer once per request and queries
`snap.Search`. This closes the existing gap where graph/nav were built once at
startup and never refreshed (D25).

## 3. The index (D23 — only what search reads)

Per note, in memory:

```go
type entry struct {
    RelPath    string   // primary key; entries kept sorted by this
    Title      string   // original case (for display)
    TitleFold  string   // fold(Title) — for matching
    NoteType   string
    Domain     string
    Status     string
    Slug       string
    Topics     []string
    PlainText  string   // original case (for snippet)
    PlainFold  string   // fold(PlainText) — for matching
    metadataCapable bool // governed instance metadata, not merely readable text
}
type Index struct {
    entries []*entry // sorted by RelPath at build time
    metadataAvailable bool
    metadataDiagnostic string
}
```

No link structure (future backlinks, design §10), no raw frontmatter. Every
readable note stays in the text corpus; `metadataCapable` only governs whether
its frontmatter may answer instance-metadata projections. The `fold` copies
double the text in memory (a few MB) to buy a zero-config match; worth it.

## 4. The single match definer — `fold` (determinism's core)

```go
func fold(s string) string { return strings.ToLower(norm.NFC.String(s)) }
```

- **NFC on the Go side, at index time and query time.** `Title`/`PlainText` are
  stored NFC (original case); `TitleFold`/`PlainFold`/the query token are folded.
  Without NFC on *both* sides, NFD-form vault content is unfindable.
- **Case folding lives only here** (Go `strings.ToLower`), so "what counts as a
  match" has exactly one definer. `internal/graph`'s `normalize` also folds, but
  it is trim+NFC+**lower**; search needs NFC-only for the stored display copy, so
  export graph's NFC step as a shared primitive (e.g. `graph.NormalizeNFC`) and
  let both reuse it — do **not** write a second NFC (predictable-mistake #2).
- Accepted, noted-not-engineered: snippet offsets are located on the folded copy;
  a rare non-length-preserving fold (Turkish İ, etc.) could shift a boundary. This
  vault is zh/ja/en; clamp the snippet bounds and leave a one-line comment.

## 5. Query semantics (pinned — tests assert exactly this)

- Whitespace tokenization; each bare token matched by `strings.Contains(PlainFold
  or TitleFold, fold(token))`; multiple tokens = **AND**. No quoted phrases (a
  whitespace-free CJK run is already one token = a contiguous substring).
- Substring is **literal** — there are no wildcards, so a query `%` matches a
  literal `%` (this is free with `strings.Contains`; a test asserts it).
- **Six fixed filter keys**, literal equality, no enum validation:
  `type:` `status:` `domain:` `slug:` (equality on the field), `topic:`
  (single-value containment: the token is an exact element of `Topics`),
  `folder:` (rel_path prefix at a `/` boundary: matches `RelPath == v` or
  `strings.HasPrefix(RelPath, v+"/")`, so `folder:Writing` does not match
  `Writing-old/`).
- A repeated filter key = **AND** (`topic:a topic:b` requires both). Documented so
  it is not "last wins" by accident.
- A **pure-filter** query (no bare token) is legal — structured browsing. An
  **empty** query (no token, no filter) returns nothing (handled before scanning).

### 5a. Capability split — text/path truth versus instance metadata

- Bare terms and `folder:` use readable content and vault paths. They remain
  available when artifact policy is unavailable and include non-instance files.
- `type:`, `status:`, `domain:`, `topic:`, and `slug:` use instance metadata.
  With valid policy they exclude non-instance entries. With missing, invalid, or
  incomplete policy, any query containing one of these filters returns the
  policy diagnostic as `ErrMetadataUnavailable`; a mixed metadata-and-text query
  does not silently discard its filter or pretend there were zero matches.
- An explicit empty `non_instance_dirs = []` is valid policy and therefore keeps
  metadata search available. Omitting that required key is not equivalent.

### 5b. Parse rules — pinned (Koopa, 2026-07-03); the table below is the acceptance basis

- **R1 — classify before folding.** Whether a raw token is a filter is decided on
  the *original* text: `Type:lesson` / `TOPIC:x` have a key that is not one of the
  six lowercase keys → the whole thing is a bare token (folded *afterwards*).
- **R2 — split on the first colon.** `slug:a:b` → key=`slug`, value=`a:b`.
- **R3 — filter values are NFC, not case-folded.** A bare token goes through
  `fold` (NFC+lower); a filter value is NFC only (both sides: the index stores the
  field values NFC too). This refines "literal equality": free-text search is
  case-insensitive (prose), an enum filter is an exact selection of a canonical
  (lowercase-by-schema) value. Confirmed by Koopa.
- **R4 — `folder:` value drops one trailing slash.** `folder:Writing/` ≡
  `folder:Writing`; an empty value follows the literal formula (matches nothing,
  deterministically — no special case).

```go
type Query struct {
    Tokens  []string // folded bare tokens, in input order
    Filters []Filter // in input order; a repeated key = AND
}
type Filter struct{ Key, Value string }
```

Parser case table (hand-computed expected values):

| # | Input | Tokens | Filters | Note |
|---|---|---|---|---|
| 1 | `""` | — | — | handler returns empty before scanning |
| 2 | `"   \t "` | — | — | whitespace-only = empty query |
| 3 | `kafka` | `[kafka]` | — | |
| 4 | `Kafka` | `[kafka]` | — | fold |
| 5 | `深度 工作` | `[深度, 工作]` | — | two tokens, AND |
| 6 | `が` (NFD が) | `[が]` (NFC) | — | NFC fixture |
| 7 | `type:lesson` | — | `(type,lesson)` | |
| 8 | `深度 type:lesson 工作` | `[深度, 工作]` | `(type,lesson)` | token order preserved |
| 9 | `topic:a topic:b` | — | `(topic,a)(topic,b)` | repeated key = AND |
| 10 | `type:a type:b` | — | two `type` filters | match layer: unsatisfiable → empty result, not an error |
| 11 | `folder:Writing/` | — | `(folder,Writing)` | R4 drops trailing slash |
| 12 | `slug:a:b` | — | `(slug,a:b)` | R2 |
| 13 | `foo:bar` | `[foo:bar]` | — | not one of the six keys → bare token |
| 14 | `Type:lesson` | `[type:lesson]` | — | R1: classify before fold |
| 15 | `status:` | — | `(status,"")` | literal empty value — matches notes with empty/absent status |
| 16 | `%` | `[%]` | — | literal, no wildcard |
| 17 | `100%` | `[100%]` | — | |
| 18 | `domain:日本語` (NFD value) | — | `(domain,日本語)` (NFC) | R3 |
| 19 | `slug:ABC` | — | `(slug,ABC)` | R3: filter value not case-folded |
| 20 | `folder:` | — | `(folder,"")` | R4: deterministically matches nothing |

Match/ordering cases (six filters, 2-char CJK, two-bucket order, folder boundary,
NFD content, rebuild-twice) are in §10; the table above adds only #10 and #20 as
match-layer semantics.

## 6. Results and ordering (deterministic by construction)

- Two buckets: **title hits** (every token is in `TitleFold`) and **body hits**
  (every token is in `PlainFold`, and not already a title hit). Because `entries`
  is kept sorted by `RelPath`, each bucket is naturally in rel_path order — concat
  (title bucket, then body bucket) is the final order. Ordering is guaranteed by
  the data structure, not a sort call.
- Each result = `RelPath` + `Title` + a snippet centered on the earliest
  matched-token offset in `PlainText`; the `Status` badge appears only for a
  metadata-capable governed entry.
- No result limit in v0 (small corpus); truncation, if ever, is the panel's job.

## 7. `plain_text` extraction (pinned — otherwise "match `rg`" is undecidable)

Reuse `internal/render`'s goldmark AST (do not write a second parser — D04's
spirit); walk it collecting text:

| Content | Into `plain_text`? |
|---|---|
| frontmatter | no (structured fields are separate) |
| body text, headings, tables, task text | yes |
| wikilink `[[Target\|display]]` | both target and display (searching a filename should hit) |
| code fence contents | yes (you search for code snippets) |
| ruby: base + rt | both (searching the kana reading should hit) |
| HTML tags themselves, callout marker syntax | no |

Acceptance is **one-directional**: whatever `rg` finds in the note *body*, yomihon
finds too. `rg` also matching raw markup characters that `plain_text` strips is by
design, not a bug.

## 8. Building the index (inside the Snapshot rebuild)

- Change detection is by **mtime**: each ~2s scan `stat`-walks the vault (no file
  reads) and compares the current `{path → mtime}` set to the previous one. No
  change → do nothing. Any change → rebuild. There is **no content hash**: a full
  rebuild is ~100 ms at this scale, and hashing would force reading every file on
  every scan — mtime is both simpler and cheaper (reconsider past ~10k files).
- On a change, the Snapshot rebuilds all three models by **three independent
  walks** — `graph.Build`, `nav.Build`, `search.Build(root, artifactPolicy)` —
  not one shared pass. Search indexes non-instance documents for bare-text and
  `folder:` lookup while marking them ineligible for metadata projections.
  Rationale (D25 accepts it): a shared pass would couple the three packages'
  build APIs through a common intermediate type, while three walks of ~437 files
  stay well inside the ≤3s freshness bound and cost nothing while the vault is
  idle (only a `stat`-walk runs then). The one semantic cost: the three walks see
  three moments of the filesystem, so a note edited mid-rebuild can be represented
  inconsistently across the models *within one snapshot*. This self-heals within
  one scan cycle — the mtime set is captured before the rebuild, so a mid-rebuild
  edit is not in `prev` and the next scan rebuilds. Skew bound = one cycle.
- Delete/rename fall out of the full rebuild for free (the Snapshot is rebuilt
  wholesale, D25) — no per-note delete bookkeeping.

## 9. Handler

`internal/search` exposes a query function over an `Index` and an HTTP handler
that reads the current Snapshot, parses the query, and returns the ordered
results. Business logic stays in `search`; the handler is parse-call-render.
Metadata capability errors render the artifact-policy diagnostic rather than an
empty result set; text and folder queries do not inherit that dependency. A
minimal plain results page (like the nav sidebar) exercises it end to end. The
live `/search` and ⌘K presentation is outside this engine plan and is
governed by `docs/ux-plan.md`; Home deliberately remains a plain GET form.

## 10. Testing — all pure unit tests (a dividend of dropping PG)

No Docker, no testcontainers, no `//go:build integration`.

- Query parser (table-driven, hand-computed): tokenize; the six keys; `%`/`_`/`\`
  as literals; NFC (NFD-input fixture); pure-filter; empty query; repeated key.
- Match/results: each filter; a 2-character CJK query; multi-token AND; title-
  before-body then rel_path order; a `%`-literal query; NFD-form content hit;
  the `folder:` `/`-boundary (`folder:Writing` excludes `Writing-old/`; define and
  test `folder:Writing/` with the trailing slash too).
- Artifact capability: valid policy excludes a non-instance from every metadata
  filter and aggregate while retaining it for bare text and `folder:`; missing,
  invalid, and present-but-incomplete policy reject pure and mixed metadata
  queries with their exact diagnostic while text/folder queries stay available;
  explicit empty policy remains available.
- Determinism/concurrency: **rebuild the index twice → `cmp.Diff` identical**; a
  concurrent read during a Snapshot swap is race-free under `-race`.
- A real-vault test (`t.Skipf` when `~/obsidian` absent): the index builds, a
  known term hits an expected note.

## 11. Suggested implementation order (dependency order, not a fence)

1. Export `graph.NormalizeNFC`; add `fold` to `search`.
2. `plain_text` extraction over the render AST (+ its extraction-table tests).
3. The `Index` type + build-from-notes (test against the real vault, eyeball).
4. The query parser (pure function, fully tested).
5. The query/match + two-bucket ordering.
6. `internal/snapshot`: the `Snapshot`, the `atomic.Pointer`, the ~2s scanner
   feeding graph + nav + search; wire `main.go` to read the pointer (this also
   makes graph/nav live — D25).
7. The search handler + minimal plain results page.

Each step has an independently verifiable output. All UI/error/comment text is
English (D19), including "no results".

## 12. Cleaned up when PostgreSQL was dropped (D24)

Removed so a future session cannot reinstate PG from stale config: `sqlc.yaml`,
the Makefile `sqlc` target, the empty `migrations/`, `YOMIHON_DB` (D12), and the
PG lines in `CLAUDE.md` Facts / `design.md` §2 stack. The go-spec harness rules
under `.claude/rules/` still describe pgx/sqlc generically (they are shared
reference, not a yomihon claim). Database posture has since moved on: adoption
is a per-feature engineering call (D31), with escalation ladders in
`roadmap.md` §4.

## 13. Open decisions — resolved

1. Deterministic-only v0: **confirmed** (Koopa, 2026-07-03).
2. Engine: **in-memory** (D24); SQLite is the recorded upgrade path with a
   mechanical trigger; PostgreSQL dropped.
3. No `YOMIHON_DB` / DSN (in-memory) — settled.

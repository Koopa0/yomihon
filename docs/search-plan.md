# Search face — implementation plan (spec §3)

> Status: Part I (the lexical engine, /search page, ⌘K shell, and the live
> lexical-results enhancement) is **built and merged**; it is kept as the
> record of that plan. Part II (§§H1–H12) is the hybrid extension's plan,
> **revised 2026-07-12 to the ruling sheet (D50)** after its adversarial
> round returned RETHINK. Dispatch gate (D50.10): the vault contract's
> privacy capability must land before any Part II behavior — cloud document
> embedding, ranking/fusion, or agent-facing output. A delta-focused second
> round reviews this revision before build.
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

---

# Part II — the hybrid extension (D32 as amended by D50; revised 2026-07-12)

## H1. Shape, in one line

A second retrieval channel — `gemini-embedding-2` embeddings (D50.9; the
prior generation's retirement window opened before build, and embedding
generations are incompatible, so the corpus embeds once on the new model)
over heading-bounded chunks, exact cosine top-k in memory — fused with the
shipped lexical channel by RRF; semantic is strictly an enhancement layer,
and every surface stays whole without it (roadmap §4a).

## H2. Boundaries and the two corpora

- The lexical grammar, its filters, and their instance-contract capability
  split are frozen as shipped; hybrid adds a channel, never a syntax. The
  `source_kind` filter moved to the H face (D50.4) — this plan touches no
  grammar.
- The four walls. Exactly two egress authorizations exist, precisely
  bounded: **instance, non-private note content** to the embedding API
  (D32's bounded reading of wall 2), and **the query text of an explicitly
  requested semantic search** (D50.1). Any widening is a new decision.
- **The corpora, named.** The *embedding corpus* is instance markdown minus
  private sources — never templates, never anything under a privacy-declared
  directory. The *agent corpus* — what the CLI may output or allow to
  influence ranking — excludes private sources **before** channel depth and
  fusion, so a private note neither appears in agent output nor displaces a
  public one (no-output and no-influence, each with its own lock, H5). The
  UI's local lexical browsing keeps the full readable vault (local
  rendering is not egress); the UI's semantic channel reads only the
  embedding corpus.
- Degraded artifact policy disables the semantic channel along with the
  other instance projections (as shipped); a missing or invalid **privacy
  capability** fail-closes all of Part II (D50.10) — Part I lexical
  continues untouched.

## H3. Chunking (explicit rules, each a tested table case)

- A chunk is a heading-bounded section: the text from one heading (any
  level) to the next, extracted over the same goldmark AST layer the lexical
  index's `plain_text` uses — one extraction discipline, not two. The
  preamble before the first heading is chunk zero. Frontmatter is never
  chunk input. Code-fence text is included (code-term queries are first-
  class in this vault); wikilink targets contribute their display text,
  exactly as `plain_text` already resolves them.
- **Drop rule**: a section whose body is empty or markup-only (heading
  scaffolding, bare separators) produces no chunk. Measured 2026-07-12 on
  the real vault: 448 eligible notes → 7,247 natural sections, of which
  1,290 are empty → ≈5,960 chunks, ≈13.3 per note; ~2–3×10⁵ at the
  18k-note horizon — inside rung 1, with the rung 1→2 trigger (D32: ~10⁵
  chunks or p95 exact scan > ~100 ms) as the designed exit, and H14's
  envelope naming who measures that p95.
- **Cap**: the model input limit minus a 10% margin, **prefix included**
  (the `Title › H2 › H3 — ` context prefix counts against the budget).
- **Token counter**: a documented offline proxy (CJK characters ≈ 1
  token each; ASCII ≈ 4 characters per token), with the 10% margin
  absorbing proxy error — the count-tokens API is not called per chunk,
  which would couple indexing to the network twice. If the proxy is shown
  to under-count, the margin widens: a constant change, not a redesign.
- **Oversize fallback ladder**, in order: paragraph split → line split
  (tables and lists split at row/item boundaries) → hard rune split as the
  terminal case (a 313-line single fence exists in this vault). A fence
  splits at its own line boundaries and is never merged with prose across
  the cap. Continuation chunks carry `— part n/m` in their prefix.

## H4. The embedding pipeline and cache

- **Document embedding is serve-owned**, and only serve-owned: on a
  snapshot rebuild the scanner diffs content hashes, embeds changed/new
  chunks, and publishes to the cache. The swap never blocks on the network:
  a changed note's stale vector is masked from semantic results until its
  refresh lands (stale-masking, never stale-serving). **Query embedding is
  a per-request act of whichever surface was explicitly asked** (D50.1):
  the UI's submitted semantic search and the CLI's `--semantic` each embed
  exactly one query string per explicit action. The CLI embeds queries and
  never documents; it never writes the cache.
- **Cache identity**: (model, dimension, task types, normalization flag,
  chunker epoch, format version, vault root). Any component mismatching
  means this cache is not this corpus's cache — cold, never a partial read
  and never a silent reuse.
- **Embedding contract pinned**: task types `RETRIEVAL_DOCUMENT` /
  `RETRIEVAL_QUERY`; non-native dimensions are re-normalized after
  truncation (the API returns unnormalized vectors below the native
  dimension); the full preprocess (prefix rule, drop rule, fallback
  ladder) is part of the chunker epoch.
- **Publication contract**: one writer (the serve process); files `0600`;
  writes go temp + fsync + atomic rename; any malformed row means the whole
  file is cold; rows are purged when their note is deleted or reclassified
  (instance→non-instance, or into a privacy-declared directory).
- **Final-send revalidation**: eligibility (instance ∧ non-private) is
  re-checked against the current snapshot at the choke point that performs
  the network send — a note reclassified between collection and send is
  dropped there, and the guard locks target that choke point (H5), not the
  collector alone.
- **Storage engine**: behind a narrow interface (put / get / scan). The
  engine is an engineering call (D31) made inside the build with a
  benchmark — the candidates are the JSONL file above, SQLite (the
  storage report's leading durable candidate), and a packed immutable
  generation with an atomic manifest; the identity, atomicity, and
  cold-on-corruption contract binds whichever wins.
- **Epoch cutover** (D50.2): the old epoch serves until the new epoch is
  complete and swaps atomically — and only while the old epoch's query
  embedder is still available; a query embedded on one model never scores
  against another model's vectors. With no usable old embedder (the
  generation-retirement case) there is no old epoch to serve: semantic is
  cold until the new epoch completes.
- **Key**: `YOMIHON_EMBED_KEY`, read once in the `cmd/yomihon` wiring,
  passed down as a value, sent only as a request header. It joins the env
  wall-lock allowlist in the same PR — a deliberate, test-visible edit —
  and its absence from stdout, stderr, cache bytes, and error text is
  itself a lock (H5).

## H5. The egress guards (five flows, each with its own lock, built before any network code)

1. **Document flow**: a collect-level lock (a fixture holding private
   files, templates, and instance notes; the candidate set names exactly
   instance ∧ non-private) **and** a choke-point lock (a recording embedder
   client plus the H4 revalidation: nothing sends that the current
   snapshot no longer allows — reclassify a note between collect and send
   in the test, and the send must not happen).
2. **Query flow** (D50.1): only explicit actions embed — the UI's
   submit/toggle and the CLI's `--semantic`, at most one send per action.
   The live fragment path cannot reach the embedder: structurally (its
   handler holds no embedder client) and behaviorally (a lock drives
   typing against the fragment endpoint with a recording client and
   asserts zero requests). Pure-filter and empty-text queries never embed.
3. **Logs and metrics**: raw query text appears in no log, cache, error,
   metric, or trace — **including removing the shipped handler lines that
   log full queries today** (a live Part I defect this ruling closes; see
   the note in H11). Lock: a recording logger under error injection
   asserts absence.
4. **Error surfaces**: an embedder failure's user- or agent-visible text
   names the failure class; it never carries the query bytes or the key.
5. **Key transport**: as pinned in H4; error-path fixtures assert the key's
   absence from every output stream.

Plus the **influence lock** (D50.10's reason made mechanical): a private
source planted in a fixture must neither appear in agent-corpus output
(no-output) nor change the fused ordering of public results (no-influence)
— two separate assertions, both mutation-proven.

## H6. Retrieval and fusion (the knobs pinned, the semantics complete)

- **Filters first**: every structured filter is a hard constraint applied
  to *both* channels before depth — a semantic candidate that fails a
  filter never enters fusion. Pure-filter and empty-text queries invoke no
  semantic work at all (H5.2).
- Semantic channel: the query embeds on the same model/epoch; exact cosine
  over the chunk matrix; **chunk→note max-similarity aggregation happens
  before the depth cut**, and the channel's depth is the top 50 *notes*.
  Chunk ties inside a note break by chunk ordinal.
- Lexical channel: the shipped two-bucket ordering supplies its ranking,
  stated honestly — within a bucket the order is the index's deterministic
  fold-key-then-rel-path total order (the fold key is the index's
  NFC-folded title), a completeness order rather than a relevance claim.
  RRF's damping is the acknowledged treatment, and the eval set (H9) is
  the check that this coarse rank does not degrade outcomes.
- Fusion: RRF with **k = 60** over each channel's top 50; score =
  Σ 1/(k + rank). A note absent from a channel contributes nothing — no
  imputed ranks. (RRF arithmetic is what it is: two deep agreements can
  outscore one first place; the eval set, not intuition, judges whether
  that hurts here.)
- **Lexical completeness preserved**: the fused block reorders only the
  two top-50s; every lexical match beyond the fused block is appended
  after it in lexical order. A query with 80 lexical hits still answers
  `--limit 80` — fusion reorders the head, it never truncates the match
  set.
- Determinism: cross-note ties break by fold-key then rel-path; identical
  inputs give byte-identical output (the CLI golden depends on it).
- k and the depths are starting constants pinned for reproducibility, not
  tuned truths; changing them is an ordinary code change with the eval set
  as the regression floor.

## H7. Surfaces and the degraded matrix (the dispatch-gate table)

**Surfaces.** `/search` and ⌘K: the semantic channel joins only on an
explicit submit or toggle (D50.1); the live-results fragment stays lexical
on every cell, permanently. `yomihon search` CLI:
`yomihon search <query> [--json] [--limit N] [--semantic] [--root <path>]` —
lexical by default; `--semantic` is strict and there is no best-effort form
(D50.7). The CLI embeds only the query (H4), reads the cache read-only, and
never embeds documents.

**JSON contract** (frozen at build, golden-pinned; the D37 rule): top-level
`{query, mode, semantic, coverage, results}`. `mode` ∈ `lexical|hybrid`;
`semantic` ∈ `off|ok|unavailable`; the legal pairs are exactly three —
`lexical/off` (no `--semantic`, exit 0), `hybrid/ok` (exit 0), and
`lexical/unavailable` (strict failure, exit 3: the body carries the lexical
results honestly labeled, a typed `coverage` diagnostic naming the failure
class and, for stale-partial, the masked-note count — the exit code, never
the body, is what automation branches on; a partial answer never wears
exit 0, D50.5). `results` is always present, `[]` on zero matches. Each
result: `{rank, rel_path, title, status, snippet, heading, channels,
channel_ranks}` — enough evidence to act without a second call; `channels`
is ordered `[lexical, semantic]` filtered to those that fired. Ranks are
1-based and dense. Exit codes (this command's own frozen table — each CLI
command owns its exit vocabulary; `check`'s exit 1 means findings, this
exit 1 means internal failure): 0 = ran, 1 = internal error, 2 = usage,
3 = semantic required but unavailable. stderr on exit 3 is a single
sentence naming the reason; goldens pin one example of each legal pair,
and separate assertions pin exit code and stderr bytes.

**Collapsing rules** (each an explicit ruling; they close every cell the
core table does not enumerate):

- R1 (D50.10): privacy capability missing/invalid → all Part II behavior
  off, fail-closed; Part I lexical continues.
- R2 (D47): artifact policy missing/invalid → semantic off with the
  instance projections; bare-text/`folder:` lexical continues.
- R3 (D50.1): the live fragment is lexical in every cell.
- R4 (D50.1): pure-filter and empty-text queries never embed, any cell.
- R5 (D50.7): no best-effort surface exists.

**Core table** (artifact valid ∧ privacy valid; surfaces = UI explicit
semantic search, CLI `--semantic` strict):

| Cache | API | UI (submitted) | CLI strict |
|---|---|---|---|
| cold | up | lexical + "semantic building" indicator | exit 3, reason: cache cold |
| cold | down | lexical + offline indicator | exit 3, reason: embedder unreachable |
| cold | 429 | lexical + rate-limited indicator | exit 3, reason: rate-limited (D50.6, fail-fast) |
| warm | up | hybrid | hybrid, exit 0 |
| warm | down | lexical + offline indicator (query cannot embed) | exit 3, reason: embedder unreachable |
| warm | 429 | lexical + rate-limited indicator | exit 3, reason: rate-limited |
| stale-partial | up | hybrid over the unmasked set + refresh indicator with pending count | exit 3 + typed coverage diagnostic (D50.5) |
| stale-partial | down | lexical + offline indicator | exit 3 |
| stale-partial | 429 | lexical + rate-limited indicator | exit 3 |
| cutover | up | hybrid on the old epoch while its query embedder lives (D50.2); else as cold | same rule |
| cutover | down | lexical + offline indicator | exit 3 |
| cutover | 429 | lexical + rate-limited indicator | exit 3 |

Every cell above is EXPLICIT-RULING (D50.1/2/5/6) or CANON-DERIVED
(roadmap §4a); with R1–R5 covering the remaining axes, NEEDS-RULING = 0.
Query-side 429 is fail-fast and shares no offline latch with the background
pipeline's bounded backoff (D50.6). Each row is an acceptance test in the
build; the UI indicator texts are part of the locked strings.

## H8. Removed — source_kind moved to the H face (D50.4)

The `source_kind` index field and filter key are not a hybrid dependency;
they land with the H face's frontmatter-query work, after the canon sync
(spec §3's frozen six, Part I §3's field list) that D50.4 requires before H
implements them. B leaves the grammar untouched.

## H9. The eval set (synthetic in the repo, real-vault local — D50.8)

- **Committed fixture**: a synthetic corpus plus 40 queries (meeting
  roadmap §5a's 30–50 obligation), covering 繁中 queries against
  Japanese-lesson-shaped content, Japanese term lookups, code-term queries,
  cross-lingual paraphrases (the case lexical cannot serve), and
  filter-mixed queries. No real-vault path, query, or vector is committed
  (wall 2 — derived data does not leave the machine).
- **Per query, pinned**: the required positives (top-5 membership), the
  explicit negatives (results that must not appear — a recall-only oracle
  passes while a forbidden result rides along), the tie rule at rank 5, and
  the denominator.
- **Recorded vectors are epoch-bound**: the fixture stores the full cache
  identity (H4) it was recorded under, and the harness fails on any
  mismatch — it never silently scores new code against another epoch's
  vectors. Refreshing the fixture is a two-sided change: the update commit
  carries the paired before/after per-query diff.
- **Real-vault evaluation stays local**: the harness runs against
  `~/obsidian` when present, keeps per-query paired diffs local, and only
  content-free aggregates may be quoted in a PR.
- **Dimension** (D50.9 + H12.1): 1536 vs the native dimension is decided by
  a paired comparison on this eval set within `gemini-embedding-2`, before
  the dimension is pinned into the cache identity.
- Framing: a regression floor, not a tuning target — a change that lowers
  recall@5 or admits a forbidden result fails; raising the numbers is not
  itself a goal.

## H10. Locks and kill-tests (standards §2 discipline)

Every lock watched red before the PR: the five egress-flow locks and the
influence lock (H5); the final-send revalidation choke lock; the stale-mask
(serve a modified note, assert its old vector is absent until refresh);
cache-identity mismatch is cold; malformed-row-is-cold and
purge-on-reclassify; the epoch atomic swap and its old-embedder guard
(D50.2); filters-as-hard-constraints (a filtered-out semantic candidate
never fuses); lexical completeness past the fusion depth (`--limit` beyond
50 answers); fusion determinism (the CLI golden bytes); the exit-code
taxonomy with its stderr sentences; the eval harness failing on identity
mismatch; and the removal of the shipped raw-query log lines (error
injection asserts absence). The CLI goldens are frozen contract per D37 —
H7's JSON is the spec they pin.

## H11. Build order (dependency order, not a fence)

0. **Dispatch prerequisite, its own unit (D50.10)**: the vault contract's
   privacy capability — the `[privacy]` section, `internal/schema`'s
   derivation, snapshot carriage, and the fail-closed degradation — lands
   and is accepted before any step below starts. (The removal of the
   shipped raw-query log lines does not need to wait: it is a live Part I
   defect and may ship as an immediate micro-unit.)
1. Chunker over the existing extraction layer (+ the H3 rule table's
   tests, bound and ladder cases).
2. Cache interface + identity + publication contract (+ cold/mismatch/
   corruption/purge tests); the storage bake-off decides the engine here.
3. The egress-guard locks (H5), all five flows — before any network client
   exists.
4. Embedder client (recording fake in tests) + scanner integration with
   stale-masking + the epoch machinery and its guards.
5. Cosine top-k + fusion (+ filters-first, completeness, determinism, and
   the eval harness on recorded vectors; the dimension decision's paired
   run happens here).
6. Surfaces: /search + ⌘K explicit-submit channel merge and indicator
   states; the CLI with goldens and the exit taxonomy.
7. The env wall-lock allowlist edit for `YOMIHON_EMBED_KEY`.

## H12. Resolved questions (the 2026-07-12 ruling sheet; D50)

1. **Dimension** → decided by a paired eval-set comparison within
   `gemini-embedding-2` before pinning (D50.9); until then the dimension is
   deliberately unpinned and the cache identity carries it.
2. **Representation** → chunk-only with max aggregation (D50.3; D32
   amended). Reopens only if the eval shows broad-topic recall failing —
   not on token-limit grounds.
3. **`--semantic=best-effort`** → killed (D50.7), here and in roadmap §4a.
4. **Query-text egress** → explicit actions only (D50.1): UI submit/toggle
   and CLI `--semantic`, one send per action; the live fragment stays
   lexical; raw query text enters no log, cache, error, metric, or trace.

Open engineering calls (not rulings): the storage engine bake-off (H4) and
the token-proxy constants (H3) — both decided inside the build with
measurements, both reversible behind their interfaces.

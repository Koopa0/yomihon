# Search face — implementation plan (spec §3)

> Status: Part I (the lexical engine, /search page, ⌘K shell, and the live
> lexical-results enhancement) is **built and merged**; it is kept as the
> record of that plan. Part II (§§H1–H14) is the hybrid extension's plan,
> revised 2026-07-12 to the ruling sheet (D50) and **again 2026-07-13 to
> the six delta clarifications** (walls text synced; the fourth legal
> pair; privacy-unavailable emits no agent payload; the precedence gate;
> the Embedding-2 protocol re-pinned from its own documentation; mismatch
> and cutover as distinct states). Dispatch gate (D50.10): the vault
> contract's privacy capability must land before any Part II behavior. A
> scoped final check audits the delta round's closure table before build.
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
  scaffolding, bare separators) produces no chunk. Measured on the
  2026-07-12 vault snapshot (the vault drifts; these are dated figures,
  re-measured at build start): 448 eligible notes → 7,247 natural
  sections − 1,290 empty − 20 markup-only → 5,937 chunks, ≈13.25 per
  note. The 18k-note extrapolation is ≈2.4×10⁵ chunks — **beyond the
  rung-1→2 trigger (D32: ~10⁵ chunks or p95 exact scan > ~100 ms) by
  ~2.4×**, so at that horizon rung 2 is the designed path, not a
  surprise; H13 carries the envelope and names who measures the p95.
- **Cap**: `cap = floor(0.9 × model_input_limit)` proxy tokens, **prefix
  included** (the `Title › H2 › H3 — ` context prefix counts against the
  budget).
- **Token counter**, the exact integer formula (table-tested):
  `tokens(s) = cjk_count(s) + ceil(other_count(s) / 4)`, where `cjk_count`
  is the number of runes in the CJK Unified Ideographs, Hiragana, Katakana,
  and CJK punctuation ranges and `other_count` is every remaining rune.
  The 10% margin absorbs proxy error; the count-tokens API is not called
  per chunk. If the provider still rejects an input as oversized, the
  bounded rule is: split that chunk once more at its midpoint line
  boundary and retry each half once; a half that is rejected again is
  recorded as a failed chunk with a named diagnostic and the epoch is
  marked incomplete — loud, never silently dropped, never an unbounded
  retry loop. If the proxy is shown to under-count systematically, the
  margin constant widens: a constant change, not a redesign.
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
  **at most one** query string per explicit action — the precedence gate
  (H7) may stop the request before any embedding happens. The CLI embeds
  queries and never documents; it never writes the cache.
- **The embedding protocol is pinned from the successor model's own
  documentation** (D50.4 clarification, 2026-07-13 — the first revision
  carried the predecessor generation's semantics, a verified error). What
  is already known from the official contract: `gemini-embedding-2` takes
  no task-type field; output at a truncated dimension is normalized by the
  API (no client-side renormalization); multi-input request semantics
  differ from the predecessor, so this design sends **one chunk per
  request row and never relies on any server-side aggregation**. The
  build's first protocol step records, with an official-doc anchor per
  clause: the exact request template for the document role and the query
  role, the exact submitted bytes for a reference input, and the response
  handling at the chosen dimension. The scoped review verifies each anchor.
- **Cache identity**: (model, dimension, the protocol epoch — a hash over
  the pinned request templates and preprocess rules — the chunker epoch,
  format version, vault root). Any component mismatching means this cache
  is not this corpus's cache — cold, never a partial read and never a
  silent reuse. The full preprocess (prefix rule, drop rule, fallback
  ladder) is part of the chunker epoch.
- **Publication contract**: one writer (the serve process); files `0600`;
  writes go temp + fsync + atomic rename; any malformed row means the whole
  file is cold. Before a row is committed, the publisher re-validates that
  the embedded content hash still matches the current snapshot's bytes and
  that the note is still eligible (instance ∧ non-private) in the current
  snapshot generation — a response landing after its note changed or was
  reclassified is discarded, not published. A policy change atomically
  invalidates every affected row; deletion and reclassification purge.
- **Final-send revalidation**: eligibility (instance ∧ non-private) is
  re-checked against the current snapshot at the choke point that performs
  the network send — a note reclassified between collection and send is
  dropped there, and the guard locks target that choke point (H5), not the
  collector alone. (Send-side and publish-side revalidation are two
  separate gates; each has its own lock.)
- **Two storage layers, named separately.** The *durable vector cache* sits
  behind put / get / scan and is what the bake-off chooses — candidates:
  a header-plus-rows file (one identity header line, then one row per
  chunk vector), SQLite (the storage report's leading durable candidate),
  and a packed immutable generation with an atomic manifest; the identity,
  atomicity, and cold-on-corruption contract binds whichever wins. The *in-memory query engine* is the
  separate layer D32's put / get / top-k interface names: it loads from
  the durable cache at startup/epoch-swap and answers cosine top-k; the
  scale rungs govern it, not the cache file.
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
   metric, or trace. The shipped handler lines that logged full queries
   were removed as their own merged unit (2026-07-13), with a
   recording-logger lock under error injection asserting absence; B's
   surfaces inherit that lock and extend it to the new paths.
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
- Lexical channel: the shipped ordering supplies its ranking **unchanged**
  — title-match bucket before body-match bucket, rel-path order within
  each bucket, exactly as Part I §6 ships it (the first revision invented
  a fold-key order here; withdrawn). It is a completeness order rather
  than a relevance claim; RRF's damping is the acknowledged treatment,
  and the eval set (H9) is the check that this coarse rank does not
  degrade outcomes.
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
- Determinism: fused-score ties break by rel-path (the index's existing
  total order); identical inputs give byte-identical output (the CLI
  golden depends on it).
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

**The precedence gate** (ruled 2026-07-13; every request walks it in this
order, and the first failing stage names the reason — nothing later runs):

1. **privacy** — capability missing/invalid → fail-closed (see R1);
2. **artifact** — policy missing/invalid → instance capabilities off (R2);
3. **semantic applicability** — pure-filter or empty-text queries carry
   nothing to embed: the semantic channel is *not applicable*, never
   "degraded" (R4);
4. **cache usability** — cold / mismatch / no old epoch → semantic cannot
   be served, so **the query is never embedded** (no pointless egress);
5. **query API** — only a request that passed 1–4 embeds the query;
   network failures surface here.

**JSON contract** (frozen at build, golden-pinned; the D37 rule): top-level
`{query, mode, semantic, coverage, results}`.

- `mode` ∈ `lexical|hybrid`; `semantic` ∈ `off|ok|not-applicable|
  unavailable`. The legal pairs are exactly four:
  `lexical/off` (no `--semantic`; exit 0), `hybrid/ok` (exit 0),
  `lexical/not-applicable` (`--semantic` on a pure-filter or empty-text
  query, ruled 2026-07-13: nothing was degraded — the lexical answer is
  complete; exit 0, zero embedding), and `lexical/unavailable` (strict
  failure, exit 3).
- `coverage`: present exactly when `semantic` ≠ `ok`; a typed object
  `{reason, masked_notes?}` where `reason` is one of the frozen strings
  (`privacy-capability-unavailable`, `artifact-policy-unavailable`,
  `not-applicable`, `cache-cold`, `cache-mismatch`, `embedder-retired`,
  `embedder-unreachable`, `rate-limited`, `stale-partial`) and
  `masked_notes` (integer) appears only with `stale-partial`.
- `results`: always present, `[]` on zero matches — with one ruled
  exception: **privacy-unavailable emits no result payload at all**
  (`results` absent, exit 3) — agent output is fail-closed under D50.10,
  and an exit-3 body carrying lexical results would leak what the ruling
  forbids. Every other exit-3 body carries the lexical results honestly
  labeled; the exit code, never the body, is what automation branches on
  (D50.5).
- Each result: `{rank, rel_path, title, status, snippet, heading,
  channels, channel_ranks}`. `rank` 1-based dense; `rel_path`/`title`
  always present; `status` present iff governed; `snippet` present iff a
  body match exists (title-only matches carry none); `heading` present
  iff the best evidence sits under one; `channels` ⊆ `[lexical,
  semantic]` in that fixed order, never empty; `channel_ranks` an object
  keyed by the channels present, integer ranks.
- Exit codes (this command's own frozen vocabulary; `check`'s exit 1
  means findings — this exit 1 means internal failure): 0 = ran
  (including zero results and not-applicable), 1 = internal error,
  2 = usage, 3 = a required capability is unavailable (semantic, metadata
  filters, or privacy-gated output — the stderr sentence and `coverage.
  reason` name which). stderr on exit 3 is exactly one line:
  `yomihon search: <reason>: <one frozen sentence>`. Goldens pin one
  example of each legal pair plus the privacy-unavailable no-payload
  shape; separate assertions pin exit codes and stderr bytes.

**Collapsing rules** (each labeled with its authority):

- R1 — privacy capability missing/invalid: all Part II behavior off,
  fail-closed [EXPLICIT-RULING D50.10]. Local UI continues lexical with a
  named diagnostic [CANON-DERIVED, Part I availability]; **the search CLI
  emits no result payload, exit 3, fixed privacy stderr** [EXPLICIT-RULING
  2026-07-13].
- R2 — artifact policy missing/invalid: semantic off with the instance
  projections [CANON-DERIVED D47]; metadata-containing queries answer
  with the capability diagnostic before any embedding — UI shows it, CLI
  exits 3 with `artifact-policy-unavailable` [EXPLICIT-RULING D47 +
  gate order 2026-07-13]; bare-text/`folder:` queries continue lexical
  (UI normally; CLI `--semantic` still exits 3 — the semantic corpus is
  instance-scoped and cannot exist) [CANON-DERIVED].
- R3 — the live fragment is lexical in every cell [EXPLICIT-RULING D50.1].
- R4 — pure-filter and empty-text queries never embed; with `--semantic`
  they answer `lexical/not-applicable`, exit 0 [EXPLICIT-RULING
  2026-07-13].
- R5 — no best-effort surface exists [EXPLICIT-RULING D50.7].

**Core table** (privacy valid ∧ artifact valid ∧ semantic applicable;
surfaces = UI explicit semantic search, CLI `--semantic` strict). The
cache axis carries the background pipeline's condition as explicit
substates (ruled 2026-07-13: mixed states are enumerated, not folded);
the API axis is the **query** API alone — the background pipeline's own
backoff shares no latch with it (D50.6).

| # | Cache state | Query API | UI (submitted) | CLI strict | Authority |
|---|---|---|---|---|---|
| 1 | cold | up | lexical + "semantic index building" | exit 3 `cache-cold`; query never embedded (gate 4) | ER D50.5 + gate |
| 2 | cold | down | same as 1 — gate 4 fails first, API never consulted | same as 1 | ER gate order |
| 3 | cold | 429 | same as 1 | same as 1 | ER gate order |
| 4 | warm, background idle | up | hybrid | hybrid, exit 0 | CD §4a |
| 5 | warm, background idle | down | lexical + offline indicator (query cannot embed) | exit 3 `embedder-unreachable` | ER D50.5 |
| 6 | warm, background idle | 429 | lexical + rate-limited indicator | exit 3 `rate-limited`, fail-fast | ER D50.6 |
| 7 | stale-partial, background refreshing | up | hybrid over unmasked + pending count | exit 3 `stale-partial` + `masked_notes` | ER D50.5 |
| 8 | stale-partial, background refreshing | down/429 | lexical + offline/rate indicator | exit 3, reason = the query-API failure (gate 5 reached, failed there) | ER D50.5/6 |
| 9 | stale-partial, background stalled (its own backoff) | up | as row 7 — identical serving; the indicator's pending count simply stops shrinking | as row 7 | ER D50.6 (no shared latch) |
| 10 | stale-partial, background stalled | down/429 | as row 8 | as row 8 | ER D50.5/6 |
| 11 | cutover, old embedder alive, new epoch building | up | hybrid on the old epoch | hybrid on the old epoch, exit 0 | ER D50.2 |
| 12 | cutover, old embedder alive | down/429 | lexical + indicator | exit 3, query-API reason | ER D50.2/5/6 |
| 13 | cutover, old embedder retired | any | as cold (rows 1–3): gate 4 fails, no old epoch to serve, query never embedded | as cold | ER D50.2 |
| 14 | epoch-mismatch (unmanaged identity mismatch) | any | as cold — mismatch means this cache is not this corpus's cache (H4) | as cold | CD H4 identity |

Rows 1–3, 13, 14 collapse to the same observable (cold semantics) by the
gate — they are enumerated so nothing is folded silently. Each row is an
acceptance test in the build; UI indicator texts are locked strings.
NEEDS-RULING = 0 as of the 2026-07-13 clarifications; the scoped review
re-audits that claim per row.

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

The 2026-07-13 clarifications (Koopa, closing the delta round's open
cells; recorded in D50's amendment note): the walls text now names both
egress exceptions; `lexical/not-applicable` is the fourth legal pair
(exit 0, zero embedding); privacy-unavailable strict CLI emits no result
payload; the precedence gate is privacy → artifact → applicability →
cache → query API, with no query egress before the final stage; the
Embedding-2 protocol re-pins from the successor's own documentation; and
unmanaged epoch-mismatch and managed cutover are distinct matrix states,
with background-pipeline conditions enumerated as cache substates.

Open engineering calls (not rulings): the storage engine bake-off (H4) and
the token-proxy constants (H3) — both decided inside the build with
measurements, both reversible behind their interfaces.

## H13. Scale and capacity envelope (the numbers, dated 2026-07-12)

- **Working set (RAM, the query engine)**: chunks × dim × 4 bytes.
  Today at 5,937 chunks: ≈36 MiB at 1536, ≈73 MiB at 3072. A managed
  cutover holds two epochs: peak is double the steady figure. At the
  18k-note extrapolation (≈2.4×10⁵ chunks): ≈1.4 GiB at 1536 steady —
  which is why that horizon is rung 2 territory by design (H3), not a
  rung-1 promise.
- **Durable cache (disk)**: the vector payload dominates; base64-encoded
  float32 costs 4/3 of raw — ≈47 MiB at 1536 today, ≈95 MiB at 3072,
  before row metadata; two epochs may coexist during cutover, so disk
  peak is likewise double. A binary-payload engine (the bake-off's SQLite
  or packed-generation candidates) stores raw and drops the 4/3 factor.
- **Capacity failure is loud**: if the engine cannot build its matrix
  (allocation failure, corrupt cache larger than expected), the semantic
  channel reports `unavailable` with a named diagnostic and lexical
  serving continues — never a crash of the reading face, never a silent
  half-loaded matrix.
- **The p95 observer, named**: (a) a repo benchmark
  (`BenchmarkSemanticTopK`, synthetic corpus of pinned size) under the
  standing benchstat discipline; (b) the serve process logs the measured
  top-k p95 over the real corpus **at every epoch build** — owner: the
  scanner; cadence: each epoch. The rung-1→2 trigger (D32) is therefore
  observable in the serve log, not a folklore number.

## H14. The §5a obligations, dispositioned

| Obligation (roadmap §5a, B) | Status |
|---|---|
| Chunking rules, chunks-per-note assumption stated | MET (H3: rules, formulas, dated measurements) |
| Cache file format versioned by (model, dim) | MET, widened (H4: full identity tuple; engine via bake-off) |
| RRF specifics (k, depths, aggregation) | MET (H6) |
| §4a degraded matrix as acceptance cases | MET (H7: gate + 14 rows + R1–R5, per-row authority) |
| The eval set | MET (H9; synthetic-in-repo per D50.8) |
| Egress guard test (Diary never reaches the embedder) | MET, widened (H5: five flows + influence lock; the log flow already shipped) |

No obligation is PARTIAL or MISSING as of this revision; the scoped
review audits this table against the text.

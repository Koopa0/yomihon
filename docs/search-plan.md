# Search face — implementation plan (spec §3)

> Status: Part I (the lexical engine, /search page, ⌘K shell, and the live
> lexical-results enhancement) is **built and merged**; it is kept as the
> record of that plan. Part II (§§H1–H14) is the hybrid extension's plan,
> written to the D50 ruling sheet and hardened over seven adversarial
> rounds. The last (2026-07-13) added Koopa's **fault-ownership ruling**:
> only a response yomihon can confirm it formed wrongly is an internal
> error (exit 1); every provider fault and every unknown/unclassifiable
> response is a semantic `unavailable` (exit 3, lexical preserved), the
> default for uncertainty being the provider's fault, never a claimed
> yomihon bug. Three discriminated envelopes (answerable / capability-
> unanswerable / internal-error) told apart by their top-level key (exit 3
> covers two of them); the matrix carries rows 1–14 with per-surface
> reachability; the single-send lock covers
> every terminal including the fault-ownership ones. Dispatch gate
> (D50.10): the vault contract's privacy capability must land before any
> Part II behavior.
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

Each step has an independently verifiable output. Browser UI and browser-facing
errors follow the Traditional Chinese interface contract (D28), including the
empty-results message; source comments and wire contracts remain English.

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
  scaffolding, bare separators) produces no chunk. Two dated measurements
  of the live vault, neither reproducible after the fact (the vault
  drifts — which is why the build re-measures and records the figure with
  its vault commit): 2026-07-12 → 448 eligible notes, 7,247 sections,
  1,290 empty, 20 markup-only, **5,937 chunks ≈13.25/note**; 2026-07-13 →
  477 notes, 7,571 sections, 1,365 empty, 20 markup-only, **6,186 chunks
  ≈12.97/note**. The 10–15 chunks/note working assumption holds across
  both. The 18k-note extrapolation is ≈2.4×10⁵ chunks — **beyond the
  rung-1→2 trigger (D32: ~10⁵ chunks or p95 exact scan > ~100 ms) by
  ~2.4×**, so at that horizon rung 2 is the designed path, not a
  surprise; H13 carries the envelope and specifies the p95 measurement.
- **Cap**: `cap = floor(0.9 × model_input_limit)` proxy tokens, **prefix
  included** (the `Title › H2 › H3 — ` context prefix counts against the
  budget).
- **Token counter**, the exact integer formula (table-tested):
  `tokens(s) = cjk_count(s) + ceil(other_count(s) / 4)`, where
  `cjk_count` counts runes satisfying `unicode.Is` on the `Han`,
  `Hiragana`, or `Katakana` range tables, plus the explicit code-point
  intervals U+3000–U+303F (CJK symbols and punctuation) and
  U+FF00–U+FFEF (halfwidth/fullwidth forms); `other_count` is every
  remaining rune. The boundary is these named tables and intervals —
  nothing vaguer.
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
  documentation** (the D50.9 amendment, 2026-07-13 — the first revision
  carried the predecessor generation's semantics, a verified error). What
  is already known from the official contract: `gemini-embedding-2` takes
  no task-type field; output at a truncated dimension is normalized by the
  API (no client-side renormalization); multi-input request semantics
  differ from the predecessor, so this design sends **one chunk per
  request row and never relies on any server-side aggregation**. The
  build's first protocol step records, with an official-doc anchor per
  clause: the exact request template for the document role and the query
  role, the exact submitted bytes for a reference input, the request
  cardinality above, the response handling at the chosen dimension
  (vector extraction and the normalization the API did or did not apply),
  and **the error taxonomy → gate-state mapping, total by construction and
  split by fault ownership** (Koopa's EXPLICIT-RULING, 2026-07-13; recorded
  in the D50 amendment) — every documented provider error class maps to
  exactly one local outcome, cited to the successor's own error docs rather
  than a hardcoded status number:
  - **Provider fault, and any unknown/unclassifiable response → semantic
    `unavailable`, exit 3, the lexical answer preserved** — the default for
    uncertainty is "the provider's problem," never a claimed yomihon fault:
    the authorization refusal → `embedder-rejected` (gate 5, non-retryable);
    the throttle → `rate-limited`; a transport non-answer →
    `embedder-unreachable`; a provider server error (its `INTERNAL` / 5xx
    class) → `embedder-failed`; **and every remaining documented class the
    mapping cannot confidently attribute to our own request, plus every
    unknown or undocumented status → `embedder-failed`** (the catch-all is
    provider-side, not ours; its diagnostic says only that the API returned
    an error search could not recover from — it does not assert a
    server-side cause it cannot confirm).
  - **Only a response we can confirm is a yomihon-formed malformed
    request → the command's internal error, exit 1** (loud, because it is a
    build defect): the classes the successor's docs attribute
    unambiguously to a malformed client request. Ambiguous-origin classes
    are NOT forced here — when we cannot confirm the fault is ours, the
    catch-all above applies.
  The mapping's totality is itself asserted (H10). Because status class does
  not by itself prove fault ownership, the protocol step records, per class,
  the docs' stated origin and the outcome it maps to — the acceptance
  verifies each attribution against the successor's own documentation, and
  a class the docs do not clearly pin to a client-side malformed request
  defaults to `embedder-failed`.
- **Cache identity**: (model, dimension, the protocol epoch — a hash over
  the pinned request templates, the request cardinality, the response
  handling, and the preprocess rules — the chunker epoch, format version,
  vault root). Any component mismatching means this cache is not this
  corpus's cache — cold, never a partial read and never a silent reuse.
  A response-handling change alone therefore colds the cache: vectors
  extracted under different handling never silently mix. The full
  preprocess (prefix rule, drop rule, fallback ladder) is part of the
  chunker epoch.
- **Publication contract**: one writer (the serve process); files `0600`;
  writes go temp + fsync + atomic rename; any malformed row means the whole
  file is cold. **Publication is linearized**: commits, purges, and
  reclassification-invalidations all execute inside one critical section
  owned by the single publisher, in arrival order. A commit is
  conditional, evaluated *inside* that critical section against the
  snapshot current at commit time: the embedded content hash must still
  match and the note must still be eligible (instance ∧ non-private) —
  so a purge or reclassification that entered the section first wins, and
  the late response is discarded; check-then-commit can never interleave
  with a purge. The race lock is deterministic, not sleep-based: a test
  hook admits a reclassification between a response's arrival and its
  commit and asserts the commit is discarded (H10). A policy change
  atomically invalidates every affected row; deletion and reclassification
  purge.
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
order, and the first failing stage names the reason — nothing later runs.
Amended after the scoped round: each stage now states exactly which
queries it can fail, so the gate and the collapsing rules cannot disagree):

1. **privacy** — capability missing/invalid. Fails **every CLI request**
   (the CLI is an agent surface; with no privacy authority the agent
   corpus cannot be computed, so no payload may leave — `--semantic` or
   not). Local human surfaces (UI, live fragment) pass through to lexical
   with a named diagnostic.
2. **metadata answerability** — artifact policy missing/invalid fails
   **only metadata-bearing queries** (a filter that cannot be evaluated
   makes the query unanswerable; never ignored, never faked as zero —
   the shipped Part I behavior). Bare-text and `folder:` queries pass.
3. **semantic applicability** — pure-filter or empty-text queries carry
   nothing to embed: with `--semantic` the channel is *not applicable*,
   never "degraded" — exit 0. Reachable only after stages 1–2 pass.
3b. **semantic request** — the fork that stops every non-semantic request
   here, so no later stage can touch it. A request that did **not** ask
   for semantic (no `--semantic`, and the UI without an explicit
   semantic submit) is answered lexically now: `lexical/off`, exit 0,
   whatever the index holds. **Stages 4, 4b, and 5 are reached only by a
   text-bearing request that explicitly asked for semantic** — so cold,
   mismatch, retirement, capacity, a missing key, and every query-API
   failure are, by construction, unreachable for a plain lexical query.
   This is the single place the gate guarantees what R6/R7 assert.
4. **semantic corpus & cache usability** — the semantic corpus cannot
   exist (artifact policy invalid), or the index cannot serve this
   surface → semantic cannot be served, so **the query is never
   embedded** (no pointless egress). *The serving bar is
   surface-dependent, and this is the whole of the difference between
   the two surfaces*: the **strict CLI requires a complete index** (a
   stale-partial index fails here — D50.5 rules its answer is exit 3,
   so there is nothing to embed for), while the **UI accepts a partial
   index** and serves hybrid over the unmasked set. Evaluation order
   inside this stage: identity mismatch (the cache is not this corpus's
   at all) → index presence → embedder availability for the epoch that
   would serve → completeness (CLI only). Capacity failure (H13) is a
   stage-4 failure.
4b. **configuration preflight** (ruled 2026-07-13) — checked **only for a
   text-bearing request that explicitly asked for semantic**, and only
   after stage 4 found the index usable for this surface: with no
   embedding key configured, the state is `embedder-unconfigured` — no
   client is constructed, **no query is embedded**, and reading is never
   blocked. Serve logs the configuration diagnostic **once**, carrying
   neither the key nor any query. Plain lexical and not-applicable
   requests never reach this stage, so a missing key cannot change their
   answer. It runs after stage 4 *for the same surface*, so it never masks
   a stage-4 failure **that surface** already had: the strict CLI, which
   requires a complete index, fails at stage 4 on cold / mismatch /
   retired / stale-partial and never reaches 4b; the UI, which accepts a
   partial index, passes stage 4 on stale-partial and *can* then surface
   `embedder-unconfigured` — for the UI that is not masking a cache
   failure, because for the UI a partial index is not a failure. The
   invariant is per-surface: configuration never overrides the reason the
   *current surface* stopped at stage 4.
5. **query API** — only a request that passed 1–4b embeds the query; at
   most one call per explicit action, no in-place retry. Its outcome maps
   by the H4 fault-ownership taxonomy: a credential refusal →
   `embedder-rejected`; an unanswered call → `embedder-unreachable`; a
   throttle → `rate-limited` (fail-fast, D50.6); a provider server error
   **or any unknown/unclassifiable response** → `embedder-failed`; **only
   a response the docs confirm is a yomihon-formed malformed request →
   internal error, exit 1** (row 14). Every provider-fault outcome is exit
   3 with the lexical answer preserved (never a claimed yomihon bug). The
   class → outcome mapping is pinned at the protocol step from the
   successor's own docs, not a hardcoded status number. **No cross-request
   auth latch is authorized**: each explicit action is judged on its own
   call. Introducing one would need its own lifecycle and matrix states,
   ruled separately.

**JSON contract** (frozen at build, golden-pinned; the D37 rule). **No
envelope ever echoes the query** (ruled 2026-07-13 — the caller already
knows its input, and echoing it would put raw query text into an error
surface, which D50.1 and H5.4 forbid). Three discriminated envelopes,
told apart by **which top-level key is present** (not by exit code — exit
3 covers both the answerable and the capability-unanswerable shapes):

- **Answerable (exit 0 or 3)** — the command could answer, even if only
  lexically: `{mode, semantic, coverage?, results}`.
- **Unanswerable (exit 3)** — no honest answer exists (privacy capability
  invalid; a metadata filter that cannot be evaluated), *with or without*
  `--semantic`: `{"error": {"reason": "..."}}` — no `mode`, no `semantic`,
  no `coverage`, no `results`.
- **Internal error (exit 1)** — a build defect: a request we can confirm
  yomihon formed wrongly. The body is byte-exact (compact, as every wire
  body is — no spaces after colons):
  `{"internal_error":{"detail":"the request could not be formed correctly"}}`
  — no `mode`, no `semantic`, no `results`, no `error.reason`
  (which is reserved for the unanswerable-capability envelope). The
  `detail` is a fixed string, never the query text (which would be an
  egress into an error surface, forbidden by D50.1 / H5.4).
- **Non-JSON mode**: an unanswerable or internal-error request prints
  **nothing** on stdout and prints its one frozen stderr line (exit 3 or
  exit 1 respectively).

An agent's branch is therefore exact: exit 0/3 with `results` = a (possibly
lexical-only) answer; exit 3 with `error` = no answer, a capability is off;
exit 1 with `internal_error` = a yomihon bug, and the lexical reading face
still works in the UI.

**Byte framing** (the shipped agent-CLI convention, not a new one — the
judge face's `Finding` wire, judge-plan.md §3a, produced by the same
`WriteJSONL` discipline in `internal/judge`): a `--json` body is **one
compact JSON object** — `encoding/json` with no indentation — field order
exactly as listed here, terminated by a single trailing `\n`. The escape
surface is the shipped one, faithfully, not a paraphrase:
- CJK and `<` / `>` / `&` are raw UTF-8 (`SetEscapeHTML(false)` — its
  only effect);
- U+2028 and U+2029, which `encoding/json` escapes and offers no switch
  for, are rewritten to raw UTF-8 **after** encoding (the shipped
  `unescapeLineSeparators` step); a literal backslash-`u2028` in the
  content is not touched;
- JSON-required control escapes (`\n`, `\t`, U+0000…U+001F) stay
  escaped — valid JSON demands it, and this is not `<>&` or the two line
  separators.
Non-JSON mode prints the human results (or, for the two error envelopes,
nothing) on stdout. The goldens pin these exact bytes; H10 carries the
escape-surface lock. A compact-vs-pretty or trailing-newline ambiguity is
not left to the implementer.

Fields of the answerable envelope:

- `mode` ∈ `lexical|hybrid`; `semantic` ∈ `off|ok|not-applicable|
  unavailable`. The legal pairs are exactly four:
  `lexical/off` (no `--semantic`; exit 0), `hybrid/ok` (exit 0),
  `lexical/not-applicable` (`--semantic` on a pure-filter or empty-text
  query, ruled 2026-07-13: nothing was degraded — the lexical answer is
  complete; exit 0, zero embedding), and `lexical/unavailable` (strict
  failure, exit 3).
- `coverage`: present exactly when `semantic` ∈ `not-applicable |
  unavailable` — absent for `off` (semantic was never requested: there is
  nothing to explain) and for `ok`. A typed object `{reason,
  masked_notes?}`; `reason` is one of the frozen strings
  (`not-applicable`, `artifact-policy-unavailable`, `cache-cold`,
  `cache-mismatch`, `embedder-retired`, `capacity`,
  `embedder-unconfigured`, `stale-partial`, `embedder-unreachable`,
  `embedder-rejected`, `embedder-failed`, `rate-limited`) and
  `masked_notes` (integer) appears only with `stale-partial`.
- The two **unanswerable** reasons — `privacy-capability-unavailable` and
  `metadata-filters-unavailable` — never appear in `coverage`: they carry
  the *error* envelope instead, because no honest result set exists
  (agent output is fail-closed under D50.10; and a filter that cannot be
  evaluated must never be silently ignored or faked as zero, which is
  what the shipped Part I behavior refuses).
- `results`: present with `[]` on zero matches, in every answerable
  envelope. An exit-3 answerable body carries the **lexical-only** result
  list, honestly labeled — a partial hybrid is not a legal shape, so
  exit-3 bodies never mix channels; the exit code, never the body, is
  what automation branches on (D50.5).
- Each result: `{rank, rel_path, title, status, snippet, heading,
  channels, channel_ranks}`. `rank` 1-based dense; `rel_path`/`title`
  always present; `status` present iff governed; `snippet` present iff
  either channel contributes body evidence — a lexical body match or, in
  hybrid results, the best semantic chunk's excerpt — so a semantic-only
  hit still carries its evidence and never forces a second call;
  title-only lexical matches carry none; `heading` present iff the best
  evidence sits under one; `channels` ⊆ `[lexical, semantic]` in that
  fixed order, never empty; `channel_ranks` an object keyed by the
  channels present, integer ranks.
- Exit codes (this command's own frozen vocabulary; `check`'s exit 1
  means findings — this exit 1 means internal failure): 0 = ran
  (including zero results and not-applicable), 1 = internal error,
  2 = usage, 3 = a required capability is unavailable, **1 = internal
  error** (a confirmed-malformed request — a build defect). The machine-
  readable reason lives in whichever envelope the request carries: an
  answerable exit-3 body names it in `coverage.reason`; an unanswerable
  exit-3 body names it in `error.reason`; the exit-1 body carries
  `internal_error.detail`. The stderr line names it on every path (and is
  the only channel in non-JSON mode). stderr on exit 1 is exactly
  `yomihon search: internal: the request could not be formed correctly`.
  stderr on exit 3 is exactly one line per reason, frozen here:
  - `yomihon search: privacy-capability-unavailable: the vault contract declares no valid privacy policy, so agent-facing search output is closed`
  - `yomihon search: metadata-filters-unavailable: the vault contract declares no valid artifact policy, so metadata filters cannot be evaluated`
  - `yomihon search: artifact-policy-unavailable: the vault contract declares no valid artifact policy, so the semantic corpus cannot exist`
  - `yomihon search: cache-cold: no semantic index exists yet for this vault`
  - `yomihon search: cache-mismatch: the semantic index was built under a different configuration`
  - `yomihon search: embedder-retired: the old index's embedding model is no longer available`
  - `yomihon search: embedder-unconfigured: no embedding key is configured, so semantic search is off`
  - `yomihon search: embedder-unreachable: the embedding API did not answer`
  - `yomihon search: embedder-failed: the embedding API returned an error search could not recover from`
  - `yomihon search: embedder-rejected: the embedding API refused the credential`
  - `yomihon search: rate-limited: the embedding API is rate-limiting; try again shortly`
  - `yomihon search: stale-partial: the semantic index is missing vectors for changed notes`
  - `yomihon search: capacity: the semantic index could not be loaded into memory`
  A rejection's stderr carries that sentence and nothing else — the
  provider's own response body is never forwarded (it could echo the
  submitted text). Goldens pin one example of each legal pair, both
  unanswerable capability-error bodies, **the internal-error body
  (`{"internal_error":{"detail":"the request could not be formed correctly"}}`)**,
  and the non-JSON silent-stdout shape; separate
  assertions pin exit codes and the exact stderr bytes for each — including
  the exit-1 line `yomihon search: internal: the request could not be
  formed correctly`.

**Collapsing rules** (each labeled with its authority; each is scoped to
the gate stage it implements, so no two rules can claim the same cell):

- R1 (gate 1) — privacy capability missing/invalid: all Part II behavior
  off, fail-closed [EXPLICIT-RULING D50.10]. **Every CLI request** —
  `--semantic` or plain — emits no result payload, exit 3, the fixed
  privacy stderr [EXPLICIT-RULING 2026-07-13: the CLI is an agent
  surface, and without a privacy authority the agent corpus cannot be
  computed]. Local human surfaces (UI, live fragment) continue lexical
  with a named diagnostic [CANON-DERIVED, Part I availability — local
  rendering is not egress].
- R2 (gate 2) — artifact policy missing/invalid, metadata-bearing
  queries only: unanswerable before any embedding — UI shows the
  capability diagnostic; CLI exits 3 with `metadata-filters-unavailable`
  and no `results` field [CANON-DERIVED from the shipped Part I refusal:
  a filter is never ignored and zero is never faked]. Bare-text and
  `folder:` queries pass this stage.
- R2′ (gate 4) — artifact policy missing/invalid, **text-bearing** queries
  requesting semantic: the semantic corpus is instance-scoped and cannot
  exist — UI lexical + diagnostic; CLI `--semantic` exits 3 with
  `artifact-policy-unavailable`, body carrying the lexical results
  [CANON-DERIVED D47 + gate order]. Without `--semantic` these queries
  are ordinary lexical, exit 0 [CANON-DERIVED Part I]. A **pure-filter
  query — including `folder:`-only — never reaches this stage**: it
  fails applicability at gate 3 first (R4), so it can never be answered
  twice.
- R3 — the live fragment is lexical in every cell [EXPLICIT-RULING
  D50.1].
- R4 (gate 3) — pure-filter (any filter-only query, `folder:` included,
  per Part I §5's definition) and empty-text queries never embed; with
  `--semantic` they answer `lexical/not-applicable`, exit 0
  [EXPLICIT-RULING 2026-07-13] — **reachable only when gates 1–2 passed,
  and it terminates the semantic path**: no stage-4 or stage-5 condition
  (artifact, cache, capacity, credential, network) can re-answer a query
  that carries nothing to embed.
- R5 — no best-effort surface exists [EXPLICIT-RULING D50.7].
- R6 (**gate 4**) — capacity/build failure of the query engine: semantic
  `unavailable` with reason `capacity`; lexical serving unaffected; the
  query is never embedded. Reachable **only for a request that explicitly
  asked for semantic and carries text** — a plain lexical query (no
  `--semantic`) is answered `lexical/off`, exit 0, whatever the engine's
  capacity, because nothing it needed failed [CANON-DERIVED H13, wire
  shape 2026-07-13; scope corrected 2026-07-13].
- R7 (**gate 4b**) — no embedding key configured: semantic `unavailable`
  with reason `embedder-unconfigured`; same scope as R6 (text-bearing ∧
  explicitly semantic); no client, no egress; reading unaffected
  [EXPLICIT-RULING 2026-07-13].

**Core table** (privacy valid ∧ artifact valid ∧ semantic applicable;
surfaces = UI explicit semantic search, CLI `--semantic` strict). **Four
axes**, enumerated — *index state* (what the engine holds), *background*
(its domain depends on the index state — four substates for an incomplete
index: refreshing / backing-off / stalled / absent; five for a complete
one, adding `idle`; *absent* = no builder, as with a standalone CLI or a
serve process that has not started one; the **background pipeline is its
own axis**, D50's amendment requires its substates listed, not folded),
*configuration* (is an embedding key present), and *query API* (the state
of the single call a query embedding would make; it shares no latch with
the background — D50.6).

**Configuration ownership differs by surface, and the column means the
key that surface would use.** The UI is served by a serve process, and
that same process owns the builder — so for the UI a present key implies
a builder can run: the two background substates that *require* embedding,
`refreshing` and `backing-off`, are unreachable while unconfigured (a
refreshing background proves a key). What remains for `unconfigured` on
the UI is the **key-free** subset of that index state's domain — `stalled`
and `absent` for every index state, **plus `idle` for a complete index**
(a complete index with no key simply sits idle) — and `configured` pairs
with the full domain. The strict CLI reads a cache some *external* serve
process built and holds its **own** key for the query call — so its
configuration axis is independent of that serve's builder, and all
background substates pair with either config value. The rows below mark
the unreachable UI combinations rather than pretending they exist.

Two facts collapse the product honestly, and are stated rather than
assumed:

- **Background never changes what is served.** It changes only how long a
  state persists and what the UI's pending-count does. So each (index ×
  query-API) row below carries a *background column* naming every
  substate and its one observable difference; no row hides one.
- **The strict CLI stops at gate 4 whenever the index is not complete**
  (D50.5), so for that surface the query API is never consulted in the
  cold, stale-partial, mismatch, retired, or capacity states — the API
  column is marked *n/a (gate 4)* there, and nothing is embedded.

**Index-state resolution** is a pure function, evaluated in this order
(closing the row-4/5 overlap): identity mismatch → index absent →
embedder-for-the-serving-epoch retired → capacity failure → incomplete
(stale-partial) → complete. The first match names the state and its
`coverage.reason`.

The **background** column's domain depends on the index state: a
non-complete index has **four** substates (refreshing / backing-off /
stalled / absent — a builder either progressing, waiting, stuck, or
never present), while a **complete** index has **five** (idle /
refreshing-the-next-epoch / backing-off / stalled / absent — it adds
"idle", nothing to do). They differ only in whether and how a rebuild is
progressing (which the UI reports), never in what is served — an
invariant asserted as its own lock (H10). The **configuration** column is
reachable only where the gate reaches stage 4b; the **query API** column
only where it reaches stage 5.

| # | Index state | Background | Config | Query API | UI (submitted) | CLI strict |
|---|---|---|---|---|---|---|
| 1 | **mismatch** — cache built under another identity | refreshing / backing-off / stalled / **absent (no builder: standalone CLI, or serve with none)** | *n/a (gate 4)* | *n/a* — never embedded | lexical + `cache-mismatch` diagnostic | exit 3 `cache-mismatch` |
| 2 | **absent** — no index | all four (with *absent* the UI says "not building" rather than "building") | *n/a (gate 4)* | *n/a* | lexical + `cache-cold` diagnostic | exit 3 `cache-cold` |
| 3 | **embedder-retired** — the serving epoch's model is gone, no successor epoch yet | all four | *n/a (gate 4)* | *n/a* | lexical + `embedder-retired` diagnostic | exit 3 `embedder-retired` |
| 4 | **capacity** — index exists, cannot be loaded/built in memory (H13) | all four | *n/a (gate 4)* | *n/a* | lexical + `capacity` diagnostic | exit 3 `capacity` |
| 5 | **stale-partial** — some notes missing vectors | UI: **stalled / absent only** (unconfigured × refreshing is unreachable — a refreshing background proves a key). CLI: any (its config is independent) | unconfigured | *n/a (gate 4b for UI; gate 4 for CLI)* | lexical + `embedder-unconfigured` diagnostic | exit 3 `stale-partial` — the CLI stopped at gate 4 first, so its own config is never read |
| 6 | stale-partial | all four | configured | **UI only** `up` (the CLI is *n/a* — it stopped at gate 4 and never consults the API) | hybrid over the unmasked set + pending-count indicator | exit 3 `stale-partial` + `masked_notes` (*gate 4: strict requires a complete index; the API is never consulted*) |
| 7 | stale-partial | all four | configured | **UI only** (the CLI is *n/a* — it stopped at gate 4): down / 429 / rejected / server-error / unknown-or-unclassifiable / confirmed-malformed | UI: lexical + the matching indicator, the reason being the query-API outcome — `embedder-unreachable` / `rate-limited` / `embedder-rejected` / `embedder-failed` (both server-error and any unknown-or-unclassifiable response), or, for a confirmed-malformed request, an internal-error diagnostic while lexical reading continues | exit 3 `stale-partial` — **unchanged**; the CLI stopped at gate 4 and never reached the API, so no query-API outcome (including unclassifiable) can change its reason |
| 8 | **complete** (or complete-on-the-old-epoch during a managed cutover, old embedder alive) | UI: **idle / stalled / absent only** (unconfigured rules out refreshing-next-epoch and backing-off — both require a key). CLI: any (independent key) | unconfigured | *n/a (gate 4b for UI; gate 4 does not apply — index is complete — so CLI reaches 4b too)* | lexical + `embedder-unconfigured` diagnostic | exit 3 `embedder-unconfigured` |
| 9 | complete | all five substates | configured | up | hybrid | hybrid, exit 0 |
| 10 | complete | all five | configured | down | lexical + offline indicator | exit 3 `embedder-unreachable`, body = lexical results |
| 11 | complete | all five | configured | 429 | lexical + rate-limited indicator | exit 3 `rate-limited`, fail-fast, body = lexical results |
| 12 | complete | all five | configured | **rejected** (the provider refused the credential) | lexical + `embedder-rejected` indicator | exit 3 `embedder-rejected`, body = lexical results; no retry, no provider body forwarded |
| 13 | complete | all five | configured | **server error / unknown / unclassifiable** (provider 5xx / INTERNAL / any response we cannot confidently attribute to our own request) | lexical + `embedder-failed` indicator | exit 3 `embedder-failed`, body = lexical results |
| 14 | complete | all five | configured | **confirmed-malformed request** (a class the docs pin unambiguously to a yomihon-formed bad request) | lexical reading continues + internal-error diagnostic | **exit 1** internal error — `{"internal_error":{"detail":"the request could not be formed correctly"}}` (compact, no `results`, no `error`), stderr exactly `yomihon search: internal: the request could not be formed correctly` |

Row 14 is the one exit-1 wire shape (the answerable/error envelopes are for
exit 0 and exit 3; a build defect is neither). The unknown/unclassifiable
case is **row 13, not row 14** (Koopa's ruling: uncertainty defaults to the
provider's fault, semantic off, never a claimed yomihon error).

Authorities, per row: 1 [CD H4 identity + ER gate order] · 2 [CD §4a
"cold → loud" + ER gate order] · 3 [ER D50.2 + ER gate order] · 4 [CD H13
+ ER gate order] · 5, 8 [ER 2026-07-13 configuration preflight] · 6 [ER
D50.5 (CLI exit 3 on stale-partial) + CD §4a (UI never blank) + ER gate
order] · 7 [ER D50.5 (CLI stops at gate 4, so its exit-3 stale-partial
reason is unchanged by any query-API outcome) + ER gate order; the UI's
down / 429 / rejected / server-error / unknown-or-unclassifiable /
confirmed-malformed outcomes = CD §4a / ER D50.6 / ER 2026-07-13 credential
taxonomy / ER 2026-07-13 provider-fault (embedder-failed) / ER 2026-07-13
provider-fault (embedder-failed) / ER 2026-07-13 confirmed-malformed
(internal error) respectively] · 9 [CD §4a; the cutover half ER D50.2] · 10 [CD §4a
"unreachable → loud"] · 11 [ER D50.6] · 12, 13, 14 [ER 2026-07-13
Koopa's credential + provider-fault taxonomy].

Rows 1–4 all present a cold face but carry distinct reasons, so the
observable is never ambiguous. Each numbered row is an acceptance test.
UI indicator texts are locked strings. **NEEDS-RULING = 0** as of the
2026-07-13 rulings (recorded in D50's amendment); the final review
re-audits per row.

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
- **Two identities, not one** (the eval fixture and the p95 observer need
  different guarantees). Both are computed from the **raw protocol
  components** H4 pins — the model, the dimension, the query prompt
  structure, the document prompt structure, the request cardinality, the
  response handling (vector extraction + normalization), the preprocess
  rules, and the cache format version — *not* from the already-synthesized
  protocol-epoch hash (a hash cannot be projected). The **full cache
  identity** hashes all of those plus the corpus-specific fields (vault
  root, chunker epoch); it is exactly the H4 identity, component for
  component. The **query-vector compatibility identity** hashes only the
  components that decide whether a query vector is numerically comparable
  to a corpus vector — model, dimension, query prompt structure, and
  response handling (a query and a corpus extracted or normalized
  differently do not compare) — and deliberately excludes the document
  prompt structure, the request cardinality (a single query is always one
  input), the format version (the live query vector is never read from the
  cache file), the chunker epoch, and the vault root. The eval harness needs the **full
  cache identity**: it scores recorded query vectors against **recorded
  corpus vectors**, and reds on any mismatch (never scoring new code
  against another epoch's vectors). The p95 observer needs only the
  **query-vector compatibility identity**: it replays recorded query
  vectors against the **live** real corpus purely to time the top-k path,
  and asserts only that the query vectors' compatibility identity matches
  the live corpus's — the vault root and chunker epoch differ by design,
  and that is not a mismatch because the observer measures latency, never
  relevance. (The two prompt structures are `gemini-embedding-2`'s query
  and document forms — pinned at the protocol step from its own docs, not
  the predecessor's `RETRIEVAL_QUERY`/`RETRIEVAL_DOCUMENT` task-type
  fields, which this model does not accept.) Refreshing the fixture is a
  two-sided change: the update commit carries the paired before/after
  per-query diff.
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
influence lock (H5); the final-send revalidation choke lock; **the
linearized-publication race lock** — a deterministic test hook admits a
reclassification (or purge) between a response's arrival and its commit
and asserts the commit is discarded, no sleeps (H4); the stale-mask
(serve a modified note, assert its old vector is absent until refresh);
cache-identity mismatch is cold, including a response-handling-only change;
malformed-row-is-cold and purge-on-reclassify; the epoch atomic swap and
its old-embedder guard (D50.2); filters-as-hard-constraints (a filtered-out
semantic candidate never fuses); lexical completeness past the fusion depth
(`--limit` beyond 50 answers); fusion determinism (the CLI golden bytes);
**every legal JSON pair, both unanswerable capability-error bodies, the
byte-exact internal-error body
(`{"internal_error":{"detail":"the request could not be formed correctly"}}`),
and the non-JSON silent-stdout shape**, each with its exit code, its
compact byte framing (§the JSON contract), and its exact stderr line; **the
escape-surface lock** — a fixture answer whose fields carry CJK, `<`/`>`/`&`,
a raw U+2028 and U+2029, a literal backslash-`u2028`, and a control
character is serialized and its bytes asserted: CJK and `<>&` raw, the two
line separators raw (the shipped `unescapeLineSeparators` step ran), the
literal backslash-`u2028` untouched, and the control still `\u`-escaped —
the same divergence the judge wire pins, not a paraphrase of it; **no
envelope on any path carries the query text** (a sentinel query through
every exit path, JSON and not, asserts absence); **the
background-invariance lock** — for each index state, its **reachable**
background substates are driven (the full domain when configured — four
for a non-complete index, five for complete; and **for the UI when
unconfigured**, only the key-free subset: `stalled / absent` for a
non-complete index, `idle / stalled / absent` for a complete one, since
`refreshing` and `backing-off` need a key the UI's serve process would
also use) and the **result projection** (the ordered result list and its
channels — not the whole rendered UI, whose pending-count text is allowed
to differ) is asserted byte-identical across them; **and the fixture
harness refuses to construct an unreachable combination — but only for
the UI surface**, whose builder and query key are the same serve-process
key: an `unconfigured × refreshing` or `unconfigured × backing-off` **UI**
state fails the test setup, so no lock passes over a world the ownership
rule
forbids. The **CLI** surface holds an independent key, so
`unconfigured × refreshing` is legal for it (an external serve is
refreshing while the CLI's own call has no key) and the refusal does not
apply — the harness scopes the refusal to the UI, never rejecting a valid
CLI combination; **the configuration-preflight lock** — with no key, a
recording client factory asserts it was **never called** (not merely that
zero requests were sent — the client is never constructed) for every
surface, a plain lexical query still answers exit 0, and the serve
startup diagnostic is asserted to appear **exactly once** and to contain
neither the key nor any query; **the single-send lock** — the count is
taken at the **HTTP boundary, not a client method** (a
`http.RoundTripper`-level counter, or a local stub HTTP server): SDK
retries live below the method call, so counting the method would miss
them. Automatic retry is **disabled** at construction; the lock asserts
that one explicit action produces exactly one outbound HTTP request in
**every terminal state — success, rejection, throttle, transport failure,
provider server error (5xx), a confirmed-malformed request, and a
synthetic unknown status** (the fault-ownership terminals and the
single-send count are locked by the same table, so a 5xx path that
silently re-sent the query would fail here even though its taxonomy
outcome is correct) — and that two explicit actions produce two requests
through one long-lived client (proving no cross-request auth latch and no
retry amplification of the query bytes); **the taxonomy-totality lock** — a
table with one row per documented provider error class, asserting the
mapping is total, splits by fault ownership, and yields exactly one local
outcome each: provider-fault classes (authorization refusal →
`embedder-rejected`; throttle → `rate-limited`; transport non-answer →
`embedder-unreachable`; provider server error, **and every unknown or
unclassifiable status** → `embedder-failed`) each produce exit 3 with the
lexical answer preserved; **only a class the docs pin unambiguously to a
yomihon-formed malformed request** produces exit 1 — a synthetic unknown
status is included and asserted to land on **exit 3 `embedder-failed`, not
exit 1** (Koopa's ruling: uncertainty defaults to the provider's fault),
while a synthetic confirmed-malformed class is asserted to land on exit 1
with the internal_error envelope; the eval harness failing on
identity mismatch; and the raw-query-absence lock inherited from the
shipped log unit, extended to B's new paths. The CLI goldens are frozen
contract per D37 — H7's JSON is the spec they pin.

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
  At the 5,937-chunk figure: **34.79 MiB** at 1536, **69.57 MiB** at 3072
  (recomputed 2026-07-13 — the first pass rounded these wrong). A managed
  cutover holds two epochs: peak is double. At the 18k-note extrapolation
  (≈2.4×10⁵ chunks): ≈1.37 GiB at 1536 steady — which is why that horizon
  is rung 2 territory by design (H3), not a rung-1 promise.
- **Durable cache (disk)**: the vector payload dominates; a base64
  text-row format costs 4/3 of raw — **46.38 MiB** at 1536, **92.77 MiB**
  at 3072, before row metadata; two epochs may coexist during cutover, so
  disk peak is likewise double. A binary-payload engine (the bake-off's
  SQLite or packed-generation candidates) stores raw and drops the 4/3
  factor.
- **Capacity failure is loud**: if the engine cannot build its matrix
  (allocation failure, corrupt cache larger than expected), the semantic
  channel reports `unavailable` with a named diagnostic and lexical
  serving continues — never a crash of the reading face, never a silent
  half-loaded matrix.
- **The p95 observer, fully specified** (the rung-1→2 trigger is
  observable, not folklore):
  - *Compatibility, not full identity*: the observer asserts only the
    query-vector compatibility identity (H9) against the live corpus —
    the vault root and chunker epoch differ by design, and that is not a
    mismatch, because the measurement is latency, not relevance.
  - *Workload*, reproducible by construction and **embedding-free**: the
    committed eval set's 40 query **vectors are read from the H9 recorded
    fixture** — the observer never calls the embedding API, so a
    measurement is not an egress event and needs no key. Each vector runs
    the full top-k path (cosine over every chunk → max-aggregation → top
    50 notes) **three times in one fixed order (query 1×3, query 2×3, …)
    → 120 timings**, single-threaded, no I/O; the first repetition of the
    first query is discarded as warm-up and re-run at the end, so the
    sample count is exactly 120.
  - *Estimator*: nearest-rank p95 — sort the 120 samples ascending and
    take index `ceil(0.95 × 120) = 114` (1-based). No interpolation.
  - *Where*: (a) `BenchmarkSemanticTopK` in the repo, over a synthetic
    corpus of pinned size, under the standing benchstat discipline;
    (b) the serve process runs the same measurement over the **real**
    corpus at the end of every epoch build and logs
    `semantic top-k p95 chunks=<n> dim=<d> samples=<s> p95_ms=<x>` —
    owner: the
    scanner; cadence: once per epoch build.
  - *Trigger*: the D32 rung-1→2 condition (≈10⁵ chunks or p95 > ~100 ms)
    is read straight off that log line.

## H14. The §5a obligations, dispositioned

| Obligation (roadmap §5a, B) | Status |
|---|---|
| Chunking rules, chunks-per-note assumption stated | MET (H3: exact formulas, named Unicode boundaries, bounded provider fallback, two dated measurements) |
| Cache file format versioned by (model, dim) | MET, widened (H4: full identity tuple incl. protocol epoch; engine via bake-off) |
| RRF specifics (k, depths, aggregation) | MET (H6; shipped lexical ordering preserved) |
| §4a degraded matrix as acceptance cases | MET (H7: the gate incl. the semantic-request fork and the configuration preflight, the scoped rules R1–R7, the four-axis rows 1–14 incl. server-error/unknown → `embedder-failed` and the single confirmed-malformed exit-1 row with per-surface reachability and per-row authority, the three discriminated envelopes — answerable / unanswerable-capability / internal-error — and the frozen wire vocabulary; the unreachable UI config×background combinations are excluded by row and enforced by the invariance lock's setup refusal). The final review audits this claim; it was PARTIAL while ruling families were open, and would be again if a reachable cell lost its answer. |
| The eval set | MET (H9; synthetic-in-repo per D50.8) |
| Egress guard test (Diary never reaches the embedder) | MET, widened (H5: five flows + influence lock + the linearized-publication race lock; the log flow already shipped) |

This table is a claim, not a certificate: it is MET only if the scoped
review agrees per row. Its own history is the warning — the first
revision marked the matrix MET while it still carried unclosed cells.

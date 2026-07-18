# Search face — implementation plan (spec §3)

> Status: Part I (the lexical engine, /search page, ⌘K shell, and the live
> lexical-results enhancement) is **built and merged**; it is kept as the
> record of that plan. Part II (§§H1–H14) is the hybrid extension's plan,
> written to the D50 ruling sheet and hardened through the 2026-07-13 scoped
> adversarial sequence. The last product ruling added Koopa's
> **fault-ownership rule**:
> only a response yomihon can confirm it formed wrongly is an internal
> error (exit 1); every provider fault and every unknown/unclassifiable
> response is a semantic `unavailable` (exit 3, lexical preserved), the
> default for uncertainty being the provider's fault, never a claimed
> yomihon bug. Three discriminated envelopes (answerable / capability-
> unanswerable / internal-error) told apart by their top-level key (exit 3
> covers two of them); the CLI-only matrix carries rows 1–24 plus the explicit
> build acceptance set, with every
> reachable state named; the single-send lock covers
> every terminal including the fault-ownership ones. Dispatch gate
> (D50.10): the vault contract's privacy capability must land before any
> Part II behavior. D50.11 closes the provider boundary: semantic search is
> optional and BYOK-only, Koopa's live deployment uses his own paid project,
> and an API key is not an offline build or test prerequisite. The degraded
> matrix itself has no open cell.
> This refines `spec.md` §3 and `design.md` §6–7 into a concrete plan. The
> Part I lexical index is one of the three in-memory shared Snapshot models
> (D24/D25). Part II is deliberately separate: explicit CLI actions use one
> local SQLite generation store plus a per-process immutable RAM vector index;
> `serve` and the ~2s Snapshot scanner never touch it. The four walls
> (`CLAUDE.md`) and the dependency boundary (`spec.md` §0.1) govern everything
> below.

## 1. Shape, in one line

An in-memory index of `~419` notes (a few MB), rebuilt from the vault inside
the shared Snapshot, queried by a deterministic NFC-folded substring match plus
six structured filters. No database, no new dependency, no daemon, no Docker in
the tests.

## 2. Where it lives (D25 — the shared Snapshot)

`internal/search` owns the index and query; it does **not** own freshness.
`internal/snapshot` holds an opaque `View` behind an `atomic.Pointer[View]` and
runs one scanner about every 2 seconds. A changed rooted enumeration causes one
captured-input build: each Markdown note and owned sidecar is read at most once,
then graph, navigation, lexical search, lesson, concept, and render projections
are built from those same bytes and the pointer is swapped once. Navigation
roles and source-bound artifact policy are derived from the startup schema; a
rescan validates the policy source but does not decode a new contract. A handler
reads the pointer once, calls `View.Capture`, and queries the read-only projection
methods such as `snap.Search()`. This closes both the old restart-only freshness
gap and the independent-walk torn-generation gap (D25).

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
func fold(s string) string { return strings.ToLower(vault.NormalizeNFC(s)) }
```

- **NFC on the Go side, at index time and query time.** `Title`/`PlainText` are
  stored NFC (original case); `TitleFold`/`PlainFold`/the query token are folded.
  Without NFC on *both* sides, NFD-form vault content is unfindable.
- **Case folding lives only here** (Go `strings.ToLower`), so "what counts as a
  match" has exactly one definer. `internal/graph`'s `normalize` also folds, but
  it is trim+NFC+**lower**; search needs NFC-only for the stored display copy, so
  keep the vault's NFC step as a shared primitive (`vault.NormalizeNFC`) and
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

- Each ~2s reconciliation performs one descriptor-rooted enumeration and
  compares canonical paths, raw filesystem spelling, metadata, object identity,
  and parent-chain identity with the previously published scan. No change and
  no pending retry means no file reads or rebuild.
- On change, Snapshot reads every observed Markdown note and owned lesson
  sidecar at most once. It parses those captured bytes once and supplies the
  resulting immutable inputs to graph, navigation, lexical search, lesson,
  concept, and render projections. Search still indexes non-instance documents
  for bare-text and `folder:` lookup while marking them ineligible for metadata
  projections. No projection reopens a path, and one atomic publication means a
  request cannot see a graph, navigation model, and search index from different
  filesystem moments.
- A nested scan or read problem marks the candidate incomplete. Startup may
  publish an available partial generation so reading remains available; a later
  incomplete rescan retains the last published generation and retries. A
  successful retry replaces the whole generation. Delete and rename therefore
  require no incremental mutation protocol.

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
  concurrent read during a `snapshot.View` swap is race-free under `-race`.
- A real-vault test (`t.Skipf` when `~/obsidian` absent): the index builds, a
  known term hits an expected note.

## 11. Suggested implementation order (dependency order, not a fence)

1. Use `vault.NormalizeNFC`; add `fold` to `search`.
2. `plain_text` extraction over the render AST (+ its extraction-table tests).
3. The `Index` type + build-from-notes (test against the real vault, eyeball).
4. The query parser (pure function, fully tested).
5. The query/match + two-bucket ordering.
6. `internal/snapshot`: the `View`, the `atomic.Pointer`, the ~2s scanner
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

# Part II — the hybrid extension (D32 as amended by D50; revised 2026-07-14)

## H1. Shape, in one line

A second retrieval channel — `gemini-embedding-2` embeddings (D50.9; the
prior generation's retirement window opened before build, and embedding
generations are incompatible, so the corpus embeds once on the new model)
over heading-bounded chunks, exact cosine top-k in memory — fused with the
shipped lexical channel by RRF for the explicit CLI/agent surface. Ordinary
human search (`/search`, ⌘K, and live results) stays lexical-only; a future
Related/Find-related exploration surface requires its own ruling and may not
be mixed into ordinary search (D50 amendment, 2026-07-13).

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
  rendering is not egress) and never reads the embedding corpus, vector
  cache, embedding key, or provider state.
- **The fused set is not globally instance-only.** D47 keeps non-instance
  notes in bare-text and `folder:` lexical search, so such a note may enter a
  hybrid answer from the lexical channel and carries only `lexical` in
  `channels` / `channel_ranks`; it can never receive a semantic rank. A
  metadata-dependent filter still excludes it as D47 requires. This asymmetric
  channel capability preserves the shipped lexical contract; silently dropping
  the lexical-only note would be the contradiction.
- Degraded or source-stale artifact policy disables the semantic channel along
  with the other instance projections (as shipped); a missing or invalid **privacy
  capability, or one made stale by a changed source contract, fail-closes all
  of Part II (D50.10) — Part I lexical
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
  both. An independent 2026-07-13 recheck against the then-current, dirty
  vault working tree (base `ef0ae5449bbc`, so deliberately not presented as
  replayable commit evidence) measured **502 eligible notes, 7,790 sections,
  1,294 empty sections, and 6,496 kept chunks ≈12.94/note**. That pass also
  measured 32 no-frontmatter notes, one table-dominant note, the same
  313-line longest fence and a 5,967-token largest unsplit section. After the
  provider prompt was corrected, an aggregate-only rerun over the same 502-note
  corpus measured the exact submitted-prefix cost at **46 proxy tokens p95 / 92
  maximum**, with 6,496 chunks and zero cap failures. The working assumption therefore
  still holds on the current shape, and the CJK title/heading prefix is small
  beside the model limit rather than free. The 18k-note extrapolation is
  ≈2.4×10⁵ chunks — **beyond the
  rung-1→2 trigger (D32: ~10⁵ chunks or p95 exact scan > ~100 ms) by
  ~2.4×**, so at that horizon rung 2 is the designed path, not a
  surprise; H13 carries the envelope and specifies the p95 measurement.
- **Cap**: `cap = floor(0.9 × model_input_limit)` proxy tokens, **the complete
  provider prompt included** (`title: Title › H2 › H3 | text: `, with
  ` — part n/m` appended to the context title for continuations). The
  successor's current official limit is 8,192 tokens, so the
  build-time value is 7,372; the protocol step re-verifies that limit, and a
  provider change enters the protocol/chunker identity rather than silently
  changing the cap.
- **Token counter**, the exact integer formula (table-tested):
  `tokens(s) = cjk_count(s) + ceil(other_count(s) / 4)`, where
  `cjk_count` counts runes satisfying `unicode.Is` on the `Han`,
  `Hiragana`, or `Katakana` range tables, plus the explicit code-point
  intervals U+3000–U+303F (CJK symbols and punctuation) and
  U+FF00–U+FFEF (halfwidth/fullwidth forms); `other_count` is every
  remaining rune. The boundary is these named tables and intervals —
  nothing vaguer.
  The 10% margin absorbs proxy error; the count-tokens API is not called
  per chunk. The provider contract exposes no stable structured reason that
  distinguishes an oversized input from another rejected request, so this
  plan does **not** parse provider message text or retry a rejected body by
  splitting it after the response. A provider rejection follows H4's total
  fault taxonomy and leaves the epoch incomplete, loudly. If recorded local
  measurements show the proxy under-counts systematically, the margin
  constant widens before submission: a measured constant change, not a
  message-driven retry protocol.
- **Oversize fallback ladder**, in order: paragraph split → line split
  (tables and lists split at row/item boundaries) → hard rune split as the
  terminal case (a 313-line single fence exists in this vault). A fence
  splits at its own line boundaries and is never merged with prose across
  the cap. Continuation chunks carry `— part n/m` in their prefix. A prefix
  that alone consumes the cap is not fixable by splitting its body: it is a
  named failed chunk and makes the epoch incomplete, never an infinite split
  and never a silent title/heading truncation. That terminal is a table case.
  This exact asymmetric document form is the successor's official retrieval
  structure, pinned by `docs/semantic-provider-protocol.md`; the earlier
  `Title › H2 › H3 — body` prototype was contextual content, not yet a valid
  provider-protocol decision, and has been replaced before live egress.

## H4. The embedding pipeline and cache

- **`serve` is structurally outside semantic search.** It receives no provider
  client, key, vector store, document builder, or semantic query dependency.
  A cold cache or incompatible identity is built only by the explicit
  `yomihon search-index build` command. Once a compatible active generation
  exists, an explicit `yomihon search --semantic` action first reconciles
  current vault drift: it reuses unchanged rows, embeds changed/new eligible
  chunks, removes deleted/newly-ineligible paths locally, and activates a
  complete current generation before sending the query. Thus routine use
  self-refreshes without making the first semantic query an implicit
  whole-vault upload. The query text is still sent **at most once** per process
  action and only after reconciliation succeeds. Ordinary UI search has no
  query embedder or semantic path.
- **Attempt budgets are surface-specific.** The direct REST client has no hidden
  retry, follows no redirect, ignores environment proxy variables through a
  dedicated direct transport (`Proxy=nil`), and each method call performs one
  HTTP send. A future proxy is a new egress route and requires a ruling rather
  than inheriting `HTTPS_PROXY` silently.
  Interactive reconciliation inside `search --semantic` is fail-fast: one
  attempt per missing chunk, no sleep or retry, just like its later one-attempt
  query call. Before any document send it computes the entire missing work;
  more than **128 chunks or 100,000 submitted proxy tokens** is
  `rebuild-required`, with zero document/query egress. Only the explicit
  `search-index build` command owns the five-attempt 429 budget: it durably
  reserves each attempt in the matching staging generation before sending, so
  a crash cannot create a sixth attempt. A valid `Retry-After` up to 30s is
  honored; a longer value records `retry_not_before` and exits rather than
  blocking indefinitely. An absent/invalid value uses 1s, 4s, 9s, then 16s.
  The staging generation may resume only with the same identity and target
  corpus manifest; exhaustion leaves it incomplete and never active.
- **The embedding protocol is pinned from the successor model's own
  documentation** (the D50.9 amendment, 2026-07-13 — the first revision
  carried the predecessor generation's semantics, a verified error). What
  is already known from the official contract: `gemini-embedding-2` takes
  no task-type field; output at a truncated dimension is normalized by the
  API (no client-side renormalization); multi-input request semantics
  differ from the predecessor, so this design sends **one chunk per
  request row and never relies on any server-side aggregation**. The
  completed protocol record is `docs/semantic-provider-protocol.md`: document
  bytes are `title: {Title › heading ancestry[ — part n/m]} | text: {body}`;
  query bytes are `task: search result | query: {bare text}`. `bare text` is
  the original-case, original-Unicode bare tokens in input order joined by one
  ASCII space; recognized structured-filter tokens are excluded because they
  are local hard constraints, not retrieval prose. The REST request has
  exactly one content/part, carries `autoTruncate=false` subject to H11's
  synthetic live acceptance gate, and explicitly requests the identity
  dimension; response handling extracts one exact-size
  finite vector and applies no client-side normalization. It also pins **the
  error taxonomy → gate-state mapping, total by construction and
  split by fault ownership** (Koopa's EXPLICIT-RULING, 2026-07-13; recorded
  in the D50 amendment) — every documented provider error class maps to
  exactly one local outcome, cited to the successor's own error docs rather
  than message text or a bare status number:
  - **Provider fault, and any unknown/unclassifiable response → semantic
    `unavailable`, exit 3, the lexical answer preserved** — the default for
    uncertainty is "the provider's problem," never a claimed yomihon fault:
    agreeing `UNAUTHENTICATED`/401 or `PERMISSION_DENIED`/403 →
    `embedder-rejected` (gate 5, non-retryable); agreeing
    `RESOURCE_EXHAUSTED`/429 → `rate-limited`; a transport non-answer →
    `embedder-unreachable`; a provider server error (its `INTERNAL` / 5xx
    class) → `embedder-failed`; **and every remaining documented class the
    mapping cannot confidently attribute to our own request, plus every
    unknown or undocumented status → `embedder-failed`** (the catch-all is
    provider-side, not ours; its diagnostic says only that the API returned
    an error search could not recover from — it does not assert a
    server-side cause it cannot confirm).
  - **Only a response we can confirm is a yomihon-formed malformed
    request → the command's internal error, exit 1** (loud, because it is a
    build defect): agreeing `INVALID_ARGUMENT`/400, which the successor's
    troubleshooting contract attributes unambiguously to a malformed request
    body. Ambiguous-origin classes
    are NOT forced here — when we cannot confirm the fault is ours, the
    catch-all above applies.
  The mapping's totality is itself asserted (H10). Because status class does
  not by itself prove fault ownership, the protocol step records, per class,
  the docs' stated origin and the outcome it maps to — the acceptance
  verifies each attribution against the successor's own documentation, and
  a class the docs do not clearly pin to a client-side malformed request
  defaults to `embedder-failed`.
- **Generation identity** is the tuple `(model, dimension, protocol epoch,
  chunker epoch, vector-format version, vault root, corpus-policy
  fingerprint)`. The SQLite container's `PRAGMA user_version` is a separate
  store-schema version; it does not decide numerical comparability.
  The protocol epoch hashes the pinned request templates, request cardinality,
  response handling, and preprocess rules. The corpus-policy fingerprint
  hashes the normalized artifact and privacy capabilities that decide
  embedding eligibility. Any component mismatching means this cache is not this
  corpus's cache — cold, never a partial read and never a silent reuse.
  A response-handling change alone therefore colds the cache: vectors
  extracted under different handling never silently mix. The full
  preprocess (prefix rule, drop rule, fallback ladder) is part of the
  chunker epoch.
- **CLI-action-owned policy, fail-closed on source drift.** `serve` does not
  derive or carry a Part II capability. Each `search`/`search-index` process
  loads the contract once and does not hot-reload it, while
  `internal/schema` binds artifact/privacy capabilities to the exact source
  bytes. Source freshness is rechecked before every document send, before
  activation, and before query send. A change makes the action unavailable,
  rejects any in-flight candidate, and leaves active/previous untouched; a
  later process derives the new policy fingerprint and cannot admit a prior
  generation with a different identity. This is fail-closed invalidation, not
  live reinterpretation of the contract [CANON-DERIVED from wall 3, D47, and
  D50.10].
- **Logical row, corpus manifest, and hydration join.** Every hydrated logical
  row carries `rel_path`, the complete note-content hash, chunk ordinal, a hash
  of the exact submitted document bytes, and the vector. The normalized durable
  representation stores one `(generation_id, rel_path, note_hash)` note row and
  joins its generation-owned chunk rows by `(generation_id, rel_path)`. A
  generation also carries
  the SHA-256 of the sorted current corpus tuple
  `(rel_path, note_hash, ordinal, submitted_hash)` and its expected row count.
  Every semantic CLI action reads/chunks the eligible corpus locally and admits
  an active generation only when identity, corpus digest, count, and every row
  validate. A compatible digest mismatch starts bounded reconciliation:
  identical submitted bytes may reuse their vector, while the new row records
  the current path/hash/ordinal; changed/new chunks embed and deleted/ineligible
  chunks simply do not enter staging. Heading/snippet evidence always comes
  from that action's current local chunk, never SQLite. Immediately before
  activation the whole corpus is re-read and its digest must still equal the
  target; mismatch is `vault-changed`, leaves active byte-identical, and sends
  no query. The same action snapshot supplies ranking evidence. Filesystem
  edits cannot share SQLite's transaction, so a later edit is detected by the
  next action rather than falsely described as globally atomic.
- **Publication contract.** Only explicit CLI actions can be writers. A stable
  sibling lease supplies a nonblocking OS-released single-writer boundary.
  The stable cache parent owns `semantic.lock` and the private `semantic/`
  child; the lease is deliberately outside that replaceable child. Each reader
  or writer holds descriptor-rooted parent and child capabilities for its
  lifetime, and every Go-level create/remove/reset is relative to those roots.
  The main database inode is pinned too. Because modernc SQLite's default VFS
  still opens the database and WAL/SHM names by pathname, each attach and every
  API operation that can report a read, mutation, reset, or publication success
  is bracketed by parent/child/database identity checks; namespace drift fails
  closed rather than publishing or reporting a detached generation. This is a
  correctness and privacy lock for the owner-trusted local process, not a claim
  that yomihon can resist a malicious process already running as the same UID.
  Such a process can already edit the vault and read the user's credentials;
  expanding that threat model requires OS sandboxing or privilege separation,
  not another pathname check. Losing the lease causes one active-state re-read,
  so a writer that just finished is
  used rather than misreported as `index-refreshing`. If the target is still
  stale, the action exits with that typed reason. The database, WAL/SHM
  sidecars are `0600` beneath the `0700` store child; the sibling lease is
  `0600`, and its stable cache parent may be `0700` or a non-group/world-
  writable system cache directory. Active and
  previous generations are immutable; at most one staging generation is
  mutable. A staging generation is resumed only when its complete identity,
  target corpus digest, expected row count, policy-source freshness, and retry
  ledger match the current action. Otherwise the writer discards it locally
  before any egress. Activation is one `synchronous=FULL` transaction:
  validate staging completeness, set `previous=active`, `active=staging`,
  clear staging, and delete every unreferenced generation. Readers in a
  transaction observe either the old or new catalog/rows, never a torn mix.
  Interruption or provider/publication failure never changes active. Corruption
  is scoped by role: malformed active is `cache-corrupt` and never silently
  falls back to previous; malformed staging is discarded by the next writer;
  malformed previous cannot poison a valid active and is pruned by a writer.
  Before admitting a current-schema store, the writer runs a generated,
  sqlc-owned logical-foreign-key check covering every catalog role and every
  manifest/retry relationship in the schema. Any dangling role or orphan row is
  `cache-corrupt`: an ordinary writer fails closed, while an explicit build
  resets and bootstraps the store under the same rooted writer lease.
- **Final-send revalidation**: eligibility (instance ∧ non-private) is
  re-checked against the current snapshot at the choke point that performs
  the network send — a note reclassified between collection and send is
  dropped there, and the guard locks target that choke point (H5), not the
  collector alone. (Send-side and publish-side revalidation are two
  separate gates; each has its own lock.)
- **Two layers, named by function rather than L1/L2.** The *generation store*
  is plain SQLite through `database/sql`, `modernc.org/sqlite`, and feature-
  local sqlc output in `internal/search/semantic/catalog`; schema bootstrap and
  connection PRAGMAs plus the pre-schema `user_version`/`sqlite_schema`
  discovery needed before generated queries can assume a schema are the only
  handwritten SQL exceptions. Current-schema shape validation and every
  domain read/write remain in feature-local sqlc queries. The *immutable
  vector index* is rebuilt per CLI process from one admitted active generation:
  hydration uses 256-row keyset pages, validates the complete manifest, and
  transfers each vector slice into the index so there is no second full matrix.
  It performs exact cosine top-k without a mutex or eviction. Ristretto would
  make recall depend on eviction and has no useful cross-invocation lifetime;
  Badger would replace, not complement, SQLite's transaction/catalog role.
- **Pinned SQLite format.** `generations` carries the full H4 identity plus
  `target_corpus_fingerprint`, `expected_chunks`, and the content-free
  `top_k_p95_us` measurement. A singleton `catalog`
  names nullable, distinct active/previous/staging generation IDs.
  Generation-owned `notes` are keyed by `(generation_id, rel_path)` and carry
  `note_hash`; `chunks` are keyed by `(generation_id, rel_path, ordinal)`,
  reference their note, and carry `submitted_hash` plus nullable `vector`
  (`NULL` only while that exact staging target is pending). A staging retry
  ledger has a composite foreign key to the exact chunk and records reserved
  attempt count plus `retry_not_before`. All five tables are SQLite `STRICT`;
  hashes are raw 32-byte SHA-256, text is validated as UTF-8 on hydration, and
  every path/identity field is revalidated before admission.
  Vectors are little-endian IEEE-754 float32; the schema rejects non-BLOB,
  empty, or non-four-byte-aligned values, and hydration additionally requires
  exactly `dimension × 4` bytes with no non-finite component. The store schema
  is identified by `PRAGMA user_version`; a per-generation
  `vector_format_version` remains part of generation compatibility. Hydration
  reads catalog, generation identity, note join, chunks, and retry count in one
  read transaction. SQLite uses WAL, foreign keys, and `synchronous=FULL` on
  its sole writer connection; read handles are read-only/query-only. Schema
  bootstrap is one transaction. There is no in-place migration ladder and no
  golang-migrate: an explicit build fully replaces an unknown/incompatible
  schema. Before a future known-compatible schema bump raises `user_version`,
  it must ship a version-specific copy-forward reader that writes and validates
  a new file, preserving a vector only when full generation identity,
  vector-format version, and exact submitted bytes still agree. Version 1 has
  no predecessor importer. A numerical vector-format mismatch always rebuilds.
- **Platform boundary (D50 amendment, 2026-07-14).** Version 1's owner-only
  permission/lease proof and runtime support are limited to Darwin and Linux.
  On Windows
  every store entry point returns
  `ErrStoreUnsupportedPlatform` before filesystem, SQLite, key, or provider
  access. This leaves `serve`, UI, judge, and plain lexical CLI untouched. A
  future Windows store requires a ruled DACL model and runtime tests; synthetic
  mode bits or an unexercised `LockFileEx` path are not a privacy proof. Other
  targets carry no compile/runtime support promise in the first implementation; being
  Unix-like is not enough when the selected SQLite dependency itself may not
  compile there.
- **Generation cutover** (D50.2, amended 2026-07-14): one process owns one
  complete generation identity and queries only a compatible active generation.
  An explicit incompatible build leaves the old active generation
  transactionally intact while staging is incomplete, but the new-identity
  process does not keep or select a retired model/protocol client to query it.
  Semantic search exits 3 with `cache-mismatch` until the replacement activates.
  `previous` is a publication-retention slot; it is not an automatic ranking
  fallback or a user-visible rollback command. A query vector
  never scores against another numerical identity.
- **Key**: `YOMIHON_EMBED_KEY`, read lazily only after a semantic CLI action
  passes applicability and index/corpus preflight, then passed down as a value
  and sent only as a request header. `serve` and plain lexical search never
  read it. It joins the env
  wall-lock allowlist in the same PR — a deliberate, test-visible edit —
  and its absence from stdout, stderr, cache bytes, and error text is
  itself a lock (H5).

## H5. The egress guards (five flows, each with its own lock, closed before production network dispatch)

**One network owner, with an honest enumerated static backstop.** All outbound
embedding transport code lives in the provider client; the semantic builder
and agent search action receive only the narrow capability they consume and
never the API key or a transport. Fusion receives vectors, never an embedder.
The UI, search handler,
and live-fragment packages receive neither an embedder interface nor a vector
cache and have no semantic control. A production-source boundary test rejects
the enumerated direct `net/http`, `net`, and `crypto/tls` constructors outside
the provider owner and rejects any semantic
dependency crossing into the ordinary UI. It is driven red by both an
alternate raw-HTTP sender in the pipeline and an embedder field added to a UI
handler. It is a mechanical backstop, not a claim that static AST inspection
can prove the absence of every SDK, syscall, cgo/unsafe, or `os/exec` route.
The narrow capability graph, direct-transport rule, dependency review, and
recording HTTP-boundary tests together prove the supported provider client's behavior;
any new outbound dependency is a review event.

1. **Document flow**: a collect-level lock (a fixture holding private
   files, templates, and instance notes; the candidate set names exactly
   instance ∧ non-private) **and** a choke-point lock (a recording transport
   plus H4 revalidation: nothing sends that the current action no longer
   allows — reclassify or edit a note between collect and send, and the send
   must not happen). A separate source-drift row changes the contract after
   collection and asserts zero further sends, no activation, and unchanged
   active bytes. A corpus-manifest race adds/removes/edits a note after the
   last provider response but before activation and proves `vault-changed`, no
   query send, and no catalog flip.
2. **Query flow** (D50.1): only explicit CLI `--semantic` embeds, at most one
   send per process action. `/search`, ⌘K, and the live fragment cannot reach
   the embedder structurally: their handlers and models hold no embedder,
   vector cache, or semantic option. A behavioral lock drives submitted and
   live UI searches with a recording transport and asserts zero requests.
   Pure-filter and empty-text CLI queries never embed.
3. **Logs and metrics**: raw query text appears in no log, cache, error,
   metric, or trace. The shipped handler lines that logged full queries
   were removed as their own merged unit (2026-07-13), with a
   recording-logger lock under error injection asserting absence; B's
   surfaces inherit that lock and extend it to the new paths. The protocol
   step also inventories and disables any SDK request/response-body logging,
   debug dump, tracing payload, or telemetry transport; success and every
   error terminal run with distinct document/query sentinels while the test
   captures application logs, stderr, and every outbound HTTP request. A
   client whose implicit observability cannot be disabled and observed at
   that boundary is not admissible. An exact production-source inventory also
   pins every current direct `Write`/`Encode`, `fmt.Fprint*`, `io.WriteString`,
   and file-opening/write primitive in the agent and semantic packages to its
   named output, digest, store, or lease owner; a new direct sink or a second
   write at an existing owner fails until it is reviewed and the inventory is
   deliberately amended. Like the network-owner lock, this is a supported-path
   mechanical backstop, not a claim that AST inspection proves arbitrary syscall,
   cgo, or unsafe data flow.
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
  Chunk ties inside a note break by chunk ordinal. Equal note-level cosine
  scores break by rel-path **before** assigning 1-based positional channel
  ranks; ranks are never shared, so the RRF input is total before fusion.
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

**Surfaces.** `/search`, ⌘K, and the live-results fragment are lexical-only,
permanently. They expose no semantic toggle, query egress, vector-cache state,
embedding-key state, or provider diagnostic. If a future human fuzzy-
exploration need appears, it is a separately ruled Related/Find-related
surface and may not be mixed into ordinary search. `yomihon search` CLI:
`yomihon search [--json] [--limit N] [--semantic] [--root PATH] QUERY...` —
lexical by default; `--semantic` is strict and there is no best-effort form
(D50.7). A compatible active generation is checked against the action's current
corpus; bounded drift may cause this explicit semantic action to resume/create
staging, embed changed/new chunks, and activate a complete generation **before**
its one query send. Cold/incompatible state never auto-builds.

The argv contract is frozen: flags may appear before or between positional
tokens; `--` ends flag parsing; one or more positionals are joined by one ASCII
space (a single quoted empty positional remains the explicit empty query).
When `--root` is absent, both search commands use `YOMIHON_ROOT` and then
`~/obsidian`, exactly like `serve`; unlike the repository-oriented judge
commands, vault search is expected to work from any working directory, and a
cwd default would silently fork cache identity by invocation location.
`--json` and `--semantic` are idempotent booleans and take no value;
`--root` and `--limit` may each appear once. The default limit is 20 and the
valid range is 1–1000. Zero positionals, a duplicate value flag, an unknown
flag, an invalid limit, or a missing flag value is exit 2 with empty stdout and
one `yomihon search: ` stderr line. Query input is at most 4096 bytes and may
contain no C0/C1 control character.

**The precedence gate** (ruled 2026-07-13; every request walks it in this
order, and the first failing stage names the reason — nothing later runs.
Amended after the scoped round: each stage now states exactly which
queries it can fail, so the gate and the collapsing rules cannot disagree):

1. **privacy** — capability missing / invalid / source-stale. Fails **every CLI request**
   (the CLI is an agent surface; with no privacy authority the agent
   corpus cannot be computed, so no payload may leave — `--semantic` or
   not). Ordinary UI search does not enter this agent-output gate and follows
   its shipped lexical capability rules.
2. **metadata answerability** — artifact policy missing / invalid /
   source-stale fails
   **only metadata-bearing queries** (a filter that cannot be evaluated
   makes the query unanswerable; never ignored, never faked as zero —
   the shipped Part I behavior). Bare-text and `folder:` queries pass.
3. **semantic applicability** — pure-filter or empty-text queries carry
   nothing to embed: with `--semantic` the channel is *not applicable*,
   never "degraded" — exit 0. Reachable only after stages 1–2 pass.
3b. **semantic request** — the fork that stops every non-semantic request
   here, so no later stage can touch it. A CLI request that did **not** ask
   for semantic (no `--semantic`) is answered lexically now: `lexical/off`, exit 0,
   whatever the index holds. **Stages 4, 4b, and 5 are reached only by a
   text-bearing request that explicitly asked for semantic** — so cold,
   mismatch, retirement, capacity, a missing key, and every query-API
   failure are, by construction, unreachable for a plain lexical query.
   This is the single place the gate guarantees what R6/R7 assert.
4. **semantic corpus and active generation** — artifact-policy loss,
   unsupported store platform, missing/corrupt active state, an incompatible
   identity, or an unavailable serving protocol stops here with no
   document/query egress. Platform support is checked before any stat, mkdir,
   SQLite open, key read, or provider construction; on Windows it is
   `unsupported-platform`. A cold,
   corrupt, or incompatible store names the explicit `search-index build`
   recovery. A compatible generation is joined to the current corpus:
   - digest equal → it is current and may continue;
   - digest different → compute exact missing rows/tokens before egress;
     crossing 128 rows or 100,000 proxy tokens is `rebuild-required`;
   - within budget → configuration preflight, then nonblocking writer lease.
     Losing the lease triggers one active re-read: a now-current generation
     continues, otherwise `index-refreshing`. The winner re-reads active and
     corpus under the lease, resumes only matching staging work, performs
     single-attempt document calls, revalidates the entire corpus/policies,
     activates a complete generation, then continues. Any provider failure,
     local incomplete chunk, or `vault-changed` leaves active unchanged and
     **the query is never embedded**. Capacity failure while loading a current
     active generation is also a stage-4 terminal.
4b. **configuration preflight** — reached only by a text-bearing explicit
   semantic request after stage 4 has established either a current generation
   or bounded reconcilable work. With no key, `embedder-unconfigured`: no
   provider client is constructed and no document/query is sent. `serve`,
   plain lexical, and not-applicable requests never read the key.
5. **query API** — only a request that has a complete, current, loaded
   generation and passed 4b embeds the lossless bare-text projection; at
   most one call per explicit action, no in-place retry. Its outcome maps
   by the H4 fault-ownership taxonomy: a credential refusal →
   `embedder-rejected`; an unanswered call → `embedder-unreachable`; a
   throttle → `rate-limited` (fail-fast, D50.6); a provider server error
   **or any unknown/unclassifiable response** → `embedder-failed`; **only
   a response the docs confirm is a yomihon-formed malformed request →
   internal error, exit 1** (row 12). Every provider-fault outcome is exit
   3 with the lexical answer preserved (never a claimed yomihon bug). The
   class → outcome mapping is pinned at the protocol step from the
   successor's own docs, not a hardcoded status number. **No cross-request
   auth latch is authorized**: each explicit action is judged on its own
   call. Introducing one would need its own lifecycle and matrix states,
   ruled separately.

**JSON contract** (frozen at build, golden-pinned; the D37 rule). **No
envelope has a query-echo field or copies raw query text into diagnostics**
(ruled 2026-07-13 — the caller already knows its input, and copying it into an
error surface would violate D50.1 and H5.4). Result evidence may naturally
contain matching bytes from the returned note; that is answer content, not a
query echo. Three discriminated envelopes,
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
  `detail` is a fixed string, never copied from the query text (which would be an
  egress into an error surface, forbidden by D50.1 / H5.4).
- **Non-JSON mode**: an unanswerable or internal-error request prints
  **nothing** on stdout and prints its one frozen stderr line (exit 3 or
  exit 1 respectively).
- **Killed best-effort spelling**: `--semantic=best-effort` is not a fifth
  surface and never enters the gate. It is a usage error: exit 2, empty
  stdout (including when `--json` is also present), no query egress, and
  stderr exactly `yomihon search: flag --semantic takes no value`. A golden
  pins those bytes, so R5 closes the old best-effort cross-product rather
  than merely omitting it.
- **Other exit-2 failures** follow the shipped `runJudge` code family
  [REAL-OBSERVED in `cmd/yomihon/main.go`]: argument/flag validation and a
  local tool/setup failure that prevents the command from establishing an
  answer (for example an unreadable root)
  exit 2, with empty stdout even when `--json` was requested and one stderr
  line prefixed `yomihon search: `. They carry no JSON envelope; the three
  envelopes above are result/capability/internal-result contracts, not parser
  or process-I/O reports. Deterministic parser cases are byte-golden-pinned;
  an OS-supplied error suffix is not falsely claimed portable. No exit-2 line
  may contain the query or key. A stdout write failure also returns 2, matching
  the shipped family, but no plan can promise atomic process output after the
  OS has accepted only a prefix: that partial stdout is explicitly invalid and
  must be discarded by every nonzero-exit consumer.

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
  nothing to explain) and for `ok`. A typed object `{reason}`; `reason` is one
  of the frozen strings
  (`not-applicable`, `artifact-policy-unavailable`, `cache-cold`,
  `cache-corrupt`, `cache-mismatch`, `embedder-retired`, `capacity`,
  `unsupported-platform`, `embedder-unconfigured`, `rebuild-required`, `index-refreshing`,
  `index-incomplete`, `vault-changed`, `embedder-unreachable`,
  `embedder-rejected`, `embedder-failed`, `rate-limited`). No reason carries
  paths, submitted text, the query, or provider response bytes.
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
  2 = usage / local tool failure before an answer exists, 3 = a required
  capability is unavailable. An internal error is
  a confirmed-malformed request — a build defect. The machine-
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
  - `yomihon search: cache-cold: no semantic index exists; run yomihon search-index build`
  - `yomihon search: cache-corrupt: the semantic index is unreadable; run yomihon search-index build`
  - `yomihon search: cache-mismatch: the semantic index uses a different configuration; run yomihon search-index build`
  - `yomihon search: embedder-retired: the old index's embedding model is no longer available`
  - `yomihon search: embedder-unconfigured: no embedding key is configured, so semantic search is off`
  - `yomihon search: embedder-unreachable: the embedding API did not answer`
  - `yomihon search: embedder-failed: the embedding API returned an error search could not recover from`
  - `yomihon search: embedder-rejected: the embedding API refused the credential`
  - `yomihon search: rate-limited: the embedding API is rate-limiting; try again shortly`
  - `yomihon search: rebuild-required: the vault changed too much for an interactive refresh; run yomihon search-index build`
  - `yomihon search: index-refreshing: another process is updating the semantic index`
  - `yomihon search: index-incomplete: one or more current chunks could not be indexed`
  - `yomihon search: vault-changed: the vault changed while the semantic index was being updated; retry`
  - `yomihon search: capacity: the semantic index could not be loaded into memory`
  - `yomihon search: unsupported-platform: the semantic generation store is not supported on this platform`
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

- R1 (gate 1) — privacy capability missing / invalid / source-stale: all Part II behavior
  off, fail-closed [EXPLICIT-RULING D50.10]. **Every CLI request** —
  `--semantic` or plain — emits no result payload, exit 3, the fixed
  privacy stderr [EXPLICIT-RULING 2026-07-13: the CLI is an agent
  surface, and without a privacy authority the agent corpus cannot be
  computed]. Ordinary UI search is outside Part II and retains Part I's
  lexical behavior.
- R2 (gate 2) — artifact policy missing / invalid / source-stale, metadata-bearing
  queries only: unanswerable before any embedding — CLI exits 3 with
  `metadata-filters-unavailable` and no `results` field [CANON-DERIVED from the shipped Part I refusal:
  a filter is never ignored and zero is never faked]. Bare-text and
  `folder:` queries pass this stage.
- R2′ (gate 4) — artifact policy missing / invalid / source-stale,
  **text-bearing** queries
  requesting semantic: the semantic corpus is instance-scoped and cannot
  exist — CLI `--semantic` exits 3 with
  `artifact-policy-unavailable`, body carrying the lexical results
  [CANON-DERIVED D47 + gate order]. Without `--semantic` these queries
  are ordinary lexical, exit 0 [CANON-DERIVED Part I]. A **pure-filter
  query — including `folder:`-only — never reaches this stage**: it
  fails applicability at gate 3 first (R4), so it can never be answered
  twice.
- R3 — ordinary `/search`, ⌘K, and live-fragment search are lexical-only and
  never enter this semantic gate [EXPLICIT-RULING D50 amendment,
  2026-07-13]. This does not weaken Part I metadata answerability: an
  unavailable filter is still never ignored or faked as zero.
- R4 (gate 3) — pure-filter (any filter-only query, `folder:` included,
  per Part I §5's definition) and empty-text queries never embed; with
  `--semantic` they answer `lexical/not-applicable`, exit 0
  [EXPLICIT-RULING 2026-07-13] — **reachable only when gates 1–2 passed,
  and it terminates the semantic path**: no stage-4 or stage-5 condition
  (artifact, cache, capacity, credential, network) can re-answer a query
  that carries nothing to embed.
- R5 — no best-effort surface exists [EXPLICIT-RULING D50.7]. The killed
  spelling is the exit-2 usage shape frozen above; all of its former matrix
  cells collapse before gate 1 and perform no egress.
- R6 (**gate 4**) — capacity/build failure of the query engine: semantic
  `unavailable` with reason `capacity`; lexical serving unaffected; the
  query is never embedded. Reachable **only for a request that explicitly
  asked for semantic and carries text** — a plain lexical query (no
  `--semantic`) is answered `lexical/off`, exit 0, whatever the engine's
  capacity, because nothing it needed failed [CANON-DERIVED from spec §0.1,
  roadmap §4a, and D50.7; wire shape and scope corrected 2026-07-13].
- R7 (**gate 4b**) — no embedding key configured: semantic `unavailable`
  with reason `embedder-unconfigured`; same scope as R6 (text-bearing ∧
  explicitly semantic); no client, no egress; reading unaffected
  [EXPLICIT-RULING 2026-07-13].
- R8 (**gate 4 compatible-drift branch**) — changed work above either
  interactive bound is `rebuild-required` before egress; a held writer is
  re-read once and then either current or `index-refreshing`; local chunk
  incompleteness is `index-incomplete`; a final corpus/policy mismatch is
  `vault-changed`. Every terminal preserves lexical results, exits 3, never
  activates staging, and never sends the query. Document-provider failures use
  the same provider-owned reason taxonomy as gate 5. Interactive document
  calls never retry [EXPLICIT-RULING D50 final implementation clarification].
- R9 (**gate 4 platform preflight**) — semantic storage is unavailable on
  Windows in the first store implementation. A text-bearing explicit semantic search preserves lexical results,
  exits 3 with `unsupported-platform`, and performs zero store/key/provider
  work; `search-index build` uses the no-result error envelope and the same
  exit/reason. Plain lexical and not-applicable requests already terminated at
  gate 3/3b and cannot observe this state [EXPLICIT-RULING D50 amendment,
  2026-07-14].

**Core table** (strict CLI `--semantic`; privacy valid ∧ artifact valid ∧
semantic applicable). Ordinary UI/`serve` are structurally absent. The axes are
*active generation*, *interactive work*, *CLI configuration*, *writer lease*,
*document API*, and *query API*. Collapsing follows gate order, so cells after
the first terminal are unreachable rather than unspecified.

| # | Active / work state | Config / writer / API | CLI result | Authority |
|---|---|---|---|---|
| 0 | store platform unsupported | later axes n/a | exit 3 `unsupported-platform`; lexical for search, no payload for build | ER D50 2026-07-14 amendment |
| 1 | active absent | later axes n/a | exit 3 `cache-cold` | CD roadmap §4a + ER gate |
| 2 | catalog/active corrupt, or active has no production p95 measurement | later axes n/a | exit 3 `cache-corrupt` | ER generation clarification + D50 2026-07-14 p95 clarification |
| 3 | active identity incompatible | later axes n/a | exit 3 `cache-mismatch` | ER D50.2/final amendment |
| 4 | active identity names a retired query protocol | later axes n/a | exit 3 `embedder-retired` | ER D50.2 |
| 5 | current active cannot load | later axes n/a | exit 3 `capacity` | CD spec §0.1/roadmap §4a |
| 6 | current active | key absent | exit 3 `embedder-unconfigured` | ER configuration preflight |
| 7 | current active | query up | `hybrid/ok`, exit 0 | CD roadmap §4a |
| 8 | current active | query transport non-answer | exit 3 `embedder-unreachable`, lexical | ER fault taxonomy |
| 9 | current active | query 429 | exit 3 `rate-limited`, lexical | ER D50.6 |
| 10 | current active | query credential rejected | exit 3 `embedder-rejected`, lexical | ER credential taxonomy |
| 11 | current active | query provider/unknown | exit 3 `embedder-failed`, lexical | ER fault-ownership taxonomy |
| 12 | current active | confirmed locally malformed query request | exit 1 `internal_error` | ER fault-ownership taxonomy |
| 13 | compatible drift above either bound | no key/lease/API read | exit 3 `rebuild-required`, lexical | ER final clarification |
| 14 | bounded compatible drift | key absent | exit 3 `embedder-unconfigured`, lexical | ER gate order |
| 15 | bounded drift; lease lost; re-read still stale | writer held | exit 3 `index-refreshing`, lexical | ER final amendment/clarification |
| 16 | bounded drift; lease lost; re-read now current | query rows 7–12 | no spurious writer failure | ER final clarification |
| 17 | bounded drift | local chunk failure | exit 3 `index-incomplete`, lexical; no activation/query | CD completeness + ER final amendment |
| 18 | bounded drift | staging retry time is future, or document 429 | exit 3 `rate-limited`, lexical; no query | ER D50.6/clarification |
| 19 | bounded drift | document transport non-answer | exit 3 `embedder-unreachable`, lexical; no query | ER fault taxonomy |
| 20 | bounded drift | document credential rejected | exit 3 `embedder-rejected`, lexical; no query | ER credential taxonomy |
| 21 | bounded drift | document provider/unknown | exit 3 `embedder-failed`, lexical; no query | ER fault-ownership taxonomy |
| 22 | bounded drift | confirmed locally malformed document request | exit 1 `internal_error`; no activation/query | ER fault-ownership taxonomy |
| 23 | bounded drift completed | final manifest/policy differs | exit 3 `vault-changed`, lexical; no activation/query | ER final clarification |
| 24 | bounded drift activated | query rows 7–12 | complete current generation only | ER final amendment |

Unknown/unclassifiable provider outcomes are rows 11/21, never internal error.
A current active generation is queryable even while an unrelated writer stages
another target. Cold/mismatch never auto-build. The build command below owns
its own explicit retry and publication surface. **Matrix NEEDS-RULING = 0**;
every numbered row is an acceptance case.

**Explicit build command (D37-frozen before implementation).**
`yomihon search-index build [--json] [--root PATH]` accepts no positional
arguments and no semantic/limit flag. Boolean/value parsing, `--`, duplicate
value flags, exit-2 handling, 4096-byte/path safety, and stdout framing follow
the search parser rules above. It loads privacy/artifact authority, resolves
the current identity/corpus, and is idempotent: a current active generation
performs zero provider sends and succeeds as `current`. Otherwise it acquires
the writer lease, re-reads state, resumes only an exactly matching staging
generation, and builds to completion with the explicit five-attempt 429
policy. Ctrl-C/context cancellation leaves active untouched and resumable
staging admitted only under the H4 identity/manifest rules.

- JSON success is exactly one compact object plus `\n`, field order frozen:
  `{"status":"current","chunks":N,"embedded":0,"reused":N,"top_k_p95_us":P}`
  or `{"status":"built","chunks":N,"embedded":M,"reused":R,"top_k_p95_us":P}`.
- Human success is one stdout line:
  `semantic index current: N chunks` or
  `semantic index built: N chunks (M embedded, R reused)`.
- JSON mode emits no success progress. Human mode may emit only content-free
  `yomihon search-index: embedded M/N chunks` progress on stderr, at each 100
  newly embedded rows and completion; it never prints paths, prompts, key,
  retry bodies, or token text.
- Exit 3 uses the same byte-exact `error.reason` envelope and provider/capability
  stderr vocabulary as search, plus `index-refreshing`, `index-incomplete`,
  `vault-changed`, and `unsupported-platform`; there is no result payload. The
  platform line is exactly `yomihon search-index: unsupported-platform: the semantic generation store is not supported on this platform` plus `\n`. Exit 1 uses the same
  `internal_error` body. Usage, local filesystem/SQLite failure, stdout failure,
  or interruption is exit 2 with empty stdout and one
  `yomihon search-index: ` stderr line. A committed activation is success even
  if later best-effort WAL checkpoint maintenance fails.

The build acceptance set includes current/no-send, cold full build,
incompatible full build, compatible reuse, exact staging resume, stale staging
discard, writer held, every document-provider terminal, persisted future
`retry_not_before`, attempt exhaustion, local chunk failure, final
`vault-changed`, interruption before/after row writes, activation rollback,
old-or-new reader snapshots during commit, and active+previous+staging pruning.

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
- **Vector provenance has one separately ruled synthetic certification path
  (D57); it does not widen user-query egress.** The opt-in test-only capture
  driver lives with the provider owner and may send only the exact repo-owned
  synthetic corpus and 40 synthetic queries. It accepts no raw text or vault
  root from arguments or environment; only a local output path and the ruled
  candidate dimension vary. Each row is one direct request with retry,
  redirect, and environment proxy disabled. The driver is not linked into the
  product binary and cannot capture a real vault. It validates the completed
  artifact before writing it owner-only. Release/production dispatch refuses
  a missing or compatibility-mismatched committed recording. The unexported
  test-only builder state may persist `top_k_p95_us=0`, meaning *unmeasured*;
  no production generation or successful build wire may expose zero, and every
  measured value is stored as at least 1 μs. The dimension-pair run uses these
  synthetic rows. The real-vault
  harness is stricter: it accepts only query vectors recorded from prior
  explicit CLI `--semantic` searches under the matching compatibility identity;
  it accepts no raw query text and has no embedder/client dependency, so
  starting a 40-row evaluation cannot itself send 40 query texts. D50.8
  authorizes local eval artifacts, while D57 alone authorizes the fixed
  synthetic provider certification action; neither creates a hidden
  arbitrary-query egress surface.
- **Per query, pinned**: every required positive must appear in the top 5;
  rank 1 must be a required or explicitly acceptable positive; and a named
  related sibling is **contrastive**, not forbidden — if it appears, it must
  rank below every required positive. The fixture also pins the tie rule at
  rank 5 and the recall denominator. Structured-filter violations remain a
  separate hard failure across the complete result list, not merely the top 5.
- **Two identities, not one** (the eval fixture and the p95 observer need
  different guarantees). Both are computed from the **raw identity
  components** H4 pins — the model, the dimension, the query prompt
  structure, the document prompt structure, the request cardinality, the
  response handling (vector extraction + normalization), the preprocess
  rules, the vector-format version, and the corpus-policy fingerprint — *not*
  from the already-synthesized
  protocol-epoch hash (a hash cannot be projected). The **full cache
  identity** hashes all of those plus the corpus-specific fields (vault
  root, chunker epoch); it is exactly the H4 identity, component for
  component. The **query-vector compatibility identity** hashes only the
  components that decide whether a query vector is numerically comparable
  to a corpus vector — model, dimension, query prompt structure, and
  response handling (a query and a corpus extracted or normalized
  differently do not compare) — and deliberately excludes the document
  prompt structure, the request cardinality (a single query is always one
  input), the vector-format version (the live query vector is never read from
  the durable generation), the chunker epoch, the corpus-policy fingerprint,
  and the vault
  root. The eval harness needs the **full
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
- **Dimension** (D50.9 + H12.1): **1,536 is pinned**. The live paired
  `gemini-embedding-2` comparison gave both 1,536 and 3,072 the same 40/40
  required-positive rank-1 result and the same 40/40 contrast-below-positive
  result. With no measured retrieval benefit, 3,072 would only double vector
  payload and exact-scan work. Dimension 1,536 is therefore part of the cache
  identity and the committed recording.
- Framing: a regression floor, not a tuning target — a change that lowers
  recall@5, puts a non-positive at rank 1, ranks a contrastive sibling above
  any required positive, or violates a hard filter fails; raising the numbers
  is not itself a goal.

## H10. Locks and kill-tests (standards §2 discipline)

Every lock is watched red before the PR: the five egress-flow locks and the
influence lock (H5); **the exact network-capability boundary lock**: direct
HTTP/TCP/TLS sends, listeners, DNS lookups, client/transport methods, and their
owners and occurrence counts are frozen across semantic/agent production
source, while raw wire constructors and provider choke points have an
independent exact caller inventory. Adding a second provider-local send or
calling the raw wire from another production file is watched red. The
production-transport lock also points `HTTPS_PROXY` at a recorder and requires
zero proxy requests; the
final-send note-hash/eligibility lock;
**the corpus-wide activation race lock** (deterministically edit/add/remove a
note after the last response but before activation, assert `vault-changed`, no
query, unchanged catalog); active identity mismatch including response-
handling-only and bare-query-projection-only changes; per-role corruption
(active loud/no fallback, staging discarded, previous cannot poison active);
hydration rejects a row whose path, note hash, ordinal, submitted hash, vector
length, or finiteness is wrong; reader-during-activation observes wholly old or
new; injected failure at each activation statement rolls back; exactly one OS
writer lease holder; lease loss followed by one re-read covers both just-
completed and still-stale cases; exact staging resume admits only identical
identity/manifest/count/policy/retry state; active+previous+staging pruning;
retry attempts are durably reserved before HTTP send; and a committed
activation remains success when maintenance checkpoint is fault-injected;
filters-as-hard-constraints (a filtered-out semantic candidate never fuses)
and the **bare-query wire lock** (`深度 type:lesson 工作` sends exactly
`task: search result | query: 深度 工作`, preserving original case/Unicode and
omitting the filter); lexical completeness past the fusion depth
(`--limit` beyond 50 answers); fusion determinism (the CLI golden bytes);
**every legal JSON pair, both unanswerable capability-error bodies, the
byte-exact internal-error body
(`{"internal_error":{"detail":"the request could not be formed correctly"}}`),
the killed `--semantic=best-effort` exit-2 usage shape, and the non-JSON
silent-stdout shape**, each with its exit code, its
compact byte framing (H7's discriminated-envelope contract), and its exact
stderr line; pre-emission exit-2 parser/tool rows assert empty stdout, the
fixed prefix, and query/key absence (with exact bytes for deterministic parser
cases), while a fault-injected partial stdout write asserts exit 2 and that the
bytes are not treated as a valid envelope; **the
escape-surface lock** — a fixture answer whose fields carry each escape
class by name is serialized and its **wire bytes** asserted, each class a
separate watched-red:
  - CJK and `<` / `>` / `&` → raw UTF-8 (no `\uXXXX`);
  - U+2028 and U+2029 → raw UTF-8, i.e. the 3-byte sequences `e2 80 a8`
    and `e2 80 a9` (the shipped `unescapeLineSeparators` step ran, not
    `\u2028`/`\u2029`);
  - a **short-escape control**, a literal newline in a field → the two
    bytes `\` `n` on the wire (`\n`), not a raw 0x0a;
  - a **hex-escape control**, U+0000 in a field → the six bytes `\u0000`
    (JSON has no short form for it), proving controls without a short
    escape are still `\uXXXX`;
  - a **literal** backslash-`u2028` typed in the content (the string
    `\u2028`, a backslash then `u2028`) → on the wire as `\\u2028`
    (escaped backslash, then literal `u2028`), proving the rewrite touches
    only the real separators, never text that merely looks like their
    escape.
  This follows the shipped judge serializer byte path: judge-plan §3a pins
  compact JSON, the trailing newline, CJK, and `SetEscapeHTML(false)`, while
  `internal/judge.WriteJSONL` and its golden/fuzz locks pin the
  U+2028/U+2029 rewrite, control escaping, and literal backslash handling;
**no envelope has a dedicated query field and no non-result surface copies
the query text** (a sentinel query through every error, log, metric, trace,
and diagnostic path asserts absence; result evidence is allowed to match it
naturally); **the gate-matrix lock**
drives H7 rows 0–24 plus the explicit-build acceptance set and rejects any
later stage becoming reachable after an earlier terminal;
**the ordinary-UI-absence lock** rejects a semantic toggle, embedder, vector
cache, embedding-key read, or provider/cache diagnostic in `/search`, ⌘K, or
the live fragment, and a recording transport observes zero sends from both
submitted and live UI queries; **the configuration-preflight lock** — with no key, a
recording client factory asserts it was **never called** (not merely that
zero requests were sent — the client is never constructed) for CLI query
embedding or reconciliation, a plain lexical query still answers exit 0, and
starting `serve` under an environment containing/removing the key has
byte-identical behavior and no semantic log/cache open; **the single-send
lock** — the count is
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
retry amplification of the query bytes); the synthetic-fixture capture driver
has no network owner, refuses more than one query row per CLI process, and
asserts the completed document-cache build has stopped before query capture,
while the real-vault eval harness has no raw-query or embedder input at all;
the document-attempt-budget lock counts at the same HTTP boundary: interactive
reconcile sends each missing document at most once and crosses neither work
bound; explicit build drives every bounded 429 delay, future
`retry_not_before`, restart/resume, and exhaustion row, proving the durable
reservation prevents a sixth send and no hidden message-based size retry;
**the taxonomy-totality lock** — a
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
identity mismatch; and the no-query-echo lock inherited from the shipped log
unit, extended to B's non-result paths. The CLI goldens are frozen
contract per D37 — H7's JSON is the spec they pin.

The **rooted-store publication lock** pauses a writer after the stable sibling
lease, replaces the store child, and proves a second writer still receives
`ErrWriterHeld`; a symlink replacement produces `ErrStorePermissions` and no
database/WAL/SHM/journal in the trap. Separate deterministic hooks replace the
child between SQLite attach and explicit reset, and during activation before
commit: both actions fail closed and preserve the visible replacement file
byte-for-byte. Reader and writer `Active` paths reject a replaced child rather
than returning a detached generation. A bidirectional direct-import allowlist
rejects both a new semantic/agent production dependency and a stale approval,
  forcing every newly reachable filesystem, process, network, or observability
  capability back through boundary review. The exact inventories resolve
  selectors with `go/types`: `sync.Once.Do` and `http.Header.Get` cannot satisfy
  a network count, while `os.Root`, `os.File`, network receiver/interface
  methods, and promoted methods on the complete exported `vault.Reader` method
  set remain visible. Tracked package references still cover aliases, function
  values, dot imports, alternative standard-library path APIs, TLS key/file
  helpers, schema/recording loaders, and `ReadPrefix`-style capability methods.
  Every approved store, vault, and provider seam has an exact owner and
  occurrence count rather than a blanket exception. Additional Root access, a
  second HTTP send, and a promoted Reader method each have a watched-red
  mutation. A separate cross-build-tag
source-kind lock rejects assembly, cgo companion sources, SWIG inputs, and
precompiled system objects in semantic/agent packages, so the Go-AST locks
cannot be bypassed by adding a non-Go compilation unit. Timing sleeps are not
evidence. The store-corruption lock
independently injects dangling active/previous/staging roles and orphan
note/chunk/attempt rows: ordinary writer open must fail, and an explicit build
must rebuild cleanly under the lease.

The **Windows runtime lock** runs on `windows-latest`, exercises the package's
reader, ordinary-writer, and explicit-rebuild entry points, and asserts typed
`ErrStoreUnsupportedPlatform` plus absence of the parent directory, database,
lock, WAL, SHM, and journal. Cross-compilation is only a compile-portability
check and cannot substitute for that runtime proof. Before main dispatch, the
serve behavioral lock must also start the real serve path with a sentinel key
and store location and prove no semantic factory, store open/file creation,
provider send, or semantic diagnostic; the existing import boundary alone is
not sufficient once the thin `cmd/yomihon` dispatch imports
`internal/search/agent`.

## H11. Build order (dependency order, not a fence)

0. **Dispatch prerequisite, its own unit (D50.10)**: the vault contract's
   privacy capability — the `[privacy]` section, `internal/schema`'s
   derivation, action-snapshot carriage, and fail-closed degradation — lands
   and is accepted before a Part II command is linked into `main`, a live
   provider probe runs, or an index build can send vault bytes. Pure local
   construction and offline tests may proceed without changing the true vault;
   none of that makes Part II dispatchable. (The removal of the shipped
   raw-query log lines did not need to wait: it was a live Part I defect.)
0b. **Provider/key boundary (D32/D50.11; H12.5)**: no key is required to
    compile, run offline tests, perform the storage bake-off, or evaluate
    recorded vectors. Koopa supplies his paid-project key only when the
    network-client live verification and personal index build begin. A
    downstream user likewise supplies their own provider account and key and
    is responsible for its terms; no bundled/shared key or yomihon-operated
    proxy is a supported configuration. Before production dispatch, that key
    runs the two synthetic-only live protocol probes pinned in
    `semantic-provider-protocol.md`: the exact short direct request must
    succeed, and a count-tokens-confirmed over-limit ASCII request with
    `autoTruncate=false` must fail rather than return a truncated vector. This
    closes the verified conflict between the public REST discovery schema and
    current official SDK converters without sending vault or real query text.
1. Chunker over the existing extraction layer (+ the H3 rule table's
   tests, bound and ladder cases).
2. **Generation-store gate — completed 2026-07-14.** The corrected bake-off in
   `semantic-storage-bakeoff.md` runs the implemented sqlc-backed
   active/previous/staging lifecycle: exact-byte reuse plus one changed row,
   active-reader flip, interrupted staging/resume, activation rollback, GC,
   WAL/file high-water, and 256-row process-cold hydration. At 6,496 chunks ×
   1,536 dimensions the product path measured 1.840 s initial build, 1.025 s
   one-note drift, 74.20 ms process-cold complete hydration, 51.76 MiB one-role
   steady disk, and 210.73 MiB observed three-role peak including sidecars.
   The modernc/mattn comparison found mattn faster for build/drift but no
   proven cold-load difference; modernc remains selected for the optional
   local CLI's toolchain/release shape. The old mutable-row numbers are
   historical and not evidence for this lifecycle. Storage ownership stays
   package-private behind the concrete `Indexer`; generated queries do not
   create a domain interface.
3. The egress-guard locks (H5), all five flows — before production command
   composition can construct a client or any live probe can send.
4. Concrete embedder + CLI-owned full builder and bounded reconciler, staging
   resume, corpus-manifest activation guard, and their tests. The
   `YOMIHON_EMBED_KEY` wiring and env-wall allowlist change land in this same
   step; there is never an intermediate client build with an unreviewed env
   read.
5. Cosine top-k + fusion (+ filters-first, completeness, determinism, and
   the eval harness on recorded vectors; the dimension decision's paired
   run happens here).
6. Surface: the CLI with goldens and the exit taxonomy; structural and
   behavioral regressions prove ordinary `/search`, ⌘K, and live results stay
   lexical-only with no semantic dependency.

## H12. Resolved rulings

1. **Dimension** → **1,536**, pinned by the paired `gemini-embedding-2`
   eval-set comparison (D50.9): both candidates achieved the same 40/40
   positive rank-1 and contrast-order results, so 3,072 supplied no measured
   retrieval gain for twice the vector payload and exact-scan work.
2. **Representation** → chunk-only with max aggregation (D50.3; D32
   amended). Reopens only if the eval shows broad-topic recall failing —
   not on token-limit grounds.
3. **`--semantic=best-effort`** → killed (D50.7), here and in roadmap §4a.
4. **Query-text egress** → user-authored query text leaves only through an
   explicit CLI `--semantic` action (D50.1), one send per process action.
   D57's opt-in developer certification path is separate and accepts only
   fixed repo-owned synthetic bytes. Ordinary `/search`, ⌘K, and live results
   stay lexical; raw user query text enters no log, cache, error, metric, or
   trace. A future human Related/Find-related exploration surface needs a new
   ruling and is not authorized here.
5. **Provider account and distribution boundary** → **RULED (D50.11,
   Koopa 2026-07-13)**. Yomihon does not operate a hosted AI service.
   Semantic search is optional and BYOK-only: Koopa's live deployment uses
   his own paid Gemini project; each downstream user supplies their own
   provider account and key and is responsible for that provider's terms.
   Yomihon bundles no credential, shares no account, and operates no embedding
   proxy. The key is a live-use dependency, not a builder-dispatch or offline
   verification prerequisite. Provider-specific legal terms remain at the
   provider boundary and are not duplicated in this implementation plan.

The 2026-07-13 clarifications (Koopa, closing the delta round's open
cells; recorded in D50's amendment note): the walls text now names both
egress exceptions; `lexical/not-applicable` is the fourth legal pair
(exit 0, zero embedding); privacy-unavailable strict CLI emits no result
payload; the precedence gate is privacy → artifact → applicability →
semantic-request fork → cache → configuration → query API, with no query
egress before the final stage; the
Embedding-2 protocol re-pins from the successor's own documentation; and
identity mismatch and explicit staging are distinct storage states, but one
process queries only its compatible active generation; explicit CLI staging
does not authorize old-identity serving. The
final clarification adds bounded fail-fast interactive reconciliation,
`rebuild-required`, corpus-wide activation revalidation, resumable staging
admission, and the filter-free bare-query projection.

Open engineering call (not a ruling): the token-proxy constants (H3), decided
inside the build with measurements and reversible behind its interface. The
storage-engine call is closed by H4's 2026-07-13 bake-off.

## H13. Scale and capacity envelope (measurements through 2026-07-14)

- **Working set (RAM, the query engine)**: chunks × dim × 4 bytes.
  At the 5,937-chunk figure: **34.79 MiB** at 1536, **69.57 MiB** at 3072
  (recomputed 2026-07-13 — the first pass rounded these wrong). A query process
  hydrates in 256-row keyset pages, validates one complete generation, and
  transfers vector slices into one immutable exact index; it does not clone a
  second full matrix. Corpus/row metadata and one driver page are additional,
  and the measured process peak—not a doubled-payload formula—is authoritative.
  At the independent current-tree
  remeasurement of 6,496 chunks: **38.0625 MiB** at 1536 and **76.125 MiB** at
  3072. At the 18k-note extrapolation
  (≈2.4×10⁵ chunks): ≈1.37 GiB at 1536 steady — which is why that horizon
  is rung 2 territory by design (H3), not a rung-1 promise.
- **Durable cache (disk)**: the vector payload dominates; a base64
  text-row format costs 4/3 of raw — **46.38 MiB** at 1536, **92.77 MiB**
  at 3072 for 5,937 chunks, and **50.75 MiB / 101.50 MiB** respectively for
  6,496 chunks, before row metadata. Steady state retains active + previous;
  building may additionally retain one staging generation and WAL pages. The
  2026-07-14 implemented-store measurement at 6,496 × 1,536 observed **51.76
  MiB** for one cleanly closed active generation (**55.75 MiB** peak with
  sidecars), and **155.19 MiB** cleanly closed / **210.73 MiB** observed peak
  for active + previous + resumable staging; the peak included a **55.42 MiB
  WAL**. These are one-shot capacity observations with a 1 ms sampler, not a
  proof that no shorter filesystem transient exists. The selected SQLite BLOB
  format stores raw values and drops the base64 4/3 factor; the superseded
  mutable-row prototype is not evidence for this immutable lifecycle.
- **Provider work and paid-standard cost**: the no-failure baseline is one
  document request per kept chunk, so the current 6,496-chunk corpus starts at
  6,496 document requests; the 18k-note horizon starts at about 2.4×10⁵.
  The explicit build's persisted five-attempt budget makes the **hard
  per-generation ceilings 32,480 and 1.2×10⁶ requests**, respectively,
  including bounded 429 retries. Interactive reconcile is separately capped at
  128 one-attempt requests and 100,000 proxy tokens.
  Query work remains one request per explicit semantic action. At the
  7,372-token cap, the deliberately pessimistic no-retry current-epoch ceiling
  is 47,888,512 submitted proxy tokens;
  at the [2026-07-13 listed paid-standard rate](https://ai.google.dev/gemini-api/docs/pricing)
  of $0.20 / 1M text-input tokens, that is **≤$9.58** (and the 18k-note
  no-retry cap-everywhere ceiling is **≤$353.86**). Charging every one of the
  five allowed attempts at the cap gives the true failure-path ceilings:
  **239,442,560 tokens / $47.89 now; 8.8464×10⁹ tokens / $1,769.28 at 18k**.
  These are safety bounds, not estimates of typical content or a claim about
  whether a rejected request is billed. The protocol step re-verifies pricing
  for the ruled tier. The frozen build result records only chunks, embedded,
  reused, and top-k p95; the staging retry ledger retains only unfinished
  retry state. Provider billing is the charge authority. No durable token,
  elapsed, retry-total, or estimated-invoice schema is invented without a
  named consumer; the arithmetic above remains a reviewed safety bound.
- **Deterministic capacity failure is loud**: before allocating the exact
  index, the action requires `chunks < 100,000` and
  `chunks × dimension × 4 <= 1 GiB`. Crossing either limit, or corrupt
  metadata outside it, yields `capacity`; semantic search preserves lexical
  results and exit 3, while build emits its no-result exit-3 envelope. At
  3,072 dimensions 87,381 chunks pass and 87,382 fail; at 1,536 dimensions
  the 100,000-count gate comes first. This preflight prevents known oversized
  matrices and half-loaded indexes. It does **not** claim Go can recover an
  arbitrary runtime/OS out-of-memory termination.
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
    6,496-chunk corpus with 13 chunks/note at both 1,536 and 3,072 dimensions,
    under the standing benchstat discipline;
    (b) the CLI builder/reconciler runs the same measurement over the current
    generation immediately before activation, stores integer
    `top_k_p95_us`, and exposes it in the `search-index build --json` success
    body. Cadence: once per activated production generation; `serve` never runs
    it. Production builder construction refuses an absent or incompatible
    40-vector workload. A measured value is clamped to at least 1 μs, so zero
    is reserved for an unexported test/bootstrap state and is not dispatchable.
    If either deterministic capacity bound is already crossed, the builder
    returns `capacity` before activation and does **not** pretend it recorded a
    generation or run the exact-scan observer.
  - *Trigger*: rung 2 opens at `chunks >= 100,000`, raw vector payload over
    1 GiB, or measured p95 above ~100 ms. Crossing a trigger starts the
    comparison; it does not preselect PostgreSQL.

## H14. The §5a obligations, dispositioned

| Obligation (roadmap §5a, B) | Status |
|---|---|
| Chunking rules, chunks-per-note assumption stated | MET (H3: exact formulas, named Unicode boundaries, local pre-submit cap plus loud provider rejection, three dated measurements including current prefix cost) |
| Durable generation-store contract and format-selection gate | MET (H4/H11 and `semantic-storage-bakeoff.md`: normalized sqlc schema; active/previous/staging, resume, flip, rollback, GC, 256-row hydration, and WAL/file high-water measured on the implemented store; modernc/mattn driver trade recorded without claiming Badger was benchmarked) |
| RRF specifics (k, depths, aggregation) | MET (H6; shipped lexical ordering preserved) |
| §4a degraded matrix as acceptance cases | MET at plan level (H7: semantic-request fork; R1–R9; rows 0–24 across platform/active/work/config/writer/document/query axes; explicit-build acceptance set; three search envelopes and a frozen build wire). Ordinary UI/serve are excluded structurally. |
| The eval set | MET (H9; synthetic-in-repo per D50.8) |
| Egress guard test (private paths never reach the embedder) | MET at plan level, widened (H5/H10: one provider-transport owner, enumerated direct-constructor backstop, direct proxy-free transport, five runtime flows, no-influence, final-send and corpus-wide activation race locks; the log flow already shipped) |
| Provider account and distribution boundary | MET (H12.5/D50.11: optional BYOK, Koopa's paid live deployment, and no bundled/shared credential or proxy; it is not a matrix cell or an offline build/test prerequisite) |

`MET` here means the **plan obligation is specified**, not that H10's tests or
any Part II implementation already exist. This table is a claim, not a
certificate: it is MET only if the scoped
review agrees per row. Its own history is the warning — the first
revision marked the matrix MET while it still carried unclosed cells.

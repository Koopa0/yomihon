# Agent interface

yomihon's machine surface is five subcommands of the one binary. Their output
formats, JSON field names, and exit codes are a frozen contract: pipelines
depend on the exact bytes, and the repository's golden files and tests pin
them. This page documents that contract as it is.

There is no separate protocol version. The vault contract file declares
`schema_version = "1"` for its own format; the wire described here changes
only as a deliberate breaking change to the pinned goldens.

## Commands

| Command | Purpose |
|---|---|
| `yomihon check [--root <vault>] [path...]` | judge a vault and report findings |
| `yomihon coverage [--root <dir>]` | report concept coverage; never gates |
| `yomihon exists [--root <dir>] <name>` | answer whether a note for a name exists |
| `yomihon search [--semantic] [--] <query...>` | privacy-gated lexical (and optionally semantic) search |
| `yomihon search-index build` | build or resume the local semantic generation store |

Every command takes `--root <dir>` to name the vault; without it, the folder
the command runs in is the vault.

## The vault contract is required

All five commands refuse a folder that has no vault contract at
`System/schemas/vault-schema.toml`:

- `check`, `coverage`, and `exists` print a tool error with guidance on
  stderr and exit 2 — a folder that declares nothing gives them no vocabulary
  to answer in.
- `search` and `search-index build` write the closed-capability envelope
  `{"error":{"reason":"privacy-capability-unavailable"}}` (with `--json`),
  print a human notice on stderr, and exit 3. The same reason token covers a
  contract whose privacy policy exists but cannot be honoured: for egress, an
  undeclared policy and a rejected one are the same refusal.

Reading and browser search are not gated; only this machine surface is.

## Output format selection

Two conventions coexist, one per command family; each is deliberate and
test-pinned, and they do not match. Pass `--format json` or `--json`
explicitly in a pipeline rather than relying on pipe detection.

- `check`, `coverage`, and `exists` adapt to the terminal: with no `--format`
  flag they write the machine format when stdout is not a terminal and the
  human view when it is. `--format json|human|md` decides instead of the
  terminal. `md` is a check-only format; `coverage` and `exists` fall back to
  the human view for it.
- `search` and `search-index build` never inspect the terminal: they write
  human-readable text by default — including an empty answer for zero search
  results, which still exits 0 — and write the machine envelope only with the
  explicit `--json` flag.

## Machine formats

### `check --format json` — JSONL

One JSON object per finding, one finding per line, in a stable order. Fields:
`rule_id`, `severity`, `path`, `line` (omitted when the finding has no line),
`field` (omitted when not field-bound), `message`, `evidence`,
`suggested_action`, `source_rule`, `target` / `resolved_to` /
`collision_members` (link and collision findings only, omitted otherwise),
and `fingerprint` — a stable identity for one finding, which `--baseline
<file>` uses to subtract a prior run's findings so only new ones are reported
and gated.

### `coverage --format json` — one JSON object

`total_concepts`, `domains` (each with `domain`, `concepts`, `mounted`,
`pending_mount`, `orphan`), `pending_mount`, `orphans`, and `unrouted` (each
with `path`, `note_type`, `expected_route`).

### `exists --format json` — one JSON object

`query` and `matches`, each match carrying `path`, `field`, and `value`.

### `search --json` — one JSON object

An answer:

```json
{"mode":"lexical","semantic":"off","results":[{"rank":1,"rel_path":"Notes/a.md","title":"A","status":"ready","snippet":"…","heading":"…","channels":["lexical"],"channel_ranks":{"lexical":1}}]}
```

- `mode` is `lexical` or `hybrid`; `semantic` is `off`, `ok`,
  `not-applicable`, or `unavailable`.
- `coverage` (`{"reason":"<token>"}`) appears only when part of the request
  could not be served and names why.
- Per result, `status`, `snippet`, `heading`, and each `channel_ranks` member
  are omitted when absent; `channels` lists the retrieval channels that
  produced the result.

A refusal is `{"error":{"reason":"<token>"}}`. An internal failure is
`{"internal_error":{"detail":"…"}}`.

### `search-index build --json` — one JSON object

Success: `{"status":"current|built","chunks":N,"embedded":N,"reused":N,"top_k_p95_us":N}`.

A refusal:
`{"error":{"reason":"<token>","active_generation":"…","staging_generation":"…","retry_safe":true,"next_action":"…"}}`
where `active_generation` is one of `not-inspected`, `absent`,
`preserved-usable`, `preserved-unusable`; `staging_generation` is one of
`not-inspected`, `absent`, `incompatible`, `resumable`,
`requires-authorization`; and `next_action` is one of `retry-build`,
`wait-and-retry`, `renew-attempt-budget`, `repair-configuration`,
`repair-vault-contract`, `repair-input`, `use-supported-platform`,
`review-capacity`, `repair-yomihon`. An internal failure carries the same
recovery fields plus `detail` under `internal_error`.

### Search-family reason tokens

`not-applicable`, `privacy-capability-unavailable`,
`metadata-filters-unavailable`, `artifact-policy-unavailable`, `cache-cold`,
`cache-corrupt`, `cache-mismatch`, `embedder-retired`, `capacity`,
`embedder-unconfigured`, `rebuild-required`, `index-refreshing`,
`index-incomplete`, `vault-changed`, `embedder-unreachable`,
`embedder-rejected`, `embedder-failed`, `rate-limited`,
`attempt-budget-exhausted`, `attempt-budget-not-renewable`,
`unsupported-platform`.

## Exit codes

| Command | 0 | 1 | 2 | 3 |
|---|---|---|---|---|
| `check` | nothing named by `--deny` found | a `--deny` gate hit | could not run | — |
| `coverage` | always: it reports, never gates | — | could not run | — |
| `exists` | a match exists, describable or withheld | no match exists | could not run | — |
| `search` | answer written | internal error | could not run | refused, or semantic retrieval degraded (a lexical answer may still be written) |
| `search-index build` | index current or built | internal error | could not run | refused or could not complete |

`exists` is designed so a caller can gate a write-if-absent on the exit code
alone. On exit 3 the search family writes the reason's human line to stderr;
with `--json` the machine envelope on stdout carries the same reason token.

## What a withheld scope answers

The contract's `[privacy]` never-egress directories are scanned but never
reported from, so every agent-facing command has ground it cannot speak about.
Two commands say so and one cannot:

- `check` refuses a `[path...]` that lies inside such a directory, exit 2, and
  names the reason on stderr. It does not answer 0 findings, because an empty
  result for a scope the caller named would certify ground the command withheld
  — and a `--deny` gate would then issue a PASS over a genuine error. Every path
  under a withheld directory gets that same refusal whether or not a note is
  there, so the pair of refusals is not an existence oracle over it.
- A whole-vault run makes no claim about any one directory, so it reports what
  it may and stays exit 0.
- `exists` takes a name rather than a path, so it has no scope to refuse. A
  withheld note describes nothing about itself — it contributes no entry to
  `matches`, so no path, field, or value leaves — but it does answer: the
  report carries `"withheld": true` and the exit stays 0. The exit code is the
  documented write-if-absent gate, and the caller supplied the name, so telling
  them the name is free is the one answer that causes harm: a second,
  describable note created under a private note's own name. The field is absent
  from an ordinary answer, whose bytes are unchanged. It does appear beside
  describable matches when a withheld note shares the queried name: the reading
  page resolves names with no privacy filter, so the withheld twin renders
  every link to that name ambiguous, and without the flag a caller would write
  links believing the name resolves cleanly. The flag still names nothing.
- A file the scan cannot read stops every command at exit 2 with the file's
  vault-relative path and the operating system's reason — unless the file lies
  under a withheld directory, in which case the refusal is one fixed sentence
  carrying neither, because naming the file or its failure would describe
  ground the contract closed.

## What each count counts

Several surfaces show a number over "the notes", and they are not the same
notes. Nothing here is a defect; the sets differ because the rules that build
them differ, and a reader comparing two of them is owed the reason rather than
left to infer one.

| Surface | Its set | What it leaves out |
|---|---|---|
| The browser's status distribution and its per-status rows | Every indexed governed instance that carries a status | Files that are not notes; anything the contract's `[artifacts]` calls a non-instance; a note whose status is absent or not a single value |
| The whole-folder page's out-of-enum list | The same set, narrowed to statuses the contract does not declare for that note's type | Everything the row above leaves out, so a note missing a status entirely is in neither |
| `check` findings | Every scanned note, minus findings withheld for egress, minus `System/` unless `--all` | Findings under `[privacy].never_egress_dirs`, unconditionally; a targeted scope inside one is refused rather than counted |
| `coverage` | The contract's coverage routes over scanned notes | The same never-egress paths |
| `exists` matches | Every note exposing the name on its filename — with or without the `.md` — or on its title, any alias, or its English title. A note exposing it on two of those yields two matches | The path, field, and value of a withheld note, which is reported only as `withheld` |

The one rule they share: a never-egress directory is scanned so links resolve
consistently, and never described. A count is therefore a count of what may be
described, not of what is on disk.

## Corpus: this CLI versus browser search

The two search faces deliberately differ; results are not interchangeable.

| | `yomihon search` (this CLI) | browser search (`yomihon serve`) |
|---|---|---|
| File kinds | Markdown notes (`.md`) only | notes, plus any text-presentable file that is not a picture or a PDF |
| Size bound | none | files over 1 MiB stay readable but are not indexed |
| Privacy | vault contract required; the contract's `[privacy]` never-egress directories are excluded | no contract needed and nothing filtered, because nothing leaves the machine |
| A non-UTF-8 note | the whole command refuses (exit 2) and names the file | the server keeps serving; the affected file degrades |

An agent can therefore see zero results for content the browser finds
(non-Markdown text files), and can find notes above 1 MiB that the browser
index omits.

## Network behavior of `--semantic`

Semantic actions need the credential in `YOMIHON_EMBED_KEY`. The query text
of an explicitly requested semantic search is sent to the embedding provider
at most once per explicit action and never enters logs, caches, error
output, metrics, or traces. Note content becomes local vectors only as the
vault contract's privacy policy allows, and the vectors stay on this
machine. Everything else — reading, lexical search, diagnostics — is local.

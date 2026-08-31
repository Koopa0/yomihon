# Agent interface

yomihon's machine surface is three subcommands of the one binary. Their output
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

Every command takes `--root <dir>` to name the vault; without it, the folder
the command runs in is the vault.

## The vault contract is required

All five commands refuse a folder that has no vault contract at
`System/schemas/vault-schema.toml`:

- `check`, `coverage`, and `exists` print a tool error with guidance on
  stderr and exit 2 — a folder that declares nothing gives them no vocabulary
  to answer in.

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

## Exit codes

| Command | 0 | 1 | 2 | 3 |
|---|---|---|---|---|
| `check` | nothing named by `--deny` found | a `--deny` gate hit | could not run | — |
| `coverage` | always: it reports, never gates | — | could not run | — |
| `exists` | a match exists, describable or withheld | no match exists | could not run | — |

`exists` is designed so a caller can gate a write-if-absent on the exit code
alone.

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

# Agent interface

yomihon's machine surface is three subcommands of the one binary. Their output
formats, JSON field names, and exit codes are a frozen contract: pipelines
depend on the exact bytes, and the repository's golden files and tests pin
them. This page documents that contract as it is.

## Commands

| Command | Purpose |
|---|---|
| `yomihon check [--root <vault>] [path...]` | judge a vault and report findings |
| `yomihon coverage [--root <dir>]` | report concept coverage; never gates |
| `yomihon exists [--root <dir>] <name>` | answer whether a note for a name exists |

Every command takes `--root <dir>` to name the vault; without it, the folder
the command runs in is the vault. All three refuse a folder with no vault
contract at `System/schemas/vault-schema.toml`, printing a tool error on
stderr and exiting 2. Reading and browser search are not gated.

## Output format selection

With no flag, all three write the machine format when stdout is not a terminal
and the human view when it is. `--format json|human|md` decides instead of the
terminal; `md` is check-only, and `coverage` and `exists` fall back to the
human view for it. Pass `--format json` explicitly in a pipeline rather than
relying on pipe detection.

## Machine formats

### `check --format json` — JSONL

One JSON object per finding, one finding per line, in a stable order. Fields:
`rule_id`, `severity`, `path`, `line` (omitted when the finding has no line),
`field` (omitted when not field-bound), `message`, `evidence`,
`suggested_action`, `source_rule`, `target` / `resolved_to` /
`collision_members` (link and collision findings only, omitted otherwise), and
`fingerprint` — a stable identity for one finding, which `--baseline <file>`
subtracts a prior run's findings by, so only new ones are reported and gated.

### `coverage --format json` — one JSON object

`total_concepts`, `domains` (each with `domain`, `concepts`, `mounted`,
`pending_mount`, `orphan`), `pending_mount`, `orphans`, and `unrouted` (each
with `path`, `note_type`, `expected_route`).

### `exists --format json` — one JSON object

`query` and `matches`, each match carrying `path`, `field`, and `value`. A
match is a note exposing the queried name on its filename — with or without
the `.md` — or on its title, any alias, or its English title; a note exposing
it on two of those yields two matches.

## Exit codes

| Command | 0 | 1 | 2 |
|---|---|---|---|
| `check` | nothing named by `--deny` found | a `--deny` gate hit | could not run |
| `coverage` | always: it reports, never gates | — | could not run |
| `exists` | a match exists, describable or withheld | no match exists | could not run |

`exists` is designed so a caller can gate a write-if-absent on the exit code
alone.

## What a withheld scope answers

A never-egress directory declared under the contract's `[privacy]` is scanned
so links resolve consistently, and never described, so every count here counts
what may be described rather than what is on disk. `check` additionally drops
findings under `System/` unless `--all` is passed.

- `check` refuses a `[path...]` inside a withheld directory, exit 2, naming the
  reason on stderr. Every path under one gets that same refusal whether or not
  a note is there, so the pair of refusals is not an existence oracle. A
  whole-vault run claims nothing about any one directory: it reports what it
  may and stays exit 0.
- `exists` takes a name rather than a path, so it has no scope to refuse. A
  withheld note contributes no entry to `matches` — no path, field, or value
  leaves — but the report carries `"withheld": true` and the exit stays 0. The
  field is absent from an ordinary answer, whose bytes are unchanged, and
  appears beside describable matches when a withheld note shares the name.
- A file the scan cannot read stops every command at exit 2 with the file's
  vault-relative path and the operating system's reason — unless it lies under
  a withheld directory, where the refusal is one fixed sentence carrying
  neither.

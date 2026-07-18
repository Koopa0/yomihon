# yomihon

[![CI](https://github.com/koopa0/yomihon/actions/workflows/ci.yml/badge.svg)](https://github.com/koopa0/yomihon/actions/workflows/ci.yml)
[![Go 1.26.5+](https://img.shields.io/badge/Go-1.26.5%2B-00ADD8?style=flat&logo=go&logoColor=white)](go.mod)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue?style=flat)](LICENSE)

yomihon is a local, single-user web interface for reading and curating a
personal Markdown knowledge vault. It renders every file in the vault as a
readable, navigable page, and lets the vault's owner advance a note's
lifecycle `status` right where they finished reading — one field, one
validated transition, one git commit. Everything else in the vault is
read-only to it.

> [!WARNING]
> yomihon is under active development. Expect significant feature and
> interface changes before the first stable release.

The first public release will be a source-only `v0.x` release after the
product, privacy, cross-platform, identity, and independent-use gates are
complete. The repository is not yet declaring `v0.1.0` ready. See the
[release policy](docs/release.md) for the compatibility boundary and the
evidence required before publication.

It binds to `127.0.0.1`, has no authentication, and is built for exactly one
person on one machine. The server's derived state — the link graph, navigation
model, and lexical search index — lives in memory and is rebuilt from the
files, so the source of truth is always the vault plus its git history. The
repository contains an optional, disposable local SQLite generation store for
agent-facing semantic search. The HTTP server and ordinary search UI never open
that store or contact the embedding provider.

## Features

### Reading

Markdown renders through [goldmark](https://github.com/yuin/goldmark) with
GFM (tables, task lists, strikethrough) plus the vault dialect:

- **Wikilinks** — `[[target]]` and `[[target|display]]`, resolved against the
  whole vault; broken or ambiguous links render as flagged spans, never as
  silent guesses.
- **Transclusion** — `![[note]]` embeds a note's body one level deep.
- **Callouts** — `> [!note]`, `> [!warning]`, and friends, with `+`/`-`
  foldability rendered as native `<details>`.
- **Highlights** — `==text==` becomes `<mark>`.
- **Ruby** — hand-written `<ruby>` furigana passes through untouched, with a
  global show/hide toggle.
- **Mermaid** — fenced ` ```mermaid ` blocks render client-side, with the
  escaped source as the no-JS fallback.
- **Syntax highlighting** — server-side via
  [chroma](https://github.com/alecthomas/chroma); no highlighting JavaScript
  is shipped.
- **Headings** — CJK-safe anchor slugs and a generated table of contents.
- `%%comments%%` are stripped before rendering.

Rendering is fault-tolerant by contract: bad YAML, broken links, and unknown
callouts surface as inline diagnostics rather than being silently repaired or
invalidating the rest of the vault snapshot.

### File viewer

Every file in the vault is viewable, not just notes:

| Format | Treatment |
|---|---|
| Markdown (`.md`) | Full reading page with TOC and diagnostics |
| Images (`.png` `.jpg` `.jpeg` `.gif` `.webp` `.svg`) | Inline image page |
| PDF (`.pdf`) | Embedded PDF view |
| Text and source files (≤ 1 MiB) | Syntax-highlighted source page |
| Everything else | Info page with a raw download link |

Raw file responses are served with explicit content types, `nosniff`, a CSP
sandbox, and HTTP range support.

### Search

An in-memory lexical index over titles and bodies, opened from anywhere with
`⌘K`. The `/search` page and command palette update server-rendered results
after a short input debounce; their ordinary GET forms remain the Enter,
button, and no-JavaScript path. Home intentionally provides only that plain GET
form. Bare words AND-match as substrings; structured filters narrow by
frontmatter:

```
kanji type:lesson status:ready folder:Sources
```

Supported filter keys: `type:` `status:` `domain:` `slug:` `topic:` `folder:`.

### Status — the single write

The one thing yomihon writes is the frontmatter `status` field. A flip is
validated against the vault's own state machine (by prior state and owner),
applied as a surgical single-line rewrite — every other byte of the file is
left untouched — and recorded as one git commit in the vault under the
owner's git identity. Stale reads, concurrent writes, and a dirty work tree
are all rejected.

The state machine ships with the vault, not the binary: yomihon loads it from
`System/schemas/vault-schema.toml` inside the vault root. Without that
contract, the write face stays closed and yomihon is a pure reader.

### Vault diagnostics

A diagnostics engine (the judge) reports on vault health — broken and
ambiguous links, alias collisions, files missing from maps, frontmatter that
violates the vault schema — and classifies concept coverage. It only ever
reports; fixing a file is a human's job.

### Study paths, lessons, and reports

- **Study paths** — curriculum notes render as part → module → lesson trees
  with per-lesson resolution state and a switcher across paths.
- **Lesson pages** add text-to-speech with furigana-aware reading,
  sentence-pattern practice slots loaded from vault sidecars, and a concept
  drawer that previews linked concept notes in place.
- **Reports** — HTML briefings stored in the vault render verbatim inside
  sandboxed iframes.

## Getting started

Requires Go 1.26.5 or newer. Building CSS additionally requires the
[Tailwind CSS standalone CLI](https://tailwindcss.com/blog/standalone-cli)
(no Node); generated templates and built CSS are committed, so the product
binary builds without either.

```sh
go build -o bin/yomihon ./cmd/yomihon
bin/yomihon serve   # http://127.0.0.1:9610
```

Use `make build` after installing the generation tools described in
[CONTRIBUTING.md](CONTRIBUTING.md).

Configuration is environment-only:

| Variable | Purpose | Default |
|---|---|---|
| `YOMIHON_ROOT` | Vault path | `~/obsidian` |
| `YOMIHON_PORT` | Listen port | `9610` |
| `YOMIHON_EMBED_KEY` | User-owned Gemini credential for an explicit semantic CLI action | unset |

The listener is always `127.0.0.1`; only the port is configurable. Reading,
navigation, the judge, and lexical search support macOS, Linux, and Windows.
Status writes and the current semantic-generation store support macOS and
Linux; Windows keeps the reader open but fails those two write-backed features
before filesystem access.

This repository is Koopa's personal application, not a turnkey multi-user
service. Reading is driven by the vault contract, but the current
status-transition owner is intentionally fixed to `koopa`;
other users should treat the write face as project-specific until an explicit
local-actor contract is designed.

## Command line

```
yomihon <serve|search|search-index|check|coverage|exists> [options]
```

| Command | Purpose |
|---|---|
| `serve` | Start the reading interface |
| `search [--semantic] <query...>` | Search the vault for an agent; lexical unless semantic retrieval is explicitly requested |
| `search-index build` | Build or refresh the local semantic generation |
| `check` | Scan the vault and report diagnostics |
| `coverage` | Report how concepts are mounted into the vault's maps |
| `exists <name>` | Test whether a note exists by filename, title, or alias |

`yomihon help [command]`, top-level `-h`/`--help`, and recognized command
`-h`/`--help` forms print usage before configuration, vault scanning, key
access, or semantic-store access.

The three scan commands share `--root <dir>` (default: current directory) and
`--format json|human|md`. Markdown is a distinct report format for `check`;
for `coverage` and `exists`, `md` intentionally uses the human-readable view.
When the format flag is absent, output going to a pipe is JSON and output going
to a terminal is human-readable, so the same invocation works interactively
and in scripts.

The reading server and its search UI are always lexical-only. Semantic search
is an explicit CLI action: first run `search-index build`, then use
`search --semantic`. The generation stays in the user's cache directory,
outside the vault; a source-build user supplies their own embedding key.
Building it sends only contract-eligible instance-note chunks to the configured
Gemini API, and each semantic query sends its bare query text once. Paths
excluded by the vault privacy contract never enter either flow. Yomihon
operates no shared credential or proxy; users should review the provider's
current terms and pricing for their own account.

`check` also takes:

- `--all` — include `System/` findings, excluded by default (`Diary/` is
  always excluded)
- `--deny <severity|rule-id>` — turn matching findings into a failing exit,
  for use as a CI or pre-commit gate (repeatable)
- `--baseline <file>` — subtract a previous JSON run, reporting only new
  findings

For semantic commands, `0` means a complete answer or build, `3` means the
requested semantic result is unavailable or the request cannot be answered,
`2` means invalid command use, and `1` is reserved for a confirmed yomihon
fault. Agent JSON uses frozen discriminated envelopes so exit `3` can honestly
distinguish a usable lexical answer from no answer.

For `check`, `coverage`, and `exists`, exit codes are a separate frozen
contract: `0` clean, `1` gate hit (or, for `exists`, no match), `2` tool error.
`coverage` always exits `0`. Their JSON output is line-delimited with a stable
field order and per-finding fingerprints, so downstream tooling can diff runs
byte-for-byte.

## Design guarantees

1. **One write.** The only mutation yomihon ever performs is the `status`
   flip described above. Every other byte of the vault is read-only.
2. **Loopback server, explicit provider egress.** The HTTP server never listens
   beyond `127.0.0.1`. Only the separately authorized semantic CLI and fixed
   synthetic certification actions contact the embedding provider; ordinary
   reading and UI search remain local.
3. **One schema.** The vault's enums and state machine are read from
   `vault-schema.toml` in the vault itself; the binary carries no copy.
4. **Never "fix" a note.** Reading is fault-tolerant and problems become
   diagnostics; yomihon never edits a file to repair it.

## Project layout

Packages are organized by feature under `internal/`. `cmd/yomihon` owns command
dispatch and HTTP dependency hand-off; feature policy and production choices
remain with the package that owns the feature.

| Package | Responsibility |
|---|---|
| `vault` | Walks the vault; fault-tolerant frontmatter parsing |
| `schema` | Loads `vault-schema.toml` — the only reader of the contract |
| `render` | Vault-dialect projections: HTML plus shared searchable text and sections |
| `graph` | Wikilink resolution, the link index, link diagnostics |
| `note` | The general reading surface: Home, notes, raw bytes, and file fallbacks |
| `search` | Human lexical search; subpackages own agent CLI composition, fusion, semantic generations, and offline evaluation |
| `status` | The write face: validation, surgical rewrite, git commit |
| `judge` | The diagnostics engine behind `check` / `coverage` / `exists` |
| `syllabus` | Study-path pages |
| `lesson` | Text-to-speech, practice slots, the concept drawer |
| `nav` | The sidebar tree, siblings, and placements |
| `report` | Sandboxed briefing pages |
| `snapshot` | Atomic in-memory rebuild of all derived state |
| `ui` | templ layouts, pages, and blocks |
| `asset` | Static assets |

## Development

```sh
make tools    # install the pinned Go analysis and generation tools
make verify   # tidy + fmt + generated SQL + vet + lint + security + race + build
make test     # race-enabled, shuffled test run
make provider-live   # explicit paid synthetic Gemini protocol certification
make bench-baseline  # record ten local benchmark samples before a change
make bench-compare   # collect after samples and compare with benchstat
make gen      # regenerate templ output
make css      # rebuild Tailwind output
```

CSS, JavaScript, and browser-probe changes also run `make frontend-check`.
Its locked Node.js packages are development-only; the binary and product build
have no Node.js dependency. See [CONTRIBUTING.md](CONTRIBUTING.md) for the full
local gate and generated-file rules.

Tests use the standard library plus
[go-cmp](https://github.com/google/go-cmp). Golden files under
`internal/judge/testdata/` pin the diagnostic wire format byte-for-byte —
external tooling parses it, so those bytes are load-bearing.

## License

[MIT](LICENSE). Redistributed fonts and client assets are documented in
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).

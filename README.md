# yomihon

yomihon is a local, single-user web interface for reading and curating a
personal Markdown knowledge vault. It renders every file in the vault as a
readable, navigable page, and lets the vault's owner advance a note's
lifecycle `status` right where they finished reading — one field, one
validated transition, one git commit. Everything else in the vault is
read-only to it.

> [!WARNING]
> yomihon is under active development. Expect significant feature and
> interface changes between releases.

It binds to `127.0.0.1`, has no authentication, and is built for exactly one
person on one machine. There is no database: all derived state — the link
graph, the navigation model, the search index — lives in memory and is
rebuilt from the files, so the source of truth is always the vault plus its
git history.

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

Rendering is fault-tolerant: a page never fails to load. Bad YAML, broken
links, and unknown callouts surface as inline diagnostics instead.

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
`⌘K` (a plain `/search` form is the no-JS fallback). Bare words AND-match as
substrings; structured filters narrow by frontmatter:

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

Requires Go 1.26. Building CSS additionally requires the
[Tailwind CSS standalone CLI](https://tailwindcss.com/blog/standalone-cli)
(no Node); generated templates and built CSS are committed, so
`go build ./...` works without either.

```sh
make build          # templ generate + tailwind + go build → bin/yomihon
bin/yomihon serve   # http://127.0.0.1:9610
```

Configuration is environment-only:

| Variable | Purpose | Default |
|---|---|---|
| `YOMIHON_ROOT` | Vault path | `~/obsidian` |
| `YOMIHON_PORT` | Listen port | `9610` |

The listener is always `127.0.0.1`; only the port is configurable.

## Command line

```
yomihon <serve|check|coverage|exists> [options]
```

| Command | Purpose |
|---|---|
| `serve` | Start the reading interface |
| `check` | Scan the vault and report diagnostics |
| `coverage` | Report how concepts are mounted into the vault's maps |
| `exists <name>` | Test whether a note exists by filename, title, or alias |

The three scan commands share `--root <dir>` (default: current directory) and
`--format json|human|md`. When the format flag is absent, output going to a
pipe is JSON and output going to a terminal is human-readable, so the same
invocation works interactively and in scripts.

`check` also takes:

- `--all` — include `System/` findings, excluded by default (`Diary/` is
  always excluded)
- `--deny <severity|rule-id>` — turn matching findings into a failing exit,
  for use as a CI or pre-commit gate (repeatable)
- `--baseline <file>` — subtract a previous JSON run, reporting only new
  findings

Exit codes are a contract: `0` clean, `1` gate hit (or, for `exists`, no
match), `2` tool error. `coverage` always exits `0`. The JSON output is
line-delimited with a stable field order and per-finding fingerprints, so
downstream tooling can diff runs byte-for-byte.

## Design guarantees

1. **One write.** The only mutation yomihon ever performs is the `status`
   flip described above. Every other byte of the vault is read-only.
2. **Loopback only.** The server never listens beyond `127.0.0.1` and never
   exposes the vault or any derived data off the machine.
3. **One schema.** The vault's enums and state machine are read from
   `vault-schema.toml` in the vault itself; the binary carries no copy.
4. **Never "fix" a note.** Reading is fault-tolerant and problems become
   diagnostics; yomihon never edits a file to repair it.

## Project layout

Package-by-feature under `internal/`; `cmd/yomihon` is wiring only.

| Package | Responsibility |
|---|---|
| `vault` | Walks the vault; fault-tolerant frontmatter parsing |
| `schema` | Loads `vault-schema.toml` — the only reader of the contract |
| `render` | Markdown → HTML: wikilinks, callouts, embeds, highlights, code |
| `graph` | Wikilink resolution, the link index, link diagnostics |
| `note` | The reading page and the file viewer |
| `search` | The lexical index and query grammar |
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
make verify   # fmt + vet + lint + test + build
make test     # race-enabled, shuffled test run
make gen      # regenerate templ output
make css      # rebuild Tailwind output
```

Tests use the standard library plus
[go-cmp](https://github.com/google/go-cmp). Golden files under
`internal/judge/testdata/` pin the diagnostic wire format byte-for-byte —
external tooling parses it, so those bytes are load-bearing.

## License

[MIT](LICENSE)

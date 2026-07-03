# kurodo — a local reading and adjudication interface for a personal Obsidian vault

kurodo is a local-only, single-user interface over one Obsidian vault (`~/obsidian`).
It renders the whole vault — lessons, concepts, reports, syllabus notes — as a
readable, navigable interface, and it lets the vault's owner adjudicate a note's
`status` in place, right where they finished reading. It never serves outside
`127.0.0.1`, and it writes exactly one frontmatter field (`status`); everything
else in the vault is read-only to it. The vault files plus their git history are
the source of truth — any derived data is disposable.

## Status

Pre-release. The design is finalized (`docs/spec.md`); the skeleton is up and
real lesson notes render end to end.

Working today:

- Reading and Obsidian-dialect rendering — callouts, wikilinks, embeds,
  highlights, task lists, and ruby passed through as-is.
- The status flip (an adjudication): the write face, guarded by the vault's
  state machine and recorded as a git commit.
- Server-side syntax highlighting (chroma).
- Client-side mermaid diagrams.

Not built yet:

- Full-text search (an in-memory index; no database).
- The judge CLI — `check` / `exists` / `coverage`, inherited from kura.
- Static export, inherited from yomihon.

## Etymology

> **kurodo (蔵人)** — in the Heian court, the *kurōdo* were the palace archivists
> who kept the sovereign's document store and relayed their rulings. The name
> encodes the design: the archivist reads and prepares the record, but only the
> sovereign — here, Koopa — presses `ready`.

## Design principles: the four walls

kurodo's behavior is fenced by four walls. Crossing one is a design decision,
not a patch — see `docs/design.md` and `docs/decisions.md`.

1. **Wall 1 — the write face is one field.** The only thing kurodo writes is the
   frontmatter `status` field. Every change is validated against the vault's
   state machine (by prior state and owner) and recorded as a single git commit
   under Koopa's own git identity. The rewrite is surgical: the `status` line is
   replaced; every other byte is left untouched.
2. **Wall 2 — loopback only.** The listener hardcodes `127.0.0.1`; only the port
   is configurable. The search index and every other piece of derived data stay
   on the machine.
3. **Wall 3 — one schema contract.** The vault's schema — its enums and state
   machine — lives only in `vault-schema.toml`. `internal/schema` is the only
   package that reads it; there is no second, hardcoded copy anywhere in the repo.
4. **Wall 4 — the renderer never fixes a note.** kurodo reads fault-tolerantly
   and surfaces diagnostics for bad YAML, broken links, and name collisions, but
   it never edits a file to "fix" them. The judge reports; a human edits.

Before touching the renderer, graph, or search, read `docs/vault-model.md`: the
vault's Obsidian dialect has a spec, and generic Obsidian knowledge gets it wrong.

## Build and run

```sh
make build         # templ generate, then build bin/kurodo
make run           # go run ./cmd/kurodo serve
bin/kurodo serve   # or run the built binary directly
```

Requires Go 1.26. Styles are built separately with `make css` (Tailwind v4
standalone CLI, no Node). Configuration is read from the environment:

| Variable | Purpose | Default |
|---|---|---|
| `KURODO_ROOT` | Vault path | `~/obsidian` |
| `KURODO_PORT` | Listen port on `127.0.0.1` | `9610` |

kurodo needs no database. All derived state — the graph, the navigation model,
and the search index — is in-memory, rebuilt from the vault; the truth is always
the vault files plus their git history.

## Layout

Package-by-feature under `internal/`; `cmd/kurodo/` is wiring only.

| Package | Responsibility |
|---|---|
| `vault` | Walks the vault; splits and fault-tolerantly parses frontmatter |
| `schema` | Loads `vault-schema.toml` — the only reader of the contract (wall 3) |
| `render` | Renders the Obsidian dialect to HTML with goldmark: callouts, wikilinks, embeds, highlights, headings, code blocks |
| `graph` | Resolves wikilinks, builds the link index, reports link diagnostics |
| `note` | The reading face: loads a note, renders it, builds the TOC and diagnostics panel |
| `status` | The write face: state-machine validation, surgical single-line rewrite, git commit (wall 1) |
| `ui` | templ layouts, pages, and blocks |
| `asset` | Serves static JS and CSS |

## Ecosystem

kurodo succeeds two tools, each frozen in service until its retirement gate is met:

- **yomihon** — the Japanese-lesson reader; kurodo inherits its dialect rendering
  and static export.
- **kura** — the corpus judge; kurodo inherits its `check` / `exists` / `coverage`
  CLI and the byte-compatible output four vault pipelines depend on.

**koopa0.dev** is the public-facing sibling. kurodo is the private, local-only
human terminal of the same ecosystem.

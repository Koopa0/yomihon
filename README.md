# yomihon — a local reading and adjudication interface for a personal Obsidian vault

yomihon is a local-only, single-user interface over one Obsidian vault (`~/obsidian`).
It renders the whole vault — lessons, concepts, reports, syllabus notes — as a
readable, navigable interface, and it lets the vault's owner adjudicate a note's
`status` in place, right where they finished reading. It never serves outside
`127.0.0.1`, and it writes exactly one frontmatter field (`status`); everything
else in the vault is read-only to it. The vault files plus their git history are
the source of truth — any derived data is disposable.

Working on this repository — human or agent — start at `docs/program.md`:
it carries the delivery program, the role split, and the map of which
document owns what. The quality bar is `docs/standards.md`.

## Status

Pre-release. The skeleton is up and real lesson notes render end to end.

Working today:

- Reading and Obsidian-dialect rendering — callouts, wikilinks, embeds,
  highlights, task lists, and ruby passed through as-is.
- The status flip (an adjudication): the write face, guarded by the vault's
  state machine and recorded as a git commit.
- Full-text lexical search (an in-memory index) with structured filters.
- Syllabus trees, lesson interactions (furigana, TTS, slots, concept drawer),
  and sandboxed report pages.
- The judge engine — the vault diagnostics behind `check` / `coverage` /
  `exists`, byte-compatible with the external pipelines' frozen JSONL format;
  the four cron consumers run on it.
- Server-side syntax highlighting (chroma); client-side mermaid diagrams.

Not built yet (the sequencing blueprint is `docs/roadmap.md`):

- Hybrid semantic search (Gemini embeddings fused with the lexical index).
- The agent toolbox: graph relation queries and whole-graph export.
- The adjudication cockpit (home), static export, and the dreaming inbox.

## Past name

> The project was originally named **kurodo (蔵人)** — in the Heian court, the
> *kurōdo* were the palace archivists who kept the sovereign's document store
> and relayed their rulings. The name encoded the design: the archivist reads
> and prepares the record, but only the sovereign — here, Koopa — presses
> `ready`. It was later renamed to yomihon.

## Design principles: the four walls

yomihon's behavior is fenced by four walls. Crossing one is a design decision,
not a patch — see `docs/design.md` and `docs/decisions.md`.

1. **Wall 1 — the write face is one field.** The only thing yomihon writes is the
   frontmatter `status` field. Every change is validated against the vault's
   state machine (by prior state and owner) and recorded as a single git commit
   under Koopa's own git identity. The rewrite is surgical: the `status` line is
   replaced; every other byte is left untouched.
2. **Wall 2 — loopback only.** The listener hardcodes `127.0.0.1`; only the port
   is configurable. yomihon never serves or exposes the vault or any derived data
   beyond the machine. The one authorized outbound exception: note content —
   never `Diary/` — sent to the embedding API to compute search vectors, which
   are stored locally (`docs/decisions.md` D32).
3. **Wall 3 — one schema contract.** The vault's schema — its enums and state
   machine — lives only in `vault-schema.toml`. `internal/schema` is the only
   package that reads it; there is no second, hardcoded copy anywhere in the repo.
4. **Wall 4 — the renderer never fixes a note.** yomihon reads fault-tolerantly
   and surfaces diagnostics for bad YAML, broken links, and name collisions, but
   it never edits a file to "fix" them. The judge reports; a human edits.

Before touching the renderer, graph, or search, read `docs/vault-model.md`: the
vault's Obsidian dialect has a spec, and generic Obsidian knowledge gets it wrong.

## Build and run

```sh
make build         # templ generate, then build bin/yomihon
make run           # go run ./cmd/yomihon serve
bin/yomihon serve  # or run the built binary directly
```

Requires Go 1.26. Styles are built separately with `make css` (Tailwind v4
standalone CLI, no Node). Configuration is read from the environment:

| Variable | Purpose | Default |
|---|---|---|
| `YOMIHON_ROOT` | Vault path | `~/obsidian` |
| `YOMIHON_PORT` | Listen port on `127.0.0.1` | `9610` |

All derived state — the graph, the navigation model, and the search index — is
in-memory, rebuilt from the vault; the truth is always the vault files plus
their git history. There is currently no database; adopting one is a
per-feature engineering call with recorded triggers (`docs/roadmap.md` §4).

## Layout

Package-by-feature under `internal/`; `cmd/yomihon/` is wiring only.

| Package | Responsibility |
|---|---|
| `vault` | Walks the vault; splits and fault-tolerantly parses frontmatter |
| `schema` | Loads `vault-schema.toml` — the only reader of the contract |
| `render` | Renders the Obsidian dialect to HTML with goldmark: callouts, wikilinks, embeds, highlights, headings, code blocks |
| `graph` | Resolves wikilinks, builds the link index, reports link diagnostics |
| `note` | The reading face: loads a note, renders it, builds the TOC and diagnostics panel |
| `status` | The write face: state-machine validation, surgical single-line rewrite, git commit |
| `ui` | templ layouts, pages, and blocks |
| `asset` | Serves static JS and CSS |

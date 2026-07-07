# kurodo Design

> One-line positioning: a local reading and adjudication interface for private knowledge, the shared successor to yomihon and kura. Local, single-user, never served outside.
> Boundary model: **a large yard with irreversible walls** — the feature space is wide open; walls stand only at irreversible boundaries (the four walls are in `CLAUDE.md`). The functional spec and acceptance criteria live in `spec.md`; this document covers architecture. Except for the walls and the retirement gates, everything else is a "proposal" — the building session has latitude to adjust.

## 1. System context

```
                          ┌─ judge ── kura (15 rules, retired 2026-07-07 — judge-plan §13, D43; kurodo owns the formats)
 ~/obsidian (truth) ──────┤
   ↑ reads whole vault,    └─ reader ─ yomihon (frozen until its retirement declaration — gate closed, D40)
   │ read-only
 kurodo ──write──▶ single status field + git commit (human is the terminal)
   │            └─ CLI toolbox: vault-side agents also act through kurodo (after judge parity)
   ├─ hermes: writes go through worktree / QA-Gate; no interface with kurodo
   └─ koopa0.dev: outward-facing execution and publishing; the two never intrude on each other
```

kurodo reads everything, writes one field, never edits notes, and never oversteps. Derived data (indexes, caches) is disposable and rebuilt from the vault; the source of truth is always the vault files plus git.

## 2. Tech stack (every dependency justifies itself)

| Dependency                 | Rationale                                                                        |
| -------------------------- | -------------------------------------------------------------------------------- |
| goldmark + frontmatter ext | The pipeline foundation yomihon validated against real lessons                   |
| a-h/templ                  | Server-rendered HTML; existing muscle memory (yomihon, goilerplate)              |
| Tailwind v4 standalone CLI | No Node dependency; the typography plugin handles prose                          |
| `os/exec` git              | The audit layer must share exactly the same semantics as hand-run git; no go-git |
| vanilla JS (single file)   | Inherits from yomihon: native elements first (details/dialog), no framework      |

The search index is **in-memory** (D24) — see §6. Databases and vector stores are per-feature engineering calls (D31): the in-process shape is the default at the current scale, and the escalation ladders with explicit triggers live in `roadmap.md` §4.

**Not introduced**: fsnotify (D21 — a ~2s mtime scan does incremental indexing and handles create/delete/rename uniformly, without kqueue's non-recursive watch and lost events), Alpine (yomihon M2 already removed it — precedent), HTMX (wait until a real partial-update need appears), go-git, any ORM, any JS framework.

## 3. Binary and command surface

A single binary `kurodo`; `cmd/kurodo` does wiring only (go-spec doctrine). It is at once **a human interface (serve) and an agent interface (check/exists/coverage)** — the Claude Code on the obsidian side is a direct consumer of the CLI surface, so the output formats must align with kura (JSONL / human / md):

| Command                      | Purpose (spec in spec.md)                                                                                       |
| ---------------------------- | --------------------------------------------------------------------------------------------------------------- |
| `kurodo serve`               | The workbench itself, `127.0.0.1:9610` (9610 is goroawase for ku-ro-do; only the port is configurable)          |
| `kurodo check`               | A Go rewrite of kura check, JSONL golden comparison (yomihon SPEC §13's check plan is retired; lint moves here) |
| `kurodo exists` / `coverage` | Absorbed in sync from kura's agent toolbox; further extensions enter the yard per real vault-side needs         |
| `kurodo export`              | Absorbs yomihon's SSG export mode                                                                               |

Configuration (the config struct follows go-spec config-management): `KURODO_ROOT` (vault path, default `~/obsidian`), `KURODO_PORT` (default 9610). There is no `KURODO_DB` (the index is in-memory, D24) and **no `KURODO_ADDR`** — hard-wiring loopback is what wall 2 looks like once it has grown into code.

## 4. Package layout (proposal, package-by-feature)

```
cmd/kurodo/         wiring only: config, deps, routes, graceful shutdown
internal/vault/     fs walk (NFC-normalize paths), frontmatter splitting and fault-tolerant parsing
internal/schema/    load vault-schema.toml (enum, status_group, lifecycle, slug pattern) — the only reader of the toml (wall 3)
internal/render/    Obsidian dialect → HTML: goldmark pipeline (following yomihon parser.go) plus the ==highlight== and ![[embed]] that yomihon lacks
internal/graph/     wikilink resolution (following kura graph.rs semantics), link index, diagnostics
internal/nav/       navigation model: lifecycle folder tree, syllabus trees (Maps parsing), reports
internal/search/    in-memory search: build the index from vault text, NFC-folded substring query + structured filters, handler
internal/snapshot/  the Snapshot{Graph, Nav, Search} + the ~2s rescanner behind an atomic.Pointer (D25); handlers read the current snapshot
internal/status/    the only write: state machine validation, surgical single-line rewrite, git commit (wall 1)
internal/note/      reading feature: load + render + TOC + diagnostics panel
internal/ui/        templ: layouts / pages / blocks (yomihon's three-layer convention)
```

How the walls grow into code: `internal/status` is the only package with file-write and git capability; `internal/schema` is the only package that touches the toml; `internal/render`'s diagnostic types are read-only; the listener hard-wires loopback.

## 5. Data flow

**Read**: full scan at startup → `vault` parsing → the graph, nav, and search indexes built in memory into one `Snapshot` → a ~2s mtime scan rebuilds and swaps them on any change (D21, D25). Rendering is per-request (millisecond-scale at the 419-file scale; add an HTML cache only if measurements actually show it is slow — convergent).

**Write (the one and only path)**: the formal algorithm, UI, error vocabulary, and acceptance all live in `spec.md` §4. The skeleton: read the file for its current state → state machine validation (from + owner, actor=koopa) → dirty check → surgical single-line rewrite → atomic write-back → git commit (author = Koopa's identity, `(via kurodo)`) → PRG redirect.

## 6. Derived state — in-memory (D24, D25)

All derived state is built in memory from the vault and
held in one snapshot behind an `atomic.Pointer`:

```go
type Snapshot struct {
    Graph  *graph.Index   // wikilink resolution
    Nav    *nav.Model     // folder tree, syllabus trees, reports
    Search *search.Index  // per-note fields + NFC-folded plain_text for substring search
}
// atomic.Pointer[Snapshot]; every handler reads the pointer once per request
```

A single scanner goroutine (D25) `stat`-walks the vault about every 2 seconds;
on any mtime or file-set change it rebuilds all three and swaps the pointer
once — never a torn state (a fresh graph against a stale nav). A full rebuild
over ~419 files is ~100 ms and happens only on change. This is also the
incremental mechanism (D21) — no fsnotify — and it closes the pre-existing gap
where an edited note stayed stale in the sidebar and wikilink resolution until
a restart.

The search index holds only what search reads (D23): per note, `rel_path`,
`title`, `note_type`, `domain`, `status`, `slug`, `topics`, and the NFC-folded
`plain_text`. No link structure (that serves the H-face backlinks, `roadmap.md`)
and no raw frontmatter — added when a real consumer arrives, at zero cost since
the whole thing rebuilds from the vault.

No status history: `git log` is the history (vault-model §3). The reading and
judge faces never depend on any persistent store (spec §0.1); search is as
available as reading, because the index _is_ memory sourced from the
always-present vault. Persistence, when a trigger fires, climbs the ladders in
`roadmap.md` §4 (D24's SQLite rung for lexical; D32's vector rungs).

## 7. Search

Deterministic substring over the in-memory index (§6) plus structured
filtering; spec and acceptance in `spec.md` §3. Match has one definer — a Go
`fold(s) = strings.ToLower(nfc(s))` applied to both the stored text and the
query, so "what counts as a match" is decided in exactly one place. A linear
scan over ~419 folded strings is microsecond-scale. The hybrid semantic
extension is committed scope (D32): Gemini embeddings fused with this lexical
index via RRF — it composes with the deterministic layer, never replaces it;
design and storage ladders in `roadmap.md`.

## 8. UI (styling references the Syntax template; the implementation is all server-rendered templ)

- **Shell**: sticky 56px header (search ⌘K, furigana toggle 振, theme toggle) + left sidebar + content column + right column, on a 3-column grid that degrades to a drawer (≤900) and a fixed seal bar (≤1280). The `@theme` tokens are landed (Claude design → `internal/ui/styles/*.css`, compiled by the Tailwind v4 standalone CLI into `assets/css/output.css`, served at `/static/app.css`); the visual identity is the 2026-07-03 Reading Page redesign. Fonts are self-hosted woff2 (`assets/fonts/`, zero external requests); theme + furigana persist in a cookie and are rendered server-side onto the root element (no FOUC).
- **Left sidebar** (status-first, D26): a **Lifecycle** list (the `note` group's ordered statuses, live snapshot counts, each linking to `/search?q=status:<name>`) + the syllabus tree (two study-path notes with different structures) + a Reports area (daily-briefing HTML) + a **collapsed** Folders tree (lifecycle folders, vault order, top level ≤9). All grouping/counts trace to the toml + snapshot, never hardcoded (wall 3).
- **Content column**: typography prose; the two-bucket callout coloring; ruby passes through as-is (toggled by `visibility`, zero reflow); mermaid rendered client-side; code highlighting server-side (chroma, no prism/JS).
- **Right column**: TOC (CJK-safe slug) + frontmatter/status panel (all legal transitions, `ready` the only primary — the koopa-only seal, D27: press-and-hold ceremony as progressive enhancement, a read-only `git log -1 --format=%h` provenance line) + diagnostics column (display only, never fixes).
- **Japanese-lesson interactions**: reproduce yomihon's already-validated mechanisms as-is — furigana uses `visibility`, not `display` (prevents reflow); TTS's `data-tts` strips `<rt>/<rp>` during build/render; slots consume `System/slots/*.yaml` sidecars; the concept drawer uses a native `<dialog>`.

## 9. Goals and retirement gates

The goals (the four end-state points) are in `spec.md` §0. **No milestone fences (D15)** — implementation order is free; the only ordering suggestion (not a fence): first wire up the single "finish reading → certify" keypress (D10, the v0 shipping gate), which attacks the system's real current bottleneck — Koopa's adjudication friction.

The two retirements are **evidence-based gates, not dates** (D11):

- **yomihon's retirement gate — closed (D40)**: the five interactions + fixtures are merged; the parity and two-week observation items are waived. Retirement is effective on Koopa's declaration; yomihon stays frozen (tag `v1.0.0`) until that word is said, and is then discarded outright.
- **kura's retirement gate — met; kura declared retired 2026-07-07 (D43)**: `spec.md` §5 (JSONL byte-compat + snapshots + scan boundary + four-pipeline switchover — all merged) plus the differential fuzz campaign (`judge-plan.md` §13), which ran to its completion bar with zero unexplained divergence. kura is discarded and kurodo owns the formats (D40).

## 10. Scheduled and open items

The remaining faces and their design live in `roadmap.md` (Koopa, 2026-07-05: every remaining face ships; ordering by dependency and leverage). Formerly-unscheduled items now have homes: backlinks panel and frontmatter query → H (agent toolbox + reading-page panel, D33); the diagnostics index page and reading progress → the adjudication cockpit (roadmap §3); graph view → after the cockpit; PDF export → G's yard; MCP server → declined with a recorded reversal condition (D34). The yard stays open for what is not listed; the walls still don't block.

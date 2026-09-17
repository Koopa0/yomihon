# Data inventory

What yomihon holds, derives, and emits. The product is a single-user local
process: it reads a vault, serves `127.0.0.1`, writes one frontmatter field,
and makes no network call of any kind. There is no telemetry, analytics,
metrics or trace exporter, crash reporter, account, or remote log sink, and no
application-level encryption at rest, backup, or secure erasure. Host disk
encryption, vault sync, and any repository remote are external, and must not be
inferred from this document.

| Data | Where it lives | How long | Who can reach it |
|---|---|---|---|
| Vault files — body, frontmatter, path, attachments, contract-private content | The vault, which stays the truth; yomihon never deletes one | Vault and sync policy own it | The same OS account, and any client that can open the loopback port |
| `System/schemas/vault-schema.toml`, the machine authority | The vault; `internal/schema` is its only reader | Vault policy | As above |
| Parsed notes, rendered HTML, diagnostics, graph, search index | Process memory, rebuilt from the vault | Until the snapshot is replaced or the process exits | The process and the request being served |
| HTTP method, path, query, headers, status form; CLI flags and arguments | Request and output buffers | The request or action | The local caller |
| A rewritten note and its adjacent `.yomihon-status-*.tmp` | The vault directory | Until the rename; a crash can strand the temporary file | The filesystem and the requester |
| `slog` records: address, vault root, contract state, scan and render failures, selected status paths | stderr | Not persisted by yomihon | Whatever captures the terminal |

- **The hover card sends a link's own fragment to the server.** A browser never puts a
  URL fragment in a request; the preview endpoint takes it as a `?section=` query so the
  card can cut at the section or block the link addressed. It stays in the same loopback
  request buffers as any other query and reaches no process yomihon starts.

About those rows:

- **Nothing is request-logged.** Note bodies and raw search queries never reach
  a log. A failed query records its byte count and filter keys; a failed status
  write records its path, from, to, and error.
- **A local reader may see any vault file, private paths included**, because
  local reading is the product. Agent output is filtered instead: the
  contract's `[privacy]` never-egress directories are scanned so links resolve,
  and never described. An `exists` report may carry a bare `withheld` flag,
  naming no path, field, or value.
- **`obsidian://open` links carry the note's absolute path inside the page.**
  Following one hands that URI to the local Obsidian application; no network
  request is made.
- **Downstream copies belong to the caller.** A pipeline that consumes stdout
  owns what it keeps, and local deletion cannot recall it. Real-vault
  evaluation artifacts and agent reports stay vault-sensitive whoever created
  them; only content-free aggregates may be committed or quoted.
- **Owner and incident path:** Koopa. Stop the affected flow, preserve minimal
  evidence privately, assess local and repository copies, and report through
  `.github/SECURITY.md`.

The withholding applies to the agent-facing commands only: the HTTP reading room serves the owner every note, including those under `never_egress_dirs` (2026-09-04, #149; details in the threat model).

The reader's browser holds the reading choices and where they had the left
rail, and nothing else. Six cookies carry the choices, and the server writes
every one of them — at `/preferences`, and at the language form. Four of the
six also have a control in the header that the page's own script answers
directly: the desk, the text size, the furigana and the shortcuts, written
under the same names and for the same year, so a choice made with script and
one made without expire together. The typeface and the language have no header
control, and nothing but the server writes them. Two `sessionStorage` keys
carry the rail.

| Kept in the browser | What it holds | How long |
|---|---|---|
| `yomihon_lang` | Which language the interface speaks, `zh-Hant` or `en` | A year from the last write |
| `yomihon_theme` | The desk, `light` or `dark`; no value leaves the system's own setting deciding | A year from the last write |
| `yomihon_textsize` | The text size, `m`, `l` or `xl` | A year from the last write |
| `yomihon_font` | The typeface, `serif`, `sans` or `kai`; no value is the serif face as well | A year from the last write |
| `yomihon_ruby` | Whether furigana show, `on` or `off` | A year from the last write |
| `yomihon_shortcuts` | Whether single-key shortcuts answer, `on` or `off` | A year from the last write |
| `yomihon.nav` | Which folders of the left rail the reader opened or closed, one true-or-false each | Until the tab closes |
| `yomihon.nav.filter` | The text the reader typed into the rail's filter box, kept as typed so a narrowing survives opening a note | Until the tab closes |

- **The cookies go to the loopback listener and nowhere else.** Each carries
  `Path=/` and `SameSite=Lax`, and is deliberately neither `Secure` nor
  `HttpOnly`: the server is loopback HTTP by design, and the script that
  re-syncs a page revived from the back/forward cache has to read the value to
  compare it against the document it revived. Any script the page already runs
  can therefore read them. A cookie is scoped by host rather than by port, so
  anything else served from `127.0.0.1` to the same browser can read them too —
  the same local surface the threat model already accepts.
- **The two session keys are sent nowhere at all.** `sessionStorage` is never
  attached to a request: the rail's state and the filter text stay in the tab
  holding them and go when it closes.
- **`yomihon.nav.filter` holds the reader's own words.** Every other entry in
  the table is a value yomihon offered and the reader picked; the filter box
  keeps what they typed, as they typed it, which is usually part of the name of
  a note or folder they were looking for. That is the only vault-shaped text in
  browser storage: yomihon itself writes no note path, title, or body there,
  and the browser's own controls clear all of it.

No hosted reader exists for a user's own vault; a vault stays on the machine
that reads it. One public instance exists over this repository's own sample
notes, reached through a proxy, and the threat model describes what it accepts.
A remote database, vector service, telemetry sink, or cloud backup is absent
and unauthorized. Adding one is a new decision needing an updated threat model
and inventory, not a configuration question.

# Data inventory

What yomihon holds, derives, and emits. The product is a single-user local
process: it reads a vault, serves `127.0.0.1`, writes one frontmatter field
into a note and two reader-state files of its own outside any vault, and makes
no network call of any kind. There is no telemetry, analytics, metrics or trace exporter,
crash reporter, account, or remote log sink, and no application-level
encryption at rest, backup, or secure erasure. Host disk encryption, vault
sync, and any repository remote are external, and must not be inferred from
this document.

| Data | Where it lives | How long | Who can reach it |
|---|---|---|---|
| Vault files — body, frontmatter, path, attachments, contract-private content | The vault, which stays the truth; yomihon never deletes one | Vault and sync policy own it | The same OS account, and any client that can open the loopback port |
| `System/schemas/vault-schema.toml`, the machine authority | The vault; `internal/schema` is its only reader | Vault policy | As above |
| Parsed notes, rendered HTML, diagnostics, graph, search index | Process memory, rebuilt from the vault | Until the snapshot is replaced or the process exits | The process and the request being served |
| HTTP method, path, query, headers, status and mark forms; CLI flags and arguments | Request and output buffers | The request or action | The local caller |
| A rewritten note and its adjacent `.yomihon-status-*.tmp` | The vault directory | Until the rename; a crash can strand the temporary file | The filesystem and the requester |
| `slog` records: address, vault root, contract state, scan and render failures, selected status paths; a marks file's own absolute path when reading or writing it fails, and the note path the caller submitted when a continuation mark is refused | stderr | Not persisted by yomihon | Whatever captures the terminal |
| The reader's continuation — `reader.json` and its adjacent `.reader-*.json`, under `os.UserConfigDir()`/`yomihon`/one directory per vault, holding that vault's absolute path and one continuation point: a note path, an anchor id, a distance below it, the content identity of the bytes that were on screen, and when it was set | The platform's configuration directory, never the vault | Until the reader keeps another place or deletes the file; yomihon never expires one. A crash between the temporary file being created and renamed into place can strand the temporary file, holding that same content, and nothing removes it | The same OS account |
| The reader's uncertainty marks — `uncertainty.json` and its adjacent `.uncertainty-*.json` in the same per-vault directory. Each array entry contains only `path`, `anchor`, and `time`; no body, chosen sentence, content identity, or source version is saved | The platform's configuration directory, never the vault | Until the reader toggles that location off or deletes the file; no expiry or automatic recovery. A crash can strand a temporary file containing the same location records | The same OS account; the local reading client receives them from `GET /uncertainties` |
| A proposed thought note's frontmatter and `based_on` link | The response's read-only text field; the OS clipboard when the reader copies it; the `obsidian://new` URI when the reader chooses that handoff | The page lifetime; clipboard and external-editor policies own their copies | The local browser, clipboard, and selected editor |

- **The hover card sends a link's own fragment to the server.** A browser never puts a
  URL fragment in a request; the preview endpoint takes it as a `?section=` query so the
  card can cut at the section or block the link addressed. It stays in the same loopback
  request buffers as any other query and reaches no process yomihon starts.

About those rows:

- **A read-only note is not replaced.** A status flip checks the captured
  file's owner-write bit after the existing note and transition validation.
  If that bit is clear, the note and its siblings stay unchanged: no status
  temporary file is prepared and no success receipt is minted, even when the
  directory is writable. Readable notes still show their current status.
  Change permissions outside yomihon and reload before choosing a transition.
  This honors the owner's intent; it is not an effective ACL/group-access
  check or protection against a malicious process running as the same user.

- **The two reader-state files stay outside the vault.** Their directory name
  is a digest of the resolved absolute vault root, so two vaults keep separate
  files and the directory name spells no path. `reader.json`, when present,
  also names the vault's absolute path; `uncertainty.json` contains only its
  three-field entries. Nothing syncs or expires these marks. `yomihon check`
  and every other command-line face never read either file. Deleting one file
  clears that kind of mark; deleting the per-vault directory clears both.
- **Uncertainty marks promise a location, not recovery.** A renamed source,
  changed heading, or new sentence from an interactive control may leave the
  saved location unavailable or pointing at different content. There is no
  source identity, relocation, sentence reconstruction, backup, or lossless
  recovery. Deletion or a machine failure can lose the marks. An unreadable or
  malformed `uncertainty.json` produces a visible failure and is not replaced
  by the next toggle. The independent file also survives older versions
  replacing `reader.json`.
- **Nothing is request-logged.** Note bodies and raw search queries never reach
  a log. A failed query records its byte count and filter keys; a failed status
  write records its path, from, to, and error.
- **A local reader may see any vault file, private paths included**, because
  local reading is the product. Agent output is filtered instead: the
  contract's `[privacy]` never-egress directories are scanned so links resolve,
  and never described. An `exists` report on a name, including a vault path or
  path suffix, may carry a bare `withheld` flag, naming no path, field, or
  value.
- **`obsidian://open` links carry the note's absolute path inside the page.**
  Following one hands that URI to the local Obsidian application; no network
  request is made.
- **The optional thought handoff belongs to the external editor.** Copying the
  proposed Markdown or choosing `obsidian://new` passes frontmatter and a source
  link, not a copy of the source body. The editor owns any note it creates;
  yomihon does not create or complete a vault note through this handoff.
- **Downstream copies belong to the caller.** A pipeline that consumes stdout
  owns what it keeps, and local deletion cannot recall it. Real-vault
  evaluation artifacts and agent reports stay vault-sensitive whoever created
  them; only content-free aggregates may be committed or quoted.
- **Owner and incident path:** Koopa. Stop the affected flow, preserve minimal
  evidence privately, assess local and repository copies, and report through
  `.github/SECURITY.md`.

The withholding applies to the agent-facing commands only: the HTTP reading room serves the owner every note, including those under `never_egress_dirs` (2026-09-04, #149; details in the threat model).

## Optional thought notes

The "Leave a thought" handoff requires an explicit role in the vault's own
`System/schemas/vault-schema.toml`. Add `answer_type` to its existing
`[navigation]` table, using a member already declared in `[enums].type`.
For a vault that already declares the type `note`, the added line is:

```toml
answer_type = "note"
```

This setting is optional and has no default. Missing or invalid authority
keeps the handoff unavailable; an ordinary document and a lesson use the same
mechanism. Installing yomihon does not modify the reader's private contract or
add a new note type. The vault owner makes this opt-in edit.

The proposed Markdown contains only fields whose values can be derived from
the existing contract and source, plus a `based_on: "[[source#section]]"` link
when opened for a section, or a source-only link for a whole document. If the
role has no initial lifecycle state or more than one, the status line is
omitted. Unknown required values are omitted too; the existing judge reports missing
fields after the reader saves the note. There are no prompt excerpts,
before/after templates, or required-field pickers. The reader's editor owns
the template, words, remaining fields, and save operation.

## Browser state

Cookies and session storage hold the reading choices and where the reader had
the left rail. Six cookies carry the choices, and the server writes
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
| `yomihon_rail` | Whether the left column is shown or folded to a strip on wide windows, `open` or `collapsed` | A year from the last write |
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
  browser storage: yomihon itself writes no note path, title, or body there.
  All of it clears — the settings page's return to defaults drops the six
  cookies, and the browser's own controls drop all eight.

No hosted reader exists for a user's own vault; a vault stays on the machine
that reads it. One public instance exists over this repository's own sample
notes, reached through a proxy, and the threat model describes what it accepts.
A remote database, vector service, telemetry sink, or cloud backup is absent
and unauthorized. Adding one is a new decision needing an updated threat model
and inventory, not a configuration question.

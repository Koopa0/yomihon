# Data inventory

What yomihon holds, derives, and emits. The product is a single-user local
process: it reads a vault and edits no note in it, writes into it only the
reader's own marks (one file for each computer, under `reader-notes/.yomihon/marks/`),
serves `127.0.0.1`, writes files of its own outside any vault — the reader's
bookmarks, a ledger of the marks and a record of approved figures — and
makes no network call of any kind. A figure the owner approved runs its scripts in the browser, where they
can reach the network; the threat model says through which channels. There is no telemetry, analytics, metrics or trace exporter,
crash reporter, account, or remote log sink, and no application-level
encryption at rest, backup, or secure erasure. Host disk encryption, vault
sync, and any repository remote are external, and must not be inferred from
this document.

| Data | Where it lives | How long | Who can reach it |
|---|---|---|---|
| Vault files — body, frontmatter, path, attachments, contract-private content | The vault, which stays the truth; yomihon never edits, renames or deletes one, and writes in the vault only the reader's own marks (next rows) | Vault and sync policy own it | The same OS account, and any client that can open the loopback port |
| `System/schemas/vault-schema.toml`, the machine authority | The vault; `internal/schema` is its only reader | Vault policy | As above |
| Parsed notes, rendered HTML, diagnostics, graph, search index | Process memory, rebuilt from the vault | Until the snapshot is replaced or the process exits | The process and the request being served |
| HTTP method, path, query, headers, mark forms; CLI flags and arguments | Request and output buffers | The request or action | The local caller |
| `slog` records: address, vault root, rules file state, scan and render failures; the marks directory's absolute path when reading or writing a file in it fails, and the note path the caller submitted when a bookmark is refused | stderr | Not persisted by yomihon | Whatever captures the terminal |
| The reader's highlights, thoughts and questions — `reader-notes/.yomihon/marks/<machine id>.jsonl` in the vault: one append-only file of JSON lines for each computer, which that computer alone writes and the others only read. A line holds the format number, the machine id and the code of the process that wrote it, a revision number, which revisions of other writers' lines the page had read, the time, a checksum and the mark: a random id, its kind, a note path, an anchor id, when it was made, a deleted flag, and, for a highlight, a question or a thought, the words the reader chose with a little of what stood on either side, the content identity of the note's bytes, the words the reader wrote, and when a question was resolved. A question marked on a whole section carries only the kind, id, path, anchor and time | The vault, in its own folder `reader-notes/`, which the scan leaves out; it is the one place yomihon writes in a vault. Git carries it to other computers when the reader commits and pushes | Until the reader deletes the mark, which stays in the file as a deleted line so that an older copy of the file does not bring it back; no expiry. Words that were committed stay in git history after they are deleted. Up to 5,000 marks that are not deleted, of every computer, a line up to 64 KiB, this computer's file up to 20,000 lines, and a file past 16 MiB is not read. A crash can strand a temporary file in that folder, which the next write removes | Whatever can read the vault: the same OS account, other computers that pull it, an agent or program given the vault. Another computer's lines are untrusted data and are drawn escaped. The local reading client receives marks, with the reader's words, over `GET /mark/{id}` and the questions' path, anchor and time from `GET /questions`; `yomihon marks` writes the questions' path, anchor and time outside withheld ground to its caller, and `yomihon marks --export` writes the marks themselves, the reader's words among them, to its caller (below) |
| The reader's bookmarks and finished notes — `progress.json` and its adjacent `.progress-*.json` in the same per-vault directory, holding that vault's absolute path and the bookmarks: for each, the course it belongs to (empty for reading outside any course), a note path, an anchor id, a distance below it, the content identity of the bytes that were on screen, and when it was set; and the notes the reader has finished, each a note path and when. The file has room for a bookmark for each course and for any number of finished notes up to its cap; this version writes the bookmark for reading outside any course and one for each course the reader sets one in, the notes the reader says are finished and, when the reader adopts the marks kept for an earlier path with `yomihon marks --adopt`, the bookmarks and finished notes found there | The platform's configuration directory, never the vault | Until the reader keeps another place or deletes the file; yomihon never expires one. A crash can strand a temporary file holding that same content, and nothing removes it | The same OS account |
| The files an earlier version wrote — `marks.json` and its adjacent `.marks-*.json` (the marks, as they were kept before they moved into the vault), `reader.json` (one bookmark) and `uncertainty.json` (questions: `path`, `anchor`, `time`), in the same per-vault directory | The platform's configuration directory, never the vault | `marks.json` is read once, and the first write of a computer that has no file of marks in the vault takes its marks in; `reader.json` and `uncertainty.json` are read when `progress.json` or `marks.json` is not there yet, so that what a reader kept is converted rather than lost. None is changed, renamed or removed by yomihon. A reader who deletes them has deleted them | The same OS account. `yomihon marks` reads `uncertainty.json` read-only when there is no `marks.json` |
| The figure approvals — `figures.json` and its adjacent `.figures-*.json` in the same per-vault directory. Each entry is a figure's vault path, the SHA-256 of the bytes approved, and when; the file also names the vault's absolute path. No figure content is copied | The platform's configuration directory, never the vault | Until `yomihon revoke-figure` removes an entry or the file is deleted; an approval stops applying, without being removed, the moment the figure's bytes change | The same OS account. The reading server reads it on each page and each frame request and never writes it; `approve-figure` and `revoke-figure` write it, owner-only, by sibling-file rename. `check`, `coverage` and `exists` never read it |
| The reader's list of vaults — `vaults.json` and its adjacent `.vaults-*.json`, directly in `os.UserConfigDir()`/`yomihon`, beside the per-vault directories. For each vault the reader has opened: its id (the name of that vault's directory), its absolute path, the name it was last called (the title of its README only when the rules file lets that file be said, and the folder's name otherwise), when a page of it was last loaded (written at most once a minute) and the loopback port it is served on, which it keeps. An entry that cannot be used (a port that is not one, a path that is not a folder's, an id that is another vault's) is left out and the rest are read, each on the port it had. When the list is next rewritten, and when it could not be read at all, what it was is kept as `vaults.damaged.json` instead of being deleted: byte for byte when it was read, and renamed whole when it was too long or not a regular file. A later one is kept as `vaults.damaged-2.json`, up to `vaults.damaged-8.json`, and never replaces the first; once all eight are taken the last is replaced. The store is `internal/mark/vaults` | The platform's configuration directory, never the vault | Until `yomihon vaults remove` takes an entry out or the file is deleted; yomihon never expires one; a vault removed can be brought back only by the process that removed it, from memory. Set-aside lists stay until they are deleted. A crash can strand a temporary file holding the same content | The same OS account. The running yomihon reads it when its size or modification time changes and writes it, owner-only, by sibling-file rename; `yomihon <dir>` and `yomihon vaults remove` write it, and `yomihon vaults` reads it |
| `vaults.lock`, in the same directory — an empty file that the operating system's lock is held on while the list is changed. It holds nothing and names no vault | The platform's configuration directory | Made by the first change and kept; the lock on it is let go when the change ends or the process holding it ends, a crash included | The same OS account. Made by whatever changes the list: the running yomihon, `yomihon <dir>` and `yomihon vaults remove` |
| `marks.lock`, `figures.lock`, `ledger.lock` and `machine-file.lock`, in each per-vault directory — empty files that the operating system's lock is held on while `progress.json`, `figures.json`, `ledger.jsonl`, or this machine's file of marks in the vault (with the ledger line recorded for it) are changed. Each holds nothing and names no vault | The platform's configuration directory | Made by the first change and kept; the lock on it is let go when the change ends or the process holding it ends, a crash included | The same OS account. Made by whatever changes those files: the running yomihon, `yomihon marks --adopt` (`ledger.lock` only when it carries a ledger), `yomihon approve-figure` and `yomihon revoke-figure`; `machine-file.lock` is taken by `internal/mark/machinemarks` when a route writes a mark |
| The ledger — `ledger.jsonl` and its adjacent `.ledger-*.jsonl`, in each per-vault directory: an append-only record of what this machine writes for that vault into its file of marks in the vault, one JSON line each. A line holds its sequence number, the time to the millisecond, its kind, the machine id and the process code that wrote it, on a mark's line the id of that mark, the bytes it keeps (a mark's whole line, the reader's words among them, the whole of this machine's file of marks as a write replaced it, or a line of this machine's own value that a change replaced while it was hidden beside another computer's), as text or, when they are not UTF-8 or base64 is shorter, as base64, and a checksum. `ledger.damaged.jsonl` beside it holds, byte for byte, the lines a compaction found changed after they were written. It is what this computer's marks are put back from, without the reader being asked and without a word to the reader; the routes that write a mark write it, and `yomihon marks --adopt` copies the earlier path's | The platform's configuration directory, never the vault | Until the file is deleted; no expiry and no cap. Once it is past 8 MiB and twice what a compaction would leave of it, the next line compacts it: every line is kept but a mark's line that a newer line of the same mark by the same machine supersedes, and a line cut short by a crash is kept as it is. The copies of this machine's file of marks are kept whole for ever; past 1 GiB the ledger is not read and nothing more is recorded. A compaction killed part way can strand a `.ledger-*.jsonl` holding the same records, which the next process to write the ledger removes. The damaged lines moved there stay until deleted | The same OS account. Written only through `internal/mark/confdir`; read by the reading server, to put back the marks git took out of this computer's file, and by `yomihon marks --adopt`, which reads the earlier path's and copies it. It holds the reader's words, as the file in the vault does, and is not in the vault, so git neither carries nor removes it |
| The machine id — 8 random hexadecimal digits, made at a vault's first write and kept as a `header` line of that vault's ledger, with a code for each process of yomihon that has written for it. It is the name of this computer's file of marks in the vault and the `m` of each line, the process code the `s`. It is not derived from the host, the user or a path, and it names none of them | The ledger in the per-vault directory of the configuration directory, and, as a file name and in each line, the vault | Until the ledger is deleted, when the next write makes a new one; also replaced, and said to the log, when a line under it that none of this computer's processes wrote shows that another computer holds a copy of this configuration. What was written under the old id stays this computer's and is carried into the file of the new one | The same OS account for the ledger; whatever can read the vault for the file name and the lines |
| `instance.json`, in the same directory — a running yomihon's record of itself: its process id, the starting port it holds and when it started | The platform's configuration directory | Until the process ends, which removes it; one a crash leaves names a process that is gone, and is ignored | The same OS account. Written by the running yomihon, read by `yomihon <dir>` to find it |
| A proposed thought note's frontmatter and `based_on` link | The response's read-only text field; the OS clipboard when the reader copies it; the `obsidian://new` URI when the reader chooses that handoff | The page lifetime; clipboard and external-editor policies own their copies | The local browser, clipboard, and selected editor |

- **The hover card sends a link's own fragment to the server.** A browser never puts a
  URL fragment in a request; the preview endpoint takes it as a `?section=` query so the
  card can cut at the section or block the link addressed. It stays in the same loopback
  request buffers as any other query and reaches no process yomihon starts.

About those rows:

- **An unreadable approvals file approves nothing.** A missing file is no
  approval; one that cannot be read or carries an unknown shape is reported,
  every figure is served without script, and no later approval replaces it.
- **The marks are the one thing of the reader's that lives in the vault.**
  The highlights, thoughts and questions are in
  `reader-notes/.yomihon/marks/<machine id>.jsonl`, one file for each
  computer. Each computer writes only its own file and reads the others', which
  git brings in; two computers never write one file. The scan always leaves out
  the top-level `reader-notes/`, so they are never listed or indexed as notes,
  and a read creates nothing there, except that a page that reads the marks may put back into this computer's own file, at most every few seconds, the marks git took out of it. A write is refused when `reader-notes` is a
  link, is not a folder or is read-only, and the reader is told only that the mark could not be kept; why goes to the log. yomihon
  never edits, creates or deletes a note of the reader's or an agent's; the only
  vault paths it writes are under `reader-notes/.yomihon/marks/`. Lines of other
  computers are shown even when older or newer than this version when they can
  be; lines it cannot read are counted and logged, never dropped. A mark that two
  computers edited differently keeps every value, never overwritten silently: the
  other computer's in its own file, and this computer's own that a later change
  replaces while hidden, set aside whole in the ledger first; the reader
  sees the newest live one and nothing says there
  is another. The file is the reader's words: committed to git, they
  stay in its history, and a program or agent that can read the vault can read
  them.
- **The rest of the reader's state stays outside the vault.** The directory
  name is a digest of the resolved absolute vault root, so two vaults keep
  separate files and the directory name spells no path. `progress.json`, the
  ledger, the machine id and the earlier `marks.json` each sit there, and the
  files that name a vault name its absolute path. Nothing syncs or expires
  them. `check`, `coverage` and `exists` read none of it or of the marks.
  `yomihon marks` reads the marks and never writes them or the vault, except
  with `--adopt` (below). Every read of these files has a size cap, and a file
  longer than its cap is treated as one that cannot be read. Deleting the
  per-vault directory clears the bookmarks, the ledger and the machine id, not
  the marks in the vault.
- **The ledger is this computer's record of the marks.** One line for each change,
  per vault, in the configuration directory, where git cannot take it back. A
  line is recorded before the file in the vault is written, so it is the commit
  point; when git removes marks from this computer's own file, the next page that
  reads the marks writes them back, with nothing said to the reader and a line
  in the log. It is on the same computer as the
  vault, so it does not survive that computer's failure, and it holds the
  reader's words as the file does.
- **Reading the marks without yomihon.** The files are JSON lines, and the
  rule for which line of a mark counts is one `jq` can follow. Run this in the
  vault's folder (the program is `internal/mark/machinemarks/read-marks.jq`,
  which the tests run against what yomihon reads):

```sh
jq -c -n -R '[inputs | fromjson? | select(type == "object" and .fmt == 1 and (.mark.id | type) == "string")]
| map(. + {w: (.m + "/" + (.s // ""))})
| unique_by([.w, .rev, .mark.id])
| group_by(.mark.id)
| map(
    . as $group
    | [ $group[] | . as $line
        | select(
            any($group[] | . as $other
                | ($other.w == $line.w and $other.rev > $line.rev)
                  or ($other.w != $line.w and (($other.seen // {})[$line.w] // 0) >= $line.rev))
            | not
          )
      ]
    | if length == 0 then ($group | group_by(.w) | map(max_by(.rev))) else . end
    | sort_by([.t, .w, .rev]) | reverse
    | (map(.mark.deleted != true) | index(true) // 0) as $first
    | [.[$first]] + .[:$first] + .[$first + 1:]
    | {id: .[0].mark.id, mark: .[0].mark, conflict: (.[1:] | map(.mark))}
  )
| .[]' reader-notes/.yomihon/marks/*.jsonl
```

It prints one object for each mark: the value that counts (`mark`) and the
other values that two computers' differing edits left (`conflict`), which no page of the reader shows. It does not check the checksum of a
line, hold a field to the bounds yomihon holds it to, or read this computer's
ledger, so a line changed by hand is read by it and not by yomihon, and what
git took out of the working tree is not seen. Linux and Windows need `jq`
installed.

- **What a program that reads the files directly gets, with no export.** The
  privacy declaration binds what yomihon says, not what the account can read. A
  program or agent that can open the vault reads every mark's words, a
  question's included, from the machine files, and a mark the reader deleted
  stays there whole, its words with it, as a line marked deleted. The ledger in
  the configuration directory, outside the vault, holds every change this
  computer made, each with the words it had then, and the values it set aside,
  and is read by any program of the same account. Such a program can also write
  a machine file; yomihon finds a line that was changed (its checksum does not
  fit) and counts it, and cannot prevent it. Keeping the words from a program
  that reads files takes the operating system: another account, another
  working directory, permissions or a sandbox.

- **`yomihon marks --adopt <earlier path>` is the one command-line write to
  the reader's files in the configuration directory.** A vault's directory is named from its path, so a vault that
  moved has none of its marks until they are brought across. The command reads
  the directory the earlier path names, which has to record that path in its
  files, and writes this vault's own `marks.json` and `progress.json` under the
  same configuration directory with the marks, bookmarks and finished notes found
  there — and only those on a note this folder holds and the rules let be read,
  because a mark on a note this folder has not got, or on ground it withholds,
  would reach the list as a path this folder's rules never judged. It also carries
  the ledger kept for the earlier path, `ledger.jsonl` and the lines set aside
  beside it, whole and byte for byte, into a vault that has no ledger of its
  own, without asking the rules of it: the ledger is this machine's record of
  what it wrote into the vault's own file of marks, which moved with the vault,
  and not a list that reaches an output. A vault that has a ledger keeps it.
  The earlier
  directory is only read, never changed. Figure approvals are not carried: a
  permission about a folder's figures is given again in the new place. The
  command says only that it adopted, and no count, since a count would say how
  many marks sit on withheld ground; it never writes the vault. The `marks.json`
  it writes is taken into the vault's machine file by the first write of a
  computer that has none.
- **`yomihon marks` hands the reader's marks to whoever runs it.** It lists
  each marked location — the note's path, the anchor and the words of the
  heading or practice card it names, the note's title, a wikilink,
  and the time the mark was made, in UTC — and each thought that still
  declares `draft: true`, with its path, title and the day its `created` field
  declares. A title is the note's first `#` heading, and is left out when there
  is none. No body text is included beyond the words of a heading or
  practice card and the title. The vault is scanned whole, withheld
  ground included, as every command scans it; an item whose path is under the
  contract's `never_egress_dirs` is left out before anything about it is
  resolved into the output, and so is a location whose heading or note title,
  or a thought whose title or `based_on`, names withheld ground. Words name
  withheld ground when they link there in any link syntax — a wikilink, an
  inline or reference link, an HTML `href`, read as plain text so that a code
  span or an escape hides nothing — or when they spell a withheld directory as
  a name followed by a slash: at the start or after a character that is not a
  letter or digit, once HTML character references and percent-escapes are
  decoded (repeatedly), format characters, variation selectors, other
  default-ignorable characters and combining marks are dropped, compatibility
  forms such as fullwidth letters and case are folded, and a backslash or a
  look-alike of the solidus is read as a slash. A letter of another script that
  only looks like a Latin one, such as Cyrillic `Р`, is not recognised. Plain
  words that only share a withheld note's file name are the author's own and
  are listed as written. A location
  whose note or heading is gone is listed with its anchor left empty, since
  nothing is left to check the heading's words against; its path is the
  string recorded when the mark was made, which can name a note since renamed
  or one reached another way. Nothing says how much was left out: when no
  location is listed, stderr says only whether a marks file exists for the
  folder as its path was spelled. There is no separate sensitivity level for marks:
  the contract's privacy declaration is the whole of the filter, and without
  one the command refuses. What it writes is handled by the program that
  called it, usually an agent, and from there it can leave this machine
  through that agent's model provider — including when the reader was unsure
  of what. That is the caller's egress, not a request yomihon makes.
- **`yomihon marks --export` hands the reader's own words to whoever runs it.**
  It is the one way the command prints them, and typing the flag is the
  consent: there is no second switch, and without it no output of the command
  carries a word the reader wrote. It writes Markdown to standard output and
  nothing anywhere else. For each highlight, thought and question that is not
  deleted it writes the words the reader chose, the note's title and its path
  from the vault's root, the name of the section, and the words the reader
  wrote, grouped as open questions, thoughts, highlights and resolved
  questions; it names no vault, no folder on this machine, and no time of its
  own. Where two computers changed a mark differently it also writes every other
  value of it, and the values this computer set aside in the ledger, each under
  its mark with the time it was written (in UTC) and its state, and a deleted
  one as `deleted.`; it does not say which computer wrote a value. A mark is
  written once, whatever number of files hold lines of it, and a value that
  repeats another is not written again. Text that the vault or another computer
  wrote and a terminal would act on (control characters, line separators,
  direction marks) is written out as `\uXXXX`, so the output cannot send an
  escape sequence to the terminal it is read on.
  The privacy declaration is the same as the list's, and its gate is asked of
  everything the command is about to write, before it is written: a mark whose
  note is under a `never_egress_dirs` directory is left out whole, and so is one
  whose note's title, section, chosen words or the reader's own words link to
  withheld ground or spell a withheld directory and a slash, as above. Every
  value of a mark is asked: if one is withheld, the mark is left out whole, so
  the public side of a conflict does not reveal that a withheld one was there.
  A mark of the earlier file that the vault already holds back is not written
  from it either. Nothing says how much was left out, and the rules file is
  looked at again after the output is assembled, as for the list. The list
  refuses when a note it could not read may be a thought still open; the export
  does not read notes for that, so a note that cannot be read does not make its
  marks disappear, and they are written as they were kept. A file of marks that
  could not be read leaves the rest written and one line on stderr, without a
  count, saying the export may be incomplete. With no rules file, the line that
  nothing is withheld goes to stderr and the Markdown stays clean. The reader's
  words are Markdown of their own and are written as they are: the export keeps
  its own structure only for what it writes itself, so words that look like a
  heading are theirs. What the export writes is handled by the program that
  called it, usually an agent, and can leave this machine through that agent's
  model provider; that is the caller's egress, not a request yomihon makes.
- **`GET /questions` is not filtered.** The reading server hands the whole
  set of question marks, withheld paths included, to any local client that asks, as it
  hands every note to the reading room; the privacy declaration binds the
  command line, not the loopback port.
- **`GET /bookmark` is not filtered either.** It answers the note path, the
  anchor, the distance below it and the time of the bookmark for a course or
  for none, withheld paths included, to any local client that asks, and tells
  the browser not to keep the answer. A page shown again from a cache draws the
  place from it.
- **Questions promise a location, not recovery.** A renamed source,
  changed heading, or new sentence from an interactive control may leave the
  saved location unavailable or pointing at different content. There is no
  source identity, relocation, sentence reconstruction, or lossless recovery.
  The only backup is git, once the reader commits and pushes, and the local
  ledger. Deletion, `git clean` or a machine failure can lose marks that were
  never committed. An unreadable or malformed file of this computer's marks or
  an earlier file that cannot be converted produces a visible failure and is not
  replaced by the next change. The marks and the bookmarks are in separate
  files, so damage to one does not cost the other.
- **Nothing is request-logged.** Note bodies and raw search queries never reach
  a log. A failed query records its byte count and filter keys.
- **A local reader may see any vault file, private paths included**, because
  local reading is the product. Agent output is filtered instead: the
  contract's `[privacy]` never-egress directories are scanned so links resolve,
  and never described. An `exists` report on a name, including a vault path or
  path suffix, may carry a bare `withheld` flag, naming no path, field, or
  value.
- **`obsidian://open` links and the Copy path control carry the note's
  absolute path inside the page.** Following a link hands that URI to the local
  Obsidian application. Pressing the control writes the path, with the line of
  the section being read, to the OS clipboard, or shows it for copying by hand
  where the browser refuses. No network request is made.
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
`System/schemas/vault-schema.toml`. Give one of its note types the `thought`
role, and list `based_on` among that type's fields, which the role requires.
For a vault that declares a type named `thought`, the table is:

```toml
[types.thought]
role = "thought"
fields = ["based_on"]
```

This setting is optional and has no default. A contract that gives no type the
role, or one that cannot be used, keeps the handoff unavailable; an ordinary
document and a lesson use the same mechanism. Installing yomihon does not
modify the reader's private contract or add a new note type. The vault owner
makes this opt-in edit.

The proposed Markdown is frontmatter of three fields and nothing else: `type`,
the type that holds the role; `draft: true`; and a `based_on:
"[[source#section]]"` link when opened for a section, or a source-only link for
a whole document. It carries no title, because a note's title is its first
heading and the file name it is saved under. The thought stays on the My marks page
until they remove `draft: true` in their editor. A
field the contract also requires of the type is not proposed; `yomihon check`
reports it missing after the reader saves the note. There are no prompt
excerpts, before/after templates, or required-field pickers. The reader's
editor owns the template, words, remaining fields, and save operation.

## Read-aloud

A passage read aloud is handed to the browser's speech synthesis with an
explicitly selected voice: one the browser reports as running on this device
(`localService === true`), matching the passage's language and script. An
authored `und` passage requires a local default voice. When no such voice is
available, the page says speech is unavailable and hands over nothing. It
never selects a voice marked as remote or leaves the choice to the browser.
This boundary rests on the browser's report; it does not independently verify
the browser or operating system's speech service.

## Browser state

Cookies, session storage and local storage hold the reading choices, where the
reader had the left rail, how tall the diagrams on a page came out, and where
the reader last stopped in each note. Nine cookies carry the choices, and the server writes
every one of them at `/preferences`. Every one but the interface language is
also written by the page's own script when a choice is picked, under the same
name and for the same year, so a choice made with script and one made without
expire together. The language is the exception: its words are the server's,
and nothing but the server writes it. Three `sessionStorage` keys carry the
rail and one carries the diagram heights.

| Kept in the browser | What it holds | How long |
|---|---|---|
| `yomihon_lang` | Which language the interface speaks, `zh-Hant` or `en` | A year from the last write |
| `yomihon_theme` | The desk, `light` or `dark`; no value leaves the system's own setting deciding | A year from the last write |
| `yomihon_textsize` | The text size, `m`, `l` or `xl` | A year from the last write |
| `yomihon_font` | The typeface, `serif`, `sans` or `kai`; no value is the serif face as well | A year from the last write |
| `yomihon_ruby` | Whether furigana show, `on` or `off` | A year from the last write |
| `yomihon_paper` | The light theme's paper, `stone`, `washi` or `white`; no value is stone, and dark ignores it | A year from the last write |
| `yomihon_rail` | Whether the left column stands beside the text or is folded away on wide windows, `open` or `collapsed`; no value folds it. Where the column is a drawer, whether the drawer is open is never written here | A year from the last write |
| `yomihon_sectionbuttons` | When the buttons beside each section are drawn, `always` or `hover`; no value shows them on hover | A year from the last write |
| `yomihon_marks` | Whether the reader's own marks are drawn in the text and beside it, `show` or `hide`; no value shows them | A year from the last write |
| `yomihon_shortcuts` | Whether single-key shortcuts answer, `on` or `off` | A year from the last write |
| `yomihon_position` | Whether a note opens where the reader last stopped in it, `on` or `off`; no value leaves it on | A year from the last write |
| `yomihon.nav` | Which folders of the left rail the reader opened or closed, one true-or-false each | Until the tab closes |
| `yomihon.nav.filter` | The text the reader typed into the rail's filter box, kept as typed so a narrowing survives opening a note | Until the tab closes |
| `yomihon.railTop` | How far the left rail was scrolled when a page was left, and which rail it was scrolled in — a course's own vault path, or the kind of rail — so the next page of the same rail starts where it was | Until the tab closes |
| `yomihon.diagramHeights` | How tall each diagram came out when it was drawn, under a short hash of its source, and the window width and reading choices it was measured in, so a page the reader comes back to holds each diagram at that height before it is drawn again | Until the tab closes |
| `yomihon.recentSearches` | In `localStorage`: the words of the reader's last five searches, newest first, one each; the search panel's Clear removes them | Until the reader clears them |
| `yomihon.positions` | In `localStorage`: where the reader last stopped in each of the last 200 notes they read, newest last. Each entry is the vault's id, the note's path inside the vault, the id of the heading the reader's eyes were under and how far below it | Until the reader turns the setting off or clears it |

- **The cookies go to the loopback listener and nowhere else.** Each carries
  `Path=/` and `SameSite=Lax`, and is deliberately neither `Secure` nor
  `HttpOnly`: the server is loopback HTTP by design, and the script that
  re-syncs a page revived from the back/forward cache has to read the value to
  compare it against the document it revived. Any script the page already runs
  can therefore read them. A cookie is scoped by host rather than by port, so
  anything else served from `127.0.0.1` to the same browser can read them too —
  the same local surface the threat model already accepts.
- **The recent searches are sent nowhere.** `localStorage` is never attached
  to a request, and nothing reads the list but the search panel and the search
  page, which show it back to the reader. It holds at most five entries, a
  repeated search moves to the front instead of standing twice, and it stays
  until the reader clears it. The words are whatever the reader typed, so
  they may name a person or a subject; a browser's own "clear site data"
  removes them too.
- **The places are kept in this browser's storage for this address, and
  nowhere else.** `yomihon.positions` is in `localStorage`, which is never
  attached to a request, so the server never learns where anyone stopped. A
  browser keeps one store for each address: `127.0.0.1` and `localhost` are two
  stores, a place kept at one is not found at the other, and a private window
  or a cleared site starts with none. Each entry names the vault by its id as
  well as the note by its path, because a vault that is removed gives its port
  to the next vault opened and a path alone would hand that vault's note of the
  same name an old place; the id is derived from where the vault is and spells
  none of it. An entry holds no text of a note, but the path and the heading's
  id are the vault's own names and may say what the reader was studying. The
  list holds the last 200 notes and drops the one read longest ago. It is
  written when the reader stops scrolling and when the page is hidden or left,
  and a note read from its top keeps no entry. The setting `yomihon_position`
  turns it off, and turning it off removes the whole list; a page drawn while
  it is off removes it as well, which is how the choice reaches a browser that
  made it where no script ran. The bookmark is another place and this does
  not touch it: the server keeps it, in the configuration directory, when the
  reader presses for it.
- **The session keys are sent nowhere at all.** `sessionStorage` is never
  attached to a request: the rail's state, its scroll position, the filter
  text and the diagram heights stay in the tab holding them and go when it
  closes.
- **`yomihon.nav.filter` holds the reader's own words.** Every other entry in
  the table is a value yomihon offered and the reader picked; the filter box
  keeps what they typed, as they typed it, which is usually part of the name of
  a note or folder they were looking for. That is the only text the reader
  typed that browser storage holds: yomihon itself writes no note title or body
  there, and the only vault names it keeps are the paths that say which course
  or folder a rail's open or scrolled state belongs to. All of it clears — the
  settings page's return to defaults drops the nine cookies, and the browser's
  own controls drop all fifteen.

No hosted reader exists for a user's own vault; a vault stays on the machine
that reads it. One public instance exists over this repository's own sample
notes, reached through a proxy, and the threat model describes what it accepts.
A remote database, vector service, telemetry sink, or cloud backup is absent
and unauthorized. Adding one is a new decision needing an updated threat model
and inventory, not a configuration question.

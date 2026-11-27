# Threat and trust model

A design and test-selection input, not proof of a deployment. The boundary is a
single-user local process with an unauthenticated HTTP listener hard-coded to
`127.0.0.1`; exposing it through a proxy, tunnel, container port, or
non-loopback bind is unsupported. The four walls hold: nothing written into
the vault except the reader's own marks, loopback only, one schema source,
report and never repair. The reader's highlights, thoughts and questions are
kept in one machine file for each computer in the vault, under
`reader-notes/.yomihon/marks/`; the bookmarks, the ledger and the rest are kept
in files of yomihon's own outside the vault.

## What is protected

| Asset | Required property |
|---|---|
| Vault files, contract-private paths included | Confidential outside the local reader; never silently repaired. |
| `System/schemas/vault-schema.toml` | Sole machine authority for the folders it checks, the note types and their roles, the template directories and the privacy capability; missing, invalid or stale authority fails closed. |
| Vault bytes | Written only in `reader-notes/.yomihon/marks/`: yomihon opens no other vault file for writing, renames or removes nothing else in the vault, and creates nothing else there. No note, no contract and no attachment is ever changed. |
| The reader's folder and this machine's file in the vault | `reader-notes/.yomihon/marks/<machine id>.jsonl`, written only by the computer whose id names it, and only through `internal/mark/machinefile`. A path that leaves the vault, a link or a file that is not a folder or a regular file where the folder or the file should be is never followed or written. The marks in it are the reader's words, and an agent or a program that can read the vault can read and change them; anything that arrives in it from another computer is untrusted data. |
| The reader's own marks | Highlights, thoughts and questions are kept in the machine files above; bookmarks and finished notes are kept outside the vault, replaced whole or not at all. `marks` reads the marks without writing them, except with `--adopt`, which copies what was kept for an earlier path into this vault's own files only where this folder holds the note and the privacy declaration lets it be read, and leaves the earlier directory as it was; it answers under the same privacy declaration as the other agent-facing commands. `marks --export` is the only way the reader's own words are printed: it writes nothing, refuses `--adopt`, and asks the privacy gate of every value it is about to print, a conflict's other sides and the values this computer set aside included. Bookmark and question marks occupy independent files. Unreadable uncertainty storage is preserved. With `--adopt` it also carries the ledger kept for the earlier path, byte for byte, into a vault that has none, and leaves a vault's own ledger as it is; no route writes a ledger yet. Marks have no backup or recovery guarantee. A change is reported kept only once the file and its directory are synced to durable storage and the file reads back as written; one that fails any step is reported as failed. |
| Agent-facing results | Contract-private paths neither appear in nor influence results, with one exception: `exists` answers whether a caller-supplied name, including a vault path or path suffix, is taken, disclosing that bit and nothing else. |
| Browser authority | Authored vault bytes stay display input, never first-party script, navigation, form, frame, or automatic remote-resource authority. A figure runs in an opaque-origin frame: unapproved, it runs no script and the browser fetches nothing it names on its own; approved, it runs exactly its approved inline scripts and still holds no first-party authority. |
| Figure approvals | Kept outside the vault, owner-only, replaced whole; written only by the approval commands at a terminal, never over HTTP. Unreadable or malformed approvals are no approval. An approval names exact bytes and lapses on any change. |

## Who is trusted

Koopa is the sole user and security owner: no account system, session, role, or
token exists. The local browser and any process that can open the loopback port
are equally unauthenticated — loopback is not authentication, and browser
cross-origin protection is not a local-process access control. The vault author
supplies untrusted bytes; being in the vault grants no browser authority, and
nothing grants write authority over the vault's notes. A same-UID process sits inside the host trust base, and owner-only
modes and identity rechecks prevent accidents and many races, not a malicious
process that can already read the vault. OS, browser and filesystem are trusted.

## Where the boundaries are enforced

| Boundary | Enforcement |
|---|---|
| Network | `cmd/yomihon` creates only `127.0.0.1:<port>`. Requests are bounded in header size and read/write/idle time; final responses carry same-origin CORP, a nonce-bound CSP, `nosniff`, `no-referrer`, and DNS-prefetch refusal. |
| Rendered vault data | Only bare `ruby`, `rt`, `rp`, `br`, `kbd`, `sub`, `sup`, `mark` and `u` are admitted as authored HTML; only `ruby`, `rt` and `rp` may carry `lang=`. Other tags or attributes remain escaped text. Raw markup uses scriptless sandbox or escaped-source boundaries, and is served as a download rather than opened as a page, because a link element can ask the browser for a page outside the sandbox's reach; remote Markdown images become user-activated links. A figure is the one markup document opened as a page; see the figures section below. |
| Process to vault | `vault.Reader` and `os.Root` pin the selected root, and the pin is read-only. `internal/mark` is the one package that writes: the reader's own files under the platform's configuration directory through `internal/mark/confdir` (among them the per-vault ledger `internal/mark/ledger` keeps, which appends to and replaces its file only through confdir's `Append` and `Replace` and names no write of its own), and this machine's file of marks in the vault through `internal/mark/machinefile`, which opens a root of its own from the pinned path and writes only once that root is the same directory as the pin. A test reads every shipped file for a spelling that changes the filesystem and fails on one outside `internal/mark/confdir` and `internal/mark/machinefile`. The second is the only code that may write inside a vault: the same test holds it, by the types of each call's arguments, to making and entering `reader-notes/.yomihon/marks/`, creating a temporary file there, renaming that file over this machine's own `<machine id>.jsonl`, and removing a temporary file, so no other file in the vault can be created, opened to write, renamed or removed through it. Each folder on the way is entered through the one above it and refused if it is a link or not a folder, checked against the folder actually opened. What the test cannot read is held by machinefile's own tests: that the id its caller passes is this machine's, and that a stale temporary file is removed only when its name has the shape a temporary file is given, so no other machine's file is. What `internal/mark/machinemarks` (which the routes of `internal/mark` call) puts into that file and reads out of it is held the same way: it calls only machinefile, the ledger, confdir and sealedline, so it can write nothing else; a write holds the vault lock `machine-file.lock` and records the line in the ledger before the file is replaced. Files other machines wrote come through git and are untrusted input: each line is read only if its checksum matches and its shape is right, a line with conflict markers, a line of a later format, an oversize file and a file that is a link, a folder or a pipe are skipped and counted, so one such file neither stops a read nor hides another machine's marks; a line under this machine's own id that none of its processes wrote means another computer has a copy of this configuration, so this machine leaves the id for a new one and says so to the log; what it wrote under the old id stays its own and is carried in the file of the new one, and the foreign lines are never taken for its own; a line that is not the shape of a mark (a path that leaves the vault, a field past its bound, a highlight with words of its own, a thought with none) is skipped like a damaged one, as a mark of the earlier file out of that shape is skipped at the take-in and left in that file. This machine's own file that cannot be read does not stop a read, which goes on from the ledger and counts it, and only stops a write, which would replace what it did not read. The checksum catches damage, not a forger: anyone who can write the vault can write a valid line, which is the same trust the notes themselves have. The reader's words in a mark are read out of the engine only through a privacy gate, which is asked of every value of a mark, the values this computer set aside among them, and hides the whole mark when it refuses one. Writes take no gate: the only writer is the reading server, on the reader's behalf, and the reader owns every mark. The reading server reads under a gate that lets every mark through: it answers the reader, at their own browser on the loopback, about marks they made, and the privacy rules keep an agent's notes out of what the commands print, which is where they apply; `marks --export` reads under them. A folder that is the configuration directory, holds it or is inside it is refused at the start, when it would join the list of vaults, and by `yomihon marks --adopt`, `approve-figure` and `revoke-figure`, each before anything is written: the two are resolved through every link and `..` and compared directory by directory with `os.SameFile`, so another case of a name on a filesystem that ignores case, a link, or a second spelling of a drive is the same directory. A vault listed before that refusal is not served, is reported once, and stays listed for `yomihon vaults remove`; a listed vault is compared again when it is read, and one that overlaps or cannot be compared then is not read. A folder that cannot be compared at the start is read with nothing kept, as with no configuration directory, and the three commands refuse it. Paths are vault-relative and normalized before privileged use. |
| Contract to privileged action | `internal/schema` derives capability from the exact contract source. Agent output fails closed without valid authority. |
| Adding a bookmark | `internal/mark` writes `progress.json` under the platform's configuration directory, never into the vault, through `internal/mark/confdir`, the one place that writes there. `POST /bookmark` is capped at 4 KiB and holds each field to a fixed shape. The path must be vault-relative, in NFC, valid UTF-8 and free of control characters, and a place is accepted only for a path that is a readable note in the current snapshot. The anchor must be valid UTF-8 and id-shaped, at most 256 bytes. The offset is an integer within one document and the identity a 64-digit lowercase hex digest. Only the note is looked up; the anchor is held to its shape, since it reaches the desk only inside an address and never as text. Ordinary notes fit this shape, but the list of those that do not is not closed. A reading page still offers the control, and pressing it answers 422, when the note's own file name holds a control character (a tab or a line break, which Linux and macOS allow), or when the nearest anchor above the reader is a block address holding a character an id cannot (such as `/`, `?`, `#`, a quote, `<` or `>`) or a heading id longer than 256 bytes. The refusal of such a file name is deliberate, so that a path which is not one line of text is never stored; the refusal of such an anchor is how the route already behaved. The file is written to a sibling temporary name, synced, and renamed over, the directory is synced, and the file is read back; the bookmark is reported kept only when all of that succeeded. `GET /bookmark` answers the note, the anchor, the distance below it and the time of the place kept now, or no content, marked `no-store`, and writes nothing; it never sends the content identity. It is the reader's — an agent never calls it. |
| Marking a question | `internal/mark` writes this machine's file of marks in the vault through `internal/mark/machinemarks`, a file separate from the bookmark's. `POST /questions` is capped at 4 KiB; path and anchor must be valid UTF-8 and shaped like a vault path and a document id, and the server assigns the time. A new mark is accepted only for a path that is a readable note in the current snapshot and an anchor that note renders (an empty anchor names the note as a whole), and only while fewer than 5000 marks, of every computer, are live; clearing a mark is always allowed. Only the kind, a random id, the path, the anchor and the time are stored for such a mark. A toggle reads the marks first and refuses a write that would replace what could not be read, and a question that two computers changed differently is not taken away blind: the answer is 409. Writes are serialized within the serving process and with any other process over the same vault by the vault lock, and each is reported kept only once the ledger line and the file are synced to durable storage and the file reads back as written. `GET /questions` exposes location records to the local reading client and reports unreadable storage as failure, not an empty list. An agent never calls the write route. |
| Setting, putting back and clearing a bookmark of a course, and finishing a note | `PUT /bookmark` sets the bookmark of a course (or of reading outside any) and answers the one it replaced, `DELETE /bookmark?course=` clears one and answers it, and `PUT` and `DELETE /finished/{path...}` say a note is finished or is not. Each writes only through `internal/mark` into `progress.json`, as the form route does. A body must be `application/json` (anything else is 415), is capped at 4 KiB, may carry no field the route does not name and no second value, and every field is held to the stored shape and bounds. The note must be readable in the current snapshot. A bookmark in a course, and a finished note that names one, is kept only if the course walks the note, on its main line or on a side branch, read through the same projection of the navigation the sidebar uses, so a course nothing answers to is refused and a distrusted rules file, which closes the courses, refuses every course. The time is the server's, in UTC to the millisecond, unless a bookmark is put back, when it is the one the page was handed, brought to UTC and the millisecond, and may not be a minute ahead of this machine's clock. A page that puts one back says which bookmark it expects to replace, by its time or as none, and the store checks that under its lock: where another page has changed the bookmark since, the answer is 409 and nothing is written, so an undo cannot overwrite a later move. A note finished twice keeps the first time. The bookmarks are written in the order of their courses, so the same set of bookmarks is the same file. Clearing a bookmark, and saying a note is not finished, do not ask the vault or the course, so the bookmark of a course or a note that is gone, and the entry of a finished note that is gone, can still be cleared; only the shape of the names is held. A file that cannot be read is answered with a failure and left as it is. An agent never calls these routes. |
| Keeping, changing, removing and putting back a mark | `POST /mark` keeps a highlight, a question or a thought, `GET /mark/{id}` reads a mark with what the page has read of it, `PUT /mark/{id}` puts a deleted one back, `PATCH /mark/{id}` resolves a question or changes a question's or a thought's words, and `DELETE /mark/{id}` deletes one. Each writes only through `internal/mark` into this machine's file of marks in the vault, as the question route does. A body must be `application/json` (anything else is 415, which is all a page of another origin can send without the preflight the cross-origin protection refuses), is capped at 64 KiB, may carry no field the route does not name and no second value, and every field is held to the stored shape and bounds. The id in the address is checked against the 32 lowercase hex digits the store issues before anything is looked up, so a request for any other spelling of an id is 404 and reads nothing. A new mark is kept only in a note that is readable in the current snapshot, at an anchor that note renders, and, for a quote, only if the quote is among the words of the page the reader sees (the text of the sanitised HTML), so the route does not store words the note never had; the time and the note's content identity are made by the server. A change says which lines of the mark the page had read; one made on a version that is no longer the latest is answered 409 with what is read now and nothing is written, so an edit does not silently overwrite another computer's. A delete keeps the mark's words in its line, which is what putting it back is made from. A write the reader folder cannot take (a link, not a folder, read-only) is answered 422 with the reason, and a read creates nothing, except that a page that reads the marks may, at most every few seconds, put back into this computer's own file the marks git took out of it, from the ledger. A mark on a note the privacy declaration withholds is kept, because it is the reader's own; what the declaration withholds is what leaves for an agent, and no route here sends a mark's words anywhere but back to the page that sent them, as JSON. A file that cannot be read is answered with a failure and left as it is. An agent never calls these routes. |
| External thought-note handoff | `internal/schema` reads the optional `thought` role a note type may hold in its `[types.<name>]` table, and accepts it only on a type that lists `based_on`. The role is read once when the server starts, like the `course` and `map` roles; withdrawing the declaration takes effect at the next start. The page offers derived frontmatter (`type`, `draft: true` and a `based_on` link) for copying or an explicit external-editor action. The reader's editor creates and saves the note; the server writes no note into the vault. No private contract is installed or amended automatically. |

`YOMIHON_PORT` is the only environment value this program names, held to a
mechanically tested allowlist; the bind host is not configurable, and the vault
root comes from an argument or the working directory. One more value reaches it
without being named: `os.UserConfigDir` reads `HOME`, and `XDG_CONFIG_HOME`
where the platform has one, inside the standard library, so the allowlist
cannot see the key. That call decides where the reader's marks are kept, and
the same test refuses it anywhere but the one file of `internal/mark/confdir`
that makes the configuration directory, which the command calls at startup — a
guard that stayed green over a widened surface would be worse than none. A
directory the platform names that is not an absolute path is refused there, so
it cannot be read against the folder being served; the command then runs with
no configuration directory, as it does when the environment names none, and
the server's log and `yomihon marks` say why. The home directory is read a
second way, `os.UserHomeDir`, in the one file of `internal/vault` that spells a
path under it as `~/…` for a person; the same test refuses it anywhere else,
and the read changes how a path is written and nothing the program does.
Search queries are capped at 4,096 bytes and reject control characters. Go dependencies are pinned by
`go.mod`/`go.sum`; redistributed assets carry their LICENSE files inside `assets/` (fonts/LICENSE.txt, js/mermaid/LICENSE).

`[privacy] never_egress_dirs` binds the agent-facing output contract — `check`,
`coverage`, `exists`, `marks`, `marks --export` — and not the reading room. Measured on `examples/vault`'s
`Diary`: the CLI withholds even the name (`{"matches":[],"withheld":true}`),
while over HTTP the note's body reaches `/raw`, `/notes` and `/preview` and its
name reaches six doors; only Home, `/courses` and `/reports` are clean. The
declaration therefore governs what yomihon says to a program, not what a
process on this machine can read; loopback cannot tell a person's browser from
an agent's `curl`, and a local agent reads the files directly anyway: the machine files of marks hold every mark's words, a deleted mark's included, and the ledger beside them every change this computer made. Binding
the reading room too would be a separate decision (2026-09-04, #149).

## Figures

A figure is an `.html` file a note embeds with `![[name.html]]`, shown in a
frame. It exists because the owner ruled on 2026-11-12 to allow two tiers of
interactive figure, with security held as follows. No domain is trusted and
no figure loads anything from any domain, so a domain that changes hands, is
compromised, or redirects elsewhere has nothing to load into a figure. That is
about what comes in. An approved figure can still send data out, to any host,
through the residual channels below.

| Tier | When | Frame | Response policy |
|---|---|---|---|
| Scriptless | Every figure, unless the next row holds | `sandbox=""`, `referrerpolicy="no-referrer"`, `allow=""` | The raw-file sandbox, unchanged: `default-src 'none'`, no script, no connection, no frame, no image, font or media, inline style only |
| Scripted | The figure has inline script **and** the SHA-256 of the bytes just read is in the owner's approvals | `sandbox="allow-scripts"`, never `allow-same-origin`, `allow-forms`, `allow-popups*`, `allow-modals` or `allow-top-navigation*` | The same policy with `sandbox allow-scripts` and `script-src` listing one `'sha256-…'` per inline script, computed by the server from the parse; no host, no `'unsafe-inline'`, no `'unsafe-eval'`, no `'strict-dynamic'` |

The reading page's own policy keeps `frame-src 'self'`, and the frame's
document route answers GET only, sends no `Content-Disposition`, and reads the
file once: the digest, the tier, the script hashes and the bytes sent all come
from that one read. The figure handler itself never redirects. The router in
front of it answers a path that is not clean — a `..` segment or a doubled
slash — with a 307 to the clean path on the same origin; no address tried
(`..//evil.com`, a backslash, `///evil.com`) produced a redirect off it. The
browser applies the narrower of the frame attribute and the response policy,
so a page drawn before a file changed can only err towards running less.

A figure is framed only on its own note's page. Every other page that shows a
note's words — a hover card, a course's opening, the concept sheets a lesson
carries, the compare page, the listening page — draws each figure as its
stand-in, with no frame, so an approval runs a figure where its note is read
and never where its reader only passed a link to it or read another note. A
note embedded in another with `![[…]]` shows its own embeds as links, so a
figure in it is not framed either. Search results, backlinks and source cards
carry text, not a note's rendered body.

**Refused at serve time.** The bytes are parsed with a spec tree builder twice,
with scripting off (so `<noscript>` content is markup, as the scriptless frame
builds it) and on, and every node is visited, `template`, `svg` and `math`
content included. A figure is refused, and none of it sent, for a nested frame
(`iframe`, `frame`, `frameset`, `embed`, `object`, `fencedframe`, `portal`),
`<link>`, `<base>`, a `<meta http-equiv>` other than `content-type`,
`content-language` or `content-security-policy`, a `ping` or `attributionsrc`
attribute, a script with `src` or `href`, a script type other than classic or
module, an `on…` attribute, `javascript:` in any attribute, more than 1 MiB, or
bytes that are not UTF-8 text. Measured with the raw policy and no refusal,
Chrome 157 opened a TCP connection for a static `<iframe src>` and for one
inside `<noscript>`, and Safari 27.2 opened one for a static
`<link rel=preconnect>`; with the refusal, neither browser saw any traffic.

**Measured.** Chrome 157.0.8103.78 (headless, with its local-network checks
off so the loopback stand-in behaves as a remote host would) and Safari 27.2
(windowed, driven by a page of its own), both on macOS 27.2, against the real
route, with a listener per channel logging HTTP requests, TCP connections and
UDP packets:

- *Scriptless figures:* no request, no connection and no packet in either
  browser, for pictures, `srcset`, posters, media, `<input type=image>`, SVG
  `image` and external `use`, CSS `@import`, `url()`, `@font-face`, cursors,
  `image-set()` and list images, and an unclicked link and form. A figure with
  scripts and no approval ran none of them, framed either way.
- *Approved figures, closed in both browsers:* `fetch`, XHR, `sendBeacon`,
  `Image`, WebSocket, EventSource, workers, script by address, `import()`, CSS
  by script, `FontFace`, `window.open`, form submission, `ping`, media and
  `object`/`embed` by script, and a same-origin address that redirects outside.
  `eval`, `new Function`, `setTimeout` with a string, injected inline script,
  `javascript:` addresses, event attributes, `data:` scripts, and a `srcdoc`
  child's script of any other text did not run. Dropping one hash from the
  policy stopped exactly that script.
- *Approved figures, residual (accepted for approved figures only — the owner's
  ruling of 2026-11-12, below; the weaker browser holds for both):* a WebRTC peer sends UDP to its STUN address (both
  browsers); a navigation the figure starts — by `location`, a refresh it
  inserts, or a link it clicks — opens a TCP connection with no HTTP request
  (Chrome; Safari opened none); a frame the figure creates opens one (Chrome);
  a `<link rel=preconnect>` the figure inserts opens one, and so does a link
  with `target=_blank` it clicks (Safari). A connection to a host name is
  preceded by a DNS lookup that names the host to whoever resolves it; that is
  inferred and not measured, since the measurement used IP literals. Widening the page's `frame-src` to `*` let each navigation's
  request through in both browsers, which is the control that shows
  `frame-src 'self'` is what closes it.
- A `srcdoc` child the figure creates inherits the frame's policy, so it runs a
  script only when its text is byte for byte one the owner approved.

**The residuals are accepted** (owner's ruling, 2026-11-12). Chrome 157 does
not recognise a CSP `webrtc` directive and offers no permission to switch
WebRTC off, so an approved figure's script can send arbitrary data to any host
and port over WebRTC, and — inferred, not measured — with an endpoint of its
own it could read replies as well. The TCP
residuals carry less: a connection opens and no request follows, so what leaves
is a host name, a port and a timing, and nothing comes back. Closing them would
leave what an approved figure can do unchanged while WebRTC stays open. So an
approved figure is treated as able to send anything anywhere, and approving one
means trusting its exact bytes with that.

The standing browser probe `.github/e2e/figure-boundary.mjs` repeats the
Chrome rows on every run and turns red on any HTTP request, and on any
connection or packet outside the residuals it names. Safari has no standing
probe. Its rows come from two runs, on two builds of this change, of a
throwaway harness that is not in this repository; a Safari change is not
caught.

**Approval.** `yomihon approve-figure` reads the file once, prints every inline
script in full from that read — with control (Unicode Cc) and format (Cf,
zero-width and text-direction characters among them) characters and line and
paragraph separators (Zl, Zp) shown as escapes, and a new line started after
each character JavaScript ends a line at, so the terminal draws the code that
runs, line for line — and pins the
digest of exactly those bytes once the person types its first 8 characters
back. What the owner reads is what is approved; a change after the read lapses
the approval. The 8 characters confirm attention; they are not a lock.

What approval does **not** defend against is a program running as the owner.
The command asks for a character device other than the null device on its
input and output, which stops an answer piped in and nothing more: any program
with the owner's shell — a coding agent included — can approve, by allocating
a terminal of its own (the `script` command does) or by writing `figures.json`
directly, as the probe does. That is outside this model, like any same-UID
process. What is defended: HTML nobody approved runs no script, and an approval
binds the exact bytes it was given. The authoring skill tells agents never to
run `approve-figure` or write `figures.json` for the owner; that is a rule of
conduct, not a boundary. The public instance holds no approvals, so it serves
only scriptless figures.

## Reader marks in the vault

The reader's highlights, thoughts and questions are the one thing yomihon writes into a vault, which makes `reader-notes/.yomihon/marks/` the one place where vault content can be changed by this program, and a place git, other computers and other programs can change too. What is held, and where:

| Threat | What holds |
|---|---|
| A path or a link that leads a write out of the folder: a `reader-notes` that is a link, is not a folder or is read-only; a link or a folder where a file belongs; a mark whose path leaves the vault | Each folder on the way is entered through the one above it and refused if it is a link or not a folder, checked against the folder actually opened, and the write goes through a root of its own that is held to be the same directory as the pin. A link is refused wherever it points. The scan always leaves out the top-level `reader-notes/`, so a mark is never read as a note, and a read creates nothing (a page that reads the marks may put back this computer's own marks that git took out, which writes only inside the checks above). A mark's path is held to a vault-relative shape and a path that leaves the vault is skipped as a damaged line. |
| Lines another computer brought in through git, or another program wrote: a forged or damaged line, conflict markers, a later format, an oversize file, a file that is a link, a folder or a pipe | The files of other computers are untrusted data, never instructions. Each line is read only if its checksum matches and its shape and bounds are right; the rest are skipped and counted, and the count goes to the log and not to the reader, so one file neither stops a read nor hides another computer's marks. Every value is drawn escaped, as text, and none is read as markup, a path to open or a command. The checksum catches damage, not a forger: anyone who can write the vault can write a valid line, which is the same trust the notes have, and the reader's words in a mark are readable by whatever can read the vault. |
| A tampered or damaged ledger | The ledger is outside the vault, in the configuration directory, and each line ends in a checksum of the bytes before it. A line that is whole and does not fit its checksum is counted, never read as an event and set aside byte for byte; a line a crash cut short is read past. A line put back from the ledger is held to the same shape and bounds as one read from the file, so a line in it cannot name a path that leaves the vault. A same-UID process that rewrites the ledger consistently is inside the host trust base. |
| A race between processes, or between two routes: two yomihon processes on one vault, a retry, two tabs | A write holds the vault lock `machine-file.lock` in the configuration directory, and the ledger takes its own lock inside it, always in that order. A change says which lines of the mark the page had read; a change made on a version of this computer's own lines that is no longer the latest is answered 409 with what is read now and writes nothing. A change is made on the value the page read, which its `seen` says, and not on the newest one: a value another computer wrote that the page never showed is neither copied into the change nor replaced by it, and stays a value of the mark, so a resolve or a delete sent from an older page loses no one's words. A mark that two computers edited differently keeps every value: the other computer's stays in that computer's own file, which this one never writes, and a value of this computer's own that a change replaces while it is hidden (made on a page that was shown the other value) is first recorded whole in the local ledger as a set-aside line, before the change that replaces it, which a compaction never drops and which is never read as a value of the mark. The reader is shown the newest live value, and no page or route answer says there is another; a read of a mark carries its set-aside values beside the others, under the same gate, for a command that prints them. |
| A crash or a kill between the steps of a write | The ledger line is the commit point: it is recorded and synced before the vault is written, so a file that was not written, or was taken away, is written again from the ledger by the next change or by the next page that reads the marks. A temporary file a crash strands is removed only when its name has the shape yomihon gives one, so another computer's file is never taken for it. A file replaced while it holds anything this machine did not write is first kept whole in the ledger. |
| Git, or the reader, removing marks that were never committed (`git stash`, `git clean`, `git reset --hard`, a checkout) | The ledger keeps this computer's marks outside the working tree, and the next page that reads the marks writes back the ones missing from this computer's own file, with nothing said to the reader and a line in the log. This is a local convenience, not a backup: the ledger is on the same computer as the vault, and what was never committed and never recorded cannot be recovered. |
| A second computer with a copy of this one's configuration | A line under this machine's id that none of its processes wrote means another computer shares the configuration; this machine leaves the id for a new one, logs it once, and never takes the foreign lines for its own. |
| `yomihon marks --export` carrying the reader's words, or a withheld note's, to whoever runs it | The flag is the reader's consent and without it no output of `marks` carries a word the reader wrote. Every note path is asked of the privacy gate before anything about it is resolved, and the title, the section, the chosen words and the reader's own words are asked too (`Gate.Spells`), so a mark in withheld ground, or one whose text links to or spells withheld ground, is left out whole and nothing says it was. Every value of a mark is asked, a conflict's other side and a value this computer set aside included: one that is withheld takes the mark with it. The gate does not read notes, so a note that cannot be read does not make the command refuse, and a mark on it is printed as it was kept. A rules file that is damaged or without `[privacy]`, or one that lies above or inside another folder's, is refused as the list is; a folder with no rules file withholds nothing and says so on stderr; and the rules file is looked at again once the output is assembled. Text that comes from the vault or from another computer is written with control characters, line separators and direction marks as visible escapes, so it cannot act on the terminal; the text that is quoted is fenced when a quotation block would draw it as something else, and no quote, title, path or section can leave the shape of the citation: a quote that holds `<`, `[`, `]` or `&` is fenced too, since a consumer that draws more than the page does would read them as markup, and a path that starts with a scheme, a host, the root or a backslash is written after `./`, so a mark's path that another computer wrote is never a link to anywhere but the vault, and `#`, `?` and `%` in a path are percent escapes, so a note named `C#` is linked whole and not as `C` with a fragment. `--export` refuses `--adopt`, which writes. Not covered: the reader's own words, which are Markdown, and the display text of a link to withheld ground (an alias, or the words of a Markdown link), which is the public note's own text and which the gate leaves as written for the list too, so that a quote of it is printed without the address; and what the program that was handed the output does with it. |
| A page of another origin, or a rebinding of a name to the loopback address, asking for a write | The whole reading site is wrapped in `http.NewCrossOriginProtection`, which refuses a cross-origin write, and `origin.LoopbackOnly`, which refuses a request whose host is not the loopback address, so a rebinding is refused too; the write routes also take only `application/json` (anything else is 415), under a body cap. Neither is authentication of a local client: any process that can reach the loopback port can write marks. |
| Growth: a file that never stops growing, or a vault flooded with marks | At most 5,000 marks that are not deleted, of every computer, are kept; past that a new mark is refused and the reader is told the mark could not be kept. A line holds at most 64 KiB, this machine's file at most 20,000 lines, and a file of more than 16 MiB is skipped and counted. |

## The one public instance

This repository operates one public instance, `yomihon.koopa0.dev`, over the
sample notes committed under `examples/vault` and nothing else. The README
links it from the front page.

It is not a supported way to run yomihon, and nothing above is relaxed for it:
the binary still creates only `127.0.0.1:<port>`, the bind host is still not
configurable, and exposing the listener through a proxy or a non-loopback bind
is still unsupported. What stands in front of that listener is an operator's
decision taken outside this repository.

What that instance accepts:

- **An unauthenticated reader** over notes that are already public in this
  repository. No account, session, or token exists there either.
- **Shared reader marks, bounded, wherever those routes are exposed.** There
  is no per-visitor mark store: the bookmark belongs to the serving machine and
  vault, outside the vault directory, and the marks are the machine file of the
  served vault, in its `reader-notes/.yomihon/marks/`. Whether that folder can be
  written to there, and whether the hourly restore of the sample notes also
  clears it, are set outside this repository, and nothing here relies on either.
  What a visitor can add to the two is bounded differently.
  The marks are bounded by their guards: the place must exist in the
  served vault (a readable note and an anchor it renders), a quote must be among
  the words of the page, a thought's words are a visitor's text of at most 64 KiB
  a line, the vault holds at most 5,000 live marks, and path and anchor must be
  valid UTF-8 (so a request cannot make the file unreadable). A visitor's
  words are drawn escaped and are never markup.
  The bookmark file holds one value that replaces itself, so it cannot grow.
  Its path must be a readable note in the served vault and carry no control
  character, so a path that names no note is refused with 422 and what Home
  prints as text for the place is the vault's own, never a visitor's words. Its
  anchor is visitor-chosen text, bounded by its shape (up to 256 bytes,
  id-shaped, valid UTF-8) and shown only inside an address; the offset and the
  identity are a bounded integer and a 64-digit hex string. A visitor can still
  point the shared Home row at any note the vault holds. Invalid UTF-8 is refused
  there so that a value is not silently rewritten to U+FFFD on its way into the
  file; the file stays readable either way. Whether an anonymous instance should
  accept marks at all remains the owner's decision.

The mutable exposure is the bounded shared reader marks; the note content is
already public and nothing a visitor does changes it. A second instance, or one over any vault
that is not `examples/vault`, is a new decision.

## What is not defended

- An approved figure's residual channels above: a figure can send data to a
  WebRTC peer, and open connections that a network observer sees. Approval is
  of the bytes, not of what they intend; the report the approval command prints
  is a reading aid, not a boundary.
- A link the reader clicks inside a scriptless figure opens a TCP connection
  to its address before the page's `frame-src` refuses the request (measured
  in Chrome; not measured in Safari). Links are not refused: a reader's click
  is the reader's act.
- Keyboard operation of a figure and screen-reader behaviour inside one were
  not measured in Safari.

- No rate limit, connection quota, or handler concurrency limit, and no
  whole-vault ceiling: repeated requests consume CPU and descriptors, and a
  pathological vault can exhaust scan, render or index resources.
- No authentication of a direct local client, and no isolation from a malicious
  same-UID process. A machine with untrusted local users is outside the model.
- No encryption at rest, secure deletion, or backup and restore.
- No recovery of lost questions, detection of changed source content,
  relocation after a file or heading moves, or reconstruction of a selected
  sentence. Deleting the machine files clears those marks, and git can take
  back what was never committed; the ledger restores this computer's own, but it
  is on the same computer, so a machine failure may lose them. Its independence
  from `progress.json` means damage to one does not cost the other. The files an
  earlier version wrote, `marks.json` included, are read and never written.
- Repeated `exists` queries enumerate which names, including vault paths or
  path suffixes, answer from withheld directories, one caller-supplied name at
  a time. Accepted, because every
  alternative answer manufactures a concrete harm: a duplicate note under a
  private name, or links written against a name the page renders ambiguous.

## Response

Koopa owns severity. Treat suspected private-content egress, a write-boundary
bypass, or non-loopback exposure as high, and keep the report private under
`.github/SECURITY.md` until a regression lock and a patch exist. Where the listener
was exposed, stop the process and treat every reachable vault response as
disclosed: no access log is complete enough to prove otherwise. Any new network
egress of any kind is a new decision, not a configuration flag.

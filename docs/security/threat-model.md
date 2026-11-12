# Threat and trust model

A design and test-selection input, not proof of a deployment. The boundary is a
single-user local process with an unauthenticated HTTP listener hard-coded to
`127.0.0.1`; exposing it through a proxy, tunnel, container port, or
non-loopback bind is unsupported. The four walls hold: one written field,
loopback only, one schema source, report and never repair. The reader's own
marks are kept in two files outside the vault, which leaves the first of those
literally true of the vault directory.

## What is protected

| Asset | Required property |
|---|---|
| Vault files, contract-private paths included | Confidential outside the local reader; never silently repaired. |
| `System/schemas/vault-schema.toml` | Sole machine authority for lifecycle, instance, artifact and privacy capability; missing, invalid or stale authority fails closed. |
| The status write | Exactly one legal `status` line changes, the source is not stale, and the replacement is durable before the success response. |
| The reader's own marks | Kept outside the vault and replaced whole or not at all. No command-line face reads the continuation; `left-open` reads the uncertainty marks without writing them and answers under the same privacy declaration as the other agent-facing commands. Continuation and uncertainty marks occupy independent files. Unreadable uncertainty storage is preserved. Marks have no backup or recovery guarantee and are not held to the status write's durability. |
| Agent-facing results | Contract-private paths neither appear in nor influence results, with one exception: `exists` answers whether a caller-supplied name, including a vault path or path suffix, is taken, disclosing that bit and nothing else. |
| Browser authority | Authored vault bytes stay display input, never first-party script, navigation, form, frame, or automatic remote-resource authority. A figure runs in an opaque-origin frame: unapproved, it runs no script and the browser fetches nothing it names on its own; approved, it runs exactly its approved inline scripts and still holds no first-party authority. |
| Figure approvals | Kept outside the vault, owner-only, replaced whole; written only by the approval commands at a terminal, never over HTTP. Unreadable or malformed approvals are no approval. An approval names exact bytes and lapses on any change. |

## Who is trusted

Koopa is the sole user and security owner: no account system, session, role, or
token exists. The local browser and any process that can open the loopback port
are equally unauthenticated — loopback is not authentication, and browser
cross-origin protection is not a local-process access control. The vault author
supplies untrusted bytes; being in the vault grants no browser or write
authority. A same-UID process sits inside the host trust base, and owner-only
modes and identity rechecks prevent accidents and many races, not a malicious
process that can already read the vault. OS, browser and filesystem are trusted.

## Where the boundaries are enforced

| Boundary | Enforcement |
|---|---|
| Network | `cmd/yomihon` creates only `127.0.0.1:<port>`. Requests are bounded in header size and read/write/idle time; final responses carry same-origin CORP, a nonce-bound CSP, `nosniff`, `no-referrer`, and DNS-prefetch refusal. |
| Rendered vault data | Only bare `ruby`, `rt`, `rp`, `br`, `kbd`, `sub`, `sup`, `mark` and `u` are admitted as authored HTML; only `ruby`, `rt` and `rp` may carry `lang=`. Other tags or attributes remain escaped text. Raw markup uses scriptless sandbox or escaped-source boundaries, and is served as a download rather than opened as a page, because a link element can ask the browser for a page outside the sandbox's reach; remote Markdown images become user-activated links. A figure is the one markup document opened as a page; see the figures section below. |
| Process to vault | `vault.Reader` and `os.Root` pin the selected root. Paths are vault-relative and normalized before privileged use; the write path refuses symlinked traversal and rechecks file and parent identity. |
| Contract to privileged action | `internal/schema` derives capability from the exact contract source. Agent output and status writes both fail closed without valid authority. |
| Status mutation | `internal/status` alone writes. `POST /status` is capped at 4 KiB; a captured note without owner-write permission is refused before any temporary file or replacement, even in a writable directory. Otherwise it writes a synchronized sibling temporary file, revalidates, renames atomically, then synchronizes the directory. macOS and Linux only. |
| Marking a continuation place | `internal/mark` writes `reader.json` under the platform's configuration directory, never into the vault. `POST /marks` is capped at 4 KiB and holds each field to a fixed shape. The path must be vault-relative, in NFC, valid UTF-8 and free of control characters, and a place is accepted only for a path that is a readable note in the current snapshot. The anchor must be valid UTF-8 and id-shaped, at most 256 bytes. The offset is an integer within one document and the identity a 64-digit lowercase hex digest. Only the note is looked up; the anchor is held to its shape, since it reaches the desk only inside an address and never as text. Ordinary notes fit this shape, but the list of those that do not is not closed. A reading page still offers the control, and pressing it answers 422, when the note's own file name holds a control character (a tab or a line break, which Linux and macOS allow), or when the nearest anchor above the reader is a block address holding a character an id cannot (such as `/`, `?`, `#`, a quote, `<` or `>`) or a heading id longer than 256 bytes. The refusal of such a file name is deliberate, so that a path which is not one line of text is never stored; the refusal of such an anchor is how the route already behaved. The file is written to a sibling temporary name and renamed over. It is deliberately not synchronized to durable storage. Like the status write, it is the reader's — an agent never calls it. |
| Marking uncertainty | `internal/mark` writes the separate `uncertainty.json`. `POST /uncertainties` is capped at 4 KiB; path and anchor must be valid UTF-8 and shaped like a vault path and a document id, and the server assigns the time. A new mark is accepted only for a path that is a readable note in the current snapshot and an anchor that note renders (an empty anchor names the note as a whole), and only while fewer than 500 marks are stored; clearing a mark is always allowed. Only those three fields are stored. A toggle reads the existing array first, refuses unreadable or malformed storage, then installs a complete array by sibling-file rename. Writes are serialized within the serving process and are not synchronized to durable storage. `GET /uncertainties` exposes location records to the local reading client and reports unreadable storage as failure, not an empty list. An agent never calls the write route. |
| External thought-note handoff | `internal/schema` reads optional `[navigation].answer_type` from the vault contract and accepts only an existing type enum member. The role is read once when the server starts, like `path_types` and `map_types`; withdrawing the declaration takes effect at the next start. The page offers derived frontmatter and a `based_on` link for copying or an explicit external-editor action. The reader's editor creates and saves the note; this does not grant the server another vault write face. No private contract is installed or amended automatically. |

The owner-write check honors captured filesystem intent, not effective ACL,
group or other write access. Readable read-only notes remain readable; changing
permissions belongs outside yomihon. The check does not protect against a
malicious same-UID process; later mode changes retain the existing concurrent
write refusal.

`YOMIHON_PORT` is the only environment value this program names, held to a
mechanically tested allowlist; the bind host is not configurable, and the vault
root comes from an argument or the working directory. One more value reaches it
without being named: `os.UserConfigDir` reads `HOME`, and `XDG_CONFIG_HOME`
where the platform has one, inside the standard library, so the allowlist
cannot see the key. That call decides where the reader's marks are kept, and
the same test refuses it anywhere but the one file in `cmd/yomihon` that
resolves it at startup — a guard that stayed green over a widened surface would
be worse than none. Search queries are capped at 4,096
bytes and reject control characters. Go dependencies are pinned by
`go.mod`/`go.sum`; redistributed assets carry their LICENSE files inside `assets/` (fonts/LICENSE.txt, js/mermaid/LICENSE).

`[privacy] never_egress_dirs` binds the agent-facing output contract — `check`,
`coverage`, `exists`, `left-open` — and not the reading room. Measured on `examples/vault`'s
`Diary`: the CLI withholds even the name (`{"matches":[],"withheld":true}`),
while over HTTP the note's body reaches `/raw`, `/notes` and `/preview` and its
name reaches six doors; only Home, `/paths` and `/reports` are clean. The
declaration therefore governs what yomihon says to a program, not what a
process on this machine can read; loopback cannot tell a person's browser from
an agent's `curl`, and a local agent reads the files directly anyway. Binding
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
(`..//evil.com`, a backslash, `///evil.com`) produced a redirect off it.

A note shown inside another page — a hover card, a course's opening — draws
every figure as its stand-in, with no frame: an approval runs a figure where
its note is read, never on a page whose reader only passed over a link to it. The browser
applies the narrower of the frame attribute and the response policy, so a page
drawn before a file changed can only err towards running less.

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
and port over WebRTC, and, with an endpoint of its own, read replies. The TCP
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
script in full from that read — with control and text-direction characters
shown as escapes, so the terminal draws the code that runs — and pins the
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
- **An anonymous write face.** `POST /status` takes a transition from anyone,
  on the one field yomihon writes. The guards are the ones above: an illegal
  transition, a stale `from`, or bytes changed since the page was read are
  refused, and `published` is refused outright.
- **An hourly restore from git as the compensating control**, with the
  consequence that a visitor sees a status another visitor moved until that
  restore runs.
- **Shared reader marks, bounded, wherever those routes are exposed.** There
  is no per-visitor mark store: continuation and uncertainty marks belong to
  the serving machine and vault, and restoring the sample vault from git does
  not reset either configuration file outside it. What a visitor can add to the
  two files is bounded differently.
  The uncertainty file is bounded by three guards: the place must exist in the
  served vault (a readable note and an anchor it renders, so no visitor text is
  stored there), the file holds at most 500 marks, and path and anchor must be
  valid UTF-8 (so a request cannot make the file unreadable).
  The continuation file holds one value that replaces itself, so it cannot grow.
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

The mutable exposure is the sample notes' state and the bounded shared reader marks;
the note content is already public. A second instance, or one over any vault
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
- No Windows status publication, encryption at rest, secure deletion, or backup
  and restore.
- No recovery of lost uncertainty marks, detection of changed source content,
  relocation after a file or heading moves, or reconstruction of a selected
  sentence. Deleting `uncertainty.json` clears those marks; a machine failure
  may lose them. Its independence from `reader.json` prevents an older
  continuation writer from replacing it, without creating a migration or
  recovery promise.
- Repeated `exists` queries enumerate which names, including vault paths or
  path suffixes, answer from withheld directories, one caller-supplied name at
  a time. Accepted, because every
  alternative answer manufactures a concrete harm: a duplicate note under a
  private name, or links written against a name the page renders ambiguous.

## Response

Koopa owns severity. Treat suspected private-content egress, a write-boundary
bypass, or non-loopback exposure as high, and keep the report private under
`.github/SECURITY.md` until a regression lock and a patch exist. A status write
that failed before the rename left the note unchanged; where durability is
uncertain or both versions are on disk, do not resubmit — inspect the note and
any sibling the error names, then finish or revert by hand. Where the listener
was exposed, stop the process and treat every reachable vault response as
disclosed: no access log is complete enough to prove otherwise. Any new network
egress of any kind is a new decision, not a configuration flag.

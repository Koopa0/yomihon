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
| The reader's own marks | Kept outside the vault, replaced whole or not at all, and read by no command-line face. Continuation and uncertainty marks occupy independent files. Unreadable uncertainty storage is preserved. Marks have no backup or recovery guarantee and are not held to the status write's durability. |
| Agent-facing results | Contract-private paths neither appear in nor influence results, with one exception: `exists` answers whether a caller-supplied exact name is taken, disclosing that bit and nothing else. |
| Browser authority | Authored vault bytes stay display input, never first-party script, navigation, form, frame, or automatic remote-resource authority. |

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
| Rendered vault data | Only bare `ruby`, `rt`, `rp`, `br`, `kbd`, `sub`, `sup`, `mark` and `u` are admitted as authored HTML; only `ruby`, `rt` and `rp` may carry `lang=`. Other tags or attributes remain escaped text. Reports and raw markup use scriptless sandbox or escaped-source boundaries; every `<link` opener in a report, including one that is only text, is served with a relation no browser acts on ahead of the author's, a report marked as UTF-16 is served as plain text, and raw markup is served as a download rather than opened as a page, because a link element can ask the browser for a page outside the sandbox's reach; remote Markdown images become user-activated links. |
| Process to vault | `vault.Reader` and `os.Root` pin the selected root. Paths are vault-relative and normalized before privileged use; the write path refuses symlinked traversal and rechecks file and parent identity. |
| Contract to privileged action | `internal/schema` derives capability from the exact contract source. Agent output and status writes both fail closed without valid authority. |
| Status mutation | `internal/status` alone writes. `POST /status` is capped at 4 KiB; it writes a synchronized sibling temporary file, revalidates, renames atomically, then synchronizes the directory. macOS and Linux only. |
| Marking a continuation place | `internal/mark` writes `reader.json` under the platform's configuration directory, never into the vault. `POST /marks` is capped at 4 KiB and holds each field to a fixed shape. The path must be vault-relative, in NFC, valid UTF-8 and free of control characters, and a place is accepted only for a path that is a readable note in the current snapshot. The anchor must be valid UTF-8 and id-shaped, at most 256 bytes. The offset is an integer within one document and the identity a 64-digit lowercase hex digest. Only the note is looked up; the anchor is held to its shape, since it reaches the desk only inside an address and never as text. Ordinary notes fit this shape, but the list of those that do not is not closed. A reading page still offers the control, and pressing it answers 422, when the note's own file name holds a control character (a tab or a line break, which Linux and macOS allow), or when the nearest anchor above the reader is a block address holding a character an id cannot (such as `/`, `?`, `#`, a quote, `<` or `>`) or a heading id longer than 256 bytes. The refusal of such a file name is deliberate, so that a path which is not one line of text is never stored; the refusal of such an anchor is how the route already behaved. The file is written to a sibling temporary name and renamed over. It is deliberately not synchronized to durable storage. Like the status write, it is the reader's — an agent never calls it. |
| Marking uncertainty | `internal/mark` writes the separate `uncertainty.json`. `POST /uncertainties` is capped at 4 KiB; path and anchor must be valid UTF-8 and shaped like a vault path and a document id, and the server assigns the time. A new mark is accepted only for a path that is a readable note in the current snapshot and an anchor that note renders (an empty anchor names the note as a whole), and only while fewer than 500 marks are stored; clearing a mark is always allowed. Only those three fields are stored. A toggle reads the existing array first, refuses unreadable or malformed storage, then installs a complete array by sibling-file rename. Writes are serialized within the serving process and are not synchronized to durable storage. `GET /uncertainties` exposes location records to the local reading client and reports unreadable storage as failure, not an empty list. An agent never calls the write route. |
| External thought-note handoff | `internal/schema` reads optional `[navigation].answer_type` from the vault contract and accepts only an existing type enum member. The role is read once when the server starts, like `path_types` and `map_types`; withdrawing the declaration takes effect at the next start. The page offers derived frontmatter and a `based_on` link for copying or an explicit external-editor action. The reader's editor creates and saves the note; this does not grant the server another vault write face. No private contract is installed or amended automatically. |

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
`coverage`, `exists` — and not the reading room. Measured on `examples/vault`'s
`Diary`: the CLI withholds even the name (`{"matches":[],"withheld":true}`),
while over HTTP the note's body reaches `/raw`, `/notes` and `/preview` and its
name reaches six doors; only Home, `/paths` and `/reports` are clean. The
declaration therefore governs what yomihon says to a program, not what a
process on this machine can read; loopback cannot tell a person's browser from
an agent's `curl`, and a local agent reads the files directly anyway. Binding
the reading room too would be a separate decision (2026-09-04, #149).

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
- Repeated `exists` queries enumerate which exact names answer from withheld
  directories, one caller-supplied name at a time. Accepted, because every
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

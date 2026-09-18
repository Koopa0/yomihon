# Threat and trust model

A design and test-selection input, not proof of a deployment. The boundary is a
single-user local process with an unauthenticated HTTP listener hard-coded to
`127.0.0.1`; exposing it through a proxy, tunnel, container port, or
non-loopback bind is unsupported. The four walls hold: one written field,
loopback only, one schema source, report and never repair. The reader's own
marks are kept in one file outside the vault, which leaves the first of those
literally true of the vault directory.

## What is protected

| Asset | Required property |
|---|---|
| Vault files, contract-private paths included | Confidential outside the local reader; never silently repaired. |
| `System/schemas/vault-schema.toml` | Sole machine authority for lifecycle, instance, artifact and privacy capability; missing, invalid or stale authority fails closed. |
| The status write | Exactly one legal `status` line changes, the source is not stale, and the replacement is durable before the success response. |
| The reader's own marks | Kept outside the vault, replaced whole or not at all, and read by no command-line face. Losing one costs a press of the control, so it is not held to the status write's durability. |
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
| Rendered vault data | Only the inert authored-HTML subset is admitted. Reports and raw markup use scriptless sandbox or escaped-source boundaries; remote Markdown images become user-activated links. |
| Process to vault | `vault.Reader` and `os.Root` pin the selected root. Paths are vault-relative and normalized before privileged use; the write path refuses symlinked traversal and rechecks file and parent identity. |
| Contract to privileged action | `internal/schema` derives capability from the exact contract source. Agent output and status writes both fail closed without valid authority. |
| Status mutation | `internal/status` alone writes. `POST /status` is capped at 4 KiB; it writes a synchronized sibling temporary file, revalidates, renames atomically, then synchronizes the directory. macOS and Linux only. |
| Marking a reading place | `internal/mark` alone writes, to one file under the platform's configuration directory and never into the vault. `POST /marks` is capped at 4 KiB and refuses any path, anchor, offset or identity outside the shape a reading page stamps; the file is written to a sibling temporary name and renamed over. It is deliberately not synchronized to durable storage: what a crash costs is one place a reader keeps again. Like the status write, it is the reader's — an agent never calls it. |

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

The exposure is therefore the sample notes' state, not their content. A second
instance, or one over any vault that is not `examples/vault`, is a new decision.

## What is not defended

- No rate limit, connection quota, or handler concurrency limit, and no
  whole-vault ceiling: repeated requests consume CPU and descriptors, and a
  pathological vault can exhaust scan, render or index resources.
- No authentication of a direct local client, and no isolation from a malicious
  same-UID process. A machine with untrusted local users is outside the model.
- No Windows status publication, encryption at rest, secure deletion, or backup
  and restore.
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

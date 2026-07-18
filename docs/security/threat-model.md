# Threat and trust model

This document describes the current supported yomihon implementation. It is a
design and test-selection input, not proof of a particular deployment. The
product boundary is a local, single-user process with an unauthenticated HTTP
listener hard-coded to `127.0.0.1`. Exposing that listener through a proxy,
tunnel, container port, or non-loopback bind is unsupported; see
[`SECURITY.md`](../../SECURITY.md).

The four product walls in [`CLAUDE.md`](../../CLAUDE.md) remain authoritative:
yomihon writes only the `status` field, serves only on loopback, derives schema
and privacy authority from the vault contract, and reports rather than repairs
bad notes. D18, D32, D47, D50, D57, D58, and D59 in
[`docs/decisions.md`](../decisions.md) own the detailed rulings.

## Security objectives and protected assets

| Asset | Required property |
|---|---|
| Vault files, including contract-declared private paths | Confidential outside the supported local reader and explicitly authorized egress; never silently repaired or broadly uploaded. |
| `System/schemas/vault-schema.toml` | Sole machine authority for lifecycle, instance, artifact, and privacy capabilities; missing, invalid, or source-stale authority fails closed at privileged boundaries. |
| Status write and vault git history | Exactly one legal `status` line changes; the source is not stale or dirty; publication is durable before one path-scoped git commit and success response. |
| Agent-facing results | Contract-private paths neither appear nor influence results. Raw semantic query text is not copied into diagnostics, logs, caches, errors, metrics, or traces. |
| `YOMIHON_EMBED_KEY` | Read only for an explicit provider action, sent only in the provider request header, and never persisted by yomihon. |
| Semantic generations | Plaintext, owner-only, disposable derived state outside the vault; a query uses only one complete, compatible, current generation. |
| Browser authority | Authored vault bytes remain display input, not first-party script, navigation, form, frame, or automatic remote-resource authority. |
| Host resources and provider quota | Local requests and explicit builds must not gain unbounded network retries or silently spend provider quota. |

## Actors and identities

| Actor | Current identity and trust |
|---|---|
| Koopa, the local operator | The sole product user and security owner. Status authorization uses the fixed contract actor `koopa`; this is product identity, not cryptographic login. |
| Local browser | A supported reader on the same machine. It reaches a plain-HTTP loopback origin and receives vault-derived responses. |
| Local CLI or agent pipeline | May invoke agent-facing judge/search commands and consume stdout. Lexical search and judge commands are local reads; explicit semantic actions may update the disposable cache and call the provider. Output remains privacy-authorized, but downstream handling after stdout is outside yomihon. |
| Cross-origin web page or direct local client | Untrusted. Browser cross-origin write/read mechanisms are constrained, but a process that can directly connect to the loopback port meets no authentication challenge. |
| Vault author or synchronizer | Supplies potentially malformed or adversarial paths, Markdown, frontmatter, reports, attachments, and contract bytes. Being in the vault does not grant browser or write authority. |
| Same-UID process | Inside the current host trust base. Owner-only modes, rooted descriptors, and identity checks prevent accidents and many races; they do not claim isolation from a malicious process that can already read the vault, environment, and cache. |
| Google Gemini API operator | External service operator and recipient for the three narrowly authorized provider flows. Provider account policy, retention, deletion, availability, billing, and abuse controls are not owned by yomihon. |
| OS, browser, DNS resolver, TLS roots, filesystem, SQLite, and Git | Trusted platform dependencies. The selected `git` executable, vault Git configuration, hooks, and filters are not sandboxed. |

There is no account system, session identity, RBAC, bearer token, shared hosted
credential, or multi-user authorization model.

## Trust boundaries and capabilities

| Boundary | Capability owner and current enforcement |
|---|---|
| Network to reading server | `cmd/yomihon` creates only `127.0.0.1:<port>`. The server bounds request headers and read/write/idle time, applies Go cross-origin protection, and stamps final responses with same-origin CORP, a nonce-bound CSP, `nosniff`, `no-referrer`, and DNS-prefetch refusal. Loopback is still not authentication. |
| Browser to rendered vault data | The renderer admits only the inert authored-HTML subset ruled by D59. Reports and raw markup use scriptless sandbox/escaped-source boundaries. Remote Markdown images become user-activated links, not automatic loads. |
| Process to vault | `vault.Reader` and `os.Root` pin a selected root. Paths are vault-relative and normalized before privileged use; status rejects symlinked directory traversal and rechecks file and parent identity. |
| Contract to privileged action | `internal/schema` derives positive artifact/privacy capabilities from the exact contract source. Agent output and semantic egress fail closed without valid privacy authority; status writes fail closed without valid lifecycle/artifact authority. |
| HTTP request to status mutation | `internal/status` alone owns vault writes and Git. `POST /status` is limited to 4 KiB; path, prior status, target status, actor ownership, clean Git state, source bytes, and current artifact authority are checked before publication. |
| Status publication | `internal/status` writes a sibling temporary file, synchronizes it, revalidates authority and the original file, atomically renames, synchronizes the containing directory, then runs path-scoped `git add` and `git commit`. macOS and Linux are the only supported write platforms. |
| CLI to semantic provider | `cmd/yomihon` gives semantic actions a vault reader, contract capabilities, and a lazy credential callback. `serve` receives no provider, key reader, generation store, or semantic query capability. |
| Note chunk to final HTTP send | `internal/search/semantic.geminiEmbedder` is the production send owner. Immediately before each document send it re-reads the source through the pinned reader and revalidates instance status, privacy allowance, note hash, chunk ordinal, submitted hash, and contract-source freshness. |
| Query to final HTTP send | A successful reconcile returns a single-use query capability. It revalidates privacy/artifact source freshness and the complete corpus immediately before its one query request. |
| Process to semantic store | The store is outside the vault under the OS user cache. On macOS/Linux, the semantic directory is `0700`; database, SQLite sidecars, and the sibling writer lease are `0600`. Rooted descriptors, file identity checks, one writer lease, transactions, schema fingerprints, and complete manifests fail closed on drift or corruption. This is not encryption or hostile-same-UID isolation. |
| Status process to Git | A private same-binary child enters the already-open vault root and replaces itself with `git`; arguments are discrete and path-scoped. At the final external-process boundary, every `GIT_*` and `YOMIHON_*` variable is stripped. Unrelated ambient variables retain normal Git inheritance. Git executable/configuration/hooks remain trusted local dependencies. |

## Accepted inputs and attacker-controlled values

| Input | Treatment and remaining risk |
|---|---|
| `YOMIHON_ROOT`, `YOMIHON_PORT`, `YOMIHON_EMBED_KEY` | These are the only product configuration values selected from the environment. Root must select a directory; the bind host cannot be configured. The key is process-ambient secret material even though application code reads it lazily; it is not isolation from the same UID. The status Git child enumerates the environment only to remove all `GIT_*` and `YOMIHON_*` entries before external execution. |
| CLI arguments | Root and search query are valid UTF-8, reject control characters, and are limited to 4,096 bytes; result limit is 1–1,000. Structured filters remain local and are removed from the provider query projection. |
| HTTP method, path, query, headers, and form | Routes are method-specific. Search query is limited to 4,096 bytes and rejects control characters; the status form is capped at 4 KiB. There is no general per-client rate limit or authenticated caller quota. |
| Vault paths and filesystem namespace | Relative path validation, NFC normalization where contract semantics require it, component-prefix matching, rooted opens, symlink refusal at write/cache boundaries, and identity rechecks mitigate traversal and replacement. A malicious same-UID process remains out of scope. |
| Vault Markdown, YAML, raw HTML, reports, attachments, and sidecars | Treated as untrusted display/parser input. Rendering is fault-tolerant; authored HTML and reports do not gain executable first-party authority. The source viewer caps highlighted text at 1 MiB, but there is no declared whole-vault file-count or byte-size ceiling. |
| Vault contract and Git state | Both are security inputs. Missing/invalid/stale capability source, illegal transition, dirty target, and concurrent change close the affected action. Git executable, hooks, filters, configuration, and repository integrity are operator-trusted. |
| Provider response/status/headers | Response body is capped at 8 MiB for embedding requests, parsed as one JSON value, and must contain one finite vector of the exact dimension. Error bodies and submitted text are not retained in local error values. |
| DNS answer and TLS peer | The hostname is fixed, but resolution uses the host resolver and TLS uses Go/platform trust roots and hostname verification. There is no IP allowlist, DNS pin, DNSSEC requirement, or application certificate pin. |
| Developer certification environment | D57 accepts opt-in, credential, output path, and ruled dimension controls, but never arbitrary text, vault root, or input file. Corpus/query bytes come from fixed synthetic repository fixtures. |

## Egress and disclosure inventory

| Flow | Trigger and data | Destination and final guard |
|---|---|---|
| Local reading and raw-file response | A loopback GET may return any vault file, including a private path, because local reading by Koopa is authorized. | The requesting loopback client. No authentication; browser-origin protections reduce confused-deputy access but do not authorize direct local clients. |
| Agent stdout (`search`, `check`, `coverage`, `exists`) | Explicit CLI action; privacy-authorized findings/results only. Search result snippets naturally contain matching eligible note text. | Local stdout and whatever operator-owned pipeline consumes it. Yomihon performs no later network send and cannot enforce downstream retention. |
| Document embedding | Explicit `search-index build`, or bounded reconciliation inside explicit `search --semantic`; exact submitted chunk bytes formatted as `title: ... | text: ...`. | `POST https://generativelanguage.googleapis.com/v1beta/models/gemini-embedding-2:embedContent`, after final source/instance/privacy/hash/freshness revalidation. |
| Query embedding | One text-bearing explicit `search --semantic` action after a current generation is admitted; fixed prefix plus bare query text, at most one send per process action. | The same fixed `embedContent` endpoint, after final authority and corpus revalidation. No query-echo field or raw-query diagnostic exists. |
| Fixed synthetic provider certification | Explicit developer-only D57 action; fixed synthetic protocol probes and fixed synthetic eval corpus/queries only. | The fixed `embedContent` endpoint; the protocol probe also calls `POST https://generativelanguage.googleapis.com/v1beta/models/gemini-embedding-2:countTokens`. These actions are test-only and absent from the product command graph. |
| Status commit | One successful status publication invokes local Git; commit message contains vault-relative path and `from`/`to`. | The local vault Git repository only. Yomihon never pushes. Local hooks/filters may have effects because Git is a trusted dependency. |
| Diagnostics | Server `slog` records startup vault path, capability/scan failures, and selected route/status paths; semantic CLI stderr uses fixed reason text or content-free build counts. | Process stderr. There is no application telemetry, metrics exporter, trace exporter, crash reporter, or remote log sink. Shell redirection/service capture is deployment-owned. |

Production provider transport is direct: `Proxy` is `nil`, redirects are
refused, the endpoint is not caller-configurable, and each client method makes
one HTTP send with a 30-second request timeout. Query and interactive document
requests do not retry. Only explicit full build may retry a 429, with a durable
maximum of five attempts per chunk and bounded waits. Certification rows do not
automatically retry. A proxy or a different host is a new egress decision.

## Durable and transient data locations

| Location | Data and lifetime |
|---|---|
| Operator vault and its `.git` directory | Source of truth. Yomihon changes one status line and creates one commit; it does not create a separate vault backup. |
| Adjacent `.yomihon-status-*.tmp` | A complete rewritten note prepared with exclusive creation before rename. It is removed best-effort on handled pre-publication failure; a process or host crash may leave it for manual inspection/removal. |
| OS user cache: `yomihon/semantic/generation.sqlite` | Plaintext paths, hashes, generation identity, vectors, retry ledger, and p95 measure. SQLite WAL/SHM/journal files and the sibling `yomihon/semantic.lock` are part of the same boundary. At most active, previous, and one staging generation remain; there is no TTL. |
| Process memory | Vault bytes, rendered/search projections, agent snapshot, semantic submitted chunks, query text, vectors, and immutable exact index. They last for the request/action, snapshot generation, or process lifetime; Go does not promise secure zeroization. |
| Stderr/stdout | Not persisted by yomihon. Terminal scrollback, shell history, pipes, service managers, and redirection may retain them outside the application. |
| Provider systems | Authorized request content, key header, provider metadata, and returned vectors cross into Google-operated systems. Their retention, backups, access logs, and deletion are external and not asserted here. |
| Developer-selected synthetic recording path | Owner-only local JSON containing only synthetic identifiers/hashes and vectors. A committed synthetic recording also remains in repository history. |

The detailed field-by-field handling is in
[`docs/privacy/data-inventory.md`](../privacy/data-inventory.md).

## Abuse and denial-of-service paths

- Any local process can make unauthenticated loopback reads and can attempt
  `POST /status`. Browser cross-origin protection is not a local-process access
  control. Running yomihon on a machine with untrusted local users is outside
  the current model.
- Repeated requests can consume CPU, file descriptors, and rendering work.
  Header and server timeouts exist, but there is no rate limiter, connection
  quota, or global handler concurrency limit.
- A very large or highly pathological vault can exhaust scan, parse, render,
  graph, or in-memory lexical-index resources. Individual search inputs and
  highlighted-source display are bounded, but the whole vault has no declared
  byte/count limit.
- Full semantic build can consume provider quota proportional to all missing
  chunks. It is explicit, single-writer, capacity-gated below 100,000 chunks and
  1 GiB raw vectors, and retry-bounded; it is not a billing quota system.
- Interactive reconciliation refuses more than 128 missing chunks or 100,000
  proxy tokens before document/query egress. Query sends once. These limits do
  not constrain an operator repeatedly launching new processes.
- Oversized, malformed, slow, or non-finite provider responses are bounded and
  rejected. DNS/TLS/provider availability remains an external dependency.
- Git hooks, filters, repository locks, slow filesystems, and a hostile Git
  executable can delay, fail, or add side effects to the status action. Such a
  Git environment is unsupported, not sandboxed by yomihon.

## Third parties and supply chain

- Google operates the Gemini endpoint and the operator-owned provider account.
  Yomihon bundles no key, shared account, or proxy. Terms, retention, deletion,
  quota, and billing must be reviewed by the operator at use time.
- The OS resolver, platform CA roots, browser, filesystem, Git, and SQLite
  runtime are trusted local/platform dependencies.
- Go dependencies are pinned by `go.mod`/`go.sum`; redistributed browser assets
  and fonts are listed in [`THIRD_PARTY_NOTICES.md`](../../THIRD_PARTY_NOTICES.md).
  The canonical verification command includes vulnerability and static-security
  checks, but a passing scan is not a deployment guarantee.
- No third-party analytics, telemetry, advertising, remote fonts, or crash
  reporting is implemented.

## Detection, response, and recovery

Runtime detection is local and content-minimized: typed CLI reasons and exit
codes, status recovery pages that state whether bytes changed, stderr logs,
store permission/schema/manifest checks, and the vault Git history. There is no
central audit service, automated alerting, telemetry, or remote incident
detection.

Koopa owns severity and response. Treat suspected vault/private-content egress,
credential disclosure, a write-boundary bypass, or non-loopback exposure as
high severity. Keep reports private under the process in `SECURITY.md` until a
regression lock and patch are ready. Current fixes target latest `main`; after
releases exist, affected users are notified through a GitHub security advisory
or release note as appropriate.

Recovery actions are boundary-specific:

- **Status failed before rename:** the note is unchanged; fix the named cause,
  reload, and start a new action.
- **Durability uncertain or Git commit failed after rename:** do not resubmit.
  Inspect the note, `git diff`, index, and repository state; then complete or
  revert the change manually. Yomihon intentionally performs no second write
  disguised as rollback.
- **Semantic store corrupt, unsafe, or disclosed:** stop semantic actions,
  remove the user-cache semantic directory if appropriate, and rebuild from the
  vault. The vault remains truth; there is no semantic-store restore promise.
- **Credential suspected exposed:** remove it from the environment, revoke or
  rotate it with the provider, inspect provider-account activity, and rebuild
  only if vector integrity is in doubt. Yomihon has no credential revocation API.
- **Listener accidentally exposed:** stop the process and tunnel/proxy, restore
  loopback-only operation, and treat every reachable vault response as a
  potential disclosure. There is no access log complete enough to prove nobody
  read it.
- **Private content reached a provider or repository:** stop further sends,
  preserve minimal evidence privately, rotate credentials if relevant, use the
  provider/repository incident path, and assess external copies. Local deletion
  cannot recall third-party backups or history.

## Unsupported or unverified deployment properties

The current source does **not** establish any of the following:

- safe non-loopback, proxied, tunneled, container-exposed, hosted, or multi-user
  operation;
- authentication of a direct local HTTP client or isolation from a malicious
  same-UID process;
- Windows status publication or semantic-store support;
- application-level encryption at rest, secure deletion, backup/restore, host
  firewall state, full-disk encryption, or OS account hardening;
- provider retention/deletion terms, provider-side no-training behavior,
  residency, legal basis, or availability;
- DNS pinning, certificate pinning, outbound firewall enforcement, or resistance
  to a compromised platform resolver/root store;
- whole-vault resource bounds, per-client rate limits, or production SLOs;
- a particular deployed revision, configuration, or live provider result.

Any widening of these properties or of the three external provider egress
classes requires a new product/security decision and matching final-boundary
evidence.

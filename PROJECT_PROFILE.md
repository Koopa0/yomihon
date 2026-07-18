# Project Engineering Profile

Status: Proposed normative profile; initial approval is pending  
Standard: `ENGINEERING_STANDARD.md` version 2.0  
Last reviewed: 2026-07-18; independent review not yet performed  
Profile owner: Koopa (`@Koopa0`)  
Independent approver: Pending designation for an immutable candidate

This file resolves repository-specific applicability. `APPLIES`, `N/A`,
`DEFERRED-BY-EXCEPTION`, `UNRESOLVED`, and `UNVERIFIED` have the meanings in
`ENGINEERING_STANDARD.md`. Nothing in this profile turns missing evidence into
approval. Section 19 records the present approval state.

## 1. Product identity

```text
Product: yomihon
Repository purpose: A local, single-user reading and adjudication interface for
  one contract-bearing personal Markdown vault, plus frozen agent-facing CLI
  diagnostics and search contracts.
Non-purpose: It is not a Markdown editor, generic Obsidian server, multi-user or
  remotely exposed service, hosted AI service, plugin platform, publication
  receipt system, or general agent write API.
Primary users: Koopa is the primary human user. Source-build users may use the
  read-only and BYOK surfaces within the documented project-specific limits.
Primary user tasks: Read every vault artifact; navigate and search; inspect
  diagnostics; advance one legal lifecycle status at the end of reading; run
  check, coverage, exists, lexical search, and explicitly requested semantic
  search from the CLI.
Primary operators: Koopa, and a source-build operator for their own local
  installation and provider account.
Deployment model: One local process, hard-bound to 127.0.0.1, reading one local
  vault. Future public v0.x distribution is source-only. There is no supported
  proxy, tunnel, widened listener, container-port exposure, hosted service, or
  shared credential.
Data value and loss impact: Vault files plus git are authoritative personal
  knowledge. A status action changes one authoritative frontmatter line and
  creates one git commit. Semantic SQLite generations are disposable derived
  data. A boundary failure can corrupt an authoritative note or disclose
  private vault content, query text, or a provider credential.
Security and privacy commitments: Loopback-only ingress; no authentication in
  the accepted same-account trust model; one status-field write face; the vault
  TOML is the sole machine schema authority; rendering never repairs source;
  contract-denied paths never influence or enter egress; only the three
  explicitly ruled provider-send classes are allowed.
```

## 2. Authority and ownership

| Area | Authority or decision source | Semantic owner | Operational owner |
|---|---|---|---|
| Product behavior | `docs/product.md`, `docs/spec.md`, then accepted rulings in `docs/decisions.md` | The feature package named in `docs/design.md`; unresolved behavior returns to Koopa | Koopa |
| Public API / CLI / UI | `docs/spec.md`, the owning face plan, D14/D28/D37/D50/D55, and frozen goldens | `internal/judge`, `internal/search/agent`, the relevant HTTP feature package, and `internal/ui` only for presentation | Koopa |
| Authentication / authorization | D17's local same-account boundary, wall 2, `http.CrossOriginProtection`, and lifecycle owner rules from the vault contract | `internal/origin`, `internal/status`, and `internal/schema` | Koopa |
| Durable data | The external vault contract, vault git history, D06/D07/D22/D32/D51/D58 | `internal/status` for the sole authoritative write; `internal/search/semantic` for disposable generations | Koopa |
| Network ingress | Wall 2, D17, D30, D45, D59, and `SECURITY.md` | `cmd/yomihon` and `internal/origin` | Koopa |
| Network egress | Wall 2 and D18/D32/D39/D42/D50/D54/D57/D59 | `internal/schema` supplies privacy capability; `internal/search/semantic` owns provider sends; `internal/judge` owns agent-output publication | Koopa |
| Privacy / telemetry | `[privacy]` in the external vault contract, `docs/privacy/data-inventory.md`, wall 2, D18/D39/D42/D50/D54/D57, and `SECURITY.md` | `internal/schema` for authority; the final emitting/sending package for enforcement | Koopa |
| Release / compatibility | `docs/release.md`, `docs/merge-policy.md`, D19, D37, D50, D55, and frozen contract fixtures | The package owning each public contract; release-wide compatibility is owned here and in `docs/release.md` | Koopa |
| Incident response | `SECURITY.md` and `docs/security/threat-model.md`; provider-account response remains the operator's responsibility | The affected final-boundary owner | Koopa; no separate incident team is named |

Accepted product decisions live at: `docs/decisions.md`  
Architecture entry point: `docs/design.md`  
Glossary: No dedicated glossary exists. Core terms are resolved by Sections 5
and 6 of this profile plus `docs/product.md`, `docs/spec.md`, and
`docs/vault-model.md`; a separate glossary is `UNRESOLVED` only if these sources
cease to be sufficient.  
Ownership file: `.github/CODEOWNERS` assigns repository and control-plane review
to `@Koopa0`. This section records semantic/operational ownership; independent
acceptance remains per-change under `docs/merge-policy.md` and cannot be supplied
by the builder.

## 3. Risk classification

Base class: `R3`

Rationale: Yomihon is deployed like an R1 local application, but the standard
classifies consequences rather than topology. It reads personal and explicitly
private vault data, may send eligible document and query bytes to a paid
provider, holds a credential at the final send boundary, emits agent-consumed
reports, and can replace an authoritative note before committing it to git.
A defect can therefore cause private-data exposure or authoritative data/audit
damage. That is R3 impact even though the repository is small, single-user, and
loopback-only.

| Capability flag | APPLIES / N/A / UNRESOLVED | Reason and owned boundary |
|---|---|---|
| public-api | APPLIES | Frozen agent CLI, JSON/JSONL bytes, field order, reason strings, and exit codes are public compatibility contracts. HTTP is a supported local UI surface, not a remote API. |
| durable-storage | APPLIES | Vault status plus git are authoritative writes; the semantic SQLite store is durable but disposable derived data. |
| network-ingress | APPLIES | The HTTP server accepts local browser/curl input on hard-coded `127.0.0.1`; remote ingress is unsupported. |
| network-egress | APPLIES | Only eligible document chunks, one explicit semantic query submission, and fixed synthetic certification inputs may reach the fixed embedding endpoint. |
| credentials | APPLIES | `YOMIHON_EMBED_KEY` is read lazily for an explicit provider action and must not enter storage, output, logs, errors, fixtures, or source. |
| personal-data | APPLIES | The vault, query text, semantic vectors, paths, and derived judgments can reveal personal knowledge. |
| irreversible-actions | APPLIES | Status writes are git-recoverable, but provider disclosure and quota/billing cannot be recalled. External publication is not implemented. |
| concurrent | APPLIES | `net/http`, the scanner goroutine, atomic snapshots, status TOCTOU protection, and the semantic writer lease are concurrent boundaries. |
| distributed | N/A | There is one local authority and no cluster, quorum, leader, shared control plane, remote writer, or distributed consistency promise. A provider call is an external dependency, not a distributed product topology. |
| plugin-platform | N/A | The product loads no third-party plugins and advertises no extension lifecycle or capability API. |
| browser-ui | APPLIES | Server-rendered HTML plus bounded progressive enhancement is a primary surface. |
| desktop-ui | N/A | No native desktop shell or windowing integration is supported. |
| agent-facing | APPLIES | `check`, `coverage`, `exists`, `search`, and `search-index` have machine consumers and frozen output contracts. |
| cross-platform | APPLIES | Read/judge/lexical surfaces target macOS, Linux, and Windows; status publication and the semantic store target macOS and Linux and must refuse before filesystem access elsewhere. |
| paid-provider | APPLIES | Semantic use is BYOK and quota-bearing; live certification is explicit and paid. |
| regulated-data | UNRESOLVED | Canon establishes private/personal data but neither authorizes nor prohibits a regulated-data use case. No GO or public support claim may include regulated data until Koopa rules its scope and obligations. |

No unresolved row may support a GO verdict.

## 4. Go and platform support

```text
Go directive: go 1.26.5
Minimum supported Go version: Go 1.26.5
CI Go versions: The single version selected by go.mod through actions/setup-go;
  no older-version matrix exists.
Primary OS/architecture: No single release target is privileged. Darwin/arm64
  is the current recorded local benchmark environment.
Supported OS/architecture matrix: OS support is macOS, Linux, and Windows for
  the reader, judge, and lexical search; macOS and Linux for status publication
  and semantic storage. The repository cross-build contract is the six 64-bit
  targets darwin/amd64, darwin/arm64, linux/amd64, linux/arm64, windows/amd64,
  and windows/arm64. Hosted CI additionally supplies real macOS and Windows
  runtime evidence, but no broader GOARCH promise is inferred.
Unsupported environments and behavior: A widened listener, proxy/tunnel
  exposure, hosted/multi-user use, and provider proxying are unsupported.
  Windows retains the supported read-only surfaces and rejects status and
  semantic-store writes before filesystem/key/provider access. Where another
  target happens to compile, privileged writes still refuse, but other GOOS or
  GOARCH combinations have no compile or runtime promise.
CGO policy: The product uses the pure-Go modernc SQLite driver and is CGO-free.
  The nested mattn driver is retained only as a Linux CI comparison and is not a
  product dependency.
Race-detector platform: Linux in the main verify gate and macOS in portable-core
  and darwin-semantic-contract. Windows compiles tests and runs focused platform
  contracts without -race.
Module / workspace policy: One root module plus the intentionally separate
  tools/sqlite-driver-bakeoff module with a checked ../.. replace. No go.work is
  used. Both module graphs must be tidy.
Dependency proxy / private module policy: No private modules are used. The repo
  has no explicit GOPROXY/GONOSUMDB/GOPRIVATE policy; reproducible proxy and
  checksum behavior beyond go.sum is UNRESOLVED for release evidence.
```

Compatibility promise:

- Public Go API: Before v1, no general package or exported-Go-API stability
  promise; most implementation is under `internal/`.
- CLI: Frozen agent-facing commands retain documented JSON/JSONL shapes,
  ordering where specified, exit codes, reason strings, stdout/stderr
  separation, and privacy behavior until an explicit ruling changes them.
- HTTP/RPC/wire: No remote API is supported. Local browser route semantics,
  status codes, PRG, CSP, and frozen agent wire bytes remain product contracts.
- Storage/schema: The vault's `vault-schema.toml` is external authority.
  Status rewriting changes one line only. Semantic generations are disposable,
  identity-bound, immutable when active/previous, and replaced explicitly;
  a future known-compatible container-schema bump must copy forward into a new
  file rather than mutate in place.
- Configuration: Only `YOMIHON_ROOT`, `YOMIHON_PORT`, and lazy
  `YOMIHON_EMBED_KEY` are supported. There is no bind-address or database-path
  setting and no legacy alias.
- Upgrade window: No public release exists. Future source-only v0.x support is
  the latest release line plus `main` for security fixes; semantic identity
  changes require an explicit replacement generation.
- Deprecation window: No general pre-v1 Go API window is promised. Frozen agent
  contracts require an explicit ruling, release-note/changelog disclosure, and
  migration treatment; a minimum time window is currently `UNRESOLVED`.

## 5. Core concept register

| Concept | Non-concept | Semantic owner | Source of truth | Projections / caches | Authority | Key invariant |
|---|---|---|---|---|---|---|
| Vault truth | The snapshot, index, report, or semantic database | The vault owner; readers translate only | Vault files plus vault git | Graph, nav, search, rendered HTML, diagnostics, semantic generations | `docs/product.md`, D06 | Every derived copy is disposable or explicitly classified; none silently becomes truth or write authority. |
| Vault contract | A Go enum copy, test fixture, or plan sentence | `internal/schema` | `System/schemas/vault-schema.toml` under the configured vault root | Immutable lifecycle, navigation, artifact, privacy, and supersession capabilities | Wall 3, `docs/vault-model.md`, D47/D53/D54 | Only `internal/schema` reads the TOML; invalid or stale authority degrades/fails closed exactly at each ruled boundary. |
| Governed instance | Any readable file or any note with frontmatter | `internal/schema` and consuming feature | Contract artifact policy plus the captured vault path | Navigation rows, lifecycle counts, metadata search, status UI | D47 | Non-instances remain readable but gain no lifecycle, metadata, lesson, placement, or write identity. |
| Lifecycle transition | Content editing, a no-op, or proof of external publication | `internal/schema` defines legality; `internal/status` executes | Contract lifecycle rows plus current file bytes and actor `koopa` | Forms, badges, advanceable counts, one git commit | D07/D13/D16/D51/D52/D58 | One legal state change rewrites exactly one status line and produces one matching git commit; unsupported/uncertain authority never writes. |
| `published` selection | An external deployment receipt or live-state claim | Vault lifecycle authority | The current status value | Future publisher input only | D51 | Yomihon never special-cases the word or claims deployment success from it. |
| Reading snapshot | A second filesystem truth or durable cache | `internal/snapshot` | One descriptor-rooted captured generation plus captured contract capabilities | Opaque read-only graph/nav/search/view data | D21/D25/D56 | One request captures once; no response combines projections from different generations. |
| Rendering projection | An editor, repairer, or arbitrary authored browser authority | `internal/render`; response boundary in `internal/origin` | Captured source bytes | Escaped/inert HTML, searchable text, sections, diagnostics | Wall 4, D45/D59 | Source is never rewritten; only the ruled inert HTML dialect survives and CSP remains defense in depth. |
| Judge finding | A repair, status transition, or hidden second policy | `internal/judge` | Captured public-eligible vault evidence plus schema rules | Frozen JSONL, human, and Markdown reports | D14/D39/D42/D53/D54/D55 | Private paths neither appear nor influence public verdicts; unavailable privacy authority emits no payload. |
| Lexical search | Semantic inference or a persistent source of truth | `internal/search` | Captured readable text/path and valid metadata capability where needed | In-memory deterministic index | D21/D23/D24/D25 | Lexical search remains local and available independently of provider, key, or semantic store. |
| Semantic generation | Vault truth, a server dependency, or a cross-identity fallback | `internal/search/semantic` | Eligible captured chunks plus one complete numerical/protocol identity | Owner-only SQLite active/previous/staging generations and per-command RAM exact index | D22/D32/D50 | Only a complete compatible active generation ranks; staging is never visible and previous is never an automatic fallback. |
| Egress capability | A path literal, caller assertion, global client, or generic retry policy | `internal/schema` supplies privacy authority; final sender owns revalidation | Contract privacy/artifact bytes plus an explicit action | Action-scoped provider client or agent output | Wall 2, D18/D32/D50/D54/D57 | Authorization is rechecked adjacent to the final send; no alternate provider/output path broadens content, destination, retry, redirect, proxy, or logging. |
| Agent wire contract | Incidental JSON encoding or prose to scrape | Owning CLI feature package | Canonical plan/ruling and committed golden fixtures | stdout/stderr bytes consumed by agents and crons | D14/D37/D50/D55 | Outcome, completeness, retry safety, and next action remain machine-distinguishable without echoing private input. |

## 6. State and error contract

There is intentionally no single repository-wide error envelope. Three public
surfaces own different, frozen contracts:

```text
Judge CLI:
  Stable code: exit 0 clean/success; exit 1 gate hit or exists no-match;
    exit 2 usage/tool failure; coverage succeeds with 0.
  Human message: stderr for usage/tool failure; fixed privacy-unavailable line.
  Fault owner: command grammar, local vault/configuration, or yomihon tool.
  Retry safety: no writes; correct the named input/authority and rerun.
  Partial-result validity: tool/privacy failure emits zero stdout; no partial
    clean result.
  Recovery / next action: frozen diagnostic or command help.
  Correlation ID: N/A; local synchronous command.
  Sensitive-data policy: no private path/content in tool errors; privacy is
    rechecked immediately before output.

Semantic agent CLI:
  Stable code: exit 0 answer; exit 3 semantic unavailable or request
    unanswerable; exit 2 usage; exit 1 confirmed yomihon/internal fault.
  Human message: frozen stderr reason; JSON uses answer, error, or
    internal_error discriminated envelopes and never echoes the query.
  Fault owner: configuration, local capability/store, provider, caller, or
    confirmed yomihon request formation.
  Retry safety: query sends at most once; interactive reconciliation is
    single-attempt; explicit full build alone has bounded persisted retries.
  Partial-result validity: an answer envelope may preserve truthful lexical
    results with exit 3, but never labels partial/stale semantic ranking as
    complete. Capability failure can suppress the entire payload.
  Recovery / next action: reason directs build, configuration, retry-later, or
    local repair.
  Correlation ID: N/A; local synchronous command.
  Sensitive-data policy: no raw query, credential, provider body, denied path,
    or submitted document in logs/errors/metrics/traces.

HTTP UI:
  Stable code: ordinary HTTP statuses; status success is 303 PRG; failures use
    the table in docs/spec.md section 4.
  Human message: Traditional Chinese recovery page with allowlisted English
    technical detail only where canon requires it.
  Fault owner: form/caller, local authority/configuration, concurrent editor,
    filesystem/git, or internal failure.
  Retry safety: recovery states say whether bytes changed; post-write failures
    explicitly say not to resubmit.
  Partial-result validity: reading fails open with diagnostics; writing fails
    closed. Partial HTML is discarded before a truthful plain 500 fallback.
  Recovery / next action: safe GET links only; no POST retry control.
  Correlation ID: N/A; loopback single-user surface.
  Sensitive-data policy: generic/internal and publication-uncertain failures
    expose no wrapped detail; response is no-store.
```

Relevant states and why they are distinct:

| State | Meaning | Result shape | Status / exit | Retry | Side effects | Recovery |
|---|---|---|---|---|---|---|
| off | Semantic retrieval was not requested; lexical behavior is complete | Lexical answer | exit 0 | Safe | No provider/store action | None |
| not-applicable | `--semantic` has no bare text to embed, such as a pure-filter query | Lexical answer with semantic not-applicable | exit 0 | Safe | Zero key/store/provider access | Refine only if semantic text was intended |
| empty | A valid complete local query or judge run found zero matching results/findings | Valid empty result, not an error | exit 0 unless `exists` no-match uses 1 | Safe | None | Change query or input; do not diagnose provider failure |
| unavailable-with-lexical | Semantic was applicable and requested, but platform, generation, key, provider, capacity, drift, or writer state prevented a complete hybrid answer | Answer envelope with valid lexical results and semantic unavailable reason | exit 3 | Reason-dependent; no automatic query retry | At most the ruled document work; query at most once; failed activation leaves old active generation intact | Follow typed reason, often explicit build/configuration/retry-later |
| unanswerable | Required privacy or artifact authority is missing/invalid, so no honest result payload can be formed | Error envelope or zero judge stdout | semantic exit 3; judge exit 2 | After authority repair/restart | No protected output; no provider send | Repair contract and restart when source-drift latch requires it |
| invalid | Command grammar, form, path, transition, or owner rule is illegal | Usage/tool diagnostic or status recovery page | CLI exit 2; HTTP 400/422 | Correct first | No authoritative write/provider send | Use help or correct the named field/path/transition |
| conflict | Another writer/editor or dirty worktree prevents safe ownership | Typed unavailable or recovery page | semantic exit 3; HTTP 409 | Safe after conflict clears and state is reread | No status write; old semantic active generation remains | Reload/reconcile/commit external edits, then retry as a new action |
| stale | Page, contract authority, or semantic corpus identity no longer matches current authority | Status stale/closed recovery or semantic reconciliation outcome | HTTP 409/503; semantic exit 3 on failed reconciliation | Only after reread/restart/build as instructed | No status write on stale page; no stale vector is ranked | Reload page; restart for latched contract change; explicit build when required |
| not-found | A local route/file or `exists` identity is absent | Honest 404 or no-match result | HTTP 404; `exists` exit 1 | Safe if source may appear | None | Correct identity or create through the owning external tool |
| provider-failed | A known or unknown provider fault prevented semantic completion | Lexical answer retained; sanitized provider classification | exit 3 | Query is not retried in place; full build follows bounded schedule only for eligible retry | At most one query request; no provider body propagated | Retry a new explicit action when appropriate or use lexical result |
| durable-publication-uncertain | Status bytes may be replaced but durability could not be confirmed | Same-shell recovery says bytes changed and do not resend | HTTP 500 | Unsafe until inspected | File may have changed; no success claim | Inspect file and git manually; finish audit trail without resubmitting |
| commit-failed-after-write | Status bytes changed but git commit failed | Same-shell recovery with allowlisted git detail and do-not-resend | HTTP 500 | Unsafe until inspected | File changed, commit missing | Commit/repair manually, then reload |
| internal-error | Yomihon violated an invariant or could not safely form an operation | Internal-error envelope, generic HTTP 500, or process error | semantic exit 1; HTTP 500; serve exit 1 | Do not blind-retry | Depends on explicitly reported mutation truth | Preserve diagnostics without private data and repair invariant |
| cancelled / shutdown | Operator signal cancels command/server work | Process exit or command classification; server drains | Command-specific non-success; server returns error only if shutdown fails | New invocation after stop | Provider request may already have been sent; status mutation truth must remain explicit | Inspect outcome, then start a new action; never infer rollback from cancellation |

States declared N/A:

| Candidate state | Why it cannot occur |
|---|---|
| authenticated / unauthorized role | The supported deployment has no identity/authentication system. Loopback and same-account trust are explicit; lifecycle actor legality and browser same-origin protection are separate controls. |
| accepted-asynchronously / pending remote job | Public commands and HTTP status changes are synchronous. Semantic staging is internal publication state, not an accepted user request reported as success. |
| distributed partition / leader / quorum | No distributed topology or shared control plane exists. |
| external publication success | Yomihon does not publish externally; `status: published` is selection only. |

## 7. Dependency and package direction

Allowed high-level direction:

```text
cmd/yomihon process composition
        ↓
feature orchestration and HTTP/CLI entry points
        ↓
schema, vault, render, graph, search, judge, status, and snapshot capabilities
        ↓
feature-owned filesystem, git, SQLite, and provider implementations
        ↓
Go standard library and reviewed external dependencies
```

Repository-specific package rules:

```text
Command packages: cmd/yomihon owns environment parsing, command dispatch,
  dependency construction, signal handling, listener creation, and exit status
  only. Product policy stays in the feature owner.
Internal package boundary: Product implementation remains under internal/ and is
  organized by capability, not service/repository/model layers.
Public package policy: No public library package is promised before v1.
Generated packages: *_templ.go and internal/search/semantic/catalog are
  committed generated Go; assets/css/output.css is committed generated CSS.
  Edit their source/generator input and verify drift; never hand-edit outputs.
Forbidden dependency edges: Only internal/schema reads the vault TOML; only
  internal/status has authoritative vault-write/git capability; serve owns no
  semantic store/provider/key path; internal/ui does not read domain sources;
  core semantics do not import command or presentation policy.
Plugin / registration policy: N/A. There is no plugin registry or third-party
  component lifecycle; adding one requires an architecture decision.
Initialization / global-state policy: No hidden network/filesystem writes or
  goroutines in init. Dependencies are explicit. One scanner owns and atomically
  publishes immutable snapshots; package-global mutable product policy is
  prohibited.
```

## 8. Final side-effect boundaries

| Effect | Single owner / choke point | Required capability | Final checks | Retry / duplicate control | Evidence |
|---|---|---|---|---|---|
| Network egress | `internal/search/semantic` provider wire; fixed synthetic tests are a separate test-only path | Explicit semantic/build/certification action, current privacy/artifact capability, eligible input, lazy key | Exact endpoint/protocol, content hash, current policy source, size/token limits, direct transport, no redirect/proxy, timeout | Query and interactive reconcile send once; full build alone uses persisted 1s/4s/9s/16s retry schedule | Recording transport tests, `egress_boundary_test.go`, agent command tests; conditional live commands in Sections 13/14 |
| Durable commit | `internal/status` | Supported Darwin/Linux platform, current lifecycle and artifact policy, legal actor transition, clean file, unchanged source identity/bytes | Revalidate policy bytes and source bytes immediately before descriptor-relative rename; fsync file and parent; then scoped git add/commit | No automatic retry or rollback; stale/dirty/concurrent actions abort; post-write failure says do not resend | Real temporary git/filesystem tests, platform contracts, `make test`, Gate 2 destructive-action scenario |
| File replacement | `internal/status` for vault notes; `internal/search/semantic` for the disposable store | Status capability or explicit semantic build/reconcile writer lease | Same-filesystem/root confinement, modes, complete generation/manifest, policy freshness, atomic activation | Status is one replace; semantic active/previous are immutable and staging alone resumes; failed activation preserves active | Status TOCTOU/durability tests; semantic real-SQLite crash/activation/store tests |
| Publication / activation | External publication is N/A. Semantic generation activation is owned by `internal/search/semantic` | Complete compatible staging generation and writer lease | Complete row set, retry ledger clear, manifest/policy recheck, one SQLite transaction | `previous=active; active=staging; staging=NULL`; no automatic previous fallback | Semantic store/manifest/staging tests and Darwin/Windows platform jobs |
| Credential use | Provider construction inside `internal/search/semantic`; D57 test path for fixed certification | Explicit action plus nonblank exact `YOMIHON_EMBED_KEY` | Read lazily after earlier gates; header use only; no storage/format/log/error | One action-scoped provider; no cross-request auth latch | Provider HTTP tests, import-boundary test, conditional provider certification |
| Subprocess execution | `internal/status` same-binary git child, which descriptor-chdirs and `exec`s resolved `git` | Supported status action and inherited vault-root descriptor | Discrete argv; every `GIT_*` and `YOMIHON_*` entry stripped before external Git; fixed child protocol; clean scoped path | Each status action owns its git sequence; failure is surfaced after write | `git_child_test.go`, real temporary git/environment tests, `make test` |
| Billing / paid provider | Same provider choke point and fixed synthetic certification path | BYOK operator consent through explicit action | Exact approved bytes/destination/dimension, no hidden retry; full build retry is bounded | Operator pays; query has at most one request; certification refuses without opt-in | `make provider-live` and, when required, fixed recording capture; evidence must name date/model/dimension without exposing key |

No row is deleted as a convenience. External publication remains explicitly
N/A until a separately designed publisher exists.

## 9. Security and privacy profile

Threat model: `docs/security/threat-model.md`. It defines the supported local
trust boundary, protected assets, actors, final capabilities, disclosure flows,
abuse paths, supply chain, recovery, and unsupported deployment claims. It is a
design input, not proof of a deployed candidate; independent snapshot review is
still pending.  
Data inventory: `docs/privacy/data-inventory.md`. It covers vault and contract
bytes, in-memory projections, HTTP/CLI data, status/Git copies, credentials,
provider traffic, semantic generations, logs, and fixed synthetic certification,
including retention, deletion, recipients, backups, ownership, and incident paths.

```text
Authentication model: None. Supported use trusts one local OS account and a
  loopback-only listener. Exposing through a proxy, tunnel, container port, or
  widened bind is outside the threat model.
Authorization model: Browser cross-site status requests are rejected;
  lifecycle legality comes from the contract and actor koopa; agent outputs
  and provider sends require current privacy/artifact capabilities. Local
  same-account processes remain intentionally indistinguishable.
Credential source and rotation: YOMIHON_EMBED_KEY from the operator environment,
  read only for an explicit provider action. Rotation/revocation happens in the
  operator's provider account; yomihon stores no copy and provides no key UI.
Approved egress destinations: The fixed Gemini embedding REST endpoint owned by
  internal/search/semantic, and only for the three D32/D50.1/D57 input classes.
  No generic destination, proxy, analytics, update check, crash reporter, or CDN
  is approved.
Redirect / proxy / DNS policy: Provider redirects are refused; environment
  proxies are disabled; transport connects directly to the fixed HTTPS host.
  DNS/TLS use the Go transport. Any proxy or alternate host requires a ruling.
Sensitive log fields: Never log vault/note content, raw semantic query, provider
  submission, credential, provider response body, semantic vectors, or denied
  path evidence. Startup currently logs the configured vault root locally;
  paths and status values remain sensitive local metadata. External terminal,
  service, CI, and pipeline capture owns its retention and deletion.
Telemetry default and consent: No telemetry, analytics, remote crash reporting,
  or metrics export exists. Any addition is a new egress decision.
Retention and deletion: Vault/git retention is external to yomihon. Semantic
  active/previous/staging data is owner-only, pruned by generation role, and
  safe to delete/rebuild. No general log-retention policy exists.
Backup and derived-copy behavior: Back up the vault and git with the operator's
  vault process. Do not back up semantic generations as authority. Git is the
  status audit/recovery layer. No exercised repository runbook currently proves
  restore.
Security report path: SECURITY.md private-vulnerability-reporting procedure.
  The private reporting path must be enabled and tested before first release.
Release signing / provenance: Source-only v0.x loads
  `tools/source-artifact-bootstrap.sh` directly from the exact commit to create
  an atomic version bundle from an annotated tag, clean matching
  checkout, and full independent final-review report containing the exact
  lowercase release-evidence envelope from
  `docs/reviews/REVIEW_REPORT.template.md`. REVIEW_EVIDENCE must itself be safe
  for public release. The builder copies that full report and emits a 16-field
  public release certificate containing its bundle filename and SHA-256
  identity. The five-file bundle is the deterministic gzip-compressed git
  archive, public-safe review report, derived certificate, non-circular
  provenance sidecar, and SHA-256 manifest with four payload rows. The 25-field
  provenance binds the captured annotated-tag object, certificate, full report,
  committed review-template, bootstrap, and source-artifact-toolchain digests, commit/tree,
  engineering-standard digest, profile bytes, dependency locks, nested SQLite
  module, CI workflow, and Git/gzip builder versions. Formal `release` artifacts
  use `--require-tag` and require the commit-pinned `git version 2.55.0` and
  `Apple gzip 479` from `tools/source-artifact-toolchain.txt` to match the host
  exactly. Non-release `verification-fixture` artifacts require explicit
  checker opt-in with `--allow-fixture` and do not require the host to match that
  release toolchain. `make
  source-artifact-check`, included in `make verify`, delegates the adversarial
  contract suite to `tools/test-source-artifact.sh` and proves repeatability
  across conflicting ambient tar umasks, reconstructs and byte-compares the
  complete uncompressed git archive, reconciles the report's human Gate/Verdict
  statuses with its lowercase release envelope, requires completed human header,
  snapshot, Gate 2 scenario, verification-log, watched-red, and independent-
  certification evidence, validates the certificate and exact bundle
  membership, and rejects repaired hostile mutations. Publication uses a
  cooperative per-version lock, validates staging before one same-filesystem
  rename, rejects a competing/nested destination result, and revalidates the
  exact published root after the rename. No signing identity or hosted
  attestation service is claimed for v0.x; tagged-candidate execution remains
  UNVERIFIED, but the verification and provenance mechanism is implemented.
SBOM policy: The current source-only v0.x profile records Go/frontend lockfiles
  and the nested module in the provenance sidecar but does not produce a
  standalone SBOM. Adding an SBOM service, binaries, signing, attestations,
  containers, or installers requires a revised profile; it is not an implicit
  requirement or claim of the selected v0.x artifact mechanism.
```

## 10. Configuration contract

```text
Authoritative schema: cmd/yomihon owns the closed supported environment surface;
  the vault contract at System/schemas/vault-schema.toml owns product schema.
Precedence order: Explicit command flags for commands that define them, then
  YOMIHON_ROOT where supported, then the command default (serve/search default
  ~/obsidian; judge scan commands default --root to cwd). YOMIHON_PORT defaults
  to 9610. YOMIHON_EMBED_KEY is consulted only after an explicit applicable
  semantic action reaches its credential gate.
Unknown-key policy: The process ignores unrelated environment variables; a
  structural test allowlists the only YOMIHON_* configuration reads. The status
  Git child enumerates the environment solely to strip every current/future
  YOMIHON_* value before external execution. CLI flags are command-owned and
  unknown/cross-command flags fail with exit 2.
Secret fields: YOMIHON_EMBED_KEY only.
Dynamic reload semantics: Vault content is rescanned on an approximately
  two-second cadence and published atomically. The vault contract is loaded at
  startup; source drift latches affected capabilities closed for that process.
Restart-required fields: YOMIHON_ROOT, YOMIHON_PORT, and any vault-contract
  source change. Each CLI provider action reads the current key at process use;
  serve never reads it.
Config diagnostics command: yomihon help [command] and recognized -h/--help
  forms are side-effect-free. Startup/CLI errors are the diagnostics surface;
  no separate config-dump command exists by design, avoiding secret display.
Example configuration test: make test includes cmd/yomihon configuration and
  environment-wall tests; CI e2e-http starts a fixture with explicit
  YOMIHON_ROOT/YOMIHON_PORT and verifies one exact loopback bind.
```

## 11. Performance and capacity contract

| Path / workload | Environment | Objective | Budget | Baseline | Regression trigger | Owner |
|---|---|---|---|---|---|---|
| Snapshot change visibility over a low-thousands local vault | Supported local machine; one scanner | Edited/created/deleted content becomes visible without torn graph/nav/search state | At most 3 seconds worst case: about 2-second cadence plus rebuild margin | Canon records about 100 ms at roughly 419 notes; this is historical, not an immutable current benchmark | Bound breaks or corpus approaches about 10,000 files; measure before changing mechanism | `internal/snapshot` |
| Lexical query | Captured in-memory index | Interactive local substring/filter search | No numeric latency percentile is canonically fixed; `UNRESOLVED` for release performance claims | `BenchmarkSearch`; no immutable profile-bound result | Any claimed improvement/regression or user-visible delay requires same-machine benchstat evidence | `internal/search` |
| Semantic exact top-k | Complete compatible generation, 1,536 dimensions | Exact recall baseline before any approximate index | p95 about 100 ms opens the next-rung evaluation; it is an escalation threshold, not a universal CI timeout | Production generation records 40 queries × 3 runs; current source artifacts are not bound to this unapproved profile snapshot | p95 exceeds threshold, chunks reach 100,000, or raw vector payload exceeds 1 GiB | `internal/search/semantic` |
| Semantic generation store | Darwin/Linux owner-only cache; fixed synthetic workload | Complete atomic build/reconcile/load with bounded footprint | Fewer than 100,000 chunks and at most 1 GiB raw vectors; interactive reconcile at most 128 chunks and 100,000 proxy tokens | `docs/benchmarks/semantic-storage-2026-07-18/` contains moving-worktree Darwin/arm64 evidence, not release certification | Identity/schema/corpus/driver change or capacity rung opening | `internal/search/semantic` |
| Browser interaction/startup | Supported Chrome in CI and local loopback | Primary reading/search/status task is responsive and stable | Numeric LCP/INP/startup/memory budgets are `UNRESOLVED` | E2E behavior exists; no Core Web Vitals baseline is retained | Any performance claim or observed interaction pain requires a declared workload and budget | UI feature owner |

```text
Queue bounds: No public background queue. One snapshot scanner owns rebuilds;
  one semantic writer lease owns generation mutation. net/http request
  concurrency has no explicit application semaphore and is UNRESOLVED as a
  capacity claim.
Worker / goroutine bounds: Scanner ownership and request lifetimes are explicit;
  HTTP concurrency is governed by net/http and local-only deployment but has no
  declared maximum. Unbounded feature-owned goroutine creation is prohibited.
Request / object size limits: HTTP headers 64 KiB; status form 4,096 bytes;
  HTTP/CLI search input 4,096 bytes; highlighted source view 1 MiB; provider
  response 8 MiB; semantic exact index <100,000 chunks and <=1 GiB vectors;
  interactive reconcile <=128 chunks and <=100,000 proxy tokens.
Cache budget and eviction: Snapshot/lexical data rebuild in memory and have no
  separate eviction. Semantic storage retains active, previous, and at most one
  matching staging generation, then prunes unreferenced generations. A whole-
  process memory budget is UNRESOLVED.
Connection pool bounds: SQLite uses one open connection per store handle.
  Provider transport allows up to 100 idle connections, 90-second idle timeout,
  30-second request timeout, no proxy, and no redirects. There is no remote DB.
Startup budget: UNRESOLVED. Readiness is Home's direct HTTP 200 after the
  synchronous scan, not merely a bound socket.
Memory budget: Semantic raw vectors are capped at 1 GiB; overall process and
  snapshot memory budgets are UNRESOLVED.
Load-shedding policy: Status/search input and provider bodies are bounded;
  semantic capacity and interactive drift fail before allocation/egress.
  HTTP request concurrency shedding is UNRESOLVED.
Profiling command: No canonical profiling command exists; UNRESOLVED when a
  performance investigation requires one.
Benchmark command: make bench-baseline, then on the same machine/toolchain
  make bench-compare. CI smoke-runs all benchmarks once without a threshold.
```

A missing budget on a performance-critical path is `UNRESOLVED`, not zero.

## 12. Operational contract

```text
Service health entry point: GET / on 127.0.0.1. The CI readiness rule accepts
  only a direct 200 after the synchronous scan and exact child-owned loopback
  bind; redirects, 404, and no response are not ready. There is no separate
  health endpoint.
Liveness meaning: The local process remains running and can answer requests;
  an open socket alone is not readiness.
Readiness meaning: Home answers 200 after routes and initial snapshot are live.
  Browser fixture acceptance additionally requires exactly one successful
  contract-load log; the reading product itself intentionally remains ready
  with the write/metadata faces degraded when the contract is invalid.
Graceful shutdown deadline and behavior: SIGINT/SIGTERM cancel the process;
  http.Server.Shutdown receives 5 seconds. The site then closes owned request
  and scanner lifetimes; shutdown failure is returned and causes process failure.
Primary metrics and units: No production metrics subsystem exists. Semantic
  generation records bounded content-free counts/timings. Operational metric
  ownership is UNRESOLVED.
Log schema and redaction: slog text to stderr for serve lifecycle and safe
  diagnostics. Query/content/key/provider-body logging is forbidden. No
  retention, schema-version, or centralized access policy exists.
Trace / profiling access: No trace exporter, remote profiler, or debug endpoint.
  Ad hoc local Go profiling has no canonical runbook.
Runbook location: `docs/security/threat-model.md` defines boundary-specific
  detection, containment, credential response, and recovery. No end-to-end
  operator deployment/restore runbook or exercised recovery transcript exists.
Backup / restore procedure: Vault/git backup is external; status failures are
  inspected and repaired through the vault file plus git; semantic storage is
  deleted/rebuilt. No representative restore exercise is retained, so support
  for a formal recovery claim is UNVERIFIED.
Disaster or corruption recovery: Fail closed on uncertain status publication;
  preserve the file and manual git evidence. Treat corrupt/incompatible
  semantic storage as unavailable and rebuild explicitly. Never silently call
  an empty or partial result success.
Post-release checks: Bind identity and configuration to the source tag/commit;
  verify exact loopback socket, Home/primary reading task, one supported status
  path on Darwin/Linux, lexical and judge CLI contracts, semantic BYOK behavior
  where included, privacy refusal, logs, and the rollback/restore path. This
  checklist is required but has no exercised runbook yet.
Rollback / roll-forward trigger: A privacy/egress breach, authoritative write
  uncertainty, broken frozen agent contract, unsupported-platform side effect,
  or unrecoverable primary-task failure blocks rollout. Exact source-release
  rollback/roll-forward mechanics are UNRESOLVED.
```

## 13. Canonical commands

```text
Bootstrap:
  make tools
  npm ci --prefix .github --ignore-scripts --no-audit --fund=false
    (Node/npm is required by the unconditional frontend/browser gate)
  Tailwind CSS standalone v4.1.17 and ShellCheck 0.11.0 remain documented
    prerequisites; no one-command bootstrap installs every non-Go tool.

Run locally:
  make run

Run primary user task:
  YOMIHON_ROOT=/absolute/path/to/owned/vault go run ./cmd/yomihon serve
  then open http://127.0.0.1:9610

Generate:
  make gen && make css
Generated drift checks:
  make fmt-check css-check
  On a clean isolated candidate checkout, the complete regeneration no-op
  oracle is:
    make gen && make css && git add -A && git diff --cached --exit-code
  `git add -A` is intentional: it makes modified, deleted, and newly generated
  paths visible to the cached diff. Do not run this index-mutating oracle in a
  dirty operator checkout.

Format:
  make fmt

Build:
  make build
Read-only build gate:
  make build-check

Unit tests:
  make test
  (the repository combines package tests with -race, -count=1, -shuffle=on)

Race tests:
  make test

Integration tests:
  make test
  YOMIHON_ROOT=/absolute/path/to/operator-vault make test-real-vault
    requires explicit Koopa authorization and is mandatory exactly when changed
    code or documentation/canon alters real-vault parsing, projection, search,
    navigation, render, status, judge, or schema boundaries, or changes a
    realvault-tagged test itself. The semantic trigger controls even when such
    behavior is wired through another package; common owned surfaces include internal/vault,
    internal/schema, internal/lesson, internal/nav, internal/search,
    internal/render, internal/judge, internal/status, internal/snapshot, and
    cmd/yomihon composition. It is inapplicable and must not run otherwise. If
    applicable but authorization or the vault is unavailable, record BLOCKED;
    never convert the private-data gate to a silent skip or PASS.

E2E tests:
  make e2e-http-check

Fuzz smoke:
  make fuzz-smoke

Browser / desktop probes:
  make browser-check
  make mutation-check
  Desktop probes are N/A.

Policy validation:
  make policy-check

Static analysis:
  make vet staticcheck lint workflow-check

Security analysis:
  make gosec

Vulnerability analysis:
  make vuln

Cross-platform checks:
  make portable-build-check
  CI portable-core on macos-15 and windows-latest:
    go build ./...
    go vet ./...
    go test -race ./... -count=1
      (macOS only)
    go test ./... -run '^$' -count=1
    go test ./... -run '^TestUnsupportedPlatform' -count=1
      (the preceding two commands are Windows only)
  CI windows-semantic-contract on windows-latest:
    go test ./internal/search/semantic -run '^TestSemanticStoreEntryPointsFailBeforeCreatingFilesOnUnsupportedPlatform$' -count=1
  CI darwin-semantic-contract on macos-15:
    go test -race ./internal/search/semantic -count=1
  Runtime platform contracts cannot be replaced by cross-compilation.

Benchmarks:
  make performance-smoke
  make bench-baseline
  make bench-compare
  The baseline/compare pair is required for performance-affecting changes or
    performance claims and must use the same machine/toolchain/workload.

License / notice:
  make license-check

Release artifacts:
  The certification-grade prepare and assembly commands extract and run
    tools/source-artifact-bootstrap.sh directly from SOURCE_COMMIT, exactly as
    documented in docs/release.md. The Make targets below are checked developer
    conveniences, not proof that the working-tree Makefile was committed:
  RELEASE_VERSION=v0.1.0 SOURCE_COMMIT=<40-character-tagged-commit> \
    make source-archive-candidate
    only after the profile is approved with merge and artifact-build readiness
    GO, artifact-build-blockers none, and no active exception. This produces a
    quarantined exact archive and prints its SHA-256; it is evidence input, not
    release GO.
  RELEASE_VERSION=v0.1.0 SOURCE_COMMIT=<40-character-tagged-commit> \
    REVIEW_EVIDENCE=/absolute/path/to/final-review.md make source-artifact
    only after an independent report has inspected the tagged source and
    quarantined archive, passed all three Gates, and bound that exact archive
    SHA-256. The checkout must be clean and at SOURCE_COMMIT. REVIEW_EVIDENCE
    must be the complete independent final report for that commit and contain the exact
    lowercase release-evidence envelope from
    docs/reviews/REVIEW_REPORT.template.md: distinct atomic builder/reviewer and
    Gate 2 operator identity tokens, all three PASS Gates, final GO, no
    unresolved or blocked checks, and no active release
    exception. The report's human Verdict and Gate 1/2/3 statuses must agree
    with that envelope, and its header, snapshot with the verified archive
    digest, completed PASS Gate 2
    scenario, `make verify` log, completed watched-red row, evidence references,
    and independent certification must satisfy the structural checker. The
    report must be public-safe because the command copies it into the bundle.
    It emits a 16-field public certificate carrying the evidence class/scope,
    exact closed-profile-blocker set, report filename, and
    digest, then
    validates and atomically publishes dist/yomihon-v0.1.0/ containing exactly
    the deterministic source archive, full public-safe review report,
    certificate, 25-field provenance sidecar, and SHA-256 manifest with four
    payload rows. Provenance also records the captured annotated-tag object and
    committed review-template, bootstrap, and source-artifact-toolchain digests. The formal
    command emits artifact class `release` through `--require-tag` and requires
    the committed toolchain pin—`git version 2.55.0` and `Apple gzip 479`—to
    match exactly; test fixtures emit `verification-fixture`, require explicit
    checker `--allow-fixture`, and do not impose that host-toolchain match. It
    refuses a lightweight/mismatched or in-flight-changed tag, dirty/different
    checkout, linked output, or overwrite. A cooperative per-version lock covers
    staged validation, one same-filesystem rename, exact-root race detection,
    and post-rename validation of the published root. `make
    source-artifact-check` delegates the non-release contract tests to
    tools/test-source-artifact.sh. No signing identity, hosted attestation
    service, or standalone SBOM is claimed for source-only v0.x.

Canonical complete verification:
  make verify

Conditional external/private checks omitted from make verify:
  YOMIHON_EMBED_LIVE=1 YOMIHON_EMBED_KEY=... make provider-live
    for provider protocol/identity/egress changes and semantic release certification
  YOMIHON_ROOT=/absolute/path/to/operator-vault make test-real-vault
    only under the exact real-vault change trigger above and with explicit Koopa
    authorization; it is inapplicable and must not run otherwise
  make bench-baseline && make bench-compare
    for performance-affecting changes or claims
  make tools-check-mattn
    for the retained CGO comparison; CI runs it on Linux
```

The canonical complete verification command is: `make verify`.

For one repository checkout, `make verify` executes policy/profile integrity,
module drift, formatting plus templ/sqlc/CSS drift, vet, strict Go/frontend/
workflow lint, staticcheck, gosec, govulncheck, race tests, real-vault test
compilation, the selected SQLite driver, build, HTTP E2E, live fuzz smoke,
browser behavior and mutation probes, the declared cross-build matrix,
performance smoke, license/notice manifests, and deterministic source-artifact
provenance. A green invocation is still not a PASS record until it is bound to
an immutable candidate. Real hosted-OS runtime jobs, PR-envelope validation, the
retained mattn comparison, conditionally authorized real-vault execution,
credentialed provider certification, measured benchmark comparison, and the
actual tagged release-artifact creation remain separately named evidence.

## 14. Verification applicability matrix

| Stage | APPLIES / N/A / DEFERRED-BY-EXCEPTION / UNRESOLVED | Exact command or exception ID | CI job | Required for merge / release |
|---|---|---|---|---|
| Policy validation | APPLIES | `make policy-check` through `make verify`; PR metadata uses `node .github/check-pr-policy.mjs --self-test` and then `node .github/check-pr-policy.mjs` with the PR environment | `verify`, `pr-policy` on pull requests | Yes / Yes; the checker proves envelope structure, not review quality or branch enforcement |
| Formatting | APPLIES | `make fmt-check` through `make verify` | `verify` | Yes / Yes |
| Generated drift | APPLIES | `make fmt-check css-check` through `make verify`; on a clean isolated candidate checkout, `make gen && make css && git add -A && git diff --cached --exit-code` must pass. Adding all paths before the cached diff is required to detect modified, deleted, staged, and newly generated files. The stricter unconditional rule in `docs/standards.md` controls over narrower contributor wording. | `verify`, `assets-drift` | Yes / Yes |
| Module drift | APPLIES | `make mod-check` and `make tools-check-prepare` through `make verify`; locked npm graph through `npm ci` where frontend tools run | `verify`, frontend/browser jobs | Yes / Yes |
| Build | APPLIES | `make build-check`; platform jobs use `go build ./...` | `verify`, `portable-core` | Yes / Yes |
| `go vet` | APPLIES | `make vet`; nested modernc through `make tools-check`; retained mattn through `make tools-check-mattn` | `verify`, `portable-core` | Yes / Yes |
| `staticcheck` | APPLIES | `make staticcheck`; nested checks in `tools-check` / `tools-check-mattn` | `verify` | Yes / Yes |
| Strict lint | APPLIES | `make lint`, `make workflow-check`, and `make frontend-check`, all through `make verify` | `verify`, `lint-frontend` | Yes / Yes |
| Security analysis | APPLIES | `make gosec`; feature boundary tests run under `make test` | `verify`, E2E jobs | Yes / Yes |
| `govulncheck` | APPLIES | `make vuln` through `make verify` | `verify`, `govulncheck` | Yes / Yes |
| Unit tests | APPLIES | `make test`; nested driver tests through `make tools-check` | `verify`, `coverage` | Yes / Yes |
| Race tests | APPLIES | `make test`; nested modernc tests; real macOS semantic race test | `verify`, `portable-core` on macOS, `darwin-semantic-contract` | Yes on supported race platforms / Yes |
| Integration tests | APPLIES | Real temp git/SQLite/HTTP composition is in `make test`; `make real-vault-build-check`; with explicit Koopa authorization, `YOMIHON_ROOT=... make test-real-vault` is mandatory exactly when changed code or documentation/canon alters real-vault parsing, projection, search, navigation, render, status, judge, or schema boundaries, or changes a realvault-tagged test. It is inapplicable and must not run otherwise; an applicable run without authorization/vault is BLOCKED, not skipped. | `verify`, `e2e-http`, platform semantic jobs; real-vault run is deliberately private and has no CI job | Hermetic always; private gate exactly on the named trigger / Same trigger over the release delta |
| E2E tests | APPLIES | `make e2e-http-check` through `make verify` | `verify`, `e2e-http` | Yes / Yes |
| Fuzz smoke | APPLIES | `make fuzz-smoke` through `make verify`; it discovers every owned `Fuzz*` target, refuses an empty manifest, and runs exactly 10,000 executions for each target with one worker | `verify`, `fuzz` | Yes / Yes |
| Browser / desktop probes | APPLIES for browser; desktop N/A | `make browser-check` through `make verify`; desktop probes remain N/A because there is no desktop shell | `verify`, `e2e-behavior` | Yes for current browser product / Yes |
| Migration / compatibility | APPLIES | Frozen judge/agent/storage compatibility tests run under `make test`. A schema-version or frozen-wire change must add released-fixture/copy-forward evidence; no generic migration command exists. | `verify`, platform jobs | When compatibility surface changes / Yes |
| Mutation locks | APPLIES | `make mutation-check` through `make verify`; every new load-bearing lock also needs a retained watched-red transcript naming the applied mutation and restored green | `verify`, `e2e-mutations` | For every new/changed lock / Replay high-risk set |
| Cross-platform | APPLIES | `make portable-build-check` through `make verify`; runtime commands are the `portable-core`, `windows-semantic-contract`, and `darwin-semantic-contract` commands in Section 13 | `verify`, `portable-core`, `windows-semantic-contract`, `darwin-semantic-contract` | Cross-build locally plus real hosted OS evidence / Yes |
| Performance regression | APPLIES | `make performance-smoke` through `make verify`; performance-affecting changes or claims additionally run `make bench-baseline && make bench-compare` on the same environment. Missing startup/UI/memory budgets remain `UNRESOLVED` and prevent a general PASS. | `verify`, `fuzz` benchmark smoke | Smoke always; comparison when performance-sensitive or claimed / Yes for affected release paths |
| License / notice | APPLIES | `make license-check` through `make verify`; dependency/asset changes and releases additionally require candidate-bound semantic compatibility/notice review | `verify` | Automated manifest/module integrity always; manual review when dependency/asset scope changes / Yes |
| Artifact / provenance | APPLIES | `make source-artifact-check` through `make verify` delegates to `tools/test-source-artifact.sh`. On an approved clean tagged candidate, merge and artifact-build readiness must be GO, artifact-build blockers none, release state `PENDING-ARTIFACT`, and remaining open blockers exactly the post-artifact set. The committed-bootstrap prepare phase creates the exact quarantined archive and digest for independent review; the Make target is only a convenience wrapper. Only a public-safe release-candidate/complete-project-profile report that binds that digest, passes all Gates, closes every finding, explicitly closes exactly the post-artifact blocker set, and has independent Gate 2/certification evidence may enter committed-bootstrap final assembly; assembly rebuilds and byte-matches the archive before atomically publishing the local five-file bundle. The 16-field certificate and 25-field provenance bind report, archive, tag object, profile, standard, bootstrap, locks, CI, toolchain, and dependency inputs. Formal artifacts pin Git/gzip; fixture checks require explicit opt-in. Git replacements/grafts, repository/export attributes, gitlinks, ambient config, checkout filters, unsafe report bytes, prepared-archive destination races, post-rename corruption, and repaired evidence chains have named mutations. Signing, hosted attestations, and a standalone SBOM are not claimed. | `verify` | Reproducibility/certificate/mutation lock for merge / Prepared archive digest plus actual tagged five-file bundle and independent report/certificate/manifest/provenance inspection for release |
| Real-provider certification | APPLIES | `YOMIHON_EMBED_LIVE=1 YOMIHON_EMBED_KEY=... make provider-live`; when model/dimension/chunking/eval identity changes also run `YOMIHON_EMBED_LIVE=1 YOMIHON_EMBED_KEY=... YOMIHON_RECORDING_OUTPUT=/absolute/private/path.json YOMIHON_RECORDING_DIMENSION=1536 go test -count=1 -run='^TestCaptureSyntheticRecording$' ./internal/search/semantic` | None; deliberately credentialed | Provider protocol/identity/egress changes only / Yes when release claims semantic support |

The product jobs have no path filter and run on pull requests and pushes to
`main`; `pr-policy` is pull-request-only because it validates PR metadata.
GitHub's branch-protection and rulesets APIs were inspected on 2026-07-18 and
returned HTTP 403 for the private repository's current plan. Proposed
`EX-2026-001` records that gap but is not approved and has no exception force;
the repository must not claim mechanical enforcement or a merge GO from it.

## 15. Risk-to-evidence map

| Contract / invariant | Risk | Production boundary crossed | Test classes | Watched-red mutation | Evidence owner |
|---|---|---|---|---|---|
| Listener is loopback-only and local bytes never gain a remote serving path | Private vault exposure | Real `net.Listen` and live socket | Unit/environment-wall, live HTTP/socket E2E, route/CSP tests | `smoke.sh --self-test` plus loopback mutation in E2E | Independent reviewer |
| Status changes exactly one line, durably, then commits exactly that file | Corruption, false audit, duplicate action | Real temp filesystem, descriptor-rooted rename/fsync, real git subprocess | Golden/property/fuzz, TOCTOU, real-git integration, platform tests | Per-change mutation must break the surgical/final-boundary lock and observe the named test red; current complete transcript is `UNVERIFIED` on this profile snapshot | Independent reviewer |
| Contract is sole schema authority and write/metadata faces fail closed on invalid/stale source | Unauthorized transition or false projection | Real contract file and final status boundary | Cross-product table tests, source-drift latch, direct POST | Structural test rejects hardcoded statuses; replay required when authority path changes | Independent reviewer |
| Denied paths neither appear nor influence judge/provider output | Private-data inference or direct egress | Judge stdout and provider transport | Real graph/judge fixtures, recording transport, policy-source recheck, import analysis | Bypass/drop final privacy check must make the named privacy/egress test fail | Privacy reviewer independent of builder |
| Query is sent only for explicit applicable semantic action, at most once, and never logged/echoed | Private query leak or amplified billing | Final provider HTTP request and CLI serialization | Recording HTTP transport, CLI matrix, log/error absence tests | Alternate-send/retry/query-echo mutation required for changes to this path | Privacy reviewer independent of builder |
| Agent wire bytes and exit codes remain frozen and honest | Automation breakage or partial result reported success | stdout/stderr and process exit | Golden bytes, subprocess/CLI tests, fixture compatibility | Golden/update path must not auto-accept; deliberate byte/status mutation must fail the consumer test | Independent agent-surface reviewer |
| One immutable snapshot generation feeds graph/nav/search per request | Torn or stale authority, races | Scanner goroutine and atomic publication | Race, synctest, deterministic interleaving, request-capture tests | Publish/capture bypass mutation required when generation mechanics change | Concurrency reviewer |
| Authored Markdown/report HTML cannot execute first-party or automatic remote behavior | Script execution, automatic egress, same-origin abuse | Real browser parser, CSP, raw routes, sandbox iframe | Renderer unit/fuzz, route headers, live Chrome probes | `browser-boundary.mjs` registered mutations and exact caught markers | Security reviewer |
| Semantic activation exposes only one complete compatible generation | Partial/stale ranking, corrupt store, paid-vector loss | Real SQLite transactions/files and writer lease | Real-SQLite integration, crash/interruption, corruption, manifest, capacity, Darwin/Windows runtime | Activation/previous-fallback/staging-visibility mutations required on change; profile-bound replay currently `UNVERIFIED` | Storage reviewer independent of builder |
| Unsupported platforms refuse before target filesystem/key/provider access | Side effect outside proved durability/privacy model | Real Windows/macOS runner | Focused runtime platform tests plus compile matrix | Test must detect created directory/file/key/provider access under refusal | Independent cross-platform reviewer |
| Checked-in generated and redistributed assets match owned source | Stale build, hand edit, missing provenance | templ/sqlc/Tailwind generation and distributed source tree | Drift jobs, format checks, `make license-check`, semantic license review | Generator/manifests mechanically reject source/output or digest mismatch; profile-bound watched-red and compatibility review remain `UNVERIFIED` | Release reviewer |

No table row is a PASS record. Formal evidence must bind the replay and its
restored green result to an immutable candidate.

## 16. Gate 2 acceptance matrix

Acceptance must be conducted by a person or agent that did not implement the
candidate and uses only supported public surfaces.

| Scenario | Public entry point | Expected result / recovery | Evidence artifact | Owner |
|---|---|---|---|---|
| Clean first use | Source build; `yomihon serve` against a reviewable fixture; browser Home/note/search | Direct Home 200 after scan, readable content, discoverable help and primary task; no hidden service/provider prerequisite | Candidate CI E2E logs plus cold-session transcript/screenshot | Independent acceptance operator |
| Experienced use | Restart against an existing fixture, revisit reading and lexical workflows, then build and query a compatible semantic generation through the documented CLI | Existing local state remains understandable; ordinary reading/search stays provider-independent; semantic reuse and rebuild behavior are explicit and no hidden migration or send occurs | Second cold-session transcript with store/CLI before-after evidence | Independent acceptance operator |
| Missing configuration | `yomihon help`; serve with absent root; `search --semantic` without key | Help performs no config/filesystem/key access; missing vault fails before listen; missing key is typed `embedder-unconfigured` only after explicit applicable semantic request while lexical remains valid | CLI subprocess transcript and no-side-effect observation | Independent acceptance operator |
| Invalid input | Invalid command/flag, oversized query, malformed status form/path/transition | CLI exit 2 or HTTP 400/422 with fault ownership; no file/provider send; supported next action visible | CLI/HTTP transcript and filesystem/transport before-after evidence | Independent acceptance operator |
| Offline / dependency failure | Reader/lexical/judge plus explicit semantic CLI with provider unreachable | Local surfaces remain usable; semantic exits 3 with sanitized reason and truthful lexical result where answerable; no in-place retry | Recording/offline transport artifact and public CLI transcript | Independent acceptance operator |
| Partial or stale result | Stale status page, changed contract, compatible/incompatible semantic generation | Status 409/503 with no unauthorized write; strict semantic never ranks stale/partial vectors and names rebuild/restart/reconcile action | State matrix transcript plus store/file hashes | Independent acceptance operator |
| Cancellation and retry | SIGINT/SIGTERM during serve, semantic action, and status-adjacent work | Server drains for up to 5 seconds; query is not resent; status/semantic outcome says whether a side effect happened and whether a new action is safe | Process transcript, provider call count, git/store state | Independent acceptance operator |
| Privacy-sensitive operation | Local Diary reading; judge/semantic against denied paths; invalid privacy authority | Human local read remains available; denied content has zero output/influence/send; invalid authority produces no protected payload; no raw query/key/provider body in diagnostics | Recording transport/output hashes and sanitized log capture | Independent privacy reviewer |
| Non-ASCII / boundary input | CJK/NFC/NFD paths and search, CRLF/YAML/wikilink scars, 4,096-byte limits | Deterministic normalized behavior, frozen wire, explicit rejection at limit, no panic or path escape | Golden/fuzz corpus replay and public output transcript | Independent acceptance operator |
| Supported platforms | Public commands on Linux, macOS, Windows | Read/judge/lexical work on all three; status/semantic store work on Darwin/Linux; Windows refuses before any write/key/provider action | Named platform CI run and filesystem before-after evidence | Independent cross-platform reviewer |
| UI keyboard / narrow / no-JS | Real Chrome at keyboard-only, zoom/narrow widths, reduced motion, and JavaScript disabled | Reading/navigation and plain GET search remain truthful; status forms work without JS; focus and controls remain reachable; D49 is separately unresolved under v2 | Behavior/mutation logs plus screenshots/DOM/accessibility observations | Independent browser reviewer |
| Upgrade / migration / restart | Change contract source, semantic identity/schema, or binary version | Contract-source drift closes relevant capabilities until restart; incompatible semantic identity requires explicit build; known-compatible future schema copies forward into a new file | Before/after store and process transcript tied to released fixture | Independent compatibility reviewer |
| Destructive action / recovery | Real temp-vault status transition and induced post-write git failure | Clean success is one line/one commit/303; uncertain or commit-failed state says bytes changed and do not retry; git/file inspection enables manual recovery | `git show --stat --oneline`, byte diff, recovery transcript | Independent write-boundary reviewer |

No first-public-release Gate 2 pass exists yet. `docs/release.md` additionally
requires at least two cold independent agent sessions covering its full fixture,
BYOK, privacy, failure, and recovery set.

## 17. Review and release policy

```text
Changes requiring architecture decision: Any new/removal of a major concept,
  source of truth, durable/public format, compatibility promise, database or
  framework, cgo/unsafe/reflection-heavy infrastructure, cross-cutting
  dependency direction, privacy/egress/durability semantics, or irreversible
  side-effect owner.
Changes requiring security review: Network ingress/headers/CSP/sandbox/raw
  routes, external input/path parsing, status/git/filesystem boundaries,
  credential handling, provider transport, dependencies/workflows, generated
  artifacts, or any new executable/subprocess surface.
Changes requiring privacy review: Any data inventory, never-egress policy,
  agent output/influence, document/query/provider send, logging/error/metric/
  trace field, semantic derived copy, new destination, redirect/proxy/retry, or
  certification-input change.
Changes requiring independent Gate 2 acceptance: All R3 final-boundary changes;
  every public CLI/wire/browser workflow change; cross-platform behavior; and
  every release candidate. Pure internal refactors still require independence
  when they can alter one of those paths.
Changes requiring independent red team: Privacy/egress, credentials, status
  publication/durability, raw HTML/CSP/sandbox, frozen agent outputs, semantic
  activation, cross-platform fail-before-write, and first-public-release scope.
Changes requiring real-provider certification: Provider model/protocol,
  dimension, request/response parsing, token/truncation behavior, transport,
  egress identity, chunk/query submission, or recorded-eval identity changes;
  and any release that claims the provider-backed semantic surface without
  sufficiently current candidate-bound evidence.
Required approvals: Independent guide/reviewer acceptance first; security or
  privacy reviewer where applicable; CODEOWNER review by `@Koopa0`; Koopa owns
  product rulings, push, release, and merge. A builder cannot certify their own
  R3 change.
Merge strategy: `docs/merge-policy.md` requires one pull request bound to its
  current 40-character head, three separate PASS gates, final GO, independent
  review, applicable checks, resolved findings, and CODEOWNER approval. The
  policy target is a protected, linear `main`, but GitHub enforcement is currently
  unavailable and proposed `EX-2026-001` is unapproved; neither it nor this
  profile authorizes a merge. Even if EX-2026-001 is independently approved, it
  permits Koopa to choose at most an exception-permitted private merge with an
  `ACCEPT-WITH-GATES` verdict; that is not merge-ready and cannot be described as
  complete, release-ready, or production-ready. Squash versus rebase is not
  canonically fixed and is NEEDS-OWNER before evidence depends on resulting
  commit shape.
Immutable snapshot identity: Full commit SHA, clean/described worktree,
  ENGINEERING_STANDARD.sha256, go.mod/go.sum and nested/npm lock state,
  generated-state result, tool/OS/arch, exact commands and CI run. Release adds
  annotated source tag and captured tag-object identity, archive/artifact digest,
  the bundled public-safe independent final-review report and digest, the
  generated certificate and digest, committed review-template and
  source-artifact-toolchain digests, artifact class, and recorded Git/gzip
  builder versions. Formal release identity requires the committed
  `git version 2.55.0` and `Apple gzip 479` pin.
Release owner: Koopa.
Artifact identity / signing: Source-only v0.x uses the exact annotated tag,
  clean matching checkout, full public-safe independent final-review report,
  generated 16-field public certificate, atomic five-file version bundle,
  deterministic archive, four-row SHA-256 manifest, and 25-field provenance
  sidecar produced through the committed bootstrap. The full report is bundled; the
  certificate carries its filename and SHA-256 identity, and provenance binds
  both while also recording the captured annotated-tag object and committed
  review-template, source-artifact-bootstrap, and source-artifact-toolchain digests. Formal artifacts carry
  class `release`, use `--require-tag`, and require the commit-pinned
  `git version 2.55.0` and `Apple gzip 479` to match exactly; non-release
  `verification-fixture` checking is explicit through `--allow-fixture` and does
  not require that host match. `make source-artifact-check` delegates to
  `tools/test-source-artifact.sh` as the reproducibility, complete uncompressed
  git-archive reconstruction, completed human-report structure, Gate/Verdict
  reconciliation, certificate-contract, exact-membership, cooperative
  publication-lock/post-rename exact-root, and repaired-mutation lock. The
  profile intentionally claims no signing identity, hosted attestation service,
  or standalone SBOM. Release GO still requires execution and inspection on the
  immutable tagged candidate; the mechanism itself is not missing.
Release notes and changelog: Both are required for the exact candidate by
  docs/release.md. No canonical changelog/release-note file exists yet.
Post-release observation window / trigger: No duration is canonically fixed.
  Privacy/egress, data/audit uncertainty, primary-task failure, frozen-contract
  breakage, or unsupported-platform side effects trigger immediate stop and
  rollback/roll-forward. Duration and owner actions are NEEDS-OWNER before release.
```

Readiness claims are distinct:

- **Merge-ready** is a final `GO`: an immutable candidate commit passes `make
  verify`, every exactly triggered private/external gate, unconditional
  regeneration/no-drift, watched-red evidence for new locks, cold independent
  three-gate review, all findings disposed, CODEOWNER approval, every applicable
  PR job, and actual protected-main enforcement from `docs/merge-policy.md`.
  EX-2026-001 cannot produce this claim: if independently approved, it permits
  Koopa to choose at most an exception-permitted private merge under
  `ACCEPT-WITH-GATES`, and that merge remains explicitly not merge-ready. The
  current exception is merely proposed, so neither merge-ready nor an
  exception-permitted merge is available for this moving checkout.
- **Release-ready** means merge-ready plus every applicable Section 14 stage,
  the six first-release gates in `docs/release.md`, exact release notes and
  changelog, supported-platform evidence, privacy/secret clearance, license and
  notices, two cold independent fixture-vault sessions, private reporting test,
  real product screenshot, approved deterministic brand/favicon, provider
  evidence where semantic support is claimed, and immutable source artifact
  identity/provenance. It means source-only v0.x, not a binary, installer,
  container, hosted service, or v1 API promise.
- **Production-ready** means a named local operator environment is release-ready
  for its enabled surfaces and additionally has verified real configuration,
  vault ownership/permissions, git identity, provider credential/account/terms,
  direct-network policy, backup and exercised restore, loopback/process startup,
  capacity against the real workload, safe logs/observation, real primary-task
  and status-path acceptance, environment-specific provider certification, and
  a tested rollback/roll-forward procedure. The current repository is not
  production-ready under this standard because the runbook, restore exercise,
  capacity budgets, observation policy, and immutable certification are open.

## 18. Exceptions and current debt

Active v2-valid exceptions:

| ID | Clauses | Scope | Owner | Expiry / trigger | Gate effect |
|---|---|---|---|---|---|
| None | No independently approved v2 exception record exists | Repository | Koopa | N/A | No MUST is waived |

Proposed exception records with no current force:

| ID | Clauses | Scope | Owner | Approval / trigger | Gate effect |
|---|---|---|---|---|---|
| EX-2026-001 | Standard sections 6, 18, 21, and 22.8 | GitHub `main` protection while the repository is private on the current plan | `@Koopa0` | **Proposed; approver unassigned.** Close before public visibility or another writer gains access | Cannot support PASS or GO. If independently approved as written, maximum private-merge verdict is `ACCEPT-WITH-GATES`; release and production remain `NO-GO`. |

Legacy deviation requiring conversion or closure:

| ID | Clauses | Scope | Owner | Expiry / trigger | Gate effect |
|---|---|---|---|---|---|
| D49 | Browser keyboard accessibility and Gate 2 keyboard scenarios | Global printable shortcuts without disable/remap, limited to the current one-user local product and guarded typing/dialog contexts | Koopa | Re-rule on multi-user, remote, voice-control, or alternative-input scope | Product canon records the deviation, but it lacks the full v2 exception fields and independent approval. It is `UNRESOLVED`, not a GO-supporting exception. |

Known debt accepted by v2 policy:

| ID | Contract and risk | Containment | Owner | Closure trigger | Gate effect |
|---|---|---|---|---|---|
| None | No debt record has yet been accepted through the v2 exception/debt process | N/A | Koopa | N/A | Unresolved items below remain blockers rather than accepted debt |

Current blockers:

| ID | Contract and risk | Current containment | Owner / required ruling | Gate effect |
|---|---|---|---|---|
| PROFILE-U1 | GitHub does not currently enforce the target `main` protections, and EX-2026-001 is only proposed | PR evidence envelope, CI on PR/push, CODEOWNERS, owner-only push authority | `@Koopa0` plus an independent exception approver; account/publication choice remains owner-owned | Blocks merge GO now; blocks release and production while open |
| PROFILE-U2 | The source-artifact, public-report/certificate, and provenance mechanism exists, but no immutable tagged candidate has run it and the candidate semantic license/notice review is `UNVERIFIED` | `make source-artifact-check` in `make verify`, committed-bootstrap prepare/assembly, `make license-check`, atomic five-file bundle with public-safe structurally completed full report, 16-field class/scope/blocker-bound certificate, four-row manifest, and 25-field tag-object/review-template/bootstrap/toolchain-bound provenance; formal/fixture evidence classes, profile and human-approval preflight, exact post-artifact blocker closure, isolated Git/archive context, final source revalidation, pinned release toolchain, prepared-archive no-replace publication, cooperative bundle lock, post-rename exact-root validation, and hostile mutation tests; manual review | Release owner and independent release reviewer; no signing/SBOM ruling is required for the selected source-only v0.x mechanism | Blocks release GO until candidate-bound artifact execution, public-report/certificate/checksum/provenance/tag-object/toolchain inspection, and semantic license/notice review pass |
| PROFILE-U3 | Runtime architecture support beyond the declared six 64-bit cross-build targets, dependency-proxy/checksum policy beyond current sums, deprecation window, and final linear merge method are incomplete | Hosted OS jobs, portable cross-build gate, and current Go/npm lockfiles | Koopa for support/compatibility choices | Blocks broader support/compatibility claims; the unresolved dependency-proxy/checksum evidence also blocks release GO until resolved, classified N/A with evidence, or covered by a valid approved exception |
| PROFILE-U4 | Browser startup/interaction, whole-process memory, HTTP concurrency/shedding, and profiling budgets or baselines are absent | Input/vector bounds, `make performance-smoke`, and benchmark comparison tools exist | Koopa sets user objectives; engineering records candidate-bound workloads, budgets, and baselines | Blocks general performance claims, release-ready Gate 3 for the browser product, and production-ready claims until resolved or covered by a valid approved exception |
| PROFILE-U5 | Boundary recovery is documented, but no end-to-end operator deployment/restore exercise, source rollback mechanics, or post-release observation window and owner actions exist | Threat-model recovery, local Git audit, rebuildable semantic store | Koopa as operator/release owner | The unresolved observation window/actions block release-ready; deployment/restore and rollback exercise gaps additionally block production-ready until resolved or covered by a valid approved exception |
| PROFILE-U6 | Regulated-data applicability is unsettled | No regulated-data support claim may be made | Koopa | Blocks GO for any regulated-data use case |
| PROFILE-U7 | First-release evidence is incomplete: independent security/privacy review, private reporting test, release notes/changelog, two cold sessions, screenshot, final brand/favicon, and artifact identity | `docs/release.md` prevents claiming v0.1.0 ready | Koopa and independent acceptance operators | Blocks release-ready claim |

## 19. Profile approval

```text
Profile version: 0.1-draft
Approval binding: UNVERIFIED — no immutable commit contains this profile. At preparation
  time the moving worktree was based on
  aeb2d9f598820a04e084f4e69286f717ea0ccfcd, but the profile and much of its
  current canon were uncommitted, so that SHA is not an approval snapshot.
Prepared by: Codex drafting agent, limited to PROJECT_PROFILE.md
Architecture approval: PENDING — Koopa has not approved this profile revision
Security / privacy approval where applicable: PENDING — threat/data documents exist, but independent snapshot review was not performed
Operations approval where applicable: PENDING — end-to-end runbook and recovery evidence unresolved
Independent approval: PENDING — no independent approver or immutable candidate yet
Date: 2026-07-18
Next review trigger: After the profile and cited canon are committed on a clean
  candidate; after any risk/capability/platform/egress/write/compatibility
  change; before the first public release; and at least whenever a listed
  UNRESOLVED item is closed, or an exception is proposed, approved, renewed,
  expired, or closed.
```

## 20. Machine-readable readiness envelope

This small envelope is the mechanical projection of Sections 17–19. It does
not replace their reasoning or approvals. A change to any field MUST update the
human readiness analysis and pass independent review in the same immutable
commit. An approved profile uses the exact `EXTERNAL-RELEASE-REPORT` approval
binding token rather than attempting the impossible self-reference of placing
its containing commit SHA inside itself. The independent release report then
binds both the candidate commit and the exact `PROJECT_PROFILE.md` SHA-256, and
the certificate/provenance chain binds that report. The pre-review archive
phase accepts only an approved profile whose
merge and artifact-build readiness are GO, whose artifact-build blockers are
`none`, whose remaining open blockers are exactly the declared
post-artifact blockers, whose release state is `PENDING-ARTIFACT`, and which
has no active exception. It does not claim release GO. The
independent report binds the resulting archive digest; final assembly rebuilds
and matches those bytes before its external evidence can complete release GO.

```text
profile-status: PROPOSED
merge-readiness: NO-GO
artifact-build-readiness: NO-GO
artifact-build-blockers: PROFILE-U1,PROFILE-U2,PROFILE-U3,PROFILE-U4,PROFILE-U5,PROFILE-U6,PROFILE-U7
post-artifact-blockers: none
release-readiness: NO-GO
production-readiness: NO-GO
open-blockers: PROFILE-U1,PROFILE-U2,PROFILE-U3,PROFILE-U4,PROFILE-U5,PROFILE-U6,PROFILE-U7
active-exceptions: none
```

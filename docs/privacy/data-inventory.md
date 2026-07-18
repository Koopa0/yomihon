# Data inventory

This inventory covers the current yomihon product and D57's opt-in developer
provider certification. Product authority comes from the four walls and
D18/D32/D47/D50/D57, not from a hosted-service account or analytics policy.
The operator remains responsible for any separate legal basis required by
their use of the vault or provider.

No application telemetry, analytics, metrics exporter, trace exporter, crash
reporter, shared account, or remote log sink is implemented. Yomihon does not
implement application-level encryption at rest, backup/restore, or secure
erasure. Host disk encryption, terminal/service retention, vault sync, Git
remotes, and provider retention are external and must not be inferred from this
document.

## 1. Vault source content

- **Fields / purpose / authority:** every selected vault file: Markdown body
  and frontmatter, path, lifecycle metadata, image/PDF/source/report bytes, and
  contract-private content. These support local reading, search, diagnostics,
  and governed status changes. D18 permits local private reading but forbids
  private output or influence on agent egress.
- **Source / trigger / storage / access:** read from the operator-selected vault
  at server scan/rebuild, explicit CLI action, or direct loopback request. The
  vault remains storage and truth. The same OS account and any direct client
  able to reach the unauthenticated loopback port can access local responses;
  browser-origin protections are not direct-client authentication.
- **Retention / deletion / export / recipients:** vault and Git/sync policy own
  retention and deletion; yomihon never deletes a vault file. Any file may be
  returned to the local reader. Agent output is privacy-filtered, and only
  eligible instance-note chunks may reach Google through section 7. Local
  browser, CLI, and downstream stdout consumers are recipients.
- **Logs / backups / derived copies:** bodies and raw search queries are not
  request-logged; paths and errors may reach stderr. Yomihon creates no vault
  backup. External Git/sync/host backups, process projections (section 3),
  status temp/commit data (section 5), and eligible semantic vectors (section
  8) are copies.
- **Owner / incident path:** Koopa. Stop the affected flow, preserve minimal
  private evidence, assess local/provider/repository copies, and use
  [`SECURITY.md`](../../SECURITY.md). Local deletion cannot recall consumed
  stdout or a third-party copy.

## 2. Contract, capability, and Git metadata

- **Fields / purpose / authority:** `vault-schema.toml` bytes; lifecycle,
  navigation, artifact, and privacy declarations; policy/source fingerprints;
  Git status, hashes, author, path history, and status commit message. The
  contract is the sole machine authority; Git proves cleanliness/provenance and
  records the one write.
- **Source / trigger / storage / access:** read from the vault contract and
  local Git repository at startup, agent action, status view/flip, provenance
  lookup, semantic revalidation, and commit. Source stays in the vault;
  content-free fingerprints also enter the semantic cache. `internal/schema`
  reads the contract and `internal/status` alone executes Git.
- **Retention / deletion / export / recipients:** vault policy owns contract and
  Git retention/deletion. The contract is locally readable; commit messages
  contain path and `from`/`to`. Yomihon never pushes. Local readers/Git and any
  operator-configured remote or backup are recipients.
- **Logs / backups / derived copies:** startup logs contract version/stage count
  or an error, not a deliberate full dump. Git/external host copies are the
  backups. Captured capabilities, fingerprints, and a stale-source latch are
  derived copies.
- **Owner / incident path:** Koopa. Treat unauthorized contract/Git change as
  authority compromise, close the action, compare trusted history, restore a
  reviewed source, and restart to derive fresh capabilities.

## 3. In-memory projections

- **Fields / purpose / authority:** parsed notes, rendered HTML/text/sections,
  diagnostics, graph/navigation, lexical index/snippets, status views, eligible
  semantic chunks, vectors, and immutable exact index. They provide one
  coherent local request/action projection and are never truth.
- **Source / trigger / storage / access:** derived in RAM from vault/contract
  scans and, for vectors, the provider/admitted generation. Server snapshots
  include locally readable private content; the semantic corpus/index excludes
  private and non-instance sources. Only the process and receiving request or
  action access them.
- **Retention / deletion / export / recipients:** retained until snapshot
  replacement, action end, or process exit. Go reclamation is not secure
  zeroization. Rendered/results data goes to the local client/stdout; only the
  exact section 7 payload goes to Google.
- **Logs / backups / derived copies:** content is not routinely logged; scan or
  render failures may log path/error and query failures log size/filter keys.
  No backup. Response buffers, cache rows, temp status file, and redirected
  stdout are further copies.
- **Owner / incident path:** Koopa. Stop/restart to drop live projections, then
  investigate each response, output, cache, or provider copy separately; do not
  describe restart as secure erasure.

## 4. Local HTTP and CLI input/output

- **Fields / purpose / authority:** HTTP method/path/query/headers; status
  `path`/`from`/`to`; CLI root, flags, query and limit; pages/raw bytes;
  diagnostics and stable result envelopes. They implement the user and agent
  surfaces; query text is not telemetry.
- **Source / trigger / storage / access:** supplied per request/action by local
  browser, direct client, shell, or agent. Data stays in request/action and
  output buffers; there is no request database, query history, or app access-log
  file. HTTP is unauthenticated loopback; agent output requires valid privacy
  authority.
- **Retention / deletion / export / recipients:** yomihon retains only for the
  action. Browser/shell/service/pipeline history is external. Responses remain
  loopback; one explicit semantic query projection may reach Google; a caller
  may forward stdout and then owns it.
- **Logs / backups / derived copies:** raw search query is excluded from logs,
  cache, errors, metrics, and traces. Query failures record byte count/filter
  keys; selected status failures record path/from/to/error. External capture is
  the only backup. Parsed query, snippets, HTML, provider request, and commit
  metadata are derived copies.
- **Owner / incident path:** invoking operator and downstream consumer. Stop the
  consumer flow, restrict/delete external captures, rotate the provider key if
  implicated, and report any privacy-authority bypass privately.

## 5. Status mutation and audit commit

- **Fields / purpose / authority:** target path; submitted/current/target
  status; original and rewritten note bytes; file/parent identity and mode; Git
  cleanliness/output; commit message/hash. One schema-authorized status line is
  changed and locally audited.
- **Source / trigger / storage / access:** one local `POST /status`, current
  note, contract, and Git state. `internal/status` holds data in RAM, an
  exclusively created adjacent `.yomihon-status-*.tmp`, the replaced note, and
  Git index/history. Only this package owns the capability; publication is
  supported on macOS/Linux. Before the external Git exec, it removes every
  `GIT_*` and `YOMIHON_*` environment entry; unrelated ambient variables retain
  normal Git inheritance.
- **Retention / deletion / export / recipients:** temp lasts until rename or
  handled cleanup; a crash may leave it. Note and commit follow vault policy.
  Reversion/history rewrite is manual, never automatic rollback. Nothing is
  remotely exported; local filesystem/Git and requester are recipients. Trusted
  Git hooks/filters may execute.
- **Logs / backups / derived copies:** selected failures record path/from/to,
  changed state, and error, not deliberate full note bytes. Git error output may
  be shown after publication. The commit is audit history, not a guaranteed
  backup. Temp, Git objects/index, message, recovery view, and provenance are
  copies.
- **Owner / incident path:** Koopa, represented by actor `koopa` and vault Git
  identity. Before rename, fix/reload/start a new action. After uncertain
  durability or commit failure, do not resubmit; inspect note, `git diff`, index,
  and history, then complete or revert manually.

## 6. Embedding credential

- **Fields / purpose / authority:** operator-owned `YOMIHON_EMBED_KEY`, used
  only to authenticate an explicit semantic or fixed-synthetic provider action.
  The project supplies no shared key/account.
- **Source / trigger / storage / access:** supplied in process environment;
  application code calls its lazy reader only after local semantic gates pass,
  while developer certification requires explicit opt-in. It remains ambient
  process/RAM data and is not persisted by yomihon. Same-UID/platform mechanisms
  can inspect the environment; final-gate minimization is not host isolation.
  The status Git child strips every `YOMIHON_*` entry, including future names,
  before external Git execution.
- **Retention / deletion / export / recipients:** retained for environment and
  process lifetime and according to provider account handling. Unset/stop
  locally and revoke/rotate at Google; no memory-zeroization or revocation API
  exists. It leaves only as the HTTPS `X-Goog-Api-Key` header to the fixed
  Google endpoint.
- **Logs / backups / derived copies:** excluded from application output, errors,
  fixtures, cache, and recordings. Shell profiles, launch/service configuration,
  and host backups may copy it externally. Only transient runtime/header copies
  exist in yomihon.
- **Owner / incident path:** provider-account operator. Immediately revoke or
  rotate, remove external configuration/history copies, inspect provider
  activity/billing, and report application disclosure privately.

## 7. Provider submissions and responses

- **Fields / purpose / authority:** eligible chunks formatted as
  `title: ... | text: ...`; fixed prefix plus bare semantic query; D57 fixed
  synthetic inputs; key header/dimension; returned vector/status/retry header.
  Structured filters stay local. D32/D50 authorize vector generation/search and
  D57 authorizes certification.
- **Source / trigger / storage / access:** sourced from current eligible vault
  bytes and explicit query, or fixed synthetic fixtures. Trigger is explicit
  build, bounded reconcile plus one explicit query, or developer certification.
  Encoded request/response stays transient in RAM; Google-side storage is
  external. Final send requires positive policy/source checks.
- **Retention / deletion / export / recipients:** yomihon intentionally persists
  no raw body/query. Provider retention/deletion is unasserted and no provider
  deletion API exists here. HTTPS recipient is
  `generativelanguage.googleapis.com`: production uses
  `gemini-embedding-2:embedContent`; D57 probe also uses `:countTokens`.
- **Logs / backups / derived copies:** submitted text/query, response body, and
  key are not app-logged or retained in errors; content-free reason/counts may
  reach stderr. No local raw-request backup; provider backups are unknown.
  Vectors, hashes, fingerprints, and rankings are derived copies.
- **Owner / incident path:** Koopa owns authorization/account/billing; Google
  owns its environment. Stop sends, rotate key if needed, identify affected
  paths/query, inspect provider controls, and treat external retention/deletion
  as an incident dependency.

## 8. Semantic generation store

- **Fields / purpose / authority:** model/dimension/protocol/chunker/vector
  identity; vault root; policy/source/corpus fingerprints and count; p95; paths;
  note/submission hashes; ordinals/vectors; active/previous/staging roles; retry
  count/time. This reuses paid vectors and publishes one complete derived
  generation; it is not truth.
- **Source / trigger / storage / access:** created from local eligible corpus,
  provider vectors, and retry/measurement state by explicit build or bounded
  reconcile. Plaintext SQLite lives at
  `<os.UserCacheDir()>/yomihon/semantic/generation.sqlite` with sidecars; its
  writer lease is the sibling `<os.UserCacheDir()>/yomihon/semantic.lock`.
  macOS/Linux directory is `0700`, files/lease `0600`.
  Semantic CLI and same OS account can access it; `serve` never opens it.
- **Retention / deletion / export / recipients:** no TTL; retain active,
  previous, and at most one staging generation, pruning older unreferenced rows.
  Explicit build may replace incompatible/corrupt state; operator may delete the
  cache. There is no secure-erase/product-delete command, server route, remote
  vector store, or app export. Local semantic process is the recipient.
- **Logs / backups / derived copies:** content-free state/count/retry/progress
  may reach output; paths/vectors/hashes/text are not normal progress. No app
  backup; host backup may copy plaintext. SQLite sidecars, RAM index/query
  vector/rankings, and build report are copies.
- **Owner / incident path:** Koopa. Stop semantic commands, preserve minimal
  evidence if needed, delete cache where appropriate, rebuild from reviewed
  vault/contract, and rotate key if process/environment access is suspected.

## 9. Logs, diagnostics, and command output

- **Fields / purpose / authority:** `slog` time/level; address and vault root;
  contract state; scan/render/report path/error; selected status path/from/to and
  changed state; fixed semantic reasons/counts/p95/progress; stdout results.
  These support local operation/recovery and stable agent contracts, not
  analytics.
- **Source / trigger / storage / access:** emitted at lifecycle events,
  warnings/errors, status recovery, semantic progress/failure, and explicit CLI
  output. Storage is stdout/stderr only; terminal, parent process, service,
  pipe, or redirect controls access.
- **Retention / deletion / export / recipients:** yomihon has no retention or
  purge setting and no network sink. External capture owns retention/deletion
  and may forward output; local operator/selected downstream consumer is the
  recipient.
- **Logs / backups / derived copies:** raw semantic query, provider body, key,
  and submitted chunk are prohibited. Paths/status remain sensitive metadata.
  Service/CI/terminal backups, screenshots, copied diagnostics, redirected
  files, and reports are external copies.
- **Owner / incident path:** capturing process/operator. Stop forwarding,
  restrict/redact/delete captures where possible, assess downstream/backups,
  rotate any exposed key, and use private reporting.

## 10. Fixed synthetic certification and recordings

- **Fields / purpose / authority:** fixed synthetic protocol/eval text and 40
  queries; synthetic IDs/paths; hashes; 1,536/3,072 vectors; identity; output
  path; counts. D57 certifies provider behavior without accepting arbitrary text
  or a real vault root.
- **Source / trigger / storage / access:** repository fixtures with synthetic
  markers, used only by explicit paid test opt-in plus operator key and, for
  capture, absolute output path/ruled dimension. Data lives in test RAM,
  repository fixtures/committed synthetic recording, or a `0600` local output.
  Evaluation is absent from the product command graph.
- **Retention / deletion / export / recipients:** committed data follows Git
  history; local output follows operator policy and manual deletion. Fixed
  synthetic inputs go to Google `embedContent`; protocol probe also calls
  `countTokens`. Committed synthetic vectors/hashes may reach source users.
- **Logs / backups / derived copies:** tests report counts/dimension/failures,
  not deliberate submitted text/vector/body/key. Repository and host/CI backups
  may retain data. Vector recording and content-free observer workload are
  derived copies.
- **Owner / incident path:** Koopa. If a non-synthetic byte crosses this
  boundary, stop, quarantine output, rotate key if relevant, remove/purge
  sensitive repository artifacts where feasible, assess provider copies, and
  treat the structural input lock as a security regression.

## Downstream and unsupported copies

- Operator-created real-vault evaluation artifacts, paired diffs, or agent
  reports remain vault-sensitive even when yomihon did not create them. D50
  requires real-vault evaluation to stay local; only content-free aggregates
  may be committed or quoted.
- A future remote database/vector service, proxy, hosted reader, shared key,
  telemetry sink, or cloud backup is absent and unauthorized. It requires an
  updated threat model, inventory, destination/retention/deletion ruling, and
  final-boundary evidence.
- Provider retention, training use, legal basis, residency, deletion, and backup
  behavior are **not established by this repository**, not silently assumed.

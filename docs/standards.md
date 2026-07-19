# Engineering standards (the bar an implementing session is held to)

This document is yomihon's repository-specific working protocol. The single
normative engineering source is `ENGINEERING_STANDARD.md` version 2.0 at the
digest in `ENGINEERING_STANDARD.sha256`; `PROJECT_PROFILE.md` resolves that
standard for this product. This file may make those requirements stricter or
more concrete, but it cannot waive them. A conflict is resolved by correcting
this file or by the standard's recorded exception process, never by silently
choosing the easier sentence. The four walls (`README`, decisions D02) remain
the authority for product purpose and permitted behavior; they do not turn a
missing engineering gate into PASS.

## 1. Working protocol

- **Docs are canon; conversations are not.** A ruling that lives only in a
  chat has not happened. Rulings land in `decisions.md`; sequencing and
  unbuilt-face design land in `roadmap.md`; per-face contracts land in that
  face's plan doc.
- **Plan doc before code** for every remaining face (B / H / D / G): author
  the plan doc per `roadmap.md` §5a, run one adversarial review round over it,
  get Koopa's approval, then build. `judge-plan.md` is the reference for what
  a plan doc owes: byte-exact contracts, testing strategy, a divergence or
  open-decision register, acceptance criteria.
- **Verification proves conformance, never authority.** Before dispatching code
  that combines independent predicates, enumerate their cross-product (for
  example role × resolution × target-instance state) and give every row an
  authority label: `REAL-OBSERVED`, `EXPLICIT-RULING`, `CANON-DERIVED`, or
  `NEEDS-RULING`. A `NEEDS-RULING` row stops implementation. Tests, fixtures,
  mutations, campaigns, and later documentation may prove that code follows a
  row; none may manufacture the row's authority. The owning canon lands before
  the first behavior commit, or a later correction is recorded honestly as an
  amendment with the missed row and prevention mechanism named.
- **Builder and acceptor are different contexts.** The implementer
  self-verifies (below), then an independent session re-verifies from scratch
  before anything is pushed: re-run the gates, re-read the diff, and re-run
  the kill-tests. Acceptance that only reads the builder's report is not
  acceptance.
- **Koopa presses the last key**: pushes, merges, and the retirement
  declarations are his decisions, always. Execution of a specific one may be
  delegated on his explicit instruction for that instance (push, PR-opening,
  and a local docs merge all have precedent); the guide never pushes or
  merges on its own initiative.

## 2. Testing bar

- **Every lock must be able to fail.** A test that gates a guarantee
  (loopback-only, drift, privacy drop, wire format) is proven by mutation:
  plant the defect it exists to catch, watch it go red, revert, and say so in
  the PR. A lock that has never been red is a hope, not a lock.
- **A lock is not done until CI can watch it fail.** A self-testing probe
  carries `MUTATE` modes, and they obey one contract. Every mode lives in the
  same structure that dispatches it, so `MUTATE=list` prints exactly the modes
  that exist. Every assertion names the **site** it guards; every mode names
  the site it aims at; the caught marker prints only when the aimed-at site is
  the one that fired — an unrelated assertion failing during a mutated run is a
  real regression, not a catch, and the run goes red without the marker. A
  probe refuses to start when a mode aims at a site that does not exist, or
  when a site has no mode aiming at it: an assertion nothing has tried to break
  is a lock nobody has watched fail. A mutation whose needle matches nothing
  prints `MUTATE-RESULT: not-applied <mode>` and exits **2**; a detection prints
  `MUTATE-RESULT: caught <mode>` and exits **1**. The `e2e-mutations` job runs
  every listed mode and requires exit 1 together with that exact marker line —
  never mere non-zero, because "the needle matched nothing" is also non-zero.
  A kill-test that dies then goes red the day it dies.
- **The environment wall is audited across the binary, not one directory.** The
  guard parses every non-test file the `yomihon` binary is built from — the
  command's own sources and those of every package it links, as the toolchain
  reports them — so a configuration read cannot hide one import away. A new
  environment key is therefore a deliberate edit to the wall test's allowlist,
  and never a quiet addition: read it in the `cmd/yomihon` wiring and pass it
  down as a field, never from inside `internal/`. B-lexical's embedding key
  arrives exactly this way.
- **Goldens are frozen bytes.** No `-update` flags, no regeneration paths in
  test code. A golden changes only when a ruling changes the contract, in its
  own commit, citing the ruling.
- **Invariants must be the real contract.** Before asserting a property
  (idempotence, symmetry, ordering), read the implementation and the register:
  some behaviors are deliberately "irregular" because the bytes are the
  contract (`stripTarget` is deliberately non-idempotent; scalar values are
  deliberately normalized). Asserting the aspirational property and then
  "fixing" the code to satisfy it is a wire-format change in disguise.
- **Seeds encode scars.** Every input class that ever produced a real defect
  (CRLF fences, `...` terminators, duplicate keys, merge keys, tagged and
  block scalars, U+2028/U+2029, 0x1F, NFD filenames, empty paths) appears as
  an explicit fuzz seed or fixture. Regression coverage is free after that:
  the ordinary test run replays all seeds.
- **Platform honesty.** Filesystem normalization differs (macOS folds NFC/NFD,
  Linux does not): a test that depends on folding must probe for it and skip
  cleanly, and a green local run is not a green CI run until CI says so.
- **Delete tests that cannot fail.** A test that restates the code, mocks
  everything it touches, or asserts a tautology is noise and gets removed,
  not maintained.

## 3. CI bar

- **Pin everything that executes.** Toolchain versions come from `go.mod`
  (`tool` directives) or an explicit pinned version; downloaded binaries are
  checksum-verified before they run. "latest" is forbidden in a workflow.
- **Jobs are failure classes.** A job's name answers "what broke?" in
  kebab-case nouns, and the names of one family read as one family:
  `assets-drift`, `lint-frontend`, `e2e-http`, `e2e-behavior`,
  `e2e-mutations`, `fuzz`. A name says what is asserted, not how the check is
  run — `-smoke` says only that the check is brief. An umbrella job named after
  the workflow itself (`ci` inside `CI`) says nothing and is a naming defect.
  The Go gate mirrors the local gate (`make verify`), and advisory scans that
  can turn red without a code change (vulnerability databases) live in their
  own job so they never block an unrelated PR ambiguously.
- **One workflow file while the jobs share pins.** Shared env (pinned
  versions, checksums) stays in one place; split files only when jobs stop
  sharing anything.
- **Permissions are minimal and explicit** (`contents: read`); every widening
  is a reviewed decision. Workflows carry a concurrency group that cancels
  superseded runs, and every job carries a timeout.
- **Node is development-only** (frontend linting and browser probes); the
  product build and runtime never acquire a Node step or dependency. The
  committed lockfile makes local and runner checks use the same dependency
  graph.
- **The reference binary never entered CI.** The conformance tests that
  compared against it were env-gated and skipped when the binary was absent —
  design, not a gap; they were deleted when the reference engine was declared
  retired (D43), and the goldens they backed remain the contract.

**The debts recorded against this bar on 2026-07-06 were paid in full the
next day (PR #21):** the umbrella job is now `verify` and mirrors the local
gate, the vulnerability scan runs in its own pinned `govulncheck` job, the
golangci-lint installer is checksum-verified before it runs, the workflow
carries a concurrency group and per-job timeouts, and the Makefile's Go
targets scope past a stray `node_modules/` tree. The Tailwind fetch step's
discipline — pinned version, checksum verified before execution — is now the
pattern every installer step follows.

## 4. Taste

- **Comments are self-contained** and state the durable reason in domain
  language. No decision numbers, no `docs/` paths, no plan-document names, no
  retired-project names, no scheduling vocabulary, no ALL-CAPS emphasis. The
  reader has this repository and nothing else.
- **Naming follows the Go style the repo already exhibits**: package-by-
  feature, no stutter (`judge.Finding`, never `judge.JudgeFinding`), no
  generic packages (`util`, `common`), constructors named `New`.
- **Generated files are never edited by hand** (`*_templ.go`,
  `assets/css/output.css`) — change the source, run the generator. Vendored
  and generated paths are marked in `.gitattributes` so language statistics,
  diffs, and search stay honest.
- **Zero text coupling to the retired references** in code, tests, strings,
  and env names; the docs that record the gates are the one exception and are
  rewritten at declaration time.
- **UI work climbs the interaction ladder (D41)**: semantic HTML, then CSS,
  then Baseline Web APIs, then a small vanilla-JS progressive enhancement only
  when a concrete need remains or the script is materially clearer than the
  native alternative. JavaScript is allowed, but it is not the default and may
  not take over a working no-JS core path. htmx, Alpine, and any other client
  abstraction require a concrete unmet need and D41's dependency-admission
  discussion before use. The app stays a server-rendered MPA; no
  client-framework runtime. Motion, loading, and transition polish are in-scope
  quality (`ux-plan.md`), held to the same ladder and to
  `prefers-reduced-motion`.
- **Web platform baseline (Koopa, 2026-07-08): the target is Baseline 2026.**
  Core UX prefers Baseline Widely-available features; Newly-available
  features are welcome as progressive enhancement; Limited-availability
  features require explicit justification and a fallback, recorded where
  they are used. The Web-API rung reads through this lens: a browser-specific
  API may enhance a core action, never carry it alone.

## 5. Verification protocol (before any push)

1. `make verify` is the one complete automated repository gate. It checks
   policy/profile drift, modules and generated projections, formatting, CSS,
   vet, curated golangci-lint, independent staticcheck/gosec/govulncheck,
   race tests, real-vault test compilation without reading a vault, the
   selected modernc bake-off path, workflow and shell syntax, builds,
   frontend/browser behavior, HTTP composition, fuzz smoke, watched-red
   mutations, six 64-bit cross-builds, benchmark smoke, licences/checksums,
   and deterministic source-artifact provenance. Every mandatory failure is
   non-zero. Formal Gate evidence reruns it on the immutable reviewed commit.
2. CI separately runs the retained mattn comparison because that rejected
   candidate requires CGO, and records focused jobs for readable failure
   ownership. Those jobs do not replace the canonical gate.
3. Credentialed or private-data checks remain explicit: `make provider-live`
   and `make test-real-vault` refuse to run without their opt-ins. Their
   applicability is decided by `PROJECT_PROFILE.md`, not by a silent skip.
4. Regeneration is a no-op: `make gen && make css` leaves the tree clean.
5. Kill-tests for every new lock (see §2), stated in the PR with the exact
   failure and restored-green result.
6. Hygiene greps over the changed files, all expected to come back empty
   (word-boundary patterns run under `grep -P` or the system grep — `git
   grep -E` silently drops `\b`, matching nothing and passing vacuously):
   - `grep -ri kura -- '*.go'` (zero coupling),
   - `grep -rnE '§|\bD[0-9]{1,3}\b|docs/|kurodo|[Ss]tage|[Pp]hase|\b(NEVER|MUST|ONLY|VERBATIM)\b'`
     over changed `.go`/`.templ`/workflow files (comment discipline; the
     ALL-CAPS class is checked case-sensitively),
   - `git status --porcelain` (no untracked residue, nothing harness-owned
     staged).
7. Commits: conventional type, English, lowercase imperative subject, body
   explains why, one logical change each, no attribution trailers, files
   staged by name.
8. PR bodies use `.github/PULL_REQUEST_TEMPLATE.md` and summarize scope,
   user impact, verification, watched-red evidence, and remaining decisions.
   Formal reviews required by the profile bind all three Gate verdicts and
   their evidence to an immutable commit or artifact in the review report.
   Gate 2 is performed through supported public surfaces by a person or agent
   that did not implement the change. Bot review comments are triaged
   against the real code — findings are either fixed or refuted line by line,
   never waved through.
9. **Every finding reaches one of three states before merge**: fixed in this
   PR, queued as a named unit in `program.md`, or refused with a written
   reason. "Fixed on another branch" is none of them — that is how a defect a
   reviewer handed over four days early still shipped. A refusal states the
   mechanism it refutes; a finding whose mechanism is wrong can still carry a
   point worth taking, and the reply says so.
10. **A too-large notice from the review bot is answered, not absorbed.** A PR
   the bot declines to read has not been reviewed. Trigger the review by hand,
   or split the PR. Silence from a reviewer is not a passing verdict, and four
   PRs in this repository's history were never read because nobody noticed the
   notice.

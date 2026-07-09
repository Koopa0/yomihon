# Engineering standards (the bar an implementing session is held to)

This document exists because the sessions that built the codebase so far will
not be the sessions that finish it. Everything an implementer needs in order
to produce work at the required standard must be readable from this
repository — this file is the enforceable definition of "done well". Where a
rule here conflicts with convenience, the rule wins; where it conflicts with
the four walls (`README`, decisions D02), the walls win.

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
  kebab-case nouns: `assets-drift`, `frontend-lint`, `e2e-smoke`,
  `fuzz-smoke`. An umbrella job named after the workflow itself (`ci` inside
  `CI`) says nothing and is a naming defect. The Go gate mirrors the local
  gate (`make verify`), and advisory scans that can turn red without a code
  change (vulnerability databases) live in their own job so they never block
  an unrelated PR ambiguously.
- **One workflow file while the jobs share pins.** Shared env (pinned
  versions, checksums) stays in one place; split files only when jobs stop
  sharing anything.
- **Permissions are minimal and explicit** (`contents: read`); every widening
  is a reviewed decision. Workflows carry a concurrency group that cancels
  superseded runs, and every job carries a timeout.
- **Node lives only on the runner** (frontend linting, future screenshot
  e2e); the product build never acquires a Node step.
- **The reference binary never entered CI.** The conformance tests that
  compared against it were env-gated and skipped when the binary was absent —
  design, not a gap; they were deleted when the reference engine was declared
  retired (D43), and the goldens they backed remain the contract.

**Known debts against this bar (recorded 2026-07-06, queued for one dedicated
chore PR — do not copy these patterns while they remain):** the umbrella `ci`
job (rename to `verify`, mirroring the local gate, with the vulnerability
scan split into its own job); `govulncheck@latest` (pin it); the
golangci-lint installer fetched from a moving ref and piped to `sh` without a
checksum (pin the ref; verify what runs); no concurrency group and no job
timeouts. Until that PR lands, the Tailwind fetch step — pinned version,
checksum verified before execution — is the exemplar to copy, not the
installer steps above it.

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
  then Chromium-native Web APIs, then a small vanilla-JS enhancement, then —
  when it genuinely earns its place — a mature, vendorable library admitted
  per D41's criteria and recorded as a decision. The app stays a
  server-rendered MPA; no client-framework runtime. Motion, loading, and
  transition polish are in-scope quality (`ux-plan.md`), held to the same
  ladder and to `prefers-reduced-motion`.
- **Web platform baseline (Koopa, 2026-07-08): the target is Baseline 2026.**
  Core UX prefers Baseline Widely-available features; Newly-available
  features are welcome as progressive enhancement; Limited-availability
  features require explicit justification and a fallback, recorded where
  they are used. The ladder's Chromium-native rung reads through this lens:
  a Chromium-only API may enhance a core action, never carry it alone.

## 5. Verification protocol (before any push)

1. `make verify` — fmt, vet, lint (zero issues), test, build.
2. Regeneration is a no-op: `make gen && make css` leaves the tree clean.
3. Kill-tests for every new lock (see §2), stated in the PR.
4. Hygiene greps over the changed files, all expected to come back empty
   (word-boundary patterns run under `grep -P` or the system grep — `git
   grep -E` silently drops `\b`, matching nothing and passing vacuously):
   - `grep -ri kura -- '*.go'` (zero coupling),
   - `grep -rnE '§|\bD[0-9]{1,3}\b|docs/|kurodo|[Ss]tage|[Pp]hase|\b(NEVER|MUST|ONLY|VERBATIM)\b'`
     over changed `.go`/`.templ`/workflow files (comment discipline; the
     ALL-CAPS class is checked case-sensitively),
   - `git status --porcelain` (no untracked residue, nothing harness-owned
     staged).
5. Commits: conventional type, English, lowercase imperative subject, body
   explains why, one logical change each, no attribution trailers, files
   staged by name.
6. PR bodies carry a summary and a test plan, including which kill-tests ran
   and what is verified versus assumed. Bot review comments are triaged
   against the real code — findings are either fixed or refuted line by line,
   never waved through.

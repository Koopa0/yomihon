# Engineering Acceptance Review

Review ID: `REV-YYYY-NNN`  
Date: YYYY-MM-DD  
Reviewer: @independent-reviewer  
Builder: @builder  
Standard: `ENGINEERING_STANDARD.md` version 2.0  
Review class: verification-fixture / release-candidate  
Project profile revision: commit or digest

# Verdict

Status: `GO / ACCEPT-WITH-GATES / NO-GO`

One-sentence rationale:

# Snapshot

| Item | Value | Status |
|---|---|---|
| Commit | | verified / unverified |
| Artifact digest | sha256:... | verified / N/A / unverified |
| Worktree | clean / described | |
| Generated state | | |
| Dependency lock state | | |
| Go version | | |
| OS / architecture | | |
| CI run | | |
| Active exceptions | | |

Scope explicitly reviewed: source-artifact-contract / complete-project-profile

Scope explicitly not certified:

# User need, scope, and risk

```text
User or operator need:
Intended outcome:
Non-goals:
Risk class:
Capability flags affected:
Public contracts affected:
Durable contracts affected:
Security and privacy boundaries affected:
Operational boundaries affected:
```

# Concept model

| Field | Finding | Status | Authority / evidence |
|---|---|---|---|
| Concept / non-concept | | | |
| Semantic owner | | | |
| Source of truth | | | |
| Captured capabilities | | | |
| Projections / caches | | | |
| Legal / impossible states | | | |
| Irreversible effects | | | |
| Load-bearing invariant | | | |

# Initial objections

1.
2.
3.

# Authority ledger

| Behavior or rule | Authority class | Source | Status | Notes |
|---|---|---|---|---|
| | | | | |

# Findings

| ID | Severity | Status | Disposition | Location / surface | Defect | Concrete failure scenario | Required action | Owner |
|---|---|---|---|---|---|---|---|---|
| F-001 | | verified / inferred | CLOSED / OPEN | | | | | |

# Profile blocker closure

Every formal release report lists exactly the machine-profile
`post-artifact-blockers` it closes. A verification fixture uses one `None`
row and `closed-profile-blockers: none`; prose or a generic GO cannot close an
unlisted profile obligation.

| Blocker ID | Disposition | Evidence | Owner |
|---|---|---|---|
| Profile blocker ID / None | CLOSED / N/A | | |

# State, failure, and event analysis

| Input / state / interleaving | Result shape | Status / exit | Fault owner | Retry / duplicate | Side effects | Recovery | Evidence |
|---|---|---|---|---|---|---|---|
| | | | | | | | |

Review cancellation, response loss after commit, authority change in flight,
stale publication, competing terminal actions, crash boundaries, and staging
visibility wherever the change makes them reachable.

# Gate 1 — Architecture and open-source engineering quality

Status: `PASS / PARTIAL / FAIL`

- Package and dependency ownership:
- Go API and implementation quality:
- Public, wire, storage, and configuration contracts:
- Security, privacy, data lifecycle, and supply chain:
- Documentation, licensing, release, and maintainability:
- Blockers: none / describe blockers

# Gate 2 — Real-user and third-party-agent usability

Status: `PASS / PARTIAL / FAIL`

For a release candidate, record two cold sessions that are distinct from each
other and independent of the builder. Record operator identity and session ID
as separate fields so appending a session suffix cannot disguise a builder as
an acceptance operator. One non-builder operator may run both sessions only
when the session IDs and retained artifacts are distinct; a combined artifact
is acceptable only when it identifies both session records separately. A
formal release envelope uses two distinct `sha256:` identities for those
records; URL spelling is not a content identity. Builder, reviewer, and
operator values are atomic identity tokens: an optional leading `@`, one ASCII
letter or digit, then at most 127 ASCII letters, digits, `.`, `_`, or `-`. They
are not display names or lists. Session IDs use the same token grammar without
`@`. This syntax prevents
compound identities from bypassing comparison; it does not replace the human
reviewer's obligation to verify who controlled each named identity.

Cold session 1 operator:  
Cold session 1 session ID:  
Cold session 2 operator:  
Cold session 2 session ID:  
Both sessions independent from implementation: yes / no  
Both sessions directly executed the named public surfaces: yes / no  
Supported public surfaces used:  
Environment:

| Scenario | Steps | Expected | Observed | Friction / ambiguity | Artifact | Status |
|---|---|---|---|---|---|---|
| Clean first use | | | | | | |
| Experienced use | | | | | | |
| Missing / malformed configuration | | | | | | |
| Invalid / ambiguous input | | | | | | |
| Offline / provider failure | | | | | | |
| Partial / stale result | | | | | | |
| Cancellation / retry / recovery | | | | | | |
| Privacy-sensitive operation | | | | | | |
| Non-ASCII / long / boundary input | | | | | | |
| Supported / unsupported platforms | | | | | | |
| Keyboard / narrow / no-JS | | | | | | |
| Upgrade / restart | | | | | | |
| Destructive action / recovery | | | | | | |

Blockers: none / describe blockers

# Gate 3 — Test and evidence-system quality

Status: `PASS / PARTIAL / FAIL`

| Claim | Production entry point and composition crossed | Real dependency / format | Test class | Gap |
|---|---|---|---|---|
| | | | | |

| Test class | Required | Command / target | Result | Quality assessment |
|---|---|---|---|---|
| Unit / integration / E2E | | | | |
| Golden / data-driven | | | | |
| Fuzz / property | | | | |
| Race / deterministic concurrency | | | | |
| Browser | | | | |
| Migration / compatibility | | | | |
| Fault / robustness | | | | |
| Benchmark / load | | | | |
| Cross-platform | | | | |

## Watched-red evidence

| Invariant | Mutation | Expected failing check | Observed red | Failure names contract | Restored green | Artifact |
|---|---|---|---|---|---|---|
| | | | | | | |

Blockers: none / describe blockers

# Security, privacy, performance, and operations

| Boundary / workload | Authority / budget | Final enforcement / measurement | Bypass or fault search | Exact evidence | Status |
|---|---|---|---|---|---|
| Network egress | | | | | |
| Credential use | | | | | |
| Personal-data storage | | | | | |
| Logs / diagnostics | | | | | |
| Files / subprocesses | | | | | |
| Performance / capacity | | | | | |
| Recovery / rollback | | | | | |

# Compatibility and release

```text
Public API impact:
CLI / machine-output impact:
Wire impact:
Storage / schema impact:
Configuration impact:
Supported upgrade path:
Downgrade behavior:
Artifact identity:
Release notes:
Post-release verification:
```

# Contradiction hunt

| Surface pair | Question | Result | Status |
|---|---|---|---|
| Canon ↔ code | Does behavior still match authority? | | |
| Code ↔ package ownership | Is the rule implemented once? | | |
| Code ↔ schema / wire / CLI / UI | Do names and invariants agree? | | |
| Code ↔ logs / metrics / tests | Do observed states and evidence agree? | | |
| Docs ↔ observed use | Can a stranger complete and recover? | | |
| Fix ↔ unrelated behavior | Did the repair open another contradiction? | | |

# NEEDS-OWNER, exceptions, and blocked evidence

| Item | Status | Why | Options / mitigation | Owner / next gate |
|---|---|---|---|---|
| | UNRESOLVED / DEFERRED-BY-EXCEPTION / UNVERIFIED / BLOCKED | | | |

# Verification log

| Command | Environment | Result | Artifact / excerpt |
|---|---|---|---|
| | | | |

# Independent reviewer certification

```text
Reviewer:
Snapshot:
Date:
I directly inspected or executed the evidence marked verified:
Scope certified: source-artifact-contract / complete-project-profile
Scope explicitly not certified: none / describe exclusions
Gate 1:
Gate 2:
Gate 3:
Final verdict:
```

# Release evidence envelope

Complete this block only when the report is the `REVIEW_EVIDENCE` input to a
source release. The independent reviewer owns this certification, and its
values must match the human Verdict and Gate sections above. Values are
byte-exact and must describe the same immutable candidate; the artifact checker
requires every canonical section to contain completed data or an explicit N/A,
and rejects a self-reviewer, contradictory section statuses, non-PASS Gates,
active exceptions, unresolved evidence, or any finding not explicitly marked
`CLOSED`. These structural checks do not
judge the intellectual quality of the review. The bundle includes this
public-safe report plus a minimal certificate that records its SHA-256.
Evidence references must be public-safe `https://` URLs or `sha256:<64 lowercase
hex characters>` identities for retained evidence whose location and custody are
explained in the human sections above. A digest identifies evidence; it does not
make inaccessible evidence independently inspectable.

`review-class` and `certified-scope` are a machine boundary, not prose labels.
A `verification-fixture` may certify only `source-artifact-contract`; a formal
tagged release requires `release-candidate` plus `complete-project-profile` and
may not exclude any project-profile release scope. Its Snapshot artifact digest
must identify the exact quarantined source archive that the final assembler
rebuilds and byte-matches; `N/A-before-bundle-build` is not a release claim.

```text
reviewed-commit: replace-with-40-character-commit
project-profile-sha256: replace-with-64-lowercase-hex
review-class: verification-fixture-or-release-candidate
certified-scope: source-artifact-contract-or-complete-project-profile
closed-profile-blockers: none-or-exact-comma-separated-profile-blocker-IDs
builder: replace-with-builder-identity
independent-reviewer: replace-with-independent-reviewer-identity
gate-1: PASS
gate-2: PASS
gate-3: PASS
final-verdict: GO
unverified-or-blocked-checks: none
unresolved-owner-decisions: none
active-exceptions: none
verification-command: make verify
verification-result: PASS
verification-environment: replace-with-os-architecture-go-and-tool-versions
verification-artifact: replace-with-https-url-or-sha256
ci-run: replace-with-https-url
gate-2-session-1-artifact: replace-with-sha256-for-formal-release
gate-2-session-2-artifact: replace-with-different-sha256-for-formal-release
watched-red-artifact: replace-with-https-url-or-sha256
independent-certification: complete
```

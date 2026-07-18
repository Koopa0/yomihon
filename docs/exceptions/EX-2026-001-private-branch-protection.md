# Engineering Standard Exception

Exception ID: `EX-2026-001`  
Status: Proposed  
Created: 2026-07-18  
Owner: @Koopa0  
Independent approver: not yet assigned

## Clauses and scope

- Standard clauses: §6, §18, §21, §22.8
- Repository paths / components: GitHub `main` branch and repository rules
- Versions / commits / environments: private `Koopa0/yomihon` repository on
  the GitHub plan active on 2026-07-18
- Merge, release, or production scope affected: every merge; public release
  and production claims remain blocked

## Why compliance is not currently possible

Read-only calls to both the branch-protection and repository-rulesets APIs
returned HTTP 403 with the message “Upgrade to GitHub Pro or make this
repository public to enable this feature.” The repository must not be made
public early merely to turn on the control intended to protect publication.

## Alternatives considered

| Alternative | Why rejected | Evidence |
|---|---|---|
| Make the repository public now | Violates the release policy by exposing an uncertified product | `docs/release.md` |
| Upgrade the account immediately | A commercial account change is Koopa's decision, not a repository implementation step | GitHub API response |
| Treat CI and CODEOWNERS as enforcement | They are advisory without protected-branch enforcement and can be bypassed by a direct push | GitHub protection model |
| Add a custom merge proxy | Adds credentials and a new control plane while still not preventing owner-side direct pushes | Architecture review |

## Concrete risk

```text
Failure scenario: a direct push or merge bypasses required CI, three-gate review, or immutable-snapshot evidence.
Affected users / operators: Koopa now; future source users if an uncertified snapshot becomes public.
Security / privacy / durability impact: an unreviewed egress, write, or publication regression can reach main.
Compatibility impact: a frozen CLI or storage contract can change without its required review.
Operational impact: main may no longer be a reviewable release candidate.
Worst credible consequence: private vault data is sent or mutated outside the authorized boundary and the bypass is presented as reviewed.
```

## Mitigation and compensating evidence

- Containment: agents do not push without Koopa's explicit instance-specific
  instruction; all candidate work uses pull requests and the tracked PR
  evidence envelope.
- Monitoring / detection: CI runs on pull requests and pushes to `main`; Koopa
  reviews the head commit and required job results before merge.
- Additional tests or manual certification: the `pr-policy` job validates the
  three Gate fields, immutable commit, reviewer separation, unresolved checks,
  and approved exception references.
- Rollback / disable path: revert the unauthorized commit and invalidate every
  certificate bound to it; do not publish or tag that snapshot.
- User-facing limitation or disclosure: no public release or production-ready
  claim while this exception remains open.

## Ownership and closure

```text
Implementation owner: @Koopa0
Risk owner: @Koopa0
Expiry date or objective review trigger: immediately before public visibility, or before any additional writer receives access, whichever comes first
Closure condition: enable protected main or a repository ruleset with the settings in docs/merge-policy.md, verify them through the API, and remove this exception from the active profile table
Tracking issue / decision: this record
```

## Gate effect

- Gate 1: cannot be PASS until this proposal is independently approved; an
  approved record permits at most ACCEPT-WITH-GATES for a private-repository
  merge.
- Gate 2: unaffected for product-surface evidence.
- Gate 3: CI evidence remains valid for the reviewed commit, but bypass
  prevention is not mechanically complete.
- Maximum permitted verdict while open: `ACCEPT-WITH-GATES` for merge;
  `NO-GO` for release-ready and production-ready.

## Approval

```text
Approved / rejected: pending
Approver: not yet assigned
Date: pending
Snapshot: pending
Reason: pending
```

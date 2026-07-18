# Merge and branch-protection policy

This document defines the repository-side enforcement target. It does not
claim that GitHub currently enforces settings that its API refuses to expose.
The product and acceptance authorities are `ENGINEERING_STANDARD.md` and
`PROJECT_PROFILE.md`.

## Pull requests

Every merge to `main` uses a pull request bound to its current 40-character
head commit. The pull request records separate Gate 1, Gate 2, and Gate 3
verdicts, the independent reviewer, the evidence location, unresolved or
blocked checks, and approved exception IDs. The `pr-policy` check rejects a
missing or stale commit identity, a non-PASS gate, a non-GO final verdict, a
builder acting as the independent reviewer, an unverified or unresolved item,
or an exception that is not recorded as approved.

The check validates the evidence envelope; it cannot decide whether the
review was intellectually independent or competent. The CODEOWNER reviews
that fact and the cited evidence. Gate 2 evidence must come from a person or
agent that did not implement the change and must use supported public
surfaces.

## Target protection for `main`

Before public release, `main` is to be protected with all of these settings:

- pull requests required; direct pushes and force pushes refused;
- one approving review and CODEOWNER review required;
- stale approvals dismissed when the head commit changes;
- all review conversations resolved;
- required status checks current with the head commit, including `pr-policy`
  and every mandatory job named by `PROJECT_PROFILE.md`;
- administrators included; deletion refused; linear history required.

GitHub's API returned HTTP 403 on 2026-07-18 for both branch protection and
repository rulesets because this repository is private on the current plan.
That is an external enforcement gap, not an N/A capability. It is governed by
`EX-2026-001` and must close before the repository becomes public or another
writer receives access, whichever happens first.

## Readiness claims

- **Merge-ready** is a claim about one immutable change and the merge gates in
  the profile. It does not authorize a release.
- **Release-ready** additionally requires the release, compatibility,
  independent-use, privacy, licence, documentation, and source-artifact gates
  in `docs/release.md`.
- **Production-ready** additionally requires the operator's real vault,
  credential, provider, platform, recovery, and post-start checks. A source
  release cannot certify another user's environment.

`UNVERIFIED`, `BLOCKED`, `UNRESOLVED`, a moving worktree, or an unapproved or
expired exception cannot be reported as PASS.

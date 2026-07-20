# Merge and branch-protection policy

This document defines the repository-side enforcement target. It does not
claim that GitHub currently enforces settings that its API refuses to expose.
The product and acceptance authorities are `ENGINEERING_STANDARD.md` and
`PROJECT_PROFILE.md`.

## Pull requests

Every merge to `main` uses a pull request. CI owns reproducible facts such as
builds, lint, generated drift, tests, platform contracts, vulnerability checks,
and artifact integrity. It does not parse pull-request prose to decide whether
a reviewer was independent, a Gate passed, or an exception was approved.

Maintainers own those judgments. Before merge they review the current head,
confirm every applicable CI job passed, resolve review conversations, and
inspect the evidence required by `PROJECT_PROFILE.md`. A formal R2/R3,
public-surface, final-boundary, or release review uses
`docs/reviews/REVIEW_REPORT.template.md` and binds its three Gate verdicts to
the exact commit or artifact. Gate 2 evidence must come from a person or agent
that did not implement the change and must use supported public surfaces.

The merge decision requires no unresolved blocker in the change's applicable
scope and no active exception that forbids the merge. Merging a change does
not by itself establish release-ready or production-ready status.

## Target protection for `main`

Before public release, `main` is to be protected with all of these settings:

- pull requests required; direct pushes and force pushes refused;
- one approving review and CODEOWNER review required;
- stale approvals dismissed when the head commit changes;
- all review conversations resolved;
- required technical status checks current with the head commit, as named by
  `PROJECT_PROFILE.md`;
- administrators included; deletion refused; linear history required.

GitHub's API returned HTTP 403 on 2026-07-18 for both branch protection and
repository rulesets because this repository is private on the current plan.
In the current owner-only repository, Koopa manually confirms the current
head, technical checks, and required review before merge. Protected `main`
must be enabled before the repository becomes public or another writer
receives access, whichever happens first. This is a platform limitation and
publication precondition, not evidence supplied by CI.

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

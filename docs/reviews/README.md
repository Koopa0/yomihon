# Engineering review records

Formal reviews use `REVIEW_REPORT.template.md` and are bound to an immutable
40-character commit or an artifact digest. Use one record per reviewed
snapshot; a later code or policy change invalidates it unless the reviewer
explicitly certifies that the changed scope is irrelevant.

The builder records self-review and watched-red evidence, but cannot issue the
independent Gate 2 certificate or the final R3 verdict. Reviewers mark every
check PASS, FAIL, N/A, UNVERIFIED, or BLOCKED. `UNVERIFIED`, `BLOCKED`,
`UNRESOLVED`, a moving worktree, and an unapproved or expired exception are
never PASS.

Evidence containing vault paths, note text, queries, credentials, semantic
vectors, or environment dumps does not belong in this repository. Retain a
privacy-safe transcript, exact command, result, mutation, tool versions, and
artifact digest instead.

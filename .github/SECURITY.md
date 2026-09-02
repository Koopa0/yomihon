# Security policy

## Supported versions

No releases yet. Fixes land on `main`; older snapshots are not maintained.

## Reporting a vulnerability

Report privately through [GitHub's private advisory form](https://github.com/koopa0/yomihon/security/advisories/new), never a public issue, and attach no vault content.
yomihon is a single-user local reader: it listens only on `127.0.0.1`, makes no
outbound network call, and writes one frontmatter field, so anything that
writes outside `status`, or leaves the machine, is worth reporting.

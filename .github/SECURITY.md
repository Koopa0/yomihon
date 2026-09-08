# Security policy

## Supported versions

Report against `main` or the latest release — what `go install …@latest` gives
you. Fixes land on `main`; earlier releases are not maintained.

## Reporting a vulnerability

Report privately through [GitHub's private advisory form](https://github.com/koopa0/yomihon/security/advisories/new), never a public issue, and attach no vault content.
yomihon is a single-user local reader: it listens only on `127.0.0.1`, makes no
outbound network call, and writes one frontmatter field, so anything that
writes outside `status`, or leaves the machine, is worth reporting.

## Verifying a downloaded binary

A release binary carries a signed statement of where it came from. To check
that the file you downloaded was built by this repository's release workflow
from a specific commit, run:

```bash
gh attestation verify yomihon_v0.2.0_darwin_arm64 --repo koopa0/yomihon
```

The command prints the commit and the workflow that produced the binary, and
fails if the file was rebuilt or replaced anywhere between that run and your
disk. `SHA256SUMS` answers a narrower question — whether the download arrived
intact — and it is served from the same page as the binary, so it cannot tell
you the binary is the one this repository built.

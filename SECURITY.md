# Security policy

## Supported versions

yomihon has not published a public release. Security fixes are made on the
latest `main`; older snapshots are not maintained. After source-only `v0.x`
publication begins, only the latest release line and `main` will be considered
for fixes unless a release note says otherwise. See the
[release policy](docs/release.md).

## Reporting a vulnerability

Use GitHub's private vulnerability reporting for this repository when it is
available. If that private form is unavailable, open a public issue asking the
maintainer to enable a private reporting channel, but do not describe the
vulnerability in that issue.

Never attach or paste:

- vault files or excerpts;
- semantic SQLite generations or vector data;
- search queries or logs containing private material;
- API keys, credentials, or environment dumps.

Please include the affected revision, operating system, impact, and minimal
reproduction steps once a private channel is established. Do not test against
systems or data you do not own.

## Security boundary

yomihon is a single-user local application. Its HTTP server is intentionally
bound to loopback and has no authentication; exposing it through a proxy,
tunnel, container port, or widened listener is outside the supported threat
model. The vault remains the source of truth. Semantic indexes are disposable
local derived data, and embedding API access uses the operator's own key.

The ordinary reading server and browser search do not contact the embedding
provider. An explicit `search-index build` sends only contract-eligible
instance-note chunks; an explicit `search --semantic` sends its bare query text
once. Contract-declared private paths are excluded before their bytes are
opened for either agent-facing flow. The provider credential is read only at
the final semantic gate, sent only in the request header, and is never written
to the generation store, logs, diagnostics, or fixtures. Provider account
terms, retention, and billing remain the operator's responsibility.

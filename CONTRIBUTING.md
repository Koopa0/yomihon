# Contributing

yomihon is a small, local-first Go program. Changes should preserve the
product's narrow trust boundaries and the repository's package-by-feature
design; a green linter is a minimum gate, not a substitute for reviewing the
meaning of an API or package.

## Prerequisites

- Go 1.26.5 or newer
- Git
- Node.js 22.23.1 for the versioned frontend and browser checks
- a locally installed stable Chrome for browser acceptance
- ShellCheck 0.11.0 for workflow and E2E shell checks
- the Tailwind CSS standalone CLI v4.1.17 for stylesheet drift verification
  and rebuilding `assets/css/output.css` (CI pins both the version and artifact
  checksum)

Install the pinned Go analysis and generation tools into `GOBIN`:

```sh
make tools
```

Install the locked development-only JavaScript tools when changing CSS,
JavaScript, or browser probes:

```sh
npm ci --prefix .github --ignore-scripts --no-audit --fund=false
```

Node.js is not a runtime or product-build dependency.

## Before changing code

Start with the tracked engineering authority in `ENGINEERING_STANDARD.md` and
`PROJECT_PROFILE.md`, then the product canon in `docs/product.md`,
`docs/design.md`, `docs/decisions.md`, and `docs/standards.md`. Search, graph,
and rendering changes also require `docs/vault-model.md`. Product behavior
belongs in those canonical documents before an implementation silently
chooses it. Maintainers may use additional local agent harnesses, but a clean
source checkout never depends on untracked instructions to build, test, or
review a contribution.

Keep packages named for the domain concept they own. Avoid generic buckets,
layered `service`/`repository` directories, framework-shaped interfaces, and
types named only for their plumbing role. Names should become shorter at the
use site, as in `search.Query` and `semantic.Indexer`.

## Verification

The root Go gate is:

```sh
make verify
```

It checks deterministic repository integrity, module and generated-projection
drift, Go and web formatting/linting, `go vet`, staticcheck, gosec,
govulncheck, race tests, HTTP and browser behavior, fuzz smoke, watched-red
mutations, portable builds, licence and dependency integrity, performance
smoke, and deterministic source-artifact provenance. The vulnerability
database is time-varying, so a result records its execution time even though
it remains a mandatory non-zero stage. The retained mattn comparison requires
CGO and is gated separately in CI with `make tools-check-mattn`; it is not a
product dependency.

During focused frontend iteration, run the narrower target:

```sh
make frontend-check
```

For template or stylesheet changes, regenerate the committed outputs:

```sh
make gen
make css
```

Generated `*_templ.go`, sqlc output, and `assets/css/output.css` are never
edited by hand. Every test that protects a security, privacy, wire-format, or
availability invariant must be observed failing under the defect it claims to
catch before the change is considered complete.

For a performance claim, collect the before and after samples on the same
machine and Go toolchain:

```sh
make bench-baseline
# apply the candidate change
make bench-compare
```

Both targets collect ten benchmark samples. Only the resulting `benchstat`
comparison supports a performance claim; one-off timings and shared-CI
absolute values do not.

## Pull requests

Keep one coherent change per pull request and use the repository pull-request
template. Describe the user-visible or architectural reason, the exact
verification run, and every watched-red mutation. Formal reviews required by
`PROJECT_PROFILE.md` use `docs/reviews/REVIEW_REPORT.template.md`; Gate 2 must
be performed through a supported public surface by a reviewer who did not
implement the change. CI does not decide whether those human review Gates
passed. `UNVERIFIED`, `BLOCKED`, or `UNRESOLVED` is never PASS, and deviations
require an approved record under `docs/exceptions/`.
Do not include vault content, semantic database generations, query text, logs
that may contain private material, or API keys in commits, issues, fixtures, or
pull requests.

## Compatibility and releases

Yomihon is preparing for source-only `v0.x` releases. A pre-v1 tag does not
promise stability for the Go package layout or exported Go APIs. It also does
not permit casual changes to frozen agent output: JSON/JSONL bytes, field order,
exit codes, and reason strings identified as contracts in the plan documents
must keep their golden evidence or go through an explicit product ruling.

The publication checklist and platform boundary live in
[docs/release.md](docs/release.md). A merged change may be complete without
making the repository ready for `v0.1.0`; do not describe a release as ready
until every gate in that document is backed by current evidence.

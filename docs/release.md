# Release policy

Status: **publication policy, not a release announcement**. Yomihon has not
published `v0.1.0` or any other public version.

## Distribution

The first public versions will be MIT-licensed, source-only `v0.x` releases.
They will be tagged with semantic versions, but the project will not publish
prebuilt binaries, installers, containers, or a hosted service in that phase.
Users build the program with Go 1.26.5 or newer.

The lexical reader, judge, and ordinary search surfaces target macOS, Linux,
and Windows. The status write face and current optional semantic-generation
store target macOS and Linux; on Windows the reader remains usable while both
features refuse before their first write. Those platform boundaries do not
imply a `v1.0.0` product release. Semantic search is
bring-your-own-key: every user supplies and pays for their own
embedding-provider account. Yomihon operates no shared key or proxy.

## Compatibility

Before `v1.0.0`, the Go package layout and exported Go APIs may change without
a compatibility shim. Most implementation packages are under `internal/` and
are not a public library surface.

That pre-v1 latitude does **not** relax agent contracts. JSON/JSONL fields,
field order where frozen, exit codes, reason strings, and other byte-level CLI
contracts identified as frozen in the canonical plan documents remain stable
until an explicit product ruling changes them. Any such change must be called
out as a breaking agent-contract change in the release notes and changelog;
incrementing a `v0.x` version alone is not permission to change those bytes.

## Gate for the first public release

Koopa makes the publication decision only after all of the following evidence
exists on the release candidate:

1. The full repository verification chain and required GitHub Actions jobs pass
   on Linux, macOS, and Windows for the surfaces supported on each platform.
2. The repository contains no vault material, API key, environment dump,
   semantic generation, or query-bearing log/fixture. The private vulnerability
   reporting path is enabled and tested without publishing a vulnerability.
3. The MIT license and third-party notices cover every redistributed asset;
   release notes and a changelog describe the exact candidate rather than a
   later working tree.
4. At least two cold, independent agent sessions exercise the product as
   third-party users of an Obsidian fixture vault rather than as builders.
   The release report records each operator and session ID in separate fields;
   a session suffix cannot turn a builder identity into independent evidence.
   Builder, reviewer, and operator fields each contain one canonical identity
   token, never a display name or list; session IDs are separate canonical
   tokens. Structural comparison does not attest who controls a token, so the
   independent reviewer must still verify that identity.
   Its formal envelope binds the two retained session records by distinct
   SHA-256 identities, not by URL spelling.
   Their scenario set covers setup and reading, lexical discovery, diagnostics,
   the frozen agent CLI, BYOK semantic search, privacy refusal,
   offline/provider failure, and recovery from invalid local state. Findings
   are fixed, explicitly deferred, or rejected with evidence.
5. The README contains at least one current, privacy-safe screenshot captured
   from the real application against a reviewable fixture. It must not be a
   mockup or a generated substitute for product evidence.
6. The [brand brief](brand.md) grounds the identity in the product rather than a
   generic book, search, sparkle, or Japanese-glyph motif. A human-selected
   direction has then been drawn as a deterministic vector mark, reduced into
   a favicon from the same geometry, checked at 16, 24, and 32 CSS pixels in
   monochrome and on light and dark browser surfaces, served locally as SVG,
   and referenced from the document head. No image-generation output is the
   logo or favicon source. No manifest, install flow, or PWA scope is implied.

The brand mark and the real product screenshot are separate evidence: neither
substitutes for the other. A banner remains optional and is not part of the
favicon decision; the professional no-banner presentation remains a valid
outcome. Until these gates close, development commits may be pushed and
reviewed without presenting the project as released.

## Source artifact and provenance

The release owner creates an annotated semantic-version candidate tag only
after the approved profile says merge and artifact-build readiness are GO,
artifact-build blockers are `none`, and the checkout is clean at that commit.
That is eligibility to create evidence, not release GO. First prepare the exact
archive in quarantine. The certification-grade entry point is loaded directly
from the immutable commit; a working-tree Makefile or shell script is not an
execution authority:

```sh
RELEASE_VERSION=v0.1.0
SOURCE_COMMIT=<40-character-tagged-commit>
SOURCE_ARCHIVE="dist/candidates/yomihon-${RELEASE_VERSION}.tar.gz"
BOOTSTRAP=$(mktemp "${TMPDIR:-/tmp}/yomihon-source-artifact-bootstrap.XXXXXX")
trap 'rm -f "$BOOTSTRAP"' 0 HUP INT TERM
GIT_NO_REPLACE_OBJECTS=1 git show "${SOURCE_COMMIT}:tools/source-artifact-bootstrap.sh" >"$BOOTSTRAP"
sh "$BOOTSTRAP" --prepare-archive "$RELEASE_VERSION" "$SOURCE_COMMIT" "$SOURCE_ARCHIVE"
```

`make source-archive-candidate` is a checked convenience wrapper around this
sequence, not evidence that its working-tree Makefile was the committed one.

The command prints the archive SHA-256. An independent reviewer inspects that
archive and the tagged source, executes the three Gates, and records the exact
`sha256:<digest>` as the verified Snapshot artifact. Only then is the final
candidate bundle assembled, rebuilding the archive from the tag rather than
trusting the quarantined file:

```sh
RELEASE_VERSION=v0.1.0
SOURCE_COMMIT=<40-character-tagged-commit>
REVIEW_EVIDENCE=/absolute/path/to/final-review.md
BOOTSTRAP=$(mktemp "${TMPDIR:-/tmp}/yomihon-source-artifact-bootstrap.XXXXXX")
trap 'rm -f "$BOOTSTRAP"' 0 HUP INT TERM
GIT_NO_REPLACE_OBJECTS=1 git show "${SOURCE_COMMIT}:tools/source-artifact-bootstrap.sh" >"$BOOTSTRAP"
sh "$BOOTSTRAP" --assemble "$RELEASE_VERSION" "$SOURCE_COMMIT" "$REVIEW_EVIDENCE" dist
```

`make source-artifact` is the corresponding convenience wrapper. Formal
release evidence uses the operator-visible committed-bootstrap sequence above.

The command refuses a version whose tag does not resolve to `SOURCE_COMMIT`
and refuses a lightweight tag, a dirty or different checkout, linked output,
or overwrite. `REVIEW_EVIDENCE` is the public-safe, completed review report
created after the archive candidate. Its reviewer-owned release envelope names
that exact commit, the exact `PROJECT_PROFILE.md` SHA-256, a distinct builder
and reviewer, two distinct cold-session records, all three PASS Gates, final
GO, no unresolved/blocked checks, and no active release exception. Formal mode
also requires `review-class: release-candidate`,
`certified-scope: complete-project-profile`, and an approved project-profile
readiness envelope with merge and artifact-build GO, no artifact-build
blockers, release state `PENDING-ARTIFACT`, open blockers equal to the named
post-artifact blockers, final human approvals, approval binding
`EXTERNAL-RELEASE-REPORT`, and no active exception. That token avoids an
impossible self-referential commit field: the external report carries the
candidate commit and profile digest, and the generated certificate/provenance
chain binds the report. Its
`closed-profile-blockers` field and closure table must enumerate exactly that
post-artifact set; a generic GO cannot erase another obligation. Fixture
evidence cannot be promoted by changing the command-line mode. The builder
takes one byte snapshot of the report, verifies that the human Verdict/Gate
sections agree with the envelope, copies the report into the release bundle,
and derives a 16-field certificate that records the evidence class/scope,
closed profile blockers, and report SHA-256. The report's verified artifact digest must equal the archive
that final assembly independently rebuilds. The external report must be valid UTF-8 without raw control,
NUL, U+2028, or U+2029 bytes. Provenance
and the manifest bind both files, so a consumer can inspect and re-hash the
evidence rather than receiving an opaque digest with no artifact.

The command stages and validates the complete version directory before one
same-filesystem rename under `dist/`. The bundle contains a gzip-compressed
`git archive`, the review report, its release certificate, a provenance
sidecar, and a SHA-256 manifest covering all four payload files. Provenance
binds the archive to the commit, tree, annotated-tag object, verification
evidence, normative engineering-standard digest, project-profile bytes, Go and
frontend dependency lockfiles, nested SQLite bake-off module, CI workflow,
committed bootstrap, and the Git/gzip builder versions. The 25-field provenance
records the bootstrap digest. Formal release creation also requires the exact
versions in the committed `tools/source-artifact-toolchain.txt`; recording an
ambient version after the fact is not treated as a reproducible pin. Changing
either release tool is a reviewed toolchain change that must update that file
and rerun the artifact mutations. Verification fixtures record their ambient
tools but are classified `verification-fixture` and accepted only with
`--allow-fixture`; their review class/scope is
`verification-fixture`/`source-artifact-contract`, so they cannot be presented
as release artifacts. Archive creation disables replacement objects and
ambient repository/config selection, refuses grafts, replacement refs,
repository-local attributes, gitlinks, and committed `export-ignore` or
`export-subst`, and runs in a fresh bare Git context with empty global/system
attribute roots. The checker independently compares the archive entry set with
`git ls-tree` as well as byte-comparing a reconstructed tar. `make verify`
rebuilds the test bundle twice under conflicting ambient `tar.umask`
configuration, compares it byte-for-byte, then repairs self-checksums around
hostile report, certificate, provenance, and subset-archive mutations to prove
the semantic checker—not merely a checksum—turns red.

The committed-bootstrap mutation installs a reversible clean/smudge filter
that makes the working-tree builder exit immediately while Git still reports a
clean checkout. The canonical entry point must still prepare and assemble from
the committed bytes. This proves resistance to accidental checkout transforms;
it is not a cryptographic attestation against an operator who deliberately
executes a different command.

The artifact checker proves the report's canonical evidence structure is
complete: every template section has completed data or an explicit N/A row,
every canonical table has its exact header, and every fixed semantic row is
present and complete. It additionally validates the header and immutable snapshot, a completed
PASS Gate 2 scenario on a named public surface, the exact `make verify` log
entry, a watched-red row, evidence references, matching human Gate/Verdict
sections, no blocker or release-negative structured status, two distinct Gate
2 session identities and evidence references that are independent from the
builder, and the independent certification. Each evidence reference must occur
on a completed PASS scenario; a composite transcript may be used only through
distinct references that identify its two session records separately. The pre-assembly
source-release report records `Artifact digest` as the reviewed archive's
`sha256:<digest>`/`verified`; the generated manifest and
provenance then bind the archive/report/certificate digests without asking the
report to contain the digest of the bundle that contains it. Those checks are scoped to their
canonical fields and tables, so a finding may quote an incomplete-template or
malformed-wire example without being mistaken for an unfilled report. The
checker cannot judge whether the reviewer's observations are intellectually
sound. That judgment remains Gate ownership work by a genuinely independent
reviewer; a structurally complete report is not review quality evidence by
itself.

Publication is serialized by a per-version lock. After the final absence
check, the builder performs the same-filesystem directory rename and validates
the exact destination root again. A competing destination therefore produces
a named failure and cleanup rather than a false "published" result or a hidden
nested staging directory. This is coordination between release builders, not
a claim that arbitrary same-user filesystem tampering is impossible; the
manifest and checker remain the post-publication integrity boundary.
Prepared single-file archives use a same-filesystem hard-link publication so
an entry that appears between the last check and publication cannot be
overwritten.
The mutation suite also changes a valid release checkout only after staged
validation and proves the final source revalidation refuses publication.

No signing identity or hosted provenance service is claimed for `v0.x`.
Checksums plus the commit- and review-bound provenance sidecar are the declared
source-only mechanism; adding binaries, signing, hosted attestations, an SBOM
service, containers, or installers requires a revised release profile rather
than an implicit extension of this command.

## Readiness boundary

- **Merge-ready** means one immutable change has three PASS Gates, canonical
  verification, no unresolved checks, and only valid approved exceptions. It
  does not authorize a tag.
- **Release-ready** adds every gate in this document, exact release notes and
  changelog, the source artifact/provenance check, and a current platform and
  compatibility review.
- **Production-ready** is operator- and environment-specific: it additionally
  requires the real vault/configuration, BYOK provider where used, recovery
  exercise, and post-start validation. A source archive cannot certify these
  facts for another user.

Any `UNVERIFIED`, `BLOCKED`, or `UNRESOLVED` item keeps the dependent readiness
claim false. A later commit invalidates the prior verdict and artifact evidence.

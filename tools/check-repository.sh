#!/bin/sh

# Check repository facts that are deterministic in a clean checkout. Human
# review quality, reviewer independence, Gate verdicts, and exception approval
# deliberately remain outside this script.
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
cd "$root"

fail() {
	echo "repository-check: $*" >&2
	exit 1
}

sha256_file() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | awk '{print $1}'
	elif command -v shasum >/dev/null 2>&1; then
		shasum -a 256 "$1" | awk '{print $1}'
	else
		fail "sha256sum or shasum is required"
	fi
}

required_files='ENGINEERING_STANDARD.md
ENGINEERING_STANDARD.sha256
PROJECT_PROFILE.md
README.md
LICENSE
CONTRIBUTING.md
SECURITY.md
AGENTS.md
CLAUDE.md
.github/CODEOWNERS
.github/PULL_REQUEST_TEMPLATE.md
docs/merge-policy.md
docs/release.md'

printf '%s\n' "$required_files" | while IFS= read -r file; do
	[ -s "$file" ] || fail "required repository file is missing or empty: $file"
	git ls-files --error-unmatch -- "$file" >/dev/null 2>&1 ||
		fail "required repository file is not tracked: $file"
done

standard_count=$(git ls-files ':(glob)**/ENGINEERING_STANDARD*.md' | wc -l | tr -d ' ')
[ "$standard_count" = 1 ] || fail "found $standard_count normative-standard candidates, want exactly one"

expected_standard_sha=$(awk 'NF == 2 && $2 == "ENGINEERING_STANDARD.md" {print $1}' ENGINEERING_STANDARD.sha256)
[ ${#expected_standard_sha} -eq 64 ] || fail "ENGINEERING_STANDARD.sha256 has no single SHA-256 identity"
actual_standard_sha=$(sha256_file ENGINEERING_STANDARD.md)
[ "$actual_standard_sha" = "$expected_standard_sha" ] || fail "ENGINEERING_STANDARD.md changed without updating its normative digest"

forbidden_paths=$(git ls-files -- \
	':(glob)**/.env' \
	':(glob)**/.env.*' \
	':(glob)**/*.pem' \
	':(glob)**/*.key' \
	':(glob)**/*.p12' \
	':(glob)**/*.pfx' \
	':(glob)**/*.sqlite' \
	':(glob)**/*.sqlite3' \
	':(glob)**/*.db' \
	':(glob)**/*-wal' \
	':(glob)**/*-shm' \
	':(glob)**/*-journal')
[ -z "$forbidden_paths" ] || fail "forbidden credential or derived-store path is tracked: $forbidden_paths"

# Public documentation must stay self-contained: a clean-clone reader
# understands the product from the README and the docs tree alone, without the
# maintainer contributor files. Forbid any tracked public doc from citing those
# files as authority. Scope is tracked Markdown only -- README.md and every
# docs/**/*.md, listed through git so ignored files, the local harness,
# generated output, and any untracked working path are never scanned. The
# :(glob) magic is required: a plain docs/**/*.md pathspec silently skips the
# top-level docs so the scan would pass vacuously.
#
# The scope producer runs on its own, never inside a pipeline, so a git failure
# or an empty result cannot be swallowed into a silent pass: enumerate the scope
# first, fail if git cannot list it, fail if it is empty, and only then scan.
# The scan itself fails closed on the three distinct grep outcomes, so an
# unreadable file, a broken symlink, or any read error can never be mistaken for
# "no reference found": exit 0 means a forbidden reference was found (fail,
# listing the hits); exit 1 is the only clean outcome (no match); any other exit
# means that tracked document could not be scanned (fail, naming it).
public_docs=$(git ls-files -- 'README.md' ':(glob)docs/**/*.md') ||
	fail "could not enumerate the tracked public documentation scope"
[ -n "$public_docs" ] ||
	fail "tracked public documentation scope is empty; refusing a vacuous self-containment scan"
while IFS= read -r doc; do
	scan_status=0
	hits=$(grep -nE 'AGENTS\.md|CLAUDE\.md' -- "$doc") || scan_status=$?
	if [ "$scan_status" -eq 0 ]; then
		fail "public documentation must be self-contained: $doc cites a forbidden maintainer authority (AGENTS.md or CLAUDE.md); state the product boundary in the documentation itself. Offending lines:
$hits"
	elif [ "$scan_status" -ne 1 ]; then
		fail "public documentation self-containment scan could not read tracked document $doc (grep exit $scan_status); a public document that cannot be scanned is treated as a violation, not a pass"
	fi
done <<EOF
$public_docs
EOF

echo "repository-check: tracked repository facts are consistent"

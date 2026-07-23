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

required_files='README.md
LICENSE
SECURITY.md
THIRD_PARTY_NOTICES.md'

printf '%s\n' "$required_files" | while IFS= read -r file; do
	[ -s "$file" ] || fail "required repository file is missing or empty: $file"
	git ls-files --error-unmatch -- "$file" >/dev/null 2>&1 ||
		fail "required repository file is not tracked: $file"
done

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
# understands the product from the public files alone, without the maintainer
# contributor files. Forbid any tracked public document from citing those files
# as authority. The scope is the exact set of tracked public prose files, named
# explicitly and listed through git so an untracked, renamed, or ignored path is
# never silently scanned in their place.
#
# The scope producer runs on its own, never inside a pipeline, so a git failure
# or an empty result cannot be swallowed into a silent pass: enumerate the scope
# first, fail if git cannot list it, fail if it is empty, and only then scan.
# The scan itself fails closed on the three distinct grep outcomes, so an
# unreadable file, a broken symlink, or any read error can never be mistaken for
# "no reference found": exit 0 means a forbidden reference was found (fail,
# listing the hits); exit 1 is the only clean outcome (no match); any other exit
# means that tracked document could not be scanned (fail, naming it).
public_docs=$(git ls-files -- 'README.md' 'SECURITY.md' 'THIRD_PARTY_NOTICES.md') ||
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

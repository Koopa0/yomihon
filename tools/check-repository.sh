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

echo "repository-check: tracked repository facts are consistent"

#!/bin/sh

# Repository hygiene: refuse to track credentials, private keys, or derived
# database stores. This is independent of any release or review process -- a
# secret or a generated store must never enter history.
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
cd "$root"

forbidden=$(git ls-files -- \
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
[ -z "$forbidden" ] || {
	echo "tracked-paths: forbidden credential or derived-store path is tracked: $forbidden" >&2
	exit 1
}
echo "tracked-paths: no credential or derived-store path is tracked"

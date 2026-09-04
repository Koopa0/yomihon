#!/bin/sh

# Pins the finding set two fixture vaults must produce from `yomihon check`,
# so a contract edit that silently drops privacy coverage, or introduces a new
# unreviewed schema or link fault, is caught here instead of only surfacing as
# a changed screenshot or a browser probe that happens to touch the same note.
#
# Neither fixture is expected to come back clean. The e2e vault exercises the
# reading faces without conforming to every field on purpose — see the comment
# at the top of its own contract for what that covers, though one pinned line
# below it does not reach: schema.unmatched_knowledge_dir on Sources looks like
# a leftover rather than a plan, and is pinned as observed rather than fixed
# here. The example vault keeps two small faults on display because they are
# the worked example a reader opens to see what a diagnostic looks like.
# Either way, this compares the exact rule_id+path set `check` reports against
# the pinned list below, so a change to it is reviewed rather than silently
# waved through, whatever prompted it.
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
cd "$root"

tmp=$(mktemp -d "${TMPDIR:-/tmp}/yomihon-check-fixtures.XXXXXX")
trap 'rm -rf "$tmp"' 0 HUP INT TERM

go build -o "$tmp/yomihon" ./cmd/yomihon

# $1: a file of JSONL Finding objects. rule_id and path are always the first
# and third keys, in that order (judge.Finding's own field order), and no path
# in either pinned fixture contains a literal double quote, so this single
# capture per key is exact rather than approximate.
extract() {
    sed -n 's/.*"rule_id":"\([^"]*\)".*"path":"\([^"]*\)".*/\1\t\2/p' "$1" | LC_ALL=C sort -u
}

# $1: pinned list. $2: freshly extracted rule_id+path set. Prints the
# explanation before the diff, and neutralizes diff's own exit status, so both
# are visible regardless of how "set -e" would otherwise short-circuit the
# function on the first non-zero command.
compare() {
    if ! cmp -s "$1" "$2"; then
        echo "check-fixtures: $1 no longer matches what yomihon check reports (- pinned, + actual)" >&2
        diff -u "$1" "$2" || true
        exit 1
    fi
}

# check --deny denies the command a clean exit at that severity or above, but
# still reports every finding regardless of --deny; the e2e vault's pinned
# findings are why 1 is an accepted exit here, not just 0. Exit 2 means the
# command could not run at all — most notably, the fail-closed refusal this
# gate exists to catch — and stays a hard failure either way.
if "$tmp/yomihon" check --root .github/e2e/vault --format json --deny warn \
    >"$tmp/e2e-vault.json" 2>"$tmp/e2e-vault.err"; then
    status=0
else
    status=$?
fi
if [ "$status" -ne 0 ] && [ "$status" -ne 1 ]; then
    cat "$tmp/e2e-vault.err" >&2
    echo "check-fixtures: yomihon check --root .github/e2e/vault exited $status, want 0 or 1" >&2
    exit 1
fi
extract "$tmp/e2e-vault.json" >"$tmp/e2e-vault.actual"
compare "tools/check-fixtures.e2e-vault.expected" "$tmp/e2e-vault.actual"

"$tmp/yomihon" check --root examples/vault --format json >"$tmp/examples-vault.json"
extract "$tmp/examples-vault.json" >"$tmp/examples-vault.actual"
compare "tools/check-fixtures.examples-vault.expected" "$tmp/examples-vault.actual"

echo "check-fixtures: both fixtures' check findings match their pinned list"

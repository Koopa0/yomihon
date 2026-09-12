#!/bin/sh
# Fail when any commit in a range carries an attribution trailer in its message.
# The pattern tolerates leading whitespace, any case, and space before the colon.
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
script="$root/tools/check-commit-attribution-trailers.sh"

trailer_re='^[[:space:]]*(co-authored-by|signed-off-by|co-committed-by)[[:space:]]*:'

line_is_trailer() {
	printf '%s\n' "$1" | grep -qiE "$trailer_re"
}

check_range() {
	git_dir=$1
	range=$2
	status=0
	for sha in $(git -C "$git_dir" rev-list "$range"); do
		line_num=0
		while IFS= read -r line || [ -n "$line" ]; do
			line_num=$((line_num + 1))
			if line_is_trailer "$line"; then
				echo "commit-attribution: $sha line $line_num: $line" >&2
				status=1
			fi
		done <<EOF
$(git -C "$git_dir" log -1 --format=%B "$sha")
EOF
	done
	return "$status"
}

run_self_test() {
	tmp=$(mktemp -d "${TMPDIR:-/tmp}/commit-attribution-selftest.XXXXXX")
	trap 'rm -rf "$tmp"' EXIT INT HUP
	cd "$tmp"
	git init -q
	git config user.email 'self-test@yomihon.local'
	git config user.name 'self-test'

	echo base >file
	git add file
	git commit -q -m 'chore: clean base'
	base=$(git rev-parse HEAD)
	case_num=0

	fail_case() {
		msg=$1
		case_num=$((case_num + 1))
		echo "$case_num" >"case-$case_num"
		git add .
		git commit -q -m "$msg"
		if sh "$script" --git-dir "$tmp" "${base}..HEAD"; then
			echo "commit-attribution self-test: expected failure for case $case_num" >&2
			exit 1
		fi
		git reset --hard "$base"
	}

	fail_case "$(printf 'feat: co-authored trailer\n\nCo-authored-by: Agent <agent@example.com>')"
	fail_case "$(printf 'feat: indented trailer\n\n  Co-Authored-By: Agent <agent@example.com>')"
	fail_case "$(printf 'feat: spaced colon\n\nSigned-off-by : Human <human@example.com>')"
	fail_case "$(printf 'feat: co-committed trailer\n\nco-committed-by: Tool <tool@example.com>')"

	echo pass >file-pass
	git add file-pass
	git commit -q -m 'feat: no trailer'
	if ! sh "$script" --git-dir "$tmp" "${base}..HEAD"; then
		echo 'commit-attribution self-test: expected pass for a clean branch' >&2
		exit 1
	fi

	echo 'commit-attribution: self-test passed'
}

git_dir=$root
range=

while [ $# -gt 0 ]; do
	case "$1" in
		--self-test)
			run_self_test
			exit 0
			;;
		--git-dir)
			shift
			[ $# -gt 0 ] || {
				echo 'commit-attribution: --git-dir requires a path' >&2
				exit 2
			}
			git_dir=$1
			shift
			;;
		--)
			shift
			break
			;;
		-*)
			echo "commit-attribution: unknown option $1" >&2
			exit 2
			;;
		*)
			range=$1
			shift
			;;
	esac
done

if [ -z "$range" ]; then
	echo 'commit-attribution: a commit range is required (for example origin/main..HEAD)' >&2
	exit 2
fi

if ! git -C "$git_dir" rev-parse --verify "${range%%..*}" >/dev/null 2>&1; then
	echo "commit-attribution: unknown revision ${range%%..*}" >&2
	exit 2
fi
if ! git -C "$git_dir" rev-parse --verify "${range##*..}" >/dev/null 2>&1; then
	echo "commit-attribution: unknown revision ${range##*..}" >&2
	exit 2
fi

check_range "$git_dir" "$range" || exit 1
echo 'commit-attribution: no attribution trailer in the commit range'

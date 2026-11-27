#!/bin/sh
# Fail when any commit in a range carries an attribution trailer in its message.
# The pattern tolerates leading whitespace, any case, and space before the colon.
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)

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

git_dir=$root
range=

while [ $# -gt 0 ]; do
	case "$1" in
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

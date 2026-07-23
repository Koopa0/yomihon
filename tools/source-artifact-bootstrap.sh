#!/bin/sh

# Materialize and execute the release-control scripts from one immutable commit.
# The canonical invocation pipes this file from `git show COMMIT:...`; it must
# not trust executable bytes produced by the caller's checkout filters.
set -eu

usage() {
	echo "usage: source-artifact-bootstrap --prepare-archive VERSION COMMIT OUTPUT_FILE" >&2
	echo "       source-artifact-bootstrap --assemble VERSION COMMIT OUTPUT_DIR" >&2
	exit 2
}

mode=${1:-}
case "$mode" in
--prepare-archive) [ "$#" -eq 4 ] || usage ;;
--assemble) [ "$#" -eq 4 ] || usage ;;
*) usage ;;
esac

version=$2
commit=$3
original_directory=$(pwd -P)
case "$mode" in
--prepare-archive)
	case "$4" in /*) output=$4 ;; *) output=$original_directory/$4 ;; esac
	;;
--assemble)
	case "$4" in /*) output=$4 ;; *) output=$original_directory/$4 ;; esac
	;;
esac

unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_COMMON_DIR
unset GIT_OBJECT_DIRECTORY GIT_ALTERNATE_OBJECT_DIRECTORIES
unset GIT_NAMESPACE GIT_REPLACE_REF_BASE GIT_SHALLOW_FILE GIT_GRAFT_FILE
unset GIT_CONFIG GIT_CONFIG_COUNT GIT_CONFIG_PARAMETERS
unset GIT_CONFIG_GLOBAL GIT_CONFIG_SYSTEM GIT_EXEC_PATH
unset GIT_TEMPLATE_DIR GIT_DEFAULT_HASH GIT_OBJECT_FORMAT
unset GIT_LITERAL_PATHSPECS GIT_GLOB_PATHSPECS GIT_NOGLOB_PATHSPECS GIT_ICASE_PATHSPECS
export GIT_NO_REPLACE_OBJECTS=1
export GIT_CONFIG_NOSYSTEM=1
export GIT_ATTR_NOSYSTEM=1

root=$(git rev-parse --show-toplevel) || {
	echo 'source-artifact-bootstrap: run from the source repository' >&2
	exit 1
}
cd "$root"
printf '%s\n' "$commit" | grep -Eq '^[0-9a-f]{40}$' || {
	echo 'source-artifact-bootstrap: COMMIT must be a lowercase 40-character object ID' >&2
	exit 2
}
[ "$(git rev-parse --verify "$commit^{commit}" 2>/dev/null || true)" = "$commit" ] || {
	echo 'source-artifact-bootstrap: COMMIT does not resolve to itself' >&2
	exit 1
}
git cat-file -e "$commit:tools/source-artifact-bootstrap.sh" 2>/dev/null || {
	echo 'source-artifact-bootstrap: bootstrap is missing from the source commit' >&2
	exit 1
}

temporary=$(mktemp -d "${TMPDIR:-/tmp}/yomihon-source-artifact-bootstrap.XXXXXX")
cleanup() {
	rm -rf "$temporary"
}
trap cleanup 0 HUP INT TERM
mkdir "$temporary/home" "$temporary/xdg" "$temporary/template"

run_isolated_git() {
	env \
		HOME="$temporary/home" \
		XDG_CONFIG_HOME="$temporary/xdg" \
		GIT_CONFIG_GLOBAL=/dev/null \
		GIT_CONFIG_SYSTEM=/dev/null \
		GIT_CONFIG_NOSYSTEM=1 \
		GIT_ATTR_NOSYSTEM=1 \
		GIT_NO_REPLACE_OBJECTS=1 \
		GIT_TEMPLATE_DIR="$temporary/template" \
		GIT_DEFAULT_HASH=sha1 \
		git "$@"
}

run_isolated_git clone --no-local --no-checkout -q "$root" "$temporary/repository"
run_isolated_git -C "$temporary/repository" checkout --detach -q "$commit"

for control_file in \
	tools/source-artifact-bootstrap.sh \
	tools/build-source-artifact.sh \
	tools/check-source-artifact.sh \
	tools/source-artifact-lib.sh; do
	git show "$commit:$control_file" | cmp - "$temporary/repository/$control_file" || {
		echo "source-artifact-bootstrap: checkout transformed release-control bytes: $control_file" >&2
		exit 1
	}
done

[ "$(run_isolated_git -C "$temporary/repository" rev-parse HEAD)" = "$commit" ] || {
	echo 'source-artifact-bootstrap: isolated checkout has the wrong HEAD' >&2
	exit 1
}
[ -z "$(run_isolated_git -C "$temporary/repository" status --porcelain --untracked-files=all)" ] || {
	echo 'source-artifact-bootstrap: isolated checkout is not clean' >&2
	exit 1
}

cd "$temporary/repository"
case "$mode" in
--prepare-archive)
	sh tools/build-source-artifact.sh --prepare-archive "$version" "$commit" "$output"
	;;
--assemble)
	sh tools/build-source-artifact.sh --require-tag "$version" "$commit" "$output"
	;;
esac

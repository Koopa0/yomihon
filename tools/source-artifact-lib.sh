#!/bin/sh

# Shared, side-effect-free primitives for the source artifact builder and
# checker. Callers own temporary-directory cleanup and user-facing errors.

source_artifact_sanitize_git_environment() {
	# Release identity must not depend on a caller-selected repository, index,
	# object store, namespace, replacement base, pathspec mode, or config file.
	# GIT_CONFIG_KEY_* / VALUE_* are inert once GIT_CONFIG_COUNT is unset.
	unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_COMMON_DIR
	unset GIT_OBJECT_DIRECTORY GIT_ALTERNATE_OBJECT_DIRECTORIES
	unset GIT_NAMESPACE GIT_REPLACE_REF_BASE GIT_SHALLOW_FILE GIT_GRAFT_FILE
	unset GIT_CONFIG GIT_CONFIG_COUNT GIT_CONFIG_PARAMETERS
	unset GIT_CONFIG_GLOBAL GIT_CONFIG_SYSTEM GIT_EXEC_PATH
	unset GIT_TEMPLATE_DIR GIT_DEFAULT_HASH GIT_OBJECT_FORMAT
	unset GIT_LITERAL_PATHSPECS GIT_GLOB_PATHSPECS GIT_NOGLOB_PATHSPECS GIT_ICASE_PATHSPECS
	unset GZIP TAR_OPTIONS
	export GIT_NO_REPLACE_OBJECTS=1
	export GIT_CONFIG_NOSYSTEM=1
	export GIT_ATTR_NOSYSTEM=1
}

source_artifact_git_context_problem() (
	attributes=$(git rev-parse --git-path info/attributes) || {
		printf '%s\n' unreadable-git-paths
		return 0
	}
	if [ -s "$attributes" ]; then
		printf '%s\n' repository-local-attributes
		return 0
	fi
	grafts=$(git rev-parse --git-path info/grafts) || {
		printf '%s\n' unreadable-git-paths
		return 0
	}
	if [ -s "$grafts" ]; then
		printf '%s\n' grafts
		return 0
	fi
	replacements=$(git for-each-ref --format='%(refname)' refs/replace) || {
		printf '%s\n' unreadable-replacement-refs
		return 0
	}
	if [ -n "$replacements" ]; then
		printf '%s\n' replacement-refs
		return 0
	fi
	return 1
)

source_artifact_has_reserved_archive_attributes() (
	sa_attributes_commit=$1
	# The artifact contract is the complete, byte-preserving commit tree.
	# export-ignore and export-subst intentionally define a different product,
	# so they are forbidden rather than silently interpreted.
	git grep -I -q -E '(^|[[:space:]])-?export-(ignore|subst)(=|[[:space:]]|$)' \
		"$sa_attributes_commit" -- .gitattributes ':(glob)**/.gitattributes'
)

source_artifact_has_gitlinks() (
	sa_gitlinks_commit=$1
	sa_gitlinks_tree=$(git ls-tree -r "$sa_gitlinks_commit") || return 2
	printf '%s\n' "$sa_gitlinks_tree" | awk '$1 == "160000" { found = 1 } END { exit !found }'
)

source_artifact_make_archive() (
	sa_archive_commit=$1
	sa_archive_prefix=$2
	sa_archive_output=$3
	sa_archive_scratch=$4

	mkdir -p "$sa_archive_scratch/home" "$sa_archive_scratch/xdg" "$sa_archive_scratch/template" || return 2
	env \
		HOME="$sa_archive_scratch/home" \
		XDG_CONFIG_HOME="$sa_archive_scratch/xdg" \
		GIT_CONFIG_GLOBAL=/dev/null \
		GIT_CONFIG_SYSTEM=/dev/null \
		GIT_CONFIG_NOSYSTEM=1 \
		GIT_ATTR_NOSYSTEM=1 \
		GIT_NO_REPLACE_OBJECTS=1 \
		GIT_TEMPLATE_DIR="$sa_archive_scratch/template" \
		GIT_DEFAULT_HASH=sha1 \
		git init --bare --object-format=sha1 --template="$sa_archive_scratch/template" -q "$sa_archive_scratch/repository.git" || return 2
	sa_objects=$(git rev-parse --git-path objects) || return 2
	case "$sa_objects" in
	/*) ;;
	*) sa_objects=$PWD/$sa_objects ;;
	esac
	printf '%s\n' "$sa_objects" >"$sa_archive_scratch/repository.git/objects/info/alternates" || return 2

	# A fresh bare repository has no info/attributes or local config. Empty HOME
	# and XDG roots plus the no-system switches remove global/system attributes;
	# replacements remain disabled even when the object store contains refs for
	# the caller's repository.
	env \
		HOME="$sa_archive_scratch/home" \
		XDG_CONFIG_HOME="$sa_archive_scratch/xdg" \
		GIT_DIR="$sa_archive_scratch/repository.git" \
		GIT_CONFIG_GLOBAL=/dev/null \
		GIT_CONFIG_SYSTEM=/dev/null \
		GIT_CONFIG_NOSYSTEM=1 \
		GIT_ATTR_NOSYSTEM=1 \
		GIT_NO_REPLACE_OBJECTS=1 \
		git -c tar.umask=0002 archive --format=tar --prefix="$sa_archive_prefix" "$sa_archive_commit" >"$sa_archive_output"
)

source_artifact_tree_paths() (
	sa_tree_commit=$1
	sa_tree_output=$2
	git -c core.quotePath=false ls-tree -r --name-only "$sa_tree_commit" >"$sa_tree_output" || return 2
	# Quoted output means the tree contains a control- or escape-sensitive path.
	# Such paths cannot be compared unambiguously with portable tar listings.
	! grep -q '^"' "$sa_tree_output"
)

source_artifact_archive_paths() (
	sa_paths_archive=$1
	sa_paths_prefix=$2
	sa_paths_output=$3
	sa_tar_listing=$(tar -tf "$sa_paths_archive") || return 2
	printf '%s\n' "$sa_tar_listing" | awk -v prefix="$sa_paths_prefix" '
		$0 == prefix { next }
		index($0, prefix) == 1 {
			path = substr($0, length(prefix) + 1)
			if (path !~ /\/$/) print path
		}
	' >"$sa_paths_output"
)

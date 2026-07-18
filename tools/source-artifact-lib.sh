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

source_artifact_review_text_is_safe() (
	sa_text_file=$1
	command -v iconv >/dev/null 2>&1 || return 2
	command -v od >/dev/null 2>&1 || return 2
	sa_decimal=$(mktemp "${TMPDIR:-/tmp}/yomihon-review-decimal.XXXXXX") || return 2
	sa_hex_file=$(mktemp "${TMPDIR:-/tmp}/yomihon-review-hex.XXXXXX") || {
		rm -f "$sa_decimal"
		return 2
	}
	trap 'rm -f "$sa_decimal" "$sa_hex_file"' 0 HUP INT TERM

	# iconv rejects malformed UTF-8. The byte pass rejects NUL, every raw C0
	# control except TAB/LF, DEL, and the two Unicode line separators that have
	# repeatedly made Markdown look textual while breaking byte-oriented tools.
	iconv -f UTF-8 -t UTF-8 "$sa_text_file" >/dev/null 2>&1 || return 1
	od -An -v -t u1 "$sa_text_file" >"$sa_decimal" || return 2
	awk '
		{
			for (i = 1; i <= NF; i++) {
				if (($i < 32 && $i != 9 && $i != 10) || $i == 127) exit 1
			}
		}
	' "$sa_decimal" || return 1
	od -An -v -t x1 "$sa_text_file" >"$sa_hex_file" || return 2
	sa_hex=$(tr -d ' \n' <"$sa_hex_file") || return 2
	case "$sa_hex" in
	*e280a8* | *e280a9*) return 1 ;;
	esac
	return 0
)

# Release principals are deliberately single, machine-comparable tokens rather
# than display names or lists. This is only a structural identity check; the
# independent review still owns proving who controlled the named principal.
source_artifact_token_is_canonical() (
	sa_token=$1
	LC_ALL=C printf '%s\n' "$sa_token" | grep -Eq '^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$'
)

source_artifact_identity_is_canonical() (
	sa_identity=$1
	case "$sa_identity" in
	@*) sa_identity=${sa_identity#@} ;;
	esac
	source_artifact_token_is_canonical "$sa_identity"
)

source_artifact_normalize_identity() {
	printf '%s\n' "$1" | sed 's/^@//' | tr '[:upper:]' '[:lower:]'
}

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

source_artifact_profile_approval_field() (
	sa_profile_commit=$1
	sa_profile_key=$2
	git show "$sa_profile_commit:PROJECT_PROFILE.md" | awk -v key="$sa_profile_key" '
		$0 == "## 19. Profile approval" { headings++; inside = 1; next }
		inside && /^## / { inside = 0 }
		inside && index($0, key ": ") == 1 {
			count++
			value = substr($0, length(key) + 3)
		}
		END { if (headings != 1 || count != 1 || value == "") exit 1; print value }
	'
)

source_artifact_profile_readiness_is_canonical() (
	sa_profile_commit=$1
	git show "$sa_profile_commit:PROJECT_PROFILE.md" | awk '
		$0 == "## 20. Machine-readable readiness envelope" {
			headings++
			inside_section = 1
			next
		}
		inside_section && /^## / { inside_section = 0; in_block = 0 }
		inside_section && $0 == "```text" {
			blocks++
			in_block = 1
			next
		}
		in_block && $0 == "```" {
			closes++
			in_block = 0
			next
		}
		in_block {
			rows++
			split_at = index($0, ": ")
			if (!split_at) { bad = 1; next }
			key = substr($0, 1, split_at - 1)
			value = substr($0, split_at + 2)
			expected[1] = "profile-status"
			expected[2] = "merge-readiness"
			expected[3] = "artifact-build-readiness"
			expected[4] = "artifact-build-blockers"
			expected[5] = "post-artifact-blockers"
			expected[6] = "release-readiness"
			expected[7] = "production-readiness"
			expected[8] = "open-blockers"
			expected[9] = "active-exceptions"
			if (rows > 9 || key != expected[rows] || value == "") bad = 1
		}
		END { exit headings != 1 || blocks != 1 || closes != 1 || rows != 9 || in_block || bad }
	'
)

source_artifact_profile_readiness_field() (
	sa_profile_commit=$1
	sa_profile_key=$2
	source_artifact_profile_readiness_is_canonical "$sa_profile_commit" || return 1
	git show "$sa_profile_commit:PROJECT_PROFILE.md" | awk -v key="$sa_profile_key" '
		$0 == "## 20. Machine-readable readiness envelope" { inside_section = 1; next }
		inside_section && /^## / { inside_section = 0; in_block = 0 }
		inside_section && $0 == "```text" { in_block = 1; next }
		in_block && $0 == "```" { in_block = 0; next }
		in_block && index($0, key ": ") == 1 {
			count++
			value = substr($0, length(key) + 3)
		}
		END { if (count != 1 || value == "") exit 1; print value }
	'
)

source_artifact_profile_approvals_are_final() (
	sa_profile_commit=$1
	sa_profile_version=$(source_artifact_profile_approval_field "$sa_profile_commit" 'Profile version') || return 1
	sa_profile_binding=$(source_artifact_profile_approval_field "$sa_profile_commit" 'Approval binding') || return 1
	sa_architecture_approval=$(source_artifact_profile_approval_field "$sa_profile_commit" 'Architecture approval') || return 1
	sa_security_approval=$(source_artifact_profile_approval_field "$sa_profile_commit" 'Security / privacy approval where applicable') || return 1
	sa_operations_approval=$(source_artifact_profile_approval_field "$sa_profile_commit" 'Operations approval where applicable') || return 1
	sa_independent_approval=$(source_artifact_profile_approval_field "$sa_profile_commit" 'Independent approval') || return 1
	case "$sa_profile_version" in *draft* | *DRAFT*) return 1 ;; esac
	[ "$sa_profile_binding" = EXTERNAL-RELEASE-REPORT ] || return 1
	case "$sa_architecture_approval" in APPROVED | 'APPROVED —'*) ;; *) return 1 ;; esac
	case "$sa_security_approval" in APPROVED | 'APPROVED —'* | N/A | 'N/A —'*) ;; *) return 1 ;; esac
	case "$sa_operations_approval" in APPROVED | 'APPROVED —'* | N/A | 'N/A —'*) ;; *) return 1 ;; esac
	case "$sa_independent_approval" in APPROVED | 'APPROVED —'*) ;; *) return 1 ;; esac
)

source_artifact_profile_listed_blockers() (
	sa_profile_commit=$1
	git show "$sa_profile_commit:PROJECT_PROFILE.md" | awk '
		$0 == "## 18. Exceptions and current debt" { section_headings++; inside_section = 1; next }
		inside_section && /^## / { inside_section = 0; inside_table = 0 }
		inside_section && $0 == "Current blockers:" { captions++; expect_header = 1; next }
		expect_header && $0 == "| ID | Contract and risk | Current containment | Owner / required ruling | Gate effect |" {
			headers++
			expect_header = 0
			expect_separator = 1
			next
		}
		expect_separator {
			if ($0 != "|---|---|---|---|---|") bad = 1
			separators++
			expect_separator = 0
			inside_table = 1
			next
		}
		inside_table && /^[[:space:]]*$/ { inside_table = 0; next }
		inside_table && /^\|/ {
			row = $0
			gsub(/\\\|/, "\034", row)
			columns = split(row, cells, "[|]")
			if (columns != 7) bad = 1
			for (i = 2; i <= 6; i++) {
				gsub(/^[[:space:]]+|[[:space:]]+$/, "", cells[i])
				if (cells[i] == "") bad = 1
			}
			id = cells[2]
			rows++
			if (id == "None") { none_rows++; next }
			if (id !~ /^[A-Z][A-Z0-9]*(-[A-Z0-9]+)*$/ || seen[id]++) bad = 1
			if (value != "") value = value ","
			value = value id
		}
		END {
			if (section_headings != 1 || captions != 1 || headers != 1 || separators != 1 ||
			    expect_header || expect_separator || rows < 1 || bad ||
			    (none_rows && (rows != 1 || value != ""))) exit 1
			if (value == "") value = "none"
			print value
		}
	'
)

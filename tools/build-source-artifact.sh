#!/bin/sh

# Build a deterministic source archive and non-circular provenance sidecar from
# one immutable Git commit. Publication and signing are deliberately outside
# this script; source-artifact requires an existing tag before release output
# can be created.
set -eu

usage() {
	echo "usage: $0 --prepare-archive VERSION COMMIT OUTPUT_FILE" >&2
	echo "       $0 [--require-tag] VERSION COMMIT OUTPUT_DIR" >&2
	exit 2
}

require_tag=false
prepare_archive=false
if [ "${1:-}" = "--prepare-archive" ]; then
	prepare_archive=true
	require_tag=true
	shift
elif [ "${1:-}" = "--require-tag" ]; then
	require_tag=true
	shift
fi

[ "$#" -eq 3 ] || usage

version=$1
commit=$2
if [ "$prepare_archive" = true ]; then
	archive_output=$3
else
	output=$3
fi

number='(0|[1-9][0-9]*)'
prerelease='(0|[1-9][0-9]*|[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*)'
build='[0-9A-Za-z-]+'
printf '%s\n' "$version" | grep -Eq "^v${number}\\.${number}\\.${number}(-${prerelease}(\\.${prerelease})*)?(\\+${build}(\\.${build})*)?$" || {
	echo "source-artifact: invalid semantic version: $version" >&2
	exit 2
}
printf '%s\n' "$commit" | grep -Eq '^[0-9a-f]{40}$' || {
	echo 'source-artifact: COMMIT must be a lowercase 40-character object ID' >&2
	exit 2
}

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
cd "$root"
# shellcheck source=tools/source-artifact-lib.sh
. "$root/tools/source-artifact-lib.sh"
source_artifact_sanitize_git_environment

resolved=$(git rev-parse --verify "$commit^{commit}")
[ "$resolved" = "$commit" ] || {
	echo "source-artifact: $commit does not resolve to itself" >&2
	exit 1
}

required_paths='Makefile
go.mod
go.sum
.github/package-lock.json
.github/workflows/ci.yml
tools/build-source-artifact.sh
tools/check-source-artifact.sh
tools/source-artifact-bootstrap.sh
tools/source-artifact-lib.sh
tools/source-artifact-toolchain.txt
tools/test-source-artifact.sh
tools/sqlite-driver-bakeoff/go.mod
tools/sqlite-driver-bakeoff/go.sum'
printf '%s\n' "$required_paths" | while IFS= read -r path; do
	git cat-file -e "$commit:$path" 2>/dev/null || {
		echo "source-artifact: required commit blob is missing: $path" >&2
		exit 1
	}
done

if git_context_problem=$(source_artifact_git_context_problem); then
	echo "source-artifact: unsafe Git context: $git_context_problem" >&2
	exit 1
fi
if source_artifact_has_reserved_archive_attributes "$commit"; then
	echo 'source-artifact: export-ignore and export-subst attributes are not allowed' >&2
	exit 1
else
	attribute_status=$?
	[ "$attribute_status" -eq 1 ] || {
		echo 'source-artifact: could not inspect committed Git attributes' >&2
		exit 1
	}
fi
if source_artifact_has_gitlinks "$commit"; then
	echo 'source-artifact: gitlinks are not supported by the complete-tree source artifact' >&2
	exit 1
else
	gitlink_status=$?
	[ "$gitlink_status" -eq 1 ] || {
		echo 'source-artifact: could not inspect the source tree for gitlinks' >&2
		exit 1
	}
fi

toolchain_field() {
	git show "$commit:tools/source-artifact-toolchain.txt" | awk -v key="$1" '
		index($0, key ": ") == 1 {
			count++
			value = substr($0, length(key) + 3)
		}
		END { if (count != 1 || value == "") exit 1; print value }
	' || {
		echo "source-artifact: release toolchain field is missing, empty, or duplicated: $1" >&2
		exit 1
	}
}
[ "$(toolchain_field format)" = yomihon-source-artifact-toolchain-v1 ] || {
	echo 'source-artifact: unknown release toolchain format' >&2
	exit 1
}
pinned_git_version=$(toolchain_field git-version)
pinned_gzip_version=$(toolchain_field gzip-version)

sha256_stream() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum | awk '{print $1}'
	elif command -v shasum >/dev/null 2>&1; then
		shasum -a 256 | awk '{print $1}'
	else
		echo 'source-artifact: sha256sum or shasum is required' >&2
		exit 1
	fi
}

blob_sha256() {
	git show "$commit:$1" | sha256_stream
}

write_source_archive() {
	sa_archive_path=$1
	sa_archive_scratch=$2
	sa_archive_prefix="yomihon-${version#v}/"
	sa_temporary_tar="$sa_archive_scratch/source.tar"
	source_artifact_make_archive "$commit" "$sa_archive_prefix" "$sa_temporary_tar" "$sa_archive_scratch/git" || {
		echo 'source-artifact: isolated Git archive failed' >&2
		exit 1
	}
	env GZIP= gzip -n -c "$sa_temporary_tar" >"$sa_archive_path"
	rm -f "$sa_temporary_tar"
	sa_archive_commit=$(gzip -dc "$sa_archive_path" | git get-tar-commit-id)
	[ "$sa_archive_commit" = "$commit" ] || {
		echo 'source-artifact: archive commit identity is missing or incorrect' >&2
		exit 1
	}
	chmod 0644 "$sa_archive_path"
}

artifact_class=verification-fixture
tag_object=not-required
if [ "$require_tag" = true ]; then
	artifact_class=release
	tag_object=$(git rev-parse --verify "refs/tags/$version" 2>/dev/null || true)
fi

check_release_source() {
	[ "$require_tag" = true ] || return 0
	[ "$(git cat-file -t "refs/tags/$version" 2>/dev/null || true)" = tag ] || {
		echo "source-artifact: $version must be an annotated tag" >&2
		exit 1
	}
	[ "$(git rev-parse --verify "refs/tags/$version")" = "$tag_object" ] || {
		echo "source-artifact: tag object changed while building $version" >&2
		exit 1
	}
	[ "$(git cat-file tag "$tag_object" | awk 'NF == 0 { exit } $1 == "tag" && NF == 2 { count++; value = $2 } END { if (count != 1) exit 1; print value }')" = "$version" ] || {
		echo "source-artifact: annotated tag object name does not match $version" >&2
		exit 1
	}
	[ "$(git rev-parse --verify "refs/tags/$version^{commit}")" = "$commit" ] || {
		echo "source-artifact: tag $version does not identify $commit" >&2
		exit 1
	}
	[ "$(git rev-parse HEAD)" = "$commit" ] || {
		echo 'source-artifact: release checkout HEAD does not match SOURCE_COMMIT' >&2
		exit 1
	}
	[ -z "$(git status --porcelain --untracked-files=all)" ] || {
		echo 'source-artifact: release checkout is not clean' >&2
		exit 1
	}
	[ "$(git version)" = "$pinned_git_version" ] || {
		echo "source-artifact: release requires $pinned_git_version" >&2
		exit 1
	}
	[ "$(gzip --version 2>&1 | sed -n '1p')" = "$pinned_gzip_version" ] || {
		echo "source-artifact: release requires $pinned_gzip_version" >&2
		exit 1
	}
}
check_release_source

if [ "$prepare_archive" = true ]; then
	case "$archive_output" in
	/*) ;;
	*) archive_output=$root/$archive_output ;;
	esac
	archive_parent=$(dirname "$archive_output")
	archive_name=${archive_output##*/}
	[ -n "$archive_name" ] || usage
	if [ -e "$archive_parent" ] || [ -L "$archive_parent" ]; then
		[ -d "$archive_parent" ] && [ ! -L "$archive_parent" ] || {
			echo "source-artifact: archive output parent is not a real directory: $archive_parent" >&2
			exit 1
		}
	else
		mkdir -m 0755 "$archive_parent"
	fi
	[ ! -e "$archive_output" ] && [ ! -L "$archive_output" ] || {
		echo "source-artifact: refusing to overwrite $archive_output" >&2
		exit 1
	}
	archive_stage=$(mktemp -d "$archive_parent/.${archive_name}.XXXXXX")
	trap 'rm -rf "$archive_stage"' 0 HUP INT TERM
	write_source_archive "$archive_stage/$archive_name" "$archive_stage/context"
	check_release_source
	[ ! -e "$archive_output" ] && [ ! -L "$archive_output" ] || {
		echo "source-artifact: archive destination appeared before publication: $archive_output" >&2
		exit 1
	}
	# The stage and destination share a parent filesystem. A hard link is the
	# portable no-replace publication primitive: link(2) fails atomically when
	# any destination entry already exists, including a dangling symlink.
	ln "$archive_stage/$archive_name" "$archive_output" || {
		echo "source-artifact: archive destination appeared before publication: $archive_output" >&2
		exit 1
	}
	rm "$archive_stage/$archive_name"
	archive_sha=$(sha256_stream <"$archive_output")
	echo "source-artifact: prepared review candidate $archive_output (sha256:$archive_sha)"
	exit 0
fi

scratch_dir=$(mktemp -d "${TMPDIR:-/tmp}/yomihon-source-archive.XXXXXX")
stage=
publication_lock=
cleanup() {
	[ -z "$stage" ] || rm -rf "$stage"
	[ -z "$publication_lock" ] || rmdir "$publication_lock" 2>/dev/null || true
	rm -rf "$scratch_dir"
}
trap cleanup 0 HUP INT TERM

bundle_name="yomihon-$version"
if [ -e "$output" ] || [ -L "$output" ]; then
	[ -d "$output" ] && [ ! -L "$output" ] || {
		echo "source-artifact: output parent is not a real directory: $output" >&2
		exit 1
	}
else
	mkdir -m 0755 "$output"
fi

destination="$output/$bundle_name"
publication_lock="$output/.${bundle_name}.lock"
mkdir -m 0700 "$publication_lock" 2>/dev/null || {
	echo "source-artifact: publication lock is already held: $publication_lock" >&2
	exit 1
}
[ ! -e "$destination" ] && [ ! -L "$destination" ] || {
	echo "source-artifact: refusing to overwrite $destination" >&2
	exit 1
}

stage=$(mktemp -d "$output/.${bundle_name}.XXXXXX")

archive_name="yomihon-$version.tar.gz"
provenance_name="yomihon-$version.provenance"
manifest_name="yomihon-$version-SHA256SUMS"

write_source_archive "$stage/$archive_name" "$scratch_dir/archive-context"
chmod 0644 "$stage/$archive_name"

archive_sha=$(sha256_stream <"$stage/$archive_name")
tree=$(git rev-parse "$commit^{tree}")

cat >"$stage/$provenance_name" <<EOF
format: yomihon-source-provenance-v2
artifact-class: $artifact_class
version: $version
source-commit: $commit
source-tree: $tree
release-tag-object: $tag_object
archive: $archive_name
archive-sha256: $archive_sha
source-artifact-toolchain-sha256: $(blob_sha256 tools/source-artifact-toolchain.txt)
source-artifact-bootstrap-sha256: $(blob_sha256 tools/source-artifact-bootstrap.sh)
go-mod-sha256: $(blob_sha256 go.mod)
go-sum-sha256: $(blob_sha256 go.sum)
frontend-lock-sha256: $(blob_sha256 .github/package-lock.json)
bakeoff-go-mod-sha256: $(blob_sha256 tools/sqlite-driver-bakeoff/go.mod)
bakeoff-go-sum-sha256: $(blob_sha256 tools/sqlite-driver-bakeoff/go.sum)
ci-workflow-sha256: $(blob_sha256 .github/workflows/ci.yml)
git-version: $(git version)
gzip-version: $(gzip --version 2>&1 | sed -n '1p')
EOF

chmod 0644 "$stage/$provenance_name"
provenance_sha=$(sha256_stream <"$stage/$provenance_name")
{
	printf '%s  %s\n' "$archive_sha" "$archive_name"
	printf '%s  %s\n' "$provenance_sha" "$provenance_name"
} >"$stage/$manifest_name"
chmod 0644 "$stage/$manifest_name"

check_bundle() {
	if [ "$artifact_class" = release ]; then
		sh tools/check-source-artifact.sh "$version" "$commit" "$1"
	else
		sh tools/check-source-artifact.sh --allow-fixture "$version" "$commit" "$1"
	fi
}

check_bundle "$stage"
check_release_source
[ ! -e "$destination" ] && [ ! -L "$destination" ] || {
	echo "source-artifact: destination appeared before publication: $destination" >&2
	exit 1
}
chmod 0755 "$stage"
stage_name=${stage##*/}
if ! mv "$stage" "$destination"; then
	echo "source-artifact: publication rename failed: $destination" >&2
	exit 1
fi
if [ -d "$destination/$stage_name" ]; then
	stage="$destination/$stage_name"
	echo "source-artifact: destination changed during publication: $destination" >&2
	exit 1
fi
[ ! -e "$stage" ] && [ ! -L "$stage" ] || {
	echo "source-artifact: publication did not consume staging directory: $stage" >&2
	exit 1
}
stage=

if ! check_bundle "$destination"; then
	quarantine=$(mktemp -d "$output/.${bundle_name}.failed.XXXXXX") || {
		echo "source-artifact: published bundle failed validation and could not be quarantined: $destination" >&2
		exit 1
	}
	rmdir "$quarantine"
	if mv "$destination" "$quarantine"; then
		echo "source-artifact: published bundle failed validation and was quarantined: $quarantine" >&2
	else
		echo "source-artifact: published bundle failed validation and remains at: $destination" >&2
	fi
	exit 1
fi

echo "source-artifact: published $destination"

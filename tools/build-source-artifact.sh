#!/bin/sh

# Build a deterministic source archive and non-circular provenance sidecar from
# one immutable Git commit. Publication and signing are deliberately outside
# this script; source-artifact requires an existing tag before release output
# can be created.
set -eu

usage() {
	echo "usage: $0 --prepare-archive VERSION COMMIT OUTPUT_FILE" >&2
	echo "       $0 [--require-tag] VERSION COMMIT VERIFICATION_RECORD OUTPUT_DIR" >&2
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

if [ "$prepare_archive" = true ]; then
	[ "$#" -eq 3 ] || usage
else
	[ "$#" -eq 4 ] || usage
fi

version=$1
commit=$2
if [ "$prepare_archive" = true ]; then
	archive_output=$3
else
	evidence=$3
	output=$4
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

required_paths='ENGINEERING_STANDARD.md
ENGINEERING_STANDARD.sha256
PROJECT_PROFILE.md
Makefile
docs/release.md
docs/reviews/REVIEW_REPORT.template.md
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
tools/testdata/source-artifact-approved-profile.md
tools/testdata/source-artifact-review.md.in
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

profile_field() {
	source_artifact_profile_readiness_field "$commit" "$1" || {
		echo "source-artifact: profile readiness envelope is malformed or missing field: $1" >&2
		exit 1
	}
}

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

if [ "$require_tag" = true ]; then
	[ "$(profile_field profile-status)" = APPROVED ] || {
		echo 'source-artifact: formal source artifact requires an approved project profile' >&2
		exit 1
	}
	[ "$(profile_field merge-readiness)" = GO ] || {
		echo 'source-artifact: project profile merge readiness is not GO' >&2
		exit 1
	}
	[ "$(profile_field artifact-build-readiness)" = GO ] || {
		echo 'source-artifact: project profile artifact-build readiness is not GO' >&2
		exit 1
	}
	[ "$(profile_field artifact-build-blockers)" = none ] || {
		echo 'source-artifact: project profile has artifact-build blockers' >&2
		exit 1
	}
	[ "$(profile_field release-readiness)" = PENDING-ARTIFACT ] || {
		echo 'source-artifact: project profile is not in the PENDING-ARTIFACT state' >&2
		exit 1
	}
	post_artifact_blockers=$(profile_field post-artifact-blockers)
	[ "$post_artifact_blockers" != none ] || {
		echo 'source-artifact: project profile names no blocker for artifact evidence to close' >&2
		exit 1
	}
	[ "$(profile_field open-blockers)" = "$post_artifact_blockers" ] || {
		echo 'source-artifact: open profile blockers are not exclusively post-artifact blockers' >&2
		exit 1
	}
	[ "$(profile_field active-exceptions)" = none ] || {
		echo 'source-artifact: project profile has active exceptions' >&2
		exit 1
	}
	source_artifact_profile_approvals_are_final "$commit" || {
		echo 'source-artifact: approved profile has incomplete human approvals' >&2
		exit 1
	}
	[ "$(source_artifact_profile_listed_blockers "$commit")" = "$(profile_field open-blockers)" ] || {
		echo 'source-artifact: profile readiness blockers disagree with the Current blockers table' >&2
		exit 1
	}
fi

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

[ -f "$evidence" ] && [ ! -L "$evidence" ] || {
	echo "source-artifact: verification record is missing, linked, or not a regular file: $evidence" >&2
	exit 1
}
require_safe_review_text() {
	label=$1
	path=$2
	if source_artifact_review_text_is_safe "$path"; then
		return 0
	else
		text_status=$?
	fi
	case "$text_status" in
	1) echo "source-artifact: $label is not safe UTF-8 text" >&2 ;;
	2) echo 'source-artifact: working iconv and od are required to validate review evidence' >&2 ;;
	*) echo "source-artifact: $label validation failed unexpectedly" >&2 ;;
	esac
	exit 1
}
require_safe_review_text 'verification record' "$evidence"

snapshot_dir=$(mktemp -d "${TMPDIR:-/tmp}/yomihon-release-evidence.XXXXXX")
stage=
publication_lock=
cleanup() {
	[ -z "$stage" ] || rm -rf "$stage"
	[ -z "$publication_lock" ] || rmdir "$publication_lock" 2>/dev/null || true
	rm -rf "$snapshot_dir"
}
trap cleanup 0 HUP INT TERM
evidence_snapshot="$snapshot_dir/review.md"
cp "$evidence" "$evidence_snapshot"
cmp -s "$evidence" "$evidence_snapshot" || {
	echo 'source-artifact: verification record changed while it was captured' >&2
	exit 1
}
require_safe_review_text 'captured verification record' "$evidence_snapshot"
evidence=$evidence_snapshot

report_field() {
	awk -v key="$1" '
		index($0, key ": ") == 1 {
			count++
			value = substr($0, length(key) + 3)
			sub(/^[[:space:]]+/, "", value)
			sub(/[[:space:]]+$/, "", value)
		}
		END { if (count != 1 || value == "") exit 1; print value }
	' "$evidence" || {
		echo "source-artifact: review field is missing, empty, or duplicated: $1" >&2
		exit 1
	}
}

reviewed_commit=$(report_field reviewed-commit)
reviewed_profile_sha256=$(report_field project-profile-sha256)
review_class=$(report_field review-class)
certified_scope=$(report_field certified-scope)
closed_profile_blockers=$(report_field closed-profile-blockers)
builder=$(report_field builder)
reviewer=$(report_field independent-reviewer)
[ "$reviewed_commit" = "$commit" ] || {
	echo 'source-artifact: review evidence names a different commit' >&2
	exit 1
}
[ "$reviewed_profile_sha256" = "$(blob_sha256 PROJECT_PROFILE.md)" ] || {
	echo 'source-artifact: review evidence names a different project-profile digest' >&2
	exit 1
}
if [ "$artifact_class" = release ]; then
	[ "$review_class" = release-candidate ] || {
		echo 'source-artifact: formal release requires release-candidate review evidence' >&2
		exit 1
	}
	[ "$certified_scope" = complete-project-profile ] || {
		echo 'source-artifact: formal release requires complete-project-profile certification' >&2
		exit 1
	}
	[ "$closed_profile_blockers" = "$post_artifact_blockers" ] || {
		echo 'source-artifact: review evidence does not close exactly the post-artifact profile blockers' >&2
		exit 1
	}
else
	[ "$review_class" = verification-fixture ] || {
		echo 'source-artifact: verification fixture requires verification-fixture review evidence' >&2
		exit 1
	}
	[ "$certified_scope" = source-artifact-contract ] || {
		echo 'source-artifact: verification fixture requires source-artifact-contract certification' >&2
		exit 1
	}
	[ "$closed_profile_blockers" = none ] || {
		echo 'source-artifact: verification fixture must not close project-profile blockers' >&2
		exit 1
	}
fi
for gate in gate-1 gate-2 gate-3; do
	[ "$(report_field "$gate")" = PASS ] || {
		echo "source-artifact: review evidence does not pass $gate" >&2
		exit 1
	}
done
[ "$(report_field final-verdict)" = GO ] || {
	echo 'source-artifact: review evidence final verdict is not GO' >&2
	exit 1
}
for field in unverified-or-blocked-checks unresolved-owner-decisions active-exceptions; do
	[ "$(report_field "$field")" = none ] || {
		echo "source-artifact: review evidence has an open $field" >&2
		exit 1
	}
done
source_artifact_identity_is_canonical "$builder" || {
	echo 'source-artifact: builder is not a canonical identity token' >&2
	exit 1
}
source_artifact_identity_is_canonical "$reviewer" || {
	echo 'source-artifact: independent reviewer is not a canonical identity token' >&2
	exit 1
}
normalized_builder=$(source_artifact_normalize_identity "$builder")
normalized_reviewer=$(source_artifact_normalize_identity "$reviewer")
[ "$normalized_builder" != "$normalized_reviewer" ] || {
	echo 'source-artifact: builder cannot be the independent reviewer' >&2
	exit 1
}

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
evidence_name="yomihon-$version-verification.md"
manifest_name="yomihon-$version-SHA256SUMS"

write_source_archive "$stage/$archive_name" "$snapshot_dir/archive-context"
review_sha=$(sha256_stream <"$evidence")
report_name="yomihon-$version-review.md"
cp "$evidence" "$stage/$report_name"
cat >"$stage/$evidence_name" <<EOF
format: yomihon-release-certificate-v1
reviewed-commit: $reviewed_commit
review-class: $review_class
certified-scope: $certified_scope
closed-profile-blockers: $closed_profile_blockers
builder: $builder
independent-reviewer: $reviewer
review-report: $report_name
review-report-sha256: $review_sha
gate-1: PASS
gate-2: PASS
gate-3: PASS
final-verdict: GO
unverified-or-blocked-checks: none
unresolved-owner-decisions: none
active-exceptions: none
EOF
chmod 0644 "$stage/$archive_name" "$stage/$evidence_name" "$stage/$report_name"

archive_sha=$(sha256_stream <"$stage/$archive_name")
evidence_sha=$(sha256_stream <"$stage/$evidence_name")
tree=$(git rev-parse "$commit^{tree}")
standard_identity=$(git show "$commit:ENGINEERING_STANDARD.sha256" | awk '$2 == "ENGINEERING_STANDARD.md" {print $1}')
printf '%s\n' "$standard_identity" | grep -Eq '^[0-9a-f]{64}$' || {
	echo 'source-artifact: normative standard identity is malformed' >&2
	exit 1
}
actual_standard_identity=$(blob_sha256 ENGINEERING_STANDARD.md)
[ "$standard_identity" = "$actual_standard_identity" ] || {
	echo 'source-artifact: normative standard bytes do not match their identity' >&2
	exit 1
}

cat >"$stage/$provenance_name" <<EOF
format: yomihon-source-provenance-v1
artifact-class: $artifact_class
version: $version
source-commit: $commit
source-tree: $tree
release-tag-object: $tag_object
archive: $archive_name
archive-sha256: $archive_sha
verification-evidence: $evidence_name
verification-evidence-sha256: $evidence_sha
review-report: $report_name
review-report-sha256: $review_sha
engineering-standard-sha256: $standard_identity
project-profile-sha256: $(blob_sha256 PROJECT_PROFILE.md)
review-template-sha256: $(blob_sha256 docs/reviews/REVIEW_REPORT.template.md)
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
	printf '%s  %s\n' "$evidence_sha" "$evidence_name"
	printf '%s  %s\n' "$review_sha" "$report_name"
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

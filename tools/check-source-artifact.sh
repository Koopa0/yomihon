#!/bin/sh

# Validate one complete source-release bundle. The bundle is valid only as a
# set: the deterministic source archive, its machine-readable provenance, and
# the checksum manifest that covers them. This checker is also run before the
# builder publishes the staging directory.
set -eu

allow_fixture=false
if [ "${1:-}" = --allow-fixture ]; then
	allow_fixture=true
	shift
fi

[ "$#" -eq 3 ] || {
	echo "usage: $0 [--allow-fixture] VERSION COMMIT BUNDLE_DIR" >&2
	exit 2
}

version=$1
commit=$2
bundle=$3

case "$bundle" in
/*) ;;
*) bundle=$PWD/$bundle ;;
esac
root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
cd "$root"
# shellcheck source=tools/source-artifact-lib.sh
. "$root/tools/source-artifact-lib.sh"
source_artifact_sanitize_git_environment

fail() {
	echo "source-artifact-check: $*" >&2
	exit 1
}

printf '%s\n' "$commit" | grep -Eq '^[0-9a-f]{40}$' || fail "COMMIT is not a lowercase 40-character object ID"
[ "$(git rev-parse --verify "$commit^{commit}" 2>/dev/null || true)" = "$commit" ] || fail "COMMIT does not resolve to itself"
if git_context_problem=$(source_artifact_git_context_problem); then
	fail "unsafe Git context: $git_context_problem"
fi
if source_artifact_has_reserved_archive_attributes "$commit"; then
	fail "export-ignore and export-subst attributes are not allowed"
else
	attribute_status=$?
	[ "$attribute_status" -eq 1 ] || fail "could not inspect committed Git attributes"
fi
if source_artifact_has_gitlinks "$commit"; then
	fail "gitlinks are not supported by the complete-tree source artifact"
else
	gitlink_status=$?
	[ "$gitlink_status" -eq 1 ] || fail "could not inspect the source tree for gitlinks"
fi

[ -d "$bundle" ] && [ ! -L "$bundle" ] || fail "bundle is not a real directory: $bundle"

archive_name="yomihon-$version.tar.gz"
provenance_name="yomihon-$version.provenance"
manifest_name="yomihon-$version-SHA256SUMS"

for name in "$archive_name" "$provenance_name" "$manifest_name"; do
	[ -f "$bundle/$name" ] && [ ! -L "$bundle/$name" ] || fail "missing or linked bundle file: $name"
done

count=0
for path in "$bundle"/* "$bundle"/.[!.]* "$bundle"/..?*; do
	[ -e "$path" ] || [ -L "$path" ] || continue
	count=$((count + 1))
	case "${path##*/}" in
	"$archive_name" | "$provenance_name" | "$manifest_name") ;;
	*) fail "unexpected bundle file: ${path##*/}" ;;
	esac
done
[ "$count" -eq 3 ] || fail "bundle must contain exactly three files"

manifest="$bundle/$manifest_name"
[ "$(wc -l <"$manifest" | tr -d ' ')" -eq 2 ] || fail "manifest must contain exactly two rows"
for name in "$archive_name" "$provenance_name"; do
	awk -v name="$name" '
		NF == 2 && length($1) == 64 && $1 !~ /[^0-9a-f]/ && $2 == name { count++ }
		END { exit count != 1 }
	' "$manifest" || fail "manifest does not cover $name exactly once"
done

if command -v sha256sum >/dev/null 2>&1; then
	(cd "$bundle" && sha256sum -c "$manifest_name") >/dev/null || fail "bundle checksum mismatch"
elif command -v shasum >/dev/null 2>&1; then
	(cd "$bundle" && shasum -a 256 -c "$manifest_name") >/dev/null || fail "bundle checksum mismatch"
else
	fail "sha256sum or shasum is required"
fi

sha256_file() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | awk '{print $1}'
	else
		shasum -a 256 "$1" | awk '{print $1}'
	fi
}

sha256_blob() {
	git cat-file -e "$commit:$1" 2>/dev/null || fail "commit blob is missing: $1"
	if command -v sha256sum >/dev/null 2>&1; then
		git show "$commit:$1" | sha256sum | awk '{print $1}'
	else
		git show "$commit:$1" | shasum -a 256 | awk '{print $1}'
	fi
}

provenance="$bundle/$provenance_name"
provenance_field() {
	awk -v key="$1" '
		index($0, key ": ") == 1 { count++; value = substr($0, length(key) + 3) }
		END { if (count != 1 || value == "") exit 1; print value }
	' "$provenance" || fail "provenance field is missing, empty, or duplicated: $1"
}

provenance_count=0
while IFS= read -r line; do
	key=${line%%:*}
	case "$key" in
	format | artifact-class | version | source-commit | source-tree | release-tag-object | archive | archive-sha256 | source-artifact-toolchain-sha256 | source-artifact-bootstrap-sha256 | go-mod-sha256 | go-sum-sha256 | frontend-lock-sha256 | bakeoff-go-mod-sha256 | bakeoff-go-sum-sha256 | ci-workflow-sha256 | git-version | gzip-version) ;;
	*) fail "unknown provenance field: $key" ;;
	esac
	provenance_count=$((provenance_count + 1))
done <"$provenance"
[ "$provenance_count" -eq 18 ] || fail "provenance must contain exactly 18 fields"

[ "$(provenance_field format)" = yomihon-source-provenance-v2 ] || fail "unknown provenance format"
artifact_class=$(provenance_field artifact-class)
[ "$artifact_class" = release ] || [ "$artifact_class" = verification-fixture ] || fail "unknown artifact class"
[ "$artifact_class" = release ] || [ "$allow_fixture" = true ] || fail "verification fixture requires --allow-fixture"
[ "$artifact_class" = verification-fixture ] || [ "$allow_fixture" = false ] || fail "formal release must not be checked as a fixture"
[ "$(provenance_field version)" = "$version" ] || fail "provenance version mismatch"
[ "$(provenance_field source-commit)" = "$commit" ] || fail "provenance commit mismatch"
[ "$(provenance_field source-tree)" = "$(git rev-parse "$commit^{tree}")" ] || fail "provenance tree mismatch"
tag_object=$(provenance_field release-tag-object)
if [ "$artifact_class" = release ]; then
	printf '%s\n' "$tag_object" | grep -Eq '^[0-9a-f]{40}$' || fail "release tag object is malformed"
	[ "$(git cat-file -t "$tag_object" 2>/dev/null || true)" = tag ] || fail "release tag object is not an annotated tag"
	[ "$(git rev-parse "$tag_object^{commit}")" = "$commit" ] || fail "release tag object does not identify the source commit"
	[ "$(git cat-file tag "$tag_object" | awk 'NF == 0 { exit } $1 == "tag" && NF == 2 { count++; value = $2 } END { if (count != 1) exit 1; print value }')" = "$version" ] || fail "annotated tag object name does not match version"
	[ "$(git rev-parse --verify "refs/tags/$version" 2>/dev/null || true)" = "$tag_object" ] || fail "release tag ref does not identify the recorded tag object"
else
	[ "$tag_object" = not-required ] || fail "verification fixture must not claim a release tag object"
fi
[ "$(provenance_field archive)" = "$archive_name" ] || fail "provenance archive name mismatch"
[ "$(provenance_field archive-sha256)" = "$(sha256_file "$bundle/$archive_name")" ] || fail "provenance archive digest mismatch"
[ "$(provenance_field source-artifact-toolchain-sha256)" = "$(sha256_blob tools/source-artifact-toolchain.txt)" ] || fail "source artifact toolchain digest mismatch"
[ "$(provenance_field source-artifact-bootstrap-sha256)" = "$(sha256_blob tools/source-artifact-bootstrap.sh)" ] || fail "source artifact bootstrap digest mismatch"
[ "$(provenance_field go-mod-sha256)" = "$(sha256_blob go.mod)" ] || fail "go.mod digest mismatch"
[ "$(provenance_field go-sum-sha256)" = "$(sha256_blob go.sum)" ] || fail "go.sum digest mismatch"
[ "$(provenance_field frontend-lock-sha256)" = "$(sha256_blob .github/package-lock.json)" ] || fail "frontend lock digest mismatch"
[ "$(provenance_field bakeoff-go-mod-sha256)" = "$(sha256_blob tools/sqlite-driver-bakeoff/go.mod)" ] || fail "bake-off go.mod digest mismatch"
[ "$(provenance_field bakeoff-go-sum-sha256)" = "$(sha256_blob tools/sqlite-driver-bakeoff/go.sum)" ] || fail "bake-off go.sum digest mismatch"
[ "$(provenance_field ci-workflow-sha256)" = "$(sha256_blob .github/workflows/ci.yml)" ] || fail "CI workflow digest mismatch"
[ -n "$(provenance_field git-version)" ] || fail "Git builder version is missing"
[ -n "$(provenance_field gzip-version)" ] || fail "gzip builder version is missing"

toolchain_field() {
	git show "$commit:tools/source-artifact-toolchain.txt" | awk -v key="$1" '
		index($0, key ": ") == 1 { count++; value = substr($0, length(key) + 3) }
		END { if (count != 1 || value == "") exit 1; print value }
	' || fail "source artifact toolchain field is missing, empty, or duplicated: $1"
}
[ "$(toolchain_field format)" = yomihon-source-artifact-toolchain-v1 ] || fail "unknown source artifact toolchain format"
if [ "$artifact_class" = release ]; then
	[ "$(provenance_field git-version)" = "$(toolchain_field git-version)" ] || fail "release Git version does not match the pinned toolchain"
	[ "$(provenance_field gzip-version)" = "$(toolchain_field gzip-version)" ] || fail "release gzip version does not match the pinned toolchain"
fi

tmp=$(mktemp -d "${TMPDIR:-/tmp}/yomihon-source-artifact-check.XXXXXX")
cleanup() {
	rm -rf "$tmp"
}
trap cleanup 0 HUP INT TERM
prefix="yomihon-${version#v}/"
source_artifact_make_archive "$commit" "$prefix" "$tmp/expected.tar" "$tmp/archive-context" || fail "isolated Git archive failed"
gzip -dc "$bundle/$archive_name" >"$tmp/actual.tar" || fail "archive gzip stream is invalid"
cmp -s "$tmp/expected.tar" "$tmp/actual.tar" || fail "archive is not the complete commit tree"
source_artifact_tree_paths "$commit" "$tmp/tree.paths" || fail "source tree contains an unsupported control- or escape-sensitive path"
source_artifact_archive_paths "$tmp/actual.tar" "$prefix" "$tmp/archive.paths" || fail "could not enumerate source archive paths"
LC_ALL=C sort "$tmp/tree.paths" >"$tmp/tree.sorted"
LC_ALL=C sort "$tmp/archive.paths" >"$tmp/archive.sorted"
cmp -s "$tmp/tree.sorted" "$tmp/archive.sorted" || fail "archive entry set does not match the complete commit tree"

echo "source-artifact-check: verified $bundle"

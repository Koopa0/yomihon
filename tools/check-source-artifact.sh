#!/bin/sh

# Validate one complete source-release bundle. The bundle is valid only as a
# set: archive, provenance, independent verification record, and checksum
# manifest. This checker is also run before the builder publishes the staging
# directory.
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

require_safe_review_text() {
	label=$1
	path=$2
	if source_artifact_review_text_is_safe "$path"; then
		return 0
	else
		text_status=$?
	fi
	case "$text_status" in
	1) fail "$label is not safe UTF-8 text" ;;
	2) fail "working iconv and od are required to validate review evidence" ;;
	*) fail "$label validation failed unexpectedly" ;;
	esac
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
evidence_name="yomihon-$version-verification.md"
report_name="yomihon-$version-review.md"
manifest_name="yomihon-$version-SHA256SUMS"

for name in "$archive_name" "$provenance_name" "$evidence_name" "$report_name" "$manifest_name"; do
	[ -f "$bundle/$name" ] && [ ! -L "$bundle/$name" ] || fail "missing or linked bundle file: $name"
done

count=0
for path in "$bundle"/* "$bundle"/.[!.]* "$bundle"/..?*; do
	[ -e "$path" ] || [ -L "$path" ] || continue
	count=$((count + 1))
	case "${path##*/}" in
	"$archive_name" | "$provenance_name" | "$evidence_name" | "$report_name" | "$manifest_name") ;;
	*) fail "unexpected bundle file: ${path##*/}" ;;
	esac
done
[ "$count" -eq 5 ] || fail "bundle must contain exactly five files"

manifest="$bundle/$manifest_name"
[ "$(wc -l <"$manifest" | tr -d ' ')" -eq 4 ] || fail "manifest must contain exactly four rows"
for name in "$archive_name" "$provenance_name" "$evidence_name" "$report_name"; do
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

profile_field() {
	source_artifact_profile_readiness_field "$commit" "$1" || fail "profile readiness envelope is malformed or missing field: $1"
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
	format | artifact-class | version | source-commit | source-tree | release-tag-object | archive | archive-sha256 | verification-evidence | verification-evidence-sha256 | review-report | review-report-sha256 | engineering-standard-sha256 | project-profile-sha256 | review-template-sha256 | source-artifact-toolchain-sha256 | source-artifact-bootstrap-sha256 | go-mod-sha256 | go-sum-sha256 | frontend-lock-sha256 | bakeoff-go-mod-sha256 | bakeoff-go-sum-sha256 | ci-workflow-sha256 | git-version | gzip-version) ;;
	*) fail "unknown provenance field: $key" ;;
	esac
	provenance_count=$((provenance_count + 1))
done <"$provenance"
[ "$provenance_count" -eq 25 ] || fail "provenance must contain exactly 25 fields"

[ "$(provenance_field format)" = yomihon-source-provenance-v1 ] || fail "unknown provenance format"
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
[ "$(provenance_field verification-evidence)" = "$evidence_name" ] || fail "provenance evidence name mismatch"
[ "$(provenance_field verification-evidence-sha256)" = "$(sha256_file "$bundle/$evidence_name")" ] || fail "provenance evidence digest mismatch"
[ "$(provenance_field review-report)" = "$report_name" ] || fail "provenance review-report name mismatch"
[ "$(provenance_field review-report-sha256)" = "$(sha256_file "$bundle/$report_name")" ] || fail "provenance review-report digest mismatch"
[ "$(provenance_field engineering-standard-sha256)" = "$(sha256_blob ENGINEERING_STANDARD.md)" ] || fail "engineering standard digest mismatch"
[ "$(provenance_field project-profile-sha256)" = "$(sha256_blob PROJECT_PROFILE.md)" ] || fail "profile digest mismatch"
[ "$(provenance_field review-template-sha256)" = "$(sha256_blob docs/reviews/REVIEW_REPORT.template.md)" ] || fail "review template digest mismatch"
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
	[ "$(profile_field profile-status)" = APPROVED ] || fail "formal source artifact requires an approved project profile"
	[ "$(profile_field merge-readiness)" = GO ] || fail "project profile merge readiness is not GO"
	[ "$(profile_field artifact-build-readiness)" = GO ] || fail "project profile artifact-build readiness is not GO"
	[ "$(profile_field artifact-build-blockers)" = none ] || fail "project profile has artifact-build blockers"
	[ "$(profile_field release-readiness)" = PENDING-ARTIFACT ] || fail "project profile is not in the PENDING-ARTIFACT state"
	post_artifact_blockers=$(profile_field post-artifact-blockers)
	[ "$post_artifact_blockers" != none ] || fail "project profile names no blocker for artifact evidence to close"
	[ "$(profile_field open-blockers)" = "$post_artifact_blockers" ] || fail "open profile blockers are not exclusively post-artifact blockers"
	[ "$(profile_field active-exceptions)" = none ] || fail "project profile has active exceptions"
	source_artifact_profile_approvals_are_final "$commit" || fail "approved profile has incomplete human approvals"
	[ "$(source_artifact_profile_listed_blockers "$commit")" = "$(profile_field open-blockers)" ] || fail "profile readiness blockers disagree with the Current blockers table"
fi

standard_identity=$(git show "$commit:ENGINEERING_STANDARD.sha256" | awk '$2 == "ENGINEERING_STANDARD.md" {print $1}')
[ "$(provenance_field engineering-standard-sha256)" = "$standard_identity" ] || fail "engineering-standard identity mismatch"
[ "$standard_identity" = "$(sha256_blob ENGINEERING_STANDARD.md)" ] || fail "engineering-standard bytes do not match their identity"

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

evidence="$bundle/$evidence_name"
require_safe_review_text "release certificate" "$evidence"
evidence_field() {
	awk -v key="$1" '
		index($0, key ": ") == 1 {
			count++
			value = substr($0, length(key) + 3)
			sub(/^[[:space:]]+/, "", value)
			sub(/[[:space:]]+$/, "", value)
		}
		END { if (count != 1 || value == "") exit 1; print value }
	' "$evidence" || fail "release certificate field is missing, empty, or duplicated: $1"
}

[ "$(wc -l <"$evidence" | tr -d ' ')" -eq 16 ] || fail "release certificate must contain exactly 16 fields"
[ "$(evidence_field format)" = yomihon-release-certificate-v1 ] || fail "unknown release certificate format"
[ "$(evidence_field reviewed-commit)" = "$commit" ] || fail "release certificate commit mismatch"
review_class=$(evidence_field review-class)
certified_scope=$(evidence_field certified-scope)
closed_profile_blockers=$(evidence_field closed-profile-blockers)
if [ "$artifact_class" = release ]; then
	[ "$review_class" = release-candidate ] || fail "formal release requires release-candidate review evidence"
	[ "$certified_scope" = complete-project-profile ] || fail "formal release requires complete-project-profile certification"
	[ "$closed_profile_blockers" = "$post_artifact_blockers" ] || fail "release certificate does not close exactly the post-artifact profile blockers"
else
	[ "$review_class" = verification-fixture ] || fail "verification fixture requires verification-fixture review evidence"
	[ "$certified_scope" = source-artifact-contract ] || fail "verification fixture requires source-artifact-contract certification"
	[ "$closed_profile_blockers" = none ] || fail "verification fixture must not close project-profile blockers"
fi
[ "$(evidence_field review-report)" = "$report_name" ] || fail "release certificate report name mismatch"
[ "$(evidence_field review-report-sha256)" = "$(sha256_file "$bundle/$report_name")" ] || fail "release certificate report digest mismatch"
[ "$(evidence_field gate-1)" = PASS ] || fail "release certificate Gate 1 is not PASS"
[ "$(evidence_field gate-2)" = PASS ] || fail "release certificate Gate 2 is not PASS"
[ "$(evidence_field gate-3)" = PASS ] || fail "release certificate Gate 3 is not PASS"
[ "$(evidence_field final-verdict)" = GO ] || fail "release certificate final verdict is not GO"
[ "$(evidence_field unverified-or-blocked-checks)" = none ] || fail "release certificate has unverified or blocked checks"
[ "$(evidence_field unresolved-owner-decisions)" = none ] || fail "release certificate has unresolved owner decisions"
[ "$(evidence_field active-exceptions)" = none ] || fail "release certificate has active exceptions"
printf '%s\n' "$(evidence_field review-report-sha256)" | grep -Eq '^[0-9a-f]{64}$' || fail "release certificate report digest is malformed"

builder=$(evidence_field builder)
reviewer=$(evidence_field independent-reviewer)
source_artifact_identity_is_canonical "$builder" || fail "builder is not a canonical identity token"
source_artifact_identity_is_canonical "$reviewer" || fail "independent reviewer is not a canonical identity token"
normalized_builder=$(source_artifact_normalize_identity "$builder")
normalized_reviewer=$(source_artifact_normalize_identity "$reviewer")
[ "$normalized_builder" != "$normalized_reviewer" ] || fail "builder cannot certify the independent review"
case "$(printf '%s' "$normalized_builder" | tr '[:upper:]' '[:lower:]')" in
pending | todo | tbd | replace-* | list-*) fail "verification evidence has a placeholder builder" ;;
esac
case "$(printf '%s' "$normalized_reviewer" | tr '[:upper:]' '[:lower:]')" in
pending | todo | tbd | replace-* | list-*) fail "verification evidence has a placeholder reviewer" ;;
esac

report="$bundle/$report_name"
require_safe_review_text "review report" "$report"
report_field() {
	awk -v key="$1" '
		index($0, key ": ") == 1 {
			count++
			value = substr($0, length(key) + 3)
			sub(/^[[:space:]]+/, "", value)
			sub(/[[:space:]]+$/, "", value)
		}
		END { if (count != 1 || value == "") exit 1; print value }
	' "$report" || fail "review-report release field is missing, empty, or duplicated: $1"
}

section_field() {
	awk -v heading="$1" -v key="$2" '
		$0 == heading { headings++; inside = 1; next }
		inside && /^# / { inside = 0 }
		inside && index($0, key ": ") == 1 {
			count++
			value = substr($0, length(key) + 3)
			sub(/[[:space:]]+$/, "", value)
		}
		END { if (headings != 1 || count != 1 || value == "") exit 1; print value }
	' "$report" || fail "review-report section field is missing, empty, or duplicated: $1 / $2"
}

section_bullet_field() {
	awk -v heading="$1" -v key="$2" '
		$0 == heading { headings++; inside = 1; next }
		inside && /^# / { inside = 0 }
		inside && index($0, "- " key ": ") == 1 {
			count++
			value = substr($0, length(key) + 5)
			sub(/[[:space:]]+$/, "", value)
		}
		END { if (headings != 1 || count != 1 || value == "") exit 1; print value }
	' "$report" || fail "review-report bullet field is missing, empty, or duplicated: $1 / $2"
}

require_section_fields() {
	heading=$1
	while IFS= read -r field; do
		[ -n "$field" ] || continue
		section_field "$heading" "$field" >/dev/null
	done
}

require_section_bullets() {
	heading=$1
	while IFS= read -r field; do
		[ -n "$field" ] || continue
		section_bullet_field "$heading" "$field" >/dev/null
	done
}

table_field() {
	awk -F '|' -v heading="$1" -v row="$2" -v column="$3" '
		$0 == heading { headings++; inside = 1; next }
		inside && /^# / { inside = 0 }
		inside && /^\|/ {
			label = $2
			gsub(/^[[:space:]]+|[[:space:]]+$/, "", label)
			if (label == row) {
				count++
				value = $(column + 1)
				gsub(/^[[:space:]]+|[[:space:]]+$/, "", value)
			}
		}
		END { if (headings != 1 || count != 1 || value == "") exit 1; print value }
	' "$report" || fail "review-report table field is missing, empty, or duplicated: $1 / $2 / $3"
}

require_complete_table() {
	heading=$1
	header=$2
	awk -F '|' -v heading="$heading" -v header="$header" '
		$0 == heading { headings++; inside = 1; next }
		inside && /^# / { inside = 0 }
		inside && /^\|/ {
			first = $2
			gsub(/^[[:space:]]+|[[:space:]]+$/, "", first)
			if (first == header) {
				headers++
				target = 1
				next
			}
			if (target && first !~ /^-+$/) {
				complete = 1
				for (i = 2; i < NF; i++) {
					value = $i
					gsub(/^[[:space:]]+|[[:space:]]+$/, "", value)
					if (value == "") complete = 0
				}
				if (complete) rows++
			}
			next
		}
		target && /^[[:space:]]*$/ { target = 0 }
		END { exit headings != 1 || headers != 1 || rows < 1 }
	' "$report" || fail "review report has no completed row in table: $heading / $header"
}

require_table_schema() {
	rts_heading=$1
	rts_header=$2
	rts_separator=$3
	awk -v heading="$rts_heading" -v header="$rts_header" -v separator="$rts_separator" '
		$0 == heading { headings++; inside = 1; next }
		inside && /^# / { inside = 0 }
		inside && $0 == header { headers++; expect_separator = 1; next }
		expect_separator {
			if ($0 != separator) bad = 1
			expect_separator = 0
		}
		END { exit headings != 1 || headers != 1 || expect_separator || bad }
	' "$report" || fail "review report table schema changed: $rts_heading / $rts_header"
}

require_named_table_rows() {
	rnr_heading=$1
	rnr_columns=$2
	while IFS= read -r rnr_label; do
		[ -n "$rnr_label" ] || continue
		awk -F '|' -v heading="$rnr_heading" -v label="$rnr_label" -v columns="$rnr_columns" '
			$0 == heading { headings++; inside = 1; next }
			inside && /^# / { inside = 0 }
			inside && /^\|/ {
				row = $2
				gsub(/^[[:space:]]+|[[:space:]]+$/, "", row)
				if (row != label) next
				matches++
				if (NF != columns + 2) bad = 1
				for (i = 2; i < NF; i++) {
					value = $i
					gsub(/^[[:space:]]+|[[:space:]]+$/, "", value)
					if (value == "") bad = 1
				}
			}
			END { exit headings != 1 || matches != 1 || bad }
		' "$report" || fail "review report canonical table row is missing or incomplete: $rnr_heading / $rnr_label"
	done
}

profile_blocker_closure() {
	awk -F '|' '
		$0 == "# Profile blocker closure" { headings++; inside = 1; next }
		inside && /^# / { inside = 0 }
		inside && /^\|/ {
			id = $2
			disposition = $3
			evidence = $4
			owner = $5
			gsub(/^[[:space:]]+|[[:space:]]+$/, "", id)
			gsub(/^[[:space:]]+|[[:space:]]+$/, "", disposition)
			gsub(/^[[:space:]]+|[[:space:]]+$/, "", evidence)
			gsub(/^[[:space:]]+|[[:space:]]+$/, "", owner)
			if (id == "Blocker ID" || id ~ /^-+$/) next
			rows++
			if (id == "None") {
				none_rows++
				if (disposition != "N/A" || evidence == "" || owner == "") bad = 1
				next
			}
			if (id !~ /^[A-Z][A-Z0-9]*(-[A-Z0-9]+)*$/ || seen[id]++ || disposition != "CLOSED" || evidence == "" || owner == "") bad = 1
			if (value != "") value = value ","
			value = value id
		}
		END {
			if (headings != 1 || rows < 1 || bad || (none_rows && (rows != 1 || value != ""))) exit 1
			if (value == "") value = "none"
			print value
		}
	' "$report" || fail "review report profile-blocker closure is malformed"
}

require_release_safe_table_values() {
	heading=$1
	awk -F '|' -v heading="$heading" '
		$0 == heading { headings++; inside = 1; next }
		inside && /^# / { inside = 0 }
		inside && /^[[:space:]]*$/ {
			for (column in checked) delete checked[column]
			next
		}
		inside && /^\|/ {
			is_separator = 1
			for (i = 2; i < NF; i++) {
				value = $i
				gsub(/^[[:space:]]+|[[:space:]]+$/, "", value)
				if (value !~ /^-+$/) is_separator = 0
			}
			if (is_separator) next
			for (i = 2; i < NF; i++) {
				value = $i
				gsub(/^[[:space:]]+|[[:space:]]+$/, "", value)
				if (value == "Status" || value == "Result" || value == "Disposition" || value == "Gap") {
					checked[i] = 1
					next
				}
				if (!checked[i]) continue
				gsub(/^[`*_[:space:]]+|[`*_[:space:]]+$/, "", value)
				value = toupper(value)
				if (value !~ /^(PASS|VERIFIED|INFERRED|N\/A|NONE|CLOSED|YES|NO)$/) bad++
			}
		}
		END { exit headings != 1 || bad != 0 }
	' "$report" || fail "review report contains a non-final table status: $heading"
}

require_closed_findings() {
	awk -F '|' '
		$0 == "# Findings" { headings++; inside = 1; next }
		inside && /^# / { inside = 0 }
		inside && /^\|/ {
			id = $2
			disposition = $5
			gsub(/^[[:space:]]+|[[:space:]]+$/, "", id)
			gsub(/^[[:space:]]+|[[:space:]]+$/, "", disposition)
			if (id == "ID") {
				headers++
				if (disposition != "Disposition") bad++
				next
			}
			if (id ~ /^-+$/) next
			if (id != "") {
				rows++
				if (disposition != "CLOSED") bad++
			}
		}
		END { exit headings != 1 || headers != 1 || rows < 1 || bad != 0 }
	' "$report" || fail "review report contains a finding that is not CLOSED"
}

require_numbered_objection() {
	awk '
		$0 == "# Initial objections" { headings++; inside = 1; next }
		inside && /^# / { inside = 0 }
		inside && /^[0-9]+\.[[:space:]]+[^[:space:]]/ { rows++ }
		END { exit headings != 1 || rows < 1 }
	' "$report" || fail "review report has no completed initial objection"
}

require_watched_red_row() {
	awk -F '|' '
		$0 == "## Watched-red evidence" { headings++; inside = 1; next }
		inside && /^#/ { inside = 0 }
		inside && /^\|/ {
			invariant = $2
			mutation = $3
			expected = $4
			observed = $5
			names_contract = $6
			restored = $7
			artifact = $8
			gsub(/^[[:space:]]+|[[:space:]]+$/, "", invariant)
			gsub(/^[[:space:]]+|[[:space:]]+$/, "", mutation)
			gsub(/^[[:space:]]+|[[:space:]]+$/, "", expected)
			gsub(/^[[:space:]]+|[[:space:]]+$/, "", observed)
			gsub(/^[[:space:]]+|[[:space:]]+$/, "", names_contract)
			gsub(/^[[:space:]]+|[[:space:]]+$/, "", restored)
			gsub(/^[[:space:]]+|[[:space:]]+$/, "", artifact)
			observed = tolower(observed)
			names_contract = tolower(names_contract)
			restored = tolower(restored)
			if (invariant != "Invariant" && invariant !~ /^-+$/ && mutation != "" && expected != "" &&
			    (observed == "yes" || observed == "pass") &&
			    (names_contract == "yes" || names_contract == "pass") &&
			    (restored == "yes" || restored == "pass") && artifact != "") {
				rows++
				if (rows == 1) value = artifact
			}
		}
		END { if (headings != 1 || rows < 1) exit 1; print value }
	' "$report" || fail "review report has no completed watched-red row"
}

require_gate2_scenario_artifact() {
	rgsa_artifact=$1
	awk -F '|' -v wanted="$rgsa_artifact" '
		$0 == "# Gate 2 — Real-user and third-party-agent usability" { headings++; inside = 1; next }
		inside && /^# / { inside = 0 }
		inside && /^\|/ {
			scenario = $2
			steps = $3
			expected = $4
			observed = $5
			artifact = $7
			status = $8
			gsub(/^[[:space:]]+|[[:space:]]+$/, "", scenario)
			gsub(/^[[:space:]]+|[[:space:]]+$/, "", steps)
			gsub(/^[[:space:]]+|[[:space:]]+$/, "", expected)
			gsub(/^[[:space:]]+|[[:space:]]+$/, "", observed)
			gsub(/^[[:space:]]+|[[:space:]]+$/, "", artifact)
			gsub(/^[[:space:]]+|[[:space:]]+$/, "", status)
			if (scenario != "Scenario" && scenario !~ /^-+$/ && steps != "" && expected != "" && observed != "" && artifact == wanted && status == "PASS") {
				rows++
			}
		}
		END { if (headings != 1 || rows < 1) exit 1 }
	' "$report" || fail "review report has no completed PASS Gate 2 scenario for $rgsa_artifact"
}

top_field() {
	awk -v key="$1" '
		$0 == "# Verdict" { done = 1 }
		!done && index($0, key ": ") == 1 {
			count++
			value = substr($0, length(key) + 3)
			sub(/[[:space:]]+$/, "", value)
		}
		END { if (count != 1 || value == "") exit 1; print value }
	' "$report" || fail "review-report header field is missing, empty, or duplicated: $1"
}

certification_field() {
	awk -v key="$1" '
		$0 == "# Independent reviewer certification" { inside = 1; next }
		inside && $0 == "# Release evidence envelope" { inside = 0 }
		inside && index($0, key ": ") == 1 {
			count++
			value = substr($0, length(key) + 3)
			sub(/[[:space:]]+$/, "", value)
		}
		END { if (count != 1 || value == "") exit 1; print value }
	' "$report" || fail "independent certification field is missing, empty, or duplicated: $1"
}

validate_evidence_reference() {
	label=$1
	value=$2
	case "$value" in
	https://*)
		printf '%s\n' "$value" | grep -Eq '^https://[^[:space:]]+$' || fail "review report $label is not an HTTPS URL or SHA-256 identity"
		url_rest=${value#https://}
		authority=${url_rest%%[/?#]*}
		case "$authority" in
		'' | *@* | *'['* | *']'*) fail "review report $label is not an HTTPS URL or SHA-256 identity" ;;
		esac
		host=$authority
		case "$authority" in
		*:*)
			host=${authority%:*}
			port=${authority##*:}
			case "$host" in *:*) fail "review report $label is not an HTTPS URL or SHA-256 identity" ;; esac
			printf '%s\n' "$port" | grep -Eq '^[0-9]+$' || fail "review report $label is not an HTTPS URL or SHA-256 identity"
			[ "$port" -ge 1 ] 2>/dev/null && [ "$port" -le 65535 ] 2>/dev/null || fail "review report $label is not an HTTPS URL or SHA-256 identity"
			;;
		esac
		normalized_host=$(printf '%s\n' "$host" | tr '[:upper:]' '[:lower:]')
		printf '%s\n' "$normalized_host" | grep -Eq '^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]([a-z0-9-]{0,61}[a-z0-9])?$' ||
			fail "review report $label is not an HTTPS URL or SHA-256 identity"
		if [ "$artifact_class" = release ]; then
			if printf '%s\n' "$normalized_host" | grep -Eiq '(^|\.)(localhost|local|internal|intranet|home|lan|invalid|test|example|onion)$|(^|\.)example\.(com|net|org)$'; then
				fail "review report $label uses a reserved or local HTTPS origin"
			fi
		fi
		;;
	sha256:*)
		printf '%s\n' "${value#sha256:}" | grep -Eq '^[0-9a-f]{64}$' || fail "review report $label is not an HTTPS URL or SHA-256 identity"
		;;
	*) fail "review report $label is not an HTTPS URL or SHA-256 identity" ;;
	esac
	return 0
}

section_status() {
	awk -v heading="$1" '
		$0 == heading { headings++; inside = 1; next }
		inside && /^# / { inside = 0 }
		inside && /^Status: / {
			statuses++
			value = substr($0, 9)
			gsub(/^`|`$/, "", value)
		}
		END { if (headings != 1 || statuses != 1 || value == "") exit 1; print value }
	' "$report" || fail "review-report section status is missing or ambiguous: $1"
}

[ "$(report_field reviewed-commit)" = "$commit" ] || fail "review report commit mismatch"
[ "$(report_field project-profile-sha256)" = "$(sha256_blob PROJECT_PROFILE.md)" ] || fail "review report project-profile digest mismatch"
[ "$(report_field builder)" = "$builder" ] || fail "review report builder mismatch"
[ "$(report_field independent-reviewer)" = "$reviewer" ] || fail "review report reviewer mismatch"
[ "$(report_field gate-1)" = PASS ] || fail "review report release Gate 1 is not PASS"
[ "$(report_field gate-2)" = PASS ] || fail "review report release Gate 2 is not PASS"
[ "$(report_field gate-3)" = PASS ] || fail "review report release Gate 3 is not PASS"
[ "$(report_field final-verdict)" = GO ] || fail "review report release verdict is not GO"
[ "$(report_field unverified-or-blocked-checks)" = none ] || fail "review report names unverified or blocked checks"
[ "$(report_field unresolved-owner-decisions)" = none ] || fail "review report names unresolved owner decisions"
[ "$(report_field active-exceptions)" = none ] || fail "review report names active exceptions"
[ "$(report_field review-class)" = "$review_class" ] || fail "review report class disagrees with the release certificate"
[ "$(report_field certified-scope)" = "$certified_scope" ] || fail "review report certified scope disagrees with the release certificate"
[ "$(report_field closed-profile-blockers)" = "$closed_profile_blockers" ] || fail "review report profile-blocker closure disagrees with the release certificate"
[ "$(top_field 'Project profile revision')" = "$commit" ] || fail "review report profile revision does not identify the reviewed commit"
[ "$(top_field Standard)" = "\`ENGINEERING_STANDARD.md\` version 2.0" ] || fail "review report does not name the normative engineering standard"
[ "$(top_field 'Review class')" = "$review_class" ] || fail "review report header class disagrees with its release envelope"
printf '%s\n' "$(top_field 'Review ID')" | awk '$0 ~ /^`REV-[0-9]{4}-[0-9]{3}`$/ { valid = 1 } END { exit !valid }' || fail "review report ID is malformed"
printf '%s\n' "$(top_field Date)" | grep -Eq '^[0-9]{4}-[0-9]{2}-[0-9]{2}$' || fail "review report date is malformed"
[ -n "$(section_field '# Verdict' 'One-sentence rationale')" ] || fail "review report rationale is empty"
require_section_fields '# Snapshot' <<'EOF'
Scope explicitly reviewed
Scope explicitly not certified
EOF
require_section_fields '# User need, scope, and risk' <<'EOF'
User or operator need
Intended outcome
Non-goals
Risk class
Capability flags affected
Public contracts affected
Durable contracts affected
Security and privacy boundaries affected
Operational boundaries affected
EOF
require_complete_table '# Concept model' 'Field'
require_numbered_objection
require_complete_table '# Authority ledger' 'Behavior or rule'
require_complete_table '# Findings' 'ID'
require_closed_findings
require_complete_table '# Profile blocker closure' 'Blocker ID'
require_complete_table '# State, failure, and event analysis' 'Input / state / interleaving'
require_section_bullets '# Gate 1 — Architecture and open-source engineering quality' <<'EOF'
Package and dependency ownership
Go API and implementation quality
Public, wire, storage, and configuration contracts
Security, privacy, data lifecycle, and supply chain
Documentation, licensing, release, and maintainability
Blockers
EOF
require_section_fields '# Gate 2 — Real-user and third-party-agent usability' <<'EOF'
Cold session 1 operator
Cold session 1 session ID
Cold session 2 operator
Cold session 2 session ID
Both sessions independent from implementation
Both sessions directly executed the named public surfaces
Supported public surfaces used
Environment
Blockers
EOF
require_complete_table '# Gate 3 — Test and evidence-system quality' 'Claim'
require_complete_table '# Gate 3 — Test and evidence-system quality' 'Test class'
section_field '# Gate 3 — Test and evidence-system quality' Blockers >/dev/null
require_complete_table '# Security, privacy, performance, and operations' 'Boundary / workload'
require_section_fields '# Compatibility and release' <<'EOF'
Public API impact
CLI / machine-output impact
Wire impact
Storage / schema impact
Configuration impact
Supported upgrade path
Downgrade behavior
Artifact identity
Release notes
Post-release verification
EOF
require_complete_table '# Contradiction hunt' 'Surface pair'
require_complete_table '# NEEDS-OWNER, exceptions, and blocked evidence' 'Item'

require_table_schema '# Snapshot' '| Item | Value | Status |' '|---|---|---|'
require_table_schema '# Concept model' '| Field | Finding | Status | Authority / evidence |' '|---|---|---|---|'
require_table_schema '# Authority ledger' '| Behavior or rule | Authority class | Source | Status | Notes |' '|---|---|---|---|---|'
require_table_schema '# Findings' '| ID | Severity | Status | Disposition | Location / surface | Defect | Concrete failure scenario | Required action | Owner |' '|---|---|---|---|---|---|---|---|---|'
require_table_schema '# Profile blocker closure' '| Blocker ID | Disposition | Evidence | Owner |' '|---|---|---|---|'
require_table_schema '# State, failure, and event analysis' '| Input / state / interleaving | Result shape | Status / exit | Fault owner | Retry / duplicate | Side effects | Recovery | Evidence |' '|---|---|---|---|---|---|---|---|'
require_table_schema '# Gate 2 — Real-user and third-party-agent usability' '| Scenario | Steps | Expected | Observed | Friction / ambiguity | Artifact | Status |' '|---|---|---|---|---|---|---|'
require_table_schema '# Gate 3 — Test and evidence-system quality' '| Claim | Production entry point and composition crossed | Real dependency / format | Test class | Gap |' '|---|---|---|---|---|'
require_table_schema '# Gate 3 — Test and evidence-system quality' '| Test class | Required | Command / target | Result | Quality assessment |' '|---|---|---|---|---|'
require_table_schema '## Watched-red evidence' '| Invariant | Mutation | Expected failing check | Observed red | Failure names contract | Restored green | Artifact |' '|---|---|---|---|---|---|---|'
require_table_schema '# Security, privacy, performance, and operations' '| Boundary / workload | Authority / budget | Final enforcement / measurement | Bypass or fault search | Exact evidence | Status |' '|---|---|---|---|---|---|'
require_table_schema '# Contradiction hunt' '| Surface pair | Question | Result | Status |' '|---|---|---|---|'
require_table_schema '# NEEDS-OWNER, exceptions, and blocked evidence' '| Item | Status | Why | Options / mitigation | Owner / next gate |' '|---|---|---|---|---|'
require_table_schema '# Verification log' '| Command | Environment | Result | Artifact / excerpt |' '|---|---|---|---|'

require_named_table_rows '# Snapshot' 3 <<'EOF'
Commit
Artifact digest
Worktree
Generated state
Dependency lock state
Go version
OS / architecture
CI run
Active exceptions
EOF
require_named_table_rows '# Concept model' 4 <<'EOF'
Concept / non-concept
Semantic owner
Source of truth
Captured capabilities
Projections / caches
Legal / impossible states
Irreversible effects
Load-bearing invariant
EOF
require_named_table_rows '# Gate 2 — Real-user and third-party-agent usability' 7 <<'EOF'
Clean first use
Experienced use
Missing / malformed configuration
Invalid / ambiguous input
Offline / provider failure
Partial / stale result
Cancellation / retry / recovery
Privacy-sensitive operation
Non-ASCII / long / boundary input
Supported / unsupported platforms
Keyboard / narrow / no-JS
Upgrade / restart
Destructive action / recovery
EOF
require_named_table_rows '# Gate 3 — Test and evidence-system quality' 5 <<'EOF'
Unit / integration / E2E
Golden / data-driven
Fuzz / property
Race / deterministic concurrency
Browser
Migration / compatibility
Fault / robustness
Benchmark / load
Cross-platform
EOF
require_named_table_rows '# Security, privacy, performance, and operations' 6 <<'EOF'
Network egress
Credential use
Personal-data storage
Logs / diagnostics
Files / subprocesses
Performance / capacity
Recovery / rollback
EOF
require_named_table_rows '# Contradiction hunt' 4 <<'EOF'
Canon ↔ code
Code ↔ package ownership
Code ↔ schema / wire / CLI / UI
Code ↔ logs / metrics / tests
Docs ↔ observed use
Fix ↔ unrelated behavior
EOF
for heading in \
	'# Concept model' \
	'# Snapshot' \
	'# Authority ledger' \
	'# Findings' \
	'# Profile blocker closure' \
	'# Gate 2 — Real-user and third-party-agent usability' \
	'# Gate 3 — Test and evidence-system quality' \
	'# Security, privacy, performance, and operations' \
	'# Contradiction hunt' \
	'# NEEDS-OWNER, exceptions, and blocked evidence' \
	'# Verification log'; do
	require_release_safe_table_values "$heading"
done
[ "$(profile_blocker_closure)" = "$closed_profile_blockers" ] || fail "review report profile-blocker closure rows disagree with the release envelope"
[ "$(section_bullet_field '# Gate 1 — Architecture and open-source engineering quality' Blockers)" = none ] || fail "human Gate 1 names blockers while claiming PASS"
[ "$(section_field '# Gate 2 — Real-user and third-party-agent usability' Blockers)" = none ] || fail "human Gate 2 names blockers while claiming PASS"
[ "$(section_field '# Gate 3 — Test and evidence-system quality' Blockers)" = none ] || fail "human Gate 3 names blockers while claiming PASS"
header_reviewer=$(printf '%s\n' "$(top_field Reviewer)" | sed 's/^@//' | tr '[:upper:]' '[:lower:]')
header_builder=$(printf '%s\n' "$(top_field Builder)" | sed 's/^@//' | tr '[:upper:]' '[:lower:]')
[ "$header_reviewer" = "$normalized_reviewer" ] || fail "review report header reviewer mismatch"
[ "$header_builder" = "$normalized_builder" ] || fail "review report header builder mismatch"
[ "$(table_field '# Snapshot' Commit 2)" = "$commit" ] || fail "review report snapshot commit mismatch"
[ "$(table_field '# Snapshot' Commit 3)" = verified ] || fail "review report snapshot commit is not verified"
artifact_digest=$(table_field '# Snapshot' 'Artifact digest' 2)
artifact_digest_status=$(table_field '# Snapshot' 'Artifact digest' 3)
if [ "$artifact_class" = release ]; then
	[ "$artifact_digest" = "sha256:$(sha256_file "$bundle/$archive_name")" ] || fail "release report artifact digest does not identify the source archive"
	[ "$artifact_digest_status" = verified ] || fail "release report artifact-digest status is not verified"
else
	printf '%s\n' "$artifact_digest" | grep -Eq '^sha256:[0-9a-f]{64}$' || fail "fixture report artifact digest is malformed"
	[ "$artifact_digest_status" = verified ] || fail "fixture artifact-digest status is not verified"
fi
[ "$(table_field '# Snapshot' Worktree 2)" = clean ] || fail "review report release worktree is not clean"
[ "$(table_field '# Snapshot' Worktree 3)" = verified ] || fail "review report worktree status is not verified"
[ "$(table_field '# Snapshot' 'Generated state' 2)" = PASS ] || fail "review report generated state is not PASS"
[ "$(table_field '# Snapshot' 'Generated state' 3)" = verified ] || fail "review report generated-state status is not verified"
[ "$(table_field '# Snapshot' 'Dependency lock state' 2)" = PASS ] || fail "review report dependency lock state is not PASS"
[ "$(table_field '# Snapshot' 'Dependency lock state' 3)" = verified ] || fail "review report dependency-lock status is not verified"
[ -n "$(table_field '# Snapshot' 'Go version' 2)" ] || fail "review report Go version is empty"
[ "$(table_field '# Snapshot' 'Go version' 3)" = verified ] || fail "review report Go-version status is not verified"
[ -n "$(table_field '# Snapshot' 'OS / architecture' 2)" ] || fail "review report OS and architecture are empty"
[ "$(table_field '# Snapshot' 'OS / architecture' 3)" = verified ] || fail "review report OS/architecture status is not verified"
[ "$(table_field '# Snapshot' 'Active exceptions' 2)" = none ] || fail "review report snapshot names active exceptions"
[ "$(table_field '# Snapshot' 'Active exceptions' 3)" = verified ] || fail "review report active-exception status is not verified"
[ "$(section_field '# Gate 2 — Real-user and third-party-agent usability' 'Both sessions independent from implementation')" = yes ] || fail "Gate 2 session independence is not affirmed"
[ "$(section_field '# Gate 2 — Real-user and third-party-agent usability' 'Both sessions directly executed the named public surfaces')" = yes ] || fail "Gate 2 sessions did not affirm direct public-surface execution"
gate2_operator1=$(section_field '# Gate 2 — Real-user and third-party-agent usability' 'Cold session 1 operator')
gate2_session1=$(section_field '# Gate 2 — Real-user and third-party-agent usability' 'Cold session 1 session ID')
gate2_operator2=$(section_field '# Gate 2 — Real-user and third-party-agent usability' 'Cold session 2 operator')
gate2_session2=$(section_field '# Gate 2 — Real-user and third-party-agent usability' 'Cold session 2 session ID')
source_artifact_identity_is_canonical "$gate2_operator1" || fail "Gate 2 cold-session operator 1 is not a canonical identity token"
source_artifact_identity_is_canonical "$gate2_operator2" || fail "Gate 2 cold-session operator 2 is not a canonical identity token"
source_artifact_token_is_canonical "$gate2_session1" || fail "Gate 2 cold-session ID 1 is not a canonical token"
source_artifact_token_is_canonical "$gate2_session2" || fail "Gate 2 cold-session ID 2 is not a canonical token"
normalized_gate2_operator1=$(source_artifact_normalize_identity "$gate2_operator1")
normalized_gate2_operator2=$(source_artifact_normalize_identity "$gate2_operator2")
normalized_gate2_session1=$(printf '%s\n' "$gate2_session1" | tr '[:upper:]' '[:lower:]')
normalized_gate2_session2=$(printf '%s\n' "$gate2_session2" | tr '[:upper:]' '[:lower:]')
[ "$normalized_gate2_operator1" != "$normalized_builder" ] || fail "builder cannot run Gate 2 cold session 1"
[ "$normalized_gate2_operator2" != "$normalized_builder" ] || fail "builder cannot run Gate 2 cold session 2"
[ "$normalized_gate2_session1" != "$normalized_gate2_session2" ] || fail "Gate 2 cold-session IDs are not distinct"
for gate2_identity in "$normalized_gate2_operator1" "$normalized_gate2_operator2" "$normalized_gate2_session1" "$normalized_gate2_session2"; do
	case "$gate2_identity" in
	pending | todo | tbd | n/a | replace-* | list-*) fail "Gate 2 cold-session field is a placeholder" ;;
	esac
done
[ "$(section_field '# Snapshot' 'Scope explicitly reviewed')" = "$certified_scope" ] || fail "snapshot review scope disagrees with its certified scope"
if [ "$artifact_class" = release ]; then
	[ "$(section_field '# Snapshot' 'Scope explicitly not certified')" = none ] || fail "formal release report excludes project-profile scope"
fi
[ -n "$(section_field '# Gate 2 — Real-user and third-party-agent usability' 'Supported public surfaces used')" ] || fail "review report Gate 2 public surface is empty"
[ -n "$(section_field '# Gate 2 — Real-user and third-party-agent usability' Environment)" ] || fail "review report Gate 2 environment is empty"
[ "$(report_field verification-command)" = 'make verify' ] || fail "review report verification command is not make verify"
[ "$(report_field verification-result)" = PASS ] || fail "review report verification result is not PASS"
verification_environment=$(report_field verification-environment)
case "$(printf '%s' "$verification_environment" | tr '[:upper:]' '[:lower:]')" in
pending | todo | tbd | replace-* | list-*) fail "review report verification environment is a placeholder" ;;
esac
verification_artifact=$(report_field verification-artifact)
validate_evidence_reference verification-artifact "$verification_artifact"
ci_run=$(report_field ci-run)
case "$ci_run" in
https://*)
	printf '%s\n' "$ci_run" | grep -Eq '^https://[^[:space:]]+$' || fail "review report CI run is not an HTTPS URL"
	;;
*) fail "review report CI run is not an HTTPS URL" ;;
esac
validate_evidence_reference 'CI run' "$ci_run"
[ "$(table_field '# Snapshot' 'CI run' 2)" = "$ci_run" ] || fail "review report snapshot CI run mismatch"
[ "$(table_field '# Snapshot' 'CI run' 3)" = verified ] || fail "review report snapshot CI run is not verified"
gate2_session1_artifact=$(report_field gate-2-session-1-artifact)
gate2_session2_artifact=$(report_field gate-2-session-2-artifact)
validate_evidence_reference gate-2-session-1-artifact "$gate2_session1_artifact"
validate_evidence_reference gate-2-session-2-artifact "$gate2_session2_artifact"
if [ "$artifact_class" = release ]; then
	case "$gate2_session1_artifact:$gate2_session2_artifact" in
	sha256:*:sha256:*) ;;
	*) fail "formal Gate 2 cold-session evidence must use SHA-256 identities" ;;
	esac
fi
[ "$gate2_session1_artifact" != "$gate2_session2_artifact" ] || fail "Gate 2 cold-session evidence references are not distinct"
watched_red_artifact=$(report_field watched-red-artifact)
validate_evidence_reference watched-red-artifact "$watched_red_artifact"
require_gate2_scenario_artifact "$gate2_session1_artifact"
require_gate2_scenario_artifact "$gate2_session2_artifact"
[ "$(table_field '# Verification log' 'make verify' 3)" = PASS ] || fail "review report verification log does not pass make verify"
verification_log_artifact=$(table_field '# Verification log' 'make verify' 4)
validate_evidence_reference 'verification log artifact' "$verification_log_artifact"
[ "$verification_log_artifact" = "$verification_artifact" ] || fail "verification log artifact disagrees with the release envelope"
watched_red_row_artifact=$(require_watched_red_row)
validate_evidence_reference 'watched-red row artifact' "$watched_red_row_artifact"
[ "$watched_red_row_artifact" = "$watched_red_artifact" ] || fail "watched-red row artifact disagrees with the release envelope"
[ "$(report_field independent-certification)" = complete ] || fail "review report independent certification is incomplete"
[ "$(certification_field Reviewer)" = "$reviewer" ] || fail "independent certification reviewer mismatch"
[ "$(certification_field Snapshot)" = "$commit" ] || fail "independent certification snapshot mismatch"
[ "$(certification_field Date)" = "$(top_field Date)" ] || fail "independent certification date mismatch"
[ "$(certification_field 'I directly inspected or executed the evidence marked verified')" = yes ] || fail "independent certification does not affirm direct inspection"
[ "$(certification_field 'Scope certified')" = "$certified_scope" ] || fail "independent certification scope disagrees with the release envelope"
[ -n "$(certification_field 'Scope explicitly not certified')" ] || fail "independent certification exclusion scope is empty"
if [ "$artifact_class" = release ]; then
	[ "$(certification_field 'Scope explicitly not certified')" = none ] || fail "formal independent certification excludes project-profile scope"
fi
[ "$(certification_field 'Gate 1')" = PASS ] || fail "independent certification Gate 1 is not PASS"
[ "$(certification_field 'Gate 2')" = PASS ] || fail "independent certification Gate 2 is not PASS"
[ "$(certification_field 'Gate 3')" = PASS ] || fail "independent certification Gate 3 is not PASS"
[ "$(certification_field 'Final verdict')" = GO ] || fail "independent certification final verdict is not GO"
[ "$(section_status '# Verdict')" = GO ] || fail "human review verdict contradicts the release envelope"
[ "$(section_status '# Gate 1 — Architecture and open-source engineering quality')" = PASS ] || fail "human Gate 1 contradicts the release envelope"
[ "$(section_status '# Gate 2 — Real-user and third-party-agent usability')" = PASS ] || fail "human Gate 2 contradicts the release envelope"
[ "$(section_status '# Gate 3 — Test and evidence-system quality')" = PASS ] || fail "human Gate 3 contradicts the release envelope"

echo "source-artifact-check: verified $bundle"

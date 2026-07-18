#!/bin/sh

# Validate the tracked repository control plane. This check proves structure
# and internal consistency; it does not turn a pending human Gate verdict into
# PASS.
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
cd "$root"

fail() {
	echo "policy-check: $*" >&2
	exit 1
}

sha256_file() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | awk '{print $1}'
	elif command -v shasum >/dev/null 2>&1; then
		shasum -a 256 "$1" | awk '{print $1}'
	else
		fail "sha256sum or shasum is required"
	fi
}

required_files='ENGINEERING_STANDARD.md
ENGINEERING_STANDARD.sha256
PROJECT_PROFILE.md
README.md
LICENSE
CONTRIBUTING.md
SECURITY.md
AGENTS.md
CLAUDE.md
.github/CODEOWNERS
.github/PULL_REQUEST_TEMPLATE.md
.github/check-pr-policy.mjs
docs/design.md
docs/decisions.md
docs/merge-policy.md
docs/release.md
docs/security/threat-model.md
docs/privacy/data-inventory.md
docs/reviews/REVIEW_REPORT.template.md
docs/exceptions/QUALITY_EXCEPTION.template.md
tools/build-source-artifact.sh
tools/check-source-artifact.sh
tools/source-artifact-bootstrap.sh
tools/source-artifact-lib.sh
tools/source-artifact-toolchain.txt
tools/test-source-artifact.sh
tools/testdata/source-artifact-approved-profile.md
tools/testdata/source-artifact-review.md.in'

printf '%s\n' "$required_files" | while IFS= read -r file; do
	[ -s "$file" ] || fail "required control-plane file is missing or empty: $file"
	if git check-ignore -q "$file"; then
		fail "required control-plane file is ignored: $file"
	fi
done

standard_count=$(find . \
	-path './.git' -prune -o \
	-path './node_modules' -prune -o \
	-path './.github/node_modules' -prune -o \
	-type f -name 'ENGINEERING_STANDARD*.md' -print | wc -l | tr -d ' ')
[ "$standard_count" = 1 ] || fail "found $standard_count normative-standard candidates, want exactly one"

expected_standard_sha=$(awk 'NF == 2 && $2 == "ENGINEERING_STANDARD.md" {print $1}' ENGINEERING_STANDARD.sha256)
[ ${#expected_standard_sha} -eq 64 ] || fail "ENGINEERING_STANDARD.sha256 has no single SHA-256 identity"
actual_standard_sha=$(sha256_file ENGINEERING_STANDARD.md)
[ "$actual_standard_sha" = "$expected_standard_sha" ] || fail "ENGINEERING_STANDARD.md changed without updating its normative digest"

grep -q '^# Repository Engineering, Acceptance, and Evidence Standard$' ENGINEERING_STANDARD.md ||
	fail "normative standard title changed"
grep -q '^Version: 2\.0  *$' ENGINEERING_STANDARD.md || fail "normative standard version is not 2.0"

for section in $(seq 1 20); do
	grep -q "^## ${section}\." PROJECT_PROFILE.md || fail "PROJECT_PROFILE.md is missing section ${section}"
done
if grep -En 'YYYY-MM-DD|@owner|@reviewer-or-team|_{6,}|replace-with-' PROJECT_PROFILE.md >/dev/null; then
	fail "PROJECT_PROFILE.md still contains template placeholders"
fi
grep -Eq '^Base class: .*R3.*$' PROJECT_PROFILE.md || fail "PROJECT_PROFILE.md must resolve the current private-data and irreversible-action risk as R3"
if ! grep -q 'The canonical complete verification command is:' PROJECT_PROFILE.md ||
	! grep -q 'make verify' PROJECT_PROFILE.md; then
	fail "PROJECT_PROFILE.md does not name make verify as the canonical command"
fi

profile_field() {
	awk -v key="$1" '
		index($0, key ": ") == 1 { count++; value = substr($0, length(key) + 3) }
		END { if (count != 1 || value == "") exit 1; print value }
	' PROJECT_PROFILE.md || fail "profile readiness field is missing, empty, or duplicated: $1"
}
profile_status=$(profile_field profile-status)
merge_readiness=$(profile_field merge-readiness)
artifact_build_readiness=$(profile_field artifact-build-readiness)
artifact_build_blockers=$(profile_field artifact-build-blockers)
post_artifact_blockers=$(profile_field post-artifact-blockers)
release_readiness=$(profile_field release-readiness)
production_readiness=$(profile_field production-readiness)
open_blockers=$(profile_field open-blockers)
active_exceptions=$(profile_field active-exceptions)
case "$profile_status" in
PROPOSED)
	grep -q '^Status: Proposed normative profile; initial approval is pending  *$' PROJECT_PROFILE.md ||
		fail "proposed profile envelope contradicts the human profile status"
	;;
APPROVED)
	grep -q '^Status: Approved normative profile  *$' PROJECT_PROFILE.md ||
		fail "approved profile envelope contradicts the human profile status"
	;;
*) fail "unknown project-profile status: $profile_status" ;;
esac
case "$merge_readiness" in GO | NO-GO) ;; *) fail "unknown merge-readiness value: $merge_readiness" ;; esac
case "$artifact_build_readiness" in GO | NO-GO) ;; *) fail "unknown artifact-build-readiness value: $artifact_build_readiness" ;; esac
case "$release_readiness" in GO | NO-GO | PENDING-ARTIFACT) ;; *) fail "unknown release-readiness value: $release_readiness" ;; esac
case "$production_readiness" in GO | NO-GO | N/A) ;; *) fail "unknown production-readiness value: $production_readiness" ;; esac
validate_blocker_set() {
	vbs_value=$1
	vbs_label=$2
	[ "$vbs_value" = none ] && return 0
	printf '%s\n' "$vbs_value" | grep -Eq '^[A-Z][A-Z0-9]*(-[A-Z0-9]+)*(,[A-Z][A-Z0-9]*(-[A-Z0-9]+)*)*$' ||
		fail "$vbs_label is not a canonical comma-separated blocker set"
	printf '%s\n' "$vbs_value" | tr ',' '\n' | awk '!seen[$0]++ { next } { exit 1 }' ||
		fail "$vbs_label contains a duplicate blocker ID"
}
validate_blocker_set "$artifact_build_blockers" artifact-build-blockers
validate_blocker_set "$post_artifact_blockers" post-artifact-blockers
validate_blocker_set "$open_blockers" open-blockers

listed_blockers=$(awk -F '|' '
	$0 == "Current blockers:" { inside = 1; next }
	inside && /^## / { inside = 0 }
	inside && /^\|/ {
		id = $2
		gsub(/^[[:space:]]+|[[:space:]]+$/, "", id)
		if (id == "ID" || id ~ /^-+$/) next
		rows++
		if (id == "None") { none_rows++; next }
		if (id !~ /^[A-Z][A-Z0-9]*(-[A-Z0-9]+)*$/ || seen[id]++) bad = 1
		if (value != "") value = value ","
		value = value id
	}
	END {
		if (rows < 1 || bad || (none_rows && (rows != 1 || value != ""))) exit 1
		if (value == "") value = "none"
		print value
	}
' PROJECT_PROFILE.md) || fail "Current blockers table is malformed or contains duplicate/non-canonical IDs"
[ "$open_blockers" = "$listed_blockers" ] || fail "profile readiness blockers disagree with the Current blockers table"

profile_approval_field() {
	paf_key=$1
	awk -v key="$paf_key" '
		$0 == "## 19. Profile approval" { headings++; inside = 1; next }
		inside && $0 == "## 20. Machine-readable readiness envelope" { inside = 0 }
		inside && index($0, key ": ") == 1 {
			count++
			value = substr($0, length(key) + 3)
		}
		END { if (headings != 1 || count != 1 || value == "") exit 1; print value }
	' PROJECT_PROFILE.md || fail "profile approval field is missing, empty, or duplicated: $paf_key"
}
approval_starts_with() {
	asw_value=$1
	asw_prefix=$2
	asw_label=$3
	case "$asw_value" in
	"$asw_prefix" | "$asw_prefix —"*) ;;
	*) fail "$asw_label does not record $asw_prefix" ;;
	esac
}
approval_binding=$(profile_approval_field 'Approval binding')
if [ "$profile_status" = APPROVED ]; then
	case "$(profile_approval_field 'Profile version')" in *draft* | *DRAFT*) fail "approved profile still has a draft version" ;; esac
	[ "$approval_binding" = EXTERNAL-RELEASE-REPORT ] ||
		fail "approved profile is not bound through the candidate release report"
	approval_starts_with "$(profile_approval_field 'Architecture approval')" APPROVED 'architecture approval'
	security_approval=$(profile_approval_field 'Security / privacy approval where applicable')
	case "$security_approval" in APPROVED | 'APPROVED —'* | N/A | 'N/A —'*) ;; *) fail "security/privacy approval is neither APPROVED nor justified N/A" ;; esac
	operations_approval=$(profile_approval_field 'Operations approval where applicable')
	case "$operations_approval" in APPROVED | 'APPROVED —'* | N/A | 'N/A —'*) ;; *) fail "operations approval is neither APPROVED nor justified N/A" ;; esac
	approval_starts_with "$(profile_approval_field 'Independent approval')" APPROVED 'independent approval'
else
	case "$approval_binding" in UNVERIFIED* | PENDING*) ;; *) fail "proposed profile approval binding is not pending or unverified" ;; esac
fi
if [ "$artifact_build_readiness" = GO ]; then
	[ "$profile_status" = APPROVED ] || fail "artifact-build GO requires an approved profile"
	[ "$merge_readiness" = GO ] || fail "artifact-build GO requires merge GO"
	[ "$artifact_build_blockers" = none ] || fail "artifact-build GO cannot name artifact-build blockers"
	[ "$active_exceptions" = none ] || fail "artifact-build GO cannot name active exceptions"
fi
if [ "$release_readiness" = PENDING-ARTIFACT ]; then
	[ "$artifact_build_readiness" = GO ] || fail "PENDING-ARTIFACT requires artifact-build GO"
	[ "$post_artifact_blockers" != none ] || fail "PENDING-ARTIFACT must name the blockers the artifact evidence closes"
	[ "$open_blockers" = "$post_artifact_blockers" ] || fail "PENDING-ARTIFACT cannot retain non-artifact blockers"
fi
if [ "$release_readiness" = GO ]; then
	[ "$profile_status" = APPROVED ] || fail "release GO requires an approved profile"
	[ "$merge_readiness" = GO ] || fail "release GO requires merge GO"
	[ "$artifact_build_readiness" = GO ] || fail "release GO requires artifact-build GO"
	[ "$open_blockers" = none ] || fail "release GO cannot name open profile blockers"
	[ "$active_exceptions" = none ] || fail "release GO cannot name active exceptions"
fi

if ! grep -q "repository-specific working protocol" docs/standards.md ||
	! grep -q 'normative engineering source is .*ENGINEERING_STANDARD.md.* version 2.0' docs/standards.md; then
	fail "docs/standards.md does not subordinate itself to the normative standard/profile"
fi
grep -q 'Normative engineering and evidence bar' docs/program.md ||
	fail "docs/program.md omits the normative standard/profile authority"
grep -q '^source-artifact:' Makefile || fail "Makefile has no tagged source-artifact command"
grep -q '^source-archive-candidate:' Makefile || fail "Makefile has no pre-review source-archive command"
grep -q '^source-artifact-check:' Makefile || fail "Makefile has no reproducibility/provenance check"
verify_line=$(sed -n '/^verify:/p' Makefile)
for stage in policy-check vuln browser-check mutation-check portable-build-check license-check source-artifact-check; do
	printf '%s\n' "$verify_line" | grep -qw "$stage" || fail "make verify omits mandatory stage: $stage"
done
[ "$(grep -c -- '-fuzztime=10000x -parallel=1' Makefile)" -eq 1 ] ||
	fail "fuzz smoke must give every discovered target exactly 10,000 executions with one worker"
grep -q 'source-artifact-bootstrap.sh' Makefile || fail "release targets do not load the committed bootstrap"
grep -q -- '--assemble' Makefile || fail "release artifact has no final assembly phase"
grep -q -- '--prepare-archive' Makefile || fail "release artifact has no non-circular pre-review archive phase"
grep -q 'REVIEW_EVIDENCE' Makefile || fail "release artifact is not bound to independent verification evidence"
grep -q 'sh tools/test-source-artifact.sh' Makefile || fail "source-artifact contract tests are not in make verify"
[ "$(wc -l <tools/source-artifact-toolchain.txt | tr -d ' ')" -eq 3 ] || fail "source artifact toolchain must contain exactly three fields"
grep -q '^format: yomihon-source-artifact-toolchain-v1$' tools/source-artifact-toolchain.txt || fail "source artifact toolchain format is missing"
grep -q '^git-version: .\+$' tools/source-artifact-toolchain.txt || fail "source artifact Git version is not pinned"
grep -q '^gzip-version: .\+$' tools/source-artifact-toolchain.txt || fail "source artifact gzip version is not pinned"
grep -q 'source-artifact-toolchain-sha256' tools/build-source-artifact.sh || fail "source artifact provenance omits the pinned toolchain"
grep -q 'source-artifact-bootstrap-sha256' tools/build-source-artifact.sh || fail "source artifact provenance omits its committed bootstrap"
grep -q '^Status: .*GO / ACCEPT-WITH-GATES / NO-GO.*$' docs/reviews/REVIEW_REPORT.template.md ||
	fail "review report has no machine-checkable human verdict"
has_normative_applicability_states() {
	hnas_profile=$1
	hnas_review=$2
	grep -q '^| Stage | APPLIES / N/A / DEFERRED-BY-EXCEPTION / UNRESOLVED |' "$hnas_profile" &&
		grep -q '^| | UNRESOLVED / DEFERRED-BY-EXCEPTION / UNVERIFIED / BLOCKED |' "$hnas_review"
}
has_normative_applicability_states PROJECT_PROFILE.md docs/reviews/REVIEW_REPORT.template.md ||
	fail "profile and review template do not use the normative applicability states"
policy_state_tmp=$(mktemp -d "${TMPDIR:-/tmp}/yomihon-policy-state.XXXXXX")
trap 'rm -rf "$policy_state_tmp"' 0 HUP INT TERM
sed 's/DEFERRED-BY-EXCEPTION/DEFERRED/' PROJECT_PROFILE.md >"$policy_state_tmp/profile.md"
cp docs/reviews/REVIEW_REPORT.template.md "$policy_state_tmp/review.md"
if has_normative_applicability_states "$policy_state_tmp/profile.md" "$policy_state_tmp/review.md"; then
	fail "applicability-state self-test accepted the DEFERRED alias"
fi
cp PROJECT_PROFILE.md "$policy_state_tmp/profile.md"
sed 's/DEFERRED-BY-EXCEPTION/EXCEPTION/' docs/reviews/REVIEW_REPORT.template.md >"$policy_state_tmp/review.md"
if has_normative_applicability_states "$policy_state_tmp/profile.md" "$policy_state_tmp/review.md"; then
	fail "applicability-state self-test accepted the EXCEPTION alias"
fi
grep -q '^# Release evidence envelope$' docs/reviews/REVIEW_REPORT.template.md ||
	fail "review report omits the release certificate envelope"
grep -q '^## Source artifact and provenance$' docs/release.md ||
	fail "release policy omits source artifact/provenance"
grep -q '^## Readiness boundary$' docs/release.md ||
	fail "release policy does not distinguish readiness claims"

grep -q 'Gate 1 — Architecture and open-source engineering quality' .github/PULL_REQUEST_TEMPLATE.md ||
	fail "pull-request template omits Gate 1"
grep -q 'Gate 2 — Real-user and third-party-agent usability' .github/PULL_REQUEST_TEMPLATE.md ||
	fail "pull-request template omits Gate 2"
grep -q 'Gate 3 — Test and evidence-system quality' .github/PULL_REQUEST_TEMPLATE.md ||
	fail "pull-request template omits Gate 3"
grep -q 'Reviewed commit:' .github/PULL_REQUEST_TEMPLATE.md || fail "pull-request template omits immutable commit identity"
grep -q '^## Target protection for ' docs/merge-policy.md || fail "branch-protection target is undocumented"
grep -q '^## Readiness claims$' docs/merge-policy.md || fail "merge, release, and production readiness are not distinguished"
grep -q 'Gate 2 — Real-user and third-party-agent usability' docs/reviews/REVIEW_REPORT.template.md ||
	fail "review template omits independent user acceptance"
grep -q '^## Ownership and closure$' docs/exceptions/QUALITY_EXCEPTION.template.md ||
	fail "exception template omits ownership and closure"

node .github/check-pr-policy.mjs --self-test

git ls-files | while IFS= read -r file; do
	case "$file" in
	.env | .env.* | *.pem | *.key | *.p12 | *.pfx | *.sqlite | *.sqlite3 | *.db | *-wal | *-shm)
		fail "forbidden secret or derived-store path is tracked: $file"
		;;
	esac
done

echo "policy-check: repository control plane is internally consistent"

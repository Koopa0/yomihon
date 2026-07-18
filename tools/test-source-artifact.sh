#!/bin/sh

# Adversarial contract tests for the source-only release builder and checker.
# Every mutation is expected to fail for its own named reason class.
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
cd "$root"

tmp=$(mktemp -d "${TMPDIR:-/tmp}/yomihon-source-artifact-test.XXXXXX")
cleanup() {
	rm -rf "$tmp"
}
trap cleanup 0 HUP INT TERM

fail() {
	echo "source-artifact-test: $*" >&2
	exit 1
}

expect_failure() {
	ef_name=$1
	ef_want_status=$2
	ef_marker=$3
	shift 3
	ef_status=0
	"$@" >"$tmp/failure.log" 2>&1 || ef_status=$?
	[ "$ef_status" -eq "$ef_want_status" ] || {
		cat "$tmp/failure.log" >&2
		fail "$ef_name exited $ef_status, want $ef_want_status"
	}
	grep -Fq "$ef_marker" "$tmp/failure.log" || {
		cat "$tmp/failure.log" >&2
		fail "$ef_name did not emit its contract marker: $ef_marker"
	}
}

expect_success() {
	es_name=$1
	shift
	es_status=0
	"$@" >"$tmp/success.log" 2>&1 || es_status=$?
	if [ "$es_status" -ne 0 ]; then
		cat "$tmp/success.log" >&2
		fail "$es_name exited $es_status"
	fi
}

write_evidence() {
	we_file=$1
	we_reviewed=$2
	we_gate2=${3:-PASS}
	we_repository=${4:-.}
	we_profile=$(mktemp "$tmp/profile-for-evidence.XXXXXX")
	git -C "$we_repository" show "$we_reviewed:PROJECT_PROFILE.md" >"$we_profile" || {
		rm -f "$we_profile"
		fail 'could not read PROJECT_PROFILE.md for review evidence'
	}
	we_profile_sha256=$(sha256_file "$we_profile")
	rm -f "$we_profile"
	sed \
		-e "s/COMMIT/$we_reviewed/g" \
		-e "s/PROFILE_SHA256/$we_profile_sha256/g" \
		-e "s/GATE2/$we_gate2/g" \
		tools/testdata/source-artifact-review.md.in >"$we_file"
}

write_release_evidence() {
	wre_file=$1
	wre_reviewed=$2
	wre_archive_digest=${3:-1111111111111111111111111111111111111111111111111111111111111111}
	wre_repository=${4:-.}
	write_evidence "$wre_file.fixture" "$wre_reviewed" PASS "$wre_repository"
	sed \
		-e 's/^Review class: verification-fixture  $/Review class: release-candidate  /' \
		-e 's/^One-sentence rationale: The synthetic release fixture satisfies every structural evidence contract\.$/One-sentence rationale: The complete fixture-repository profile satisfies every structural evidence contract./' \
		-e "s/^| Artifact digest | sha256:1111111111111111111111111111111111111111111111111111111111111111 | verified |$/| Artifact digest | sha256:$wre_archive_digest | verified |/" \
		-e "s/^| None | N\/A | N\/A | reviewer-session |$/| ARTIFACT-EVIDENCE | CLOSED | sha256:$wre_archive_digest | reviewer-session |/" \
		-e 's/^Scope explicitly reviewed: source-artifact-contract  $/Scope explicitly reviewed: complete-project-profile  /' \
		-e 's/^Scope explicitly not certified: Product behavior outside the synthetic artifact fixture\.$/Scope explicitly not certified: none/' \
		-e 's/^Supported public surfaces used: Source artifact CLI\.  $/Supported public surfaces used: Complete fixture-repository profile surface.  /' \
		-e 's/^Scope certified: source-artifact-contract$/Scope certified: complete-project-profile/' \
		-e 's/^Scope explicitly not certified: Product behavior\.$/Scope explicitly not certified: none/' \
		-e 's/^review-class: verification-fixture$/review-class: release-candidate/' \
		-e 's/^certified-scope: source-artifact-contract$/certified-scope: complete-project-profile/' \
		-e 's/^closed-profile-blockers: none$/closed-profile-blockers: ARTIFACT-EVIDENCE/' \
		-e 's#https://example\.invalid/yomihon/reviews/gate-2-session-1#sha256:2222222222222222222222222222222222222222222222222222222222222222#g' \
		-e 's#https://example\.invalid/yomihon/reviews/gate-2-session-2#sha256:3333333333333333333333333333333333333333333333333333333333333333#g' \
		-e 's#https://example\.invalid/yomihon#https://github.com/koopa0/yomihon#g' \
		"$wre_file.fixture" >"$wre_file"
	rm "$wre_file.fixture"
}

run_committed_bootstrap() {
	rcb_repository=$1
	rcb_commit=$2
	shift 2
	rcb_script=$(mktemp "$tmp/committed-bootstrap.XXXXXX")
	GIT_NO_REPLACE_OBJECTS=1 git -C "$rcb_repository" show "$rcb_commit:tools/source-artifact-bootstrap.sh" >"$rcb_script" || {
		rm -f "$rcb_script"
		fail 'could not extract committed source-artifact bootstrap'
	}
	(
		cd "$rcb_repository"
		sh "$rcb_script" "$@"
	)
	rcb_status=$?
	rm -f "$rcb_script"
	return "$rcb_status"
}

sha256_file() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | awk '{print $1}'
	else
		shasum -a 256 "$1" | awk '{print $1}'
	fi
}

replace_manifest_digest() {
	rmd_manifest=$1
	rmd_name=$2
	rmd_digest=$3
	awk -v name="$rmd_name" -v digest="$rmd_digest" '
		$2 == name { $1 = digest }
		{ print $1 "  " $2 }
	' "$rmd_manifest" >"$rmd_manifest.new"
	mv "$rmd_manifest.new" "$rmd_manifest"
}

repair_fixture_report_chain() {
	rfrc_bundle=$1
	rfrc_report="$rfrc_bundle/yomihon-v0.0.0-check-review.md"
	rfrc_certificate="$rfrc_bundle/yomihon-v0.0.0-check-verification.md"
	rfrc_provenance="$rfrc_bundle/yomihon-v0.0.0-check.provenance"
	rfrc_manifest="$rfrc_bundle/yomihon-v0.0.0-check-SHA256SUMS"
	rfrc_report_digest=$(sha256_file "$rfrc_report")
	sed "s/^review-report-sha256:.*/review-report-sha256: $rfrc_report_digest/" "$rfrc_certificate" >"$rfrc_certificate.new"
	mv "$rfrc_certificate.new" "$rfrc_certificate"
	rfrc_certificate_digest=$(sha256_file "$rfrc_certificate")
	sed -e "s/^review-report-sha256:.*/review-report-sha256: $rfrc_report_digest/" -e "s/^verification-evidence-sha256:.*/verification-evidence-sha256: $rfrc_certificate_digest/" "$rfrc_provenance" >"$rfrc_provenance.new"
	mv "$rfrc_provenance.new" "$rfrc_provenance"
	replace_manifest_digest "$rfrc_manifest" yomihon-v0.0.0-check-review.md "$rfrc_report_digest"
	replace_manifest_digest "$rfrc_manifest" yomihon-v0.0.0-check-verification.md "$rfrc_certificate_digest"
	replace_manifest_digest "$rfrc_manifest" yomihon-v0.0.0-check.provenance "$(sha256_file "$rfrc_provenance")"
}

replace_exact_line() {
	rel_file=$1
	rel_old=$2
	rel_new=$3
	rel_count=$(grep -cxF "$rel_old" "$rel_file" || true)
	[ "$rel_count" -eq 1 ] || fail "mutation needle occurred $rel_count times, want exactly 1: $rel_old"
	awk -v old="$rel_old" -v new="$rel_new" '$0 == old { print new; next } { print }' "$rel_file" >"$rel_file.new"
	mv "$rel_file.new" "$rel_file"
	grep -qxF "$rel_new" "$rel_file" || fail "mutation did not produce its replacement: $rel_new"
}

expect_bundle_line_failure() {
	eblf_name=$1
	eblf_slug=$2
	eblf_relative=$3
	eblf_old=$4
	eblf_new=$5
	eblf_marker=$6
	eblf_bundle="$tmp/$eblf_slug"
	cp -R "$pristine" "$eblf_bundle"
	replace_exact_line "$eblf_bundle/$eblf_relative" "$eblf_old" "$eblf_new"
	repair_fixture_report_chain "$eblf_bundle"
	expect_failure "$eblf_name" 1 "$eblf_marker" sh tools/check-source-artifact.sh --allow-fixture v0.0.0-check "$commit" "$eblf_bundle"
}

expect_missing_canonical_row() {
	emcr_name=$1
	emcr_slug=$2
	emcr_row=$3
	emcr_marker=$4
	emcr_bundle="$tmp/$emcr_slug"
	cp -R "$pristine" "$emcr_bundle"
	emcr_report="$emcr_bundle/yomihon-v0.0.0-check-review.md"
	awk -F '|' -v row="$emcr_row" '
		/^\|/ {
			label = $2
			gsub(/^[[:space:]]+|[[:space:]]+$/, "", label)
			if (label == row) next
		}
		{ print }
	' "$emcr_report" >"$emcr_report.new"
	mv "$emcr_report.new" "$emcr_report"
	repair_fixture_report_chain "$emcr_bundle"
	expect_failure "$emcr_name" 1 "$emcr_marker" sh tools/check-source-artifact.sh --allow-fixture v0.0.0-check "$commit" "$emcr_bundle"
}

expect_profile_preflight_failure() {
	eppf_name=$1
	eppf_slug=$2
	eppf_old=$3
	eppf_new=$4
	eppf_marker=$5
	eppf_repository="$tmp/profile-$eppf_slug-repository"
	eppf_version="v0.0.0-$eppf_slug"
	eppf_evidence="$tmp/profile-$eppf_slug-review.md"
	git clone -q "$fixture" "$eppf_repository"
	git -C "$eppf_repository" config user.name artifact-test
	git -C "$eppf_repository" config user.email artifact-test@example.invalid
	sed "s|^$eppf_old$|$eppf_new|" "$eppf_repository/PROJECT_PROFILE.md" >"$eppf_repository/PROJECT_PROFILE.md.new"
	mv "$eppf_repository/PROJECT_PROFILE.md.new" "$eppf_repository/PROJECT_PROFILE.md"
	git -C "$eppf_repository" add PROJECT_PROFILE.md
	git -C "$eppf_repository" commit -qm "invalidate $eppf_slug profile preflight"
	eppf_commit=$(git -C "$eppf_repository" rev-parse HEAD)
	git -C "$eppf_repository" tag -a "$eppf_version" -m "$eppf_version" "$eppf_commit"
	write_release_evidence "$eppf_evidence" "$eppf_commit" 1111111111111111111111111111111111111111111111111111111111111111 "$eppf_repository"
	expect_failure "$eppf_name" 1 "$eppf_marker" \
		run_committed_bootstrap "$eppf_repository" "$eppf_commit" --assemble "$eppf_version" "$eppf_commit" "$eppf_evidence" "$tmp/profile-$eppf_slug-output"
}

expect_provenance_digest_failure() {
	epdf_field=$1
	epdf_slug=$2
	epdf_marker=$3
	epdf_bundle="$tmp/provenance-$epdf_slug"
	cp -R "$pristine" "$epdf_bundle"
	epdf_provenance="$epdf_bundle/yomihon-v0.0.0-check.provenance"
	epdf_manifest="$epdf_bundle/yomihon-v0.0.0-check-SHA256SUMS"
	sed "s/^$epdf_field:.*/$epdf_field: 0000000000000000000000000000000000000000000000000000000000000000/" "$epdf_provenance" >"$epdf_provenance.new"
	mv "$epdf_provenance.new" "$epdf_provenance"
	replace_manifest_digest "$epdf_manifest" yomihon-v0.0.0-check.provenance "$(sha256_file "$epdf_provenance")"
	expect_failure "provenance $epdf_field binding" 1 "$epdf_marker" sh tools/check-source-artifact.sh --allow-fixture v0.0.0-check "$commit" "$epdf_bundle"
}

commit=$(git rev-parse HEAD)
evidence="$tmp/review.md"
write_evidence "$evidence" "$commit"
profile_sha256=$(sed -n 's/^project-profile-sha256: //p' "$evidence")
[ ${#profile_sha256} -eq 64 ] || fail 'generated review evidence has no project-profile digest'

for invalid in v1 v01.2.3 v1.02.3 v1.2.03 v1.2.3-01 v1.2.3-alpha..1 v1.2.3- v1.2.3+; do
	expect_failure "invalid semantic version $invalid" 2 'source-artifact: invalid semantic version:' \
		sh tools/build-source-artifact.sh "$invalid" "$commit" "$evidence" "$tmp/invalid"
done

GZIP=-9 GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=tar.umask GIT_CONFIG_VALUE_0=0077 \
	sh tools/build-source-artifact.sh v0.0.0-check "$commit" "$evidence" "$tmp/first"
sh tools/build-source-artifact.sh v0.0.0-check "$commit" "$evidence" "$tmp/second"
diff -ru "$tmp/first" "$tmp/second"
sh tools/build-source-artifact.sh v1.2.3-alpha.1+build.7 "$commit" "$evidence" "$tmp/valid-semver"

pristine="$tmp/second/yomihon-v0.0.0-check"
expect_failure 'fixture checker requires explicit opt-in' 1 'source-artifact-check: verification fixture requires --allow-fixture' sh tools/check-source-artifact.sh v0.0.0-check "$commit" "$pristine"
sh tools/check-source-artifact.sh --allow-fixture v0.0.0-check "$commit" "$pristine"
expect_failure overwrite 1 'source-artifact: refusing to overwrite' sh tools/build-source-artifact.sh v0.0.0-check "$commit" "$evidence" "$tmp/second"

mkdir "$tmp/locked-output"
mkdir "$tmp/locked-output/.yomihon-v0.0.0-check.lock"
expect_failure 'publication lock' 1 'source-artifact: publication lock is already held' sh tools/build-source-artifact.sh v0.0.0-check "$commit" "$evidence" "$tmp/locked-output"

mkdir "$tmp/race-bin"
cat >"$tmp/race-bin/mv" <<'EOF'
#!/bin/sh
set -eu
[ "$#" -eq 2 ]
mkdir "$2"
exec "$REAL_MV" "$@"
EOF
chmod 0755 "$tmp/race-bin/mv"
real_mv=$(command -v mv)
expect_failure 'destination appearance during publication' 1 'source-artifact: destination changed during publication' env REAL_MV="$real_mv" PATH="$tmp/race-bin:$PATH" sh tools/build-source-artifact.sh v0.0.0-race "$commit" "$evidence" "$tmp/race-output"
race_destination="$tmp/race-output/yomihon-v0.0.0-race"
[ -d "$race_destination" ] || fail 'publication-race fixture did not create the competing destination'
[ -z "$(find "$race_destination" -mindepth 1 -print -quit)" ] || fail 'publication-race cleanup left staged payload inside the competing destination'
[ ! -e "$tmp/race-output/.yomihon-v0.0.0-race.lock" ] || fail 'publication-race cleanup left the cooperative lock'

mkdir "$tmp/postcheck-bin"
cat >"$tmp/postcheck-bin/sh" <<'EOF'
#!/bin/sh
set -eu
last=
for argument do last=$argument; done
case "$last" in
*/yomihon-v0.0.0-postcheck)
	printf '%s\n' 'post-rename mutation' >>"$last/yomihon-v0.0.0-postcheck-review.md"
	: >"$POSTCHECK_MARKER"
	;;
esac
exec "$REAL_SH" "$@"
EOF
chmod 0755 "$tmp/postcheck-bin/sh"
postcheck_marker="$tmp/postcheck.marker"
expect_failure 'post-rename validation' 1 'source-artifact: published bundle failed validation and was quarantined:' env REAL_SH="$(command -v sh)" POSTCHECK_MARKER="$postcheck_marker" PATH="$tmp/postcheck-bin:$PATH" sh tools/build-source-artifact.sh v0.0.0-postcheck "$commit" "$evidence" "$tmp/postcheck-output"
[ -e "$postcheck_marker" ] || fail 'post-rename validation mutation did not execute'
[ ! -e "$tmp/postcheck-output/yomihon-v0.0.0-postcheck" ] || fail 'failed post-rename validation left the canonical destination occupied'
[ "$(find "$tmp/postcheck-output" -maxdepth 1 -type d -name '.yomihon-v0.0.0-postcheck.failed.*' | wc -l | tr -d ' ')" -eq 1 ] || fail 'failed post-rename validation did not retain exactly one quarantine'

mkdir "$tmp/outside"
ln -s "$tmp/outside" "$tmp/linked-output"
expect_failure 'linked output parent' 1 'source-artifact: output parent is not a real directory' sh tools/build-source-artifact.sh v0.0.0-check "$commit" "$evidence" "$tmp/linked-output"

mkdir "$tmp/dangling-parent"
ln -s "$tmp/nowhere" "$tmp/dangling-parent/yomihon-v0.0.0-check"
expect_failure 'dangling destination link' 1 'source-artifact: refusing to overwrite' sh tools/build-source-artifact.sh v0.0.0-check "$commit" "$evidence" "$tmp/dangling-parent"

ln -s "$evidence" "$tmp/linked-evidence.md"
expect_failure 'linked verification evidence' 1 'source-artifact: verification record is missing, linked, or not a regular file' sh tools/build-source-artifact.sh v0.0.0-check "$commit" "$tmp/linked-evidence.md" "$tmp/linked-evidence-output"

unsafe_text_evidence="$tmp/unsafe-text-evidence.md"
cp "$evidence" "$unsafe_text_evidence"
printf '\000' >>"$unsafe_text_evidence"
expect_failure 'binary verification evidence' 1 'source-artifact: verification record is not safe UTF-8 text' sh tools/build-source-artifact.sh v0.0.0-check "$commit" "$unsafe_text_evidence" "$tmp/unsafe-text-evidence-output"

line_separator_evidence="$tmp/line-separator-evidence.md"
cp "$evidence" "$line_separator_evidence"
printf '\342\200\250' >>"$line_separator_evidence"
expect_failure 'raw Unicode line separator in verification evidence' 1 'source-artifact: verification record is not safe UTF-8 text' sh tools/build-source-artifact.sh v0.0.0-check "$commit" "$line_separator_evidence" "$tmp/line-separator-evidence-output"

invalid_utf8_evidence="$tmp/invalid-utf8-evidence.md"
cp "$evidence" "$invalid_utf8_evidence"
printf '\377' >>"$invalid_utf8_evidence"
expect_failure 'invalid UTF-8 verification evidence' 1 'source-artifact: verification record is not safe UTF-8 text' sh tools/build-source-artifact.sh v0.0.0-check "$commit" "$invalid_utf8_evidence" "$tmp/invalid-utf8-evidence-output"

mkdir "$tmp/broken-text-tools"
cat >"$tmp/broken-text-tools/od" <<'EOF'
#!/bin/sh
exit 1
EOF
chmod 0755 "$tmp/broken-text-tools/od"
expect_failure 'failed byte validator' 1 'source-artifact: working iconv and od are required to validate review evidence' env PATH="$tmp/broken-text-tools:$PATH" sh tools/build-source-artifact.sh v0.0.0-check "$commit" "$evidence" "$tmp/broken-text-tool-output"

bad_evidence="$tmp/bad-evidence.md"
write_evidence "$bad_evidence" "$commit" PARTIAL
expect_failure 'non-PASS verification evidence' 1 'source-artifact: review evidence does not pass gate-2' sh tools/build-source-artifact.sh v0.0.0-check "$commit" "$bad_evidence" "$tmp/bad-evidence-output"
[ ! -e "$tmp/bad-evidence-output/yomihon-v0.0.0-check" ] || fail 'failed validation published a destination'
if [ -d "$tmp/bad-evidence-output" ] && [ -n "$(find "$tmp/bad-evidence-output" -name '.yomihon-v0.0.0-check.*' -print)" ]; then
	fail 'failed validation left staging data'
fi

hollow_evidence="$tmp/hollow-evidence.md"
cp docs/reviews/REVIEW_REPORT.template.md "$hollow_evidence"
sed \
	-e "s/^Status: \`GO \/ ACCEPT-WITH-GATES \/ NO-GO\`$/Status: GO/" \
	-e "s/^Status: \`PASS \/ PARTIAL \/ FAIL\`$/Status: PASS/" \
		-e "s/^reviewed-commit: replace-with-40-character-commit$/reviewed-commit: $commit/" \
		-e "s/^project-profile-sha256: replace-with-64-lowercase-hex$/project-profile-sha256: $profile_sha256/" \
	-e 's/^review-class: verification-fixture-or-release-candidate$/review-class: verification-fixture/' \
	-e 's/^certified-scope: source-artifact-contract-or-complete-project-profile$/certified-scope: source-artifact-contract/' \
	-e 's/^closed-profile-blockers: none-or-exact-comma-separated-profile-blocker-IDs$/closed-profile-blockers: none/' \
	-e 's/^builder: replace-with-builder-identity$/builder: builder-session/' \
	-e 's/^independent-reviewer: replace-with-independent-reviewer-identity$/independent-reviewer: reviewer-session/' \
	"$hollow_evidence" >"$hollow_evidence.new"
mv "$hollow_evidence.new" "$hollow_evidence"
expect_failure 'hollow review report' 1 'source-artifact-check: review report profile revision does not identify the reviewed commit' sh tools/build-source-artifact.sh v0.0.0-check "$commit" "$hollow_evidence" "$tmp/hollow-evidence-output"
[ ! -e "$tmp/hollow-evidence-output/yomihon-v0.0.0-check" ] || fail 'hollow review published a destination'

duplicate_evidence="$tmp/duplicate-evidence.md"
cp "$evidence" "$duplicate_evidence"
printf '%s\n' 'final-verdict: NO-GO' >>"$duplicate_evidence"
expect_failure 'contradictory review envelope' 1 'source-artifact: review field is missing, empty, or duplicated: final-verdict' sh tools/build-source-artifact.sh v0.0.0-check "$commit" "$duplicate_evidence" "$tmp/duplicate-evidence-output"

identity_evidence="$tmp/identity-evidence.md"
sed -e 's/^builder: builder-session$/builder: @same/' -e 's/^independent-reviewer: reviewer-session$/independent-reviewer: @same /' "$evidence" >"$identity_evidence"
expect_failure 'reviewer identity alias' 1 'source-artifact: builder cannot be the independent reviewer' sh tools/build-source-artifact.sh v0.0.0-check "$commit" "$identity_evidence" "$tmp/identity-evidence-output"

compound_identity_evidence="$tmp/compound-identity-evidence.md"
sed 's/^builder: builder-session$/builder: builder-session + helper/' "$evidence" >"$compound_identity_evidence"
expect_failure 'compound builder identity' 1 'source-artifact: builder is not a canonical identity token' sh tools/build-source-artifact.sh v0.0.0-check "$commit" "$compound_identity_evidence" "$tmp/compound-identity-evidence-output"

compound_reviewer_evidence="$tmp/compound-reviewer-evidence.md"
sed 's/^independent-reviewer: reviewer-session$/independent-reviewer: reviewer-session + helper/' "$evidence" >"$compound_reviewer_evidence"
expect_failure 'compound reviewer identity' 1 'source-artifact: independent reviewer is not a canonical identity token' sh tools/build-source-artifact.sh v0.0.0-check "$commit" "$compound_reviewer_evidence" "$tmp/compound-reviewer-evidence-output"

cp -R "$pristine" "$tmp/archive-mutation"
printf 'tamper' >>"$tmp/archive-mutation/yomihon-v0.0.0-check.tar.gz"
expect_failure 'archive checksum mutation' 1 'source-artifact-check: bundle checksum mismatch' sh tools/check-source-artifact.sh --allow-fixture v0.0.0-check "$commit" "$tmp/archive-mutation"

cp -R "$pristine" "$tmp/manifest-mutation"
sed '/\.provenance$/d' "$tmp/manifest-mutation/yomihon-v0.0.0-check-SHA256SUMS" >"$tmp/manifest-mutation/manifest.new"
mv "$tmp/manifest-mutation/manifest.new" "$tmp/manifest-mutation/yomihon-v0.0.0-check-SHA256SUMS"
expect_failure 'manifest coverage mutation' 1 'source-artifact-check: manifest must contain exactly four rows' sh tools/check-source-artifact.sh --allow-fixture v0.0.0-check "$commit" "$tmp/manifest-mutation"

cp -R "$pristine" "$tmp/hidden-mutation"
printf '%s\n' hidden >"$tmp/hidden-mutation/.hidden"
expect_failure 'hidden payload mutation' 1 'source-artifact-check: unexpected bundle file: .hidden' sh tools/check-source-artifact.sh --allow-fixture v0.0.0-check "$commit" "$tmp/hidden-mutation"

cp -R "$pristine" "$tmp/provenance-mutation"
provenance="$tmp/provenance-mutation/yomihon-v0.0.0-check.provenance"
sed 's/^source-commit:/source-commit-wrong:/' "$provenance" >"$tmp/provenance-mutation/provenance.new"
mv "$tmp/provenance-mutation/provenance.new" "$provenance"
manifest="$tmp/provenance-mutation/yomihon-v0.0.0-check-SHA256SUMS"
replace_manifest_digest "$manifest" yomihon-v0.0.0-check.provenance "$(sha256_file "$provenance")"
expect_failure 'provenance field mutation' 1 'source-artifact-check: unknown provenance field: source-commit-wrong' sh tools/check-source-artifact.sh --allow-fixture v0.0.0-check "$commit" "$tmp/provenance-mutation"

expect_provenance_digest_failure engineering-standard-sha256 engineering-standard 'source-artifact-check: engineering standard digest mismatch'
expect_provenance_digest_failure project-profile-sha256 project-profile 'source-artifact-check: profile digest mismatch'
expect_provenance_digest_failure review-template-sha256 review-template 'source-artifact-check: review template digest mismatch'
expect_provenance_digest_failure source-artifact-toolchain-sha256 source-artifact-toolchain 'source-artifact-check: source artifact toolchain digest mismatch'
expect_provenance_digest_failure source-artifact-bootstrap-sha256 source-artifact-bootstrap 'source-artifact-check: source artifact bootstrap digest mismatch'
expect_provenance_digest_failure go-mod-sha256 go-mod 'source-artifact-check: go.mod digest mismatch'
expect_provenance_digest_failure go-sum-sha256 go-sum 'source-artifact-check: go.sum digest mismatch'
expect_provenance_digest_failure frontend-lock-sha256 frontend-lock 'source-artifact-check: frontend lock digest mismatch'
expect_provenance_digest_failure bakeoff-go-mod-sha256 bakeoff-go-mod 'source-artifact-check: bake-off go.mod digest mismatch'
expect_provenance_digest_failure bakeoff-go-sum-sha256 bakeoff-go-sum 'source-artifact-check: bake-off go.sum digest mismatch'
expect_provenance_digest_failure ci-workflow-sha256 ci-workflow 'source-artifact-check: CI workflow digest mismatch'
expect_provenance_digest_failure archive-sha256 archive 'source-artifact-check: provenance archive digest mismatch'
expect_provenance_digest_failure verification-evidence-sha256 verification-evidence 'source-artifact-check: provenance evidence digest mismatch'
expect_provenance_digest_failure review-report-sha256 review-report 'source-artifact-check: provenance review-report digest mismatch'

cp -R "$pristine" "$tmp/certificate-report-digest"
certificate="$tmp/certificate-report-digest/yomihon-v0.0.0-check-verification.md"
sed 's/^review-report-sha256:.*/review-report-sha256: 0000000000000000000000000000000000000000000000000000000000000000/' "$certificate" >"$tmp/certificate-report-digest/certificate.new"
mv "$tmp/certificate-report-digest/certificate.new" "$certificate"
certificate_digest=$(sha256_file "$certificate")
provenance="$tmp/certificate-report-digest/yomihon-v0.0.0-check.provenance"
sed "s/^verification-evidence-sha256:.*/verification-evidence-sha256: $certificate_digest/" "$provenance" >"$tmp/certificate-report-digest/provenance.new"
mv "$tmp/certificate-report-digest/provenance.new" "$provenance"
manifest="$tmp/certificate-report-digest/yomihon-v0.0.0-check-SHA256SUMS"
replace_manifest_digest "$manifest" yomihon-v0.0.0-check-verification.md "$certificate_digest"
replace_manifest_digest "$manifest" yomihon-v0.0.0-check.provenance "$(sha256_file "$provenance")"
expect_failure 'certificate report digest binding' 1 'source-artifact-check: release certificate report digest mismatch' sh tools/check-source-artifact.sh --allow-fixture v0.0.0-check "$commit" "$tmp/certificate-report-digest"

cp -R "$pristine" "$tmp/evidence-mutation"
mutated_evidence="$tmp/evidence-mutation/yomihon-v0.0.0-check-verification.md"
sed 's/^gate-2: PASS$/gate-2: PARTIAL/' "$mutated_evidence" >"$tmp/evidence-mutation/evidence.new"
mv "$tmp/evidence-mutation/evidence.new" "$mutated_evidence"
evidence_digest=$(sha256_file "$mutated_evidence")
provenance="$tmp/evidence-mutation/yomihon-v0.0.0-check.provenance"
sed "s/^verification-evidence-sha256:.*/verification-evidence-sha256: $evidence_digest/" "$provenance" >"$tmp/evidence-mutation/provenance.new"
mv "$tmp/evidence-mutation/provenance.new" "$provenance"
manifest="$tmp/evidence-mutation/yomihon-v0.0.0-check-SHA256SUMS"
replace_manifest_digest "$manifest" yomihon-v0.0.0-check-verification.md "$evidence_digest"
replace_manifest_digest "$manifest" yomihon-v0.0.0-check.provenance "$(sha256_file "$provenance")"
expect_failure 'Gate 2 evidence mutation' 1 'source-artifact-check: release certificate Gate 2 is not PASS' sh tools/check-source-artifact.sh --allow-fixture v0.0.0-check "$commit" "$tmp/evidence-mutation"

cp -R "$pristine" "$tmp/malformed-evidence-url"
report="$tmp/malformed-evidence-url/yomihon-v0.0.0-check-review.md"
sed 's|^ci-run: https://example.invalid/yomihon/actions/runs/fixture$|ci-run: https:///missing-authority|' "$report" >"$tmp/malformed-evidence-url/report.new"
mv "$tmp/malformed-evidence-url/report.new" "$report"
repair_fixture_report_chain "$tmp/malformed-evidence-url"
expect_failure 'malformed evidence URL' 1 'source-artifact-check: review report CI run is not an HTTPS URL' sh tools/check-source-artifact.sh --allow-fixture v0.0.0-check "$commit" "$tmp/malformed-evidence-url"

cp -R "$pristine" "$tmp/incomplete-gate2"
report="$tmp/incomplete-gate2/yomihon-v0.0.0-check-review.md"
sed 's/^Supported public surfaces used: Source artifact CLI\.  $/Supported public surfaces used:/' "$report" >"$tmp/incomplete-gate2/report.new"
mv "$tmp/incomplete-gate2/report.new" "$report"
repair_fixture_report_chain "$tmp/incomplete-gate2"
expect_failure 'incomplete Gate 2 human evidence' 1 'source-artifact-check: review-report section field is missing, empty, or duplicated: # Gate 2 — Real-user and third-party-agent usability / Supported public surfaces used' sh tools/check-source-artifact.sh --allow-fixture v0.0.0-check "$commit" "$tmp/incomplete-gate2"

cp -R "$pristine" "$tmp/incomplete-certification"
report="$tmp/incomplete-certification/yomihon-v0.0.0-check-review.md"
sed 's/^I directly inspected or executed the evidence marked verified: yes$/I directly inspected or executed the evidence marked verified: no/' "$report" >"$tmp/incomplete-certification/report.new"
mv "$tmp/incomplete-certification/report.new" "$report"
repair_fixture_report_chain "$tmp/incomplete-certification"
expect_failure 'incomplete independent certification' 1 'source-artifact-check: independent certification does not affirm direct inspection' sh tools/check-source-artifact.sh --allow-fixture v0.0.0-check "$commit" "$tmp/incomplete-certification"

cp -R "$pristine" "$tmp/incomplete-watched-red"
report="$tmp/incomplete-watched-red/yomihon-v0.0.0-check-review.md"
sed 's/^| Fixture requires opt-in\. | Omit --allow-fixture\. |/| Fixture requires opt-in. | |/' "$report" >"$tmp/incomplete-watched-red/report.new"
mv "$tmp/incomplete-watched-red/report.new" "$report"
repair_fixture_report_chain "$tmp/incomplete-watched-red"
expect_failure 'incomplete watched-red mutation' 1 'source-artifact-check: review report has no completed watched-red row' sh tools/check-source-artifact.sh --allow-fixture v0.0.0-check "$commit" "$tmp/incomplete-watched-red"

cp -R "$pristine" "$tmp/incomplete-canonical-section"
report="$tmp/incomplete-canonical-section/yomihon-v0.0.0-check-review.md"
sed '/^| Concept \/ non-concept |/d' "$report" >"$tmp/incomplete-canonical-section/report.new"
mv "$tmp/incomplete-canonical-section/report.new" "$report"
repair_fixture_report_chain "$tmp/incomplete-canonical-section"
expect_failure 'incomplete canonical report section' 1 'source-artifact-check: review report canonical table row is missing or incomplete: # Concept model / Concept / non-concept' sh tools/check-source-artifact.sh --allow-fixture v0.0.0-check "$commit" "$tmp/incomplete-canonical-section"

expect_missing_canonical_row 'missing non-first concept row' missing-concept-row 'Semantic owner' \
	'source-artifact-check: review report canonical table row is missing or incomplete: # Concept model / Semantic owner'
expect_missing_canonical_row 'missing non-first Gate 2 scenario' missing-gate2-row 'Experienced use' \
	'source-artifact-check: review report canonical table row is missing or incomplete: # Gate 2 — Real-user and third-party-agent usability / Experienced use'
expect_missing_canonical_row 'missing non-first Gate 3 class' missing-gate3-row 'Golden / data-driven' \
	'source-artifact-check: review report canonical table row is missing or incomplete: # Gate 3 — Test and evidence-system quality / Golden / data-driven'
expect_missing_canonical_row 'missing non-first security boundary' missing-security-row 'Credential use' \
	'source-artifact-check: review report canonical table row is missing or incomplete: # Security, privacy, performance, and operations / Credential use'
expect_missing_canonical_row 'missing non-first contradiction row' missing-contradiction-row 'Code ↔ package ownership' \
	'source-artifact-check: review report canonical table row is missing or incomplete: # Contradiction hunt / Code ↔ package ownership'

cp -R "$pristine" "$tmp/wrong-standard"
report="$tmp/wrong-standard/yomihon-v0.0.0-check-review.md"
sed "s/^Standard: \`ENGINEERING_STANDARD.md\` version 2\\.0  \$/Standard: NOT-THE-NORMATIVE-STANDARD  /" "$report" >"$tmp/wrong-standard/report.new"
mv "$tmp/wrong-standard/report.new" "$report"
repair_fixture_report_chain "$tmp/wrong-standard"
expect_failure 'review standard identity' 1 'source-artifact-check: review report does not name the normative engineering standard' sh tools/check-source-artifact.sh --allow-fixture v0.0.0-check "$commit" "$tmp/wrong-standard"

cp -R "$pristine" "$tmp/wrong-report-profile-digest"
report="$tmp/wrong-report-profile-digest/yomihon-v0.0.0-check-review.md"
sed 's/^project-profile-sha256: [0-9a-f][0-9a-f]*$/project-profile-sha256: 0000000000000000000000000000000000000000000000000000000000000000/' "$report" >"$tmp/wrong-report-profile-digest/report.new"
mv "$tmp/wrong-report-profile-digest/report.new" "$report"
repair_fixture_report_chain "$tmp/wrong-report-profile-digest"
expect_failure 'review profile digest binding' 1 'source-artifact-check: review report project-profile digest mismatch' sh tools/check-source-artifact.sh --allow-fixture v0.0.0-check "$commit" "$tmp/wrong-report-profile-digest"

cp -R "$pristine" "$tmp/unverified-snapshot"
report="$tmp/unverified-snapshot/yomihon-v0.0.0-check-review.md"
sed 's/^| Go version | go1\.26\.5 | verified |$/| Go version | go1.26.5 | unverified |/' "$report" >"$tmp/unverified-snapshot/report.new"
mv "$tmp/unverified-snapshot/report.new" "$report"
repair_fixture_report_chain "$tmp/unverified-snapshot"
expect_failure 'unverified snapshot status' 1 'source-artifact-check: review report contains a non-final table status: # Snapshot' sh tools/check-source-artifact.sh --allow-fixture v0.0.0-check "$commit" "$tmp/unverified-snapshot"

cp -R "$pristine" "$tmp/extra-unverified-snapshot"
report="$tmp/extra-unverified-snapshot/yomihon-v0.0.0-check-review.md"
sed '/^| Active exceptions |/i\
| Supplemental release state | pending | UNVERIFIED: not executed |' "$report" >"$tmp/extra-unverified-snapshot/report.new"
mv "$tmp/extra-unverified-snapshot/report.new" "$report"
repair_fixture_report_chain "$tmp/extra-unverified-snapshot"
expect_failure 'additional unverified snapshot row' 1 'source-artifact-check: review report contains a non-final table status: # Snapshot' sh tools/check-source-artifact.sh --allow-fixture v0.0.0-check "$commit" "$tmp/extra-unverified-snapshot"

cp -R "$pristine" "$tmp/renamed-snapshot-status"
report="$tmp/renamed-snapshot-status/yomihon-v0.0.0-check-review.md"
sed 's/^| Item | Value | Status |$/| Item | Value | Assessment |/' "$report" >"$tmp/renamed-snapshot-status/report.new"
mv "$tmp/renamed-snapshot-status/report.new" "$report"
repair_fixture_report_chain "$tmp/renamed-snapshot-status"
expect_failure 'renamed canonical status column' 1 'source-artifact-check: review report table schema changed: # Snapshot' sh tools/check-source-artifact.sh --allow-fixture v0.0.0-check "$commit" "$tmp/renamed-snapshot-status"

cp -R "$pristine" "$tmp/gate2-blocker"
report="$tmp/gate2-blocker/yomihon-v0.0.0-check-review.md"
awk '
	$0 == "# Gate 2 — Real-user and third-party-agent usability" { gate2 = 1 }
	gate2 && $0 == "Blockers: none" { print "Blockers: privacy failure remains."; gate2 = 0; next }
	{ print }
' "$report" >"$tmp/gate2-blocker/report.new"
mv "$tmp/gate2-blocker/report.new" "$report"
repair_fixture_report_chain "$tmp/gate2-blocker"
expect_failure 'human Gate 2 blocker' 1 'source-artifact-check: human Gate 2 names blockers while claiming PASS' sh tools/check-source-artifact.sh --allow-fixture v0.0.0-check "$commit" "$tmp/gate2-blocker"

cp -R "$pristine" "$tmp/unresolved-owner"
report="$tmp/unresolved-owner/yomihon-v0.0.0-check-review.md"
sed 's/^| None | N\/A | No open synthetic item\. | N\/A | reviewer-session |$/| Credential authority | UNRESOLVED: owner pending | Provider owner is unknown. | Decide authority. | Koopa |/' "$report" >"$tmp/unresolved-owner/report.new"
mv "$tmp/unresolved-owner/report.new" "$report"
repair_fixture_report_chain "$tmp/unresolved-owner"
expect_failure 'decorated unresolved owner table' 1 'source-artifact-check: review report contains a non-final table status: # NEEDS-OWNER, exceptions, and blocked evidence' sh tools/check-source-artifact.sh --allow-fixture v0.0.0-check "$commit" "$tmp/unresolved-owner"

cp -R "$pristine" "$tmp/open-finding"
report="$tmp/open-finding/yomihon-v0.0.0-check-review.md"
sed 's/^| None | N\/A | verified | CLOSED |/| F-001 | HIGH | verified | OPEN |/' "$report" >"$tmp/open-finding/report.new"
mv "$tmp/open-finding/report.new" "$report"
repair_fixture_report_chain "$tmp/open-finding"
expect_failure 'open finding with repaired evidence chain' 1 'source-artifact-check: review report contains a finding that is not CLOSED' sh tools/check-source-artifact.sh --allow-fixture v0.0.0-check "$commit" "$tmp/open-finding"

cp -R "$pristine" "$tmp/watched-red-reference-mismatch"
report="$tmp/watched-red-reference-mismatch/yomihon-v0.0.0-check-review.md"
sed 's/^watched-red-artifact: sha256:1111111111111111111111111111111111111111111111111111111111111111$/watched-red-artifact: sha256:2222222222222222222222222222222222222222222222222222222222222222/' "$report" >"$tmp/watched-red-reference-mismatch/report.new"
mv "$tmp/watched-red-reference-mismatch/report.new" "$report"
repair_fixture_report_chain "$tmp/watched-red-reference-mismatch"
expect_failure 'watched-red evidence reference disagreement' 1 'source-artifact-check: watched-red row artifact disagrees with the release envelope' sh tools/check-source-artifact.sh --allow-fixture v0.0.0-check "$commit" "$tmp/watched-red-reference-mismatch"

expect_bundle_line_failure 'checker rejects compound certificate builder' certificate-compound-builder \
	yomihon-v0.0.0-check-verification.md \
	'builder: builder-session' 'builder: builder-session + helper' \
	'source-artifact-check: builder is not a canonical identity token'
expect_bundle_line_failure 'checker rejects compound certificate reviewer' certificate-compound-reviewer \
	yomihon-v0.0.0-check-verification.md \
	'independent-reviewer: reviewer-session' 'independent-reviewer: reviewer-session + helper' \
	'source-artifact-check: independent reviewer is not a canonical identity token'
expect_bundle_line_failure 'checker rejects aliased certificate principals' certificate-principal-alias \
	yomihon-v0.0.0-check-verification.md \
	'independent-reviewer: reviewer-session' 'independent-reviewer: builder-session' \
	'source-artifact-check: builder cannot certify the independent review'
expect_bundle_line_failure 'checker rejects placeholder certificate builder' certificate-placeholder-builder \
	yomihon-v0.0.0-check-verification.md \
	'builder: builder-session' 'builder: pending' \
	'source-artifact-check: verification evidence has a placeholder builder'
expect_bundle_line_failure 'checker rejects placeholder certificate reviewer' certificate-placeholder-reviewer \
	yomihon-v0.0.0-check-verification.md \
	'independent-reviewer: reviewer-session' 'independent-reviewer: pending' \
	'source-artifact-check: verification evidence has a placeholder reviewer'
expect_bundle_line_failure 'checker binds report header reviewer' header-reviewer-binding \
	yomihon-v0.0.0-check-review.md \
	'Reviewer: @reviewer-session  ' 'Reviewer: @other-reviewer  ' \
	'source-artifact-check: review report header reviewer mismatch'
expect_bundle_line_failure 'checker binds report header builder' header-builder-binding \
	yomihon-v0.0.0-check-review.md \
	'Builder: @builder-session  ' 'Builder: @other-builder  ' \
	'source-artifact-check: review report header builder mismatch'
expect_bundle_line_failure 'checker binds report-envelope builder' report-builder-binding \
	yomihon-v0.0.0-check-review.md \
	'builder: builder-session' 'builder: other-builder' \
	'source-artifact-check: review report builder mismatch'
expect_bundle_line_failure 'checker binds report-envelope reviewer' report-reviewer-binding \
	yomihon-v0.0.0-check-review.md \
	'independent-reviewer: reviewer-session' 'independent-reviewer: other-reviewer' \
	'source-artifact-check: review report reviewer mismatch'
expect_bundle_line_failure 'Gate 2 independence must be affirmed' gate2-independence-affirmation \
	yomihon-v0.0.0-check-review.md \
	'Both sessions independent from implementation: yes  ' 'Both sessions independent from implementation: no  ' \
	'source-artifact-check: Gate 2 session independence is not affirmed'
expect_bundle_line_failure 'Gate 2 public-surface execution must be affirmed' gate2-public-surface-affirmation \
	yomihon-v0.0.0-check-review.md \
	'Both sessions directly executed the named public surfaces: yes  ' 'Both sessions directly executed the named public surfaces: no  ' \
	'source-artifact-check: Gate 2 sessions did not affirm direct public-surface execution'
expect_bundle_line_failure 'Gate 2 session 1 evidence reference is validated' gate2-session-1-reference \
	yomihon-v0.0.0-check-review.md \
	'gate-2-session-1-artifact: https://example.invalid/yomihon/reviews/gate-2-session-1' 'gate-2-session-1-artifact: not-an-evidence-reference' \
	'source-artifact-check: review report gate-2-session-1-artifact is not an HTTPS URL or SHA-256 identity'
expect_bundle_line_failure 'Gate 2 session 2 evidence reference is validated' gate2-session-2-reference \
	yomihon-v0.0.0-check-review.md \
	'gate-2-session-2-artifact: https://example.invalid/yomihon/reviews/gate-2-session-2' 'gate-2-session-2-artifact: not-an-evidence-reference' \
	'source-artifact-check: review report gate-2-session-2-artifact is not an HTTPS URL or SHA-256 identity'
expect_bundle_line_failure 'independent certification binds the reviewer' certification-reviewer-binding \
	yomihon-v0.0.0-check-review.md \
	'Reviewer: reviewer-session' 'Reviewer: other-reviewer' \
	'source-artifact-check: independent certification reviewer mismatch'

cp -R "$pristine" "$tmp/builder-gate2"
report="$tmp/builder-gate2/yomihon-v0.0.0-check-review.md"
sed 's#^Cold session 1 operator: reviewer-session  $#Cold session 1 operator: @builder-session  #' "$report" >"$tmp/builder-gate2/report.new"
mv "$tmp/builder-gate2/report.new" "$report"
repair_fixture_report_chain "$tmp/builder-gate2"
expect_failure 'builder as Gate 2 operator' 1 'source-artifact-check: builder cannot run Gate 2 cold session 1' sh tools/check-source-artifact.sh --allow-fixture v0.0.0-check "$commit" "$tmp/builder-gate2"

cp -R "$pristine" "$tmp/compound-gate2-operator"
report="$tmp/compound-gate2-operator/yomihon-v0.0.0-check-review.md"
sed 's#^Cold session 1 operator: reviewer-session  $#Cold session 1 operator: builder-session + reviewer-session  #' "$report" >"$tmp/compound-gate2-operator/report.new"
mv "$tmp/compound-gate2-operator/report.new" "$report"
repair_fixture_report_chain "$tmp/compound-gate2-operator"
expect_failure 'compound Gate 2 operator identity' 1 'source-artifact-check: Gate 2 cold-session operator 1 is not a canonical identity token' sh tools/check-source-artifact.sh --allow-fixture v0.0.0-check "$commit" "$tmp/compound-gate2-operator"

expect_bundle_line_failure 'compound Gate 2 operator 2 identity' compound-gate2-operator-2 \
	yomihon-v0.0.0-check-review.md \
	'Cold session 2 operator: reviewer-session  ' 'Cold session 2 operator: builder-session + reviewer-session  ' \
	'source-artifact-check: Gate 2 cold-session operator 2 is not a canonical identity token'

cp -R "$pristine" "$tmp/compound-gate2-session"
report="$tmp/compound-gate2-session/yomihon-v0.0.0-check-review.md"
sed 's#^Cold session 2 session ID: cold-2  $#Cold session 2 session ID: cold 2  #' "$report" >"$tmp/compound-gate2-session/report.new"
mv "$tmp/compound-gate2-session/report.new" "$report"
repair_fixture_report_chain "$tmp/compound-gate2-session"
expect_failure 'non-canonical Gate 2 session ID' 1 'source-artifact-check: Gate 2 cold-session ID 2 is not a canonical token' sh tools/check-source-artifact.sh --allow-fixture v0.0.0-check "$commit" "$tmp/compound-gate2-session"

expect_bundle_line_failure 'non-canonical Gate 2 session ID 1' compound-gate2-session-1 \
	yomihon-v0.0.0-check-review.md \
	'Cold session 1 session ID: cold-1  ' 'Cold session 1 session ID: cold 1  ' \
	'source-artifact-check: Gate 2 cold-session ID 1 is not a canonical token'

expect_bundle_line_failure 'builder as Gate 2 operator 2' builder-gate2-operator-2 \
	yomihon-v0.0.0-check-review.md \
	'Cold session 2 operator: reviewer-session  ' 'Cold session 2 operator: @builder-session  ' \
	'source-artifact-check: builder cannot run Gate 2 cold session 2'

expect_bundle_line_failure 'placeholder Gate 2 operator 1' placeholder-gate2-operator-1 \
	yomihon-v0.0.0-check-review.md \
	'Cold session 1 operator: reviewer-session  ' 'Cold session 1 operator: pending  ' \
	'source-artifact-check: Gate 2 cold-session field is a placeholder'
expect_bundle_line_failure 'placeholder Gate 2 operator 2' placeholder-gate2-operator-2 \
	yomihon-v0.0.0-check-review.md \
	'Cold session 2 operator: reviewer-session  ' 'Cold session 2 operator: pending  ' \
	'source-artifact-check: Gate 2 cold-session field is a placeholder'
expect_bundle_line_failure 'placeholder Gate 2 session 1' placeholder-gate2-session-1 \
	yomihon-v0.0.0-check-review.md \
	'Cold session 1 session ID: cold-1  ' 'Cold session 1 session ID: pending  ' \
	'source-artifact-check: Gate 2 cold-session field is a placeholder'
expect_bundle_line_failure 'placeholder Gate 2 session 2' placeholder-gate2-session-2 \
	yomihon-v0.0.0-check-review.md \
	'Cold session 2 session ID: cold-2  ' 'Cold session 2 session ID: pending  ' \
	'source-artifact-check: Gate 2 cold-session field is a placeholder'

cp -R "$pristine" "$tmp/duplicate-gate2-session"
report="$tmp/duplicate-gate2-session/yomihon-v0.0.0-check-review.md"
sed 's#^Cold session 2 session ID: cold-2  $#Cold session 2 session ID: cold-1  #' "$report" >"$tmp/duplicate-gate2-session/report.new"
mv "$tmp/duplicate-gate2-session/report.new" "$report"
repair_fixture_report_chain "$tmp/duplicate-gate2-session"
expect_failure 'duplicate Gate 2 cold-session ID' 1 'source-artifact-check: Gate 2 cold-session IDs are not distinct' sh tools/check-source-artifact.sh --allow-fixture v0.0.0-check "$commit" "$tmp/duplicate-gate2-session"

cp -R "$pristine" "$tmp/duplicate-gate2-artifact"
report="$tmp/duplicate-gate2-artifact/yomihon-v0.0.0-check-review.md"
sed 's#https://example.invalid/yomihon/reviews/gate-2-session-2#https://example.invalid/yomihon/reviews/gate-2-session-1#g' "$report" >"$tmp/duplicate-gate2-artifact/report.new"
mv "$tmp/duplicate-gate2-artifact/report.new" "$report"
repair_fixture_report_chain "$tmp/duplicate-gate2-artifact"
expect_failure 'duplicate Gate 2 cold-session evidence' 1 'source-artifact-check: Gate 2 cold-session evidence references are not distinct' sh tools/check-source-artifact.sh --allow-fixture v0.0.0-check "$commit" "$tmp/duplicate-gate2-artifact"

cp -R "$pristine" "$tmp/unbound-gate2-session"
report="$tmp/unbound-gate2-session/yomihon-v0.0.0-check-review.md"
sed 's#| Experienced use | Repeat the deterministic build and compare bytes. | Identical bundle bytes. | Identical bytes observed. | none | https://example.invalid/yomihon/reviews/gate-2-session-2 | PASS |#| Experienced use | Repeat the deterministic build and compare bytes. | Identical bundle bytes. | Identical bytes observed. | none | N/A | N/A |#' "$report" >"$tmp/unbound-gate2-session/report.new"
mv "$tmp/unbound-gate2-session/report.new" "$report"
repair_fixture_report_chain "$tmp/unbound-gate2-session"
expect_failure 'Gate 2 second session has no PASS scenario' 1 'source-artifact-check: review report has no completed PASS Gate 2 scenario for https://example.invalid/yomihon/reviews/gate-2-session-2' sh tools/check-source-artifact.sh --allow-fixture v0.0.0-check "$commit" "$tmp/unbound-gate2-session"

expect_bundle_line_failure 'Gate 2 first session has no PASS scenario' unbound-gate2-session-1 \
	yomihon-v0.0.0-check-review.md \
	'| Clean first use | Run the documented source artifact checker. | Fixture is accepted only with explicit opt-in. | Expected contract observed. | none | https://example.invalid/yomihon/reviews/gate-2-session-1 | PASS |' \
	'| Clean first use | Run the documented source artifact checker. | Fixture is accepted only with explicit opt-in. | Expected contract observed. | none | N/A | N/A |' \
	'source-artifact-check: review report has no completed PASS Gate 2 scenario for https://example.invalid/yomihon/reviews/gate-2-session-1'

cp -R "$pristine" "$tmp/report-contradiction"
report="$tmp/report-contradiction/yomihon-v0.0.0-check-review.md"
awk '
	$0 == "# Gate 2 — Real-user and third-party-agent usability" { gate2 = 1 }
	gate2 && $0 == "Status: PASS" { print "Status: PARTIAL"; gate2 = 0; next }
	{ print }
' "$report" >"$tmp/report-contradiction/report.new"
mv "$tmp/report-contradiction/report.new" "$report"
repair_fixture_report_chain "$tmp/report-contradiction"
expect_failure 'human review contradiction' 1 'source-artifact-check: human Gate 2 contradicts the release envelope' sh tools/check-source-artifact.sh --allow-fixture v0.0.0-check "$commit" "$tmp/report-contradiction"

cp -R "$pristine" "$tmp/subset-archive"
archive="$tmp/subset-archive/yomihon-v0.0.0-check.tar.gz"
git -c tar.umask=0002 archive --format=tar --prefix='yomihon-0.0.0-check/' "$commit" README.md | gzip -n >"$archive"
archive_digest=$(sha256_file "$archive")
provenance="$tmp/subset-archive/yomihon-v0.0.0-check.provenance"
sed "s/^archive-sha256:.*/archive-sha256: $archive_digest/" "$provenance" >"$tmp/subset-archive/provenance.new"
mv "$tmp/subset-archive/provenance.new" "$provenance"
manifest="$tmp/subset-archive/yomihon-v0.0.0-check-SHA256SUMS"
replace_manifest_digest "$manifest" yomihon-v0.0.0-check.tar.gz "$archive_digest"
replace_manifest_digest "$manifest" yomihon-v0.0.0-check.provenance "$(sha256_file "$provenance")"
expect_failure 'subset archive with repaired hashes' 1 'source-artifact-check: archive is not the complete commit tree' sh tools/check-source-artifact.sh --allow-fixture v0.0.0-check "$commit" "$tmp/subset-archive"

fixture="$tmp/repository"
mkdir "$fixture"
git archive HEAD | tar -x -C "$fixture"
cp tools/testdata/source-artifact-approved-profile.md "$fixture/PROJECT_PROFILE.md"
git -C "$fixture" init -q
git -C "$fixture" config user.name artifact-test
git -C "$fixture" config user.email artifact-test@example.invalid
cat >"$fixture/tools/source-artifact-toolchain.txt" <<EOF
format: yomihon-source-artifact-toolchain-v1
git-version: $(git version)
gzip-version: $(gzip --version 2>&1 | sed -n '1p')
EOF
git -C "$fixture" add -A
git -C "$fixture" commit -qm 'test source artifact'
fixture_commit=$(git -C "$fixture" rev-parse HEAD)
fixture_evidence="$tmp/fixture-review.md"
write_release_evidence "$fixture_evidence" "$fixture_commit" 1111111111111111111111111111111111111111111111111111111111111111 "$fixture"

expect_failure 'missing release tag' 1 'source-artifact: v0.0.0-check must be an annotated tag' sh "$fixture/tools/build-source-artifact.sh" --require-tag v0.0.0-check "$fixture_commit" "$fixture_evidence" "$tmp/no-tag"
git -C "$fixture" tag v0.0.0-check "$fixture_commit"
expect_failure 'lightweight release tag' 1 'source-artifact: v0.0.0-check must be an annotated tag' sh "$fixture/tools/build-source-artifact.sh" --require-tag v0.0.0-check "$fixture_commit" "$fixture_evidence" "$tmp/lightweight"
git -C "$fixture" tag -d v0.0.0-check >/dev/null

git -C "$fixture" tag -a v0.0.0-other -m v0.0.0-other "$fixture_commit"
git -C "$fixture" update-ref refs/tags/v0.0.0-check "$(git -C "$fixture" rev-parse refs/tags/v0.0.0-other)"
expect_failure 'annotated tag internal name mismatch' 1 'source-artifact: annotated tag object name does not match v0.0.0-check' sh "$fixture/tools/build-source-artifact.sh" --require-tag v0.0.0-check "$fixture_commit" "$fixture_evidence" "$tmp/wrong-tag-name"
git -C "$fixture" update-ref -d refs/tags/v0.0.0-check
git -C "$fixture" tag -d v0.0.0-other >/dev/null

git -C "$fixture" tag -a v0.0.0-check -m v0.0.0-check "$fixture_commit"

candidate_archive="$tmp/prepared/yomihon-v0.0.0-check.tar.gz"
mkdir -p "${candidate_archive%/*}"
expect_success 'prepare exact formal review archive' run_committed_bootstrap "$fixture" "$fixture_commit" \
	--prepare-archive v0.0.0-check "$fixture_commit" "$candidate_archive"
candidate_digest=$(sha256_file "$candidate_archive")
write_release_evidence "$fixture_evidence" "$fixture_commit" "$candidate_digest" "$fixture"

wrong_digest_evidence="$tmp/wrong-archive-digest-review.md"
write_release_evidence "$wrong_digest_evidence" "$fixture_commit" 2222222222222222222222222222222222222222222222222222222222222222 "$fixture"
expect_failure 'formal report archive digest mismatch' 1 'source-artifact-check: release report artifact digest does not identify the source archive' \
	run_committed_bootstrap "$fixture" "$fixture_commit" --assemble v0.0.0-check "$fixture_commit" "$wrong_digest_evidence" "$tmp/wrong-archive-digest"

wrong_closure_evidence="$tmp/wrong-profile-closure-review.md"
sed 's/^closed-profile-blockers: ARTIFACT-EVIDENCE$/closed-profile-blockers: OTHER-REVIEW/' "$fixture_evidence" >"$wrong_closure_evidence"
expect_failure 'formal report closes wrong profile blocker set' 1 'source-artifact: review evidence does not close exactly the post-artifact profile blockers' \
	run_committed_bootstrap "$fixture" "$fixture_commit" --assemble v0.0.0-check "$fixture_commit" "$wrong_closure_evidence" "$tmp/wrong-profile-closure"

url_gate2_evidence="$tmp/url-gate2-review.md"
sed 's#sha256:2222222222222222222222222222222222222222222222222222222222222222#https://github.com/koopa0/yomihon/reviews/gate-2-session-1#g' "$fixture_evidence" >"$url_gate2_evidence"
expect_failure 'formal Gate 2 evidence uses a URL identity' 1 'source-artifact-check: formal Gate 2 cold-session evidence must use SHA-256 identities' \
	run_committed_bootstrap "$fixture" "$fixture_commit" --assemble v0.0.0-check "$fixture_commit" "$url_gate2_evidence" "$tmp/url-gate2"

duplicate_gate2_digest_evidence="$tmp/duplicate-gate2-digest-review.md"
sed 's/sha256:3333333333333333333333333333333333333333333333333333333333333333/sha256:2222222222222222222222222222222222222222222222222222222222222222/g' "$fixture_evidence" >"$duplicate_gate2_digest_evidence"
expect_failure 'formal Gate 2 evidence reuses one digest' 1 'source-artifact-check: Gate 2 cold-session evidence references are not distinct' \
	run_committed_bootstrap "$fixture" "$fixture_commit" --assemble v0.0.0-check "$fixture_commit" "$duplicate_gate2_digest_evidence" "$tmp/duplicate-gate2-digest"

wrong_closure_row_evidence="$tmp/wrong-profile-closure-row-review.md"
sed 's/^| ARTIFACT-EVIDENCE | CLOSED |/| OTHER-REVIEW | CLOSED |/' "$fixture_evidence" >"$wrong_closure_row_evidence"
expect_failure 'formal report closure rows disagree with envelope' 1 'source-artifact-check: review report profile-blocker closure rows disagree with the release envelope' \
	run_committed_bootstrap "$fixture" "$fixture_commit" --assemble v0.0.0-check "$fixture_commit" "$wrong_closure_row_evidence" "$tmp/wrong-profile-closure-row"

fixture_only_evidence="$tmp/fixture-only-review.md"
write_evidence "$fixture_only_evidence" "$fixture_commit" PASS "$fixture"
expect_failure 'fixture evidence cannot become a formal release' 1 'source-artifact: formal release requires release-candidate review evidence' \
	run_committed_bootstrap "$fixture" "$fixture_commit" --assemble v0.0.0-check "$fixture_commit" "$fixture_only_evidence" "$tmp/fixture-as-release"

profile_blocked_fixture="$tmp/profile-blocked-repository"
git clone -q "$fixture" "$profile_blocked_fixture"
git -C "$profile_blocked_fixture" config user.name artifact-test
git -C "$profile_blocked_fixture" config user.email artifact-test@example.invalid
sed 's/^merge-readiness: GO$/merge-readiness: NO-GO/' "$profile_blocked_fixture/PROJECT_PROFILE.md" >"$profile_blocked_fixture/PROJECT_PROFILE.md.new"
mv "$profile_blocked_fixture/PROJECT_PROFILE.md.new" "$profile_blocked_fixture/PROJECT_PROFILE.md"
git -C "$profile_blocked_fixture" add PROJECT_PROFILE.md
git -C "$profile_blocked_fixture" commit -qm 'block profile merge readiness'
profile_blocked_commit=$(git -C "$profile_blocked_fixture" rev-parse HEAD)
git -C "$profile_blocked_fixture" tag -a v0.0.0-profile-blocked -m v0.0.0-profile-blocked "$profile_blocked_commit"
write_release_evidence "$fixture_evidence" "$profile_blocked_commit" 1111111111111111111111111111111111111111111111111111111111111111 "$profile_blocked_fixture"
expect_failure 'profile release readiness cannot outrun merge readiness' 1 'source-artifact: project profile merge readiness is not GO' \
	run_committed_bootstrap "$profile_blocked_fixture" "$profile_blocked_commit" --assemble v0.0.0-profile-blocked "$profile_blocked_commit" "$fixture_evidence" "$tmp/profile-blocked"
write_release_evidence "$fixture_evidence" "$fixture_commit" "$candidate_digest" "$fixture"

expect_profile_preflight_failure \
	'profile status is not approved' profile-status \
	'profile-status: APPROVED' 'profile-status: PROPOSED' \
	'source-artifact: formal source artifact requires an approved project profile'
expect_profile_preflight_failure \
	'profile artifact-build readiness is not GO' artifact-readiness \
	'artifact-build-readiness: GO' 'artifact-build-readiness: NO-GO' \
	'source-artifact: project profile artifact-build readiness is not GO'
expect_profile_preflight_failure \
	'profile artifact-build blockers are not empty' artifact-blockers \
	'artifact-build-blockers: none' 'artifact-build-blockers: BUILD-ENV' \
	'source-artifact: project profile has artifact-build blockers'
expect_profile_preflight_failure \
	'profile release state is not pending artifact evidence' release-state \
	'release-readiness: PENDING-ARTIFACT' 'release-readiness: GO' \
	'source-artifact: project profile is not in the PENDING-ARTIFACT state'
expect_profile_preflight_failure \
	'profile names no post-artifact blocker' post-artifact-empty \
	'post-artifact-blockers: ARTIFACT-EVIDENCE' 'post-artifact-blockers: none' \
	'source-artifact: project profile names no blocker for artifact evidence to close'
expect_profile_preflight_failure \
	'profile open blockers exceed post-artifact blockers' open-blockers \
	'open-blockers: ARTIFACT-EVIDENCE' 'open-blockers: OTHER-BLOCKER' \
	'source-artifact: open profile blockers are not exclusively post-artifact blockers'
expect_profile_preflight_failure \
	'profile has an active release exception' active-exception \
	'active-exceptions: none' 'active-exceptions: EX-TEST' \
	'source-artifact: project profile has active exceptions'

for readiness_shape in reordered extra-row duplicate-block; do
	readiness_fixture="$tmp/profile-readiness-$readiness_shape-repository"
	readiness_version="v0.0.0-readiness-$readiness_shape"
	git clone -q "$fixture" "$readiness_fixture"
	git -C "$readiness_fixture" config user.name artifact-test
	git -C "$readiness_fixture" config user.email artifact-test@example.invalid
	case "$readiness_shape" in
	reordered)
		awk '
			$0 == "merge-readiness: GO" { merge = $0; next }
			$0 == "artifact-build-readiness: GO" { print; print merge; next }
			{ print }
		' "$readiness_fixture/PROJECT_PROFILE.md" >"$readiness_fixture/PROJECT_PROFILE.md.new"
		;;
	extra-row)
		awk '
			{ print }
			$0 == "profile-status: APPROVED" { print "unknown-readiness-field: forbidden" }
		' "$readiness_fixture/PROJECT_PROFILE.md" >"$readiness_fixture/PROJECT_PROFILE.md.new"
		;;
	duplicate-block)
		awk '
			{ print }
			$0 == "active-exceptions: none" { duplicate_after_close = 1; next }
			duplicate_after_close && $0 == "```" {
				print ""
				print "```text"
				print "```"
				duplicate_after_close = 0
			}
		' "$readiness_fixture/PROJECT_PROFILE.md" >"$readiness_fixture/PROJECT_PROFILE.md.new"
		;;
	esac
	mv "$readiness_fixture/PROJECT_PROFILE.md.new" "$readiness_fixture/PROJECT_PROFILE.md"
	case "$readiness_shape" in
	reordered)
		awk '
			$0 == "artifact-build-readiness: GO" { artifact = NR }
			$0 == "merge-readiness: GO" { merge = NR }
			END { exit !(artifact && merge && artifact < merge) }
		' "$readiness_fixture/PROJECT_PROFILE.md" || fail 'readiness reorder mutation did not apply'
		;;
	extra-row)
		grep -qxF 'unknown-readiness-field: forbidden' "$readiness_fixture/PROJECT_PROFILE.md" || fail 'readiness extra-row mutation did not apply'
		;;
	duplicate-block)
		[ "$(grep -cxF '```text' "$readiness_fixture/PROJECT_PROFILE.md" || true)" -eq 3 ] || fail 'readiness duplicate-block mutation did not apply'
		;;
	esac
	git -C "$readiness_fixture" add PROJECT_PROFILE.md
	git -C "$readiness_fixture" commit -qm "invalidate $readiness_shape readiness structure"
	readiness_commit=$(git -C "$readiness_fixture" rev-parse HEAD)
	git -C "$readiness_fixture" tag -a "$readiness_version" -m "$readiness_version" "$readiness_commit"
	write_release_evidence "$fixture_evidence" "$readiness_commit" 1111111111111111111111111111111111111111111111111111111111111111 "$readiness_fixture"
	expect_failure "profile readiness structure $readiness_shape" 1 'source-artifact: profile readiness envelope is malformed or missing field: profile-status' \
		run_committed_bootstrap "$readiness_fixture" "$readiness_commit" --assemble "$readiness_version" "$readiness_commit" "$fixture_evidence" "$tmp/readiness-$readiness_shape-output"
done

misplaced_profile_fixture="$tmp/profile-misplaced-envelope-repository"
git clone -q "$fixture" "$misplaced_profile_fixture"
git -C "$misplaced_profile_fixture" config user.name artifact-test
git -C "$misplaced_profile_fixture" config user.email artifact-test@example.invalid
awk '
	$0 == "## 18. Exceptions and current debt" { print "profile-status: APPROVED"; print "" }
	$0 == "profile-status: APPROVED" { next }
	{ print }
' "$misplaced_profile_fixture/PROJECT_PROFILE.md" >"$misplaced_profile_fixture/PROJECT_PROFILE.md.new"
mv "$misplaced_profile_fixture/PROJECT_PROFILE.md.new" "$misplaced_profile_fixture/PROJECT_PROFILE.md"
git -C "$misplaced_profile_fixture" add PROJECT_PROFILE.md
git -C "$misplaced_profile_fixture" commit -qm 'move readiness field outside its owner section'
misplaced_profile_commit=$(git -C "$misplaced_profile_fixture" rev-parse HEAD)
git -C "$misplaced_profile_fixture" tag -a v0.0.0-misplaced-envelope -m v0.0.0-misplaced-envelope "$misplaced_profile_commit"
write_release_evidence "$fixture_evidence" "$misplaced_profile_commit" 1111111111111111111111111111111111111111111111111111111111111111 "$misplaced_profile_fixture"
expect_failure 'profile readiness field moved outside section 20' 1 'source-artifact: profile readiness envelope is malformed or missing field: profile-status' \
	run_committed_bootstrap "$misplaced_profile_fixture" "$misplaced_profile_commit" --assemble v0.0.0-misplaced-envelope "$misplaced_profile_commit" "$fixture_evidence" "$tmp/misplaced-envelope"

misplaced_approval_fixture="$tmp/profile-misplaced-approval-repository"
git clone -q "$fixture" "$misplaced_approval_fixture"
git -C "$misplaced_approval_fixture" config user.name artifact-test
git -C "$misplaced_approval_fixture" config user.email artifact-test@example.invalid
awk '
	$0 == "Independent approval: APPROVED" { approval = $0; next }
	$0 == "## 20. Machine-readable readiness envelope" {
		print "## Approval appendix"
		print ""
		print approval
		print ""
	}
	{ print }
' "$misplaced_approval_fixture/PROJECT_PROFILE.md" >"$misplaced_approval_fixture/PROJECT_PROFILE.md.new"
mv "$misplaced_approval_fixture/PROJECT_PROFILE.md.new" "$misplaced_approval_fixture/PROJECT_PROFILE.md"
awk '
	$0 == "## Approval appendix" {
		getline
		getline
		if ($0 == "Independent approval: APPROVED") found = 1
	}
	END { exit !found }
' "$misplaced_approval_fixture/PROJECT_PROFILE.md" || fail 'approval relocation mutation did not apply'
git -C "$misplaced_approval_fixture" add PROJECT_PROFILE.md
git -C "$misplaced_approval_fixture" commit -qm 'move approval under an unrelated sibling section'
misplaced_approval_commit=$(git -C "$misplaced_approval_fixture" rev-parse HEAD)
git -C "$misplaced_approval_fixture" tag -a v0.0.0-misplaced-approval -m v0.0.0-misplaced-approval "$misplaced_approval_commit"
write_release_evidence "$fixture_evidence" "$misplaced_approval_commit" 1111111111111111111111111111111111111111111111111111111111111111 "$misplaced_approval_fixture"
expect_failure 'profile approval moved outside section 19' 1 'source-artifact: approved profile has incomplete human approvals' \
	run_committed_bootstrap "$misplaced_approval_fixture" "$misplaced_approval_commit" --assemble v0.0.0-misplaced-approval "$misplaced_approval_commit" "$fixture_evidence" "$tmp/misplaced-approval-output"

expect_profile_preflight_failure \
	'profile version is still a draft' profile-version \
	'Profile version: 1.0' 'Profile version: 1.0-draft' \
	'source-artifact: approved profile has incomplete human approvals'
expect_profile_preflight_failure \
	'profile architecture approval is pending' architecture-approval \
	'Architecture approval: APPROVED' 'Architecture approval: PENDING — fixture mutation' \
	'source-artifact: approved profile has incomplete human approvals'
expect_profile_preflight_failure \
	'profile security approval is pending' security-approval \
	'Security / privacy approval where applicable: N/A — synthetic repository with no personal data' 'Security / privacy approval where applicable: PENDING — fixture mutation' \
	'source-artifact: approved profile has incomplete human approvals'
expect_profile_preflight_failure \
	'profile operations approval is pending' operations-approval \
	'Operations approval where applicable: N/A — temporary local fixture repository' 'Operations approval where applicable: PENDING — fixture mutation' \
	'source-artifact: approved profile has incomplete human approvals'

approval_blocked_fixture="$tmp/approval-blocked-repository"
git clone -q "$fixture" "$approval_blocked_fixture"
git -C "$approval_blocked_fixture" config user.name artifact-test
git -C "$approval_blocked_fixture" config user.email artifact-test@example.invalid
sed 's/^Independent approval: APPROVED$/Independent approval: PENDING — fixture mutation/' "$approval_blocked_fixture/PROJECT_PROFILE.md" >"$approval_blocked_fixture/PROJECT_PROFILE.md.new"
mv "$approval_blocked_fixture/PROJECT_PROFILE.md.new" "$approval_blocked_fixture/PROJECT_PROFILE.md"
git -C "$approval_blocked_fixture" add PROJECT_PROFILE.md
git -C "$approval_blocked_fixture" commit -qm 'invalidate profile human approval'
approval_blocked_commit=$(git -C "$approval_blocked_fixture" rev-parse HEAD)
git -C "$approval_blocked_fixture" tag -a v0.0.0-approval-blocked -m v0.0.0-approval-blocked "$approval_blocked_commit"
write_release_evidence "$fixture_evidence" "$approval_blocked_commit" 1111111111111111111111111111111111111111111111111111111111111111 "$approval_blocked_fixture"
expect_failure 'approved envelope with pending human approval' 1 'source-artifact: approved profile has incomplete human approvals' \
	run_committed_bootstrap "$approval_blocked_fixture" "$approval_blocked_commit" --assemble v0.0.0-approval-blocked "$approval_blocked_commit" "$fixture_evidence" "$tmp/approval-blocked"

approval_binding_fixture="$tmp/approval-binding-repository"
git clone -q "$fixture" "$approval_binding_fixture"
git -C "$approval_binding_fixture" config user.name artifact-test
git -C "$approval_binding_fixture" config user.email artifact-test@example.invalid
sed 's/^Approval binding: EXTERNAL-RELEASE-REPORT$/Approval binding: looks-approved-but-is-not-bound/' "$approval_binding_fixture/PROJECT_PROFILE.md" >"$approval_binding_fixture/PROJECT_PROFILE.md.new"
mv "$approval_binding_fixture/PROJECT_PROFILE.md.new" "$approval_binding_fixture/PROJECT_PROFILE.md"
git -C "$approval_binding_fixture" add PROJECT_PROFILE.md
git -C "$approval_binding_fixture" commit -qm 'invalidate external profile approval binding'
approval_binding_commit=$(git -C "$approval_binding_fixture" rev-parse HEAD)
git -C "$approval_binding_fixture" tag -a v0.0.0-approval-binding -m v0.0.0-approval-binding "$approval_binding_commit"
write_release_evidence "$fixture_evidence" "$approval_binding_commit" 1111111111111111111111111111111111111111111111111111111111111111 "$approval_binding_fixture"
expect_failure 'approved profile has arbitrary approval-binding prose' 1 'source-artifact: approved profile has incomplete human approvals' \
	run_committed_bootstrap "$approval_binding_fixture" "$approval_binding_commit" --assemble v0.0.0-approval-binding "$approval_binding_commit" "$fixture_evidence" "$tmp/approval-binding"

blocker_table_fixture="$tmp/blocker-table-repository"
git clone -q "$fixture" "$blocker_table_fixture"
git -C "$blocker_table_fixture" config user.name artifact-test
git -C "$blocker_table_fixture" config user.email artifact-test@example.invalid
sed 's/^| ARTIFACT-EVIDENCE |/| OTHER-REVIEW |/' "$blocker_table_fixture/PROJECT_PROFILE.md" >"$blocker_table_fixture/PROJECT_PROFILE.md.new"
mv "$blocker_table_fixture/PROJECT_PROFILE.md.new" "$blocker_table_fixture/PROJECT_PROFILE.md"
git -C "$blocker_table_fixture" add PROJECT_PROFILE.md
git -C "$blocker_table_fixture" commit -qm 'desynchronize profile blocker table'
blocker_table_commit=$(git -C "$blocker_table_fixture" rev-parse HEAD)
git -C "$blocker_table_fixture" tag -a v0.0.0-blocker-table -m v0.0.0-blocker-table "$blocker_table_commit"
write_release_evidence "$fixture_evidence" "$blocker_table_commit" 1111111111111111111111111111111111111111111111111111111111111111 "$blocker_table_fixture"
expect_failure 'profile blocker table disagrees with readiness envelope' 1 'source-artifact: profile readiness blockers disagree with the Current blockers table' \
	run_committed_bootstrap "$blocker_table_fixture" "$blocker_table_commit" --assemble v0.0.0-blocker-table "$blocker_table_commit" "$fixture_evidence" "$tmp/blocker-table"

blocker_owner_fixture="$tmp/blocker-owner-repository"
git clone -q "$fixture" "$blocker_owner_fixture"
git -C "$blocker_owner_fixture" config user.name artifact-test
git -C "$blocker_owner_fixture" config user.email artifact-test@example.invalid
sed 's/^## 18\. Exceptions and current debt$/## 18. Not the blocker owner/' "$blocker_owner_fixture/PROJECT_PROFILE.md" >"$blocker_owner_fixture/PROJECT_PROFILE.md.new"
mv "$blocker_owner_fixture/PROJECT_PROFILE.md.new" "$blocker_owner_fixture/PROJECT_PROFILE.md"
git -C "$blocker_owner_fixture" add PROJECT_PROFILE.md
git -C "$blocker_owner_fixture" commit -qm 'move blocker table outside its owner section'
blocker_owner_commit=$(git -C "$blocker_owner_fixture" rev-parse HEAD)
git -C "$blocker_owner_fixture" tag -a v0.0.0-blocker-owner -m v0.0.0-blocker-owner "$blocker_owner_commit"
write_release_evidence "$fixture_evidence" "$blocker_owner_commit" 1111111111111111111111111111111111111111111111111111111111111111 "$blocker_owner_fixture"
expect_failure 'profile blocker table moved outside section 18' 1 'source-artifact: profile readiness blockers disagree with the Current blockers table' \
	run_committed_bootstrap "$blocker_owner_fixture" "$blocker_owner_commit" --assemble v0.0.0-blocker-owner "$blocker_owner_commit" "$fixture_evidence" "$tmp/blocker-owner"

for blocker_shape in malformed-columns invalid-id duplicate-caption duplicate-id empty-owner; do
	blocker_shape_fixture="$tmp/blocker-shape-$blocker_shape-repository"
	blocker_shape_version="v0.0.0-blocker-$blocker_shape"
	git clone -q "$fixture" "$blocker_shape_fixture"
	git -C "$blocker_shape_fixture" config user.name artifact-test
	git -C "$blocker_shape_fixture" config user.email artifact-test@example.invalid
	case "$blocker_shape" in
	malformed-columns)
		awk '
			/^\| ARTIFACT-EVIDENCE \|/ {
				print "| ARTIFACT-EVIDENCE | risk | containment | owner |"
				next
			}
			{ print }
		' "$blocker_shape_fixture/PROJECT_PROFILE.md" >"$blocker_shape_fixture/PROJECT_PROFILE.md.new"
		;;
	invalid-id)
		sed 's/^| ARTIFACT-EVIDENCE |/| artifact-evidence |/' "$blocker_shape_fixture/PROJECT_PROFILE.md" >"$blocker_shape_fixture/PROJECT_PROFILE.md.new"
		;;
	duplicate-caption)
		awk '{ print; if ($0 == "Current blockers:") print }' "$blocker_shape_fixture/PROJECT_PROFILE.md" >"$blocker_shape_fixture/PROJECT_PROFILE.md.new"
		;;
	duplicate-id)
		awk '{ print; if ($0 ~ /^\| ARTIFACT-EVIDENCE \|/) print }' "$blocker_shape_fixture/PROJECT_PROFILE.md" >"$blocker_shape_fixture/PROJECT_PROFILE.md.new"
		;;
	empty-owner)
		awk '
			/^\| ARTIFACT-EVIDENCE \|/ {
				print "| ARTIFACT-EVIDENCE | risk | containment |  | gate effect |"
				next
			}
			{ print }
		' "$blocker_shape_fixture/PROJECT_PROFILE.md" >"$blocker_shape_fixture/PROJECT_PROFILE.md.new"
		;;
	esac
	mv "$blocker_shape_fixture/PROJECT_PROFILE.md.new" "$blocker_shape_fixture/PROJECT_PROFILE.md"
	case "$blocker_shape" in
	malformed-columns)
		grep -qxF '| ARTIFACT-EVIDENCE | risk | containment | owner |' "$blocker_shape_fixture/PROJECT_PROFILE.md" || fail 'malformed blocker-column mutation did not apply'
		;;
	invalid-id)
		grep -q '^| artifact-evidence |' "$blocker_shape_fixture/PROJECT_PROFILE.md" || fail 'invalid blocker-ID mutation did not apply'
		;;
	duplicate-caption)
		[ "$(grep -cxF 'Current blockers:' "$blocker_shape_fixture/PROJECT_PROFILE.md" || true)" -eq 2 ] || fail 'duplicate blocker-caption mutation did not apply'
		;;
	duplicate-id)
		[ "$(grep -c '^| ARTIFACT-EVIDENCE |' "$blocker_shape_fixture/PROJECT_PROFILE.md" || true)" -eq 2 ] || fail 'duplicate blocker-ID mutation did not apply'
		;;
	empty-owner)
		grep -qxF '| ARTIFACT-EVIDENCE | risk | containment |  | gate effect |' "$blocker_shape_fixture/PROJECT_PROFILE.md" || fail 'empty blocker-owner mutation did not apply'
		;;
	esac
	git -C "$blocker_shape_fixture" add PROJECT_PROFILE.md
	git -C "$blocker_shape_fixture" commit -qm "invalidate blocker table with $blocker_shape"
	blocker_shape_commit=$(git -C "$blocker_shape_fixture" rev-parse HEAD)
	git -C "$blocker_shape_fixture" tag -a "$blocker_shape_version" -m "$blocker_shape_version" "$blocker_shape_commit"
	write_release_evidence "$fixture_evidence" "$blocker_shape_commit" 1111111111111111111111111111111111111111111111111111111111111111 "$blocker_shape_fixture"
	expect_failure "profile blocker table $blocker_shape" 1 'source-artifact: profile readiness blockers disagree with the Current blockers table' \
		run_committed_bootstrap "$blocker_shape_fixture" "$blocker_shape_commit" --assemble "$blocker_shape_version" "$blocker_shape_commit" "$fixture_evidence" "$tmp/blocker-$blocker_shape-output"
done
write_release_evidence "$fixture_evidence" "$fixture_commit" "$candidate_digest" "$fixture"

reserved_origin_evidence="$tmp/reserved-origin-review.md"
sed 's#https://github\.com/koopa0/yomihon#https://example.invalid/yomihon#g' "$fixture_evidence" >"$reserved_origin_evidence"
expect_failure 'formal evidence reserved origin' 1 'uses a reserved or local HTTPS origin' \
	run_committed_bootstrap "$fixture" "$fixture_commit" --assemble v0.0.0-check "$fixture_commit" "$reserved_origin_evidence" "$tmp/reserved-origin"

expect_success 'assemble candidate-bound formal bundle' run_committed_bootstrap "$fixture" "$fixture_commit" \
	--assemble v0.0.0-check "$fixture_commit" "$fixture_evidence" "$tmp/tagged"
(cd "$fixture" && cmp "$candidate_archive" "$tmp/tagged/yomihon-v0.0.0-check/yomihon-v0.0.0-check.tar.gz")
(cd "$fixture" && sh tools/check-source-artifact.sh v0.0.0-check "$fixture_commit" "$tmp/tagged/yomihon-v0.0.0-check")

git -C "$fixture" tag -a v0.0.0-prepare-race -m v0.0.0-prepare-race "$fixture_commit"
mkdir "$tmp/prepare-mutation-bin"
cat >"$tmp/prepare-mutation-bin/gzip" <<'EOF'
#!/bin/sh
set -eu
case "${1:-}" in
--version) exec "$PREPARE_REAL_GZIP" "$@" ;;
esac
"$PREPARE_REAL_GZIP" "$@"
status=$?
if [ ! -e "$PREPARE_MUTATION_MARKER" ]; then
	printf '%s\n' 'prepare mutation' >>README.md
	: >"$PREPARE_MUTATION_MARKER"
fi
exit "$status"
EOF
chmod 0755 "$tmp/prepare-mutation-bin/gzip"
real_gzip=$(command -v gzip)
saved_path=$PATH
PREPARE_REAL_GZIP=$real_gzip
PREPARE_MUTATION_MARKER="$tmp/prepare-mutation.marker"
export PREPARE_REAL_GZIP PREPARE_MUTATION_MARKER
PATH="$tmp/prepare-mutation-bin:$PATH"
export PATH
expect_failure 'in-flight prepared-archive source mutation' 1 'source-artifact: release checkout is not clean' \
	run_committed_bootstrap "$fixture" "$fixture_commit" --prepare-archive v0.0.0-prepare-race "$fixture_commit" "$tmp/prepare-mutation.tar.gz"
PATH=$saved_path
export PATH
unset PREPARE_REAL_GZIP PREPARE_MUTATION_MARKER
[ -e "$tmp/prepare-mutation.marker" ] || fail 'prepared-archive source mutation did not execute'
[ ! -e "$tmp/prepare-mutation.tar.gz" ] || fail 'prepared-archive mutation published an archive'

git -C "$fixture" tag -a v0.0.0-prepare-destination-race -m v0.0.0-prepare-destination-race "$fixture_commit"
mkdir "$tmp/prepare-destination-race-bin"
cat >"$tmp/prepare-destination-race-bin/ln" <<'EOF'
#!/bin/sh
set -eu
last=
for argument do last=$argument; done
printf '%s\n' rival >"$last"
: >"$PREPARE_DESTINATION_MARKER"
exec "$PREPARE_REAL_LN" "$@"
EOF
chmod 0755 "$tmp/prepare-destination-race-bin/ln"
prepare_race_output="$tmp/prepare-destination-race.tar.gz"
saved_path=$PATH
PREPARE_REAL_LN=$(command -v ln)
PREPARE_DESTINATION_MARKER="$tmp/prepare-destination-race.marker"
export PREPARE_REAL_LN PREPARE_DESTINATION_MARKER
PATH="$tmp/prepare-destination-race-bin:$PATH"
export PATH
expect_failure 'prepared-archive destination appearance' 1 'source-artifact: archive destination appeared before publication' \
	run_committed_bootstrap "$fixture" "$fixture_commit" --prepare-archive v0.0.0-prepare-destination-race "$fixture_commit" "$prepare_race_output"
PATH=$saved_path
export PATH
unset PREPARE_REAL_LN PREPARE_DESTINATION_MARKER
[ -e "$tmp/prepare-destination-race.marker" ] || fail 'prepared-archive destination race did not execute'
[ "$(cat "$prepare_race_output")" = rival ] || fail 'prepared-archive destination race overwrote the competing file'

mkdir -p "$fixture/.git/info"
printf '%s\n' 'README.md export-ignore' >"$fixture/.git/info/attributes"
expect_failure 'repository-local archive attributes' 1 'source-artifact: unsafe Git context: repository-local-attributes' sh "$fixture/tools/build-source-artifact.sh" --require-tag v0.0.0-check "$fixture_commit" "$fixture_evidence" "$tmp/local-attributes"
rm "$fixture/.git/info/attributes"

printf '%s\n' "$fixture_commit" >"$fixture/.git/info/grafts"
expect_failure 'repository-local graft' 1 'source-artifact: unsafe Git context: grafts' sh "$fixture/tools/build-source-artifact.sh" --require-tag v0.0.0-check "$fixture_commit" "$fixture_evidence" "$tmp/grafts"
rm "$fixture/.git/info/grafts"

global_git="$tmp/global-git"
mkdir -p "$global_git/home" "$global_git/xdg/git" "$global_git/template/info"
printf '%s\n' 'README.md export-ignore' >"$global_git/xdg/git/attributes"
printf '%s\n' 'README.md export-ignore' >"$global_git/template/info/attributes"
git config --file "$global_git/home/.gitconfig" init.templateDir "$global_git/template"
HOME="$global_git/home" XDG_CONFIG_HOME="$global_git/xdg" sh "$fixture/tools/build-source-artifact.sh" --require-tag v0.0.0-check "$fixture_commit" "$fixture_evidence" "$tmp/global-attributes"
tar -tzf "$tmp/global-attributes/yomihon-v0.0.0-check/yomihon-v0.0.0-check.tar.gz" | grep -qxF 'yomihon-0.0.0-check/README.md' || fail 'isolated archive honored ambient global export-ignore'

smudge_fixture="$tmp/smudge-repository"
git clone -q "$fixture" "$smudge_fixture"
git -C "$smudge_fixture" config user.name artifact-test
git -C "$smudge_fixture" config user.email artifact-test@example.invalid
printf '%s\n' 'tools/build-source-artifact.sh filter=release-control' >>"$smudge_fixture/.gitattributes"
printf '%s\n' 'Makefile filter=make-control' >>"$smudge_fixture/.gitattributes"
git -C "$smudge_fixture" add .gitattributes
git -C "$smudge_fixture" commit -qm 'declare local release-control filter surface'
smudge_commit=$(git -C "$smudge_fixture" rev-parse HEAD)
git -C "$smudge_fixture" tag -a v0.0.0-smudge -m v0.0.0-smudge "$smudge_commit"
git -C "$smudge_fixture" config filter.release-control.smudge "sed 's/^set -eu$/exit 99/'"
git -C "$smudge_fixture" config filter.release-control.clean "sed 's/^exit 99$/set -eu/'"
git -C "$smudge_fixture" config filter.make-control.smudge "sed 's/^# Prepare the exact tagged archive/# SMUDGED Prepare the exact tagged archive/'"
git -C "$smudge_fixture" config filter.make-control.clean "sed 's/^# SMUDGED Prepare the exact tagged archive/# Prepare the exact tagged archive/'"
git -C "$smudge_fixture" rm -q -- tools/build-source-artifact.sh Makefile
git -C "$smudge_fixture" checkout -q HEAD -- tools/build-source-artifact.sh Makefile
grep -qxF 'exit 99' "$smudge_fixture/tools/build-source-artifact.sh" || fail 'smudge fixture did not transform the working-tree builder'
grep -q '^# SMUDGED Prepare the exact tagged archive' "$smudge_fixture/Makefile" || fail 'smudge fixture did not transform the working-tree Makefile'
[ -z "$(git -C "$smudge_fixture" status --porcelain --untracked-files=all)" ] || fail 'smudge fixture is not Git-clean after reversible transformation'
expect_failure 'Make convenience rejects transformed working-tree Makefile' 2 'working-tree Makefile differs from SOURCE_COMMIT' \
	make -C "$smudge_fixture" RELEASE_VERSION=v0.0.0-smudge SOURCE_COMMIT="$smudge_commit" SOURCE_ARCHIVE="$tmp/make-smudge-candidate.tar.gz" source-archive-candidate
smudge_candidate="$tmp/smudge-candidate.tar.gz"
expect_success 'committed bootstrap ignores reversible worktree smudge' run_committed_bootstrap "$smudge_fixture" "$smudge_commit" \
	--prepare-archive v0.0.0-smudge "$smudge_commit" "$smudge_candidate"
smudge_digest=$(sha256_file "$smudge_candidate")
smudge_evidence="$tmp/smudge-review.md"
write_release_evidence "$smudge_evidence" "$smudge_commit" "$smudge_digest" "$smudge_fixture"
expect_success 'committed bootstrap assembles despite reversible worktree smudge' run_committed_bootstrap "$smudge_fixture" "$smudge_commit" \
	--assemble v0.0.0-smudge "$smudge_commit" "$smudge_evidence" "$tmp/smudge-output"
cmp "$smudge_candidate" "$tmp/smudge-output/yomihon-v0.0.0-smudge/yomihon-v0.0.0-smudge.tar.gz"

mkdir "$tmp/source-revalidation-bin"
cat >"$tmp/source-revalidation-bin/sh" <<'EOF'
#!/bin/sh
set -eu
"$REAL_SH" "$@"
if [ "${1:-}" = tools/check-source-artifact.sh ] && [ ! -e "$MUTATION_MARKER" ]; then
	printf '%s\n' 'in-flight release mutation' >>"$MUTATION_REPOSITORY/README.md"
	: >"$MUTATION_MARKER"
fi
EOF
chmod 0755 "$tmp/source-revalidation-bin/sh"
real_sh=$(command -v sh)
expect_failure 'in-flight release source mutation' 1 'source-artifact: release checkout is not clean' env REAL_SH="$real_sh" MUTATION_MARKER="$tmp/source-revalidation.marker" MUTATION_REPOSITORY="$fixture" PATH="$tmp/source-revalidation-bin:$PATH" "$real_sh" "$fixture/tools/build-source-artifact.sh" --require-tag v0.0.0-check "$fixture_commit" "$fixture_evidence" "$tmp/source-revalidation"
[ -e "$tmp/source-revalidation.marker" ] || fail 'in-flight release source mutation did not execute'
[ ! -e "$tmp/source-revalidation/yomihon-v0.0.0-check" ] || fail 'in-flight source mutation published a release destination'
GIT_NO_REPLACE_OBJECTS=1 git -C "$fixture" restore README.md

printf '%s\n' 'replacement-tree' >>"$fixture/README.md"
git -C "$fixture" add README.md
git -C "$fixture" commit -qm 'replacement target'
replacement_commit=$(git -C "$fixture" rev-parse HEAD)
git -C "$fixture" replace "$fixture_commit" "$replacement_commit"
git -C "$fixture" reset --hard "$fixture_commit" >/dev/null
expect_failure 'Git replacement source identity' 1 'source-artifact: unsafe Git context: replacement-refs' sh "$fixture/tools/build-source-artifact.sh" --require-tag v0.0.0-check "$fixture_commit" "$fixture_evidence" "$tmp/replacement-ref"
git -C "$fixture" replace -d "$fixture_commit" >/dev/null
GIT_NO_REPLACE_OBJECTS=1 git -C "$fixture" reset --hard "$fixture_commit" >/dev/null

attribute_fixture="$tmp/committed-attribute-repository"
git clone -q "$fixture" "$attribute_fixture"
git -C "$attribute_fixture" config user.name artifact-test
git -C "$attribute_fixture" config user.email artifact-test@example.invalid
printf '%s\n' 'README.md export-ignore' >>"$attribute_fixture/.gitattributes"
git -C "$attribute_fixture" add .gitattributes
git -C "$attribute_fixture" commit -qm 'add forbidden archive projection'
attribute_commit=$(git -C "$attribute_fixture" rev-parse HEAD)
git -C "$attribute_fixture" tag -a v0.0.0-export -m v0.0.0-export "$attribute_commit"
write_release_evidence "$fixture_evidence" "$attribute_commit" 1111111111111111111111111111111111111111111111111111111111111111 "$attribute_fixture"
expect_failure 'committed export-ignore attribute' 1 'source-artifact: export-ignore and export-subst attributes are not allowed' sh "$attribute_fixture/tools/build-source-artifact.sh" --require-tag v0.0.0-export "$attribute_commit" "$fixture_evidence" "$tmp/committed-attribute"

gitlink_fixture="$tmp/gitlink-repository"
git clone -q "$fixture" "$gitlink_fixture"
git -C "$gitlink_fixture" config user.name artifact-test
git -C "$gitlink_fixture" config user.email artifact-test@example.invalid
git -C "$gitlink_fixture" update-index --add --cacheinfo "160000,$fixture_commit,embedded-repository"
git -C "$gitlink_fixture" commit -qm 'add unsupported gitlink'
gitlink_commit=$(git -C "$gitlink_fixture" rev-parse HEAD)
git -C "$gitlink_fixture" tag -a v0.0.0-gitlink -m v0.0.0-gitlink "$gitlink_commit"
write_release_evidence "$fixture_evidence" "$gitlink_commit" 1111111111111111111111111111111111111111111111111111111111111111 "$gitlink_fixture"
expect_failure 'gitlink source tree' 1 'source-artifact: gitlinks are not supported by the complete-tree source artifact' sh "$gitlink_fixture/tools/build-source-artifact.sh" --require-tag v0.0.0-gitlink "$gitlink_commit" "$fixture_evidence" "$tmp/gitlink"

write_release_evidence "$fixture_evidence" "$fixture_commit" "$candidate_digest" "$fixture"

toolchain_fixture="$tmp/toolchain-repository"
git clone -q "$fixture" "$toolchain_fixture"
git -C "$toolchain_fixture" config user.name artifact-test
git -C "$toolchain_fixture" config user.email artifact-test@example.invalid
sed 's/^git-version:.*/git-version: git version 0.0.0/' "$toolchain_fixture/tools/source-artifact-toolchain.txt" >"$toolchain_fixture/tools/source-artifact-toolchain.txt.new"
mv "$toolchain_fixture/tools/source-artifact-toolchain.txt.new" "$toolchain_fixture/tools/source-artifact-toolchain.txt"
git -C "$toolchain_fixture" add tools/source-artifact-toolchain.txt
git -C "$toolchain_fixture" commit -qm 'pin unavailable Git fixture'
wrong_git_commit=$(git -C "$toolchain_fixture" rev-parse HEAD)
git -C "$toolchain_fixture" tag -a v0.0.0-wrong-git -m v0.0.0-wrong-git "$wrong_git_commit"
write_release_evidence "$fixture_evidence" "$wrong_git_commit" 1111111111111111111111111111111111111111111111111111111111111111 "$toolchain_fixture"
expect_failure 'pinned release Git mismatch' 1 'source-artifact: release requires git version 0.0.0' sh "$toolchain_fixture/tools/build-source-artifact.sh" --require-tag v0.0.0-wrong-git "$wrong_git_commit" "$fixture_evidence" "$tmp/wrong-git-toolchain"

sed -e "s/^git-version:.*/git-version: $(git version)/" -e 's/^gzip-version:.*/gzip-version: gzip 0.0.0/' "$toolchain_fixture/tools/source-artifact-toolchain.txt" >"$toolchain_fixture/tools/source-artifact-toolchain.txt.new"
mv "$toolchain_fixture/tools/source-artifact-toolchain.txt.new" "$toolchain_fixture/tools/source-artifact-toolchain.txt"
git -C "$toolchain_fixture" add tools/source-artifact-toolchain.txt
git -C "$toolchain_fixture" commit -qm 'pin unavailable gzip fixture'
wrong_gzip_commit=$(git -C "$toolchain_fixture" rev-parse HEAD)
git -C "$toolchain_fixture" tag -a v0.0.0-wrong-gzip -m v0.0.0-wrong-gzip "$wrong_gzip_commit"
write_release_evidence "$fixture_evidence" "$wrong_gzip_commit" 1111111111111111111111111111111111111111111111111111111111111111 "$toolchain_fixture"
expect_failure 'pinned release gzip mismatch' 1 'source-artifact: release requires gzip 0.0.0' sh "$toolchain_fixture/tools/build-source-artifact.sh" --require-tag v0.0.0-wrong-gzip "$wrong_gzip_commit" "$fixture_evidence" "$tmp/wrong-gzip-toolchain"

printf '\nrelease-dirty-test\n' >>"$fixture/README.md"
expect_failure 'dirty release checkout' 1 'source-artifact: release checkout is not clean' sh "$fixture/tools/build-source-artifact.sh" --require-tag v0.0.0-check "$fixture_commit" "$fixture_evidence" "$tmp/dirty"
git -C "$fixture" add README.md
git -C "$fixture" commit -qm 'make tag stale'
different_commit=$(git -C "$fixture" rev-parse HEAD)
write_release_evidence "$fixture_evidence" "$different_commit" 1111111111111111111111111111111111111111111111111111111111111111 "$fixture"
expect_failure 'tag and commit mismatch' 1 'source-artifact: tag v0.0.0-check does not identify' sh "$fixture/tools/build-source-artifact.sh" --require-tag v0.0.0-check "$different_commit" "$fixture_evidence" "$tmp/wrong-commit"

git -C "$fixture" rm -q PROJECT_PROFILE.md
git -C "$fixture" commit -qm 'remove required provenance input'
missing_blob_commit=$(git -C "$fixture" rev-parse HEAD)
expect_failure 'missing required commit blob' 1 'source-artifact: required commit blob is missing: PROJECT_PROFILE.md' sh "$fixture/tools/build-source-artifact.sh" v0.0.0-missing "$missing_blob_commit" "$fixture_evidence" "$tmp/missing-blob"

echo 'source-artifact-test: all contract mutations were rejected'

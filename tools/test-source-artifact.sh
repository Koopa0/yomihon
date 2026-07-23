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

# The exact, frozen bytes of the working-tree-Makefile mismatch diagnostic. The
# recipes read caller values only through the shell, never by letting Make expand
# them into recipe source, so this text never carries a caller value.
MAKEFILE_MISMATCH_DIAGNOSTIC='working-tree Makefile differs from SOURCE_COMMIT; the release tooling and its bootstrap arguments are the ones committed at SOURCE_COMMIT, not this working tree. Check out SOURCE_COMMIT and drive the release from its own committed Makefile and tools/source-artifact-bootstrap.sh.'

# A literal backtick, so the command-substitution payload below carries one
# without the test's own shell running it.
backtick='`'

# Each release recipe must read RELEASE_VERSION / SOURCE_COMMIT / SOURCE_ARCHIVE
# only as inert data. A caller value must never enter the recipe source Make
# expands, and must never run a command -- neither by breaking shell quoting nor
# by being evaluated as a Make function when Make exports the variable. This
# drives the smudge fixture (its working Makefile already differs from its
# committed one) so the guard fires, and checks four breakout shapes.
#
# Three shapes attack shell quoting: single quote, double quote, and backtick.
# The fourth attacks the Make layer: a $(shell ...) value would run while Make
# exports the variable, unless the recipe's $(value ...) freeze has already
# turned it into a literal string. (A plain $(...) that is not a Make function is
# swallowed by Make's own expansion, so $(shell ...) is the live form to test.)
expect_makefile_value_stays_data() {
	emvd_target=$1
	emvd_var=$2
	emvd_slug=$3
	# Pass 1 (behavioural): for every breakout shape the value must never execute
	# (the sentinel stays absent), the target still reaches its rejection path
	# (exit 2), and the diagnostic is byte-for-byte the frozen text with no caller
	# value inside it. This pass runs first so that re-introducing an unquoted
	# expansion is caught by the created sentinel rather than the dry-run check.
	for emvd_form in single-quote double-quote command-substitution make-function; do
		emvd_marker="MARK-$emvd_slug-$emvd_form"
		emvd_sentinel="$tmp/mkval-run-$emvd_slug-$emvd_form.sentinel"
		rm -f "$emvd_sentinel"
		case "$emvd_form" in
		single-quote) emvd_payload="$emvd_marker '; : > $emvd_sentinel; echo '" ;;
		double-quote) emvd_payload="$emvd_marker \"; : > $emvd_sentinel; echo \"" ;;
		command-substitution) emvd_payload="$emvd_marker ${backtick}: > $emvd_sentinel${backtick}" ;;
		make-function) emvd_payload="$emvd_marker \$(shell : > $emvd_sentinel)" ;;
		esac
		emvd_rv=v0.0.0-smudge
		emvd_sc="$smudge_commit"
		emvd_sa="$tmp/mkval-run-$emvd_slug-$emvd_form.out.tar.gz"
		case "$emvd_var" in
		RELEASE_VERSION) emvd_rv="$emvd_payload" ;;
		SOURCE_COMMIT) emvd_sc="$emvd_payload" ;;
		SOURCE_ARCHIVE) emvd_sa="$emvd_payload" ;;
		esac
		emvd_label="$emvd_target/$emvd_var/$emvd_form"
		emvd_log="$tmp/mkval-run-$emvd_slug-$emvd_form.log"
		emvd_status=0
		make -C "$smudge_fixture" "$emvd_target" \
			RELEASE_VERSION="$emvd_rv" SOURCE_COMMIT="$emvd_sc" SOURCE_ARCHIVE="$emvd_sa" \
			>"$emvd_log" 2>&1 || emvd_status=$?
		[ ! -e "$emvd_sentinel" ] || { cat "$emvd_log" >&2; fail "$emvd_label: the caller value executed and created the sentinel"; }
		[ "$emvd_status" -eq 2 ] || { cat "$emvd_log" >&2; fail "$emvd_label: make exited $emvd_status, want 2"; }
		emvd_diag=$(grep -F 'working-tree Makefile differs from SOURCE_COMMIT' "$emvd_log" || true)
		[ "$emvd_diag" = "$MAKEFILE_MISMATCH_DIAGNOSTIC" ] || { cat "$emvd_log" >&2; fail "$emvd_label: the mismatch diagnostic is missing or its bytes changed"; }
		if printf '%s\n' "$emvd_diag" | grep -Fq "$emvd_marker"; then
			cat "$emvd_log" >&2
			fail "$emvd_label: the diagnostic carried the caller value"
		fi
	done

	# Pass 2 (structural): the value must never enter the recipe source Make
	# expands. A dry run prints the recipe without running it, so the marker must
	# be absent from that printed source; the value stayed on the environment
	# channel. A dry run runs nothing, so it cannot create the sentinel.
	for emvd_form in single-quote double-quote command-substitution make-function; do
		emvd_marker="MARK-$emvd_slug-$emvd_form"
		emvd_sentinel="$tmp/mkval-dry-$emvd_slug-$emvd_form.sentinel"
		rm -f "$emvd_sentinel"
		case "$emvd_form" in
		single-quote) emvd_payload="$emvd_marker '; : > $emvd_sentinel; echo '" ;;
		double-quote) emvd_payload="$emvd_marker \"; : > $emvd_sentinel; echo \"" ;;
		command-substitution) emvd_payload="$emvd_marker ${backtick}: > $emvd_sentinel${backtick}" ;;
		make-function) emvd_payload="$emvd_marker \$(shell : > $emvd_sentinel)" ;;
		esac
		emvd_rv=v0.0.0-smudge
		emvd_sc="$smudge_commit"
		emvd_sa="$tmp/mkval-dry-$emvd_slug-$emvd_form.out.tar.gz"
		case "$emvd_var" in
		RELEASE_VERSION) emvd_rv="$emvd_payload" ;;
		SOURCE_COMMIT) emvd_sc="$emvd_payload" ;;
		SOURCE_ARCHIVE) emvd_sa="$emvd_payload" ;;
		esac
		emvd_label="$emvd_target/$emvd_var/$emvd_form"
		emvd_dry="$tmp/mkval-dry-$emvd_slug-$emvd_form.dry"
		make -n -C "$smudge_fixture" "$emvd_target" \
			RELEASE_VERSION="$emvd_rv" SOURCE_COMMIT="$emvd_sc" SOURCE_ARCHIVE="$emvd_sa" \
			>"$emvd_dry" 2>&1 || true
		if grep -Fq "$emvd_marker" "$emvd_dry"; then
			cat "$emvd_dry" >&2
			fail "$emvd_label: the caller value entered the recipe source Make expands"
		fi
		[ ! -e "$emvd_sentinel" ] || fail "$emvd_label: the dry run created the sentinel"
	done
}

commit=$(git rev-parse HEAD)

for invalid in v1 v01.2.3 v1.02.3 v1.2.03 v1.2.3-01 v1.2.3-alpha..1 v1.2.3- v1.2.3+; do
	expect_failure "invalid semantic version $invalid" 2 'source-artifact: invalid semantic version:' \
		sh tools/build-source-artifact.sh "$invalid" "$commit" "$tmp/invalid"
done

GZIP=-9 GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=tar.umask GIT_CONFIG_VALUE_0=0077 \
	sh tools/build-source-artifact.sh v0.0.0-check "$commit" "$tmp/first"
sh tools/build-source-artifact.sh v0.0.0-check "$commit" "$tmp/second"
diff -ru "$tmp/first" "$tmp/second"
sh tools/build-source-artifact.sh v1.2.3-alpha.1+build.7 "$commit" "$tmp/valid-semver"

pristine="$tmp/second/yomihon-v0.0.0-check"
expect_failure 'fixture checker requires explicit opt-in' 1 'source-artifact-check: verification fixture requires --allow-fixture' sh tools/check-source-artifact.sh v0.0.0-check "$commit" "$pristine"
sh tools/check-source-artifact.sh --allow-fixture v0.0.0-check "$commit" "$pristine"
expect_failure overwrite 1 'source-artifact: refusing to overwrite' sh tools/build-source-artifact.sh v0.0.0-check "$commit" "$tmp/second"

mkdir "$tmp/locked-output"
mkdir "$tmp/locked-output/.yomihon-v0.0.0-check.lock"
expect_failure 'publication lock' 1 'source-artifact: publication lock is already held' sh tools/build-source-artifact.sh v0.0.0-check "$commit" "$tmp/locked-output"

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
expect_failure 'destination appearance during publication' 1 'source-artifact: destination changed during publication' env REAL_MV="$real_mv" PATH="$tmp/race-bin:$PATH" sh tools/build-source-artifact.sh v0.0.0-race "$commit" "$tmp/race-output"
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
	printf '%s\n' 'post-rename mutation' >>"$last/yomihon-v0.0.0-postcheck.provenance"
	: >"$POSTCHECK_MARKER"
	;;
esac
exec "$REAL_SH" "$@"
EOF
chmod 0755 "$tmp/postcheck-bin/sh"
postcheck_marker="$tmp/postcheck.marker"
expect_failure 'post-rename validation' 1 'source-artifact: published bundle failed validation and was quarantined:' env REAL_SH="$(command -v sh)" POSTCHECK_MARKER="$postcheck_marker" PATH="$tmp/postcheck-bin:$PATH" sh tools/build-source-artifact.sh v0.0.0-postcheck "$commit" "$tmp/postcheck-output"
[ -e "$postcheck_marker" ] || fail 'post-rename validation mutation did not execute'
[ ! -e "$tmp/postcheck-output/yomihon-v0.0.0-postcheck" ] || fail 'failed post-rename validation left the canonical destination occupied'
[ "$(find "$tmp/postcheck-output" -maxdepth 1 -type d -name '.yomihon-v0.0.0-postcheck.failed.*' | wc -l | tr -d ' ')" -eq 1 ] || fail 'failed post-rename validation did not retain exactly one quarantine'

mkdir "$tmp/outside"
ln -s "$tmp/outside" "$tmp/linked-output"
expect_failure 'linked output parent' 1 'source-artifact: output parent is not a real directory' sh tools/build-source-artifact.sh v0.0.0-check "$commit" "$tmp/linked-output"

mkdir "$tmp/dangling-parent"
ln -s "$tmp/nowhere" "$tmp/dangling-parent/yomihon-v0.0.0-check"
expect_failure 'dangling destination link' 1 'source-artifact: refusing to overwrite' sh tools/build-source-artifact.sh v0.0.0-check "$commit" "$tmp/dangling-parent"

cp -R "$pristine" "$tmp/archive-mutation"
printf 'tamper' >>"$tmp/archive-mutation/yomihon-v0.0.0-check.tar.gz"
expect_failure 'archive checksum mutation' 1 'source-artifact-check: bundle checksum mismatch' sh tools/check-source-artifact.sh --allow-fixture v0.0.0-check "$commit" "$tmp/archive-mutation"

cp -R "$pristine" "$tmp/manifest-mutation"
sed '/\.provenance$/d' "$tmp/manifest-mutation/yomihon-v0.0.0-check-SHA256SUMS" >"$tmp/manifest-mutation/manifest.new"
mv "$tmp/manifest-mutation/manifest.new" "$tmp/manifest-mutation/yomihon-v0.0.0-check-SHA256SUMS"
expect_failure 'manifest coverage mutation' 1 'source-artifact-check: manifest must contain exactly two rows' sh tools/check-source-artifact.sh --allow-fixture v0.0.0-check "$commit" "$tmp/manifest-mutation"

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

expect_provenance_digest_failure source-artifact-toolchain-sha256 source-artifact-toolchain 'source-artifact-check: source artifact toolchain digest mismatch'
expect_provenance_digest_failure source-artifact-bootstrap-sha256 source-artifact-bootstrap 'source-artifact-check: source artifact bootstrap digest mismatch'
expect_provenance_digest_failure go-mod-sha256 go-mod 'source-artifact-check: go.mod digest mismatch'
expect_provenance_digest_failure go-sum-sha256 go-sum 'source-artifact-check: go.sum digest mismatch'
expect_provenance_digest_failure frontend-lock-sha256 frontend-lock 'source-artifact-check: frontend lock digest mismatch'
expect_provenance_digest_failure bakeoff-go-mod-sha256 bakeoff-go-mod 'source-artifact-check: bake-off go.mod digest mismatch'
expect_provenance_digest_failure bakeoff-go-sum-sha256 bakeoff-go-sum 'source-artifact-check: bake-off go.sum digest mismatch'
expect_provenance_digest_failure ci-workflow-sha256 ci-workflow 'source-artifact-check: CI workflow digest mismatch'
expect_provenance_digest_failure archive-sha256 archive 'source-artifact-check: provenance archive digest mismatch'

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

expect_failure 'missing release tag' 1 'source-artifact: v0.0.0-check must be an annotated tag' sh "$fixture/tools/build-source-artifact.sh" --require-tag v0.0.0-check "$fixture_commit" "$tmp/no-tag"
git -C "$fixture" tag v0.0.0-check "$fixture_commit"
expect_failure 'lightweight release tag' 1 'source-artifact: v0.0.0-check must be an annotated tag' sh "$fixture/tools/build-source-artifact.sh" --require-tag v0.0.0-check "$fixture_commit" "$tmp/lightweight"
git -C "$fixture" tag -d v0.0.0-check >/dev/null

git -C "$fixture" tag -a v0.0.0-other -m v0.0.0-other "$fixture_commit"
git -C "$fixture" update-ref refs/tags/v0.0.0-check "$(git -C "$fixture" rev-parse refs/tags/v0.0.0-other)"
expect_failure 'annotated tag internal name mismatch' 1 'source-artifact: annotated tag object name does not match v0.0.0-check' sh "$fixture/tools/build-source-artifact.sh" --require-tag v0.0.0-check "$fixture_commit" "$tmp/wrong-tag-name"
git -C "$fixture" update-ref -d refs/tags/v0.0.0-check
git -C "$fixture" tag -d v0.0.0-other >/dev/null

git -C "$fixture" tag -a v0.0.0-check -m v0.0.0-check "$fixture_commit"

candidate_archive="$tmp/prepared/yomihon-v0.0.0-check.tar.gz"
mkdir -p "${candidate_archive%/*}"
expect_success 'prepare exact tagged source archive' run_committed_bootstrap "$fixture" "$fixture_commit" \
	--prepare-archive v0.0.0-check "$fixture_commit" "$candidate_archive"

expect_success 'assemble tagged source bundle' run_committed_bootstrap "$fixture" "$fixture_commit" \
	--assemble v0.0.0-check "$fixture_commit" "$tmp/tagged"
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
expect_failure 'repository-local archive attributes' 1 'source-artifact: unsafe Git context: repository-local-attributes' sh "$fixture/tools/build-source-artifact.sh" --require-tag v0.0.0-check "$fixture_commit" "$tmp/local-attributes"
rm "$fixture/.git/info/attributes"

printf '%s\n' "$fixture_commit" >"$fixture/.git/info/grafts"
expect_failure 'repository-local graft' 1 'source-artifact: unsafe Git context: grafts' sh "$fixture/tools/build-source-artifact.sh" --require-tag v0.0.0-check "$fixture_commit" "$tmp/grafts"
rm "$fixture/.git/info/grafts"

global_git="$tmp/global-git"
mkdir -p "$global_git/home" "$global_git/xdg/git" "$global_git/template/info"
printf '%s\n' 'README.md export-ignore' >"$global_git/xdg/git/attributes"
printf '%s\n' 'README.md export-ignore' >"$global_git/template/info/attributes"
git config --file "$global_git/home/.gitconfig" init.templateDir "$global_git/template"
HOME="$global_git/home" XDG_CONFIG_HOME="$global_git/xdg" sh "$fixture/tools/build-source-artifact.sh" --require-tag v0.0.0-check "$fixture_commit" "$tmp/global-attributes"
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
git -C "$smudge_fixture" config filter.make-control.smudge "sed 's/^# Prepare the exact tagged source archive/# SMUDGED Prepare the exact tagged source archive/'"
git -C "$smudge_fixture" config filter.make-control.clean "sed 's/^# SMUDGED Prepare the exact tagged source archive/# Prepare the exact tagged source archive/'"
git -C "$smudge_fixture" rm -q -- tools/build-source-artifact.sh Makefile
git -C "$smudge_fixture" checkout -q HEAD -- tools/build-source-artifact.sh Makefile
grep -qxF 'exit 99' "$smudge_fixture/tools/build-source-artifact.sh" || fail 'smudge fixture did not transform the working-tree builder'
grep -q '^# SMUDGED Prepare the exact tagged source archive' "$smudge_fixture/Makefile" || fail 'smudge fixture did not transform the working-tree Makefile'
[ -z "$(git -C "$smudge_fixture" status --porcelain --untracked-files=all)" ] || fail 'smudge fixture is not Git-clean after reversible transformation'
expect_failure 'Make convenience rejects transformed working-tree Makefile' 2 'working-tree Makefile differs from SOURCE_COMMIT' \
	make -C "$smudge_fixture" RELEASE_VERSION=v0.0.0-smudge SOURCE_COMMIT="$smudge_commit" SOURCE_ARCHIVE="$tmp/make-smudge-candidate.tar.gz" source-archive-candidate
smudge_candidate="$tmp/smudge-candidate.tar.gz"
expect_success 'committed bootstrap ignores reversible worktree smudge' run_committed_bootstrap "$smudge_fixture" "$smudge_commit" \
	--prepare-archive v0.0.0-smudge "$smudge_commit" "$smudge_candidate"
expect_success 'committed bootstrap assembles despite reversible worktree smudge' run_committed_bootstrap "$smudge_fixture" "$smudge_commit" \
	--assemble v0.0.0-smudge "$smudge_commit" "$tmp/smudge-output"
cmp "$smudge_candidate" "$tmp/smudge-output/yomihon-v0.0.0-smudge/yomihon-v0.0.0-smudge.tar.gz"

# The benign path still delivers the actual caller values through the data
# channel. A fresh clone keeps this run's own dist/ output from disturbing later
# tests on the shared fixture.
benign_repo="$tmp/makefile-benign-repository"
git clone -q "$fixture" "$benign_repo"
make -C "$benign_repo" source-archive-candidate \
	RELEASE_VERSION=v0.0.0-check SOURCE_COMMIT="$fixture_commit" SOURCE_ARCHIVE="$tmp/benign-candidate.tar.gz" \
	>"$tmp/benign-candidate.log" 2>&1 || { cat "$tmp/benign-candidate.log" >&2; fail 'benign source-archive-candidate rejected a valid request'; }
[ -f "$tmp/benign-candidate.tar.gz" ] || fail 'benign source-archive-candidate did not honor the passed SOURCE_ARCHIVE'
make -C "$benign_repo" source-artifact \
	RELEASE_VERSION=v0.0.0-check SOURCE_COMMIT="$fixture_commit" \
	>"$tmp/benign-artifact.log" 2>&1 || { cat "$tmp/benign-artifact.log" >&2; fail 'benign source-artifact rejected a valid request'; }
[ -d "$benign_repo/dist/yomihon-v0.0.0-check" ] || fail 'benign source-artifact did not honor the passed RELEASE_VERSION and SOURCE_COMMIT'

# The bootstrap SOURCE_ARCHIVE argument is only reached on the success path. A
# hostile output path there stays inert data too: the build cannot use it, but no
# injected command runs. This runs before the rejection-path matrix so that a
# recipe that mis-expands SOURCE_ARCHIVE is caught by the created sentinel.
sa_success_sentinel="$tmp/makefile-success-source-archive.sentinel"
rm -f "$sa_success_sentinel"
sa_hostile_out="$tmp/makefile-success-${backtick}: > $sa_success_sentinel${backtick}.tar.gz"
make -C "$benign_repo" source-archive-candidate \
	RELEASE_VERSION=v0.0.0-check SOURCE_COMMIT="$fixture_commit" SOURCE_ARCHIVE="$sa_hostile_out" \
	>"$tmp/makefile-success-source-archive.log" 2>&1 || true
[ ! -e "$sa_success_sentinel" ] || { cat "$tmp/makefile-success-source-archive.log" >&2; fail 'the bootstrap SOURCE_ARCHIVE argument executed a hostile output path'; }

expect_makefile_value_stays_data source-archive-candidate RELEASE_VERSION candidate-release-version
expect_makefile_value_stays_data source-archive-candidate SOURCE_COMMIT candidate-source-commit
expect_makefile_value_stays_data source-archive-candidate SOURCE_ARCHIVE candidate-source-archive
expect_makefile_value_stays_data source-artifact RELEASE_VERSION artifact-release-version
expect_makefile_value_stays_data source-artifact SOURCE_COMMIT artifact-source-commit

# Structural lock: neither release recipe may Make-expand a caller-controlled
# value into shell source, in either the $(VAR) or the equivalent ${VAR} form
# (Make treats both identically). The escaped shell form $${VAR} is safe, so the
# escaped dollars are stripped before the scan; a single leading dollar before
# ( or { is the hazard. Scope strictly to each recipe body so the default-variable
# definitions above the targets are never implicated.
for makefile_recipe in source-archive-candidate source-artifact; do
	recipe_body=$(awk -v target="$makefile_recipe:" '
		$0 == target { inside = 1; next }
		inside && !/^\t/ { inside = 0 }
		inside { print }
	' Makefile)
	[ -n "$recipe_body" ] || fail "could not extract the $makefile_recipe recipe body"
	if printf '%s\n' "$recipe_body" | sed 's/\$\$//g' | grep -Eq '[$][({](RELEASE_VERSION|SOURCE_COMMIT|SOURCE_ARCHIVE)[)}]'; then
		printf '%s\n' "$recipe_body" >&2
		fail "$makefile_recipe recipe Make-expands a caller-controlled value into shell source"
	fi
done

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
expect_failure 'in-flight release source mutation' 1 'source-artifact: release checkout is not clean' env REAL_SH="$real_sh" MUTATION_MARKER="$tmp/source-revalidation.marker" MUTATION_REPOSITORY="$fixture" PATH="$tmp/source-revalidation-bin:$PATH" "$real_sh" "$fixture/tools/build-source-artifact.sh" --require-tag v0.0.0-check "$fixture_commit" "$tmp/source-revalidation"
[ -e "$tmp/source-revalidation.marker" ] || fail 'in-flight release source mutation did not execute'
[ ! -e "$tmp/source-revalidation/yomihon-v0.0.0-check" ] || fail 'in-flight source mutation published a release destination'
GIT_NO_REPLACE_OBJECTS=1 git -C "$fixture" restore README.md

printf '%s\n' 'replacement-tree' >>"$fixture/README.md"
git -C "$fixture" add README.md
git -C "$fixture" commit -qm 'replacement target'
replacement_commit=$(git -C "$fixture" rev-parse HEAD)
git -C "$fixture" replace "$fixture_commit" "$replacement_commit"
git -C "$fixture" reset --hard "$fixture_commit" >/dev/null
expect_failure 'Git replacement source identity' 1 'source-artifact: unsafe Git context: replacement-refs' sh "$fixture/tools/build-source-artifact.sh" --require-tag v0.0.0-check "$fixture_commit" "$tmp/replacement-ref"
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
expect_failure 'committed export-ignore attribute' 1 'source-artifact: export-ignore and export-subst attributes are not allowed' sh "$attribute_fixture/tools/build-source-artifact.sh" --require-tag v0.0.0-export "$attribute_commit" "$tmp/committed-attribute"

gitlink_fixture="$tmp/gitlink-repository"
git clone -q "$fixture" "$gitlink_fixture"
git -C "$gitlink_fixture" config user.name artifact-test
git -C "$gitlink_fixture" config user.email artifact-test@example.invalid
git -C "$gitlink_fixture" update-index --add --cacheinfo "160000,$fixture_commit,embedded-repository"
git -C "$gitlink_fixture" commit -qm 'add unsupported gitlink'
gitlink_commit=$(git -C "$gitlink_fixture" rev-parse HEAD)
git -C "$gitlink_fixture" tag -a v0.0.0-gitlink -m v0.0.0-gitlink "$gitlink_commit"
expect_failure 'gitlink source tree' 1 'source-artifact: gitlinks are not supported by the complete-tree source artifact' sh "$gitlink_fixture/tools/build-source-artifact.sh" --require-tag v0.0.0-gitlink "$gitlink_commit" "$tmp/gitlink"

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
expect_failure 'pinned release Git mismatch' 1 'source-artifact: release requires git version 0.0.0' sh "$toolchain_fixture/tools/build-source-artifact.sh" --require-tag v0.0.0-wrong-git "$wrong_git_commit" "$tmp/wrong-git-toolchain"

sed -e "s/^git-version:.*/git-version: $(git version)/" -e 's/^gzip-version:.*/gzip-version: gzip 0.0.0/' "$toolchain_fixture/tools/source-artifact-toolchain.txt" >"$toolchain_fixture/tools/source-artifact-toolchain.txt.new"
mv "$toolchain_fixture/tools/source-artifact-toolchain.txt.new" "$toolchain_fixture/tools/source-artifact-toolchain.txt"
git -C "$toolchain_fixture" add tools/source-artifact-toolchain.txt
git -C "$toolchain_fixture" commit -qm 'pin unavailable gzip fixture'
wrong_gzip_commit=$(git -C "$toolchain_fixture" rev-parse HEAD)
git -C "$toolchain_fixture" tag -a v0.0.0-wrong-gzip -m v0.0.0-wrong-gzip "$wrong_gzip_commit"
expect_failure 'pinned release gzip mismatch' 1 'source-artifact: release requires gzip 0.0.0' sh "$toolchain_fixture/tools/build-source-artifact.sh" --require-tag v0.0.0-wrong-gzip "$wrong_gzip_commit" "$tmp/wrong-gzip-toolchain"

printf '\nrelease-dirty-test\n' >>"$fixture/README.md"
expect_failure 'dirty release checkout' 1 'source-artifact: release checkout is not clean' sh "$fixture/tools/build-source-artifact.sh" --require-tag v0.0.0-check "$fixture_commit" "$tmp/dirty"
git -C "$fixture" add README.md
git -C "$fixture" commit -qm 'make tag stale'
different_commit=$(git -C "$fixture" rev-parse HEAD)
expect_failure 'tag and commit mismatch' 1 'source-artifact: tag v0.0.0-check does not identify' sh "$fixture/tools/build-source-artifact.sh" --require-tag v0.0.0-check "$different_commit" "$tmp/wrong-commit"

git -C "$fixture" rm -q go.sum
git -C "$fixture" commit -qm 'remove required provenance input'
missing_blob_commit=$(git -C "$fixture" rev-parse HEAD)
expect_failure 'missing required commit blob' 1 'source-artifact: required commit blob is missing: go.sum' sh "$fixture/tools/build-source-artifact.sh" v0.0.0-missing "$missing_blob_commit" "$tmp/missing-blob"

echo 'source-artifact-test: all contract mutations were rejected'

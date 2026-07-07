package judge

// The differential runner. For each seeded vault it drives both engines over
// check, coverage, and exists, compares the output after the known departures
// are normalized away, and compares the gate exit codes; a surviving difference
// is a real divergence, so it preserves the vault, prints the seed and the
// first differing hunk, and stops. A run also proves the generator earned its
// keep: it aggregates which rule surfaces the vaults actually exercised and
// fails if any was never reached, guarding against a generator that fuzzes hard
// but touches nothing.
//
// The whole file is opt-in. It runs only when a reference binary is named, so
// it never runs in continuous integration and dies with a plain file removal
// when the reference is retired. The generator's own coverage is proven
// separately, without the reference, by the rule-reach check below.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/koopa0/kurodo/internal/schema"
)

// requiredRules is every rule_id a run must reach at least once, the guard
// against a generator that produces bytes but no findings.
var requiredRules = []string{
	"schema.enum", "schema.required", "schema.unknown_key", "schema.slug",
	"schema.domain_folder", "schema.legacy_tag", "schema.provenance", "schema.frontmatter",
	"link.broken", "link.title_not_alias", "collision.alias", "provenance.unresolved",
	"map.disk_mismatch", "map.disk_unlisted", "link.broken.path",
}

// TestJudgeDifferentialFuzz drives the seeded vaults through both engines and
// fails on the first surviving difference. It is gated on the reference binary
// and skips cleanly without it.
func TestJudgeDifferentialFuzz(t *testing.T) {
	tool := referenceTool()
	if tool == "" {
		t.Skip("set KURODO_REFERENCE_BIN to the conformance reference binary to run the differential")
	}
	rounds := envInt(t, "KURODO_DIFF_ROUNDS", focusCount)
	if rounds <= 0 {
		// A run of zero or fewer rounds exercises nothing yet reports success;
		// that vacuous pass is as silent a misconfiguration as a bad value, so
		// it stops the run rather than proceeding with no coverage.
		t.Fatalf("KURODO_DIFF_ROUNDS=%d: a run needs a positive number of rounds", rounds)
	}
	base := int64(envInt(t, "KURODO_DIFF_SEED_BASE", 0))

	reached := map[string]int{}
	cats := map[string]int{}
	for i := range rounds {
		seed := base + int64(i)
		dir := t.TempDir()
		man := writeVault(t, dir, seed)
		for k, v := range man.categories {
			cats[k] += v
		}

		runCheckRound(t, tool, dir, seed, &man, reached)
		runCoverageRound(t, tool, dir, seed, &man)
		runExistsRound(t, tool, dir, seed, &man)
	}

	logDistribution(t, rounds, reached, cats)
	if rounds >= focusCount {
		assertRuleReach(t, reached, cats["noFrontmatter"])
		assertInfluenceReach(t, cats)
	} else {
		t.Logf("reach check skipped: %d rounds is fewer than the %d needed to guarantee every surface", rounds, focusCount)
	}
}

// logDistribution reports how many times each rule surface and each finding-less
// planted construct was reached across a run, so the operator can see the
// fuzzing landed where it was aimed.
func logDistribution(t *testing.T, rounds int, reached, cats map[string]int) {
	t.Helper()
	var b strings.Builder
	fmt.Fprintf(&b, "rule reach over %d rounds:\n", rounds)
	for _, r := range requiredRules {
		fmt.Fprintf(&b, "  %-24s %d\n", r, reached[r])
	}
	fmt.Fprintf(&b, "  %-24s %d\n", "(no-frontmatter plants)", cats["noFrontmatter"])
	fmt.Fprintf(&b, "  %-24s %d\n", "(journal-mount plants)", cats["diaryMount"])
	fmt.Fprintf(&b, "  %-24s %d", "(journal-planned plants)", cats["diaryPlanned"])
	t.Log(b.String())
}

// assertInfluenceReach fails when the generator no longer plants both journal-
// influence constructs. Neither adds a rule_id the reach tally would catch — a
// journal mount produces no finding, and a journal-planned link is an ordinary
// broken link — so the manifest counters are what prove the constructs still
// appear, and with them the divergence the harness is meant to exercise.
func assertInfluenceReach(t *testing.T, cats map[string]int) {
	t.Helper()
	for _, cat := range []string{"diaryMount", "diaryPlanned"} {
		if cats[cat] == 0 {
			t.Errorf("the generator never planted the %q construct; its journal-influence divergence would go unexercised", cat)
		}
	}
}

// runCheckRound compares the check JSONL after the reference lines this engine
// omits are dropped, then compares the gate exit codes on two error-severity
// gates, which never touch the journal and so need no normalization.
func runCheckRound(t *testing.T, tool, dir string, seed int64, man *genManifest, reached map[string]int) {
	t.Helper()
	refJSONL, refErrExit := refRun(t, tool, "check", "--root", dir, "--format", "json", "--deny", "error")
	gotJSONL, gotErrExit, err := RunCheck(&CheckOptions{Root: dir, Format: FormatJSON, Deny: []string{"error"}})
	if err != nil {
		t.Fatalf("RunCheck(seed=%d): %v", seed, err)
	}
	normRef := normalizeCheckJSONL(refJSONL, man)
	if !bytes.Equal(normRef, gotJSONL) {
		failDivergence(t, "check json", seed, gotJSONL, normRef)
	}
	tallyRules(gotJSONL, reached)

	if refErrExit != gotErrExit {
		failExit(t, "check --deny error exit", seed, gotErrExit, refErrExit)
	}
	_, refReqExit := refRun(t, tool, "check", "--root", dir, "--format", "json", "--deny", "schema.required")
	_, gotReqExit, err := RunCheck(&CheckOptions{Root: dir, Format: FormatJSON, Deny: []string{"schema.required"}})
	if err != nil {
		t.Fatalf("RunCheck(seed=%d, deny schema.required): %v", seed, err)
	}
	if refReqExit != gotReqExit {
		failExit(t, "check --deny schema.required exit", seed, gotReqExit, refReqExit)
	}
	runRenderedReports(t, tool, dir, seed, man)
}

// runRenderedReports compares the human and markdown reports. They aggregate the
// findings into scoreboards and folded groups, which cannot be normalized after
// the fact, so they are compared only when the vault carries no finding this
// engine drops — then the two views are directly comparable. The markdown is
// compared after its two-line tool-identity preamble, which names this engine
// rather than reproducing the reference tool's name in a note this engine files.
func runRenderedReports(t *testing.T, tool, dir string, seed int64, man *genManifest) {
	t.Helper()
	if man.hasDiary || len(man.commentPathRefs) > 0 {
		return
	}
	refMd, _ := refRun(t, tool, "check", "--root", dir, "--format", "md")
	gotMd, _, err := RunCheck(&CheckOptions{Root: dir, Format: FormatMarkdown})
	if err != nil {
		t.Fatalf("RunCheck(seed=%d, md): %v", seed, err)
	}
	if !bytes.Equal(reportBody(gotMd), reportBody(refMd)) {
		failDivergence(t, "check markdown body", seed, reportBody(gotMd), reportBody(refMd))
	}
	refHuman, _ := refRun(t, tool, "check", "--root", dir, "--format", "human")
	gotHuman, _, err := RunCheck(&CheckOptions{Root: dir, Format: FormatHuman})
	if err != nil {
		t.Fatalf("RunCheck(seed=%d, human): %v", seed, err)
	}
	if !bytes.Equal(refHuman, gotHuman) {
		failDivergence(t, "check human", seed, gotHuman, refHuman)
	}
}

// runCoverageRound compares coverage. A vault free of the journal and of an
// empty-domain concept is compared byte-for-byte; otherwise the reference
// report is normalized and compared structurally.
func runCoverageRound(t *testing.T, tool, dir string, seed int64, man *genManifest) {
	t.Helper()
	refCov, _ := refRun(t, tool, "coverage", "--root", dir, "--format", "json")
	gotCov, _, err := RunCoverage(&CoverageOptions{Root: dir, Format: FormatJSON})
	if err != nil {
		t.Fatalf("RunCoverage(seed=%d): %v", seed, err)
	}
	if !man.hasDiary && man.emptyDomainConcepts == 0 {
		if !bytes.Equal(refCov, gotCov) {
			failDivergence(t, "coverage json", seed, gotCov, refCov)
		}
		return
	}
	var refParsed, gotParsed Coverage
	mustJSON(t, refCov, &refParsed, seed, "reference coverage")
	mustJSON(t, gotCov, &gotParsed, seed, "coverage")
	norm := normalizeCoverage(&refParsed, man)
	if diff := cmp.Diff(gotParsed, norm, cmpopts.EquateEmpty()); diff != "" {
		failStructured(t, "coverage json", seed, diff)
	}
}

// runExistsRound queries names that should hit, one that must miss, and the
// journal names this engine must hide. A public or missing name is compared
// byte-for-byte with its exit code; a journal name is compared against the
// normalized report and the exit code that report implies. A query is never
// empty, so the empty-query divergence is not exercised.
func runExistsRound(t *testing.T, tool, dir string, seed int64, man *genManifest) {
	t.Helper()
	for _, name := range firstN(man.publicNames, 2) {
		refEx, refExit := refRun(t, tool, "exists", "--root", dir, "--format", "json", name)
		gotEx, gotExit, err := RunExists(&ExistsOptions{Root: dir, Name: name, Format: FormatJSON})
		if err != nil {
			t.Fatalf("RunExists(seed=%d, %q): %v", seed, name, err)
		}
		if !bytes.Equal(refEx, gotEx) {
			failDivergence(t, "exists "+name, seed, gotEx, refEx)
		}
		if refExit != gotExit {
			failExit(t, "exists "+name+" exit", seed, gotExit, refExit)
		}
	}

	miss := fmt.Sprintf("absent-name-%d-zzz", seed)
	refEx, refExit := refRun(t, tool, "exists", "--root", dir, "--format", "json", miss)
	gotEx, gotExit, err := RunExists(&ExistsOptions{Root: dir, Name: miss, Format: FormatJSON})
	if err != nil {
		t.Fatalf("RunExists(seed=%d, miss): %v", seed, err)
	}
	if !bytes.Equal(refEx, gotEx) {
		failDivergence(t, "exists miss", seed, gotEx, refEx)
	}
	if refExit != gotExit {
		failExit(t, "exists miss exit", seed, gotExit, refExit)
	}

	for _, name := range man.diaryNames {
		refRaw, _ := refRun(t, tool, "exists", "--root", dir, "--format", "json", name)
		gotEx, gotExit, err := RunExists(&ExistsOptions{Root: dir, Name: name, Format: FormatJSON})
		if err != nil {
			t.Fatalf("RunExists(seed=%d, diary %q): %v", seed, name, err)
		}
		var refParsed, gotParsed existsReport
		mustJSON(t, refRaw, &refParsed, seed, "reference exists")
		mustJSON(t, gotEx, &gotParsed, seed, "exists")
		norm := normalizeExists(refParsed)
		if diff := cmp.Diff(gotParsed, norm, cmpopts.EquateEmpty()); diff != "" {
			failStructured(t, "exists (journal) "+name, seed, diff)
		}
		if existsExit(norm) != gotExit {
			failExit(t, "exists (journal) "+name+" exit", seed, gotExit, existsExit(norm))
		}
	}
}

// TestDiffFuzzRuleReach proves the generator reaches every rule surface without
// the reference binary, so the guarantee runs on every ordinary test pass. It
// walks one guaranteed vault per focus category and asserts this engine's own
// output touched every required rule and that a no-frontmatter note was planted.
func TestDiffFuzzRuleReach(t *testing.T) {
	t.Parallel()
	reached := map[string]int{}
	cats := map[string]int{}
	for seed := range int64(focusCount) {
		dir := t.TempDir()
		man := writeVault(t, dir, seed)
		for k, v := range man.categories {
			cats[k] += v
		}
		findings, err := Check(dir)
		if err != nil {
			t.Fatalf("Check(seed=%d): %v", seed, err)
		}
		for i := range findings {
			reached[findings[i].RuleID]++
		}
	}
	assertRuleReach(t, reached, cats["noFrontmatter"])
	assertInfluenceReach(t, cats)
}

// assertRuleReach fails when a required rule was never reached or when no
// no-frontmatter note was planted, printing the full distribution so a
// generator regression is diagnosable at a glance.
func assertRuleReach(t *testing.T, reached map[string]int, noFrontmatter int) {
	t.Helper()
	var missing []string
	for _, r := range requiredRules {
		if reached[r] == 0 {
			missing = append(missing, r)
		}
	}
	if noFrontmatter == 0 {
		missing = append(missing, "(no-frontmatter classification)")
	}
	if len(missing) == 0 {
		return
	}
	var b strings.Builder
	b.WriteString("rule reach incomplete; the generator no longer exercises every surface\n")
	fmt.Fprintf(&b, "missing: %s\n", strings.Join(missing, ", "))
	b.WriteString("distribution:\n")
	for _, r := range requiredRules {
		fmt.Fprintf(&b, "  %-24s %d\n", r, reached[r])
	}
	fmt.Fprintf(&b, "  %-24s %d\n", "(no-frontmatter plants)", noFrontmatter)
	t.Error(b.String())
}

// refRun runs the reference binary once and returns its stdout and process exit
// code. A nonzero exit is expected — an existence miss or a gate hit still
// prints valid output — so only a failure to run the tool at all is fatal.
func refRun(t *testing.T, tool string, args ...string) (stdout []byte, exitCode int) {
	t.Helper()
	// #nosec G204 -- tool is resolved from a trusted environment variable set by the operator, not from user input
	out, err := exec.CommandContext(t.Context(), tool, args...).Output()
	if err == nil {
		return out, 0
	}
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		return out, exit.ExitCode()
	}
	t.Fatalf("run reference %v: %v", args, err)
	return nil, -1
}

// tallyRules counts each finding's rule_id from a JSONL stream into reached.
func tallyRules(jsonl []byte, reached map[string]int) {
	for _, line := range strings.Split(string(jsonl), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var f struct {
			RuleID string `json:"rule_id"`
		}
		if json.Unmarshal([]byte(line), &f) == nil && f.RuleID != "" {
			reached[f.RuleID]++
		}
	}
}

func mustJSON(t *testing.T, data []byte, v any, seed int64, what string) {
	t.Helper()
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("seed %d: parse %s %q: %v", seed, what, data, err)
	}
}

// failDivergence preserves the vault, records both outputs, and stops with the
// seed and the first differing hunk. A surviving difference is a real
// divergence to adjudicate, not to paper over.
func failDivergence(t *testing.T, surface string, seed int64, got, want []byte) {
	t.Helper()
	preserved := preserve(seed, got, want)
	t.Fatalf("%s diverged on seed %d — a real divergence to bring to review\npreserved: %s\nfirst diff:\n%s",
		surface, seed, preserved, firstDiff(want, got))
}

// failStructured stops on a structural difference (coverage or a journal
// existence report), preserving the vault and printing the diff.
func failStructured(t *testing.T, surface string, seed int64, diff string) {
	t.Helper()
	preserved := preserve(seed, []byte(diff), nil)
	t.Fatalf("%s diverged on seed %d — a real divergence to bring to review\npreserved: %s\n(-got +reference-normalized):\n%s",
		surface, seed, preserved, diff)
}

// failExit stops on a gate exit-code difference.
func failExit(t *testing.T, surface string, seed int64, got, want int) {
	t.Helper()
	preserved := preserve(seed, nil, nil)
	t.Fatalf("%s diverged on seed %d: this engine exited %d, the reference %d\npreserved: %s",
		surface, seed, got, want, preserved)
}

// preserve rebuilds the failing vault and records both outputs in a temporary
// directory that outlives the run, so a divergence is reproducible from the
// files as well as from the seed. The vault is regenerated from the seed rather
// than copied, since the same seed plans a byte-identical vault. Write failures
// are swallowed: the seed alone still reproduces the vault. The directory is
// deliberately not the test's own tree, which is removed on completion.
func preserve(seed int64, got, want []byte) string {
	art, err := os.MkdirTemp("", fmt.Sprintf("kurodo-divergence-seed-%d-", seed))
	if err != nil {
		return fmt.Sprintf("(could not preserve; reproduce from seed %d)", seed)
	}
	contract, notes, _ := planVault(seed)
	_ = writeUnder(art, filepath.FromSlash("vault/"+schema.ContractRelPath), contract)
	for _, n := range notes {
		_ = writeUnder(art, filepath.Join("vault", filepath.FromSlash(n.Path)), n.Body)
	}
	if got != nil {
		_ = os.WriteFile(filepath.Join(art, "got.txt"), got, 0o600)
	}
	if want != nil {
		_ = os.WriteFile(filepath.Join(art, "reference.txt"), want, 0o600)
	}
	return art
}

// writeUnder writes content to base/rel, creating parent directories.
func writeUnder(base, rel, content string) error {
	full := filepath.Join(base, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		return err
	}
	return os.WriteFile(full, []byte(content), 0o600)
}

// firstDiff returns the first differing line of two outputs with a little
// context, so a failure names the exact line that diverged rather than dumping
// the whole stream.
func firstDiff(want, got []byte) string {
	wl := strings.Split(string(want), "\n")
	gl := strings.Split(string(got), "\n")
	n := max(len(wl), len(gl))
	for i := range n {
		w, g := lineAt(wl, i), lineAt(gl, i)
		if w != g {
			return fmt.Sprintf("line %d:\n  reference: %s\n  this engine: %s", i+1, w, g)
		}
	}
	return "(no line-level difference; a trailing-byte difference)"
}

func lineAt(lines []string, i int) string {
	if i < len(lines) {
		return lines[i]
	}
	return "(absent)"
}

func firstN(s []string, n int) []string {
	if len(s) < n {
		return s
	}
	return s[:n]
}

// parseEnvInt resolves a run's integer setting, keeping unset distinct from
// misconfigured. An absent or empty value is a legitimate default and yields
// the fallback; a value that is present but unparseable is a typo the run must
// refuse rather than silently shrink to the default. On a bad value it returns
// an error naming the variable and the offending text.
func parseEnvInt(key, raw string, fallback int) (int, error) {
	if raw == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s=%q is not an integer", key, raw)
	}
	return n, nil
}

// envInt reads an integer run setting from the environment, failing the test
// loudly when the value is present but unparseable so a fuzzing run configured
// by a typo refuses to run instead of falling back to its default unnoticed.
func envInt(t *testing.T, key string, fallback int) int {
	t.Helper()
	n, err := parseEnvInt(key, os.Getenv(key), fallback)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// TestParseEnvInt pins the three behaviours that keep a run from starting under
// a silently-wrong configuration: an unset value falls back, a valid value
// (including a legal zero seed base) parses, and a present-but-unparseable
// value errors while naming the variable and the offending text.
func TestParseEnvInt(t *testing.T) {
	t.Parallel()
	const (
		key      = "KURODO_DIFF_ROUNDS"
		fallback = 7
	)
	tests := []struct {
		name    string
		raw     string
		want    int
		wantErr bool
	}{
		{name: "empty falls back", raw: "", want: fallback},
		{name: "valid parses", raw: "42", want: 42},
		{name: "zero parses", raw: "0", want: 0},
		{name: "negative parses", raw: "-3", want: -3},
		{name: "garbage errors", raw: "abc", wantErr: true},
		{name: "trailing garbage errors", raw: "12x", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseEnvInt(key, tt.raw, fallback)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseEnvInt(%q, %q) error = %v, wantErr %v", key, tt.raw, err, tt.wantErr)
			}
			if tt.wantErr {
				if !strings.Contains(err.Error(), key) || !strings.Contains(err.Error(), tt.raw) {
					t.Errorf("parseEnvInt(%q, %q) error = %q, want it to name both the variable and the value", key, tt.raw, err)
				}
				return
			}
			if got != tt.want {
				t.Errorf("parseEnvInt(%q, %q) = %d, want %d", key, tt.raw, got, tt.want)
			}
		})
	}
}

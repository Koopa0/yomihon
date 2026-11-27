package judge

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// existsFoldSkeleton is the tracked part of the width-fold vault, and
// existsFoldNote is the note written into it at test time. The note's filename
// carries a fullwidth colon, which a Go module zip rejects, so it cannot be
// tracked under that name and every `go install module@version` would fail.
const (
	existsFoldSkeleton = "testdata/vault-exists-fold"
	existsFoldNote     = "testdata/exists-fold-note.md"
)

// existsFoldVault returns a temporary vault made of the tracked skeleton plus
// the note under its fullwidth-colon filename, byte for byte as tracked.
func existsFoldVault(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(existsFoldSkeleton)); err != nil {
		t.Fatalf("copy %s: %v", existsFoldSkeleton, err)
	}
	body, err := os.ReadFile(existsFoldNote)
	if err != nil {
		t.Fatalf("read %s: %v", existsFoldNote, err)
	}
	vault, err := os.OpenRoot(root)
	if err != nil {
		t.Fatalf("open %s: %v", root, err)
	}
	t.Cleanup(func() {
		if err := vault.Close(); err != nil {
			t.Errorf("close %s: %v", root, err)
		}
	})
	if err := vault.MkdirAll("Concepts/golang", 0o750); err != nil {
		t.Fatalf("mkdir Concepts/golang: %v", err)
	}
	if err := vault.WriteFile("Concepts/golang/"+existsFoldExact+".md", body, 0o600); err != nil {
		t.Fatalf("write fixture note: %v", err)
	}
	return root
}

// existsFoldExact is the fixture filename as written: a fullwidth colon
// (U+FF1A) between the two CJK runs. existsFoldASCII is the same letters
// with the ASCII colon an unswitched IME leaves behind.
const (
	existsFoldExact = "指標：逃逸分析"
	existsFoldASCII = "指標:逃逸分析"
)

// TestExistsReportsAWidthFoldAsANearMatch holds the hole an agent falls
// into: a name search already answers with a fold-equal title is told
// free. The fixture filename carries a fullwidth colon. Queried with that
// colon it is an exact hit and exits 0; queried with the ASCII twin it is
// a near match and exits 1. The exit code stays a write-if-absent gate
// and is not flipped for a near-match-only answer.
func TestExistsReportsAWidthFoldAsANearMatch(t *testing.T) {
	t.Parallel()

	t.Run("fullwidth colon is an exact hit", func(t *testing.T) {
		t.Parallel()
		got, exit, err := existsBytes(t.Context(), &ExistsOptions{
			Root: existsFoldVault(t), Name: existsFoldExact, Format: FormatJSON,
		})
		if err != nil {
			t.Fatalf("RunExists: %v", err)
		}
		if exit != 0 {
			t.Errorf("RunExists(%q) exit = %d, want 0", existsFoldExact, exit)
		}
		if bytes.Contains(got, []byte("near_matches")) {
			t.Errorf("an exact answer carries near_matches: %s", got)
		}
		if !bytes.Contains(got, []byte(`"field":"filename"`)) || !bytes.Contains(got, []byte(`"field":"title"`)) {
			t.Errorf("exact hit missed a field the fixture exposes: %s", got)
		}
	})

	t.Run("human names the near match and still says the name does not exist", func(t *testing.T) {
		t.Parallel()
		got, exit, err := existsBytes(t.Context(), &ExistsOptions{
			Root: existsFoldVault(t), Name: existsFoldASCII, Format: FormatHuman,
		})
		if err != nil {
			t.Fatalf("RunExists: %v", err)
		}
		if exit != 1 {
			t.Errorf("human RunExists(%q) exit = %d, want 1", existsFoldASCII, exit)
		}
		rendered := string(got)
		if !strings.Contains(rendered, "does not exist") {
			t.Errorf("a near-match-only answer no longer says the name does not exist:\n%s", rendered)
		}
		if !strings.Contains(rendered, "is near 1 note(s):") {
			t.Errorf("the human answer does not name the fold-equal note:\n%s", rendered)
		}
		if !strings.Contains(rendered, "Concepts/golang/"+existsFoldExact+".md") {
			t.Errorf("the human answer does not name the fixture path:\n%s", rendered)
		}
	})
}

// TestEveryCommandWithholdsTheJournal holds what a withheld directory owes each
// command over the diary fixture, whose journal holds a note mistyped as a
// concept: check and coverage name nothing under it in any format, and exists
// answers a name only a journal note carries as present while describing
// nothing about it, because its exit code is a write-if-absent gate and a
// caller told the name is free would create a second note under it.
func TestEveryCommandWithholdsTheJournal(t *testing.T) {
	t.Parallel()
	root := judgeFixtureRootWithPrivacy(t, "testdata/vault-diary", "Diary")
	for _, format := range []Format{FormatJSON, FormatHuman, FormatMarkdown} {
		t.Run(format.String(), func(t *testing.T) {
			t.Parallel()
			checkOutput, _, err := checkBytes(t.Context(), &CheckOptions{Root: root, All: true, Format: format})
			if err != nil {
				t.Fatalf("RunCheck() error = %v", err)
			}
			if bytes.Contains(checkOutput, []byte("Diary")) {
				t.Errorf("RunCheck() names the journal:\n%s", checkOutput)
			}
			if !bytes.Contains(checkOutput, []byte("Notes/keep.md")) {
				t.Errorf("RunCheck() dropped the public finding beside the journal:\n%s", checkOutput)
			}
			if format == FormatMarkdown {
				return
			}

			coverageOutput, _, err := coverageBytes(t.Context(), &CoverageOptions{Root: root, Format: format})
			if err != nil {
				t.Fatalf("RunCoverage() error = %v", err)
			}
			if bytes.Contains(coverageOutput, []byte("Diary")) {
				t.Errorf("RunCoverage() names the journal:\n%s", coverageOutput)
			}

			existsOutput, exit, err := existsBytes(t.Context(), &ExistsOptions{Root: root, Name: "Private Session Note", Format: format})
			if err != nil {
				t.Fatalf("RunExists() error = %v", err)
			}
			if exit != 0 {
				t.Errorf("RunExists() exit = %d, want 0 for a name only a journal note carries", exit)
			}
			for _, leaked := range []string{"Diary", "2026-07-01", `"path"`, `"field"`, `"value"`} {
				if bytes.Contains(existsOutput, []byte(leaked)) {
					t.Errorf("RunExists() exposed %q about a withheld note:\n%s", leaked, existsOutput)
				}
			}

			_, exit, err = existsBytes(t.Context(), &ExistsOptions{Root: root, Name: "No Note Carries This Name", Format: format})
			if err != nil {
				t.Fatalf("RunExists(absent) error = %v", err)
			}
			if exit != 1 {
				t.Errorf("RunExists(absent) exit = %d, want 1", exit)
			}
		})
	}

	withheld, _, err := existsBytes(t.Context(), &ExistsOptions{Root: root, Name: "Private Session Note", Format: FormatJSON})
	if err != nil {
		t.Fatalf("RunExists() error = %v", err)
	}
	if got, want := string(withheld), "{\"query\":\"Private Session Note\",\"matches\":[],\"withheld\":true}\n"; got != want {
		t.Errorf("RunExists() JSON = %q, want %q", got, want)
	}
	plain, _, err := existsBytes(t.Context(), &ExistsOptions{Root: root, Name: "keep", Format: FormatJSON})
	if err != nil {
		t.Fatalf("RunExists(keep) error = %v", err)
	}
	if bytes.Contains(plain, []byte("withheld")) || !bytes.Contains(plain, []byte("Notes/keep.md")) {
		t.Errorf("RunExists(keep) = %s, want the public note matched plainly", plain)
	}
}

// TestDenyErrorGatesASchemaError holds the exit code a scheduled run scripts
// against: --deny error fails over the report fixture's schema error, and no
// --deny does not.
func TestDenyErrorGatesASchemaError(t *testing.T) {
	t.Parallel()
	const root = "testdata/vault-report"
	_, denied, err := checkBytes(t.Context(), &CheckOptions{Root: root, Format: FormatJSON, Deny: []string{"error"}})
	if err != nil {
		t.Fatalf("RunCheck: %v", err)
	}
	if denied != 1 {
		t.Errorf("--deny error exit = %d, want 1 (a schema error is present)", denied)
	}
	_, clean, err := checkBytes(t.Context(), &CheckOptions{Root: root, Format: FormatJSON})
	if err != nil {
		t.Fatalf("RunCheck: %v", err)
	}
	if clean != 0 {
		t.Errorf("no --deny exit = %d, want 0", clean)
	}
}

// mkFinding builds a minimal finding with a rule id and severity, enough to
// exercise the gate.
func mkFinding(rule RuleID, sev Severity) Finding {
	return Finding{RuleID: rule, Severity: sev, Path: "a.md"}
}

// TestGated covers the deny matrix: a severity keyword gates at or above the
// lowest denied level, a rule id gates only at warn or above, an info finding
// never gates through its rule id, and an empty deny never gates.
func TestGated(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		findings []Finding
		deny     []string
		want     bool
	}{
		{name: "error denied by error keyword", findings: []Finding{mkFinding("schema.enum", SeverityError)}, deny: []string{"error"}, want: true},
		{name: "error denied by warn keyword", findings: []Finding{mkFinding("schema.enum", SeverityError)}, deny: []string{"warn"}, want: true},
		{name: "warn not denied by error keyword", findings: []Finding{mkFinding("link.broken", SeverityWarn)}, deny: []string{"error"}, want: false},
		{name: "warn denied by warn keyword", findings: []Finding{mkFinding("link.broken", SeverityWarn)}, deny: []string{"warn"}, want: true},
		{name: "info denied by info keyword", findings: []Finding{mkFinding("link.broken", SeverityInfo)}, deny: []string{"info"}, want: true},
		{name: "warn denied by its rule id", findings: []Finding{mkFinding("link.broken", SeverityWarn)}, deny: []string{"link.broken"}, want: true},
		{name: "info never gates by rule id", findings: []Finding{mkFinding("link.broken", SeverityInfo)}, deny: []string{"link.broken"}, want: false},
		{name: "warn not denied by a different rule id", findings: []Finding{mkFinding("link.broken", SeverityWarn)}, deny: []string{"collision.alias"}, want: false},
		{name: "empty deny never gates", findings: []Finding{mkFinding("schema.enum", SeverityError)}, deny: nil, want: false},
		{name: "two keywords take the lower threshold", findings: []Finding{mkFinding("link.broken", SeverityWarn)}, deny: []string{"error", "warn"}, want: true},
		{name: "keyword and rule id coexist", findings: []Finding{mkFinding("schema.enum", SeverityError)}, deny: []string{"warn", "collision.alias"}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := gated(tt.findings, tt.deny); got != tt.want {
				t.Errorf("gated(%v, %v) = %t, want %t", tt.findings, tt.deny, got, tt.want)
			}
		})
	}
}

// TestRunCheckRejectsUnknownDeny asserts an unknown --deny token is a tool error
// (the caller turns it into exit 2), while a severity keyword and a real rule id
// are accepted.
func TestRunCheckRejectsUnknownDeny(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		token   string
		wantErr bool
	}{
		{token: "bogus", wantErr: true},
		{token: "warn"},
		{token: "link.broken"},
	} {
		_, _, err := checkBytes(t.Context(), &CheckOptions{Root: "testdata/vault-report", Format: FormatJSON, Deny: []string{tt.token}})
		if (err != nil) != tt.wantErr {
			t.Errorf("RunCheck(--deny %q) error = %v, want error %t", tt.token, err, tt.wantErr)
		}
	}
}

// TestParseBaseline pins what the parser collects: current-version
// fingerprints, with blank lines and the trailing newline not findings. What it
// refuses is TestRunCheckBaselineStrictness.
func TestParseBaseline(t *testing.T) {
	t.Parallel()
	jsonl := `{"rule_id":"link.broken","fingerprint":"v1:aaaa"}` + "\n\n" + `{"fingerprint":"v1:bbbb"}` + "\n"
	got, err := parseBaseline(jsonl)
	if err != nil {
		t.Fatalf("parseBaseline(valid) error = %v", err)
	}
	want := map[string]bool{"v1:aaaa": true, "v1:bbbb": true}
	if len(got) != len(want) {
		t.Fatalf("parseBaseline collected %v, want %v", got, want)
	}
	for fp := range want {
		if !got[fp] {
			t.Errorf("parseBaseline missing fingerprint %q", fp)
		}
	}
}

// TestRunCheckBaseline asserts a baseline of a run's own output leaves nothing
// new, a baseline of one line drops exactly that finding, and an unreadable
// baseline file is a tool error.
func TestRunCheckBaseline(t *testing.T) {
	t.Parallel()
	const root = "testdata/vault-report"
	full, _, err := checkBytes(t.Context(), &CheckOptions{Root: root, Format: FormatJSON})
	if err != nil {
		t.Fatalf("RunCheck: %v", err)
	}
	lines := bytes.Split(bytes.TrimRight(full, "\n"), []byte("\n"))

	dir := t.TempDir()
	base := filepath.Join(dir, "baseline.jsonl")
	if err = os.WriteFile(base, full, 0o600); err != nil {
		t.Fatalf("write baseline: %v", err)
	}
	got, exit, err := checkBytes(t.Context(), &CheckOptions{Root: root, Format: FormatJSON, Baseline: base})
	if err != nil {
		t.Fatalf("RunCheck with full baseline: %v", err)
	}
	if len(got) != 0 || exit != 0 {
		t.Errorf("baseline of the whole run left %d bytes, exit %d; want empty, 0", len(got), exit)
	}

	oneLine := filepath.Join(dir, "one.jsonl")
	if err = os.WriteFile(oneLine, append(bytes.Clone(lines[0]), '\n'), 0o600); err != nil {
		t.Fatalf("write baseline: %v", err)
	}
	got, _, err = checkBytes(t.Context(), &CheckOptions{Root: root, Format: FormatJSON, Baseline: oneLine})
	if err != nil {
		t.Fatalf("RunCheck with one-line baseline: %v", err)
	}
	gotLines := bytes.Split(bytes.TrimRight(got, "\n"), []byte("\n"))
	if len(gotLines) != len(lines)-1 {
		t.Errorf("one-line baseline left %d findings, want %d", len(gotLines), len(lines)-1)
	}
	if bytes.Contains(got, lines[0]) {
		t.Error("the finding named by the baseline should have been dropped")
	}

	if _, _, err := checkBytes(t.Context(), &CheckOptions{Root: root, Format: FormatJSON, Baseline: filepath.Join(dir, "missing.jsonl")}); err == nil {
		t.Error("an unreadable baseline should be a tool error")
	}
}

// TestRunCheckBaselineStrictness asserts the baseline reader refuses input it
// cannot fully honor, naming the line where it can, since a skipped line
// subtracts less than the caller believes and reports old findings as new. An
// empty file stays a valid, empty baseline.
func TestRunCheckBaselineStrictness(t *testing.T) {
	t.Parallel()
	const root = "testdata/vault-report"
	writeBaseline := func(t *testing.T, content string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "baseline.jsonl")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write baseline: %v", err)
		}
		return path
	}

	for _, tt := range []struct {
		name    string
		content string
		wantErr string
	}{
		{name: "a bare-hex fingerprint is a version mismatch", content: `{"rule_id":"link.broken","fingerprint":"c6a289a5d2524c77"}` + "\n", wantErr: "version"},
		{name: "an unparsable line", content: `{"fingerprint":"v1:c6a289a5d2524c77"}` + "\nnot json at all\n", wantErr: "line 2"},
		{name: "a finding without a fingerprint", content: `{"rule_id":"collision.alias"}` + "\n", wantErr: "line 1"},
		{name: "a non-string fingerprint", content: `{"fingerprint":7}` + "\n", wantErr: "line 1"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := checkBytes(t.Context(), &CheckOptions{Root: root, Format: FormatJSON, Baseline: writeBaseline(t, tt.content)})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("RunCheck(baseline %q) error = %v, want one naming %q", tt.content, err, tt.wantErr)
			}
		})
	}

	t.Run("an empty file is a valid empty baseline", func(t *testing.T) {
		t.Parallel()
		full, _, err := checkBytes(t.Context(), &CheckOptions{Root: root, Format: FormatJSON})
		if err != nil {
			t.Fatalf("RunCheck without baseline: %v", err)
		}
		got, exit, err := checkBytes(t.Context(), &CheckOptions{Root: root, Format: FormatJSON, Baseline: writeBaseline(t, "")})
		if err != nil {
			t.Fatalf("RunCheck with empty baseline: %v", err)
		}
		if !bytes.Equal(got, full) || exit != 0 {
			t.Errorf("an empty baseline left %d of %d bytes, exit %d; want all of them, 0", len(got), len(full), exit)
		}
	})
}

// TestRunOnEmptyVault asserts the three commands run cleanly over a directory
// with no notes, emitting the empty forms and the expected exit codes rather
// than failing.
func TestRunOnEmptyVault(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTestContract(t, dir, nil)

	check, exit, err := checkBytes(t.Context(), &CheckOptions{Root: dir, Format: FormatJSON})
	if err != nil {
		t.Fatalf("RunCheck: %v", err)
	}
	if len(check) != 0 || exit != 0 {
		t.Errorf("empty check = %q, exit %d; want empty, 0", check, exit)
	}

	cov, exit, err := coverageBytes(t.Context(), &CoverageOptions{Root: dir, Format: FormatJSON})
	if err != nil {
		t.Fatalf("RunCoverage: %v", err)
	}
	if want := "{\"total_concepts\":0,\"domains\":[],\"pending_mount\":[],\"orphans\":[]}\n"; string(cov) != want {
		t.Errorf("empty coverage = %q, want %q", cov, want)
	}
	if exit != 0 {
		t.Errorf("empty coverage exit = %d, want 0", exit)
	}

	ex, exit, err := existsBytes(t.Context(), &ExistsOptions{Root: dir, Name: "Anything", Format: FormatJSON})
	if err != nil {
		t.Fatalf("RunExists: %v", err)
	}
	if want := "{\"query\":\"Anything\",\"matches\":[]}\n"; string(ex) != want {
		t.Errorf("empty exists = %q, want %q", ex, want)
	}
	if exit != 1 {
		t.Errorf("empty exists exit = %d, want 1", exit)
	}
}

// TestCheckPathFilter asserts the positional path arguments filter the output to
// the findings a path touches: a folder prefix keeps a whole subtree, an exact
// file keeps only that file, and the vault root keeps everything. The shell's
// punctuation around a path — a trailing slash, a leading "./" — is not part of
// the name. The distinct citing paths of the kept findings are compared against
// the exact expected set, so both over- and under-filtering fail.
func TestCheckPathFilter(t *testing.T) {
	t.Parallel()
	const root = "testdata/vault-report"
	whole := []string{
		"Concepts/golang/Bad.md",
		"Concepts/golang/Map.md",
		"Concepts/golang/Slice.md",
		"Concepts/japanese/Kana.md",
	}
	tests := []struct {
		name  string
		paths []string
		want  []string
	}{
		{name: "whole japanese folder", paths: []string{"Concepts/japanese"}, want: []string{"Concepts/japanese/Kana.md"}},
		{name: "trailing slash is trimmed", paths: []string{"Concepts/japanese/"}, want: []string{"Concepts/japanese/Kana.md"}},
		{name: "leading dot slash is trimmed", paths: []string{"./Concepts/japanese"}, want: []string{"Concepts/japanese/Kana.md"}},
		{name: "exact file", paths: []string{"Concepts/golang/Bad.md"}, want: []string{"Concepts/golang/Bad.md"}},
		{name: "vault root keeps everything", paths: []string{"."}, want: whole},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			findings, err := runCheckAction(t.Context(), root, tt.paths, false)
			if err != nil {
				t.Fatalf("check: %v", err)
			}
			got := []string{}
			for _, f := range findings {
				if !slices.Contains(got, f.Path) {
					got = append(got, f.Path)
				}
			}
			slices.Sort(got)
			if !slices.Equal(got, tt.want) {
				t.Errorf("check(%q, paths=%v) citing paths = %v, want %v", root, tt.paths, got, tt.want)
			}
		})
	}
}

// TestCheckPathFilterRejectsEmpty asserts a path filter that normalizes to
// nothing — an empty argument, or one that is only slashes — is a tool error
// rather than a silent scan of nothing, so a caller cannot mistake an empty
// filter for a clean corpus.
func TestCheckPathFilterRejectsEmpty(t *testing.T) {
	t.Parallel()
	for _, p := range []string{"", "/", "///"} {
		if _, err := runCheckAction(t.Context(), "testdata/vault-report", []string{p}, false); err == nil {
			t.Errorf("check with path filter %q = nil error, want a tool error", p)
		}
	}
}

// TestCheckPathFilterRefusesUnobservedScope asserts a path the scan never
// observed is refused, for the same reason an empty one is: it can match
// nothing, so filtering with it returns an empty report that reads exactly like
// a clean one. A misspelled folder, a name that stops mid-segment, a path from
// another machine — each would otherwise answer a question about ground this
// command never covered. The refusal names the path back so the reader can see
// which of their arguments was the typo.
func TestCheckPathFilterRefusesUnobservedScope(t *testing.T) {
	t.Parallel()
	for _, p := range []string{
		"Concepts/gola",                 // stops inside a segment of Concepts/golang
		"Concepts/golang/Missing.md",    // a file that is not there
		"concepts/golang",               // the vault spells it with a capital
		"nope",                          // nothing like it anywhere
		"/Users/someone/vault/Concepts", // an absolute path from somewhere else
		"Concepts/japanese/../golang",   // unresolved traversal is not a canonical path
	} {
		findings, err := runCheckAction(t.Context(), "testdata/vault-report", []string{p}, false)
		if err == nil {
			t.Errorf("check with path filter %q = %d findings and nil error, want a tool error", p, len(findings))
			continue
		}
		if !strings.Contains(err.Error(), p) {
			t.Errorf("check with path filter %q: error %q does not name the path back", p, err)
		}
	}
}

// TestCheckPathFilterRefusesAPathThatIsNotVaultRelative asserts a filter that
// climbs out of the vault or has an empty segment is refused for its shape, in
// a folder with no rules file, rather than as ground a rules file withholds.
func TestCheckPathFilterRefusesAPathThatIsNotVaultRelative(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, "Notes/a.md", "# A\n")
	for _, p := range []string{"../Notes", "Notes//a.md", "Notes/../Notes"} {
		_, err := runCheckAction(t.Context(), root, []string{p}, false)
		if err == nil || !strings.Contains(err.Error(), vaultRelativeExample) {
			t.Errorf("check with path filter %q = %v, want the refusal that asks for a vault-relative path", p, err)
		}
	}
}

// TestCheckPathFilterStopsAtPathBoundary asserts a prefix matches only whole
// path segments, so a folder does not drag in a sibling whose name merely
// starts the same way. This is asserted on the matcher rather than through
// check, because a prefix that stops mid-segment names nothing the scan
// observed and is now refused before any matching happens.
func TestCheckPathFilterStopsAtPathBoundary(t *testing.T) {
	t.Parallel()
	const path = "Concepts/golang/Bad.md"
	tests := []struct {
		name   string
		prefix string
		want   bool
	}{
		{name: "exact file", prefix: path, want: true},
		{name: "parent folder", prefix: "Concepts/golang", want: true},
		{name: "grandparent folder", prefix: "Concepts", want: true},
		{name: "vault root", prefix: ".", want: true},
		{name: "sibling sharing a name start", prefix: "Concepts/go", want: false},
		{name: "unrelated folder", prefix: "Writing", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := underAnyPrefix(path, []string{tt.prefix}); got != tt.want {
				t.Errorf("underAnyPrefix(%q, %q) = %t, want %t", path, tt.prefix, got, tt.want)
			}
		})
	}
}

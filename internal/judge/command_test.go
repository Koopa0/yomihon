package judge

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

// wantGolden asserts got equals the golden file byte for byte, dumping hex on a
// difference so a stray byte is visible.
//
// There is no flag and no environment switch that rewrites a golden, and the
// absence is the design: these bytes are parsed by programs outside this
// repository, so a difference is a question about whether the format changed,
// and a one-key answer turns every such question into a rewrite. A change that
// is genuinely wanted is made by editing the golden file by hand, in its own
// commit, so what a reviewer reads is the byte change itself beside the reason
// the bytes moved.
func wantGolden(t *testing.T, got []byte, golden string) {
	t.Helper()
	want, err := os.ReadFile(golden) // #nosec G304 -- golden is a fixed testdata path from the test table, not untrusted input
	if err != nil {
		t.Fatalf("read golden %s: %v", golden, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("output differs from golden %s\ngot:\n%s\nwant:\n%s\ngot hex:\n%s\nwant hex:\n%s",
			golden, got, want, hex.Dump(got), hex.Dump(want))
	}
}

// TestRunCoverageGolden asserts the coverage command's compact JSON and human
// renderings equal the reference tool's bytes over the report fixture.
func TestRunCoverageGolden(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		format Format
		golden string
	}{
		{name: "json", format: FormatJSON, golden: "testdata/golden/coverage.golden"},
		{name: "human", format: FormatHuman, golden: "testdata/golden/coverage-human.golden"},
		{name: "md falls back to human", format: FormatMarkdown, golden: "testdata/golden/coverage-human.golden"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, exit, err := RunCoverage(t.Context(), &CoverageOptions{Root: "testdata/vault-report", Format: tt.format})
			if err != nil {
				t.Fatalf("RunCoverage: %v", err)
			}
			if exit != 0 {
				t.Errorf("RunCoverage exit = %d, want 0", exit)
			}
			wantGolden(t, got, tt.golden)
		})
	}
}

// TestRunExistsGolden asserts the exists command's JSON and human renderings
// and its exit code — 0 when a match exists, 1 when none does — over the report
// fixture, covering a multi-note alias hit, a title/title_en hit, and a miss.
func TestRunExistsGolden(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		query    string
		format   Format
		golden   string
		wantExit int
	}{
		{name: "alias across two notes", query: "shared", format: FormatJSON, golden: "testdata/golden/exists-shared.golden", wantExit: 0},
		{name: "title and title_en", query: "Slices in Depth", format: FormatJSON, golden: "testdata/golden/exists-titleen.golden", wantExit: 0},
		{name: "no match", query: "Nonexistent Concept", format: FormatJSON, golden: "testdata/golden/exists-missing.golden", wantExit: 1},
		{name: "human match", query: "shared", format: FormatHuman, golden: "testdata/golden/exists-shared-human.golden", wantExit: 0},
		{name: "human miss", query: "Nonexistent Concept", format: FormatHuman, golden: "testdata/golden/exists-missing-human.golden", wantExit: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, exit, err := RunExists(t.Context(), &ExistsOptions{Root: "testdata/vault-report", Name: tt.query, Format: tt.format})
			if err != nil {
				t.Fatalf("RunExists: %v", err)
			}
			if exit != tt.wantExit {
				t.Errorf("RunExists(%q) exit = %d, want %d", tt.query, exit, tt.wantExit)
			}
			wantGolden(t, got, tt.golden)
		})
	}
}

// TestExistsSkipsDiary pins that the existence oracle never surfaces a note in
// the private daily journal. An agent's dedup query must not learn a journal
// note's name, path, or alias, so a query that would match only a journal note
// returns no match, while a public note still matches — the skip is scoped, not
// a blanket empty.
// TestExistsSkipsDiary holds both halves of what a private note owes this
// oracle. It describes nothing about itself — no path, no matched field, no
// value — and it still answers, because the exit code is documented as a
// write-if-absent gate and a caller told the name is free creates a second
// note under a private note's own name.
func TestExistsSkipsDiary(t *testing.T) {
	t.Parallel()
	root := judgeFixtureRootWithPrivacy(t, "testdata/vault-diary", "Diary")
	notes, err := collectNotes(t.Context(), root)
	if err != nil {
		t.Fatalf("collectNotes: %v", err)
	}
	authority := loadTestAuthority(t, root)

	r := existsLookup(notes, "Private Session Note", authority)
	if len(r.Matches) != 0 {
		t.Errorf("exists(%q) described %d match(es); a journal note must name nothing about itself: %+v", "Private Session Note", len(r.Matches), r.Matches)
	}
	if !r.Withheld || !r.found() {
		t.Errorf("exists(%q) answered absent for a note that exists; a write-if-absent gate would create a duplicate of it", "Private Session Note")
	}
	// The two answers a caller actually receives, asserted as the bytes they
	// are: the machine field AGENT_INTERFACE.md documents, and the sentence a
	// person reads. Checking only the struct would leave both free to be
	// deleted with the suite green.
	wire, err := marshalWire(r)
	if err != nil {
		t.Fatalf("marshalWire: %v", err)
	}
	if got, want := string(wire), "{\"query\":\"Private Session Note\",\"matches\":[],\"withheld\":true}\n"; got != want {
		t.Errorf("exists JSON = %q, want %q", got, want)
	}
	rendered := renderExists(r)
	if !strings.Contains(rendered, "withholds") {
		t.Errorf("the human answer does not say the name is answered by something it may not describe:\n%s", rendered)
	}
	for _, leaked := range []string{"Diary/", "Private Session Note.md", "title"} {
		if strings.Contains(rendered, leaked) {
			t.Errorf("the human answer leaks %q about a withheld note:\n%s", leaked, rendered)
		}
	}

	// And the ordinary answer's bytes are unchanged, which is what lets the
	// new field ship without moving a frozen contract.
	plain, err := marshalWire(existsLookup(notes, "keep", authority))
	if err != nil {
		t.Fatalf("marshalWire: %v", err)
	}
	if strings.Contains(string(plain), "withheld") {
		t.Errorf("an ordinary answer carries the withheld field: %s", plain)
	}

	if r := existsLookup(notes, "keep", authority); !r.found() || r.Withheld {
		t.Errorf("exists(%q) = found %t withheld %t, want the public note matched plainly", "keep", r.found(), r.Withheld)
	}

	// A name nothing carries stays absent, or the gate would refuse every
	// write in a vault that merely declares a private directory.
	if r := existsLookup(notes, "No Note Anywhere Carries This", authority); r.found() || r.Withheld {
		t.Errorf("a name nothing carries was reported present (found %t withheld %t)", r.found(), r.Withheld)
	}
}

// TestCoverageExcludesDiary pins that the coverage report names no note in the
// private daily journal, even a journal note mistyped as a concept. The fixture
// includes such a note; coverage drops every path under Diary/, so it never
// reaches the report, and this asserts that unconditional exclusion holds.
func TestCoverageExcludesDiary(t *testing.T) {
	t.Parallel()
	root := judgeFixtureRootWithPrivacy(t, "testdata/vault-diary", "Diary")
	got, _, err := RunCoverage(t.Context(), &CoverageOptions{Root: root, Format: FormatJSON})
	if err != nil {
		t.Fatalf("RunCoverage: %v", err)
	}
	if bytes.Contains(got, []byte("Diary")) {
		t.Errorf("coverage output names the private daily journal:\n%s", got)
	}
}

func TestJudgeUsesConfiguredPrivacyBoundary(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	contract, err := os.ReadFile("testdata/vault-supersession/System/schemas/vault-schema.toml")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	contract = bytes.Replace(contract, []byte("never_egress_dirs = []"), []byte(`never_egress_dirs = ["Private"]`), 1)
	write(t, root, schema.ContractRelPath, string(contract))
	write(t, root, "Private/Hidden.md", "---\ntitle: Hidden\ntype: concept\nstatus: ready\n---\n\n[[Private Missing]]\n")
	write(t, root, "Private/Brief.md", "---\ntitle: Brief\ntype: research-brief\nstatus: ready\n---\n")
	write(t, root, "Private/Old Lesson.md", "---\ntitle: Old Lesson\ntype: lesson\nstatus: ready\nsuccessors: [new]\n---\n")
	write(t, root, "Private/Private Archived.md", "---\ntitle: Private Archived\ntype: lesson\nstatus: archived\n---\n")
	write(t, root, "Concepts/Public.md", "---\ntitle: Public\ntype: concept\nstatus: ready\n---\n\n[[Public Missing]]\n")
	write(t, root, "Maps/Private Link.md", "---\ntitle: Private Link\ntype: study-path\nstatus: ready\n---\n\n[[Private Archived]]\n")

	for _, format := range []Format{FormatJSON, FormatHuman, FormatMarkdown} {
		checkOutput, _, err := RunCheck(t.Context(), &CheckOptions{Root: root, Format: format})
		if err != nil {
			t.Fatalf("RunCheck(%d) error = %v", format, err)
		}
		if bytes.Contains(checkOutput, []byte("Private/")) {
			t.Errorf("RunCheck(%d) exposed configured private output:\n%s", format, checkOutput)
		}
		if !bytes.Contains(checkOutput, []byte("Concepts/Public.md")) {
			t.Errorf("RunCheck(%d) dropped public output with private output:\n%s", format, checkOutput)
		}

		coverageOutput, _, err := RunCoverage(t.Context(), &CoverageOptions{Root: root, Format: format})
		if err != nil {
			t.Fatalf("RunCoverage(%d) error = %v", format, err)
		}
		if bytes.Contains(coverageOutput, []byte("Private/")) {
			t.Errorf("RunCoverage(%d) exposed configured private output:\n%s", format, coverageOutput)
		}
		if !bytes.Contains(coverageOutput, []byte("Concepts/Public.md")) {
			t.Errorf("RunCoverage(%d) dropped public output with private output:\n%s", format, coverageOutput)
		}

		// A private-only match answers present and describes nothing. Exit 1
		// here would be the answer that matters most to get wrong: the exit
		// code is the documented write-if-absent gate, so "absent" sends the
		// caller to create a second note under this one's name.
		existsOutput, exit, err := RunExists(t.Context(), &ExistsOptions{Root: root, Name: "Hidden", Format: format})
		if err != nil {
			t.Fatalf("RunExists(%d) error = %v", format, err)
		}
		if exit != 0 {
			t.Errorf("RunExists(%d) exit = %d, want 0 for a private-only match", format, exit)
		}
		for _, leaked := range []string{"Private/", "Hidden.md", "\"path\"", "\"field\"", "\"value\""} {
			if bytes.Contains(existsOutput, []byte(leaked)) {
				t.Errorf("RunExists(%d) exposed %q about a withheld note:\n%s", format, leaked, existsOutput)
			}
		}

		// The gate still opens for a name nothing carries, or a vault that
		// merely declares a private directory could never create anything.
		_, absentExit, err := RunExists(t.Context(), &ExistsOptions{Root: root, Name: "No Note Carries This Name", Format: format})
		if err != nil {
			t.Fatalf("RunExists(%d, absent) error = %v", format, err)
		}
		if absentExit != 1 {
			t.Errorf("RunExists(%d, absent) exit = %d, want 1", format, absentExit)
		}
	}
}

// TestCronPayloads pins the four bytes the pipelines depend on: the exit code
// under --deny error, the two literals the log grinder greps for in the JSONL,
// the markdown report body, and the human summary's first line. The report
// fixture carries a schema error, warnings, and hidden findings, so all four
// surfaces are non-trivial.
func TestCronPayloads(t *testing.T) {
	t.Parallel()
	const root = "testdata/vault-report"

	t.Run("deny error exit code", func(t *testing.T) {
		t.Parallel()
		_, denied, err := RunCheck(t.Context(), &CheckOptions{Root: root, Format: FormatJSON, Deny: []string{"error"}})
		if err != nil {
			t.Fatalf("RunCheck: %v", err)
		}
		if denied != 1 {
			t.Errorf("--deny error exit = %d, want 1 (a schema error is present)", denied)
		}
		_, clean, err := RunCheck(t.Context(), &CheckOptions{Root: root, Format: FormatJSON})
		if err != nil {
			t.Fatalf("RunCheck: %v", err)
		}
		if clean != 0 {
			t.Errorf("no --deny exit = %d, want 0", clean)
		}
	})

	t.Run("jsonl grep literals", func(t *testing.T) {
		t.Parallel()
		out, _, err := RunCheck(t.Context(), &CheckOptions{Root: root, Format: FormatJSON})
		if err != nil {
			t.Fatalf("RunCheck: %v", err)
		}
		for _, literal := range []string{`"rule_id":"link.broken"`, `"severity":"warn"`} {
			if !bytes.Contains(out, []byte(literal)) {
				t.Errorf("JSONL is missing the grinder literal %q", literal)
			}
		}
	})

	t.Run("markdown report body", func(t *testing.T) {
		t.Parallel()
		out, _, err := RunCheck(t.Context(), &CheckOptions{Root: root, Format: FormatMarkdown})
		if err != nil {
			t.Fatalf("RunCheck: %v", err)
		}
		wantGolden(t, out, "testdata/golden/report-md.golden")
	})

	t.Run("human first line", func(t *testing.T) {
		t.Parallel()
		out, _, err := RunCheck(t.Context(), &CheckOptions{Root: root, Format: FormatHuman})
		if err != nil {
			t.Fatalf("RunCheck: %v", err)
		}
		first, _, _ := bytes.Cut(out, []byte("\n"))
		want := "11 findings: 3 error, 6 warn, 2 hidden (1 planned forward-refs, 1 external paths)"
		if string(first) != want {
			t.Errorf("human first line = %q, want %q", first, want)
		}
	})
}

// mkFinding builds a minimal finding with a rule id and severity, enough to
// exercise the gate.
func mkFinding(rule string, sev Severity) Finding {
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
	if _, _, err := RunCheck(t.Context(), &CheckOptions{Root: "testdata/vault-report", Format: FormatJSON, Deny: []string{"bogus"}}); err == nil {
		t.Error("RunCheck with --deny bogus = nil error, want a tool error")
	}
	for _, token := range append([]string{"error", "warn", "info"}, ruleIDs...) {
		if _, _, err := RunCheck(t.Context(), &CheckOptions{Root: "testdata/vault-report", Format: FormatJSON, Deny: []string{token}}); err != nil {
			t.Errorf("RunCheck with --deny %q = %v, want no error", token, err)
		}
	}
}

func TestRunCheckDenySupersessionRules(t *testing.T) {
	t.Parallel()

	for _, ruleID := range []string{predecessorNotArchivedRule, archivedNavigationRule} {
		t.Run(ruleID, func(t *testing.T) {
			t.Parallel()

			_, exit, err := RunCheck(t.Context(), &CheckOptions{
				Root:   "testdata/vault-supersession",
				Format: FormatJSON,
				Deny:   []string{ruleID},
			})
			if err != nil {
				t.Fatalf("RunCheck(--deny %q) error = %v", ruleID, err)
			}
			if exit != 1 {
				t.Errorf("RunCheck(--deny %q) exit = %d, want 1", ruleID, exit)
			}
		})
	}
}

// TestParseBaseline pins the parser itself: current-version fingerprints are
// collected, blank lines and the trailing newline are not findings, and each
// shape the strict reader refuses — a line that is not a JSON object, a
// finding without a fingerprint, a non-string fingerprint, and a fingerprint
// of another version — is an error rather than a silent skip. The
// command-level surface of the same refusals is TestRunCheckBaselineStrictness.
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

	refused := map[string]string{
		"not a JSON object":      "not json at all\n",
		"no fingerprint":         `{"rule_id":"collision.alias"}` + "\n",
		"non-string fingerprint": `{"fingerprint":7}` + "\n",
		"foreign version":        `{"fingerprint":"aaaa"}` + "\n",
	}
	for name, input := range refused {
		if _, err := parseBaseline(input); err == nil {
			t.Errorf("parseBaseline(%s) = nil error, want a refusal", name)
		}
	}
}

// TestRunCheckBaseline asserts a baseline of a run's own output leaves nothing
// new, a baseline of one line drops exactly that finding, and an unreadable
// baseline file is a tool error.
func TestRunCheckBaseline(t *testing.T) {
	t.Parallel()
	const root = "testdata/vault-report"
	full, _, err := RunCheck(t.Context(), &CheckOptions{Root: root, Format: FormatJSON})
	if err != nil {
		t.Fatalf("RunCheck: %v", err)
	}
	lines := bytes.Split(bytes.TrimRight(full, "\n"), []byte("\n"))

	dir := t.TempDir()
	base := filepath.Join(dir, "baseline.jsonl")
	if err = os.WriteFile(base, full, 0o600); err != nil {
		t.Fatalf("write baseline: %v", err)
	}
	got, exit, err := RunCheck(t.Context(), &CheckOptions{Root: root, Format: FormatJSON, Baseline: base})
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
	got, _, err = RunCheck(t.Context(), &CheckOptions{Root: root, Format: FormatJSON, Baseline: oneLine})
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

	if _, _, err := RunCheck(t.Context(), &CheckOptions{Root: root, Format: FormatJSON, Baseline: filepath.Join(dir, "missing.jsonl")}); err == nil {
		t.Error("an unreadable baseline should be a tool error")
	}
}

// TestRunCheckBaselineStrictness asserts the baseline reader refuses input it
// cannot fully honor: a fingerprint carrying another algorithm version, a line
// that is not a JSON object, and a finding without a string fingerprint each
// stop the run instead of being silently skipped, since a skipped line
// subtracts less than the caller believes and reports old findings as new. An
// empty file stays a valid, empty baseline. Each refusal is a tool error here;
// the exit code a caller scripts against is decided by the binary, and pinned
// beside the command line that decides it.
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

	t.Run("a bare-hex fingerprint is a version mismatch", func(t *testing.T) {
		t.Parallel()
		base := writeBaseline(t, `{"rule_id":"link.broken","fingerprint":"c6a289a5d2524c77"}`+"\n")
		_, _, err := RunCheck(t.Context(), &CheckOptions{Root: root, Format: FormatJSON, Baseline: base})
		if err == nil {
			t.Fatal("a baseline of another fingerprint version must be an error, not a silent no-op subtraction")
		}
		if !strings.Contains(err.Error(), "version") {
			t.Errorf("error %q should name the version mismatch", err)
		}
	})

	t.Run("an unparsable line is an error naming its line", func(t *testing.T) {
		t.Parallel()
		base := writeBaseline(t, `{"fingerprint":"v1:c6a289a5d2524c77"}`+"\nnot json at all\n")
		_, _, err := RunCheck(t.Context(), &CheckOptions{Root: root, Format: FormatJSON, Baseline: base})
		if err == nil {
			t.Fatal("an unparsable baseline line must be an error, not skipped")
		}
		if !strings.Contains(err.Error(), "line 2") {
			t.Errorf("error %q should name line 2", err)
		}
	})

	t.Run("a finding without a fingerprint is an error", func(t *testing.T) {
		t.Parallel()
		base := writeBaseline(t, `{"rule_id":"collision.alias"}`+"\n")
		_, _, err := RunCheck(t.Context(), &CheckOptions{Root: root, Format: FormatJSON, Baseline: base})
		if err == nil {
			t.Fatal("a baseline finding without a fingerprint must be an error, not skipped")
		}
		if !strings.Contains(err.Error(), "line 1") {
			t.Errorf("error %q should name line 1", err)
		}
	})

	t.Run("a non-string fingerprint is an error", func(t *testing.T) {
		t.Parallel()
		base := writeBaseline(t, `{"fingerprint":7}`+"\n")
		if _, _, err := RunCheck(t.Context(), &CheckOptions{Root: root, Format: FormatJSON, Baseline: base}); err == nil {
			t.Fatal("a non-string fingerprint must be an error, not skipped")
		}
	})

	t.Run("an empty file is a valid empty baseline", func(t *testing.T) {
		t.Parallel()
		full, _, err := RunCheck(t.Context(), &CheckOptions{Root: root, Format: FormatJSON})
		if err != nil {
			t.Fatalf("RunCheck without baseline: %v", err)
		}
		got, exit, err := RunCheck(t.Context(), &CheckOptions{Root: root, Format: FormatJSON, Baseline: writeBaseline(t, "")})
		if err != nil {
			t.Fatalf("RunCheck with empty baseline: %v", err)
		}
		if !bytes.Equal(got, full) {
			t.Error("an empty baseline must subtract nothing")
		}
		if exit != 0 {
			t.Errorf("exit = %d, want 0", exit)
		}
	})
}

// TestResolveFormat asserts an explicit flag wins and, without one, a terminal
// gets the human view and a pipe the machine view.
func TestResolveFormat(t *testing.T) {
	t.Parallel()
	json, human := FormatJSON, FormatHuman
	tests := []struct {
		name     string
		explicit *Format
		isTTY    bool
		want     Format
	}{
		{name: "explicit json over a terminal", explicit: &json, isTTY: true, want: FormatJSON},
		{name: "explicit human over a pipe", explicit: &human, isTTY: false, want: FormatHuman},
		{name: "default terminal is human", explicit: nil, isTTY: true, want: FormatHuman},
		{name: "default pipe is json", explicit: nil, isTTY: false, want: FormatJSON},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ResolveFormat(tt.explicit, tt.isTTY); got != tt.want {
				t.Errorf("ResolveFormat(%v, %t) = %d, want %d", tt.explicit, tt.isTTY, got, tt.want)
			}
		})
	}
}

// TestRunOnEmptyVault asserts the three commands run cleanly over a directory
// with no notes, emitting the empty forms and the expected exit codes rather
// than failing.
func TestRunOnEmptyVault(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTestContract(t, dir, nil)

	check, exit, err := RunCheck(t.Context(), &CheckOptions{Root: dir, Format: FormatJSON})
	if err != nil {
		t.Fatalf("RunCheck: %v", err)
	}
	if len(check) != 0 || exit != 0 {
		t.Errorf("empty check = %q, exit %d; want empty, 0", check, exit)
	}

	cov, exit, err := RunCoverage(t.Context(), &CoverageOptions{Root: dir, Format: FormatJSON})
	if err != nil {
		t.Fatalf("RunCoverage: %v", err)
	}
	if want := "{\"total_concepts\":0,\"domains\":[],\"pending_mount\":[],\"orphans\":[],\"unrouted\":[]}\n"; string(cov) != want {
		t.Errorf("empty coverage = %q, want %q", cov, want)
	}
	if exit != 0 {
		t.Errorf("empty coverage exit = %d, want 0", exit)
	}

	ex, exit, err := RunExists(t.Context(), &ExistsOptions{Root: dir, Name: "Anything", Format: FormatJSON})
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
		"Writing/lessons/golang/L1.md",
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

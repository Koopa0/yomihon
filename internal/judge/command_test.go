package judge

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// wantGolden asserts got equals the golden file byte for byte, dumping hex on a
// difference so a stray byte is visible.
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
			got, exit, err := RunCoverage(&CoverageOptions{Root: "testdata/vault-report", Format: tt.format})
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
			got, exit, err := RunExists(&ExistsOptions{Root: "testdata/vault-report", Name: tt.query, Format: tt.format})
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
		_, denied, err := RunCheck(&CheckOptions{Root: root, Format: FormatJSON, Deny: []string{"error"}})
		if err != nil {
			t.Fatalf("RunCheck: %v", err)
		}
		if denied != 1 {
			t.Errorf("--deny error exit = %d, want 1 (a schema error is present)", denied)
		}
		_, clean, err := RunCheck(&CheckOptions{Root: root, Format: FormatJSON})
		if err != nil {
			t.Fatalf("RunCheck: %v", err)
		}
		if clean != 0 {
			t.Errorf("no --deny exit = %d, want 0", clean)
		}
	})

	t.Run("jsonl grep literals", func(t *testing.T) {
		t.Parallel()
		out, _, err := RunCheck(&CheckOptions{Root: root, Format: FormatJSON})
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
		out, _, err := RunCheck(&CheckOptions{Root: root, Format: FormatMarkdown})
		if err != nil {
			t.Fatalf("RunCheck: %v", err)
		}
		wantGolden(t, out, "testdata/golden/report-md.golden")
	})

	t.Run("human first line", func(t *testing.T) {
		t.Parallel()
		out, _, err := RunCheck(&CheckOptions{Root: root, Format: FormatHuman})
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
	if _, _, err := RunCheck(&CheckOptions{Root: "testdata/vault-report", Format: FormatJSON, Deny: []string{"bogus"}}); err == nil {
		t.Error("RunCheck with --deny bogus = nil error, want a tool error")
	}
	for _, token := range []string{"error", "link.broken"} {
		if _, _, err := RunCheck(&CheckOptions{Root: "testdata/vault-report", Format: FormatJSON, Deny: []string{token}}); err != nil {
			t.Errorf("RunCheck with --deny %q = %v, want no error", token, err)
		}
	}
}

// TestParseBaseline asserts a baseline collects the fingerprint of each parsable
// line and skips a malformed line and a line with no fingerprint.
func TestParseBaseline(t *testing.T) {
	t.Parallel()
	jsonl := `{"rule_id":"link.broken","fingerprint":"aaaa"}
not json at all
{"rule_id":"collision.alias"}
{"fingerprint":"bbbb"}
`
	got := parseBaseline(jsonl)
	want := map[string]bool{"aaaa": true, "bbbb": true}
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
	full, _, err := RunCheck(&CheckOptions{Root: root, Format: FormatJSON})
	if err != nil {
		t.Fatalf("RunCheck: %v", err)
	}
	lines := bytes.Split(bytes.TrimRight(full, "\n"), []byte("\n"))

	dir := t.TempDir()
	base := filepath.Join(dir, "baseline.jsonl")
	if err = os.WriteFile(base, full, 0o600); err != nil {
		t.Fatalf("write baseline: %v", err)
	}
	got, exit, err := RunCheck(&CheckOptions{Root: root, Format: FormatJSON, Baseline: base})
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
	got, _, err = RunCheck(&CheckOptions{Root: root, Format: FormatJSON, Baseline: oneLine})
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

	if _, _, err := RunCheck(&CheckOptions{Root: root, Format: FormatJSON, Baseline: filepath.Join(dir, "missing.jsonl")}); err == nil {
		t.Error("an unreadable baseline should be a tool error")
	}
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

	check, exit, err := RunCheck(&CheckOptions{Root: dir, Format: FormatJSON})
	if err != nil {
		t.Fatalf("RunCheck: %v", err)
	}
	if len(check) != 0 || exit != 0 {
		t.Errorf("empty check = %q, exit %d; want empty, 0", check, exit)
	}

	cov, exit, err := RunCoverage(&CoverageOptions{Root: dir, Format: FormatJSON})
	if err != nil {
		t.Fatalf("RunCoverage: %v", err)
	}
	if want := "{\"total_concepts\":0,\"domains\":[],\"pending_mount\":[],\"orphans\":[],\"unrouted\":[]}\n"; string(cov) != want {
		t.Errorf("empty coverage = %q, want %q", cov, want)
	}
	if exit != 0 {
		t.Errorf("empty coverage exit = %d, want 0", exit)
	}

	ex, exit, err := RunExists(&ExistsOptions{Root: dir, Name: "Anything", Format: FormatJSON})
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
// file keeps only that file, and a partial segment matches nothing because a
// prefix must end at a path boundary. The distinct citing paths of the kept
// findings are compared against the exact expected set, so both over- and
// under-filtering fail.
func TestCheckPathFilter(t *testing.T) {
	t.Parallel()
	const root = "testdata/vault-report"
	tests := []struct {
		name  string
		paths []string
		want  []string
	}{
		{name: "whole japanese folder", paths: []string{"Concepts/japanese"}, want: []string{"Concepts/japanese/Kana.md"}},
		{name: "trailing slash is trimmed", paths: []string{"Concepts/japanese/"}, want: []string{"Concepts/japanese/Kana.md"}},
		{name: "exact file", paths: []string{"Concepts/golang/Bad.md"}, want: []string{"Concepts/golang/Bad.md"}},
		{name: "partial segment matches nothing", paths: []string{"Concepts/gola"}, want: []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			findings, err := check(root, tt.paths, false)
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

package judge

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

// goldens lists every recorded output under testdata/golden beside the run
// that produces it. TestGoldens compares each run with its file and
// TestRegenerateGoldens writes each run over its file, so the two read one
// table and cannot drift apart.
var goldens = []struct {
	golden  string
	produce func(t *testing.T) []byte
}{
	// The check engine over one fixture vault each.
	{"check.jsonl", checked("testdata/vault")},
	{"maps.jsonl", checked("testdata/vault-maps")},
	{"escapes.jsonl", checked("testdata/vault-escapes")},
	{"lines.jsonl", checked("testdata/vault-lines")},
	{"planned.jsonl", checked("testdata/vault-planned")},
	{"markdown-encoded.jsonl", checked("testdata/vault-markdown-encoded")},
	{"diskref.jsonl", checked("testdata/vault-diskref")},
	{"scope.jsonl", checked("testdata/vault-scope")},
	{"mapmismatch.jsonl", checked("testdata/vault-mapmismatch")},
	// One course written every way the grammar refuses, so each rule has a
	// line.
	{"paths.jsonl", checked("testdata/vault-paths")},
	// The same grammar under a vault that names its course type something
	// else, so the type comes from the rules file.
	{"course.jsonl", checked("testdata/vault-course")},
	// The same vault under a refused [navigation] table: the course findings
	// give way to the one saying the course rules could not run.
	{"navigation-unavailable.jsonl", checked("testdata/vault-navigation-unavailable")},
	{"edges.jsonl", checked("testdata/vault-edges")},
	{"fragments.jsonl", checked("testdata/vault-fragments")},
	// A block address on a callout's title and a heading after a
	// self-closing <pre/>, judged the way the page reads them.
	{"scan.jsonl", checked("testdata/vault-scan-agree")},
	// A directory the rules file spells in another case, one holding a file
	// that is not a note, and one that is not there.
	{"knowledge-scope.jsonl", checked("testdata/vault-knowledge-scope")},
	{"namecollision.jsonl", checked("testdata/vault-namecollision")},
	{"namecollision-privacy.jsonl", checked("testdata/vault-namecollision-privacy", "Private")},
	{"report.jsonl", checked("testdata/vault-report")},
	{"report-hidden.jsonl", checked("testdata/vault-report-hidden")},
	{"alias-based-on.jsonl", checked("testdata/vault-alias-based-on")},
	// A file reference inside a comment is not a live reference, the same
	// way a commented-out wikilink is not.
	{"comment-scope.jsonl", checked("testdata/vault-comment-scope")},
	{"diary.jsonl", checked("testdata/vault-diary", "Diary")},
	// A note reached only through a symbolic link.
	{"symlink.jsonl", checked("testdata/vault-symlink")},
	{"titlecollision.jsonl", checked("testdata/vault-titlecollision")},
	{"titlecollision-privacy.jsonl", checked("testdata/vault-titlecollision", "Private")},
	{"callout-title.jsonl", checked("testdata/vault-callout-title")},
	{"unused-definitions.jsonl", checked("testdata/vault-unused-definitions")},
	{"missing-images.jsonl", checked("testdata/vault-missing-images")},
	// The fixture's sealed note loses its permissions at test time, which a
	// checked-out tree cannot record.
	{"unreadable.jsonl", func(t *testing.T) []byte {
		t.Helper()
		return runCheck(t, sealedVault(t))
	}},

	// The frontmatter engine alone.
	{"schema.jsonl", schemaChecked("testdata/vault-schema")},
	{"coercion.jsonl", schemaChecked("testdata/vault-coercion")},
	{"strictness.jsonl", schemaChecked("testdata/vault-strictness")},
	{"unclosed.jsonl", schemaChecked("testdata/vault-unclosed")},
	{"reference-sequence.jsonl", schemaChecked("testdata/vault-reference-sequence")},
	{"type-dependent.jsonl", schemaChecked("testdata/vault-type-dependent")},

	// The reports a person reads.
	{"report-human.golden", reported("testdata/vault-report", FormatHuman)},
	{"report-md.golden", reported("testdata/vault-report", FormatMarkdown)},
	{"report-hidden-human.golden", reported("testdata/vault-report-hidden", FormatHuman)},

	{"coverage.golden", covered(FormatJSON)},
	{"coverage-human.golden", covered(FormatHuman)},

	// exists exits 0 when a name matches and 1 when none does, a near match
	// included.
	{"exists-shared.golden", existed("shared", FormatJSON, 0)},
	{"exists-shared-human.golden", existed("shared", FormatHuman, 0)},
	{"exists-title.golden", existed("Slices in Depth", FormatJSON, 0)},
	{"exists-missing.golden", existed("Nonexistent Concept", FormatJSON, 1)},
	{"exists-missing-human.golden", existed("Nonexistent Concept", FormatHuman, 1)},
	{"exists-near-width.golden", func(t *testing.T) []byte {
		t.Helper()
		return existsOutput(t, &ExistsOptions{Root: existsFoldVault(t), Name: existsFoldASCII, Format: FormatJSON}, 1)
	}},
}

func TestGoldens(t *testing.T) {
	t.Parallel()
	for _, tt := range goldens {
		t.Run(tt.golden, func(t *testing.T) {
			t.Parallel()
			wantGolden(t, tt.produce(t), filepath.Join("testdata", "golden", tt.golden))
		})
	}
}

// TestRegenerateGoldens rewrites every golden from its run. It skips unless
// YOMIHON_REGEN_GOLDENS is set, so an ordinary test run never rewrites what it
// compares against.
func TestRegenerateGoldens(t *testing.T) {
	if os.Getenv("YOMIHON_REGEN_GOLDENS") == "" {
		t.Skip("set YOMIHON_REGEN_GOLDENS=1 to rewrite the goldens")
	}
	for _, tt := range goldens {
		path := filepath.Join("testdata", "golden", tt.golden)
		if err := os.WriteFile(path, tt.produce(t), 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
}

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

// checked runs the check engine over fixture. A fixture without a rules file
// gets the test contract, withholding privateDirs.
func checked(fixture string, privateDirs ...string) func(t *testing.T) []byte {
	return func(t *testing.T) []byte {
		t.Helper()
		return runCheck(t, judgeFixtureRootWithPrivacy(t, fixture, privateDirs...))
	}
}

func schemaChecked(fixture string) func(t *testing.T) []byte {
	return func(t *testing.T) []byte {
		t.Helper()
		return runSchema(t, fixture)
	}
}

// reported renders the check command's report over fixture in format.
func reported(fixture string, format Format) func(t *testing.T) []byte {
	return func(t *testing.T) []byte {
		t.Helper()
		out, _, err := checkBytes(t.Context(), &CheckOptions{Root: fixture, Format: format})
		if err != nil {
			t.Fatalf("RunCheck(%q, %v): %v", fixture, format, err)
		}
		return out
	}
}

// covered renders the coverage command over the report fixture in format.
func covered(format Format) func(t *testing.T) []byte {
	return func(t *testing.T) []byte {
		t.Helper()
		out, exit, err := coverageBytes(t.Context(), &CoverageOptions{Root: "testdata/vault-report", Format: format})
		if err != nil {
			t.Fatalf("RunCoverage(%v): %v", format, err)
		}
		if exit != 0 {
			t.Errorf("RunCoverage(%v) exit = %d, want 0", format, exit)
		}
		return out
	}
}

// existed asks the exists command for name over the report fixture.
func existed(name string, format Format, wantExit int) func(t *testing.T) []byte {
	return func(t *testing.T) []byte {
		t.Helper()
		return existsOutput(t, &ExistsOptions{Root: "testdata/vault-report", Name: name, Format: format}, wantExit)
	}
}

func existsOutput(t *testing.T, o *ExistsOptions, wantExit int) []byte {
	t.Helper()
	out, exit, err := existsBytes(t.Context(), o)
	if err != nil {
		t.Fatalf("RunExists(%q): %v", o.Name, err)
	}
	if exit != wantExit {
		t.Errorf("RunExists(%q) exit = %d, want %d", o.Name, exit, wantExit)
	}
	return out
}

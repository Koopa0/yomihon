package judge

import (
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/koopa0/yomihon/internal/schema"
)

// TestLintFrontmatterAgreesWithTheCheckCommand is the whole point of the
// exported seam: the reading pages must reach the same verdict the check
// command reports, not a second reading of the same rules that drifts from it.
//
// The oracle is the command itself over a fixture vault carrying every
// schema-class rule, note by note. Comparing against a list written here
// instead would only pin what this test's author believed on the day.
func TestLintFrontmatterAgreesWithTheCheckCommand(t *testing.T) {
	t.Parallel()

	const fixture = "testdata/vault-schema"
	root := judgeFixtureRoot(t, fixture)

	contract, err := schema.Load(root)
	if err != nil {
		t.Fatalf("schema.Load() error = %v", err)
	}
	commanded, err := Check(t.Context(), root)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	want := map[string][]Finding{}
	for _, f := range commanded {
		if strings.HasPrefix(string(f.RuleID), "schema.") {
			want[f.Path] = append(want[f.Path], f)
		}
	}
	if len(want) == 0 {
		t.Fatalf("the fixture reports no schema findings, so this test would pass on any implementation")
	}

	seen := 0
	for rel, data := range fixtureNotes(t, root) {
		got, lintErr := LintFrontmatter(rel, data, contract)
		if lintErr != nil {
			t.Fatalf("LintFrontmatter(%s) error = %v", rel, lintErr)
		}
		if diff := cmp.Diff(want[rel], got); diff != "" {
			t.Errorf("LintFrontmatter(%s) disagrees with the check command (-command +seam):\n%s", rel, diff)
		}
		seen += len(got)
	}
	if seen == 0 {
		t.Error("the seam reported nothing anywhere, so agreement above proves nothing")
	}
}

// TestLintFrontmatterWithoutAContractSaysNothing covers the case the reading
// pages reach that the command never does: a folder with no contract, or one
// whose contract could not be read. There is no vocabulary to judge against,
// so there is nothing to report — and reporting nothing is different from
// reporting that the note is clean, which is the caller's distinction to make.
func TestLintFrontmatterWithoutAContractSaysNothing(t *testing.T) {
	t.Parallel()

	got, err := LintFrontmatter("Concepts/golang/Bad.md", []byte("---\nstatus: bogus\n---\n\nbody\n"), nil)
	if err != nil {
		t.Fatalf("LintFrontmatter(nil contract) error = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("LintFrontmatter(nil contract) = %v, want nothing said", got)
	}
}

// TestLintFrontmatterDomainRoots catches wrong folder depth, missing nested
// validation, invented domains outside roots, and bypassed frontmatter exits.
func TestLintFrontmatterDomainRoots(t *testing.T) {
	t.Parallel()
	type diagnostic struct {
		Rule    RuleID
		Field   string
		Target  string
		Message string
	}
	const valid = "---\ntitle: L1\ntype: lesson\nstatus: draft\ndomain: golang\nslug: l1\ncreated: 2026-06-01\nupdated: 2026-06-01\n---\n"
	for _, tt := range []struct {
		name               string
		roots              string
		path               string
		body               string
		requireFrontmatter bool
		want               []diagnostic
	}{
		{name: "nested match", roots: `["Writing/lessons"]`, path: "Writing/lessons/golang/L1.md", body: valid},
		{name: "nested mismatch", roots: `["Writing/lessons"]`, path: "Writing/lessons/golang/L1.md", body: strings.Replace(valid, "domain: golang", "domain: japanese", 1), want: []diagnostic{{"schema.domain_folder", "domain", "japanese", `domain "japanese" does not match its folder golang`}}},
		{name: "renamed mismatch", roots: `["Archive/studies"]`, path: "Archive/studies/rust/L1.md", body: valid, want: []diagnostic{{"schema.domain_folder", "domain", "golang", `domain "golang" does not match its folder rust`}}},
		{name: "first directory", roots: `["Writing/lessons"]`, path: "Writing/lessons/japanese/drills/D1.md", body: valid, want: []diagnostic{{"schema.domain_folder", "domain", "golang", `domain "golang" does not match its folder japanese`}}},
		{name: "undeclared nested", roots: `["Sources"]`, path: "Writing/lessons/golang/L1.md", body: strings.Replace(valid, "domain: golang", "domain: japanese", 1)},
		{name: "empty roots", roots: `[]`, path: "Writing/lessons/golang/L1.md", body: strings.Replace(valid, "domain: golang", "domain: japanese", 1)},
		{name: "nested direct child", roots: `["Writing/lessons"]`, path: "Writing/lessons/Overview.md", body: valid},
		{name: "top level direct child", roots: `["Writing"]`, path: "Writing/Overview.md", body: valid},
		{name: "sibling prefix", roots: `["Writing/lessons"]`, path: "Writing/lessons-extra/japanese/L1.md", body: valid},
		{name: "case distinct", roots: `["Writing/Lessons"]`, path: "Writing/lessons/japanese/L1.md", body: valid},
		{name: "normalization distinct", roots: `["Writing/Cafe\u0301"]`, path: "Writing/Caf\u00e9/japanese/L1.md", body: valid},
		{name: "no frontmatter legal", roots: `["Writing/lessons"]`, path: "Writing/lessons/golang/L1.md", body: "body\n"},
		{name: "no frontmatter required", roots: `["Writing/lessons"]`, path: "Writing/lessons/golang/L1.md", body: "body\n", requireFrontmatter: true, want: []diagnostic{{"schema.frontmatter", "", "", "frontmatter is missing"}}},
		{name: "top level no frontmatter legal", roots: `["Writing"]`, path: "Writing/japanese/L1.md", body: "body\n"},
		{name: "top level no frontmatter required", roots: `["Writing"]`, path: "Writing/japanese/L1.md", body: "body\n", requireFrontmatter: true, want: []diagnostic{{"schema.frontmatter", "", "", "frontmatter is missing"}}},
		{name: "empty frontmatter", roots: `["Writing/lessons"]`, path: "Writing/lessons/golang/L1.md", body: "---\n---\nbody\n", want: []diagnostic{{"schema.required", "title", "", "title is required"}, {"schema.required", "type", "", "type is required"}, {"schema.required", "domain", "", "domain is required"}}},
		{name: "top level empty frontmatter", roots: `["Writing"]`, path: "Writing/golang/L1.md", body: "---\n---\nbody\n", want: []diagnostic{{"schema.required", "title", "", "title is required"}, {"schema.required", "type", "", "type is required"}, {"schema.required", "domain", "", "domain is required"}}},
		{name: "missing domain", roots: `["Writing/lessons"]`, path: "Writing/lessons/golang/L1.md", body: strings.Replace(valid, "domain: golang\n", "", 1), want: []diagnostic{{"schema.required", "domain", "", "domain is required"}}},
		{name: "unclosed fence legal", roots: `["Writing/lessons"]`, path: "Writing/lessons/golang/L1.md", body: "---\ntitle: L1\ntype: lesson\nstatus: draft\n--\n\nbody\n", want: []diagnostic{{"schema.frontmatter", "", "", "frontmatter opens on line 1 and never closes"}}},
		{name: "unclosed fence required", roots: `["Writing/lessons"]`, path: "Writing/lessons/golang/L1.md", body: "---\ntitle: L1\ntype: lesson\nstatus: draft\n--\n\nbody\n", requireFrontmatter: true, want: []diagnostic{{"schema.frontmatter", "", "", "frontmatter opens on line 1 and never closes"}}},
		{name: "unclosed fence outside knowledge", roots: `["Elsewhere/studies"]`, path: "Elsewhere/studies/japanese/L1.md", body: "---\ntitle: L1\n--\n"},
		{name: "thematic break legal", roots: `["Writing/lessons"]`, path: "Writing/lessons/golang/L1.md", body: "---\n\nbody\n"},
		{name: "thematic break required", roots: `["Writing/lessons"]`, path: "Writing/lessons/golang/L1.md", body: "---\n\nbody\n", requireFrontmatter: true, want: []diagnostic{{"schema.frontmatter", "", "", "frontmatter is missing"}}},
		{name: "malformed frontmatter", roots: `["Writing/lessons"]`, path: "Writing/lessons/golang/L1.md", body: "---\ntitle: [\n---\n", want: []diagnostic{{"schema.frontmatter", "", "", "frontmatter is not valid YAML"}}},
		{name: "non scalar domain", roots: `["Writing/lessons"]`, path: "Writing/lessons/japanese/L1.md", body: strings.Replace(valid, "domain: golang", "domain: [golang]", 1)},
		{name: "skipped basename", roots: `["Writing/lessons"]`, path: "Writing/lessons/japanese/README.md", body: valid},
		{name: "outside knowledge", roots: `["Elsewhere/studies"]`, path: "Elsewhere/studies/japanese/L1.md", body: valid},
		{name: "document group", roots: `["Writing/lessons"]`, path: "Writing/lessons/japanese/L1.md", body: "---\ntype: system\ndomain: golang\nstatus: active\n---\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			text := contractFixture(t, nil,
				[2]string{`domain_equals_folder_under = ["Concepts"]`, `domain_equals_folder_under = ` + tt.roots},
				[2]string{`knowledge_dirs = ["Concepts", "Sources", "Maps", "Writing", "Synthesis", "Inbox"]`, `knowledge_dirs = ["Writing", "Archive"]`},
				[2]string{`required = ["title", "type", "domain", "status", "created", "updated"]`, `required = ["title", "type", "domain"]`},
			)
			if tt.requireFrontmatter {
				text = strings.Replace(text, "no_frontmatter_is_legal = true", "no_frontmatter_is_legal = false", 1)
			}
			write(t, root, schema.ContractRelPath, text)
			contract, err := schema.Load(root)
			if err != nil {
				t.Fatalf("schema.Load() error = %v", err)
			}
			findings, err := LintFrontmatter(tt.path, []byte(tt.body), contract)
			if err != nil {
				t.Fatalf("LintFrontmatter() error = %v", err)
			}
			var got []diagnostic
			for _, f := range findings {
				d := diagnostic{Rule: f.RuleID, Message: f.Message}
				if f.Field != nil {
					d.Field = *f.Field
				}
				if f.Target != nil {
					d.Target = *f.Target
				}
				got = append(got, d)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("LintFrontmatter() diagnostics mismatch (-want +got):\n%s", diff)
			}
			// Agreement supplements the literal oracle above; it cannot replace it.
			write(t, root, tt.path, tt.body)
			commanded, err := Check(t.Context(), root)
			if err != nil {
				t.Fatalf("Check() error = %v", err)
			}
			var fromCommand []Finding
			for _, f := range commanded {
				if f.Path == tt.path && strings.HasPrefix(string(f.RuleID), "schema.") {
					fromCommand = append(fromCommand, f)
				}
			}
			if diff := cmp.Diff(findings, fromCommand); diff != "" {
				t.Errorf("Check() disagrees with LintFrontmatter (-seam +command):\n%s", diff)
			}
		})
	}
}

// fixtureNotes reads every markdown file under a fixture root, keyed by its
// slash-form path relative to that root.
func fixtureNotes(tb testing.TB, root string) map[string][]byte {
	tb.Helper()
	var rels []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || !strings.HasSuffix(path, ".md") {
			return walkErr
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		rels = append(rels, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		tb.Fatalf("walk the fixture: %v", err)
	}
	notes := make(map[string][]byte, len(rels))
	for _, rel := range rels {
		data, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel))) // #nosec G304 -- a path this test just walked under its own fixture
		if readErr != nil {
			tb.Fatalf("read %s: %v", rel, readErr)
		}
		notes[rel] = data
	}
	return notes
}

// TestFrontmatterOnlyParseAgreesWithTheFullParse holds the parse the frontmatter
// lint uses to the parse the check command uses. Over every fixture vault under
// testdata, found on disk so that a vault added later is held to it without
// anyone remembering to list it, a note read for its frontmatter alone must come
// out as the note read whole, less the fields extracted from the body, and the
// frontmatter lint must say what the whole parse makes the rules say.
func TestFrontmatterOnlyParseAgreesWithTheFullParse(t *testing.T) {
	t.Parallel()

	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatalf("ReadDir(testdata) error = %v", err)
	}
	// Every field the body extraction fills. A field added to note and filled
	// there is not listed, so it shows as a difference until someone decides
	// whether the frontmatter lint reads it.
	bodyFields := cmpopts.IgnoreFields(note{}, "body", "wikilinks", "pathRefs", "plannedNames", "calloutTitles",
		"sectionAnchors", "excerptSectionAnchors", "blockAddresses", "sequence")
	vaults, linted, findings := 0, 0, 0
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "vault") {
			continue
		}
		vaults++
		root := judgeFixtureRoot(t, filepath.Join("testdata", entry.Name()))
		contract, loadErr := schema.Load(root)
		if loadErr != nil {
			t.Fatalf("schema.Load(%s) error = %v", entry.Name(), loadErr)
		}
		for rel, data := range fixtureNotes(t, root) {
			whole := parseNote(rel, data)
			wantFindings, checkErr := checkSchema([]note{whole}, contract)
			if checkErr != nil {
				t.Fatalf("checkSchema(%s/%s) error = %v", entry.Name(), rel, checkErr)
			}
			sortFindings(wantFindings)
			gotFindings, lintErr := LintFrontmatter(rel, data, contract)
			if lintErr != nil {
				t.Fatalf("LintFrontmatter(%s/%s) error = %v", entry.Name(), rel, lintErr)
			}
			if diff := cmp.Diff(wantFindings, gotFindings); diff != "" {
				t.Errorf("LintFrontmatter(%s/%s) differs from the whole parse's findings (-whole +frontmatter only):\n%s", entry.Name(), rel, diff)
			}

			if diff := cmp.Diff(whole, parseFrontmatter(rel, data), cmp.AllowUnexported(note{}, fmValue{}), bodyFields); diff != "" {
				t.Errorf("parseFrontmatter(%s/%s) differs from the whole parse outside its body fields (-whole +frontmatter only):\n%s", entry.Name(), rel, diff)
			}
			linted++
			findings += len(gotFindings)
		}
	}
	if vaults == 0 || linted == 0 || findings == 0 {
		t.Errorf("compared %d notes in %d fixture vaults, with %d findings: nothing here would catch a divergence", linted, vaults, findings)
	}
}

// lintProbePath and lintProbeFrontmatter are the note the cost locks measure: a
// concept, so the slug rules, which only a lesson meets, are not part of what
// each note costs.
const (
	lintProbePath        = "Concepts/golang/Probe.md"
	lintProbeFrontmatter = "---\ntitle: Probe\ntype: concept\ndomain: golang\nstatus: seed\n---\n"
)

// lintContract loads the loader's own fixture contract with each substitution
// applied, as contractFixture does, so a lock can vary one line of it.
func lintContract(tb testing.TB, replacements ...[2]string) *schema.Contract {
	tb.Helper()
	root := tb.TempDir()
	write(tb, root, schema.ContractRelPath, contractFixture(tb, nil, replacements...))
	contract, err := schema.Load(root)
	if err != nil {
		tb.Fatalf("schema.Load() error = %v", err)
	}
	return contract
}

// denseBody is a body of at least size bytes in which every few lines hand the
// body extraction something to do: wikilinks with and without a fragment, a
// path reference, a block address, headings, a fence, a callout and a planned
// name.
func denseBody(size int) string {
	const unit = "## Heading\n\nSee [[Goroutine]], [[Channel#Part]] and [a link](Concepts/golang/X.md), `code` ^anchor\n\n```go\nfunc f() {}\n```\n\n> [!note] Title\n> a planned [[Name]] (planned)\n\n"
	return strings.Repeat(unit, size/len(unit)+1)
}

// TestLintFrontmatterCostDoesNotGrowWithTheBody holds the frontmatter lint to
// reading the frontmatter. Its verdict never depends on the body, so a note of
// 64 KB dense with links, headings and fences must cost what a note of 1 KB
// does: a body extracted and then thrown away is the cost this guards. The
// whole parse is measured on the same two notes, so the bodies are known to be
// ones that extraction costs more on.
func TestLintFrontmatterCostDoesNotGrowWithTheBody(t *testing.T) {
	// Not parallel: testing.AllocsPerRun is unreliable while other tests run.
	contract := lintContract(t)
	small := []byte(lintProbeFrontmatter + denseBody(1<<10))
	large := []byte(lintProbeFrontmatter + denseBody(64<<10))

	lintAllocs := func(data []byte) float64 {
		return testing.AllocsPerRun(20, func() {
			if _, err := LintFrontmatter(lintProbePath, data, contract); err != nil {
				t.Fatalf("LintFrontmatter() error = %v", err)
			}
		})
	}
	smallLint, largeLint := lintAllocs(small), lintAllocs(large)
	if smallLint == 0 || math.Abs(largeLint-smallLint) >= smallLint/10 {
		t.Errorf("LintFrontmatter allocates %v for a 1 KB body and %v for a 64 KB body, want the two within 10%%", smallLint, largeLint)
	}

	parseAllocs := func(data []byte) float64 {
		return testing.AllocsPerRun(1, func() { _ = parseNote(lintProbePath, data) })
	}
	if smallParse, largeParse := parseAllocs(small), parseAllocs(large); largeParse < 2*smallParse {
		t.Errorf("the whole parse allocates %v for a 1 KB body and %v for a 64 KB body: these bodies would not show a lint that extracted them", smallParse, largeParse)
	}
}

// TestFrontmatterLinterResolvesTheContractOnce holds the contract the rules read
// to being resolved once for the notes a linter judges, not once per note. The
// slug pattern here is a long alternation, so compiling it costs several times
// what linting a note does, and a Lint that compiled it would cost about what a
// one-shot LintFrontmatter does, which resolves the contract every call.
func TestFrontmatterLinterResolvesTheContractOnce(t *testing.T) {
	// Not parallel: testing.AllocsPerRun is unreliable while other tests run.
	words := make([]string, 300)
	for i := range words {
		words[i] = "word" + strconv.Itoa(i)
	}
	contract := lintContract(t, [2]string{
		`slug_pattern = "^[a-z0-9]+(-[a-z0-9]+)*$"`,
		`slug_pattern = "^(` + strings.Join(words, "|") + `)$"`,
	})
	data := []byte(lintProbeFrontmatter + "body\n")
	lint, err := NewFrontmatterLinter(contract)
	if err != nil {
		t.Fatalf("NewFrontmatterLinter() error = %v", err)
	}

	perNote := testing.AllocsPerRun(20, func() { _ = lint.Lint(lintProbePath, data) })
	oneShot := testing.AllocsPerRun(20, func() {
		if _, err := LintFrontmatter(lintProbePath, data, contract); err != nil {
			t.Fatalf("LintFrontmatter() error = %v", err)
		}
	})
	if perNote*2 > oneShot {
		t.Errorf("Lint allocates %v per note and LintFrontmatter %v, want Lint under half of it", perNote, oneShot)
	}
}

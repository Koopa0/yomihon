package judge

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
)

// TestLintFrontmatterWithoutAContractSaysNothing covers the case the reading
// pages reach that the command never does: a folder with no contract, or one
// whose contract could not be read. There is no vocabulary to judge against,
// so there is nothing to report — and reporting nothing is different from
// reporting that the note is clean, which is the caller's distinction to make.
func TestLintFrontmatterWithoutAContractSaysNothing(t *testing.T) {
	t.Parallel()

	got := lintFrontmatter("Concepts/golang/Bad.md", []byte("---\ntype: bogus\n---\n\nbody\n"), nil)
	if len(got) != 0 {
		t.Errorf("lintFrontmatter(nil contract) = %v, want nothing said", got)
	}
}

// TestLintFrontmatterDomainRoots holds how the lint reads a note's domain: off
// the first folder under a declared root, reported against the note when the
// rules file's domain vocabulary does not name it, and not at all outside a
// declared root or directly under one.
func TestLintFrontmatterDomainRoots(t *testing.T) {
	t.Parallel()
	const valid = "---\ntype: lesson\nslug: l1\n---\n# L1\n"
	rust := []string{"schema.enum domain rust"}
	for _, tt := range []struct {
		name  string
		roots string
		path  string
		want  []string
	}{
		{name: "nested match", roots: `["Writing/lessons"]`, path: "Writing/lessons/golang/L1.md"},
		{name: "nested mismatch", roots: `["Writing/lessons"]`, path: "Writing/lessons/rust/L1.md", want: rust},
		{name: "renamed mismatch", roots: `["Archive/studies"]`, path: "Archive/studies/rust/L1.md", want: rust},
		{name: "first directory", roots: `["Writing/lessons"]`, path: "Writing/lessons/rust/drills/D1.md", want: rust},
		{name: "undeclared nested", roots: `["Sources"]`, path: "Writing/lessons/rust/L1.md"},
		{name: "nested direct child", roots: `["Writing/lessons"]`, path: "Writing/lessons/Overview.md"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			contract := lintContract(t,
				[2]string{`domain_from = ["Concepts"]`, `domain_from = ` + tt.roots},
				[2]string{`checked = ["Concepts", "Sources", "Maps", "Writing", "Synthesis", "Inbox"]`, `checked = ["Writing", "Archive"]`},
				[2]string{`domain = ["golang", "japanese", "meta"]`, `domain = ["golang", "japanese"]`},
				[2]string{`required = ["type", "created"]`, `required = ["type"]`},
			)
			var got []string
			for _, f := range lintFrontmatter(tt.path, []byte(valid), contract) {
				got = append(got, string(f.RuleID)+" "+*f.Field+" "+*f.Target)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("lintFrontmatter(%q) findings (-want +got):\n%s", tt.path, diff)
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

// TestFrontmatterOnlyParseAgreesWithTheFullParse holds the frontmatter lint,
// which reads a note's frontmatter alone, to what the rules say over the note
// read whole, across every fixture vault under testdata.
func TestFrontmatterOnlyParseAgreesWithTheFullParse(t *testing.T) {
	t.Parallel()

	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatalf("ReadDir(testdata) error = %v", err)
	}
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
			wantFindings := checkSchema([]note{whole}, contract)
			sortFindings(wantFindings)
			gotFindings := lintFrontmatter(rel, data, contract)
			if diff := cmp.Diff(wantFindings, gotFindings); diff != "" {
				t.Errorf("lintFrontmatter(%s/%s) differs from the whole parse's findings (-whole +frontmatter only):\n%s", entry.Name(), rel, diff)
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
	lintProbeFrontmatter = "---\ntype: concept\ncreated: 2026-01-01\nbased_on: \"[[Source]]\"\n---\n"
)

// lintContract loads the loader's own fixture contract with each substitution
// applied, as contractFixture does, so a test can vary one line of it.
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
		return testing.AllocsPerRun(20, func() { _ = lintFrontmatter(lintProbePath, data, contract) })
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

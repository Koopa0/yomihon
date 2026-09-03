package judge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

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
		if strings.HasPrefix(f.RuleID, "schema.") {
			want[f.Path] = append(want[f.Path], f)
		}
	}
	if len(want) == 0 {
		t.Fatalf("the fixture reports no schema findings, so this test would pass on any implementation")
	}

	var notes []string
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || !strings.HasSuffix(path, ".md") {
			return walkErr
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		notes = append(notes, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("walk the fixture: %v", err)
	}

	seen := 0
	for _, rel := range notes {
		data, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel))) // #nosec G304 -- a path this test just walked under its own fixture
		if readErr != nil {
			t.Fatalf("read %s: %v", rel, readErr)
		}
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

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
		if strings.HasPrefix(string(f.RuleID), "schema.") {
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

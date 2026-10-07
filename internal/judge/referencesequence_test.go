package judge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
)

// TestReferenceSequenceGolden keeps one repair per field, including the
// product-owned reference and the contract-owned replacement reference.
func TestReferenceSequenceGolden(t *testing.T) {
	t.Parallel()
	got := runSchema(t, "testdata/vault-reference-sequence")
	t.Log("hit: reference sequence schema producer reached")
	wantGolden(t, got, "testdata/golden/reference-sequence.jsonl")
}

func TestReferenceSequenceShapes(t *testing.T) {
	t.Parallel()
	contract, err := schema.Load("testdata/vault-reference-sequence")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name   string
		fields string
		want   []string
	}{
		{name: "flow", fields: "based_on: [[Note]]\n", want: []string{"based_on"}},
		{name: "block", fields: "based_on:\n  - [Note]\n", want: []string{"based_on"}},
		{name: "mixed", fields: "based_on: [\"[[Good]]\", [Bad]]\n", want: []string{"based_on"}},
		{name: "several nested items once", fields: "based_on: [[One], [Two], [[Three]]]\n", want: []string{"based_on"}},
		{name: "both fixed references", fields: "based_on: [[One]]\nrelated: [[Two]]\n", want: []string{"based_on", "related"}},
		{name: "general replacement", fields: "lineage: [[Note]]\n", want: []string{"lineage"}},
		{name: "alias item", fields: "source_locator: &nested [Note]\nbased_on: [*nested]\n", want: []string{"based_on"}},
		{name: "alias outer list", fields: "source_locator: &nested [[Note]]\nbased_on: *nested\n", want: []string{"based_on"}},
		{name: "quoted flow", fields: "based_on: [\"[[Note]]\", \"Note\"]\n"},
		{name: "quoted scalar", fields: "based_on: \"[[Note]]\"\n"},
		{name: "quoted block", fields: "based_on:\n  - '[[Note]]'\n"},
		{name: "flat list", fields: "based_on: [One, Two]\n"},
		{name: "mapping item", fields: "based_on: [{name: Note}]\n"},
		{name: "locator excluded", fields: "source_locator: [[Note]]\n"},
		{name: "lesson fields in general note excluded", fields: "predecessor: [[One]]\nsuccessors: [[Two]]\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data := []byte("---\ntitle: Probe\ntype: concept\nstatus: draft\n" + tt.fields + "---\n\nReadable body.\n")
			findings, lintErr := LintFrontmatter("Concepts/Probe.md", data, contract)
			if lintErr != nil {
				t.Fatal(lintErr)
			}
			t.Log("hit: reference shape linter reached")
			var got []string
			for _, finding := range findings {
				if finding.RuleID == "schema.reference_nested_sequence" {
					if finding.Field == nil {
						t.Fatal("caught: reference finding has no field")
					}
					got = append(got, *finding.Field)
				}
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("caught: reference sequence fields (-want +got):\n%s", diff)
			}
		})
	}
}

func TestReferenceSequenceCommandPrivacyAndDeny(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name     string
		path     string
		deny     []string
		wantExit int
		wantRows int
	}{
		{name: "public ungated", path: "Concepts/Probe.md", wantRows: 2},
		{name: "public severity gate", path: "Concepts/Probe.md", deny: []string{"error"}, wantExit: 1, wantRows: 2},
		{name: "public rule gate", path: "Concepts/Probe.md", deny: []string{"schema.reference_nested_sequence"}, wantExit: 1, wantRows: 2},
		{name: "private even with all", path: "Concepts/Private/Secret.md"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			clean := filepath.Join(root, "Concepts", "Clean.md")
			if err := os.MkdirAll(filepath.Dir(clean), 0o750); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(clean, []byte("---\ntitle: Clean\ntype: memo\nstatus: draft\n---\n\nBody.\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			for _, file := range []struct{ source, destination string }{
				{source: "System/schemas/vault-schema.toml", destination: "System/schemas/vault-schema.toml"},
				{source: "Concepts/Probe.md", destination: tt.path},
			} {
				data, readErr := os.ReadFile(filepath.Join("testdata", "vault-reference-sequence", file.source)) // #nosec G304 -- fixed isolated fixture sources
				if readErr != nil {
					t.Fatal(readErr)
				}
				if tt.wantRows == 0 && file.source == "System/schemas/vault-schema.toml" {
					const privateDeclaration = `never_egress_dirs = ["Private"]`
					if strings.Count(string(data), privateDeclaration) != 1 {
						t.Fatal("private fixture has no unique privacy declaration")
					}
					data = []byte(strings.Replace(string(data), privateDeclaration, `never_egress_dirs = ["Concepts/Private"]`, 1))
				}
				full := filepath.Join(root, filepath.FromSlash(file.destination))
				if mkdirErr := os.MkdirAll(filepath.Dir(full), 0o750); mkdirErr != nil {
					t.Fatal(mkdirErr)
				}
				if writeErr := os.WriteFile(full, data, 0o600); writeErr != nil { // #nosec G703 -- fixed fixture paths below t.TempDir
					t.Fatal(writeErr)
				}
			}
			if tt.wantRows == 0 {
				wantPrivateReferenceFindings(t, root, tt.path)
			}
			output, exit, runErr := RunCheck(t.Context(), &CheckOptions{Root: root, All: true, Deny: tt.deny, Format: FormatJSON})
			if runErr != nil {
				t.Fatalf("caught: reference rule command refused: %v", runErr)
			}
			t.Log("hit: reference sequence command reached")
			if exit != tt.wantExit {
				t.Errorf("caught: reference command exit = %d, want %d", exit, tt.wantExit)
			}
			if tt.wantRows == 0 {
				if len(output) != 0 {
					t.Errorf("caught: private reference egress = %q, want empty", output)
				}
			} else {
				wantGolden(t, output, "testdata/golden/reference-sequence.jsonl")
			}
		})
	}
}

func wantPrivateReferenceFindings(t *testing.T, root, path string) {
	t.Helper()
	contract, err := schema.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join("testdata", "vault-reference-sequence", "Concepts", "Probe.md"))
	if err != nil {
		t.Fatal(err)
	}
	findings, err := LintFrontmatter(path, data, contract)
	if err != nil {
		t.Fatal(err)
	}
	var fields []string
	for _, finding := range findings {
		if finding.RuleID == "schema.reference_nested_sequence" && finding.Field != nil {
			fields = append(fields, *finding.Field)
		}
	}
	if diff := cmp.Diff([]string{"based_on", "lineage"}, fields); diff != "" {
		t.Fatalf("caught: private fixture did not produce reference findings (-want +got):\n%s", diff)
	}
}

func TestReferenceSequenceContractSelection(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name         string
		replacements [][2]string
		noteType     string
		fields       string
		want         []string
	}{
		{name: "lesson fields", noteType: "lesson", fields: "predecessor: [[Old]]\nsuccessors: [[New]]\nlineage: [[Ignored]]\n", want: []string{"predecessor / vault-schema.toml#supersession", "successors / vault-schema.toml#supersession"}},
		{name: "renamed general field", replacements: [][2]string{{"lineage", "replaces"}}, noteType: "memo", fields: "replaces: [[New]]\nlineage: [[Ignored]]\n", want: []string{"replaces / vault-schema.toml#supersession"}},
		{name: "renamed lesson fields", replacements: [][2]string{{`"predecessor"`, `"earlier"`}, {`"successors"`, `"later"`}}, noteType: "lesson", fields: "earlier: [[Old]]\nlater: [[New]]\npredecessor: [[Ignored]]\n", want: []string{"earlier / vault-schema.toml#supersession", "later / vault-schema.toml#supersession"}},
		{name: "unknown type has no status group", noteType: "unknown", fields: "lineage: [[Ignored]]\nbased_on: [[Source]]\n", want: []string{"based_on / yomihon"}},
		{name: "fixed field configured too", replacements: [][2]string{{"general_link_field = \"lineage\"", "general_link_field = \"based_on\""}}, noteType: "concept", fields: "based_on: [[Source]]\n", want: []string{"based_on / yomihon"}},
		{name: "provenance requirement does not declare a reference", replacements: [][2]string{{"concept_requires_provenance = [\"based_on\", \"source_locator\"]", "concept_requires_provenance = [\"aliases\"]"}}, noteType: "concept", fields: "aliases: [[Ignored]]\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data, err := os.ReadFile("testdata/vault-reference-sequence/System/schemas/vault-schema.toml")
			if err != nil {
				t.Fatal(err)
			}
			declaration := string(data)
			for _, replacement := range tt.replacements {
				if !strings.Contains(declaration, replacement[0]) {
					t.Fatalf("contract replacement %q matched no declaration", replacement[0])
				}
				declaration = strings.ReplaceAll(declaration, replacement[0], replacement[1])
			}
			root, err := os.OpenRoot(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if closeErr := root.Close(); closeErr != nil {
					t.Error(closeErr)
				}
			})
			if err := root.WriteFile("contract.toml", []byte(declaration), 0o600); err != nil {
				t.Fatal(err)
			}
			contract, err := schema.LoadFile(filepath.Join(root.Name(), "contract.toml"))
			if err != nil {
				t.Fatal(err)
			}
			findings, err := LintFrontmatter("Concepts/Probe.md", []byte("---\ntitle: Probe\ntype: "+tt.noteType+"\nstatus: draft\n"+tt.fields+"---\n\nBody.\n"), contract)
			if err != nil {
				t.Fatal(err)
			}
			t.Log("hit: contract reference selector reached")
			var got []string
			for _, finding := range findings {
				if finding.RuleID == "schema.reference_nested_sequence" {
					if finding.Field == nil {
						t.Fatal("caught: configured reference finding has no field")
					}
					got = append(got, *finding.Field+" / "+finding.SourceRule)
				}
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("caught: contract reference fields (-want +got):\n%s", diff)
			}
		})
	}
}

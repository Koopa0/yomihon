package judge

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
)

// Invalid type leaves the conditional fields undecidable; it does not make
// their declared names unknown or suppress genuinely unknown fields.
func TestTypeDependentFrontmatter(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name        string
		frontmatter string
		want        []string
	}{
		{name: "invalid lesson", frontmatter: "type: Lesson\nlevel: fundamental\nslug: invalid-type\n", want: []string{"schema.enum:type:Lesson", "schema.type_dependent:type:Lesson"}},
		{name: "one deferred key", frontmatter: "type: Lesson\nslug: invalid-type\n", want: []string{"schema.enum:type:Lesson", "schema.type_dependent:type:Lesson"}},
		{name: "reordered keys", frontmatter: "slug: invalid-type\nlevel: fundamental\ntype: Lesson\n", want: []string{"schema.enum:type:Lesson", "schema.type_dependent:type:Lesson"}},
		{name: "genuine unknown", frontmatter: "type: Lesson\nlevel: fundamental\nslug: invalid-type\nextra: yes\n", want: []string{"schema.enum:type:Lesson", "schema.unknown_key::extra", "schema.type_dependent:type:Lesson"}},
		{name: "no dependent fields", frontmatter: "type: Lesson\n", want: []string{"schema.enum:type:Lesson"}},
		{name: "unknown without dependent fields", frontmatter: "type: Lesson\nextra: yes\n", want: []string{"schema.enum:type:Lesson", "schema.unknown_key::extra"}},
		{name: "valid lesson", frontmatter: "type: lesson\nlevel: fundamental\nslug: valid-lesson\n"},
		{name: "valid other type", frontmatter: "type: memo\nlevel: fundamental\nslug: valid-lesson\n", want: []string{"schema.unknown_key::level", "schema.unknown_key::slug"}},
		{name: "missing type", frontmatter: "slug: valid-lesson\n", want: []string{"schema.required:type:", "schema.unknown_key::slug"}},
		{name: "blank type", frontmatter: "type: ''\nslug: valid-lesson\n", want: []string{"schema.required:type:", "schema.unknown_key::slug"}},
		{name: "list type", frontmatter: "type: [Lesson]\nslug: valid-lesson\n", want: []string{"schema.unknown_key::slug"}},
		{name: "numeric scalar", frontmatter: "type: 7\nslug: invalid-type\n", want: []string{"schema.enum:type:7", "schema.type_dependent:type:7"}},
		{name: "boolean scalar", frontmatter: "type: true\nslug: invalid-type\n", want: []string{"schema.enum:type:true", "schema.type_dependent:type:true"}},
		{name: "NFC declared type", frontmatter: "type: cafe\u0301\nslug: valid-lesson\n", want: []string{"schema.unknown_key::slug"}},
		{name: "independent unknown title", frontmatter: "title: Probe\ntype: Lesson\nslug: invalid-type\n", want: []string{"schema.enum:type:Lesson", "schema.unknown_key::title", "schema.type_dependent:type:Lesson"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			contract := typeDependentContract(t, root, "")
			data := []byte("---\n" + tt.frontmatter + "---\n\n# Probe\n\nBody.\n")
			write(t, root, "Notes/Probe.md", string(data))
			linted := LintFrontmatter("Notes/Probe.md", data, contract)
			linter := NewFrontmatterLinter(contract)
			checked, err := Check(t.Context(), root)
			if err != nil {
				t.Fatalf("Check() error = %v", err)
			}
			for _, entry := range []struct {
				name     string
				findings []Finding
			}{
				{name: "lint", findings: linted},
				{name: "reused linter", findings: linter.Lint("Notes/Probe.md", data)},
				{name: "registered check", findings: checked},
			} {
				var got []string
				for _, f := range entry.findings {
					if !strings.HasPrefix(string(f.RuleID), "schema.") {
						continue
					}
					field, target := "", ""
					if f.Field != nil {
						field = *f.Field
					}
					if f.Target != nil {
						target = *f.Target
					}
					got = append(got, string(f.RuleID)+":"+field+":"+target)
				}
				slices.Sort(got)
				want := slices.Clone(tt.want)
				slices.Sort(want)
				if diff := cmp.Diff(want, got); diff != "" {
					t.Errorf("caught: %s type-dependent findings mismatch (-want +got):\n%s", entry.name, diff)
				}
			}
		})
	}
}

// The isolated wire fixture adds two records without changing an existing
// fixture's bytes or deriving an expected record from the judge.
func TestTypeDependentGolden(t *testing.T) {
	t.Parallel()
	const root = "testdata/vault-type-dependent"
	want, err := os.ReadFile("testdata/golden/type-dependent.jsonl")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if got := runSchema(t, root); !bytes.Equal(want, got) {
		t.Errorf("caught: type-dependent schema bytes mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
	for _, deny := range [][]string{{"error"}, {"schema.type_dependent"}} {
		got, exit, runErr := RunCheck(t.Context(), &CheckOptions{Root: root, Format: FormatJSON, Deny: deny})
		if runErr != nil {
			t.Fatalf("RunCheck() error = %v", runErr)
		}
		if !bytes.Equal(want, got) || exit != 1 {
			t.Errorf("caught: RunCheck(%v) = (%s, %d), want (%s, 1)", deny, got, exit, want)
		}
	}
}

// A private path keeps the new reason behind the same publication boundary
// as every other finding, including an explicitly requested all scope.
func TestTypeDependentPrivacy(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	typeDependentContract(t, root, "")
	data, err := os.ReadFile("testdata/vault-type-dependent/System/schemas/vault-schema.toml")
	if err != nil {
		t.Fatalf("read fixture contract: %v", err)
	}
	const privateDeclaration = `never_egress_dirs = ["Private"]`
	if strings.Count(string(data), privateDeclaration) != 1 {
		t.Fatal("private fixture has no unique privacy declaration")
	}
	write(t, root, schema.ContractRelPath, strings.Replace(string(data), privateDeclaration, `never_egress_dirs = ["Notes/Private"]`, 1))
	contract, err := schema.Load(root)
	if err != nil {
		t.Fatalf("load private fixture contract: %v", err)
	}
	write(t, root, "Notes/Clean.md", "---\ntype: memo\n---\n")
	const privatePath = "Notes/Private/Secret.md"
	const body = "---\ntype: Lesson\nslug: secret\n---\n"
	write(t, root, privatePath, body)
	findings := LintFrontmatter(privatePath, []byte(body), contract)
	var rules []string
	for _, finding := range findings {
		rules = append(rules, string(finding.RuleID))
	}
	if diff := cmp.Diff([]string{"schema.enum", "schema.type_dependent"}, rules); diff != "" {
		t.Fatalf("caught: private fixture did not produce the type-dependent verdict (-want +got):\n%s", diff)
	}
	for _, all := range []bool{false, true} {
		output, exit, err := RunCheck(t.Context(), &CheckOptions{Root: root, Format: FormatJSON, All: all})
		if err != nil {
			t.Fatalf("RunCheck() error = %v", err)
		}
		if len(output) != 0 || exit != 0 {
			t.Errorf("caught: private type-dependent finding published: %q, exit %d; want empty output and exit 0", output, exit)
		}
	}
}

func TestTypeDependentContractBoundaries(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	contract := typeDependentContract(t, root, "slug only")
	got := LintFrontmatter("Notes/Probe.md", []byte("---\ntype: Lesson\nlevel: x\n---\n"), contract)
	var rules []string
	for _, finding := range got {
		rules = append(rules, string(finding.RuleID))
	}
	if diff := cmp.Diff([]string{"schema.enum", "schema.unknown_key"}, rules); diff != "" {
		t.Errorf("caught: a field no type lists was deferred rather than called unknown (-want +got):\n%s", diff)
	}
}

// Conditional names come from the contract, including names added there;
// independently declared enum rules continue judging their values.
func TestTypeDependentAdditionalDeclaredFields(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	data, err := os.ReadFile("testdata/vault-type-dependent/System/schemas/vault-schema.toml")
	if err != nil {
		t.Fatalf("read contract: %v", err)
	}
	if strings.Count(string(data), `fields = ["level", "slug"]`) != 1 || strings.Count(string(data), "[enums]\n") != 1 {
		t.Fatal("fixture contract has no unique lesson field list or enum table")
	}
	declaration := strings.Replace(string(data), `fields = ["level", "slug"]`, `fields = ["level", "slug", "verified_on"]`, 1)
	declaration = strings.Replace(declaration, "[enums]\n", "[enums]\nlevel = [\"fundamental\"]\n", 1)
	write(t, root, schema.ContractRelPath, declaration)
	contract, err := schema.Load(root)
	if err != nil {
		t.Fatalf("schema.Load() error = %v", err)
	}
	body := []byte("---\ntype: Lesson\nslug: x\nlevel: x\nverified_on: x\n---\n")
	findings := LintFrontmatter("Notes/Probe.md", body, contract)
	var got []string
	for _, finding := range findings {
		field := ""
		if finding.Field != nil {
			field = *finding.Field
		}
		got = append(got, string(finding.RuleID)+":"+field)
	}
	want := []string{"schema.enum:type", "schema.enum:level", "schema.type_dependent:type"}
	slices.Sort(got)
	slices.Sort(want)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("caught: additional type-dependent fields mismatch (-want +got):\n%s", diff)
	}
}

func typeDependentContract(t *testing.T, root, mode string) *schema.Contract {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "vault-type-dependent", schema.ContractRelPath))
	if err != nil {
		t.Fatalf("read contract: %v", err)
	}
	if mode == "slug only" {
		if strings.Count(string(data), `fields = ["level", "slug"]`) != 1 {
			t.Fatal("fixture contract has no unique lesson field list")
		}
		data = []byte(strings.Replace(string(data), `fields = ["level", "slug"]`, `fields = ["slug"]`, 1))
	}
	write(t, root, schema.ContractRelPath, string(data))
	contract, err := schema.Load(root)
	if err != nil {
		t.Fatalf("schema.Load() error = %v", err)
	}
	return contract
}

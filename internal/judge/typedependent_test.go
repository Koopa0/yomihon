package judge

import (
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
	contract := typeDependentContract(t)
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
			data := []byte("---\n" + tt.frontmatter + "---\n\n# Probe\n\nBody.\n")
			var got []string
			for _, f := range lintFrontmatter("Notes/Probe.md", data, contract) {
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
				t.Errorf("lintFrontmatter() type-dependent findings mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestTypeDependentContractBoundaries(t *testing.T) {
	t.Parallel()
	contract := typeDependentContract(t, [2]string{`fields = ["level", "slug"]`, `fields = ["slug"]`})
	var rules []string
	for _, finding := range lintFrontmatter("Notes/Probe.md", []byte("---\ntype: Lesson\nlevel: x\n---\n"), contract) {
		rules = append(rules, string(finding.RuleID))
	}
	if diff := cmp.Diff([]string{"schema.enum", "schema.unknown_key"}, rules); diff != "" {
		t.Errorf("a field no type lists was deferred rather than called unknown (-want +got):\n%s", diff)
	}
}

// Conditional names come from the contract, including names added there;
// independently declared enum rules continue judging their values.
func TestTypeDependentAdditionalDeclaredFields(t *testing.T) {
	t.Parallel()
	contract := typeDependentContract(t,
		[2]string{`fields = ["level", "slug"]`, `fields = ["level", "slug", "verified_on"]`},
		[2]string{"[enums]\n", "[enums]\nlevel = [\"fundamental\"]\n"},
	)
	body := []byte("---\ntype: Lesson\nslug: x\nlevel: x\nverified_on: x\n---\n")
	var got []string
	for _, finding := range lintFrontmatter("Notes/Probe.md", body, contract) {
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
		t.Errorf("additional type-dependent fields mismatch (-want +got):\n%s", diff)
	}
}

// typeDependentContract loads the type-dependent fixture's rules file with
// each replacement applied once.
func typeDependentContract(t *testing.T, replacements ...[2]string) *schema.Contract {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "vault-type-dependent", schema.ContractRelPath))
	if err != nil {
		t.Fatalf("read contract: %v", err)
	}
	text := string(data)
	for _, r := range replacements {
		if strings.Count(text, r[0]) != 1 {
			t.Fatalf("the fixture contract does not hold %q exactly once", r[0])
		}
		text = strings.Replace(text, r[0], r[1], 1)
	}
	root := t.TempDir()
	write(t, root, schema.ContractRelPath, text)
	contract, err := schema.Load(root)
	if err != nil {
		t.Fatalf("schema.Load() error = %v", err)
	}
	return contract
}

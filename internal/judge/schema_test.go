package judge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
)

// TestDeclaredOptionalEnums holds the distinction between an undeclared
// vocabulary and a declared vocabulary that rejects the note's value. Type
// retains its own rule when every optional vocabulary is open.
func TestDeclaredOptionalEnums(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		declaration string
		frontmatter string
		want        []string
	}{
		{name: "level/omitted", frontmatter: "type: memo\nlevel: book\n"},
		{name: "level/empty", declaration: "level = []\n", frontmatter: "type: memo\nlevel: book\n"},
		{name: "level/matching", declaration: "level = [\"book\"]\n", frontmatter: "type: memo\nlevel: book\n"},
		{name: "level/mismatching", declaration: "level = [\"course\"]\n", frontmatter: "type: memo\nlevel: book\n", want: []string{"schema.enum level book"}},
		{name: "type remains dedicated", frontmatter: "type: unknown\n", want: []string{"schema.enum type unknown"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			write(t, root, schema.ContractRelPath, memoContract(tt.declaration))
			contract, err := schema.Load(root)
			if err != nil {
				t.Fatalf("schema.Load() error = %v", err)
			}
			if diff := cmp.Diff(tt.want, findingKeys(lintFrontmatter("Notes/Probe.md", []byte("---\n"+tt.frontmatter+"---\nBody.\n"), contract))); diff != "" {
				t.Errorf("lintFrontmatter() findings (-want +got):\n%s", diff)
			}
		})
	}
}

// TestUnmatchedKnowledgeDirIsVisibleInTheDefaultScope holds the finding on the
// surface a caller actually reads. A directory that was renamed, or a contract
// copied from another vault, reaches nothing, and the finding announcing it is
// filed against the directory rather than the contract file, since System/
// paths are dropped from every run that did not ask for them.
func TestUnmatchedKnowledgeDirIsVisibleInTheDefaultScope(t *testing.T) {
	t.Parallel()

	root := judgeFixtureRoot(t, "testdata/vault-knowledge-scope")
	for _, all := range []bool{false, true} {
		findings, err := runCheckAction(t.Context(), root, nil, all)
		if err != nil {
			t.Fatalf("check(all=%v): %v", all, err)
		}
		var reported []string
		for i := range findings {
			if findings[i].RuleID == "schema.unmatched_knowledge_dir" {
				reported = append(reported, findings[i].Path)
			}
		}
		if diff := cmp.Diff([]string{"Sources"}, reported); diff != "" {
			t.Errorf("check(all=%v) unmatched directories mismatch (-want +got):\n%s", all, diff)
		}
	}
}

// TestAnEmptyKnowledgeDirIsNotAnUnmatchedOne holds the line between a
// declaration that reaches nothing and one whose folder is simply empty, as an
// inbox whose notes have all been processed is. A fixture cannot carry an empty
// directory, so this builds the vault it needs.
func TestAnEmptyKnowledgeDirIsNotAnUnmatchedOne(t *testing.T) {
	t.Parallel()

	contract, err := os.ReadFile(filepath.Join("testdata", "vault-knowledge-scope", filepath.FromSlash(schema.ContractRelPath)))
	if err != nil {
		t.Fatalf("ReadFile(contract) error = %v", err)
	}
	const declared = `checked = ["notes", "archive", "Sources"]`
	text := strings.Replace(string(contract), declared, `checked = ["Notes", "Inbox"]`, 1)
	if text == string(contract) {
		t.Fatalf("fixture drift: the contract no longer declares %s", declared)
	}
	root := t.TempDir()
	write(t, root, schema.ContractRelPath, text)
	write(t, root, "Notes/kept.md", "---\ntype: note\n---\n# Kept\n")
	if mkdirErr := os.MkdirAll(filepath.Join(root, "Inbox"), 0o750); mkdirErr != nil {
		t.Fatalf("MkdirAll(Inbox) error = %v", mkdirErr)
	}

	findings, err := runCheckAction(t.Context(), root, nil, false)
	if err != nil {
		t.Fatalf("check(): %v", err)
	}
	for i := range findings {
		if findings[i].RuleID == "schema.unmatched_knowledge_dir" {
			t.Errorf("check() reported %q as unmatched; its directory is there and empty", findings[i].Path)
		}
	}
}

func TestLintArticleLanguage(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, schema.ContractRelPath, contractFixture(t, nil))
	run := newLintRun(loadTestAuthority(t, root).contract)
	tests := []struct {
		name string
		yaml string
		want int
	}{
		{name: "missing is allowed", yaml: "type: note", want: 0},
		{name: "invalid", yaml: "type: note\nlang: not_a_tag", want: 1},
		{name: "non-string", yaml: "type: note\nlang: [ja]", want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			n := parseNote("Writing/T.md", []byte("---\n"+tt.yaml+"\n---\n"))
			got := run.articleLanguage(&n)
			if len(got) != tt.want {
				t.Fatalf("articleLanguage() = %#v, want %d finding(s)", got, tt.want)
			}
			if tt.want == 1 && (got[0].RuleID != "schema.language" || got[0].Field == nil || *got[0].Field != "lang") {
				t.Errorf("finding = %#v, want schema.language on lang", got[0])
			}
		})
	}
}

// TestALanguageThatIsNotTextIsNotCalledEmpty asserts a lang value YAML reads as
// something other than text is told so, rather than told it is empty.
func TestALanguageThatIsNotTextIsNotCalledEmpty(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, schema.ContractRelPath, contractFixture(t, nil))
	run := newLintRun(loadTestAuthority(t, root).contract)
	// reason is the finding's message without the value it quotes.
	reason := func(value string) string {
		n := parseNote("Writing/T.md", []byte("---\ntype: note\nlang: "+value+"\n---\n"))
		got := run.articleLanguage(&n)
		if len(got) != 1 {
			t.Fatalf("articleLanguage(lang: %q) = %d findings, want 1", value, len(got))
		}
		return strings.Replace(got[0].Message, ` "`+value+`"`, "", 1)
	}
	if empty, date := reason(""), reason("2024-01-01"); empty == date {
		t.Errorf("an empty lang and a date are given the same reason: %q", date)
	}
}

// TestTheProvenanceRuleFollowsTheContractsOwnConceptType holds the frontmatter
// rules to the vocabulary the contract layer owns: the rule that asks a
// distilled idea where it came from applies only where the contract declares a
// concept type, and an undeclared type draws its own finding instead.
func TestTheProvenanceRuleFollowsTheContractsOwnConceptType(t *testing.T) {
	t.Parallel()

	const body = "---\ntype: concept\ncreated: 2026-01-01\n---\n# Idea\n\nbody\n"
	const relPath = "Concepts/golang/Idea.md"

	tests := []struct {
		name         string
		replacements [][2]string
		want         []string
	}{
		{
			name: "a vault whose contract declares a concept corpus",
			want: []string{"schema.provenance"},
		},
		{
			name:         "a vault whose contract declares none",
			replacements: [][2]string{{"[types.concept]\n", ""}},
			want:         []string{"schema.enum"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			write(t, root, schema.ContractRelPath, contractFixture(t, nil, tt.replacements...))
			contract := loadTestAuthority(t, root).contract

			findings := lintFrontmatter(relPath, []byte(body), contract)
			var rules []string
			for _, f := range findings {
				rules = append(rules, string(f.RuleID))
			}
			if diff := cmp.Diff(tt.want, rules); diff != "" {
				t.Errorf("rules reported (-want +got):\n%s", diff)
			}
		})
	}
}

// TestScalarTypeKeepsItsRouting preserves the scalar type used to route the
// type's own fields, even when graph metadata does not accept its shape: a type
// written 1 or true, quoted or not, is the declared type of that name, so the
// field it lists is allowed and the field it requires is asked for. An
// undeclared scalar is the control.
func TestScalarTypeKeepsItsRouting(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, schema.ContractRelPath, contractFixture(t, nil, [2]string{
		"[types.inbox]\n",
		"[types.\"1\"]\nfields = [\"verified_on\"]\nrequired = [\"verified_on\"]\n\n" +
			"[types.\"true\"]\nfields = [\"verified_on\"]\nrequired = [\"verified_on\"]\n\n[types.inbox]\n",
	}))
	contract := loadTestAuthority(t, root).contract
	for _, tc := range []struct {
		name, kind, fields string
		want               []string
	}{
		{name: "numeric declared type", kind: "1", fields: "verified_on: 2026-01-01\n"},
		{name: "boolean declared type", kind: "true", fields: "verified_on: 2026-01-01\n"},
		{name: "quoted numeric declared type", kind: "'1'", fields: "verified_on: 2026-01-01\n"},
		{name: "quoted boolean declared type", kind: "'true'", fields: "verified_on: 2026-01-01\n"},
		{name: "numeric type missing its field", kind: "1", want: []string{"schema.required verified_on"}},
		{name: "quoted boolean type missing its field", kind: "'true'", want: []string{"schema.required verified_on"}},
		{
			name: "undeclared numeric type", kind: "2", fields: "verified_on: 2026-01-01\n",
			want: []string{"schema.enum type 2", "schema.type_dependent type 2"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := "---\ntype: " + tc.kind + "\ncreated: 2026-01-01\n" + tc.fields + "---\nBody.\n"
			got := lintFrontmatter("Writing/Scalar.md", []byte(body), contract)
			if diff := cmp.Diff(tc.want, findingKeys(got)); diff != "" {
				t.Errorf("lintFrontmatter(type %s) findings (-want +got):\n%s", tc.kind, diff)
			}
		})
	}
}

// TestOptionalEnumFoldsTheSpelling holds an optional vocabulary to one answer
// per declared word: a note's value spelled in another Unicode form names the
// word the contract lists. A word the contract never declares is the control,
// and its finding carries the note's own bytes.
func TestOptionalEnumFoldsTheSpelling(t *testing.T) {
	t.Parallel()

	const (
		composed   = "\u304c\u3044\u306d\u3093"
		decomposed = "\u304b\u3099\u3044\u306d\u3093"
		other      = "\u304f\u3099\u3093"
	)
	root := t.TempDir()
	write(t, root, schema.ContractRelPath, memoContract(`level = ["`+composed+`"]`+"\n"))
	contract, err := schema.Load(root)
	if err != nil {
		t.Fatalf("schema.Load() error = %v", err)
	}
	for _, tt := range []struct {
		name  string
		value string
		want  []string
	}{
		{name: "the declared spelling", value: composed},
		{name: "the decomposed spelling", value: decomposed},
		{name: "a word the contract never declared", value: other, want: []string{"schema.enum level " + other}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data := []byte("---\ntype: memo\nlevel: " + tt.value + "\n---\nBody.\n")
			if diff := cmp.Diff(tt.want, findingKeys(lintFrontmatter("Notes/Probe.md", data, contract))); diff != "" {
				t.Errorf("lintFrontmatter(level %q) findings (-want +got):\n%s", tt.value, diff)
			}
		})
	}
}

// TestATypeWrittenDecomposedIsTheDeclaredType asserts a note that writes its
// type in decomposed form, as a Mac keyboard hands it over, is judged as the
// type the contract declares: it takes that type's required fields, and a unit
// type's slug requirement.
func TestATypeWrittenDecomposedIsTheDeclaredType(t *testing.T) {
	t.Parallel()

	const (
		composed   = "\u304c\u3044\u306d\u3093"
		decomposed = "\u304b\u3099\u3044\u306d\u3093"
	)
	if composed == decomposed || schema.NormalizeWord(decomposed) != composed {
		t.Fatalf("the fixture spellings are not one word in two forms: %q and %q", composed, decomposed)
	}
	type fieldFinding struct{ Rule, Field string }
	tests := []struct {
		name     string
		contract string
		want     []fieldFinding
	}{
		{name: "a type's required field", contract: typeRequiredContract(composed), want: []fieldFinding{{"schema.required", "level"}}},
		{name: "a unit type's slug", contract: unitTypeContract(composed), want: []fieldFinding{{"schema.required", "slug"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			write(t, root, schema.ContractRelPath, tt.contract)
			contract, err := schema.Load(root)
			if err != nil {
				t.Fatalf("schema.Load() error = %v", err)
			}
			data := []byte("---\ntype: " + decomposed + "\n---\nBody.\n")
			var got []fieldFinding
			for _, f := range NewFrontmatterLinter(contract).Lint("Notes/Probe.md", data) {
				field := ""
				if f.Field != nil {
					field = *f.Field
				}
				got = append(got, fieldFinding{string(f.RuleID), field})
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Lint(type %q) findings mismatch (-want +got):\n%s", decomposed, diff)
			}
		})
	}
}

// memoContract declares one memo type over the Notes folder, with type and
// level as its fields and enums as the whole of its [enums] table.
func memoContract(enums string) string {
	return `[folders]
checked = ["Notes"]
templates = []

[enums]
` + enums + `
[fields]
common = ["type", "level"]
required = ["type"]

[types.memo]

[privacy]
never_egress_dirs = []
`
}

// typeRequiredContract declares a type that requires level, naming the type
// the way the caller asks, beside a memo that does not.
// unitTypeContract declares declared as the type holding the unit role.
func unitTypeContract(declared string) string {
	return `[folders]
checked = ["Notes"]
templates = []

[enums]

[fields]
common = ["type"]
required = ["type"]

[types."` + declared + `"]
role = "lesson"
fields = ["slug"]

[privacy]
never_egress_dirs = []
`
}

func typeRequiredContract(declared string) string {
	return `[folders]
checked = ["Notes"]
templates = []

[enums]

[fields]
common = ["type"]
required = ["type"]

[types.memo]

[types."` + declared + `"]
fields = ["level"]
required = ["level"]

[privacy]
never_egress_dirs = []
`
}

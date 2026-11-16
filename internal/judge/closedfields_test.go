package judge

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
)

// TestDraftMustBeABoolean holds schema.draft: the flag is the boolean true or
// false and nothing else. The key is the product's own, so a contract that does
// not list it in fields.common still has it checked — and still allows it.
func TestDraftMustBeABoolean(t *testing.T) {
	t.Parallel()

	const common = `common = ["type", "aliases", "draft", "lang", "created", "based_on"]`
	contract := lintContract(t, [2]string{common, `common = ["type", "aliases", "lang", "created", "based_on"]`})
	for _, tt := range []struct {
		name  string
		draft string
		want  []string
	}{
		{name: "true", draft: "draft: true\n"},
		{name: "false", draft: "draft: false\n"},
		{name: "quoted yes", draft: "draft: \"yes\"\n", want: []string{`schema.draft draft "yes" must be true or false target=yes`}},
		{name: "quoted true", draft: "draft: \"true\"\n", want: []string{`schema.draft draft "true" must be true or false target=true`}},
		{name: "list", draft: "draft: [true]\n", want: []string{"schema.draft draft must be true or false"}},
		{name: "empty", draft: "draft:\n", want: []string{"schema.draft draft must be true or false"}},
		{name: "absent", draft: ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			body := "---\ntype: note\ncreated: 2026-01-01\n" + tt.draft + "---\n# Probe\n"
			got := LintFrontmatter("Writing/Probe.md", []byte(body), contract)
			if diff := cmp.Diff(tt.want, describeFindings(got)); diff != "" {
				t.Errorf("LintFrontmatter(%q) mismatch (-want +got):\n%s", tt.draft, diff)
			}
		})
	}
}

// TestTheFieldListIsClosed holds schema.unknown_key to the list yomihon
// defines and the contract hands out. The derived names and the retired
// workflow fields are unknown wherever they are written; the type, the draft
// flag and the names Obsidian reserves need no declaration; a field one type
// lists is unknown on another declared type and deferred on an undeclared one;
// and the concept type carries the provenance fields the concept rule names.
func TestTheFieldListIsClosed(t *testing.T) {
	t.Parallel()

	contract := lintContract(t, [2]string{
		`common = ["type", "aliases", "draft", "lang", "created", "based_on"]`,
		`common = ["type", "created"]`,
	})
	unknown := func(key string) string {
		return `schema.unknown_key frontmatter "` + key + `" is not a known field target=` + key
	}
	for _, tt := range []struct {
		name   string
		path   string
		fields string
		want   []string
	}{
		{
			name:   "derived and retired names",
			path:   "Writing/Probe.md",
			fields: "type: note\ntitle: Probe\ndomain: golang\nstatus: draft\n",
			want:   []string{unknown("domain"), unknown("status"), unknown("title")},
		},
		{
			name:   "built-in and reserved names",
			path:   "Writing/Probe.md",
			fields: "type: note\naliases: [Other]\ntags: [a]\ncssclasses: [wide]\ndraft: true\n",
		},
		{
			name:   "a lesson's field on another declared type",
			path:   "Writing/Probe.md",
			fields: "type: note\nslug: probe\n",
			want:   []string{unknown("slug")},
		},
		{
			name:   "a lesson's field on an undeclared type",
			path:   "Writing/Probe.md",
			fields: "type: Lessen\nslug: probe\n",
			want: []string{
				`schema.enum type "Lessen" is not an allowed type target=Lessen`,
				`schema.type_dependent type "Lessen" is not valid, so type-only fields cannot be judged until type is valid target=Lessen`,
			},
		},
		{
			name:   "a concept's provenance field",
			path:   "Concepts/golang/Probe.md",
			fields: "type: concept\nsource_locator: p. 1\n",
		},
		{
			name:   "a provenance field off the concept type",
			path:   "Writing/Probe.md",
			fields: "type: note\nsource_locator: p. 1\n",
			want:   []string{unknown("source_locator")},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			body := "---\n" + tt.fields + "created: 2026-01-01\n---\n# Probe\n"
			got := LintFrontmatter(tt.path, []byte(body), contract)
			if diff := cmp.Diff(tt.want, describeFindings(got)); diff != "" {
				t.Errorf("LintFrontmatter() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestTheDomainIsReadOffTheFolder holds the derived domain: the first folder
// below any root folders.domain_from names, nothing for a file directly under a
// root or outside every root. A note never writes it, and the vocabulary
// [enums] states for it is checked against what the folder says — only when
// the contract states one.
func TestTheDomainIsReadOffTheFolder(t *testing.T) {
	t.Parallel()

	const roots = `domain_from = ["Concepts"]`
	notes := map[string]string{
		"Concepts/golang/A.md":           "golang",
		"Writing/lessons/japanese/B.md":  "japanese",
		"Writing/lessons/rust/deep/C.md": "rust",
		"Concepts/Direct.md":             "",
		"Writing/lessons/Overview.md":    "",
		"Maps/M.md":                      "",
	}
	vaultWith := func(t *testing.T, replacements ...[2]string) string {
		t.Helper()
		root := t.TempDir()
		replacements = append(replacements, [2]string{roots, `domain_from = ["Concepts", "Writing/lessons"]`})
		write(t, root, schema.ContractRelPath, contractFixture(t, nil, replacements...))
		for path := range notes {
			write(t, root, path, "---\ntype: note\ncreated: 2026-01-01\n---\n# Note\n")
		}
		return root
	}

	t.Run("derived", func(t *testing.T) {
		t.Parallel()
		collected, err := collectNotes(t.Context(), vaultWith(t))
		if err != nil {
			t.Fatalf("collectNotes() error = %v", err)
		}
		got := map[string]string{}
		for i := range collected {
			got[collected[i].path] = collected[i].domain
		}
		if diff := cmp.Diff(notes, got); diff != "" {
			t.Errorf("note domains (-want +got):\n%s", diff)
		}
	})

	enumFindings := func(t *testing.T, root string) []string {
		t.Helper()
		findings, err := Check(t.Context(), root)
		if err != nil {
			t.Fatalf("Check() error = %v", err)
		}
		var got []string
		for i := range findings {
			if findings[i].RuleID == "schema.enum" {
				got = append(got, findings[i].Path+" "+findings[i].Message)
			}
		}
		return got
	}

	t.Run("declared vocabulary", func(t *testing.T) {
		t.Parallel()
		got := enumFindings(t, vaultWith(t))
		want := []string{`Writing/lessons/rust/deep/C.md domain "rust" is not an allowed value`}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("domain enum findings (-want +got):\n%s", diff)
		}
	})

	t.Run("no vocabulary", func(t *testing.T) {
		t.Parallel()
		got := enumFindings(t, vaultWith(t, [2]string{`domain = ["golang", "japanese", "meta"]` + "\n", ""}))
		if len(got) != 0 {
			t.Errorf("domain enum findings without a declared vocabulary = %q, want none", got)
		}
	})
}

// aliasCitationContract is the alias-resolution vault: plain notes, and a
// thought whose based_on is the citation the rule reads.
const aliasCitationContract = `schema_version = "2"

[folders]
checked = ["Notes", "Thoughts", "Private"]
templates = []

[enums]

[fields]
common = ["type", "aliases", "based_on"]
required = ["type"]

[types.note]

[types.thought]
role = "thought"
fields = ["based_on"]

[privacy]
never_egress_dirs = ["Private"]
`

// TestAThoughtCitingAnAliasIsTold holds provenance.alias_resolved. A thought's
// based_on that reaches its note only through an alias is told so; one that
// names a file, an alias that repeats the file's own name, or an alias two
// notes share is not, and a name nothing answers to stays
// provenance.unresolved. The section and display text are taken off before the
// name is read. Only a thought is asked.
func TestAThoughtCitingAnAliasIsTold(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name     string
		noteType string
		cite     string
		want     []string
	}{
		{name: "an alias", noteType: "thought", cite: `"[[Old Name]]"`, want: []string{"provenance.alias_resolved [[Old Name]] -> Notes/Successor.md"}},
		{name: "a bare alias", noteType: "thought", cite: `Old Name`, want: []string{"provenance.alias_resolved Old Name -> Notes/Successor.md"}},
		{name: "an alias with a section and display text", noteType: "thought", cite: `"[[Old Name#Part|the words]]"`, want: []string{"provenance.alias_resolved [[Old Name#Part|the words]] -> Notes/Successor.md"}},
		{name: "an alias cited by a note that is not a thought", noteType: "note", cite: `"[[Old Name]]"`},
		{name: "a file's own name", noteType: "thought", cite: `"[[Plain]]"`},
		{name: "a path to a file", noteType: "thought", cite: `"[[Notes/Plain]]"`},
		{name: "an alias repeating the file's own name", noteType: "thought", cite: `"[[Echo]]"`},
		{name: "an alias two notes share", noteType: "thought", cite: `"[[Twin]]"`},
		{name: "a name nothing answers to", noteType: "thought", cite: `"[[Nowhere]]"`, want: []string{"provenance.unresolved [[Nowhere]] -> "}},
		{name: "an alias of a withheld note", noteType: "thought", cite: `"[[Hidden Name]]"`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			write(t, root, schema.ContractRelPath, aliasCitationContract)
			write(t, root, "Notes/Successor.md", "---\ntype: note\naliases: [Old Name]\n---\n# Successor\n")
			write(t, root, "Notes/Plain.md", "---\ntype: note\n---\n# Plain\n")
			write(t, root, "Notes/Echo.md", "---\ntype: note\naliases: [Echo]\n---\n# Echo\n")
			write(t, root, "Notes/Left.md", "---\ntype: note\naliases: [Twin]\n---\n# Left\n")
			write(t, root, "Notes/Right.md", "---\ntype: note\naliases: [Twin]\n---\n# Right\n")
			write(t, root, "Private/Kept.md", "---\ntype: note\naliases: [Hidden Name]\n---\n# Kept\n")
			write(t, root, "Thoughts/Reaction.md", "---\ntype: "+tt.noteType+"\nbased_on: "+tt.cite+"\n---\n# Reaction\n")

			out, _, err := RunCheck(t.Context(), &CheckOptions{Root: root, All: true, Format: FormatJSON})
			if err != nil {
				t.Fatalf("RunCheck() error = %v", err)
			}
			if bytes.Contains(out, []byte("Private/")) {
				t.Errorf("check named the withheld note:\n%s", out)
			}
			var got []string
			for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
				if line == "" {
					continue
				}
				var f struct {
					RuleID     string  `json:"rule_id"`
					Path       string  `json:"path"`
					Target     *string `json:"target"`
					ResolvedTo *string `json:"resolved_to"`
				}
				if err := json.Unmarshal([]byte(line), &f); err != nil {
					t.Fatalf("decode %q: %v", line, err)
				}
				if f.Path != "Thoughts/Reaction.md" || !strings.HasPrefix(f.RuleID, "provenance.") {
					continue
				}
				entry := f.RuleID + " " + *f.Target + " -> "
				if f.ResolvedTo != nil {
					entry += *f.ResolvedTo
				}
				got = append(got, entry)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("provenance findings on the citing note (-want +got):\n%s", diff)
			}
		})
	}
}

// TestWithheldDirectoriesStillHoldUnderTheContract is the privacy invariant
// under the closed-field contract: a directory never_egress_dirs names stays
// withheld from check, coverage and exists. No finding names a note there by
// its path, its collision membership or what it resolved to, and exists says
// only that a withheld note answers to the name.
func TestWithheldDirectoriesStillHoldUnderTheContract(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		fixture string
		private string
		// held is a name only a withheld note answers to.
		held string
		// rule is one the fixture reports from its public notes, so the run
		// is shown to judge the vault rather than to say nothing at all.
		rule RuleID
	}{
		{fixture: "testdata/vault-namecollision-privacy", private: "Private", held: "journal", rule: "collision.name"},
		{fixture: "testdata/vault-diary", private: "Diary", held: "2026-07-01", rule: "link.broken"},
	} {
		t.Run(tt.private, func(t *testing.T) {
			t.Parallel()
			root := judgeFixtureRootWithPrivacy(t, tt.fixture, tt.private)
			prefix := tt.private + "/"
			if loadTestAuthority(t, root).egressAllowed(prefix + "probe.md") {
				t.Fatalf("the contract written for %s does not withhold %s", tt.fixture, prefix)
			}

			for _, all := range []bool{false, true} {
				findings, err := runCheckAction(t.Context(), root, nil, all)
				if err != nil {
					t.Fatalf("check(all=%v) error = %v", all, err)
				}
				if _, ok := findingByRule(findings, tt.rule); !ok {
					t.Errorf("check(all=%v) reported no %s, so its silence about %s proves nothing", all, tt.rule, prefix)
				}
				for i := range findings {
					f := &findings[i]
					named := strings.HasPrefix(f.Path, prefix) || f.ResolvedTo != nil && strings.HasPrefix(*f.ResolvedTo, prefix)
					for _, member := range f.CollisionMembers {
						named = named || strings.HasPrefix(member, prefix)
					}
					if named {
						t.Errorf("check(all=%v) named a withheld note: %+v", all, *f)
					}
				}
			}
			for _, format := range []Format{FormatJSON, FormatHuman, FormatMarkdown} {
				out, _, err := RunCheck(t.Context(), &CheckOptions{Root: root, All: true, Format: format})
				if err != nil {
					t.Fatalf("RunCheck(%d) error = %v", format, err)
				}
				if bytes.Contains(out, []byte(prefix)) {
					t.Errorf("RunCheck(%d) named a withheld note:\n%s", format, out)
				}
				out, _, err = RunCoverage(t.Context(), &CoverageOptions{Root: root, Format: format})
				if err != nil {
					t.Fatalf("RunCoverage(%d) error = %v", format, err)
				}
				if bytes.Contains(out, []byte(prefix)) {
					t.Errorf("RunCoverage(%d) named a withheld note:\n%s", format, out)
				}
			}

			out, exit, err := RunExists(t.Context(), &ExistsOptions{Root: root, Name: tt.held, Format: FormatJSON})
			if err != nil {
				t.Fatalf("RunExists(%q) error = %v", tt.held, err)
			}
			if want := `{"query":"` + tt.held + `","matches":[],"withheld":true}` + "\n"; exit != 0 || string(out) != want {
				t.Errorf("RunExists(%q) = (%q, %d), want (%q, 0)", tt.held, out, exit, want)
			}
		})
	}
}

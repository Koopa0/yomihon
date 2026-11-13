package judge

import (
	"bytes"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
)

// leftoverLockContract is the shared Lock vault for the #238 leftovers that
// need a loaded contract: one knowledge note type under Notes, no report type,
// and no field beyond type in any declared list.
const leftoverLockContract = `schema_version = "2"

[folders]
checked = ["Notes"]
templates = []

[enums]

[fields]
common = ["type"]
required = ["type"]

[types.note]

[privacy]
never_egress_dirs = []
`

func writeLeftoverLockVault(t *testing.T, contract string, notes map[string]string) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, schema.ContractRelPath, contract)
	for path, body := range notes {
		write(t, root, path, body)
	}
	return root
}

func schemaFindingMessages(findings []Finding) []string {
	var out []string
	for i := range findings {
		if strings.HasPrefix(string(findings[i].RuleID), "schema.") {
			out = append(out, string(findings[i].RuleID)+": "+findings[i].Message)
		}
	}
	return out
}

// TestMarkdownCheckReportFrontmatterTheVaultAccepts is the Lock for the
// markdown check body: a vault that does not declare type report is told so
// instead of being handed `type: report`, which its own check rejects. A vault
// that does declare report gets a block carrying the type alone — the title is
// the report's own first heading — and LintFrontmatter accepts it.
func TestMarkdownCheckReportFrontmatterTheVaultAccepts(t *testing.T) {
	t.Parallel()

	const kept = "---\ntype: note\n---\n# Kept\n\nBody.\n"

	t.Run("undeclared report is told, not filed as a failing note", func(t *testing.T) {
		t.Parallel()
		root := writeLeftoverLockVault(t, leftoverLockContract, map[string]string{"Notes/Kept.md": kept})
		out, _, err := RunCheck(t.Context(), &CheckOptions{Root: root, Format: FormatMarkdown})
		if err != nil {
			t.Fatalf("RunCheck() error = %v", err)
		}
		if bytes.Contains(out, []byte("type: report")) {
			t.Errorf("markdown report wrote type: report, which this contract does not declare:\n%s", out)
		}
		if !bytes.HasPrefix(out, []byte("This vault's contract does not accept a fileable check report, so this body is not a note.\n\n# yomihon check\n")) {
			t.Errorf("markdown report did not open by telling the reader the contract cannot host the note:\n%s", out)
		}

		contract, err := schema.Load(root)
		if err != nil {
			t.Fatalf("schema.Load() error = %v", err)
		}
		filed := "Notes/check.md"
		write(t, root, filed, string(out))
		findings := LintFrontmatter(filed, out, contract)
		if got := schemaFindingMessages(findings); len(got) != 0 {
			t.Errorf("filed markdown report produced schema findings: %v", got)
		}
	})

	t.Run("declared report writes frontmatter the vault accepts", func(t *testing.T) {
		t.Parallel()
		contractText := strings.Replace(leftoverLockContract, "[types.note]\n", "[types.note]\n\n[types.report]\n", 1)
		root := writeLeftoverLockVault(t, contractText, map[string]string{"Notes/Kept.md": kept})
		out, _, err := RunCheck(t.Context(), &CheckOptions{Root: root, Format: FormatMarkdown})
		if err != nil {
			t.Fatalf("RunCheck() error = %v", err)
		}
		if !bytes.HasPrefix(out, []byte("---\ntype: report\n---\n\n# yomihon check\n")) {
			t.Errorf("markdown report did not open with a block carrying only type: report on a vault that declares it:\n%s", out)
		}

		contract, err := schema.Load(root)
		if err != nil {
			t.Fatalf("schema.Load() error = %v", err)
		}
		filed := "Notes/check.md"
		write(t, root, filed, string(out))
		findings := LintFrontmatter(filed, out, contract)
		if got := schemaFindingMessages(findings); len(got) != 0 {
			t.Errorf("filed markdown report produced schema findings: %v", got)
		}
	})
}

// TestExistsNeverMatchesTitleEn is the Lock for exists / title_en. title_en is
// not a field yomihon defines, so no contract can declare it: a note that
// writes it draws schema.unknown_key from check, and exists must not answer to
// it. The local title and the English title are different strings, so a match
// on title_en could not be excused as a folded title hit.
func TestExistsNeverMatchesTitleEn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		contract func(t *testing.T) string
		path     string
		body     string
	}{
		{
			name:     "note",
			contract: func(*testing.T) string { return leftoverLockContract },
			path:     "Notes/Local Name.md",
			body:     "---\ntype: note\ntitle_en: English Name\n---\n# Local Name\n\nBody.\n",
		},
		{
			name: "lesson",
			contract: func(t *testing.T) string {
				t.Helper()
				return contractFixture(t, nil)
			},
			path: "Writing/lessons/golang/Local Name.md",
			body: "---\ntype: lesson\ncreated: 2026-01-01\nslug: local-name\ntitle_en: English Name\n---\n# Local Name\n\nBody.\n",
		},
		{
			name: "concept",
			contract: func(t *testing.T) string {
				t.Helper()
				return contractFixture(t, nil)
			},
			path: "Concepts/golang/Local Name.md",
			body: "---\ntype: concept\ncreated: 2026-01-01\nsource_locator: p. 1\ntitle_en: English Name\n---\n# Local Name\n\nBody.\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := writeLeftoverLockVault(t, tt.contract(t), map[string]string{tt.path: tt.body})
			out, exit, err := RunExists(t.Context(), &ExistsOptions{Root: root, Name: "English Name", Format: FormatJSON})
			if err != nil {
				t.Fatalf("RunExists() error = %v", err)
			}
			if exit != 1 {
				t.Errorf("RunExists() exit = %d, want 1 (title_en is not a field)\n%s", exit, out)
			}
			if bytes.Contains(out, []byte(`"field":"title_en"`)) {
				t.Errorf("exists matched title_en:\n%s", out)
			}

			out, exit, err = RunExists(t.Context(), &ExistsOptions{Root: root, Name: "Local Name", Format: FormatJSON})
			if err != nil {
				t.Fatalf("RunExists(Local Name) error = %v", err)
			}
			if exit != 0 || !bytes.Contains(out, []byte(`"field":"title"`)) {
				t.Errorf("RunExists(Local Name) exit = %d, want 0 with a title match from the heading:\n%s", exit, out)
			}

			contract, err := schema.Load(root)
			if err != nil {
				t.Fatalf("schema.Load() error = %v", err)
			}
			findings := LintFrontmatter(tt.path, []byte(tt.body), contract)
			want := []string{`schema.unknown_key: frontmatter "title_en" is not a known field`}
			if diff := cmp.Diff(want, schemaFindingMessages(findings)); diff != "" {
				t.Errorf("check on the note's frontmatter (-want +got):\n%s", diff)
			}
		})
	}
}

// TestArticleLanguageGateIsTheContracts is the Lock for the schema.language
// bypass: stuffing lang onto a Definition copy must not open the gate.
// ArticleLanguage() reads the contract, and this vault does not list lang in
// fields.common.
func TestArticleLanguageGateIsTheContracts(t *testing.T) {
	t.Parallel()

	root := writeLeftoverLockVault(t, leftoverLockContract, nil)
	contract := loadTestAuthority(t, root).contract
	definition := contract.Definition()
	definition.Fields.Common = append(definition.Fields.Common, "lang")
	run := newLintRun(contract)
	n := parseNote("Notes/Probe.md", []byte("---\ntype: note\nlang: not_a_tag\n---\n"))
	if got := run.articleLanguage(&n); len(got) != 0 {
		t.Errorf("articleLanguage() = %#v, want none: the Definition copy listed lang and the contract did not", got)
	}
}

package judge

import (
	"bytes"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

// leftoverLockContract is the shared Lock vault for the #238 leftovers that
// need a loaded contract: one knowledge note under Notes, no report type, no
// title_en in any declared field list, and a default note status group.
const leftoverLockContract = `schema_version = "1"

[enums]
type = ["note"]

[enums.status]
note = ["draft"]

[fields]
required = ["title", "type"]
known = ["title", "type", "status"]

[scan]
knowledge_dirs = ["Notes"]
skip_basenames = []

[navigation]
path_types = []
map_types = []

[artifacts]
non_instance_dirs = []

[privacy]
never_egress_dirs = []

[[lifecycle]]
status = "draft"
applies_to = ["*"]
from = []
owner = ["koopa"]
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
// instead of being handed `type: report` and `tool: yomihon`, which its own
// check rejects. A vault that does declare report, with required fields this
// face can fill, gets a frontmatter block LintFrontmatter accepts.
func TestMarkdownCheckReportFrontmatterTheVaultAccepts(t *testing.T) {
	t.Parallel()

	t.Run("undeclared report is told, not filed as a failing note", func(t *testing.T) {
		t.Parallel()
		root := writeLeftoverLockVault(t, leftoverLockContract, map[string]string{
			"Notes/Kept.md": "---\ntitle: Kept\ntype: note\nstatus: draft\n---\nBody.\n",
		})
		out, _, err := RunCheck(t.Context(), &CheckOptions{Root: root, Format: FormatMarkdown})
		if err != nil {
			t.Fatalf("RunCheck() error = %v", err)
		}
		if bytes.Contains(out, []byte("type: report")) {
			t.Errorf("markdown report wrote type: report, which this contract does not declare:\n%s", out)
		}
		if bytes.Contains(out, []byte("tool: yomihon")) {
			t.Errorf("markdown report wrote tool: yomihon, which fields.known does not list:\n%s", out)
		}
		if !bytes.Contains(out, []byte("does not accept a fileable check report")) {
			t.Errorf("markdown report did not tell the reader the contract cannot host the note:\n%s", out)
		}

		contract, err := schema.Load(root)
		if err != nil {
			t.Fatalf("schema.Load() error = %v", err)
		}
		filed := "Notes/check.md"
		write(t, root, filed, string(out))
		findings, err := LintFrontmatter(filed, out, contract)
		if err != nil {
			t.Fatalf("LintFrontmatter() error = %v", err)
		}
		if got := schemaFindingMessages(findings); len(got) != 0 {
			t.Errorf("filed markdown report produced schema findings: %v", got)
		}
	})

	t.Run("declared report writes frontmatter the vault accepts", func(t *testing.T) {
		t.Parallel()
		contractText := strings.Replace(leftoverLockContract, `type = ["note"]`, `type = ["note", "report"]`, 1)
		root := writeLeftoverLockVault(t, contractText, map[string]string{
			"Notes/Kept.md": "---\ntitle: Kept\ntype: note\nstatus: draft\n---\nBody.\n",
		})
		out, _, err := RunCheck(t.Context(), &CheckOptions{Root: root, Format: FormatMarkdown})
		if err != nil {
			t.Fatalf("RunCheck() error = %v", err)
		}
		if !bytes.Contains(out, []byte("type: report")) {
			t.Errorf("markdown report omitted type: report on a vault that declares it:\n%s", out)
		}
		if bytes.Contains(out, []byte("tool: yomihon")) {
			t.Errorf("markdown report wrote tool: yomihon, which fields.known does not list:\n%s", out)
		}

		contract, err := schema.Load(root)
		if err != nil {
			t.Fatalf("schema.Load() error = %v", err)
		}
		filed := "Notes/check.md"
		write(t, root, filed, string(out))
		findings, err := LintFrontmatter(filed, out, contract)
		if err != nil {
			t.Fatalf("LintFrontmatter() error = %v", err)
		}
		if got := schemaFindingMessages(findings); len(got) != 0 {
			t.Errorf("filed markdown report produced schema findings: %v", got)
		}
	})
}

// TestExistsMatchesTitleEnOnlyWhenKnown is the Lock for exists / title_en.
// The local title and the English title are different strings, so a match on
// title_en cannot be excused as a folded title hit. When the contract omits
// title_en from every declared list, the name is absent; when fields.known or
// a per-type list such as fields.lesson_only declares it, the match names it.
func TestExistsMatchesTitleEnOnlyWhenKnown(t *testing.T) {
	t.Parallel()

	const note = "---\ntitle: Local Name\ntitle_en: English Name\ntype: note\nstatus: draft\n---\nBody.\n"

	t.Run("omitted from fields.known", func(t *testing.T) {
		t.Parallel()
		root := writeLeftoverLockVault(t, leftoverLockContract, map[string]string{
			"Notes/Local Name.md": note,
		})
		out, exit, err := RunExists(t.Context(), &ExistsOptions{Root: root, Name: "English Name", Format: FormatJSON})
		if err != nil {
			t.Fatalf("RunExists() error = %v", err)
		}
		if exit != 1 {
			t.Errorf("RunExists() exit = %d, want 1 (title_en is not a known field)\n%s", exit, out)
		}
		if bytes.Contains(out, []byte(`"field":"title_en"`)) {
			t.Errorf("exists matched title_en outside fields.known:\n%s", out)
		}
	})

	t.Run("declared in fields.known", func(t *testing.T) {
		t.Parallel()
		contractText := strings.Replace(leftoverLockContract,
			`known = ["title", "type", "status"]`,
			`known = ["title", "type", "status", "title_en"]`, 1)
		root := writeLeftoverLockVault(t, contractText, map[string]string{
			"Notes/Local Name.md": note,
		})
		out, exit, err := RunExists(t.Context(), &ExistsOptions{Root: root, Name: "English Name", Format: FormatJSON})
		if err != nil {
			t.Fatalf("RunExists() error = %v", err)
		}
		if exit != 0 {
			t.Errorf("RunExists() exit = %d, want 0 (title_en is known)\n%s", exit, out)
		}
		if !bytes.Contains(out, []byte(`"field":"title_en"`)) {
			t.Errorf("exists did not match title_en when fields.known declares it:\n%s", out)
		}
	})

	t.Run("declared in fields.lesson_only", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		write(t, root, schema.ContractRelPath, contractFixture(t, nil))
		write(t, root, "Writing/lessons/golang/Local Name.md", "---\ntitle: Local Name\ntitle_en: English Name\ntype: lesson\ndomain: golang\nstatus: draft\ncreated: 2026-01-01\nupdated: 2026-01-01\nslug: local-name\n---\nBody.\n")
		out, exit, err := RunExists(t.Context(), &ExistsOptions{Root: root, Name: "English Name", Format: FormatJSON})
		if err != nil {
			t.Fatalf("RunExists() error = %v", err)
		}
		if exit != 0 {
			t.Errorf("RunExists() exit = %d, want 0 (title_en is lesson_only)\n%s", exit, out)
		}
		if !bytes.Contains(out, []byte(`"field":"title_en"`)) {
			t.Errorf("exists did not match title_en when fields.lesson_only declares it:\n%s", out)
		}
	})
}

// TestArticleLanguageGateIsTheContracts is the Lock for the schema.language
// bypass: stuffing lang onto the Definition copy that lintRun holds must not
// open the gate. ArticleLanguage() reads the contract, and this vault does not
// list lang in fields.known.
func TestArticleLanguageGateIsTheContracts(t *testing.T) {
	t.Parallel()

	root := writeLeftoverLockVault(t, leftoverLockContract, nil)
	run, err := newLintRun(loadTestAuthority(t, root).contract)
	if err != nil {
		t.Fatalf("newLintRun() error = %v", err)
	}
	run.definition.Fields.Known = append(append([]string{}, run.definition.Fields.Known...), "lang")
	n := parseNote("Notes/Probe.md", []byte("---\ntitle: Probe\ntype: note\nlang: not_a_tag\n---\n"))
	got := run.articleLanguage(&n)
	if len(got) != 0 {
		t.Errorf("articleLanguage() = %#v, want none: the Definition copy listed lang and the contract did not", got)
	}
}

// TestThirdStatusGroupKeepsKnowledgeRules is the Lock for the
// systemDocumentGroup leftover: the group name is schema.SystemDocumentGroup,
// and assigning a type to any other group does not silence knowledge-note
// validation. A template filed under "ops" still draws the required-field
// findings the system group would have waived.
func TestThirdStatusGroupKeepsKnowledgeRules(t *testing.T) {
	t.Parallel()

	if schema.SystemDocumentGroup != "system" {
		t.Fatalf("SystemDocumentGroup = %q, want %q", schema.SystemDocumentGroup, "system")
	}

	root := t.TempDir()
	write(t, root, schema.ContractRelPath, contractFixture(t, nil,
		[2]string{`system = ["active", "archived"]`, `ops = ["active", "archived"]`},
		[2]string{`system = ["system", "template", "guide"]`, `ops = ["system", "template", "guide"]`},
	))
	contract := loadTestAuthority(t, root).contract

	body := []byte("---\ntitle: Template\ntype: template\nstatus: active\n---\nbody\n")
	findings, err := LintFrontmatter("Concepts/golang/Template.md", body, contract)
	if err != nil {
		t.Fatalf("LintFrontmatter() error = %v", err)
	}
	for i := range findings {
		if findings[i].RuleID == "schema.required" {
			return
		}
	}
	t.Errorf("a type in group ops drew no knowledge-note requirement; light document rules stay on %s", schema.SystemDocumentGroup)
}

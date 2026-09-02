package judge

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
)

// TestCheckSchemaGolden drives the frontmatter engine over fixture vaults and
// asserts the emitted bytes equal each golden. The rules fixture triggers
// every schema rule at least once and exercises the four scope behaviors that
// must produce nothing: a skipped basename, a note outside the knowledge
// directories, a note with no frontmatter, and a note whose block does not
// parse (which yields exactly one fault). The coercion fixture pins how an
// unquoted scalar's value reaches a finding: booleans lowercased, integers in
// decimal, nulls dropped, and everything else — quoted text, the 1.1 boolean
// words, out-of-range numbers, reals, aliases, custom-tagged and block scalars
// (whose trailing newline is kept or stripped by their chomping) — left as the
// reference reads them. The strictness fixture pins the parse boundary: a
// repeated key (nested, in a list, flow or block), a tab indent, an invalid
// escape, and an unterminated quote each become a single parse fault; a merge
// key is read as an ordinary key; and the fence handling matches — an empty
// block reads as present-but-empty, a "..." close and a closing fence at end of
// file are both honored. Each golden holds the schema subset of the reference
// tool's sorted output over that same fixture.
func TestCheckSchemaGolden(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		fixture string
		golden  string
	}{
		{name: "rules", fixture: "testdata/vault-schema", golden: "testdata/golden/schema.jsonl"},
		{name: "scalar coercion", fixture: "testdata/vault-coercion", golden: "testdata/golden/coercion.jsonl"},
		{name: "parser strictness", fixture: "testdata/vault-strictness", golden: "testdata/golden/strictness.jsonl"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			want, err := os.ReadFile(tt.golden)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			got := runSchema(t, tt.fixture)
			if !bytes.Equal(got, want) {
				t.Errorf("schema findings differ from golden %s\ngot:\n%s\nwant:\n%s\ngot hex:\n%s\nwant hex:\n%s",
					tt.golden, got, want, hex.Dump(got), hex.Dump(want))
			}
		})
	}
}

// TestKnowledgeScopeFoldsTheDeclaredSpelling pins the repair for the quietest
// failure this face had: a contract naming "notes" where the folder is "Notes"
// turned every frontmatter rule off for that folder, and the run stayed exit 0
// with no diagnostic anywhere, because the notes the rules were written for
// were simply never examined. On a case-insensitive filesystem both spellings
// open the same folder, so the scope now folds the way the privacy and
// artifact scopes always have.
func TestKnowledgeScopeFoldsTheDeclaredSpelling(t *testing.T) {
	t.Parallel()

	got := runCheck(t, "testdata/vault-knowledge-scope")
	if !bytes.Contains(got, []byte(`"path":"Notes/bad.md"`)) {
		t.Errorf("check output = %s, want the note under the differently-spelled directory linted", got)
	}
}

// TestUnmatchedKnowledgeDirIsVisibleInTheDefaultScope holds the finding on the
// surface a caller actually reads. Folding the spelling repairs one way a
// declaration can reach nothing; a directory that was renamed, or a contract
// copied from another vault, reaches nothing in a way no fold can recover, and
// the finding announcing it would be invisible if it were filed against the
// contract file, since System/ paths are dropped from every run that did not
// ask for them.
func TestUnmatchedKnowledgeDirIsVisibleInTheDefaultScope(t *testing.T) {
	t.Parallel()

	root := judgeFixtureRoot(t, "testdata/vault-knowledge-scope")
	for _, all := range []bool{false, true} {
		findings, err := runCheckAction(root, nil, all)
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
// declaration that reaches nothing and one whose folder is simply empty. The
// live vault this was first run against declares an inbox and had processed
// everything in it, so an emptiness test would have told its owner to correct a
// contract that is right, and failed a gate over a vault with nothing wrong
// with it. A fixture cannot carry an empty directory — nothing records one —
// so this builds the vault it needs.
func TestAnEmptyKnowledgeDirIsNotAnUnmatchedOne(t *testing.T) {
	t.Parallel()

	contract, err := os.ReadFile(filepath.Join("testdata", "vault-knowledge-scope", filepath.FromSlash(schema.ContractRelPath)))
	if err != nil {
		t.Fatalf("ReadFile(contract) error = %v", err)
	}
	const declared = `knowledge_dirs = ["notes", "archive", "Sources"]`
	text := strings.Replace(string(contract), declared, `knowledge_dirs = ["Notes", "Inbox"]`, 1)
	if text == string(contract) {
		t.Fatalf("fixture drift: the contract no longer declares %s", declared)
	}
	root := t.TempDir()
	write(t, root, schema.ContractRelPath, text)
	write(t, root, "Notes/kept.md", "---\ntitle: Kept\ntype: note\nstatus: draft\n---\n")
	if mkdirErr := os.MkdirAll(filepath.Join(root, "Inbox"), 0o750); mkdirErr != nil {
		t.Fatalf("MkdirAll(Inbox) error = %v", mkdirErr)
	}

	findings, err := runCheckAction(root, nil, false)
	if err != nil {
		t.Fatalf("check(): %v", err)
	}
	for i := range findings {
		if findings[i].RuleID == "schema.unmatched_knowledge_dir" {
			t.Errorf("check() reported %q as unmatched; its directory is there and empty", findings[i].Path)
		}
	}
}

func TestLessonStatusUsesOnlyItsConfiguredGroup(t *testing.T) {
	t.Parallel()

	contract, err := schema.Load("testdata/vault-supersession")
	if err != nil {
		t.Fatalf("schema.Load() error = %v", err)
	}
	n := parseNote("Writing/Invalid.md", []byte(`---
title: Invalid
type: lesson
status: bogus
slug: invalid
---
`))
	findings, err := checkSchema([]note{n}, contract)
	if err != nil {
		t.Fatalf("checkSchema() error = %v", err)
	}

	var statusFindings []Finding
	for i := range findings {
		if findings[i].RuleID == "schema.enum" && findings[i].Field != nil && *findings[i].Field == "status" {
			statusFindings = append(statusFindings, findings[i])
		}
	}
	if len(statusFindings) != 1 {
		t.Fatalf("status findings = %v, want exactly one lesson-group finding", statusFindings)
	}
	if got, want := statusFindings[0].Message, `status "bogus" is not a valid lesson status`; got != want {
		t.Errorf("status message = %q, want %q", got, want)
	}
}

// TestSystemDocumentsTakeTheLightStatusRule pins the routing the contract's
// status groups decide: a type the contract places in the system group gets
// the light rule — its status reads against the system enum, and the full
// knowledge-note requirements never fire for it.
func TestSystemDocumentsTakeTheLightStatusRule(t *testing.T) {
	t.Parallel()

	contract, err := schema.Load("testdata/vault-schema")
	if err != nil {
		t.Fatalf("schema.Load() error = %v", err)
	}
	n := parseNote("Sources/Guide.md", []byte(`---
title: Guide
type: guide
status: bogus
---
`))
	findings, err := checkSchema([]note{n}, contract)
	if err != nil {
		t.Fatalf("checkSchema() error = %v", err)
	}

	var statusFindings []Finding
	for i := range findings {
		if findings[i].RuleID != "schema.enum" || findings[i].Field == nil {
			continue
		}
		if *findings[i].Field == "status" {
			statusFindings = append(statusFindings, findings[i])
		}
	}
	if len(statusFindings) != 1 {
		t.Fatalf("status findings = %v, want exactly one system-group finding", statusFindings)
	}
	if got, want := statusFindings[0].Message, `status "bogus" is not a valid system status`; got != want {
		t.Errorf("status message = %q, want %q", got, want)
	}
	for i := range findings {
		if findings[i].RuleID == "schema.required" {
			t.Errorf("a system document drew a knowledge-note requirement: %+v", findings[i])
		}
	}
}

// undeclaredTypeContract declares a single note type and no status groups, so
// a note written with any other type resolves to no status group at all. Its
// note statuses are draft and ready.
const undeclaredTypeContract = `schema_version = "1"

[enums]
type = ["article"]

[enums.status]
note = ["draft", "ready"]

[fields]
required = ["title", "type"]
known = ["title", "type", "status"]

[scan]
knowledge_dirs = ["Concepts"]
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
owner = ["curator"]

[[lifecycle]]
status = "ready"
applies_to = ["*"]
from = ["draft"]
owner = ["curator"]
`

// TestUndeclaredTypeStatusReadsAgainstTheNoteGroup pins the fallback for an
// in-scope note whose type the contract never declared: such a type resolves
// to no status group at all, yet its status still reads against the general
// note group. The type already carries its own finding, and a group that does
// not exist could neither name the enum to check nor the group to blame in
// the reason — so a status the note group allows draws no finding, and one it
// rejects is blamed in the note group's own words.
func TestUndeclaredTypeStatusReadsAgainstTheNoteGroup(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	write(t, root, schema.ContractRelPath, undeclaredTypeContract)
	contract, err := schema.Load(root)
	if err != nil {
		t.Fatalf("schema.Load() error = %v", err)
	}

	valid := parseNote("Concepts/Valid.md", []byte("---\ntitle: Valid\ntype: mystery\nstatus: ready\n---\n"))
	invalid := parseNote("Concepts/Invalid.md", []byte("---\ntitle: Invalid\ntype: mystery\nstatus: bogus\n---\n"))
	findings, err := checkSchema([]note{valid, invalid}, contract)
	if err != nil {
		t.Fatalf("checkSchema() error = %v", err)
	}

	var statusFindings []Finding
	for i := range findings {
		if findings[i].RuleID == "schema.enum" && findings[i].Field != nil && *findings[i].Field == "status" {
			statusFindings = append(statusFindings, findings[i])
		}
	}
	if len(statusFindings) != 1 {
		t.Fatalf("status findings = %+v, want exactly one, on the note whose status the note group rejects", statusFindings)
	}
	if got, want := statusFindings[0].Path, "Concepts/Invalid.md"; got != want {
		t.Errorf("status finding path = %q, want %q", got, want)
	}
	if got, want := statusFindings[0].Message, `status "bogus" is not a valid status`; got != want {
		t.Errorf("status message = %q, want %q", got, want)
	}
}

func TestLintArticleLanguage(t *testing.T) {
	t.Parallel()
	definition := schema.Definition{Fields: schema.Fields{Known: []string{"title", "lang"}}}
	tests := []struct {
		name string
		yaml string
		want int
	}{
		{name: "missing is allowed", yaml: "title: T", want: 0},
		{name: "valid Japanese", yaml: "title: T\nlang: ja", want: 0},
		{name: "valid canonicalizable", yaml: "title: T\nlang: zh-hant", want: 0},
		{name: "invalid", yaml: "title: T\nlang: not_a_tag", want: 1},
		{name: "non-string", yaml: "title: T\nlang: [ja]", want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			n := parseNote("Writing/T.md", []byte("---\n"+tt.yaml+"\n---\n"))
			run := &lintRun{definition: definition}
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

func TestCheckSchemaIncludesArticleLanguage(t *testing.T) {
	t.Parallel()
	contract, err := schema.Load("testdata/vault-schema")
	if err != nil {
		t.Fatalf("schema.Load() error = %v", err)
	}
	n := parseNote("Sources/Invalid-language.md", []byte("---\ntitle: Invalid language\ntype: lesson\ndomain: japanese\nstatus: draft\nslug: invalid-language\nlang: not_a_tag\n---\n"))
	findings, err := checkSchema([]note{n}, contract)
	if err != nil {
		t.Fatalf("checkSchema() error = %v", err)
	}
	var languageFindings []Finding
	for i := range findings {
		if findings[i].RuleID == "schema.language" {
			languageFindings = append(languageFindings, findings[i])
		}
	}
	if len(languageFindings) != 1 {
		t.Fatalf("schema.language findings = %#v, want exactly one", languageFindings)
	}
	if languageFindings[0].Field == nil || *languageFindings[0].Field != "lang" {
		t.Errorf("schema.language finding = %#v, want field lang", languageFindings[0])
	}
}

// runSchema collects the notes under root, runs the frontmatter engine, sorts,
// and returns the wire bytes.
func runSchema(t *testing.T, root string) []byte {
	t.Helper()
	notes, err := collectNotes(root)
	if err != nil {
		t.Fatalf("collectNotes(%q): %v", root, err)
	}
	s, err := schema.Load(root)
	if err != nil {
		t.Fatalf("schema.Load(%q): %v", root, err)
	}
	findings, err := checkSchema(notes, s)
	if err != nil {
		t.Fatalf("checkSchema: %v", err)
	}
	sortFindings(findings)
	var buf bytes.Buffer
	if err := WriteJSONL(&buf, findings); err != nil {
		t.Fatalf("WriteJSONL: %v", err)
	}
	return buf.Bytes()
}

// TestTheProvenanceRuleFollowsTheContractsOwnConceptType holds the frontmatter
// rules to the vocabulary the contract layer owns. The rule that asks a
// distilled idea where it came from is written for a corpus of them, and
// whether this vault keeps such a corpus is the contract's answer, not a word
// spelled here — the coverage face already asks it that way, and this file
// asked instead whether one note happened to be typed with the same letters.
//
// The cost of the literal is a rule that fires where there is no corpus for it
// to be about: the type already draws its own finding for being undeclared,
// and a second one demanding provenance of it holds the author to a rule their
// contract never claimed covered anything. The same reading builds the notices
// the reading page prints beside a note, so the wrong answer was shown on the
// page as well as printed by the command.
func TestTheProvenanceRuleFollowsTheContractsOwnConceptType(t *testing.T) {
	t.Parallel()

	const body = "---\ntitle: Idea\ntype: concept\ndomain: golang\n" +
		"status: seedling\ncreated: 2026-01-01\nupdated: 2026-01-01\n---\nbody\n"
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
			name: "a vault whose contract declares none",
			replacements: [][2]string{
				{`"source-note", "concept", "writing"`, `"source-note", "writing"`},
				{"[[lifecycle]]\nstatus = \"seedling\"\napplies_to = [\"concept\"]\ninitial = false\nfrom = [\"cleaned\"]\nowner = [\"hermes\", \"claude\"]\n\n", ""},
			},
			want: []string{"schema.enum"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			write(t, root, schema.ContractRelPath, contractFixture(t, nil, tt.replacements...))
			contract := loadTestAuthority(t, root).contract

			findings, err := LintFrontmatter(relPath, []byte(body), contract)
			if err != nil {
				t.Fatalf("LintFrontmatter() error = %v", err)
			}
			var rules []string
			for _, f := range findings {
				rules = append(rules, f.RuleID)
			}
			if diff := cmp.Diff(tt.want, rules); diff != "" {
				t.Errorf("rules reported (-want +got):\n%s", diff)
			}
		})
	}
}

// TestADocumentsStatusIsJudgedAgainstItsOwnGroup holds the light rule that
// templates, guides and system notes answer to. Which statuses that group
// allows is the contract's declaration, and the rule reads the group the note
// was routed by rather than naming a set of its own — the routing and the enum
// were two separate spellings of the same word, and two spellings drift.
//
// The vault's own working documents keep a short lifecycle that has nothing in
// common with the one a written note travels, so a status legal for a template
// is illegal for an essay and the reverse. A rule reading the wrong group would
// therefore be wrong in both directions at once, which is what the last case
// here varies the contract to show.
func TestADocumentsStatusIsJudgedAgainstItsOwnGroup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		status       string
		replacements [][2]string
		want         []string
	}{
		{
			name:   "a status the contract lists for documents",
			status: "active",
		},
		{
			name:   "a status the contract lists only for written notes",
			status: "draft",
			want:   []string{"schema.enum"},
		},
		{
			name:         "a vault that lists other statuses for its documents",
			status:       "active",
			replacements: [][2]string{{`system = ["active", "archived"]`, `system = ["operational", "archived"]`}},
			want:         []string{"schema.enum"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			write(t, root, schema.ContractRelPath, contractFixture(t, nil, tt.replacements...))
			contract := loadTestAuthority(t, root).contract

			body := "---\ntitle: Template\ntype: template\nstatus: " + tt.status + "\n---\nbody\n"
			findings, err := LintFrontmatter("Concepts/golang/Template.md", []byte(body), contract)
			if err != nil {
				t.Fatalf("LintFrontmatter() error = %v", err)
			}
			var rules []string
			for _, f := range findings {
				rules = append(rules, f.RuleID)
			}
			if diff := cmp.Diff(tt.want, rules); diff != "" {
				t.Errorf("rules reported (-want +got):\n%s", diff)
			}
		})
	}
}

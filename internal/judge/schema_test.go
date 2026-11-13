package judge

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"slices"
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
// reference reads them. It states a vocabulary on five of the defined fields
// (level, prerequisites, slug, verified_on, verified_with), none of which a
// value there can satisfy, so every value written reaches the golden as one
// schema.enum line whichever field carries it. The strictness fixture pins the parse boundary: a
// repeated key (nested, in a list, flow or block), a tab indent, an invalid
// escape, and an unterminated quote each become a single parse fault; a merge
// key is read as an ordinary key; and the fence handling matches — an empty
// block reads as present-but-empty, a "..." close and a closing fence at end of
// file are both honored. The unclosed fixture pins the fence that never closes:
// three notes that open a block and lose its closing line each draw one finding,
// while a block that closes and two notes that open with a thematic break draw
// none. Each golden holds the schema subset of the reference tool's sorted
// output over that same fixture.
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
		{name: "unclosed fence", fixture: "testdata/vault-unclosed", golden: "testdata/golden/unclosed.jsonl"},
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

// TestDeclaredOptionalEnums holds the distinction between an undeclared
// vocabulary and a declared vocabulary that rejects the note's value. Type
// retains its own rule when every optional vocabulary is open.
func TestDeclaredOptionalEnums(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		declaration  string
		frontmatter  string
		wantOutput   string
		wantExit     int
		wantFindings []Finding
	}{
		{
			name:        "level/omitted",
			frontmatter: "type: memo\nlevel: book\n",
		},
		{
			name:        "level/empty",
			declaration: "level = []\n",
			frontmatter: "type: memo\nlevel: book\n",
		},
		{
			name:        "level/matching",
			declaration: "level = [\"book\"]\n",
			frontmatter: "type: memo\nlevel: book\n",
		},
		{
			name:        "level/mismatching",
			declaration: "level = [\"course\"]\n",
			frontmatter: "type: memo\nlevel: book\n",
			wantOutput:  `{"rule_id":"schema.enum","severity":"error","path":"Notes/Probe.md","field":"level","message":"level \"book\" is not an allowed value","evidence":"frontmatter validated against vault-schema.toml","suggested_action":"fix the frontmatter to match the schema","source_rule":"vault-schema.toml","target":"book","fingerprint":"v1:c323b71b99e2043d"}` + "\n",
			wantExit:    1,
			wantFindings: []Finding{{
				RuleID:          "schema.enum",
				Severity:        SeverityError,
				Path:            "Notes/Probe.md",
				Field:           new("level"),
				Message:         `level "book" is not an allowed value`,
				Evidence:        "frontmatter validated against vault-schema.toml",
				SuggestedAction: "fix the frontmatter to match the schema",
				SourceRule:      "vault-schema.toml",
				Target:          new("book"),
				Fingerprint:     "v1:c323b71b99e2043d",
			}},
		},
		{
			name:        "type remains dedicated",
			frontmatter: "type: unknown\n",
			wantOutput:  `{"rule_id":"schema.enum","severity":"error","path":"Notes/Probe.md","field":"type","message":"type \"unknown\" is not an allowed type","evidence":"frontmatter validated against vault-schema.toml","suggested_action":"fix the frontmatter to match the schema","source_rule":"vault-schema.toml","target":"unknown","fingerprint":"v1:3bed4c2ace05b5f0"}` + "\n",
			wantExit:    1,
			wantFindings: []Finding{{
				RuleID:          "schema.enum",
				Severity:        SeverityError,
				Path:            "Notes/Probe.md",
				Field:           new("type"),
				Message:         `type "unknown" is not an allowed type`,
				Evidence:        "frontmatter validated against vault-schema.toml",
				SuggestedAction: "fix the frontmatter to match the schema",
				SourceRule:      "vault-schema.toml",
				Target:          new("unknown"),
				Fingerprint:     "v1:3bed4c2ace05b5f0",
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			write(t, root, schema.ContractRelPath, `schema_version = "2"
[folders]
checked = ["Notes"]
templates = []
[enums]
`+tt.declaration+`[fields]
common = ["type", "level"]
required = ["type"]
[types.memo]
[privacy]
never_egress_dirs = []
`)
			data := []byte("---\n" + tt.frontmatter + "---\nBody.\n")
			write(t, root, "Notes/Probe.md", string(data))
			contract, err := schema.Load(root)
			if err != nil {
				t.Fatalf("schema.Load() error = %v", err)
			}

			t.Run("check", func(t *testing.T) {
				t.Parallel()

				got, exit, err := RunCheck(t.Context(), &CheckOptions{
					Root:   root,
					Deny:   []string{"error"},
					Format: FormatJSON,
				})
				if err != nil {
					t.Fatalf("RunCheck() error = %v", err)
				}
				if diff := cmp.Diff(tt.wantOutput, string(got)); diff != "" {
					t.Errorf("RunCheck() output mismatch (-want +got):\n%s", diff)
				}
				if exit != tt.wantExit {
					t.Errorf("RunCheck() exit = %d, want %d", exit, tt.wantExit)
				}
			})
			t.Run("lint", func(t *testing.T) {
				t.Parallel()

				got := LintFrontmatter("Notes/Probe.md", data, contract)
				if diff := cmp.Diff(tt.wantFindings, got); diff != "" {
					t.Errorf("LintFrontmatter() findings mismatch (-want +got):\n%s", diff)
				}
			})
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

// TestEveryTypeTakesTheSameRules holds that no type is let off the knowledge
// rules. The working-document types — a guide, a template, a system note —
// once took a lighter rule that waived the required fields; the contract no
// longer states such a group, so each of them answers for a missing required
// field like any other note.
func TestEveryTypeTakesTheSameRules(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	write(t, root, schema.ContractRelPath, contractFixture(t, nil))
	contract := loadTestAuthority(t, root).contract
	for _, noteType := range []string{"guide", "template", "system", "note"} {
		findings := LintFrontmatter("Writing/Doc.md", []byte("---\ntype: "+noteType+"\n---\n# Doc\n"), contract)
		if diff := cmp.Diff([]string{"schema.required created is required"}, describeFindings(findings)); diff != "" {
			t.Errorf("LintFrontmatter(type %s) mismatch (-want +got):\n%s", noteType, diff)
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
		{name: "valid Japanese", yaml: "type: note\nlang: ja", want: 0},
		{name: "valid canonicalizable", yaml: "type: note\nlang: zh-hant", want: 0},
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

func TestCheckSchemaIncludesArticleLanguage(t *testing.T) {
	t.Parallel()
	contract, err := schema.Load("testdata/vault-schema")
	if err != nil {
		t.Fatalf("schema.Load() error = %v", err)
	}
	n := parseNote("Sources/Invalid-language.md", []byte("---\ntype: lesson\nslug: invalid-language\nlang: not_a_tag\n---\n# Invalid language\n"))
	findings := checkSchema([]note{n}, contract)
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
	notes, err := collectNotes(t.Context(), root)
	if err != nil {
		t.Fatalf("collectNotes(t.Context(), %q): %v", root, err)
	}
	s, err := schema.Load(root)
	if err != nil {
		t.Fatalf("schema.Load(%q): %v", root, err)
	}
	findings := checkSchema(notes, s)
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

			findings := LintFrontmatter(relPath, []byte(body), contract)
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
		{name: "numeric type missing its field", kind: "1", want: []string{"schema.required verified_on is required"}},
		{name: "quoted boolean type missing its field", kind: "'true'", want: []string{"schema.required verified_on is required"}},
		{
			name: "undeclared numeric type", kind: "2", fields: "verified_on: 2026-01-01\n",
			want: []string{
				`schema.enum type "2" is not an allowed type target=2`,
				`schema.type_dependent type "2" is not valid, so type-only fields cannot be judged until type is valid target=2`,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := "---\ntype: " + tc.kind + "\ncreated: 2026-01-01\n" + tc.fields + "---\nBody.\n"
			got := LintFrontmatter("Writing/Scalar.md", []byte(body), contract)
			if diff := cmp.Diff(tc.want, describeFindings(got)); diff != "" {
				t.Errorf("caught: scalar type=%s (-want +got):\n%s", tc.kind, diff)
			}
		})
	}
}

// TestOptionalEnumFoldsTheSpelling holds every optional vocabulary to one
// answer per declared word. A contract is written in an editor and a note's
// frontmatter is read back off a filesystem, and the two hand over different
// byte strings for the same word, so a membership test on the bytes can call a
// note's value undeclared while it names exactly what the contract lists. The
// contract folds its own values as it loads, which left the note's value as the
// one unfolded side: a contract and a note carrying identical decomposed bytes
// disagreed, because only one of them had been folded.
//
// Both directions are covered, because a contract can be the decomposed side as
// easily as a note can. A word the contract never declares is the control that
// keeps the folding from becoming a blanket yes, and the finding it produces
// carries the note's own bytes: a reader is shown what their file says, not a
// respelling of it.
func TestOptionalEnumFoldsTheSpelling(t *testing.T) {
	t.Parallel()

	// Two words, each in two spellings, written from code points so nothing
	// between the keyboard and the compiler can fold one into the other: the
	// dakuten composed into one letter, then the bare letter followed by its
	// combining mark.
	const (
		composed        = "\u304c\u3044\u306d\u3093"
		decomposed      = "\u304b\u3099\u3044\u306d\u3093"
		otherComposed   = "\u3050\u3093"
		otherDecomposed = "\u304f\u3099\u3093"
	)
	if composed == decomposed || schema.NormalizeWord(decomposed) != composed {
		t.Fatalf("the fixture spellings are not one word in two forms: %q and %q", composed, decomposed)
	}
	if otherComposed == otherDecomposed || schema.NormalizeWord(otherDecomposed) != otherComposed {
		t.Fatalf("the control spellings are not one word in two forms: %q and %q", otherComposed, otherDecomposed)
	}
	if composed == otherComposed {
		t.Fatalf("the control word is the declared word: %q", composed)
	}

	// The vocabularies come from the fields yomihon defines, so a field added
	// there is covered here without a row. The type, the draft flag and the
	// language each answer to a rule of their own; the list is pinned so a
	// field leaving it is reported rather than silently untested.
	var fields []string
	for _, field := range schema.Fields() {
		if field == schema.FieldType || field == schema.DraftField || field == schema.FieldLang {
			continue
		}
		fields = append(fields, field)
	}
	if want := []string{"aliases", "based_on", "created", "slug", "level", "prerequisites", "verified_on", "verified_with"}; !slices.Equal(fields, want) {
		t.Fatalf("optional vocabularies = %q, want %q", fields, want)
	}

	spellings := []struct {
		name     string
		declared string
		value    string
		// wantValue is the word the finding must carry, or empty when the
		// note's value names what the contract declared.
		wantValue string
	}{
		{name: "composed contract, composed note", declared: composed, value: composed},
		{name: "composed contract, decomposed note", declared: composed, value: decomposed},
		{name: "decomposed contract, composed note", declared: decomposed, value: composed},
		{name: "decomposed contract, decomposed note", declared: decomposed, value: decomposed},
		{
			name: "a word the contract never declared", declared: composed, value: "other",
			wantValue: "other",
		},
		{
			name: "a different word, decomposed in the note", declared: composed, value: otherDecomposed,
			wantValue: otherDecomposed,
		},
	}
	for _, field := range fields {
		for _, tt := range spellings {
			t.Run(field+"/"+tt.name, func(t *testing.T) {
				t.Parallel()

				root := t.TempDir()
				write(t, root, schema.ContractRelPath, optionalEnumContract(field, tt.declared))
				data := []byte("---\ntype: memo\n" + field + ": " + tt.value + "\n---\nBody.\n")
				write(t, root, "Notes/Probe.md", string(data))
				contract, err := schema.Load(root)
				if err != nil {
					t.Fatalf("schema.Load() error = %v", err)
				}
				findings := LintFrontmatter("Notes/Probe.md", data, contract)
				var want []string
				if tt.wantValue != "" {
					want = []string{"schema.enum " + field + ` "` + tt.wantValue + `" is not an allowed value` +
						" target=" + tt.wantValue}
				}
				if diff := cmp.Diff(want, describeFindings(findings)); diff != "" {
					t.Errorf("LintFrontmatter() findings mismatch (-want +got):\n%s", diff)
				}
			})
		}
	}
}

// TestTypeRequiredFieldsFoldTheSpelling holds the fields a type adds to the
// required set to the type word, not its bytes. A type spelled one way in the
// contract and another in the note would otherwise lose its own requirement and
// the note would pass without the field its type asks for. A type that does not
// require the field is the control.
func TestTypeRequiredFieldsFoldTheSpelling(t *testing.T) {
	t.Parallel()

	const (
		composed   = "\u304c\u3044\u306d\u3093"
		decomposed = "\u304b\u3099\u3044\u306d\u3093"
	)
	if composed == decomposed || schema.NormalizeWord(decomposed) != composed {
		t.Fatalf("the fixture spellings are not one word in two forms: %q and %q", composed, decomposed)
	}

	tests := []struct {
		name     string
		declared string
		noteType string
		want     []string
	}{
		{name: "composed contract, composed note", declared: composed, noteType: composed, want: []string{"schema.required level is required"}},
		{name: "composed contract, decomposed note", declared: composed, noteType: decomposed, want: []string{"schema.required level is required"}},
		{name: "decomposed contract, composed note", declared: decomposed, noteType: composed, want: []string{"schema.required level is required"}},
		{name: "decomposed contract, decomposed note", declared: decomposed, noteType: decomposed, want: []string{"schema.required level is required"}},
		{name: "a type that does not require it", declared: composed, noteType: "memo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			write(t, root, schema.ContractRelPath, typeRequiredContract(tt.declared))
			data := []byte("---\ntype: " + tt.noteType + "\n---\nBody.\n")
			write(t, root, "Notes/Probe.md", string(data))
			contract, err := schema.Load(root)
			if err != nil {
				t.Fatalf("schema.Load() error = %v", err)
			}
			findings := LintFrontmatter("Notes/Probe.md", data, contract)
			if diff := cmp.Diff(tt.want, describeFindings(findings)); diff != "" {
				t.Errorf("LintFrontmatter() findings mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// describeFindings renders each finding as the sentence a reader is shown plus
// the value it points at, so a comparison covers both the wording and the bytes
// the finding carries.
func describeFindings(findings []Finding) []string {
	var got []string
	for i := range findings {
		f := &findings[i]
		entry := string(f.RuleID) + " " + f.Message
		if f.Target != nil {
			entry += " target=" + *f.Target
		}
		got = append(got, entry)
	}
	return got
}

// optionalEnumContract declares one word for one optional vocabulary, spelled
// the way the caller asks, and leaves every other vocabulary open.
func optionalEnumContract(field, declared string) string {
	return `schema_version = "2"

[folders]
checked = ["Notes"]
templates = []

[enums]
` + field + ` = ["` + declared + `"]

[fields]
common = ["type", "aliases", "based_on", "created", "slug", "level", "prerequisites", "verified_on", "verified_with"]
required = ["type"]

[types.memo]

[privacy]
never_egress_dirs = []
`
}

// typeRequiredContract declares a type that requires level, naming the type
// the way the caller asks, beside a memo that does not.
func typeRequiredContract(declared string) string {
	return `schema_version = "2"

[folders]
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

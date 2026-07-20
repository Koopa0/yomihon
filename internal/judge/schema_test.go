package judge

import (
	"bytes"
	"encoding/hex"
	"os"
	"testing"

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
			got := lintArticleLanguage(&n, &definition)
			if len(got) != tt.want {
				t.Fatalf("lintArticleLanguage() = %#v, want %d finding(s)", got, tt.want)
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

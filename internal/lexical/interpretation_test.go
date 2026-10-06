package lexical

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestOperatorShapedTermsKeepTheirLiteralMeaning(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, raw        string
		terms, operators []string
	}{
		{name: "flag and disjunction", raw: "goroutine -channel OR -channel", terms: []string{"goroutine", "-channel", "or", "-channel"}, operators: []string{"-channel", "OR"}},
		{name: "explicit quotes", raw: `"OR" 「-flag」`, terms: []string{"or", "-flag"}},
		{name: "lowercase and ordinary dash", raw: "or -", terms: []string{"or", "-"}},
		{name: "filter value", raw: "topic:-flag OR", terms: []string{"or"}, operators: []string{"OR"}},
		{name: "fullwidth is still a literal", raw: "ＯＲ", terms: []string{"or"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			q := Parse(tt.raw)
			want := struct{ Terms, Operators []string }{tt.terms, tt.operators}
			got := struct{ Terms, Operators []string }{q.Tokens(), q.LiteralOperatorTerms()}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("Parse(%q) interpretation (-want +got):\n%s", tt.raw, diff)
			}
			clone := q.LiteralOperatorTerms()
			if len(clone) > 0 {
				clone[0] = "changed"
				if diff := cmp.Diff(tt.operators, q.LiteralOperatorTerms()); diff != "" {
					t.Errorf("Parse(%q) exposes mutable interpretation (-want +got):\n%s", tt.raw, diff)
				}
			}
		})
	}
}

func TestFolderOffersPreserveScopeAndPathSegments(t *testing.T) {
	t.Parallel()
	idx := NewIndex([]Document{
		{RelPath: "20 Areas/Databases/one.md", Title: "SQL", PlainText: "semantic retrieval", NoteType: "lesson"},
		{RelPath: "Notes/café/a.md", Title: "NFC", PlainText: "semantic retrieval", NoteType: "lesson"},
		{RelPath: "Notes/go/deep/a.md", Title: "Deep", PlainText: "semantic retrieval", NoteType: "lesson"},
		{RelPath: "Notes/資料／庫/a.md", Title: "Wide solidus", PlainText: "semantic retrieval", NoteType: "lesson"},
		{RelPath: "Notes/Databases/not.md", Title: "No terms", PlainText: "unrelated", NoteType: "note"},
	}, validArtifactPolicy(t))
	tests := []struct {
		name, raw string
		want      []StepBack
	}{
		{name: "quoted words and scoped filter", raw: `"semantic retrieval" type:lesson folder:Databases`, want: []StepBack{{Query: `"semantic retrieval" type:lesson folder:"20 Areas/Databases"`, Count: 1}}},
		{name: "nfd basename", raw: "folder:cafe\u0301", want: []StepBack{{Query: "folder:Notes/café", Count: 1}}},
		{name: "ancestor directory", raw: "folder:go", want: []StepBack{{Query: "folder:Notes/go", Count: 1}}},
		{name: "fullwidth solidus is one segment", raw: "folder:資料／庫", want: []StepBack{{Query: "folder:Notes/資料／庫", Count: 1}}},
		{name: "real separator does not name that folder", raw: "folder:資料/庫"},
		{name: "filename is not a folder", raw: "folder:one.md"},
		{name: "literal quoted filter is not a constraint", raw: `"folder:Databases"`},
		{name: "conflicting preserved constraint", raw: "folder:Databases folder:Other"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := idx.FolderSuggestions(tt.raw)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("FolderSuggestions(%q) (-want +got):\n%s", tt.raw, diff)
			}
			for _, s := range got {
				a, err := idx.Search(Parse(s.Query), 0)
				if err != nil {
					t.Fatal(err)
				}
				if a.Total != s.Count {
					t.Errorf("Search(%q).Total = %d, offer promises %d", s.Query, a.Total, s.Count)
				}
			}
			answer, err := idx.Search(Parse(tt.raw), 0)
			if err != nil {
				t.Fatal(err)
			}
			if answer.Total != 0 {
				t.Errorf("Search(%q).Total = %d, short name must retain prefix semantics", tt.raw, answer.Total)
			}
		})
	}
}

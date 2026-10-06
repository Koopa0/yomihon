package lexical

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/vault"
)

func TestInlineFootnoteSearchUsesTheDefinitionWithoutItsDelimiters(t *testing.T) {
	t.Parallel()
	doc := DocumentFromNote(vault.Parse("Notes/Footnotes.md", []byte("Before^[行內註腳詞] after.\n")))
	if doc.PlainText != "Before after.\n行內註腳詞" {
		t.Fatalf("DocumentFromNote(inline footnote).PlainText = %q, want prose and the definition", doc.PlainText)
	}
	idx := NewIndex([]Document{doc}, validArtifactPolicy(t))
	for _, tt := range []struct {
		name, query string
		count       int
	}{
		{"definition words", "行內註腳詞", 1},
		{"spent delimiters", `"^[行內註腳詞]"`, 0},
		{"internal label", "yomihon-inline-footnote", 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := idx.Search(Parse(tt.query), -1)
			if err != nil {
				t.Fatalf("Search(%q): %v", tt.query, err)
			}
			if len(got.Results) != tt.count {
				t.Fatalf("Search(%q) returned %d results, want %d", tt.query, len(got.Results), tt.count)
			}
			if tt.count > 0 && (!strings.Contains(got.Results[0].Snippet, "行內註腳詞") || strings.Contains(got.Results[0].Snippet, "^[")) {
				t.Errorf("Search(%q).Snippet = %q, want actual definition words without delimiters", tt.query, got.Results[0].Snippet)
			}
		})
	}
}

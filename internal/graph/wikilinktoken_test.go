package graph

import (
	"slices"
	"testing"
)

func TestWikilinkTokensReadOneGrammar(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		text string
		want []string
	}{
		{"a link", "see [[Note]] now", []string{"Note"}},
		{"an embed", "![[Note|shown]]", []string{"Note|shown"}},
		{"two on a line", "[[a]] and ![[b#c]]", []string{"a", "b#c"}},
		{"an array of numbers is not a link", "x = [[1,2],[1,100]]", nil},
		{"a link after stray brackets", "[[a [[b]]", []string{"b"}},
		{"brackets in a name end the pair", "[[Target#^[note]|shown]]", nil},
		{"a name does not span lines", "[[a\nb]]", nil},
		{"an empty pair", "[[]]", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var got []string
			for _, token := range WikilinkTokens(tt.text) {
				got = append(got, token.Inner)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("WikilinkTokens(%q) inner = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}

func TestWikilinkTokenBoundsHoldTheBang(t *testing.T) {
	t.Parallel()
	const text = "a ![[b]] [[c]]"
	tokens := WikilinkTokens(text)
	if len(tokens) != 2 {
		t.Fatalf("tokens = %+v, want 2", tokens)
	}
	embed, link := tokens[0], tokens[1]
	if !embed.Embed || text[embed.Start:embed.End] != "![[b]]" || text[embed.Open():embed.Open()+2] != "[[" {
		t.Errorf("embed token = %+v", embed)
	}
	if link.Embed || text[link.Start:link.End] != "[[c]]" || link.Open() != link.Start {
		t.Errorf("link token = %+v", link)
	}
}

func TestParseFragmentTellsABlockFromASection(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ in, heading, block string }{
		{"Part one", "Part one", ""},
		{" spaced ", "spaced", ""},
		{"^abc-1", "", "abc-1"},
		{"", "", ""},
	} {
		heading, block := ParseFragment(tt.in)
		if heading != tt.heading || block != tt.block {
			t.Errorf("ParseFragment(%q) = (%q, %q), want (%q, %q)", tt.in, heading, block, tt.heading, tt.block)
		}
	}
}

package graph

import "testing"

func TestBlockAddressExtractsOnlySupportedAuthoredTokens(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ name, line, want string }{
		{name: "paragraph", line: "Para ^abc-1", want: "^abc-1"},
		{name: "marker only", line: "^abc-1", want: "^abc-1"},
		{name: "ASCII case retained", line: "Para ^AbC-1", want: "^AbC-1"},
		{name: "digits and hyphens", line: "Para ^0-9--", want: "^0-9--"},
		{name: "hyphen only", line: "Para ^-", want: "^-"},
		{name: "tab boundary", line: "Para\t^abc-1", want: "^abc-1"},
		{name: "trailing spaces and tabs", line: "Para ^abc-1 \t", want: "^abc-1"},
		{name: "last token", line: "Para ^first ^last", want: "^last"},
		{name: "Unicode prefix", line: "\u5168 ^abc-1", want: "^abc-1"},
		{name: "heading with words", line: "# Title ^abc", want: "^abc"},
		{name: "heading of only the token", line: "# ^0"},
		{name: "deep heading of only the token", line: "###### ^abc"},
		{name: "quoted heading of only the token", line: "> # ^abc"},
		{name: "hashes glued to the token", line: "#^abc", want: ""},
		{name: "empty line"},
		{name: "spaces only", line: " \t"},
		{name: "no caret", line: "Para abc-1"},
		{name: "empty token", line: "Para ^"},
		{name: "glued caret", line: "Para^abc-1"},
		{name: "underscore", line: "Para ^a_b"},
		{name: "dot", line: "Para ^a.b"},
		{name: "Unicode token", line: "Para ^\u304c"},
		{name: "combining token", line: "Para ^a\u0301"},
		{name: "fullwidth token", line: "Para ^\uff21"},
		{name: "punctuation", line: "Para ^abc!"},
		{name: "inline footnote", line: "Para ^[note]"},
		{name: "interior space", line: "Para ^a b"},
		{name: "interior tab", line: "Para ^a\tb"},
		{name: "nonbreaking boundary", line: "Para\u00a0^abc-1"},
		{name: "newline boundary", line: "Para\n^abc-1"},
		{name: "carriage return tail", line: "Para ^abc-1\r"},
		{name: "newline tail", line: "Para ^abc-1\n"},
		{name: "nonbreaking tail", line: "Para ^abc-1\u00a0"},
		{name: "adjacent wikilink", line: "Some text ^ref[[Note]]"},
		{name: "inline placeholder", line: "Some text ^ref\ue0000\ue001"},
		{name: "wide placeholder", line: "Some text ^ref\ue0020\ue003"},
		{name: "NUL tail", line: "Para ^abc-1\x00"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			t.Logf("hit: BlockAddress literal case %s", tt.name)
			if got := BlockAddress(tt.line); got != tt.want {
				t.Errorf("caught: shared block grammar changed: BlockAddress(%q) = %q, want %q", tt.line, got, tt.want)
			}
		})
	}
}

func FuzzBlockAddress(f *testing.F) {
	for _, line := range []string{
		"", "^", "^abc-1", "Para ^AbC-1 \t", "Para\t^0--9", "Para^glued",
		"Para ^a_b", "Para ^a.b", "Para ^\u304c", "Para ^[note]", "Para ^a b",
		"Some text ^ref[[Note]]", "Some text ^ref\ue0000\ue001", "^abc\x00", "^abc\n",
		string([]byte{0xff, ' ', '^', 'a'}),
	} {
		f.Add(line)
	}
	f.Fuzz(func(_ *testing.T, line string) {
		BlockAddress(line)
	})
}

func TestAHeadingOfOnlyTheTokenKeepsItAsItsWords(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ name, text, want string }{
		{name: "words then token", text: "Title ^abc", want: "Title"},
		{name: "only the token", text: "^0", want: "^0"},
		{name: "words that are hashes", text: "# ^0", want: "#"},
		{name: "token with spaces around", text: "  ^0 ", want: "  ^0 "},
		{name: "two lines, words on one", text: "Title\n^abc", want: "Title\n"},
		{name: "two lines, only tokens", text: "^a\n^b", want: "^a\n^b"},
	} {
		if got := WithoutBlockAddresses(tt.text); got != tt.want {
			t.Errorf("%s: WithoutBlockAddresses(%q) = %q, want %q", tt.name, tt.text, got, tt.want)
		}
	}
	if got := PlainHeadingWords("^0"); got != "^0" {
		t.Errorf("PlainHeadingWords(%q) = %q, want the token", "^0", got)
	}
}

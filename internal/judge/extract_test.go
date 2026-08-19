package judge

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// A backslash in front of a wikilink is the author showing the syntax rather
// than writing a link — the CommonMark backslash escape — and the reading page
// prints it as the text it is, with nothing to report. The adjudicator reads
// the same bytes for the same vault, so a name written that way is not a
// citation here either: one product cannot red a link on a page that shows no
// link, and the same extraction feeds the list of notes citing a note, which
// would otherwise name a citation the citing page does not make.
func TestExtractedLinksSkipEscapedBrackets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want []string
	}{
		{name: "a plain link is a citation", body: `[[Real]]`, want: []string{"Real"}},
		{name: "an escaped link is not", body: `\[[Ghost]]`, want: nil},
		{name: "an escaped embed is not", body: `\![[Phantom]]`, want: nil},
		{name: "a plain embed still is", body: `![[Real]]`, want: []string{"Real"}},
		{name: "an escaped backslash leaves the link", body: `\\[[Real]]`, want: []string{"Real"}},
		{name: "an odd run keeps the link escaped", body: `\\\[[Ghost]]`, want: nil},
		{name: "one escape does not cover the line", body: `see \[[Ghost]] then [[Real]]`, want: []string{"Real"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := LinkTargets(tt.body)
			if len(got) == 0 {
				got = nil
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("LinkTargets(%q) mismatch (-want +got):\n%s", tt.body, diff)
			}
		})
	}
}

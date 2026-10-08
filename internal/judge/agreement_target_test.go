package judge_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

// The fixture's delimiter escape cannot name a different note on the page
// than the check engine names. Keep the authored display text independent.
func TestAgreementWikilinkTargets(t *testing.T) {
	page := render.New(graph.BuildFromNotes(nil, nil), capturedBodies{}, noTitlesDeclared{}, everyFileHeld{})
	cases := []struct {
		inner string
		want  graph.Wikilink
	}{
		{inner: `Trail\`, want: graph.Wikilink{Target: "Trail", Display: `Trail\`}},
		{inner: `Trail\\`, want: graph.Wikilink{Target: "Trail", Display: `Trail\\`}},
		{inner: "Trail", want: graph.Wikilink{Target: "Trail", Display: "Trail"}},
		{inner: `Trail\|Alias`, want: graph.Wikilink{Target: "Trail", Display: "Alias", Aliased: true}},
	}
	for _, tt := range cases {
		parsed, ok := graph.ParseWikilink(tt.inner)
		if !ok {
			t.Fatal("not-applied: literal citation was refused")
		}
		if diff := cmp.Diff(tt.want, parsed); diff != "" {
			t.Errorf("caught: F3 wikilink-target complete name and display (-want +got):\n%s", diff)
		}
		body := "[[" + tt.inner + "]]\n"
		if diff := cmp.Diff([]string{"Trail"}, judge.LinkTargets(body)); diff != "" {
			t.Errorf("caught: F3 wikilink-target check citation (-want +got):\n%s", diff)
		}
		result := page.HTML("Notes/Reading.md", "", body, wording.En)
		actual := agreementObserve(t, result.HTML)
		want := []agreementCitation{{Target: "Trail", State: "wikilink-broken"}}
		if diff := cmp.Diff(want, actual.Citations); diff != "" {
			t.Errorf("caught: F3 wikilink-target page citation (-want +got):\n%s", diff)
		}
	}
	fragments := []struct {
		inner string
		want  graph.Wikilink
	}{
		{inner: `Trail\#Heading`, want: graph.Wikilink{Target: `Trail\`, Display: `Trail\#Heading`, Heading: "Heading"}},
		{inner: `Trail\^block`, want: graph.Wikilink{Target: `Trail\`, Display: `Trail\^block`, Block: "block"}},
		{inner: `Trail#Heading\`, want: graph.Wikilink{Target: "Trail", Display: `Trail#Heading\`, Heading: `Heading\`}},
		{inner: `Trail^block\`, want: graph.Wikilink{Target: "Trail", Display: `Trail^block\`, Block: `block\`}},
	}
	for _, tt := range fragments {
		parsed, ok := graph.ParseWikilink(tt.inner)
		if !ok {
			t.Fatal("not-applied: literal fragment control was refused")
		}
		if diff := cmp.Diff(tt.want, parsed); diff != "" {
			t.Errorf("caught: F3 wikilink-target authored fragment control (-want +got):\n%s", diff)
		}
	}
	t.Log("AGREEMENT-INVOKED F3/f3-wikilink-target")
}

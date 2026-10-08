package judge_test

import (
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

// These complete literal controls stay outside witnessed debt. A failure in a
// small agreed body must remain visible even when larger bodies disagree.
func TestAgreementPropertyControls(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		body      string
		citations []agreementCitation
		targets   []string
		blocks    []string
		headings  []string
		broken    int
	}{
		{name: "diagnostic-carrier", body: "[[Absent]]", citations: []agreementCitation{{Target: "Absent", State: "wikilink-broken"}}, targets: []string{"Absent"}, broken: 1},
		{name: "duplicate-occurrences", body: "[[A]] [[A]]", citations: []agreementCitation{{Target: "A", State: "wikilink"}, {Target: "A", State: "wikilink"}}, targets: []string{"A", "A"}},
		{name: "inline-code", body: "`[[A]]`"},
		{name: "local-heading-in-code", body: "# A\n\n`[[#A]]`\n", headings: []string{"a"}},
		{name: "block-address", body: "first ^a\n\nsecond\n", blocks: []string{"^a"}},
		{name: "heading-id", body: "## A\n", headings: []string{"a"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			bodies := capturedBodies{"Notes/A.md": "A\n"}
			page := render.New(graph.BuildFromNotes([]graph.NoteInput{{RelPath: "Notes/A.md"}}, nil), bodies, noTitlesDeclared{}, everyFileHeld{})
			result := page.HTML("Notes/Reading.md", "", tc.body, wording.En)
			observed := agreementObserveKnown(t, result.HTML, map[string]string{"Notes/A.md": "A"})
			if failures := agreementPageFailures(tc.body, &result, &observed); len(failures) != 0 {
				for i := range failures {
					f := &failures[i]
					t.Errorf("caught: %s %s literal-control=%s signature=%s", f.Property, f.Identity, tc.name, agreementSignature(f))
				}
			}
			if diff := cmp.Diff(tc.citations, observed.Citations); diff != "" {
				t.Errorf("caught: complete control carriers (-want +got):\n%s", diff)
			}
			if got := judge.LinkTargets(tc.body); !slices.Equal(tc.targets, got) {
				t.Errorf("caught: complete control targets got=%q want=%q", got, tc.targets)
			}
			if diff := cmp.Diff(tc.blocks, observed.Blocks); diff != "" {
				t.Errorf("caught: complete control block ids (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.headings, observed.Headings); diff != "" {
				t.Errorf("caught: complete control heading ids (-want +got):\n%s", diff)
			}
			if observed.CitationsInCode != 0 {
				t.Errorf("caught: P2 wikilink-in-code literal-control=%s count=%d", tc.name, observed.CitationsInCode)
			}
			if len(result.Diagnostics) != tc.broken {
				t.Errorf("caught: complete control diagnostics got=%+v want-count=%d", result.Diagnostics, tc.broken)
			}
			agreementFragments(t, []agreementCase{{Name: tc.name, Body: tc.body}}, []agreementHTML{observed})
			if tc.name == "block-address" {
				cut, found := render.Excerpt(tc.body, "^a")
				if cut != "first ^a" || !found {
					t.Errorf("caught: P3 bounded-excerpt literal-control cut=%q found=%t", cut, found)
				}
			}
		})
	}
}

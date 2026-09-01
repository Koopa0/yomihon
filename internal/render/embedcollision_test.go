package render_test

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestEmbedOfARepeatedHeadingSaysWhichOneItShows covers the excerpt a note can
// ask for and get without being told it was a choice. Where a note carries the
// same heading twice, an embed naming it takes the first, and the reader sees
// one section with nothing distinguishing it from the only section that could
// have matched.
//
// What is shown does not change: the excerpt this page already displayed is the
// one it keeps displaying, and this only puts the count beside it. Choosing
// differently would move words on a page over an ambiguity the author can
// settle in their own file, and picking a different section for them is a
// guess of exactly the kind the resolver refuses to make elsewhere.
func TestEmbedOfARepeatedHeadingSaysWhichOneItShows(t *testing.T) {
	t.Parallel()

	bodies := transclusions{
		"Twice.md": "## Dose\n\nFIRSTTEXT\n\n## Other\n\nx\n\n## Dose\n\nSECONDTEXT\n",
		"Once.md":  "## Dose\n\nONLYTEXT\n",
	}
	r := newRenderer(t, []graph.NoteInput{{RelPath: "Twice.md"}, {RelPath: "Once.md"}}, nil, bodies)

	repeated := r.HTML("note.md", "", "![[Twice#Dose]]\n", wording.En)
	if !strings.Contains(repeated.HTML, "FIRSTTEXT") {
		t.Errorf("the excerpt stopped showing the section it has always shown:\n%s", repeated.HTML)
	}
	if strings.Contains(repeated.HTML, "SECONDTEXT") {
		t.Errorf("the excerpt widened to sections the author did not ask for:\n%s", repeated.HTML)
	}
	if !strings.Contains(repeated.HTML, "embed__note") {
		t.Errorf("a reader was shown one of two identically named sections with no word about the other:\n%s", repeated.HTML)
	}
	if !strings.Contains(repeated.HTML, "2") {
		t.Errorf("the notice does not say how many sections answered to that name:\n%s", repeated.HTML)
	}

	var reported []render.Diagnostic
	for _, d := range repeated.Diagnostics {
		if d.Kind == render.DiagEmbedFragmentRepeated {
			reported = append(reported, d)
		}
	}
	if len(reported) != 1 {
		t.Fatalf("want one repeated-fragment diagnostic for the note's own status panel, got %d", len(reported))
	}
	if reported[0].Section != "Dose" {
		t.Errorf("the diagnostic does not name the fragment that was ambiguous: %+v", reported[0])
	}

	// The control: one section of that name is not a choice, and saying so
	// would make the notice meaningless everywhere it appears.
	once := r.HTML("note.md", "", "![[Once#Dose]]\n", wording.En)
	if !strings.Contains(once.HTML, "ONLYTEXT") {
		t.Fatalf("the fixture's premise is wrong: the single section did not render:\n%s", once.HTML)
	}
	if strings.Contains(once.HTML, "embed__note") {
		t.Errorf("an unambiguous excerpt was given a notice about a choice nobody made:\n%s", once.HTML)
	}
	for _, d := range once.Diagnostics {
		if d.Kind == render.DiagEmbedFragmentRepeated {
			t.Errorf("an unambiguous excerpt reported an ambiguity: %+v", d)
		}
	}
}

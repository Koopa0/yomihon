package render_test

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
)

// sectionDest is a destination with two named sections and a sentinel in each,
// so a widened excerpt is told from a scoped one by what reaches the page.
const sectionDest = "# Top\n\nTOPTEXT\n\n## Alpha\n\nALPHATEXT\n\n## Beta\n\nBETATEXT\n"

func sectionRenderer(t *testing.T) *render.Pipeline {
	t.Helper()
	return newRenderer(t, []graph.NoteInput{{Path: "B.md"}}, nil, transclusions{"B.md": sectionDest})
}

func fragmentDiagnostics(page *render.Result, kind render.DiagnosticKind) []string {
	var messages []string
	for _, d := range page.Diagnostics {
		if d.Kind == kind {
			messages = append(messages, d.Message)
		}
	}
	return messages
}

// TestSectionLinkWithNoSuchHeadingDegradesAndReports is the missing cell of
// the fragment table. A link's block address was already checked against the
// destination and dropped when nothing answered it; a link's section name was
// checked nowhere, so the address went out whether or not the destination
// stamps it. The reader was told which part of a long note they were going to
// and arrived at the top of it, with the words on screen still naming the
// section and nothing anywhere saying the promise was not kept.
func TestSectionLinkWithNoSuchHeadingDegradesAndReports(t *testing.T) {
	t.Parallel()

	r := sectionRenderer(t)
	got := r.HTML("note.md", "", "[[B#Gamma]]\n")

	if !strings.Contains(got.HTML, `<a href="/notes/B.md" class="wikilink">`) {
		t.Errorf("the link did not fall back to the note itself:\n%s", got.HTML)
	}
	if strings.Contains(got.HTML, "#gamma") {
		t.Errorf("the link kept a fragment the destination has no anchor for:\n%s", got.HTML)
	}
	messages := fragmentDiagnostics(&got, render.DiagLinkFragmentMissing)
	want := `no heading in "B.md" matched "Gamma"; the link leads to the note itself`
	if len(messages) != 1 || messages[0] != want {
		t.Errorf("fragment diagnostics = %q, want exactly one reading %q", messages, want)
	}
}

// TestSectionLinkKeepsAFragmentTheDestinationAnswers is the control the
// refusal above is worthless without: a check that drops every fragment would
// satisfy that test and break every working link on the page. The destination
// stamps its heading ids from the same slug rule the href is built with, so a
// section that exists must survive the validation untouched and report
// nothing.
func TestSectionLinkKeepsAFragmentTheDestinationAnswers(t *testing.T) {
	t.Parallel()

	r := sectionRenderer(t)
	for _, heading := range []string{"Alpha", "Beta", "Top"} {
		got := r.HTML("note.md", "", "[[B#"+heading+"]]\n")
		wantHref := `<a href="/notes/B.md#` + strings.ToLower(heading) + `" class="wikilink">`
		if !strings.Contains(got.HTML, wantHref) {
			t.Errorf("a section the destination carries lost its fragment:\nwant %s\ngot  %s", wantHref, got.HTML)
		}
		if messages := fragmentDiagnostics(&got, render.DiagLinkFragmentMissing); len(messages) != 0 {
			t.Errorf("a section that exists was reported missing: %q", messages)
		}
	}
}

// TestSectionLinkClaimsNothingAboutABodyItNeverRead holds the one case where
// silence is right. When the destination's body is outside the captured
// generation, yomihon cannot tell a heading that is absent from one it did not
// read, so it neither reports nor withdraws the address the author wrote.
func TestSectionLinkClaimsNothingAboutABodyItNeverRead(t *testing.T) {
	t.Parallel()

	r := newRenderer(t, []graph.NoteInput{{Path: "B.md"}}, nil, nil)
	got := r.HTML("note.md", "", "[[B#Gamma]]\n")

	if !strings.Contains(got.HTML, `href="/notes/B.md#gamma"`) {
		t.Errorf("an unread destination had its author's address withdrawn:\n%s", got.HTML)
	}
	if messages := fragmentDiagnostics(&got, render.DiagLinkFragmentMissing); len(messages) != 0 {
		t.Errorf("a body that was never read was reported on: %q", messages)
	}
}

// TestWidenedEmbedSaysSoInTheBody closes the other half. An embed whose
// fragment matches nothing falls back to the whole source note, which turns a
// scoped citation into an unscoped one — and the only account of that was a
// diagnostic in the right rail, which narrow viewports collapse and drop
// entirely. The words the author excluded appear in the article, so the
// article is where it has to be said.
func TestWidenedEmbedSaysSoInTheBody(t *testing.T) {
	t.Parallel()

	r := sectionRenderer(t)

	scoped := r.HTML("note.md", "", "![[B#Alpha]]\n")
	if !strings.Contains(scoped.HTML, "ALPHATEXT") {
		t.Fatalf("a scoped embed lost its own section:\n%s", scoped.HTML)
	}
	for _, sentinel := range []string{"BETATEXT", "TOPTEXT"} {
		if strings.Contains(scoped.HTML, sentinel) {
			t.Errorf("a scoped embed leaked %s from outside its section:\n%s", sentinel, scoped.HTML)
		}
	}
	if strings.Contains(scoped.HTML, "embed--widened") {
		t.Errorf("a scoped embed was marked widened:\n%s", scoped.HTML)
	}

	widened := r.HTML("note.md", "", "![[B#Gamma]]\n")
	for _, sentinel := range []string{"ALPHATEXT", "BETATEXT", "TOPTEXT"} {
		if !strings.Contains(widened.HTML, sentinel) {
			t.Errorf("the widened fallback dropped %s:\n%s", sentinel, widened.HTML)
		}
	}
	if !strings.Contains(widened.HTML, "embed--widened") {
		t.Errorf("the body carries no mark that the excerpt was widened:\n%s", widened.HTML)
	}
	if !strings.Contains(widened.HTML, "Gamma") {
		t.Errorf("the in-body mark does not name the fragment that matched nothing:\n%s", widened.HTML)
	}
}

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

// fragmentDiagnostics collects what a page said about the addresses in it that
// named a part of another note and could not be matched to one.
func fragmentDiagnostics(page *render.Result) []string {
	var messages []string
	for _, d := range page.Diagnostics {
		if d.Kind == render.DiagLinkFragmentMissing {
			messages = append(messages, d.Message)
		}
	}
	return messages
}

// TestSectionLinkWithNoSuchHeadingReportsWithoutWithdrawing is the missing
// cell of the fragment table. A link's block address was already checked
// against the destination; a link's section name was checked nowhere, so
// nothing anywhere said when a reader was sent to a part of a long note the
// destination does not answer to.
//
// The address itself survives the miss. Ids are stamped by a pass over
// rendered HTML which sees headings inside a blockquote or a list item that
// this line scan does not, so a miss here is a name the scan failed to find
// rather than one the page lacks. Withdrawing on it took working links away
// and told the reader a section they can see is not there.
func TestSectionLinkWithNoSuchHeadingReportsWithoutWithdrawing(t *testing.T) {
	t.Parallel()

	r := sectionRenderer(t)
	got := r.HTML("note.md", "", "[[B#Gamma]]\n")

	if !strings.Contains(got.HTML, `href="/notes/B.md#gamma"`) {
		t.Errorf("the link lost the address its author wrote:\n%s", got.HTML)
	}
	messages := fragmentDiagnostics(&got)
	want := `no heading in "B.md" matched "Gamma"; the address is left as written and may land at the top of the note`
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
		if messages := fragmentDiagnostics(&got); len(messages) != 0 {
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
	if messages := fragmentDiagnostics(&got); len(messages) != 0 {
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

// TestSectionLinkKeepsAnAnchorTheSourceScanCannotSee is the correction to a
// claim this package made and could not keep. The scan that validates a link's
// section name reads lines at the top level, and treats a blockquote or a list
// item as something a heading cannot be inside — but goldmark renders headings
// in both, and the id-stamping pass runs over the rendered HTML, so the page
// really does answer to those names. Withdrawing the address and reporting it
// missing was therefore wrong twice over: a link that worked stopped working,
// and the page said the destination has no such section while displaying it.
func TestSectionLinkKeepsAnAnchorTheSourceScanCannotSee(t *testing.T) {
	t.Parallel()
	const dest = "# Top\n\nTOPTEXT\n\n> ## Quoted Heading\n>\n> QUOTEDTEXT\n\n- ## Listed Heading\n\n  LISTEDTEXT\n"
	r := newRenderer(t, []graph.NoteInput{{Path: "B.md"}}, nil, transclusions{"B.md": dest})

	for _, heading := range []string{"Quoted Heading", "Listed Heading"} {
		got := r.HTML("note.md", "", "[[B#"+heading+"]]\n")
		if !strings.Contains(got.HTML, "#"+strings.ToLower(strings.ReplaceAll(heading, " ", "-"))) {
			t.Errorf("the link to %q lost its fragment; the destination stamps that id:\n%s", heading, got.HTML)
		}
		if messages := fragmentDiagnostics(&got); len(messages) != 0 {
			t.Errorf("a section the destination really carries was reported missing: %q", messages)
		}
	}

	// The destination genuinely stamps those ids, which is the fact the whole
	// test rests on. Read it off the destination's own rendering rather than
	// assuming it.
	page := r.HTML("B.md", "", dest)
	for _, id := range []string{`id="quoted-heading"`, `id="listed-heading"`} {
		if !strings.Contains(page.HTML, id) {
			t.Fatalf("the fixture's premise is wrong: the destination does not stamp %s:\n%s", id, page.HTML)
		}
	}
}

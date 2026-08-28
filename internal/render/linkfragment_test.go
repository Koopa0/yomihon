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
	return newRenderer(t, []graph.NoteInput{{RelPath: "B.md"}}, nil, transclusions{"B.md": sectionDest})
}

// fragmentDiagnostics collects what a page said about the addresses in it that
// named a part of another note and could not be matched to one.
func fragmentDiagnostics(page *render.Result) []string {
	var messages []string
	for _, d := range page.Diagnostics {
		if d.Kind == render.DiagLinkSectionMissing {
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

	r := newRenderer(t, []graph.NoteInput{{RelPath: "B.md"}}, nil, nil)
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
// The underlined half was the same mistake caught a second time. Marker
// stripping was added so a heading written inside a quote or a list item would
// be seen, but only the '#' form was ever looked for afterwards, so an
// underlined heading in those same places stayed invisible to the scan — and a
// link to one was reported broken while the reader was looking at the section
// it named. Every shape below is one the page stamps an id for.
func TestSectionLinkKeepsAnAnchorTheSourceScanCannotSee(t *testing.T) {
	t.Parallel()
	const dest = "# Top\n\nTOPTEXT\n\n> ## Quoted Heading\n>\n> QUOTEDTEXT\n\n" +
		"- ## Listed Heading\n\n  LISTEDTEXT\n\n" +
		"> Quoted Setext\n> ===\n>\n> QUOTEDSETEXTTEXT\n\n" +
		"> > Nested Setext\n> > ===\n\n" +
		"- Listed Setext\n  ===\n\n  LISTEDSETEXTTEXT\n"
	r := newRenderer(t, []graph.NoteInput{{RelPath: "B.md"}}, nil, transclusions{"B.md": dest})

	headings := []string{
		"Quoted Heading", "Listed Heading",
		"Quoted Setext", "Nested Setext", "Listed Setext",
	}

	// The destination genuinely stamps those ids, which is the fact the whole
	// test rests on. Read it off the destination's own rendering rather than
	// assuming it, and read it first — a fixture whose premise is wrong would
	// otherwise look like a passing test of the scan.
	page := r.HTML("B.md", "", dest)
	for _, heading := range headings {
		id := `id="` + strings.ToLower(strings.ReplaceAll(heading, " ", "-")) + `"`
		if !strings.Contains(page.HTML, id) {
			t.Fatalf("the fixture's premise is wrong: the destination does not stamp %s:\n%s", id, page.HTML)
		}
	}

	for _, heading := range headings {
		got := r.HTML("note.md", "", "[[B#"+heading+"]]\n")
		if !strings.Contains(got.HTML, "#"+strings.ToLower(strings.ReplaceAll(heading, " ", "-"))) {
			t.Errorf("the link to %q lost its fragment; the destination stamps that id:\n%s", heading, got.HTML)
		}
		if messages := fragmentDiagnostics(&got); len(messages) != 0 {
			t.Errorf("a section %q the destination really carries was reported missing: %q", heading, messages)
		}
	}
}

// TestSectionLinkStaysSilentAboutANoteThatEmbedsAnother is the conservative
// close on the one gap generosity cannot reach. Both scans read this note's own
// source, and a heading that arrives through an embed is written in a different
// file — so no amount of marker stripping will find it, while the page stamps
// its id all the same. Where the destination embeds anything, a name the scans
// missed is left unreported.
//
// The control beside it is what keeps that from becoming blanket silence: a
// destination that embeds nothing still gets its genuinely absent section
// reported, so the close costs exactly the notes it is aimed at.
func TestSectionLinkStaysSilentAboutANoteThatEmbedsAnother(t *testing.T) {
	t.Parallel()

	r := newRenderer(t, []graph.NoteInput{{RelPath: "B.md"}, {RelPath: "C.md"}}, nil, transclusions{
		"B.md": "# Top\n\nTOPTEXT\n\n![[C]]\n",
		"C.md": "## Brought In\n\nBROUGHTTEXT\n",
	})

	embedding := r.HTML("note.md", "", "[[B#Brought In]]\n")
	if !strings.Contains(embedding.HTML, `href="/notes/B.md#brought-in"`) {
		t.Errorf("the link lost the address its author wrote:\n%s", embedding.HTML)
	}
	if messages := fragmentDiagnostics(&embedding); len(messages) != 0 {
		t.Errorf("a section an embed brings into the destination was called missing: %q", messages)
	}

	// The premise: the destination really does stamp that id, because the
	// embed really is expanded into it.
	page := r.HTML("B.md", "", "# Top\n\nTOPTEXT\n\n![[C]]\n")
	if !strings.Contains(page.HTML, `id="brought-in"`) {
		t.Fatalf("the fixture's premise is wrong: the embed does not bring the heading in:\n%s", page.HTML)
	}

	plain := r.HTML("note.md", "", "[[C#Nowhere]]\n")
	if messages := fragmentDiagnostics(&plain); len(messages) != 1 {
		t.Errorf("a destination that embeds nothing stopped reporting a section it really lacks: %q", messages)
	}
}

// TestSectionLinkStillMissesAHeadingInsideAFence records a known limit rather
// than a behaviour worth having. The generous scan does not track fenced code,
// so a heading written inside a code sample counts as a heading it might be,
// and a link naming it is left alone instead of being reported. That is the
// safe direction — the cost is one unreported dead link, against telling a
// reader a section they can see is not there — but it is a gap, and a silent
// gap is one nobody remembers is there.
func TestSectionLinkStillMissesAHeadingInsideAFence(t *testing.T) {
	t.Parallel()

	const dest = "# Top\n\nTOPTEXT\n\n```markdown\n## Only In A Sample\n```\n"
	r := newRenderer(t, []graph.NoteInput{{RelPath: "B.md"}}, nil, transclusions{"B.md": dest})

	page := r.HTML("B.md", "", dest)
	if strings.Contains(page.HTML, `id="only-in-a-sample"`) {
		t.Fatalf("the fixture's premise is wrong: the fenced heading really is an anchor:\n%s", page.HTML)
	}

	got := r.HTML("note.md", "", "[[B#Only In A Sample]]\n")
	if messages := fragmentDiagnostics(&got); len(messages) != 0 {
		t.Errorf("the fenced-heading gap has closed; this test now records the wrong limit: %q", messages)
	}
}

package render_test

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

// sectionDest is a destination with two named sections and a sentinel in each,
// so a withheld excerpt is told from a scoped one by what reaches the page.
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
	got := r.HTML("note.md", "", "[[B#Gamma]]\n", wording.ZhHant)

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
		got := r.HTML("note.md", "", "[[B#"+heading+"]]\n", wording.ZhHant)
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
	got := r.HTML("note.md", "", "[[B#Gamma]]\n", wording.ZhHant)

	if !strings.Contains(got.HTML, `href="/notes/B.md#gamma"`) {
		t.Errorf("an unread destination had its author's address withdrawn:\n%s", got.HTML)
	}
	if messages := fragmentDiagnostics(&got); len(messages) != 0 {
		t.Errorf("a body that was never read was reported on: %q", messages)
	}
}

// TestAWithheldEmbedSaysSoInTheBody closes the other half. An embed whose
// fragment matches nothing used to fall back to the whole source note, which
// turned a scoped citation into an unscoped one: the author named one section,
// and the page answered with more than they wrote. Nothing of the note is shown
// now, and the account of that stands in the article itself, because the
// diagnostic in the right rail is one a narrow viewport collapses and drops.
// The notice names the address that matched nothing and keeps the way on to
// the note, in both languages the interface speaks.
func TestAWithheldEmbedSaysSoInTheBody(t *testing.T) {
	t.Parallel()

	r := sectionRenderer(t)

	scoped := r.HTML("note.md", "", "![[B#Alpha]]\n", wording.ZhHant)
	if !strings.Contains(scoped.HTML, "ALPHATEXT") {
		t.Fatalf("a scoped embed lost its own section:\n%s", scoped.HTML)
	}
	for _, sentinel := range []string{"BETATEXT", "TOPTEXT"} {
		if strings.Contains(scoped.HTML, sentinel) {
			t.Errorf("a scoped embed leaked %s from outside its section:\n%s", sentinel, scoped.HTML)
		}
	}
	if strings.Contains(scoped.HTML, "embed--withheld") {
		t.Errorf("a scoped embed was marked withheld:\n%s", scoped.HTML)
	}

	for _, tt := range []struct {
		lang   wording.Lang
		notice string
	}{
		{lang: wording.ZhHant, notice: `<p class="embed__note">找不到「#Gamma」：〈B〉裡沒有這個位址。</p>`},
		{lang: wording.En, notice: `<p class="embed__note">Unable to find “#Gamma” in B.</p>`},
	} {
		withheld := r.HTML("note.md", "", "![[B#Gamma]]\n", tt.lang)
		for _, sentinel := range []string{"ALPHATEXT", "BETATEXT", "TOPTEXT"} {
			if strings.Contains(withheld.HTML, sentinel) {
				t.Errorf("%s: a section the note does not have was widened into %s:\n%s", tt.lang, sentinel, withheld.HTML)
			}
		}
		want := `<div class="embed embed--withheld"><p class="embed__source">` +
			wording.EmbedSourceFrom.In(tt.lang) + `<a href="/notes/B.md">B</a></p>` + tt.notice + `</div>`
		if !strings.Contains(withheld.HTML, want) {
			t.Errorf("%s: the withheld embed is not the notice and the way on, and nothing else:\nwant %s\ngot  %s", tt.lang, want, withheld.HTML)
		}
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
	page := r.HTML("B.md", "", dest, wording.ZhHant)
	for _, heading := range headings {
		id := `id="` + strings.ToLower(strings.ReplaceAll(heading, " ", "-")) + `"`
		if !strings.Contains(page.HTML, id) {
			t.Fatalf("the fixture's premise is wrong: the destination does not stamp %s:\n%s", id, page.HTML)
		}
	}

	for _, heading := range headings {
		got := r.HTML("note.md", "", "[[B#"+heading+"]]\n", wording.ZhHant)
		if !strings.Contains(got.HTML, "#"+strings.ToLower(strings.ReplaceAll(heading, " ", "-"))) {
			t.Errorf("the link to %q lost its fragment; the destination stamps that id:\n%s", heading, got.HTML)
		}
		if messages := fragmentDiagnostics(&got); len(messages) != 0 {
			t.Errorf("a section %q the destination really carries was reported missing: %q", heading, messages)
		}
	}
}

// TestSectionLinkFollowsTheEmbedThatBringsTheHeadingIn closes the one gap
// neither scan of this note's own source can reach. A heading that arrives
// inside an embed is written in another file, and the page stamps its id all
// the same — so the check reads that file too, one level, exactly as far as
// the render goes.
//
// It used to stop instead: a destination that embedded anything had every
// unmatched name left unreported, which bought silence about the headings the
// embed really brought and paid for it with silence about every name that was
// nowhere at all.
func TestSectionLinkFollowsTheEmbedThatBringsTheHeadingIn(t *testing.T) {
	t.Parallel()

	r := newRenderer(t, []graph.NoteInput{{RelPath: "B.md"}, {RelPath: "C.md"}}, nil, transclusions{
		"B.md": "# Top\n\nTOPTEXT\n\n![[C]]\n",
		"C.md": "## Brought In\n\nBROUGHTTEXT\n",
	})

	// The premise, checked before anything is concluded from it: the
	// destination really does stamp that id, because the embed really is
	// expanded into it.
	page := r.HTML("B.md", "", "# Top\n\nTOPTEXT\n\n![[C]]\n", wording.ZhHant)
	if !strings.Contains(page.HTML, `id="brought-in"`) {
		t.Fatalf("the fixture's premise is wrong: the embed does not bring the heading in:\n%s", page.HTML)
	}

	embedding := r.HTML("note.md", "", "[[B#Brought In]]\n", wording.ZhHant)
	if !strings.Contains(embedding.HTML, `href="/notes/B.md#brought-in"`) {
		t.Errorf("the link lost the address its author wrote:\n%s", embedding.HTML)
	}
	if messages := fragmentDiagnostics(&embedding); len(messages) != 0 {
		t.Errorf("a section an embed brings into the destination was called missing: %q", messages)
	}

	// The half the old close gave away: a name that is in neither file.
	nowhere := r.HTML("note.md", "", "[[B#Nowhere]]\n", wording.ZhHant)
	if messages := fragmentDiagnostics(&nowhere); len(messages) != 1 {
		t.Errorf("a name absent from the destination and from what it embeds went unreported: %q", messages)
	}

	plain := r.HTML("note.md", "", "[[C#Nowhere]]\n", wording.ZhHant)
	if messages := fragmentDiagnostics(&plain); len(messages) != 1 {
		t.Errorf("a destination that embeds nothing stopped reporting a section it really lacks: %q", messages)
	}
}

// TestSectionLinkReadsOnlyWhatTheEmbedActuallyBrings holds the check to the
// same edges the render draws. An embed carrying a fragment shows one section,
// so only that section's headings reach the page, and a name from elsewhere in
// the same file is as absent as if the file had never been cited. An embed
// whose fragment the note does not answer to shows nothing of it, so it brings
// no heading either. An embed of something that is not a note brings no
// headings at all, and one that resolves nowhere brings nothing to read.
func TestSectionLinkReadsOnlyWhatTheEmbedActuallyBrings(t *testing.T) {
	t.Parallel()

	notes := []graph.NoteInput{{RelPath: "Scoped.md"}, {RelPath: "Withheld.md"}, {RelPath: "Parts.md"}, {RelPath: "Pic.md"}, {RelPath: "Gone.md"}, {RelPath: "Data.md"}}
	r := newRenderer(t, notes, []string{"pics/plate.png", "table.csv"}, transclusions{
		"Scoped.md":   "![[Parts#Wanted]]\n",
		"Withheld.md": "![[Parts#Nowhere]]\n",
		"Parts.md":    "## Wanted\n\nWANTEDTEXT\n\n## Unwanted\n\nUNWANTEDTEXT\n",
		"Pic.md":      "![[pics/plate.png]]\n",
		"Gone.md":     "![[NoSuchNote]]\n",
		"Data.md":     "![[table.csv]]\n",
		// A readable non-Markdown file is captured with its bytes, so this
		// body is one the scan could reach — and its first line is heading
		// shaped on purpose. The reader is handed the file whole and nothing
		// inside it is rendered, so nothing inside it is addressable either.
		"table.csv": "## Column One\n\n1,2,3\n",
	})

	// The premise for the scoped case, in both directions: the section the
	// embed names arrives, and the one beside it does not.
	page := r.HTML("Scoped.md", "", "![[Parts#Wanted]]\n", wording.ZhHant)
	if !strings.Contains(page.HTML, `id="wanted"`) {
		t.Fatalf("the fixture's premise is wrong: the named section did not arrive:\n%s", page.HTML)
	}
	if strings.Contains(page.HTML, `id="unwanted"`) {
		t.Fatalf("the fixture's premise is wrong: the embed was not scoped:\n%s", page.HTML)
	}

	// The premise for the withheld case: an address the note does not answer
	// to puts none of its headings on the page.
	withheld := r.HTML("Withheld.md", "", "![[Parts#Nowhere]]\n", wording.ZhHant)
	for _, id := range []string{`id="wanted"`, `id="unwanted"`} {
		if strings.Contains(withheld.HTML, id) {
			t.Fatalf("the fixture's premise is wrong: a withheld embed stamped %s:\n%s", id, withheld.HTML)
		}
	}

	// The premise for the non-Markdown case: its bytes really are captured
	// (so the scan could read them) and the render really does hand the file
	// over whole rather than expanding what is inside it.
	shownWhole := r.HTML("Data.md", "", "![[table.csv]]\n", wording.ZhHant)
	if !strings.Contains(shownWhole.HTML, "embed-media") {
		t.Fatalf("the fixture's premise is wrong: the file was not shown whole:\n%s", shownWhole.HTML)
	}
	if strings.Contains(shownWhole.HTML, `id="column-one"`) {
		t.Fatalf("the fixture's premise is wrong: the file's contents were rendered:\n%s", shownWhole.HTML)
	}

	for _, c := range []struct {
		name string
		link string
		want int
	}{
		{"a section the scoped embed brings", "[[Scoped#Wanted]]", 0},
		{"a section left outside the embed's own fragment", "[[Scoped#Unwanted]]", 1},
		{"a section the withheld embed would have brought", "[[Withheld#Wanted]]", 1},
		{"a name nowhere near either file", "[[Scoped#Nowhere]]", 1},
		{"a picture embed carries no headings", "[[Pic#Nowhere]]", 1},
		{"an embed that resolves nowhere carries none either", "[[Gone#Nowhere]]", 1},
		{"a file shown whole carries no addressable headings", "[[Data#Column One]]", 1},
	} {
		got := r.HTML("note.md", "", c.link+"\n", wording.ZhHant)
		if messages := fragmentDiagnostics(&got); len(messages) != c.want {
			t.Errorf("%s: want %d reports, got %d: %q", c.name, c.want, len(messages), messages)
		}
	}
}

// TestSectionLinkIgnoresAnEmbedNobodyFollows keeps the scan on the same side
// of quoting as the pass that expands embeds. Syntax inside a code span is the
// author showing what an embed looks like, and a backslash before one is the
// author asking for the brackets themselves — neither is expanded, so neither
// brings a heading, and a link naming one is naming something that is not
// there. Counting them would restore the old silence through the back door:
// any note that merely mentions the syntax would stop reporting.
func TestSectionLinkIgnoresAnEmbedNobodyFollows(t *testing.T) {
	t.Parallel()

	bodies := transclusions{
		"Quoted.md":  "## Own\n\n`![[C]]`\n",
		"Escaped.md": "## Own\n\n\\![[C]]\n",
		"C.md":       "## Brought In\n\nBROUGHTTEXT\n",
	}
	r := newRenderer(t, []graph.NoteInput{{RelPath: "Quoted.md"}, {RelPath: "Escaped.md"}, {RelPath: "C.md"}}, nil, bodies)

	for _, relPath := range []string{"Quoted.md", "Escaped.md"} {
		// The premise: the render really does leave this one alone, so there
		// really is no id for the link to have found.
		page := r.HTML(relPath, "", bodies[relPath], wording.ZhHant)
		if strings.Contains(page.HTML, `id="brought-in"`) {
			t.Fatalf("the fixture's premise is wrong: %s expanded an embed it should have shown:\n%s", relPath, page.HTML)
		}
		link := r.HTML("note.md", "", "[["+strings.TrimSuffix(relPath, ".md")+"#Brought In]]\n", wording.ZhHant)
		if messages := fragmentDiagnostics(&link); len(messages) != 1 {
			t.Errorf("%s shows the syntax rather than following it, and a link into it went unreported: %q", relPath, messages)
		}
	}
}

// TestEveryStampedHeadingIsAFragmentTheCheckAccepts is the differential lock
// between the two halves that must not drift: the pass that stamps ids over
// rendered HTML, and the scan that decides whether a link naming one is
// broken. Whatever the render stamps, the check has to leave alone — a
// reported miss on an id the page really carries is the expensive direction of
// wrong, because it withdraws a section the reader can see.
//
// The reverse does not hold and is not asserted. The check is deliberately
// generous: it accepts names it cannot prove absent, so it accepts more than
// the render stamps, and that surplus is the safe direction.
func TestEveryStampedHeadingIsAFragmentTheCheckAccepts(t *testing.T) {
	t.Parallel()

	bodies := transclusions{
		"Own.md":     "## Plain\n\nx\n\n### Deeper\n\ny\n",
		"Host.md":    "## Host Own\n\nx\n\n![[Bring]]\n",
		"Bring.md":   "## Brought\n\nx\n\n## Also Brought\n\ny\n",
		"Narrow.md":  "![[Bring#Brought]]\n",
		"Quoted.md":  "> ## Inside A Quote\n>\n> x\n",
		"Listed.md":  "- ## Inside A List\n\n  x\n",
		"Twofold.md": "## Same\n\nx\n\n## Same\n\ny\n",
	}
	var notes []graph.NoteInput
	for relPath := range bodies {
		notes = append(notes, graph.NoteInput{RelPath: relPath})
	}
	r := newRenderer(t, notes, nil, bodies)

	stamped := 0
	for relPath, body := range bodies {
		page := r.HTML(relPath, "", body, wording.ZhHant)
		for _, entry := range page.TOC {
			stamped++
			// The check is asked in the terms a reader writes: the heading's
			// own text, which is what a wikilink fragment carries.
			link := r.HTML("asking.md", "", "[["+strings.TrimSuffix(relPath, ".md")+"#"+entry.Text+"]]\n", wording.ZhHant)
			if messages := fragmentDiagnostics(&link); len(messages) != 0 {
				t.Errorf("%s stamps id %q for heading %q, and a link naming it was reported broken: %q",
					relPath, entry.ID, entry.Text, messages)
			}
		}
	}
	// A run over an empty set of headings would pass while checking nothing.
	if stamped < len(bodies) {
		t.Fatalf("the sweep found only %d stamped headings across %d notes, so it proves almost nothing", stamped, len(bodies))
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

	page := r.HTML("B.md", "", dest, wording.ZhHant)
	if strings.Contains(page.HTML, `id="only-in-a-sample"`) {
		t.Fatalf("the fixture's premise is wrong: the fenced heading really is an anchor:\n%s", page.HTML)
	}

	got := r.HTML("note.md", "", "[[B#Only In A Sample]]\n", wording.ZhHant)
	if messages := fragmentDiagnostics(&got); len(messages) != 0 {
		t.Errorf("the fenced-heading gap has closed; this test now records the wrong limit: %q", messages)
	}
}

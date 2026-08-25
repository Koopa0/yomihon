package render_test

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
)

// A link to a note nobody has written yet is styled with a help cursor, which
// tells the reader that pausing on it will explain something. Readers took that
// offer, got nothing, clicked, still got nothing, and concluded the page was
// broken. The explanation has to travel with the mark that promises it.
func TestUnwrittenLinkCarriesItsExplanation(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{Path: "運弓筆記.md"}}, nil, nil)

	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "the name that failed is the one named, not the words shown",
			body: "見 [[節拍器練習法|那份練習法]]\n",
			want: `title="還沒有「節拍器練習法」這篇筆記"`,
		},
		{
			name: "an embed of a note nobody wrote says the same thing",
			body: "![[節拍器練習法]]\n",
			want: `title="還沒有「節拍器練習法」這篇筆記"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := r.HTML("練習日誌/2026-07-30.md", "2026-07-30", tt.body)
			if !strings.Contains(got.HTML, tt.want) {
				t.Errorf("HTML(%q) = %s\nwant it to contain %s", tt.body, got.HTML, tt.want)
			}
			// The help cursor is what makes the explanation owed, so a mark
			// carrying one and nothing else is the state this guards against.
			if strings.Contains(got.HTML, `<span class="wikilink-broken">`) {
				t.Errorf("HTML(%q) still renders a help-cursor mark with no explanation:\n%s", tt.body, got.HTML)
			}
		})
	}
}

// A file whose own name contains "#" can never be addressed whole by a
// wikilink: the parse reads everything after the mark as a section name, so
// the link resolves the shorter name in front of it — and when that name is
// unwritten, the explanation used to state only the half the reader did not
// type. The explanation now also says how the link was read, so the reader
// can see the split instead of hunting for a note they can watch existing.
func TestUnwrittenLinkWithSectionFragmentExplainsTheSplit(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{Path: "井號#筆記.md"}}, nil, nil)

	tests := []struct {
		name string
		body string
	}{
		{name: "a plain link", body: "見 [[井號#筆記]]\n"},
		{name: "an embed", body: "![[井號#筆記]]\n"},
	}
	const reason = "還沒有「井號」這篇筆記；「#」之後的「筆記」被讀成章節名稱"
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := r.HTML("a.md", "A", tt.body)
			if !strings.Contains(got.HTML, `title="`+reason+`"`) {
				t.Errorf("HTML(%q) = %s\nwant the explanation to state the fragment split: %q", tt.body, got.HTML, reason)
			}
			if !strings.Contains(got.HTML, "（"+reason+"）") {
				t.Errorf("HTML(%q) does not carry the split explanation out of sight for readers who cannot hover:\n%s", tt.body, got.HTML)
			}
		})
	}
}

// The clause explains a fragment that was actually parsed; a link that
// carried none keeps the original sentence, and a fragment that parsed to
// nothing was not read as a section name. A block address is not a section
// name either — "#^blk" and "^blk" both address a block — so an unwritten
// target behind one keeps the plain sentence too.
func TestUnwrittenLinkWithoutSectionFragmentKeepsThePlainExplanation(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)

	tests := []struct {
		name string
		body string
	}{
		{name: "no fragment", body: "見 [[井號]]\n"},
		{name: "an empty fragment", body: "見 [[井號#]]\n"},
		{name: "a block fragment", body: "見 [[井號#^b1k]]\n"},
		{name: "a bare block fragment", body: "見 [[井號^b1k]]\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := r.HTML("a.md", "A", tt.body)
			if !strings.Contains(got.HTML, `title="還沒有「井號」這篇筆記"`) {
				t.Errorf("HTML(%q) lost the plain unwritten explanation:\n%s", tt.body, got.HTML)
			}
			if strings.Contains(got.HTML, "章節名稱") {
				t.Errorf("HTML(%q) explains a section split no link wrote:\n%s", tt.body, got.HTML)
			}
		})
	}
}

// The sentence an unwritten link carries is an explanation of that link, not
// part of what the section around it is called. Rendered inside a heading it
// became the heading's anchor, its table-of-contents entry, and the fragment of
// every address written at that section, so the reader met an error message
// where a section name belongs.
func TestUnwrittenLinkInAHeadingStaysOutOfItsAnchor(t *testing.T) {
	t.Parallel()

	const body = "## 見 [[節拍器練習法|那份練習法]]\n\n本文。\n"
	const name = "見 那份練習法"
	const anchor = "見-那份練習法"

	r := newRenderer(t, []graph.NoteInput{{Path: "B.md"}}, nil, transclusions{"B.md": body})
	page := r.HTML("B.md", "", body)

	if len(page.TOC) != 1 {
		t.Fatalf("TOC = %+v, want exactly one entry", page.TOC)
	}
	if page.TOC[0].Text != name {
		t.Errorf("the table of contents reads %q, want the heading's own words %q", page.TOC[0].Text, name)
	}
	if page.TOC[0].ID != anchor {
		t.Errorf("the anchor is %q, want %q", page.TOC[0].ID, anchor)
	}
	if !strings.Contains(page.HTML, `<h2 id="`+anchor+`">`) {
		t.Errorf("the heading element does not carry the anchor %q:\n%s", anchor, page.HTML)
	}

	// The explanation is still where it was owed — beside the link, out of
	// sight — because the reader who pauses on that mark is still promised it.
	if !strings.Contains(page.HTML, `<span class="y-offscreen">（還沒有「節拍器練習法」這篇筆記）</span>`) {
		t.Errorf("the link lost the explanation it promises on hover:\n%s", page.HTML)
	}

	// The scan that answers an embed's "#section" reads the same heading from
	// source. Both surfaces name a section by the words on screen, so the name
	// the page publishes is one an embed accepts.
	embedded := r.HTML("note.md", "", "![[B#"+name+"]]\n")
	if !strings.Contains(embedded.HTML, "本文。") {
		t.Errorf("an embed of the section the page names did not reach it:\n%s", embedded.HTML)
	}
	for _, d := range embedded.Diagnostics {
		if d.Kind == render.DiagEmbedFragmentMissing {
			t.Errorf("the embed reported the section absent from the note that publishes it: %s", d.Message)
		}
	}
}

// The words dropped from a heading's anchor are the renderer's own; the same
// markup authored in a note is display input and stays visible, so nothing a
// note can write reaches inside the anchor rule.
func TestAuthoredOffscreenMarkupInAHeadingIsShownRatherThanDropped(t *testing.T) {
	t.Parallel()

	r := newRenderer(t, nil, nil, nil)
	page := r.HTML("a.md", "", "## 標題<span class=\"y-offscreen\">（隱藏）</span>\n\n本文。\n")

	if len(page.TOC) != 1 {
		t.Fatalf("TOC = %+v, want exactly one entry", page.TOC)
	}
	if !strings.Contains(page.TOC[0].Text, "隱藏") {
		t.Errorf("the table of contents reads %q, want the authored words kept", page.TOC[0].Text)
	}
}

// A target that does resolve can still fail to reach the page, and saying it
// was never written would be a different and false claim.
func TestResolvedButUnavailableEmbedSaysWhatActuallyHappened(t *testing.T) {
	t.Parallel()
	// The note is in the resolver but its body was not captured, which is what
	// a generation that read the name but not the bytes looks like.
	r := newRenderer(t, []graph.NoteInput{{Path: "運弓筆記.md"}}, nil, nil)

	got := r.HTML("a.md", "A", "![[運弓筆記]]\n")
	if strings.Contains(got.HTML, "還沒有") {
		t.Errorf("HTML() told the reader the note was never written, when it exists:\n%s", got.HTML)
	}
	if !strings.Contains(got.HTML, "拿不到它的內容") {
		t.Errorf("HTML() did not say what actually failed:\n%s", got.HTML)
	}
}

// The prose mark explains the split where the link sits, but the diagnostic
// list beside the note is where a reader goes to find out what is wrong with
// the file — and there the same link arrived indistinguishable from one that
// simply named nothing. The section the author addressed travels with the
// diagnostic so the panel can state it, while the target stays the bare
// resolution key every other reader of that field looks names up by.
func TestUnresolvedLinkDiagnosticCarriesTheSectionItAddressed(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{Path: "井號#筆記.md"}}, nil, nil)

	tests := []struct {
		name        string
		body        string
		wantSection string
	}{
		{name: "a plain link", body: "見 [[井號#筆記]]\n", wantSection: "筆記"},
		{name: "an embed", body: "![[井號#筆記]]\n", wantSection: "筆記"},
		{name: "a link with display text", body: "見 [[井號#筆記|那一段]]\n", wantSection: "筆記"},
		// The controls: a link that addressed no section must not gain one,
		// and a block address is not a section name in this dialect.
		{name: "no fragment", body: "見 [[井號]]\n"},
		{name: "a block fragment", body: "見 [[井號#^b1k]]\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := r.HTML("a.md", "A", tt.body)
			if len(got.Diagnostics) != 1 {
				t.Fatalf("HTML(%q) produced %d diagnostics, want 1", tt.body, len(got.Diagnostics))
			}
			d := got.Diagnostics[0]
			if d.Kind != render.DiagWikilinkBroken {
				t.Fatalf("HTML(%q) diagnostic kind = %q, want %q", tt.body, d.Kind, render.DiagWikilinkBroken)
			}
			// The target is the name resolution failed on, and other readers
			// of this field look planned names up by it, so it stays bare.
			if d.Target != "井號" {
				t.Errorf("HTML(%q) diagnostic target = %q, want %q", tt.body, d.Target, "井號")
			}
			if d.Section != tt.wantSection {
				t.Errorf("HTML(%q) diagnostic section = %q, want %q", tt.body, d.Section, tt.wantSection)
			}
		})
	}
}

package render_test

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

// hostStamp renders body as one note's own text and returns the transcluded
// identity it would stamp.
func hostStamp(t *testing.T, r *render.Pipeline, body string) string {
	t.Helper()
	return r.HTML("host.md", "", body, wording.ZhHant).TranscludedIdentity
}

// TestTranscludedIdentityFollowsWhatTheRenderExpands pins which citations the
// stamp covers: exactly the ones whose expansion puts another note's words on
// the page. Everything the render shows without expanding — quoted syntax, an
// escaped token, a picture, a name that resolves to nothing, a body the
// generation never captured — leaves the stamp empty, so a page that
// transcluded nothing keeps the narrower polling ask it always had.
func TestTranscludedIdentityFollowsWhatTheRenderExpands(t *testing.T) {
	t.Parallel()
	r := newRenderer(t,
		[]graph.NoteInput{{RelPath: "X.md"}, {RelPath: "Ghost.md"}},
		[]string{"pic.png"},
		transclusions{"X.md": "Words from X.\n"},
	)
	for _, tt := range []struct {
		name    string
		body    string
		stamped bool
	}{
		{name: "no citation at all", body: "Plain words.\n", stamped: false},
		{name: "a plain wikilink puts no other note's words here", body: "See [[X]].\n", stamped: false},
		{name: "a whole-note embed", body: "![[X]]\n", stamped: true},
		{name: "an embed inside a callout is the host's own text", body: "> [!note]\n> ![[X]]\n", stamped: true},
		{name: "an embed inside fenced code is shown, not followed", body: "```\n![[X]]\n```\n", stamped: false},
		{name: "an embed inside a code span is quoted syntax", body: "The syntax `![[X]]` embeds.\n", stamped: false},
		{name: "an escaped embed is shown, not followed", body: "\\![[X]]\n", stamped: false},
		{name: "an unresolved embed brings no words", body: "![[Nowhere]]\n", stamped: false},
		{name: "a picture embed is not another note's words", body: "![[pic.png]]\n", stamped: false},
		{name: "an embed whose body was never captured", body: "![[Ghost]]\n", stamped: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := hostStamp(t, r, tt.body)
			if (got != "") != tt.stamped {
				t.Errorf("TranscludedIdentity of %q = %q, want stamped = %t", tt.body, got, tt.stamped)
			}
		})
	}
}

// TestTranscludedIdentityMovesWithTheExcerptAndOnlyTheExcerpt is the precision
// the stamp promises: an embed that names a section consumes that slice and
// nothing else, so an edit inside the slice moves the stamp and an edit
// elsewhere in the source note leaves it exactly where it was. Without the
// second half, every edit anywhere in an embedded note would invite a reload
// that delivers the same words — the offer that teaches a reader to stop
// believing the banner.
func TestTranscludedIdentityMovesWithTheExcerptAndOnlyTheExcerpt(t *testing.T) {
	t.Parallel()
	const host = "![[X#Sec]]\n"
	const base = "intro\n\n# Sec\n\nInside words.\n\n# Tail\n\nOutside words.\n"
	insideEdit := strings.Replace(base, "Inside words.", "Inside words, changed.", 1)
	outsideEdit := strings.Replace(base, "Outside words.", "Outside words, changed.", 1)
	if insideEdit == base || outsideEdit == base {
		t.Fatal("an edit fixture equals its base, so the comparisons below compare nothing")
	}

	project := func(xBody, hostBody string) (stamp, html string) {
		r := newRenderer(t, []graph.NoteInput{{RelPath: "X.md"}}, nil, transclusions{"X.md": xBody})
		res := r.HTML("host.md", "", hostBody, wording.ZhHant)
		return res.TranscludedIdentity, res.HTML
	}

	baseStamp, baseHTML := project(base, host)
	if baseStamp == "" {
		t.Fatal("the section embed stamped nothing, so nothing below can distinguish anything")
	}
	// The slice boundary the comparisons below lean on, proved on the page
	// itself: the excerpt carries the section and not the words after it.
	if !strings.Contains(baseHTML, "Inside words.") || strings.Contains(baseHTML, "Outside words.") {
		t.Fatalf("the rendered excerpt does not cut where these fixtures assume:\n%s", baseHTML)
	}

	if again, _ := project(base, host); again != baseStamp {
		t.Errorf("two renders of the same input stamped %q and %q; the stamp must be deterministic", baseStamp, again)
	}
	if inside, _ := project(insideEdit, host); inside == baseStamp {
		t.Error("an edit inside the embedded slice left the stamp unchanged; the page would never learn its words moved")
	}
	if outside, _ := project(outsideEdit, host); outside != baseStamp {
		t.Error("an edit outside the embedded slice moved the stamp; the page would offer a reload that returns the same words")
	}

	// A whole-note embed consumes the whole note, so the same outside edit is
	// inside its excerpt and must move the stamp.
	wholeBase, _ := project(base, "![[X]]\n")
	wholeEdited, _ := project(outsideEdit, "![[X]]\n")
	if wholeBase == "" || wholeBase == wholeEdited {
		t.Errorf("a whole-note embed's stamp did not follow the note's bytes: %q then %q", wholeBase, wholeEdited)
	}
}

// TestTranscludedIdentityBindsSourceScopeAndCount pins the parts of an excerpt
// that are not its bytes. Two notes can hold identical bytes and still render
// differently, because a relative image resolves against its own note's
// directory; a widened excerpt and a chosen-of-several excerpt both say so on
// the page; and the same note embedded twice is two excerpts. Each of these
// must reach the stamp, or a change a reload would show leaves it still.
func TestTranscludedIdentityBindsSourceScopeAndCount(t *testing.T) {
	t.Parallel()
	stampOf := func(notes []graph.NoteInput, bodies transclusions, host string) string {
		return hostStamp(t, newRenderer(t, notes, nil, bodies), host)
	}

	const shared = "The same words.\n"
	fromX := stampOf([]graph.NoteInput{{RelPath: "X.md"}}, transclusions{"X.md": shared}, "![[X]]\n")
	fromY := stampOf([]graph.NoteInput{{RelPath: "Y.md"}}, transclusions{"Y.md": shared}, "![[Y]]\n")
	if fromX == "" || fromX == fromY {
		t.Errorf("identical bytes from two sources stamped %q and %q; the source is part of the excerpt", fromX, fromY)
	}

	x := []graph.NoteInput{{RelPath: "X.md"}}
	once := stampOf(x, transclusions{"X.md": shared}, "![[X]]\n")
	twice := stampOf(x, transclusions{"X.md": shared}, "![[X]]\n\n![[X]]\n")
	if once == "" || once == twice {
		t.Errorf("one embed and two embeds of the same note stamped %q and %q", once, twice)
	}

	// A fragment that matches nothing widens to the whole note and the page
	// says so; an embed written without a fragment shows the same bytes with
	// no such notice. The two pages differ, so the stamps must.
	widened := stampOf(x, transclusions{"X.md": shared}, "![[X#Nope]]\n")
	plain := stampOf(x, transclusions{"X.md": shared}, "![[X]]\n")
	if widened == "" || widened == plain {
		t.Errorf("a widened excerpt and a plain whole-note excerpt stamped %q and %q", widened, plain)
	}

	// Two headings answering one name: the excerpt is the first and identical
	// either way, and the page counts the candidates beside it. Only that
	// count separates these two sources, so only the count can move the stamp.
	const sectionOnce = "# Sec\n\nWords.\n\n# Other\n\nTail.\n"
	const sectionTwice = "# Sec\n\nWords.\n\n# Other\n\nTail.\n\n# Sec\n\nAgain.\n"
	chosen := stampOf(x, transclusions{"X.md": sectionOnce}, "![[X#Sec]]\n")
	ofSeveral := stampOf(x, transclusions{"X.md": sectionTwice}, "![[X#Sec]]\n")
	if chosen == "" || chosen == ofSeveral {
		t.Errorf("the only candidate and the first of two stamped %q and %q; the page says which it is showing", chosen, ofSeveral)
	}
}

// TestTranscludedIdentityStopsWhereTheRenderStops pins the walk to the
// render's own depth. Expansion stops one level down, so a note embedded
// inside an embedded note never puts words on this page — and its edits must
// not move this page's stamp. A note that embeds itself is the degenerate
// case of the same rule, and the render finishing at all is the proof it
// holds.
func TestTranscludedIdentityStopsWhereTheRenderStops(t *testing.T) {
	t.Parallel()
	notes := []graph.NoteInput{{RelPath: "B.md"}, {RelPath: "C.md"}}
	const bBody = "From B.\n\n![[C]]\n"
	stampWithC := func(cBody string) string {
		return hostStamp(t, newRenderer(t, notes, nil, transclusions{"B.md": bBody, "C.md": cBody}), "![[B]]\n")
	}

	deep := stampWithC("C before.\n")
	if deep == "" {
		t.Fatal("the host embeds B and stamped nothing")
	}
	if after := stampWithC("C after.\n"); after != deep {
		t.Error("an edit two levels down moved the stamp; those words never reach this page")
	}
	if moved := hostStamp(t,
		newRenderer(t, notes, nil, transclusions{"B.md": "From B, changed.\n\n![[C]]\n", "C.md": "C before.\n"}),
		"![[B]]\n",
	); moved == deep {
		t.Error("an edit in the directly embedded note left the stamp unchanged")
	}

	self := newRenderer(t, []graph.NoteInput{{RelPath: "A.md"}}, nil, transclusions{"A.md": "Myself: ![[A]]\n"})
	res := self.HTML("A.md", "", "Myself: ![[A]]\n", wording.ZhHant)
	if res.TranscludedIdentity == "" {
		t.Error("a note embedding itself is still one expanded excerpt, and stamped nothing")
	}

	mutual := newRenderer(t,
		[]graph.NoteInput{{RelPath: "A.md"}, {RelPath: "B.md"}},
		nil,
		transclusions{"A.md": "A cites: ![[B]]\n", "B.md": "B cites: ![[A]]\n"},
	)
	if got := mutual.HTML("A.md", "", "A cites: ![[B]]\n", wording.ZhHant).TranscludedIdentity; got == "" {
		t.Error("two notes embedding each other still expand one level, and stamped nothing")
	}
}

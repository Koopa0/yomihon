package render_test

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestAMissingFileIsNotCalledAMissingNote separates two absences the page used
// to describe in one sentence. A name with a picture's extension behind it is
// a file the author meant to show, and telling them no note answers to it
// sends them looking for the wrong thing to write.
//
// The kinds that get the file wording are the ones this program already reads
// an extension for when it chooses how to display something. Nothing wider is
// guessed: the vault's own notes include one called "Go sync.Pool", and a rule
// that took any trailing dot for a file type would call it a missing picture.
func TestAMissingFileIsNotCalledAMissingNote(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)

	picture := r.HTML("note.md", "", "![[figure-3.png]]\n", wording.En)
	if strings.Contains(picture.HTML, "note called") {
		t.Errorf("a missing picture is described as a missing note:\n%s", picture.HTML)
	}
	if !strings.Contains(picture.HTML, "file called") {
		t.Errorf("a missing picture is not described as a missing file:\n%s", picture.HTML)
	}

	note := r.HTML("note.md", "", "![[Some Concept]]\n", wording.En)
	if !strings.Contains(note.HTML, "note called") {
		t.Errorf("a missing note stopped being described as one:\n%s", note.HTML)
	}

	// The control the vault itself supplies: a note whose name ends in
	// something that looks like an extension is still a note.
	dotted := r.HTML("note.md", "", "![[Go sync.Pool]]\n", wording.En)
	if !strings.Contains(dotted.HTML, "note called") {
		t.Errorf("a note whose name ends in a dotted word was called a file:\n%s", dotted.HTML)
	}
}

// TestADegradedEmbedEchoesWhatItsAuthorWrote holds the echo to the source. An
// embed the page cannot expand is shown as the text that produced it, and a
// reader comparing the page with their file has to find the same characters in
// both. Dropping the fragment showed them something they never typed and
// removed the very part that explains why the citation is scoped.
func TestADegradedEmbedEchoesWhatItsAuthorWrote(t *testing.T) {
	t.Parallel()

	r := newRenderer(t, []graph.NoteInput{{RelPath: "a/Dup.md"}, {RelPath: "b/Dup.md"}, {RelPath: "Held.md"}}, nil, transclusions{
		"a/Dup.md": "## A\n\nx\n",
		"b/Dup.md": "## B\n\ny\n",
	})

	for _, c := range []struct{ name, body, want string }{
		{"a name nothing answers to", "![[NoSuch#Sec]]\n", "![[NoSuch#Sec]]"},
		{"a name several files answer to", "![[Dup#Sec]]\n", "![[Dup#Sec]]"},
		{"a note this generation did not capture", "![[Held#Sec]]\n", "![[Held#Sec]]"},
	} {
		got := r.HTML("note.md", "", c.body, wording.En)
		if !strings.Contains(got.HTML, c.want) {
			t.Errorf("%s: the page does not echo what the author wrote (%q):\n%s", c.name, c.want, got.HTML)
		}
	}
}

// TestAnAmbiguousEmbedExplainsItselfLikeAnAmbiguousLink closes the second of
// the two places a shared name is reported. The plain-link side was given a
// sentence and a copy of it a reader who is listening receives; the embed side
// kept a bare list of paths in an attribute, which a pointer reveals and
// nothing else does.
func TestAnAmbiguousEmbedExplainsItselfLikeAnAmbiguousLink(t *testing.T) {
	t.Parallel()

	r := newRenderer(t, []graph.NoteInput{{RelPath: "a/Dup.md"}, {RelPath: "b/Dup.md"}}, nil, transclusions{
		"a/Dup.md": "## A\n\nx\n",
		"b/Dup.md": "## B\n\ny\n",
	})
	got := r.HTML("note.md", "", "![[Dup]]\n", wording.En)

	at := strings.Index(got.HTML, `class="wikilink-ambiguous"`)
	if at < 0 {
		t.Fatalf("the page carries no ambiguous embed, so this proves nothing:\n%s", got.HTML)
	}
	span := got.HTML[at:]
	if end := strings.Index(span, "</span></span>"); end >= 0 {
		span = span[:end]
	}
	if !strings.Contains(span, "does not guess") {
		t.Errorf("the ambiguous embed carries a bare list rather than a sentence; span = %q", span)
	}
	if !strings.Contains(span, "y-offscreen") {
		t.Errorf("the ambiguous embed's explanation is reachable only by pointing at it; span = %q", span)
	}
}

// TestAnEmbedInsideAnEmbedSaysItWasNotExpanded covers the one degradation that
// erased its own shape. An embed written inside a transcluded body renders as
// an ordinary link, so the page shows a citation where the author wrote an
// excerpt and nothing anywhere says the difference.
//
// The depth stops at one on purpose, and that is not what changes here. What
// changes is that the excerpt says so, once, above the words it contains.
func TestAnEmbedInsideAnEmbedSaysItWasNotExpanded(t *testing.T) {
	t.Parallel()

	r := newRenderer(t, []graph.NoteInput{{RelPath: "Outer.md"}, {RelPath: "Inner.md"}}, nil, transclusions{
		"Outer.md": "OUTERTEXT\n\n![[Inner]]\n",
		"Inner.md": "INNERTEXT\n",
	})
	got := r.HTML("note.md", "", "![[Outer]]\n", wording.En)

	if !strings.Contains(got.HTML, "OUTERTEXT") {
		t.Fatalf("the fixture's premise is wrong: the outer excerpt did not render:\n%s", got.HTML)
	}
	if strings.Contains(got.HTML, "INNERTEXT") {
		t.Fatalf("the fixture's premise is wrong: the inner embed expanded after all:\n%s", got.HTML)
	}
	if !strings.Contains(got.HTML, "embed__note") {
		t.Errorf("an excerpt whose own embed became a link says nothing about it:\n%s", got.HTML)
	}

	var reported int
	for _, d := range got.Diagnostics {
		if d.Kind == render.DiagEmbedNotExpanded {
			reported++
		}
	}
	if reported != 1 {
		t.Errorf("want one not-expanded diagnostic for the note's own account, got %d", reported)
	}

	// The control: an excerpt containing no embed of its own gains nothing.
	plain := r.HTML("note.md", "", "![[Inner]]\n", wording.En)
	if strings.Contains(plain.HTML, "embed__note") {
		t.Errorf("an excerpt with no embed of its own was given the notice:\n%s", plain.HTML)
	}
}

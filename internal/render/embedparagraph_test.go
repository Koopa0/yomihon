package render_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/wording"
)

// divInsideBareParagraph reports whether any goldmark-emitted paragraph — a
// bare <p>, the only kind that pass writes — carries a raw <div before its
// close. A browser repairs that markup by closing the paragraph at the div,
// which turns whatever followed into a bare text node outside any paragraph;
// the renderer must therefore never hand it over.
func divInsideBareParagraph(html string) bool {
	for i := 0; ; {
		p := strings.Index(html[i:], "<p>")
		if p < 0 {
			return false
		}
		p += i + len("<p>")
		end := strings.Index(html[p:], "</p>")
		if end < 0 {
			return false
		}
		if strings.Contains(html[p:p+end], "<div") {
			return true
		}
		i = p + end
	}
}

var emptyParagraph = regexp.MustCompile(`<p>\s*</p>`)

// TestAnEmbedMidSentenceLeavesItsParagraphValid pins the shape a paragraph
// takes when its author wrote an embed in the middle of a sentence: the prose
// before it stays a paragraph, the excerpt stands as its own block, and the
// prose after it is a paragraph again. The excerpt's container is a div, and a
// div left inside the paragraph was markup the browser repaired by closing the
// paragraph early — the trailing words then reached the reader as a bare text
// node no paragraph owned.
func TestAnEmbedMidSentenceLeavesItsParagraphValid(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{RelPath: "B.md"}}, nil, transclusions{
		"B.md": "B's own body text.\n",
	})
	got := r.HTML("note.md", "", "Before ![[B]] after.\n", wording.ZhHant)

	if divInsideBareParagraph(got.HTML) {
		t.Errorf("a paragraph still carries the embed's div inside it:\n%s", got.HTML)
	}
	if !strings.Contains(got.HTML, `<div class="embed">`) {
		t.Fatalf("the fixture's premise is wrong: no embed rendered:\n%s", got.HTML)
	}
	if !strings.Contains(got.HTML, "<p>Before </p>") {
		t.Errorf("the words before the embed lost their paragraph:\n%s", got.HTML)
	}
	if !strings.Contains(got.HTML, "<p> after.</p>") {
		t.Errorf("the words after the embed lost their paragraph:\n%s", got.HTML)
	}
}

// TestAnEmbedAloneInItsParagraphLeavesNoEmptyParagraph covers the ordinary
// spelling — the embed on a line of its own. Parting the paragraph there
// leaves nothing on either side, and an empty <p></p> is not something the
// author wrote.
func TestAnEmbedAloneInItsParagraphLeavesNoEmptyParagraph(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{RelPath: "B.md"}}, nil, transclusions{
		"B.md": "B's own body text.\n",
	})
	got := r.HTML("note.md", "", "![[B]]\n", wording.ZhHant)

	if divInsideBareParagraph(got.HTML) {
		t.Errorf("a paragraph still carries the embed's div inside it:\n%s", got.HTML)
	}
	if !strings.Contains(got.HTML, `<div class="embed">`) {
		t.Fatalf("the fixture's premise is wrong: no embed rendered:\n%s", got.HTML)
	}
	if m := emptyParagraph.FindString(got.HTML); m != "" {
		t.Errorf("parting the paragraph left an empty one (%q):\n%s", m, got.HTML)
	}
}

// TestAdjacentEmbedsLeaveNoEmptyParagraphBetween puts two embeds in one
// sentence: the whitespace between them is not a paragraph either.
func TestAdjacentEmbedsLeaveNoEmptyParagraphBetween(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{RelPath: "B.md"}, {RelPath: "C.md"}}, nil, transclusions{
		"B.md": "b body\n",
		"C.md": "c body\n",
	})
	got := r.HTML("note.md", "", "x ![[B]] ![[C]] y\n", wording.ZhHant)

	if divInsideBareParagraph(got.HTML) {
		t.Errorf("a paragraph still carries an embed's div inside it:\n%s", got.HTML)
	}
	if n := strings.Count(got.HTML, `<div class="embed">`); n != 2 {
		t.Fatalf("the fixture's premise is wrong: %d embeds rendered, want 2:\n%s", n, got.HTML)
	}
	if m := emptyParagraph.FindString(got.HTML); m != "" {
		t.Errorf("parting the paragraph left an empty one (%q):\n%s", m, got.HTML)
	}
	if !strings.Contains(got.HTML, "<p>x </p>") || !strings.Contains(got.HTML, "<p> y</p>") {
		t.Errorf("the prose around the embeds lost its paragraphs:\n%s", got.HTML)
	}
}

// TestAnEmbedInsideABlockquoteStaysInsideIt parts the quoted paragraph the
// same way: the excerpt's div is legal inside the blockquote, so the quote
// keeps it — only the paragraph opens around it.
func TestAnEmbedInsideABlockquoteStaysInsideIt(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{RelPath: "B.md"}}, nil, transclusions{
		"B.md": "b body\n",
	})
	got := r.HTML("note.md", "", "> quoted ![[B]] words\n", wording.ZhHant)

	if divInsideBareParagraph(got.HTML) {
		t.Errorf("the quoted paragraph still carries the embed's div inside it:\n%s", got.HTML)
	}
	at := strings.Index(got.HTML, "<blockquote>")
	if at < 0 {
		t.Fatalf("the fixture's premise is wrong: no blockquote rendered:\n%s", got.HTML)
	}
	quote := got.HTML[at:]
	if end := strings.Index(quote, "</blockquote>"); end >= 0 {
		quote = quote[:end]
	}
	if !strings.Contains(quote, `<div class="embed">`) {
		t.Errorf("the embed left the blockquote it was written in:\n%s", got.HTML)
	}
}

// TestAMediaPlaceholderMidSentenceLeavesItsParagraphValid covers the other
// block an embed can put mid-line: the labelled placeholder a non-note file
// gets. Its container is a div like the excerpt's, and the paragraph has to
// open around it the same way.
func TestAMediaPlaceholderMidSentenceLeavesItsParagraphValid(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, []string{"Diagrams/x.canvas"}, nil)
	got := r.HTML("note.md", "", "Before ![[x.canvas]] after.\n", wording.ZhHant)

	if divInsideBareParagraph(got.HTML) {
		t.Errorf("a paragraph still carries the media placeholder's div inside it:\n%s", got.HTML)
	}
	if !strings.Contains(got.HTML, `class="embed-media"`) {
		t.Fatalf("the fixture's premise is wrong: no media placeholder rendered:\n%s", got.HTML)
	}
}

// TestAPictureEmbedStaysInItsParagraph is the control: an image is phrasing
// content and belongs inside the sentence that mentions it, so nothing is
// parted for one.
func TestAPictureEmbedStaysInItsParagraph(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, []string{"pix.png"}, nil)
	got := r.HTML("note.md", "", "Before ![[pix.png]] after.\n", wording.ZhHant)

	if !strings.Contains(got.HTML, `<p>Before <img src="/raw/pix.png" alt="pix.png"> after.</p>`) {
		t.Errorf("a picture embed no longer stays inside its paragraph:\n%s", got.HTML)
	}
}

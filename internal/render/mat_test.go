package render_test

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestOnlyAnSVGOnALineOfItsOwnIsSetOnAMat holds what the mat is for. A vector
// drawing standing alone in its paragraph is an author's own, drawn for a light
// ground, and it is set on a mat so a dark page does not make a hole of it. A
// raster picture or a photograph is not that and is left as it was. A picture
// inside a sentence is part of the sentence, and the mat is a block: it would
// put a tinted bar through the line.
//
// Each row is one shape the renderer writes a picture in, so a pattern that
// matched too much or too little names the row it got wrong.
func TestOnlyAnSVGOnALineOfItsOwnIsSetOnAMat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		body   string
		matted bool
	}{
		{name: "markdown svg alone", body: "![a drawing](attachments/diagram.svg)\n", matted: true},
		{name: "wikilink svg alone", body: "![[diagram.svg]]\n", matted: true},
		{name: "wikilink svg with a width", body: "![[diagram.svg|300]]\n", matted: true},
		{name: "svg with a query", body: "![a drawing](attachments/diagram.svg?v=2)\n", matted: true},
		{name: "svg with a fragment", body: "![a drawing](attachments/diagram.svg#view)\n", matted: true},
		{name: "an extension in capitals", body: "![[Upper.SVG]]\n", matted: true},
		{name: "markdown png alone", body: "![a picture](attachments/pic.png)\n", matted: false},
		{name: "wikilink png alone", body: "![[pic.png]]\n", matted: false},
		{name: "jpeg alone", body: "![[photo.jpg]]\n", matted: false},
		{name: "svg is only part of a raster's name", body: "![a picture](attachments/not.svg.png)\n", matted: false},
		{name: "svg is only in a raster's query", body: "![a picture](attachments/pic.png?as=.svg)\n", matted: false},
		{name: "svg beside words", body: "Before ![a drawing](attachments/diagram.svg) after.\n", matted: false},
		{name: "wikilink svg beside words", body: "Before ![[diagram.svg]] after.\n", matted: false},
		{name: "two svgs in one paragraph", body: "![one](attachments/diagram.svg) ![two](attachments/diagram.svg)\n", matted: false},
		{name: "an svg that is also a link", body: "[![a drawing](attachments/diagram.svg)](https://example.invalid/)\n", matted: false},
		{name: "a remote svg is a link, not a drawing", body: "![remote](https://example.invalid/drawing.svg)\n", matted: false},
		{name: "an svg leaving the vault is not served", body: "![outside](../../outside.svg)\n", matted: false},
		{name: "a paragraph the author wrote as markup is text, not a mat", body: "<p class=\"mat\"><img src=\"/raw/attachments/diagram.svg\"></p>\n", matted: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newRenderer(t, nil, []string{"attachments/diagram.svg", "attachments/Upper.SVG", "attachments/pic.png", "attachments/photo.jpg", "attachments/not.svg.png"}, nil)
			got := r.HTML("note.md", "", tt.body, wording.ZhHant).HTML
			if has := strings.Contains(got, `<p class="mat"><img src="/raw/`); has != tt.matted {
				t.Errorf("mat present = %v, want %v:\n%s", has, tt.matted, got)
			}
			if !tt.matted && strings.Contains(got, `class="mat"`) {
				t.Errorf("a mat was drawn where none belongs:\n%s", got)
			}
		})
	}
}

// TestAnSVGOnAMatIsOtherwiseTheMarkupItWas holds the whole paragraph: the
// picture's own tag, with its source and its alternative, is what sits inside
// the mat, unchanged.
func TestAnSVGOnAMatIsOtherwiseTheMarkupItWas(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, []string{"attachments/diagram.svg"}, nil)
	got := r.HTML("note.md", "", "![[diagram.svg]]\n", wording.ZhHant).HTML
	const want = "<p class=\"mat\"><img src=\"/raw/attachments/diagram.svg\" alt=\"diagram.svg\"></p>\n"
	if got != want {
		t.Errorf("HTML().HTML = %q, want exactly %q", got, want)
	}
}

// TestAnSVGInAnExcerptIsSetOnAMat: an excerpt carries another note's body into
// this page, and a drawing in that body is still the author's drawing.
func TestAnSVGInAnExcerptIsSetOnAMat(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{RelPath: "Target.md"}}, []string{"diagram.svg"}, transclusions{
		"Target.md": "Words.\n\n![a drawing](diagram.svg)\n",
	})
	got := r.HTML("note.md", "", "![[Target]]\n", wording.ZhHant).HTML
	if !strings.Contains(got, `<p class="mat"><img src="/raw/diagram.svg"`) {
		t.Errorf("an SVG alone in an excerpt's paragraph is not on a mat:\n%s", got)
	}
}

// holdsNoPicture is a vault in which every file a note reaches for is missing.
type holdsNoPicture struct{}

func (holdsNoPicture) MissingFile(string) bool { return true }

func (holdsNoPicture) OutsideScan(string) bool { return false }

func (holdsNoPicture) IsDirectory(string) bool { return false }

// TestAnSVGTheVaultDoesNotHoldIsNotSetOnAMat: a drawing whose file is not there
// is marked where the author put it, and that marker is what stands in the
// paragraph. A mat around a broken-image icon would present a hole as if it
// were a drawing.
func TestAnSVGTheVaultDoesNotHoldIsNotSetOnAMat(t *testing.T) {
	t.Parallel()
	r := render.New(graph.BuildFromNotes(nil, []string{"attachments/gone.svg"}), transclusions{}, noTitles{}, holdsNoPicture{})
	got := r.HTML("note.md", "", "![gone](attachments/gone.svg)\n", wording.En).HTML
	if !strings.Contains(got, `class="image-missing"`) {
		t.Fatalf("the premise is wrong: the missing picture is not marked:\n%s", got)
	}
	if strings.Contains(got, `class="mat"`) {
		t.Errorf("an SVG the vault does not hold is on a mat:\n%s", got)
	}
}

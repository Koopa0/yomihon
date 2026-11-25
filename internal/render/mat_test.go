package render_test

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestAPictureOnALineOfItsOwnIsSetOnAPlate holds what the plate is for. A
// picture standing alone in its paragraph is a figure, and it is set on the
// mat's ground. A vector drawing's mat takes the system's light or dark, which
// is what the drawing's own stylesheet answers to; a raster picture is the same
// in either, so its mat takes the page's. A picture inside a sentence is part
// of the sentence, and the plate is a block: it would put a tinted bar through
// the line.
//
// Each row is one shape the renderer writes a picture in, so a pattern that
// matched too much or too little names the row it got wrong.
func TestAPictureOnALineOfItsOwnIsSetOnAPlate(t *testing.T) {
	t.Parallel()

	const (
		system = `<figure class="mat"><img src="/raw/`
		page   = `<figure class="mat mat--page"><img src="/raw/`
	)
	tests := []struct {
		name  string
		body  string
		plate string
	}{
		{name: "markdown svg alone", body: "![a drawing](attachments/diagram.svg)\n", plate: system},
		{name: "wikilink svg alone", body: "![[diagram.svg]]\n", plate: system},
		{name: "wikilink svg with a width", body: "![[diagram.svg|300]]\n", plate: system},
		{name: "svg with a query", body: "![a drawing](attachments/diagram.svg?v=2)\n", plate: system},
		{name: "svg with a fragment", body: "![a drawing](attachments/diagram.svg#view)\n", plate: system},
		{name: "an extension in capitals", body: "![[Upper.SVG]]\n", plate: system},
		{name: "markdown png alone", body: "![a picture](attachments/pic.png)\n", plate: page},
		{name: "wikilink png alone", body: "![[pic.png]]\n", plate: page},
		{name: "jpeg alone", body: "![[photo.jpg]]\n", plate: page},
		{name: "svg is only part of a raster's name", body: "![a picture](attachments/not.svg.png)\n", plate: page},
		{name: "svg is only in a raster's query", body: "![a picture](attachments/pic.png?as=.svg)\n", plate: page},
		{name: "svg beside words", body: "Before ![a drawing](attachments/diagram.svg) after.\n"},
		{name: "wikilink svg beside words", body: "Before ![[diagram.svg]] after.\n"},
		{name: "two svgs in one paragraph", body: "![one](attachments/diagram.svg) ![two](attachments/diagram.svg)\n"},
		{name: "an svg that is also a link", body: "[![a drawing](attachments/diagram.svg)](https://example.invalid/)\n"},
		{name: "a remote svg is a link, not a drawing", body: "![remote](https://example.invalid/drawing.svg)\n"},
		{name: "an svg leaving the vault is not served", body: "![outside](../../outside.svg)\n"},
		{name: "a paragraph the author wrote as markup is text, not a plate", body: "<figure class=\"mat\"><img src=\"/raw/attachments/diagram.svg\"></figure>\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newRenderer(t, nil, []string{"attachments/diagram.svg", "attachments/Upper.SVG", "attachments/pic.png", "attachments/photo.jpg", "attachments/not.svg.png"}, nil)
			got := r.HTML("note.md", "", tt.body, wording.ZhHant).HTML
			plates := strings.Count(got, "<figure")
			if tt.plate == "" {
				if plates != 0 {
					t.Errorf("a plate was drawn where none belongs:\n%s", got)
				}
				return
			}
			if plates != 1 || !strings.Contains(got, tt.plate) {
				t.Errorf("want one plate opening %q:\n%s", tt.plate, got)
			}
		})
	}
}

// TestAPlateIsCaptionedOnlyByWordsTheAuthorGave holds where a caption comes
// from: the alt text the author wrote, and nothing else. An embed with no words
// of its own is given its file's name for its alt, which says nothing a caption
// should, and an empty alt is no words at all. The caption repeats the alt, so
// it is hidden from a screen reader, which already reads the alt.
func TestAPlateIsCaptionedOnlyByWordsTheAuthorGave(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		body    string
		caption string
	}{
		{name: "markdown alt", body: "![The two transactions](attachments/diagram.svg)\n", caption: "The two transactions"},
		{name: "an embed's words", body: "![[diagram.svg|The two transactions]]\n", caption: "The two transactions"},
		{name: "markdown alt beside a width", body: "![The two transactions|300](attachments/diagram.svg)\n", caption: "The two transactions"},
		{name: "a raster picture's alt", body: "![A photograph](attachments/pic.png)\n", caption: "A photograph"},
		{name: "alt that needs escaping", body: "![A < B & \"C\"](attachments/diagram.svg)\n", caption: "A &lt; B &amp; &quot;C&quot;"},
		{name: "an embed with no words", body: "![[diagram.svg]]\n"},
		{name: "an embed with only a width", body: "![[diagram.svg|300]]\n"},
		{name: "an empty alt", body: "![](attachments/diagram.svg)\n"},
		{name: "an alt of blank space", body: "![  ](attachments/diagram.svg)\n"},
		{name: "an alt that is the file's name", body: "![diagram.svg](attachments/diagram.svg)\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newRenderer(t, nil, []string{"attachments/diagram.svg", "attachments/pic.png"}, nil)
			got := r.HTML("note.md", "", tt.body, wording.ZhHant).HTML
			if strings.Count(got, "<figure") != 1 {
				t.Fatalf("the premise is wrong: no plate:\n%s", got)
			}
			captions := strings.Count(got, "<figcaption")
			if tt.caption == "" {
				if captions != 0 {
					t.Errorf("a caption the author did not write:\n%s", got)
				}
				return
			}
			if want := `<figcaption aria-hidden="true">` + tt.caption + `</figcaption></figure>`; captions != 1 || !strings.Contains(got, want) {
				t.Errorf("want the caption %q:\n%s", want, got)
			}
		})
	}
}

// TestAPictureWiderThanTheTextIsWide holds when a picture stands at the wide
// measure: when the width the author gave it is more than the text's measure
// less the mat's inset either side, at each measure it misses. A picture given
// no width is never wide, since its own pixels are not a reading size: a
// screenshot taken at twice the screen's density is twice as many pixels as it
// was meant to be read at.
func TestAPictureWiderThanTheTextIsWide(t *testing.T) {
	t.Parallel()
	tests := []struct {
		body string
		wide string
	}{
		{body: "![[diagram.svg|300]]\n", wide: ""},
		{body: "![[diagram.svg]]\n", wide: ""},
		{body: "![[diagram.svg|471]]\n", wide: ""},
		{body: "![[diagram.svg|472]]\n", wide: "latin-m"},
		{body: "![[diagram.svg|534]]\n", wide: "latin-l latin-m"},
		{body: "![[diagram.svg|604]]\n", wide: "latin-xl latin-l latin-m"},
		{body: "![[diagram.svg|605]]\n", wide: "latin-xl latin-l latin-m m"},
		{body: "![[diagram.svg|900x300]]\n", wide: "latin-xl latin-l latin-m xl l m"},
		{body: "![wide|900](attachments/pic.png)\n", wide: "latin-xl latin-l latin-m xl l m"},
		{body: "Inside a sentence ![[diagram.svg|900]] it is not.\n", wide: ""},
	}
	for _, tt := range tests {
		t.Run(tt.body, func(t *testing.T) {
			t.Parallel()
			r := newRenderer(t, nil, []string{"attachments/diagram.svg", "attachments/pic.png"}, nil)
			got := r.HTML("note.md", "", tt.body, wording.ZhHant).HTML
			want := ` data-wide="` + tt.wide + `"`
			if tt.wide == "" {
				if strings.Contains(got, "data-wide") {
					t.Errorf("a picture that fits names a measure:\n%s", got)
				}
				return
			}
			if !strings.Contains(got, want+`><img`) {
				t.Errorf("want the plate to carry%s:\n%s", want, got)
			}
		})
	}
}

// TestAPictureOnAPlateIsOtherwiseTheMarkupItWas holds the whole paragraph: the
// picture's own tag, with its source and its alternative, is what sits on the
// plate, unchanged.
func TestAPictureOnAPlateIsOtherwiseTheMarkupItWas(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, []string{"attachments/diagram.svg"}, nil)
	got := r.HTML("note.md", "", "![[diagram.svg]]\n", wording.ZhHant).HTML
	const want = "<figure class=\"mat\"><img src=\"/raw/attachments/diagram.svg\" alt=\"diagram.svg\"></figure>\n"
	if got != want {
		t.Errorf("HTML().HTML = %q, want exactly %q", got, want)
	}
}

// TestAnSVGInAnExcerptIsSetOnAPlate: an excerpt carries another note's body
// into this page, and a drawing in that body is still the author's drawing.
func TestAnSVGInAnExcerptIsSetOnAPlate(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{RelPath: "Target.md"}}, []string{"diagram.svg"}, transclusions{
		"Target.md": "Words.\n\n![a drawing](diagram.svg)\n",
	})
	got := r.HTML("note.md", "", "![[Target]]\n", wording.ZhHant).HTML
	if !strings.Contains(got, `<figure class="mat"><img src="/raw/diagram.svg"`) {
		t.Errorf("an SVG alone in an excerpt's paragraph is not on a plate:\n%s", got)
	}
}

// holdsNoPicture is a vault in which every file a note reaches for is missing.
type holdsNoPicture struct{}

func (holdsNoPicture) MissingFile(string) bool { return true }

func (holdsNoPicture) OutsideScan(string) bool { return false }

func (holdsNoPicture) IsDirectory(string) bool { return false }

// TestAnSVGTheVaultDoesNotHoldIsNotSetOnAPlate: a drawing whose file is not
// there is marked where the author put it, and that marker is what stands in the
// paragraph. A plate around a broken-image icon would present a hole as if it
// were a drawing.
func TestAnSVGTheVaultDoesNotHoldIsNotSetOnAPlate(t *testing.T) {
	t.Parallel()
	r := render.New(graph.BuildFromNotes(nil, []string{"attachments/gone.svg"}), transclusions{}, noTitles{}, holdsNoPicture{})
	got := r.HTML("note.md", "", "![gone](attachments/gone.svg)\n", wording.En).HTML
	if !strings.Contains(got, `class="image-missing"`) {
		t.Fatalf("the premise is wrong: the missing picture is not marked:\n%s", got)
	}
	if strings.Contains(got, `class="mat`) {
		t.Errorf("an SVG the vault does not hold is on a plate:\n%s", got)
	}
}

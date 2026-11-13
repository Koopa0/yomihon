package render_test

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestAPictureOnALineOfItsOwnIsSetOnAMat holds what the mat is for. A picture
// standing alone in its paragraph is an author's drawing, and a drawing made
// for a light ground is set on a mat so a dark page does not make a hole of
// it. A picture inside a sentence is part of the sentence, and the mat is a
// block: it would put a tinted bar through the line.
//
// Each row is one shape the renderer writes a picture in, so a pattern that
// matched too much or too little names the row it got wrong.
func TestAPictureOnALineOfItsOwnIsSetOnAMat(t *testing.T) {
	t.Parallel()

	const mat = `<p class="mat"><img src="/raw/attachments/pic.png"`
	tests := []struct {
		name   string
		body   string
		matted bool
	}{
		{name: "markdown picture alone", body: "![a drawing](attachments/pic.png)\n", matted: true},
		{name: "wikilink picture alone", body: "![[pic.png]]\n", matted: true},
		{name: "wikilink picture with a width", body: "![[pic.png|300]]\n", matted: true},
		{name: "markdown picture beside words", body: "Before ![a drawing](attachments/pic.png) after.\n", matted: false},
		{name: "wikilink picture beside words", body: "Before ![[pic.png]] after.\n", matted: false},
		{name: "two pictures in one paragraph", body: "![one](attachments/pic.png) ![two](attachments/pic.png)\n", matted: false},
		{name: "a picture that is also a link", body: "[![a drawing](attachments/pic.png)](https://example.invalid/)\n", matted: false},
		{name: "a remote picture is a link, not a drawing", body: "![remote](https://example.invalid/pixel.png)\n", matted: false},
		{name: "a picture leaving the vault is not served", body: "![outside](../../outside.png)\n", matted: false},
		{name: "a paragraph the author wrote as markup is text, not a mat", body: "<p class=\"mat\"><img src=\"/raw/attachments/pic.png\"></p>\n", matted: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newRenderer(t, nil, []string{"attachments/pic.png"}, nil)
			got := r.HTML("note.md", "", tt.body, wording.ZhHant).HTML
			if has := strings.Contains(got, mat); has != tt.matted {
				t.Errorf("mat present = %v, want %v:\n%s", has, tt.matted, got)
			}
			if !tt.matted && strings.Contains(got, `class="mat"`) {
				t.Errorf("a mat was drawn where none belongs:\n%s", got)
			}
		})
	}
}

// TestAPictureInAnExcerptIsSetOnAMat: an excerpt carries another note's body
// into this page, and a drawing in that body is still the author's drawing.
func TestAPictureInAnExcerptIsSetOnAMat(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{RelPath: "Target.md"}}, []string{"pic.png"}, transclusions{
		"Target.md": "Words.\n\n![a drawing](pic.png)\n",
	})
	got := r.HTML("note.md", "", "![[Target]]\n", wording.ZhHant).HTML
	if !strings.Contains(got, `<p class="mat"><img src="/raw/pic.png"`) {
		t.Errorf("a picture alone in an excerpt's paragraph is not on a mat:\n%s", got)
	}
}

// holdsNoPicture is a vault in which every file a note reaches for is missing.
type holdsNoPicture struct{}

func (holdsNoPicture) MissingFile(string) bool { return true }

func (holdsNoPicture) OutsideScan(string) bool { return false }

func (holdsNoPicture) IsDirectory(string) bool { return false }

// TestAPictureTheVaultDoesNotHoldIsNotSetOnAMat: a picture whose file is not
// there is marked where the author put it, and that marker is what stands in
// the paragraph. A mat around a broken-image icon would present a hole as if it
// were a drawing.
func TestAPictureTheVaultDoesNotHoldIsNotSetOnAMat(t *testing.T) {
	t.Parallel()
	r := render.New(graph.BuildFromNotes(nil, []string{"attachments/gone.png"}), transclusions{}, noTitles{}, holdsNoPicture{})
	got := r.HTML("note.md", "", "![gone](attachments/gone.png)\n", wording.En).HTML
	if !strings.Contains(got, `class="image-missing"`) {
		t.Fatalf("the premise is wrong: the missing picture is not marked:\n%s", got)
	}
	if strings.Contains(got, `class="mat"`) {
		t.Errorf("a picture the vault does not hold is on a mat:\n%s", got)
	}
}

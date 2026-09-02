package render

import (
	"strings"
	"testing"
)

// These tests pin the substitution pass's contract on the rendered document
// directly, because the pass is what turns reserved markers back into
// renderer-owned markup: exactly the planted markers are redeemed, each one
// once, and every byte that is not a planted marker survives unchanged. The
// fixture strings below stand for goldmark output, so the markers appear the
// way that output carries them — comments as raw HTML blocks, the private-use
// pairs inside whatever text surrounds them.

func TestSubstituteBlocksRedeemsEachMarkerKind(t *testing.T) {
	t.Parallel()
	blocks := []string{`<div class="callout">first</div>`, `<div class="mermaid-diagram">second</div>`}
	inline := []string{`<a href="/notes/a.md">a</a>`, `<div class="embed">wide</div>`}
	doc := "<p>before</p>\n<!--yomihon-block:0-->\n<p>x 0 y</p>\n<!--yomihon-block:1-->\n<p>tail 1</p>\n"
	got := substituteBlocks(doc, blocks, inline)
	want := "<p>before</p>\n" + blocks[0] + "\n<p>x " + inline[0] + " y</p>\n" + blocks[1] +
		"\n<p>tail </p>" + inline[1] + "\n"
	if got != want {
		t.Errorf("substituteBlocks:\n got %q\nwant %q", got, want)
	}
}

func TestSubstituteBlocksRedeemsFirstOccurrenceOnly(t *testing.T) {
	t.Parallel()
	inline := []string{"<a>once</a>"}
	doc := "<p>0 and again 0</p>"
	got := substituteBlocks(doc, nil, inline)
	want := "<p><a>once</a> and again 0</p>"
	if got != want {
		t.Errorf("duplicate marker:\n got %q\nwant %q", got, want)
	}
	if strings.Count(got, "<a>once</a>") != 1 {
		t.Errorf("replacement spliced %d times, want 1", strings.Count(got, "<a>once</a>"))
	}
}

func TestSubstituteBlocksLeavesUnplantedShapesAlone(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		doc    string
		blocks []string
		inline []string
	}{
		{
			name:   "index beyond the planted set",
			doc:    "<p><!--yomihon-block:5--> and 7</p>",
			blocks: []string{"<div>b</div>"},
			inline: []string{"<a>i</a>"},
		},
		{
			name:   "non-canonical digits name no marker",
			doc:    "<p><!--yomihon-block:01--> and 00</p>",
			blocks: []string{"<div>a</div>", "<div>b</div>"},
			inline: []string{"<a>x</a>", "<a>y</a>"},
		},
		{
			name:   "empty and unterminated digit runs",
			doc:    "<p><!--yomihon-block:--> then  then 0 stray</p>",
			blocks: []string{"<div>a</div>"},
			inline: []string{"<a>x</a>"},
		},
		{
			name:   "inline pair holding block-shaped markup",
			doc:    "<p>0 stays</p>",
			inline: []string{`<div class="embed">block shaped</div>`},
		},
		{
			name:   "a run longer than any planted count names no marker",
			doc:    "<p>9999999999999999999 tail</p>",
			blocks: []string{"<div>a</div>"},
			inline: []string{"<a>x</a>"},
		},
		{
			name: "nothing planted at all",
			doc:  "<p><!--yomihon-block:0--> and 0</p>",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := substituteBlocks(tt.doc, tt.blocks, tt.inline); got != tt.doc {
				t.Errorf("substituteBlocks changed bytes it does not own:\n got %q\nwant %q", got, tt.doc)
			}
		})
	}
}

// A block-markup pair whose index holds inline-shaped markup is not redeemed,
// yet the paragraph around it is still parted: parting reads the pair's shape
// alone, before any redeeming decision, and that order is part of the frozen
// output.
func TestSubstituteBlocksPartsAroundAPairItNeverRedeems(t *testing.T) {
	t.Parallel()
	inline := []string{"<a>inline shaped</a>"}
	got := substituteBlocks("<p>a 0 b</p>", nil, inline)
	want := "<p>a </p>0<p> b</p>"
	if got != want {
		t.Errorf("unredeemed block-markup pair:\n got %q\nwant %q", got, want)
	}
}

// A marker's position in the rendered document is not its position in the
// planted order: goldmark moves a footnote definition to the document's foot,
// so a link written inside one is planted early and rendered late. The pass
// redeems by index, wherever each marker landed.
func TestSubstituteBlocksRedeemsMarkersOutOfIndexOrder(t *testing.T) {
	t.Parallel()
	inline := []string{"<a>zero</a>", "<a>one</a>", "<a>two</a>"}
	doc := "<p>2 then 0 then 1</p>"
	got := substituteBlocks(doc, nil, inline)
	want := "<p><a>two</a> then <a>zero</a> then <a>one</a></p>"
	if got != want {
		t.Errorf("out-of-order markers:\n got %q\nwant %q", got, want)
	}
}

func TestSubstituteBlocksPartsParagraphsAroundBlockMarkup(t *testing.T) {
	t.Parallel()
	inline := []string{`<div class="embed">borrowed</div>`}
	doc := "<p>lead 0 tail</p>"
	got := substituteBlocks(doc, nil, inline)
	want := "<p>lead </p>" + inline[0] + "<p> tail</p>"
	if got != want {
		t.Errorf("parting:\n got %q\nwant %q", got, want)
	}
}

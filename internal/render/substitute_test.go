package render

import (
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
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

// FuzzSubstituteBlocks locks the invariant the one-walk substitution rests
// on: the three marker families' delimiters share no bytes, so a complete
// marker of one family never contains another family's opening, and the
// leftmost pending candidate can never sit inside a span an earlier
// redemption already consumed. A rewrite that loses that property fails here
// as a slice-out-of-range panic on the very next splice — on a real page,
// that is every render of every heavily cited note — so the first property is
// simply that no input panics. The second is deliberately conservative:
// every input byte outside any marker-shaped substring must reach the output
// in order, once the bytes paragraph parting may lawfully drop (whitespace
// and bare paragraph tags) are set aside. Refused openers and unplanted
// marker shapes are preserved verbatim by the pass, and the property never
// demands more than preservation, so it holds for them as well.
func FuzzSubstituteBlocks(f *testing.F) {
	block0 := `<div class="callout">zero</div>`
	block1 := `<div class="mermaid-diagram">one</div>`
	narrow := `<a href="/notes/a.md">a</a>`
	wide := `<div class="embed">wide</div>`
	for _, doc := range []string{
		"",
		"<p>before</p>\n" + blockPlaceholder(0) + "\n<p>x " + inlinePlaceholder(0) + " y</p>\n" +
			blockPlaceholder(1) + "\n<p>tail " + blockMarkupPlaceholder(1) + "</p>\n",
		// A refused wide opener hard against a redeemed comment marker.
		wideMarkOpen + blockPlaceholder(0),
		// A refused comment opener hard against a redeemed inline marker.
		blockMarkOpen + inlinePlaceholder(0),
		// A refused inline opener whose would-be index is a redeemed wide
		// marker, closed by a dangling inline closer.
		inlineMarkOpen + blockMarkupPlaceholder(1) + inlineMarkClose,
		// Parting around a wide marker, with whitespace-only fringes.
		"<p> " + blockMarkupPlaceholder(1) + " tail</p><p>plain</p>",
		// Duplicates out of index order: the first occurrence of each index
		// is redeemed, later ones stay text.
		inlinePlaceholder(1) + " then " + inlinePlaceholder(0) + " and " + inlinePlaceholder(0),
		// Shapes that name no marker: non-canonical digits, an index nothing
		// planted, a run longer than any planted count, a bare closer.
		blockMarkOpen + "01" + blockMarkClose + " " + inlinePlaceholder(7) + " " +
			wideMarkOpen + "99999999999" + wideMarkClose + " " + inlineMarkClose,
		// Stray lead bytes of the private-use runes beside whole markers.
		"\xee\x80" + inlinePlaceholder(0) + "\xee" + blockMarkupPlaceholder(1) + "\x80",
		// Markers of all three families packed against single non-space
		// bytes, so a walk that eats one byte beside a marker loses a byte
		// the parting fold cannot excuse.
		"A" + inlinePlaceholder(0) + "B" + blockMarkupPlaceholder(1) + "C" + blockPlaceholder(0) + "D",
	} {
		f.Add(doc, block0, block1, narrow, wide)
	}

	f.Fuzz(func(t *testing.T, doc, b0, b1, i0, i1 string) {
		const maxDoc = 16 << 10
		const maxMarkup = 1 << 10
		if len(doc) > maxDoc || len(b0) > maxMarkup || len(b1) > maxMarkup ||
			len(i0) > maxMarkup || len(i1) > maxMarkup {
			return
		}
		got := substituteBlocks(doc, []string{b0, b1}, []string{i0, i1})

		var outside strings.Builder
		last := 0
		for _, span := range markerShapedSpans(doc) {
			outside.WriteString(doc[last:span[0]])
			last = span[1]
		}
		outside.WriteString(doc[last:])
		want := strippedOfPartingBytes(outside.String())
		if !containsInOrder(want, got) {
			t.Fatalf("substituteBlocks(%q, [%q %q], [%q %q]) = %q, which loses non-marker input bytes %q",
				doc, b0, b1, i0, i1, got, want)
		}
	})
}

// markerShapedSpans reports every substring of doc that spells a complete
// marker of any family — opening delimiter, canonical decimal index, closing
// delimiter — whether or not anything planted it. Complete shapes cannot
// overlap, because no family's opening occurs inside a complete marker's
// bytes, so a single left-to-right scan finds them all. These spans are
// exactly the bytes substitution may consume; every byte outside them it
// must preserve, and the shape of an index is read by the same markerIndex
// the pass itself redeems through, so the two cannot drift.
func markerShapedSpans(doc string) [][2]int {
	families := []struct{ open, close string }{
		{blockMarkOpen, blockMarkClose},
		{inlineMarkOpen, inlineMarkClose},
		{wideMarkOpen, wideMarkClose},
	}
	var spans [][2]int
	for i := 0; i < len(doc); {
		width := 0
		for _, fam := range families {
			if !strings.HasPrefix(doc[i:], fam.open) {
				continue
			}
			if _, end, ok := markerIndex(doc[i+len(fam.open):], fam.close); ok {
				width = len(fam.open) + end
			}
			break
		}
		if width == 0 {
			i++
			continue
		}
		spans = append(spans, [2]int{i, i + width})
		i += width
	}
	return spans
}

// strippedOfPartingBytes removes from s every byte paragraph parting is
// allowed to drop: bare paragraph tags and whitespace, the latter judged by
// the same fold TrimSpace applies so a multi-byte space is dropped whole. A
// byte that is not valid UTF-8 is kept as itself — replacing it with the
// replacement character would demand a byte the input never held.
func strippedOfPartingBytes(s string) string {
	s = strings.ReplaceAll(s, "<p>", "")
	s = strings.ReplaceAll(s, "</p>", "")
	var b strings.Builder
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if !unicode.IsSpace(r) {
			b.WriteString(s[i : i+size])
		}
		i += size
	}
	return b.String()
}

// containsInOrder reports whether every byte of want appears in got in the
// same order, with arbitrary insertions allowed between them. Greedy
// matching is complete for this question: taking the earliest possible byte
// each time never forecloses a later match.
func containsInOrder(want, got string) bool {
	j := 0
	for i := 0; i < len(got) && j < len(want); i++ {
		if got[i] == want[j] {
			j++
		}
	}
	return j == len(want)
}

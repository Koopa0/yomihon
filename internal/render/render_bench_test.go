package render_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

// benchLinkTargets is how many distinct notes the generated body cites. A
// citation-heavy note reaches many different targets rather than one, so the
// resolver is exercised against an index with more than a single name in it.
const benchLinkTargets = 64

// benchTargetNotes builds the cited notes, so every generated link resolves
// to exactly one existing file and the benchmark measures the ordinary
// working-link path rather than the degraded ones.
func benchTargetNotes() []graph.NoteInput {
	notes := make([]graph.NoteInput, 0, benchLinkTargets)
	for i := range benchLinkTargets {
		notes = append(notes, graph.NoteInput{RelPath: fmt.Sprintf("T%d.md", i)})
	}
	return notes
}

// benchManyLinksBody generates prose carrying n wikilinks, eight to a
// paragraph. Rendering cost was observed to grow with the number of links
// multiplied by the document's length, so the fixture holds both large at
// once: every link adds source text around it the way a heavily cited note
// does.
func benchManyLinksBody(n int) string {
	var b strings.Builder
	for i := range n {
		fmt.Fprintf(&b, "[[T%d|w%d]] lorem ipsum dolor sit amet ", i%benchLinkTargets, i)
		if i%8 == 7 {
			b.WriteString("\n\n")
		}
	}
	return b.String()
}

// benchFixedLinks is how many wikilinks the length-only fixture carries at
// every size. It is constant, so whatever growth that benchmark shows, the
// number of citations is not what produced it.
const benchFixedLinks = 64

// benchLongBody generates as many prose items as benchManyLinksBody, only the
// first benchFixedLinks of them citing anything. Each plain item carries the
// same words as a citing one without the brackets, so the two bodies differ by
// a few bytes per item and not in shape.
func benchLongBody(n int) string {
	var b strings.Builder
	for i := range n {
		if i < benchFixedLinks {
			fmt.Fprintf(&b, "[[T%d|w%d]] lorem ipsum dolor sit amet ", i%benchLinkTargets, i)
		} else {
			fmt.Fprintf(&b, "T%d w%d lorem ipsum dolor sit amet ", i%benchLinkTargets, i)
		}
		if i%8 == 7 {
			b.WriteString("\n\n")
		}
	}
	return b.String()
}

// BenchmarkHTMLManyWikilinks pins the cost of rendering a citation-heavy
// note. The three sizes double twice, so the growth order is visible in one
// run: a linear pipeline roughly doubles per step, and one that rescans the
// document per link roughly quadruples.
//
// Read alone it cannot say which factor grew, because its fixture grows both
// at once: a quadrupling here is equally consistent with cost quadratic in the
// citations, quadratic in the length, or the product of the two. It is read
// against BenchmarkHTMLLongBodyFewWikilinks, whose body grows the same way
// while the citations stay put.
func BenchmarkHTMLManyWikilinks(b *testing.B) {
	r := render.New(graph.BuildFromNotes(benchTargetNotes(), nil), transclusions{}, noTitles{}, vaultHolds{})
	for _, n := range []int{8000, 16000, 32000} {
		body := benchManyLinksBody(n)
		b.Run(fmt.Sprintf("links-%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				res := r.HTML("host.md", "", body, wording.ZhHant)
				if res.HTML == "" {
					b.Fatal("render produced no HTML")
				}
			}
		})
	}
}

// benchSectionTargetBodies gives every cited note a body carrying the heading
// the links name. A link naming a section is only checked against a body the
// reader captured, so without these the check never runs.
func benchSectionTargetBodies() transclusions {
	bodies := make(transclusions, benchLinkTargets)
	for i := range benchLinkTargets {
		var b strings.Builder
		for j := range 64 {
			fmt.Fprintf(&b, "## Section %d\n\nlorem ipsum dolor sit amet\n\n", j)
		}
		b.WriteString("## Head\n\nthe named one\n")
		bodies[fmt.Sprintf("T%d.md", i)] = b.String()
	}
	return bodies
}

// BenchmarkHTMLWikilinksIntoSections is the one that reaches the work the other
// two never do. Both of those cite a note by name alone, and a bare name is
// answered from the resolver's index; naming a section inside the note is what
// makes the renderer read that note's body looking for the heading, once for
// every link. That is the cost this file exists to watch, and until this case
// was here it was measured zero times: the fixtures wrote no "#", and the
// stand-in for captured bodies answered that it held none.
func BenchmarkHTMLWikilinksIntoSections(b *testing.B) {
	r := render.New(graph.BuildFromNotes(benchTargetNotes(), nil), benchSectionTargetBodies(), noTitles{}, vaultHolds{})
	for _, n := range []int{8000, 16000, 32000} {
		var body strings.Builder
		for i := range n {
			fmt.Fprintf(&body, "[[T%d#Head|w%d]] lorem ipsum dolor sit amet ", i%benchLinkTargets, i)
			if i%8 == 7 {
				body.WriteString("\n\n")
			}
		}
		source := body.String()
		b.Run(fmt.Sprintf("links-%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				res := r.HTML("host.md", "", source, wording.ZhHant)
				if res.HTML == "" {
					b.Fatal("render produced no HTML")
				}
			}
		})
	}
}

// BenchmarkHTMLLongBodyFewWikilinks grows a note's length while its citation
// count stays where it started, which is what makes the citation-heavy numbers
// readable. The two run at the same sizes over bodies of the same length, so
// subtracting this one from that one leaves the work the citations cost, and
// the allocation counts separate further than the times do. A regression can
// then say which factor moved, rather than leaving a reader to guess from one
// number that grew.
func BenchmarkHTMLLongBodyFewWikilinks(b *testing.B) {
	r := render.New(graph.BuildFromNotes(benchTargetNotes(), nil), transclusions{}, noTitles{}, vaultHolds{})
	for _, n := range []int{8000, 16000, 32000} {
		body := benchLongBody(n)
		b.Run(fmt.Sprintf("items-%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				res := r.HTML("host.md", "", body, wording.ZhHant)
				if res.HTML == "" {
					b.Fatal("render produced no HTML")
				}
			}
		})
	}
}

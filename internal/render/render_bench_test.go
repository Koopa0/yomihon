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

// BenchmarkHTMLManyWikilinks pins the cost of rendering a citation-heavy
// note. The three sizes double twice, so the growth order is visible in one
// run: a linear pipeline roughly doubles per step, and one that rescans the
// document per link roughly quadruples.
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

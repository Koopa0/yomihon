package render

import (
	"slices"

	"github.com/koopa0/yomihon/internal/graph"
)

// HeadingLines maps the id of each heading in r's contents to the line of body
// it opens on, counted from 0, for a result rendered from title and body. A
// reader's editor is sent to that line, and a wrong line is worse than none, so
// a heading the source cannot vouch for is left out: one in a callout, a quote
// or an embedded note, which the line walk does not see, and any one whose
// words another heading of the page shares in a way that leaves the match open.
//
// The source's headings are read by the walk every other cut of a body uses,
// over the body with its comments off, whose lines are still the body's; the
// heading the page's title stands in for is not one of them. They are matched
// in order, by level and by the name the page folds their words to, against
// the contents, whose ids already carry the suffixes the page gave repeated
// names. A heading is given its line only where the earliest and the latest
// way of matching them all put it at the same entry.
func (r *Result) HeadingLines(title, body string) map[string]int {
	type key struct {
		level int
		name  string
	}
	page := make([]key, len(r.TOC))
	for i, entry := range r.TOC {
		page[i] = key{entry.Level, graph.SectionID(entry.Text)}
	}
	stripped, _ := stripBody(body)
	_, _, titleLine, dropped := removeBodyFirstH1(title, stripped.text)
	var lines []int
	var source []key
	for _, h := range graph.Headings(stripped.text, graph.LineSkipZones(stripped.text)) {
		k := key{h.Level, graph.SectionID(headingSourceText(h.Text, h.Level))}
		// A heading whose words the page shows nowhere can take no entry, so
		// leaving it out cannot move another heading's match.
		if dropped > 0 && h.Line == titleLine || !slices.Contains(page, k) {
			continue
		}
		lines = append(lines, h.Line)
		source = append(source, k)
	}

	earliest := make([]int, len(source))
	at := 0
	for i, k := range source {
		next := slices.Index(page[at:], k)
		if next < 0 {
			// The source holds these headings in an order the page does not,
			// so no match of any of them can be vouched for.
			return nil
		}
		earliest[i] = at + next
		at = earliest[i] + 1
	}
	found := make(map[string]int, len(source))
	at = len(page) - 1
	for i, k := range slices.Backward(source) {
		// An earliest match of every heading exists, so a latest one does.
		for page[at] != k {
			at--
		}
		if at == earliest[i] {
			found[r.TOC[at].ID] = lines[i]
		}
		at--
	}
	return found
}

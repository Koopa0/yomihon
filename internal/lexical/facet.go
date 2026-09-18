package lexical

import (
	"cmp"
	"maps"
	"slices"
)

// facetTally accumulates the divisions while the entries are walked, one cell
// per value actually met. It exists so the walk that answers the query also
// answers what the answer is made of, which is the whole reason a division can
// be offered at all: counting it afterwards would mean either a second walk of
// every entry — the same substring scans, paid twice — or a count taken over
// the built stretch of results, which is short by the tail the limit dropped.
type facetTally struct {
	// stated and unstated run parallel to facetKeys rather than being keyed by
	// it, because the order a reader meets the keys in is that slice's and
	// nothing here should get to have a second opinion about it.
	stated   []map[string]*facetCount
	unstated []int
}

// facetCount is one value met while walking: how many hits are at it, the
// spelling the first note to carry it gave it, the folded form the index
// compares, and the note types that carried it. The folded form is kept
// because it, and not the spelling, is what decides two notes are at the same
// value — and because it orders values that share a count without letting the
// answer depend on which spelling arrived first.
type facetCount struct {
	written  string
	folded   string
	count    int
	carriers map[string]bool
}

// add tallies one hit. An entry the artifact policy places outside instance
// metadata is counted as stating nothing, at every key: a narrowed query asks
// for metadata and would leave that entry out, so counting its frontmatter
// here would promise a value the offer cannot deliver.
func (t *facetTally) add(e *entry) {
	if t.stated == nil {
		t.stated = make([]map[string]*facetCount, len(facetKeys))
		t.unstated = make([]int, len(facetKeys))
	}
	for i, key := range facetKeys {
		written, folded := "", ""
		if e.metadataCapable {
			written, folded = e.facetValue(key)
		}
		if folded == "" {
			t.unstated[i]++
			continue
		}
		if t.stated[i] == nil {
			t.stated[i] = make(map[string]*facetCount)
		}
		cell := t.stated[i][folded]
		if cell == nil {
			cell = &facetCount{written: written, folded: folded, carriers: make(map[string]bool)}
			t.stated[i][folded] = cell
		}
		cell.count++
		cell.carriers[e.NoteType] = true
	}
}

// facets writes the tally out in the order a reader meets the keys, each key's
// values by descending count and then by folded value, so the same index and
// the same query answer with the same bytes every time. Nothing tallied means
// nothing matched, or metadata was unavailable and no narrowing could be
// honoured; either way there is no division to offer.
func (t *facetTally) facets() []Facet {
	if t.stated == nil {
		return nil
	}
	out := make([]Facet, 0, len(facetKeys))
	for i, key := range facetKeys {
		division := Facet{Key: key, Unstated: t.unstated[i]}
		cells := make([]*facetCount, 0, len(t.stated[i]))
		for _, cell := range t.stated[i] {
			cells = append(cells, cell)
		}
		slices.SortFunc(cells, func(left, right *facetCount) int {
			return cmp.Or(
				cmp.Compare(right.count, left.count),
				cmp.Compare(left.folded, right.folded),
			)
		})
		division.Values = make([]FacetValue, len(cells))
		for at, cell := range cells {
			division.Values[at] = FacetValue{
				Value:    cell.written,
				Count:    cell.count,
				Carriers: slices.Sorted(maps.Keys(cell.carriers)),
			}
		}
		out = append(out, division)
	}
	return out
}

// facetValue returns what this entry declared for one key, as written and
// folded. A key outside the divisions answers with neither, which is what
// keeps this from quietly growing a fourth division: a key added to facetKeys
// with no arm here would tally every hit as stating nothing, and the test that
// asks each key for its own field is where that shows up.
func (e *entry) facetValue(key string) (written, folded string) {
	switch key {
	case StatusFilterKey:
		return e.Status, e.StatusFold
	case "type":
		return e.NoteType, e.NoteTypeFold
	case "domain":
		return e.Domain, e.DomainFold
	default:
		return "", ""
	}
}

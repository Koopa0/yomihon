package judge

import (
	"maps"

	"github.com/koopa0/yomihon/internal/egress"
	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/schema"
)

// A wikilink that resolves to nothing is not automatically a fault. The vault
// is written forward: a note lists the concepts it still owes under a heading
// marked as a gap, and then links those names from wherever they belong long
// before the target file exists. Those links are tracked, not broken, and the
// distinction is what separates a diagnostic worth reading from noise.
//
// Both faces have to reach the same verdict: the check writes it into its
// findings, the reading page paints it beside the prose, and the health page
// lists it. So the vocabulary, the harvesting and the verdict live here, and
// the pages consult them rather than reproducing them.

// Planned is the set of concept names a corpus has declared it intends to
// write. Membership is stored under the resolver's own normalization, so a name
// matches here exactly when it would have matched a real note; the set keeps
// its map to itself so nothing can enter unnormalized. The zero value is a
// usable empty set, which is what a corpus that declared nothing means.
type Planned struct {
	names map[string]bool
}

// NewPlanned harvests the concept names one note body declares it still owes,
// against the heading and inline marks the contract declares; a nil contract
// harvests against the default marks. A caller harvests note by note, so a
// note a parser cannot survive costs that note, and joins the sets with
// MergePlanned. Which notes to harvest is the caller's decision: the check
// harvests only from notes the rules file lets out, since a name planned in a
// private note must not soften a public broken link, while the reading page
// harvests from all of them.
func NewPlanned(body string, contract *schema.Contract) Planned {
	set := Planned{names: make(map[string]bool)}
	set.add(extractPlannedNamesWith(body, plannedMarksFrom(contract)))
	return set
}

// MergePlanned is the union of sets: the names every one of them declared.
func MergePlanned(sets ...Planned) Planned {
	merged := Planned{names: make(map[string]bool)}
	for _, set := range sets {
		maps.Copy(merged.names, set.names)
	}
	return merged
}

// add folds harvested names into the set, dropping any that normalize away to
// nothing. Both faces enter the set through here so neither can fold a name a
// different way than the other. It is reachable only from the two constructors
// in this package, which is why it may assume the set was built rather than
// left at its zero value.
func (p Planned) add(names []string) {
	for _, name := range names {
		if key := graph.NormalizeKey(name); key != "" {
			p.names[key] = true
		}
	}
}

// Has reports whether target names a concept the corpus has declared planned.
// It normalizes the target the way the resolver does, so the answer agrees with
// what resolution would have found had the note been written.
func (p Planned) Has(target string) bool {
	return p.names[graph.NormalizeKey(target)]
}

// Citation is one wikilink a note's body writes, as the link rules read it.
type Citation struct {
	// Target is the name the link resolves by.
	Target string
	// Owed says the link sits in a section a gap heading opens, which tracks it
	// as owed rather than broken.
	Owed bool
}

// Citations returns the wikilinks body cites, in document order, with each one
// read against the gap headings the contract declares. A nil contract reads
// against the default marks. It leaves out what LinkTargets leaves out.
func Citations(body string, contract *schema.Contract) []Citation {
	links := extractWikilinksWith(body, 1, plannedMarksFrom(contract).heading)
	cited := make([]Citation, len(links))
	for i, link := range links {
		cited[i] = Citation{Target: link.target, Owed: link.underGapHeading}
	}
	return cited
}

// LinkStanding is how the link rules read one wikilink.
type LinkStanding int

const (
	// LinkResolves is a link a file answers to. A name several files share is
	// here too: the name-collision rule reports it.
	LinkResolves LinkStanding = iota
	// LinkTitleOnly is a link to a name that is a note's title and is no
	// filename or alias, so it resolves to nothing (link.title_not_alias).
	LinkTitleOnly
	// LinkOwed is a link to nothing that is tracked as owed: it sits under a
	// gap heading or names a planned concept (link.broken at info).
	LinkOwed
	// LinkUnwritten is a link to nothing that nothing tracks (link.broken at
	// warn).
	LinkUnwritten
)

// Standing reads one citation the way the link rules do, so the check and the
// health page give one verdict on it. titled says whether the target is the
// title of some note the caller may describe: the caller decides which notes
// those are, the check counting only what the rules file lets out and the
// health page every note it holds.
func Standing(c Citation, idx *graph.Index, planned Planned, titled bool) LinkStanding {
	switch {
	case idx.Names(c.Target):
		return LinkResolves
	case titled:
		return LinkTitleOnly
	case c.Owed || planned.Has(c.Target):
		return LinkOwed
	default:
		return LinkUnwritten
	}
}

// LinkTargets returns the wikilinks body cites, in document order, each with
// the bytes it occupies, leaving out the ones a reader never wrote: a link
// inside a code fence or span is quoted syntax, one inside an Obsidian comment
// is content the author took back, and a bare anchor names no note. It is
// what an egress gate reads links with; Citations is the same reading for the
// pages.
func LinkTargets(body string) []egress.Link {
	links := extractWikilinks(body, 1)
	cited := make([]egress.Link, 0, len(links))
	for _, link := range links {
		cited = append(cited, egress.Link{Target: link.target, Start: link.start, End: link.end})
	}
	return cited
}

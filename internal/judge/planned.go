package judge

import (
	"iter"

	"github.com/koopa0/yomihon/internal/graph"
)

// A wikilink that resolves to nothing is not automatically a fault. The vault
// is written forward: a note lists the concepts it still owes under a heading
// marked as a gap, and then links those names from wherever they belong long
// before the target file exists. Those links are tracked, not broken, and the
// distinction is what separates a diagnostic worth reading from noise — across
// this vault only two of fifty-eight unresolved links are genuine faults.
//
// Both faces have to reach the same verdict. The adjudicator writes it into a
// frozen wire format; the reading page paints it beside the prose. When the two
// derive it separately they drift, and the reader learns to distrust whichever
// one cries wolf. So the vocabulary, the harvesting, and the predicate live
// here, and the reading page consults them rather than reproducing them.

// Planned is the set of concept names a corpus has declared it intends to
// write. Membership is stored under the resolver's own normalization, so a name
// matches here exactly when it would have matched a real note.
type Planned map[string]bool

// NewPlanned harvests the declared-but-unwritten concept names from every note
// body in bodies.
//
// Which bodies to feed it is the caller's decision, and the two faces answer it
// differently on purpose. The adjudicator's findings are an artifact that can
// leave the machine, so it harvests only from notes the contract allows to
// egress: a name planned solely in a private note must not soften a public
// broken link, or a reader of the finding could infer that private content
// names that target. The reading page never leaves the machine and its only
// reader can already open every note in the vault, so it harvests from all of
// them. The corpus differs; the verdict, once the names are in hand, does not.
func NewPlanned(bodies iter.Seq[string]) Planned {
	set := make(Planned)
	for body := range bodies {
		set.add(extractPlannedNames(body))
	}
	return set
}

// add folds harvested names into the set, dropping any that normalize away to
// nothing. Both faces enter the set through here so neither can fold a name a
// different way than the other.
func (p Planned) add(names []string) {
	for _, name := range names {
		if key := graph.NormalizeKey(name); key != "" {
			p[key] = true
		}
	}
}

// Has reports whether target names a concept the corpus has declared planned.
// It normalizes the target the way the resolver does, so the answer agrees with
// what resolution would have found had the note been written.
func (p Planned) Has(target string) bool {
	return p[graph.NormalizeKey(target)]
}

// LinkTargets returns the wikilink targets body cites, in document order, with
// the ones a reader never wrote left out: a link inside a code fence or a code
// span is quoted syntax rather than a citation, one inside an Obsidian comment
// is content the author took back, and a bare anchor names no note at all.
//
// It is exported so the reading face can build the reverse of the link graph
// from the same reading of a body the adjudicator uses. The two derive their
// edges from one extractor, so a page listing what cites a note and a finding
// about a citation can never disagree about which citations exist.
func LinkTargets(body string) []string {
	links := extractWikilinks(body, 1)
	targets := make([]string, 0, len(links))
	for _, link := range links {
		targets = append(targets, link.target)
	}
	return targets
}

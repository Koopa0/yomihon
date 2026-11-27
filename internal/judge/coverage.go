package judge

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/koopa0/yomihon/internal/egress"
	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/schema"
)

// coverageReport says how well the concept corpus is filed: per-domain concept
// counts and each concept's mount state, the concepts not yet on an index, and
// the true orphans. A concept is classified by the source of its inbound edges:
// an edge from a note whose type the rules file declares as an index mounts
// it, an edge only from another kind of note leaves it pending, and no inbound
// edge at all is a true orphan, the only real problem. Which types are concepts
// and which are indexes are the vault's own words, so a vault that names no
// concept type is told so rather than handed a zero tally.
//
// Every field is always present; the slices are empty rather than absent when
// there is nothing to list.
type coverageReport struct {
	TotalConcepts int              `json:"total_concepts"`
	Domains       []domainCoverage `json:"domains"`
	// PendingMount lists concepts reached only by a non-index edge.
	PendingMount []string `json:"pending_mount"`
	// Orphans lists concepts with no inbound edge at all.
	Orphans []string `json:"orphans"`
	// NotApplicable says why this vault's rules file leaves the concept tally
	// with nothing to be about. It is omitted for a vault that declares the
	// type.
	NotApplicable string `json:"not_applicable,omitempty"`
}

// domainCoverage is one domain's concept counts by mount state.
type domainCoverage struct {
	Domain       string `json:"domain"`
	Concepts     int    `json:"concepts"`
	Mounted      int    `json:"mounted"`
	PendingMount int    `json:"pending_mount"`
	Orphan       int    `json:"orphan"`
}

// computeCoverage classifies every concept in the corpus. It first walks the
// edges to learn which concepts are reached and whether by an index, then
// tallies each concept. Notes outside the declared knowledge layer are out of
// scope throughout, and withheld paths are excluded, so a private note never
// appears here even when typed as a concept.
func computeCoverage(notes []note, idx *graph.Index, authority scanAuthority) coverageReport {
	conceptType, declared := authority.contract.ConceptType()
	if !declared {
		notApplicable := "the rules file declares no " + strconv.Quote(conceptType) +
			" type, so there is no concept corpus to judge"
		if authority.noRules() {
			notApplicable = egress.NoRulesFile + " There is no concept type to count."
		}
		return coverageReport{
			Domains:       []domainCoverage{},
			PendingMount:  []string{},
			Orphans:       []string{},
			NotApplicable: notApplicable,
		}
	}

	scope := authority.contract.KnowledgeScope()
	mapped, referenced := mountEdges(notes, idx, scope, authority)
	rows := make(map[string]*domainCoverage)
	pending := []string{}
	orphans := []string{}
	total := 0
	for i := range notes {
		n := &notes[i]
		if n.noteType != conceptType || !scope.Includes(n.path) || !authority.gate.Allows(n.path) {
			continue
		}
		total++
		domain := n.domain
		if domain == "" {
			domain = noDomain
		}
		row := rows[domain]
		if row == nil {
			row = &domainCoverage{Domain: domain}
			rows[domain] = row
		}
		row.Concepts++
		switch {
		case mapped[n.path]:
			row.Mounted++
		case referenced[n.path]:
			row.PendingMount++
			pending = append(pending, n.path)
		default:
			row.Orphan++
			orphans = append(orphans, n.path)
		}
	}

	domains := make([]domainCoverage, 0, len(rows))
	for _, d := range slices.Sorted(maps.Keys(rows)) {
		domains = append(domains, *rows[d])
	}
	slices.Sort(pending)
	slices.Sort(orphans)
	return coverageReport{
		TotalConcepts: total,
		Domains:       domains,
		PendingMount:  pending,
		Orphans:       orphans,
	}
}

// mountEdges walks the edges to learn which concepts are reached, and which of
// those are reached by an index: a note whose type the rules file declares as
// one. A note outside the knowledge layer or under withheld ground is no
// source, so its links cannot decide a public concept's state or let a reader
// infer what the private note cites.
func mountEdges(notes []note, idx *graph.Index, scope schema.KnowledgeScope, authority scanAuthority) (mapped, referenced map[string]bool) {
	mapped = make(map[string]bool)
	referenced = make(map[string]bool)
	roles := authority.contract.NavigationRoles()
	for i := range notes {
		n := &notes[i]
		if !scope.Includes(n.path) || !authority.gate.Allows(n.path) {
			continue
		}
		fromIndex := roles.IsMapType(n.noteType)
		for _, path := range Reaches(idx, coverageTargets(n)) {
			referenced[path] = true
			if fromIndex {
				mapped[path] = true
			}
		}
	}
	return mapped, referenced
}

// coverageTargets is every name a note's edges cite: its body wikilink targets
// and its provenance references (based_on, related).
func coverageTargets(n *note) []string {
	out := make([]string, 0, len(n.wikilinks)+len(n.basedOn)+len(n.related))
	for _, w := range n.wikilinks {
		out = append(out, w.target)
	}
	out = append(out, n.basedOn...)
	out = append(out, n.related...)
	return out
}

// Reaches resolves the names one note's edges cite — body link targets, or
// frontmatter references written bare or as [[links]] — to the notes they
// reach. A unique name reaches its note, a name several files share reaches
// every one of them, so a concept cited by an ambiguous name is never counted
// an orphan, and an unresolved name reaches nothing. Coverage counts a concept
// as mounted when a note of an index type reaches it.
func Reaches(idx *graph.Index, names []string) []string {
	var out []string
	for _, name := range names {
		target, ok := referenceTarget(name)
		if !ok {
			continue
		}
		res := idx.Resolve(target)
		switch res.Kind {
		case graph.KindUnique:
			out = append(out, res.RelPath)
		case graph.KindAmbiguous:
			out = append(out, res.Candidates...)
		case graph.KindUnresolved:
		default:
			panic("judge: unknown graph.Kind: " + res.Kind.String())
		}
	}
	return out
}

// renderCoverage renders the coverage report for a terminal reader.
func renderCoverage(c *coverageReport) string {
	var s strings.Builder
	if c.NotApplicable != "" {
		s.WriteString(c.NotApplicable)
		s.WriteString("\n")
	} else {
		fmt.Fprintf(&s, "%d concepts across %d domains\n", c.TotalConcepts, len(c.Domains))
	}
	for _, d := range c.Domains {
		fmt.Fprintf(&s, "  %-16s %d concepts: %d mounted, %d pending-mount, %d orphan\n",
			d.Domain, d.Concepts, d.Mounted, d.PendingMount, d.Orphan)
	}
	if len(c.Orphans) > 0 {
		fmt.Fprintf(&s, "orphans (%d):\n", len(c.Orphans))
		for _, p := range c.Orphans {
			fmt.Fprintf(&s, "  %s\n", p)
		}
	}
	return s.String()
}

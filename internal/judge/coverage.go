package judge

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/koopa0/yomihon/internal/graph"
)

// Coverage reports how well the concept corpus is filed: per-domain concept
// counts and each concept's mount state, the concepts not yet on a map, the
// true orphans, and the routable notes that have not reached their index. A
// concept is classified by the source of its inbound edges, not its own status
// — a concept sits at seedling whether mapped or not. An edge from a note whose
// type the contract declares as a map mounts it; an edge only from another kind
// of note leaves it pending on a map; no inbound edge at all is a true orphan,
// the only real problem. The shape is part of the frozen output.
//
// Which types are concepts and which are maps are both the vault's own words,
// read from vault-schema.toml. A vault that names no concept type has no such
// corpus, and is told so rather than handed a zero tally that would read as a
// vault with nothing wrong.

// Coverage is the whole coverage report. Every field is always present; the
// slices are empty rather than absent when there is nothing to list.
type Coverage struct {
	TotalConcepts int              `json:"total_concepts"`
	Domains       []DomainCoverage `json:"domains"`
	// PendingMount lists concepts reached only by a non-map edge, not yet
	// mounted on a map (advisory).
	PendingMount []string `json:"pending_mount"`
	// Orphans lists concepts with no inbound edge at all.
	Orphans []string `json:"orphans"`
	// Unrouted lists non-concept routable notes not yet on their index.
	Unrouted []Unrouted `json:"unrouted"`
	// NotApplicable says why this vault's contract leaves the concept tally
	// with nothing to be about. It is absent from the report of a vault that
	// declares the type, so the answer for such a vault is unchanged.
	NotApplicable string `json:"not_applicable,omitempty"`
}

// DomainCoverage is one domain's concept counts by mount state.
type DomainCoverage struct {
	Domain       string `json:"domain"`
	Concepts     int    `json:"concepts"`
	Mounted      int    `json:"mounted"`
	PendingMount int    `json:"pending_mount"`
	Orphan       int    `json:"orphan"`
}

// Unrouted is a routable note (by its type) that is not on the index it
// belongs to — the "documents do not scatter" watchdog.
type Unrouted struct {
	Path     string `json:"path"`
	NoteType string `json:"note_type"`
	// ExpectedRoute is the index this type is expected to be filed under.
	ExpectedRoute string `json:"expected_route"`
}

// computeCoverage classifies every concept in the corpus. It first walks the
// reverse edges to learn which concepts are reached and whether by a map, then
// tallies each concept, then runs the all-type routing watchdog. The System/
// tree holds templates and reference notes, not knowledge content, so it is out
// of scope throughout; contract-private paths are excluded from every report,
// so a private note never appears here even when mistyped as a concept.
func computeCoverage(notes []note, idx *graph.Index, authority scanAuthority) Coverage {
	mapped, referenced := mountEdges(notes, idx, authority)
	unrouted := unroutedNotes(notes, mapped, authority)

	conceptType, declared := authority.conceptType()
	if !declared {
		return Coverage{
			Domains:      []DomainCoverage{},
			PendingMount: []string{},
			Orphans:      []string{},
			Unrouted:     unrouted,
			NotApplicable: "contract declares no " + strconv.Quote(conceptType) +
				" type, so there is no concept corpus to judge",
		}
	}

	rows := make(map[string]*DomainCoverage)
	pending := []string{}
	orphans := []string{}
	total := 0
	for i := range notes {
		n := &notes[i]
		if n.noteType != conceptType ||
			strings.HasPrefix(n.path, "System/") ||
			!authority.egressAllowed(n.path) {
			continue
		}
		total++
		domain := n.domain
		if domain == "" {
			domain = "(none)"
		}
		row := rows[domain]
		if row == nil {
			row = &DomainCoverage{Domain: domain}
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

	domainsList := make([]DomainCoverage, 0, len(rows))
	for _, d := range slices.Sorted(maps.Keys(rows)) {
		domainsList = append(domainsList, *rows[d])
	}
	slices.Sort(pending)
	slices.Sort(orphans)
	return Coverage{
		TotalConcepts: total,
		Domains:       domainsList,
		PendingMount:  pending,
		Orphans:       orphans,
		Unrouted:      unrouted,
	}
}

// mountEdges walks the reverse edges to learn which concepts are reached, and
// which of those are reached by a map. A map is a note whose type the vault's
// own contract declares as one; an edge from a map mounts its target, while an
// edge from any other note only references it.
func mountEdges(notes []note, idx *graph.Index, authority scanAuthority) (mapped, referenced map[string]bool) {
	mapped = make(map[string]bool)
	referenced = make(map[string]bool)
	for i := range notes {
		n := &notes[i]
		if strings.HasPrefix(n.path, "System/") || !authority.egressAllowed(n.path) {
			// A private note is not counted as a source: its links must not
			// decide a public concept's mount state, or a reader of coverage
			// could infer that the private note references it.
			continue
		}
		fromMap := authority.roles.IsMapType(n.noteType)
		for _, target := range coverageTargets(n) {
			for _, path := range resolveTargets(idx, target) {
				referenced[path] = true
				if fromMap {
					mapped[path] = true
				}
			}
		}
	}
	return mapped, referenced
}

// unroutedNotes lists the routable non-concept notes that are not on their
// index, ordered by path — the watchdog for a new note nobody filed.
func unroutedNotes(notes []note, mapped map[string]bool, authority scanAuthority) []Unrouted {
	unrouted := []Unrouted{}
	routes := coverageRoutes(authority)
	for i := range notes {
		n := &notes[i]
		if strings.HasPrefix(n.path, "System/") || !authority.egressAllowed(n.path) {
			continue
		}
		if route, ok := routes[n.noteType]; ok && !mapped[n.path] {
			unrouted = append(unrouted, Unrouted{Path: n.path, NoteType: n.noteType, ExpectedRoute: route})
		}
	}
	slices.SortStableFunc(unrouted, func(a, b Unrouted) int {
		return strings.Compare(a.Path, b.Path)
	})
	return unrouted
}

// coverageTargets is every name a note's edges reach: its body wikilink targets
// and its provenance references (based_on, related). The values are resolved
// through resolveTargets, so an ambiguous name contributes an edge to every
// candidate and an ambiguously-referenced concept is never miscounted as an
// orphan.
func coverageTargets(n *note) []string {
	out := make([]string, 0, len(n.wikilinks)+len(n.basedOn)+len(n.related))
	for _, w := range n.wikilinks {
		out = append(out, w.target)
	}
	out = append(out, n.basedOn...)
	out = append(out, n.related...)
	return out
}

// resolveTargets resolves one body-link or provenance value to the note path(s)
// it reaches. The value may be a bare name or a [[wikilink]] carrying a
// |display, #heading, or ^block suffix, stripped the same way a body link is; a
// value that strips to nothing reaches nothing. A unique resolution yields one
// path, an ambiguous one every candidate, and an unresolved one none.
func resolveTargets(idx *graph.Index, value string) []string {
	trimmed := strings.TrimSpace(value)
	inner := trimmed
	if rest, ok := strings.CutPrefix(trimmed, "[["); ok {
		if stripped, ok := strings.CutSuffix(rest, "]]"); ok {
			inner = stripped
		}
	}
	target, ok := stripTarget(inner)
	if !ok {
		return nil
	}
	res := idx.Resolve(target)
	switch res.Kind {
	case graph.Unique:
		return []string{res.RelPath}
	case graph.Ambiguous:
		return res.Candidates
	case graph.Unresolved:
		return nil
	default:
		panic("judge: unknown graph.Kind: " + strconv.Itoa(int(res.Kind)))
	}
}

// coverageRoutes is the index each routable non-concept type belongs on. A
// concept has its own three-way classification above; most types route only
// once their index note exists, so the research brief is the single route.
//
// vault-schema.toml has no vocabulary for routes: it names types and their
// roles, never the note a type is filed under. So this one route cannot be
// derived, and it is offered only to a vault whose contract declares the type
// it is about — a vault that never names the type is not told to file its notes
// under an index it does not have.
func coverageRoutes(authority scanAuthority) map[string]string {
	if !authority.declaresType(researchBriefType) {
		return nil
	}
	return map[string]string{researchBriefType: researchBriefIndex}
}

const (
	researchBriefType = "research-brief"

	// researchBriefIndex names the index note in the vault this route was
	// written for. It is a note title, not contract vocabulary, and nothing in
	// vault-schema.toml can confirm or replace it.
	researchBriefIndex = "Maps/研究 Brief 索引"
)

// renderCoverage renders the coverage report for a terminal reader.
func renderCoverage(c *Coverage) string {
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
	if len(c.Unrouted) > 0 {
		fmt.Fprintf(&s, "unrouted (%d):\n", len(c.Unrouted))
		for _, u := range c.Unrouted {
			fmt.Fprintf(&s, "  %s (%s) -> file under %s\n", u.Path, u.NoteType, u.ExpectedRoute)
		}
	}
	return s.String()
}

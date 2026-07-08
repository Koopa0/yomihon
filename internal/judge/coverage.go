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
// — a concept sits at seedling whether mapped or not. An edge from a map (a
// MOC, topic-map, or source-map) mounts it; an edge only from a lesson or
// another concept leaves it pending on a map; no inbound edge at all is a true
// orphan, the only real problem. The shape is part of the frozen output.

// Coverage is the whole coverage report. Every field is always present; the
// slices are empty rather than absent when there is nothing to list.
type Coverage struct {
	TotalConcepts int              `json:"total_concepts"`
	Domains       []DomainCoverage `json:"domains"`
	// PendingMount lists concepts reached only by a lesson or concept edge,
	// not yet mounted on a map (advisory).
	PendingMount []string `json:"pending_mount"`
	// Orphans lists concepts with no inbound edge at all.
	Orphans []string `json:"orphans"`
	// Unrouted lists non-concept routable notes not yet on their index.
	Unrouted []Unrouted `json:"unrouted"`
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
// of scope throughout; the private daily journal is excluded from every report,
// so a journal note never appears here even when mistyped as a concept.
func computeCoverage(notes []note, idx *graph.Index) Coverage {
	mapped, referenced := mountEdges(notes, idx)

	rows := make(map[string]*DomainCoverage)
	pending := []string{}
	orphans := []string{}
	total := 0
	for i := range notes {
		n := &notes[i]
		if n.noteType != "concept" || strings.HasPrefix(n.path, "System/") || underDiary(n.path) {
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
		Unrouted:      unroutedNotes(notes, mapped),
	}
}

// mountEdges walks the reverse edges to learn which concepts are reached, and
// which of those are reached by a map. A map here is a MOC, topic-map, or
// source-map; an edge from one mounts its target, while an edge from a lesson
// or another concept only references it.
func mountEdges(notes []note, idx *graph.Index) (mapped, referenced map[string]bool) {
	mapped = make(map[string]bool)
	referenced = make(map[string]bool)
	for i := range notes {
		n := &notes[i]
		if strings.HasPrefix(n.path, "System/") || underDiary(n.path) {
			// A note in the private daily journal is not counted as a source:
			// its links must not decide a public concept's mount state, or a
			// reader of coverage could infer that the journal references it.
			continue
		}
		fromMap := n.noteType == "moc" || n.noteType == "topic-map" || n.noteType == "source-map"
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
func unroutedNotes(notes []note, mapped map[string]bool) []Unrouted {
	unrouted := []Unrouted{}
	for i := range notes {
		n := &notes[i]
		if strings.HasPrefix(n.path, "System/") || underDiary(n.path) {
			continue
		}
		if route, ok := routeFor(n.noteType); ok && !mapped[n.path] {
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
		return []string{res.Path}
	case graph.Ambiguous:
		return res.Candidates
	case graph.Unresolved:
		return nil
	default:
		panic("judge: unknown graph.Kind: " + strconv.Itoa(int(res.Kind)))
	}
}

// routeFor reports the index a routable non-concept type belongs on. A concept
// has its own three-way classification above; most types route only once their
// index note exists, so only the research brief is wired here.
func routeFor(noteType string) (string, bool) {
	if noteType == "research-brief" {
		return "Maps/研究 Brief 索引", true
	}
	return "", false
}

// renderCoverage renders the coverage report for a terminal reader.
func renderCoverage(c *Coverage) string {
	var s strings.Builder
	fmt.Fprintf(&s, "%d concepts across %d domains\n", c.TotalConcepts, len(c.Domains))
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
